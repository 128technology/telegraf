package t128_transform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStateChangeSendFirstSample(t *testing.T) {
	r := newTransformType("state-change")
	r.Fields = map[string]string{"/my/state": "/my/state"}
	assert.Nil(t, r.Init())

	m := newMetric("foo", nil, map[string]interface{}{"/my/state": "state1"}, time.Now())

	rate := r.Apply(m)
	assert.Len(t, rate, 1)
	assert.Equal(t, rate[0].Fields()["/my/state"], "state1")
}

func TestStateChangeSendsOnExpired(t *testing.T) {
	r := newTransformType("state-change")
	r.Fields = map[string]string{"/my/state": "/my/state"}
	r.Expiration.Duration = 10 * time.Second
	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(10 * time.Second)
	t3 := t2.Add(10*time.Second + 1*time.Nanosecond)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/state": "state1"}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/state": "state1"}, t2)
	m3 := newMetric("foo", nil, map[string]interface{}{"/my/state": "state1"}, t3)

	r.Apply(m1)
	assert.Len(t, r.Apply(m2)[0].FieldList(), 0)
	assert.Len(t, r.Apply(m3)[0].FieldList(), 1)
}

func TestStateChangeLeavesUnmarkedFieldsInTact(t *testing.T) {
	r := newTransformType("state-change")
	r.Fields = map[string]string{"/my/state": "/my/state"}
	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(time.Second * 1)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/state": 50, "/unmarked": 50}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/state": 60, "/unmarked": 60}, t2)

	outputs := r.Apply(m1)
	assert.Len(t, outputs, 1)
	assert.Len(t, outputs[0].FieldList(), 2)

	v, ok := outputs[0].GetField("/my/state")
	assert.True(t, ok)
	assert.Equal(t, int64(50), v)

	v, ok = outputs[0].GetField("/unmarked")
	assert.True(t, ok)
	assert.Equal(t, int64(50), v)

	outputs = r.Apply(m2)
	assert.Len(t, outputs, 1)

	v, ok = outputs[0].GetField("/my/state")
	assert.True(t, ok)
	assert.Equal(t, int64(60), v)

	v, ok = outputs[0].GetField("/unmarked")
	assert.True(t, ok)
	assert.Equal(t, int64(60), v)
}

func TestStateChangeRemoveOriginalAndRename(t *testing.T) {
	type sample = map[string]interface{}

	testCases := []struct {
		Name           string
		Fields         map[string]string
		Samples        []sample
		PreviousFields map[string]string
		TimeDelta      time.Duration
		RemoveOriginal bool
		Result         sample
	}{
		{
			Name:   "remove-matching-new",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{},
				{"/state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "remove-matching-existing-same",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 45},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{},
		},
		{
			Name:   "remove-matching-existing-changed",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "remove-matching-expired",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 50},
			},
			TimeDelta:      6 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "remove-mismatching-new",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{},
				{"/non-state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "remove-mismatching-existing-same",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 45},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{},
		},
		{
			Name:   "remove-mismatching-existing-changed",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "remove-mismatching-expired",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 50},
			},
			TimeDelta:      6 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "leave-matching-new",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{},
				{"/state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "leave-matching-existing-same",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 45},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{},
		},
		{
			Name:   "leave-matching-existing-changed",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "leave-matching-expired",
			Fields: map[string]string{"/state": "/state"},
			Samples: []sample{
				{"/state": 45},
				{"/state": 50},
			},
			TimeDelta:      6 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/state": int64(50)},
		},
		{
			Name:   "leave-mismatching-new",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{},
				{"/non-state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/non-state": int64(50), "/state": int64(50)},
		},
		{
			Name:   "leave-mismatching-existing-same",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 45},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/non-state": int64(45)},
		},
		{
			Name:   "leave-mismatching-existing-changed",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 50},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/non-state": int64(50), "/state": int64(50)},
		},
		{
			Name:   "leave-mismatching-expired",
			Fields: map[string]string{"/state": "/non-state"},
			Samples: []sample{
				{"/non-state": 45},
				{"/non-state": 50},
			},
			TimeDelta:      6 * time.Second,
			RemoveOriginal: false,
			Result:         sample{"/non-state": int64(50), "/state": int64(50)},
		},

		{
			Name:           "previous-new",
			Fields:         map[string]string{"/state": "/state"},
			PreviousFields: map[string]string{"/previous": "/state"},
			Samples: []sample{
				{"/state": "s1"},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": "s1"},
		},
		{
			Name:           "previous-with-previous-value",
			Fields:         map[string]string{"/state": "/state"},
			PreviousFields: map[string]string{"/previous": "/state"},
			Samples: []sample{
				{"/state": "s1"},
				{"/state": "s2"},
			},
			TimeDelta:      5 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": "s2", "/previous": "s1"},
		},
		{
			Name:           "previous-matching-expired",
			Fields:         map[string]string{"/state": "/state"},
			PreviousFields: map[string]string{"/previous": "/state"},
			Samples: []sample{
				{"/state": "s1"},
				{"/state": "s2"},
			},
			TimeDelta:      6 * time.Second,
			RemoveOriginal: true,
			Result:         sample{"/state": "s2", "/previous": "s1"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			r := newTransformType("state-change")
			r.Fields = testCase.Fields
			r.PreviousFields = testCase.PreviousFields
			r.RemoveOriginal = testCase.RemoveOriginal
			r.Expiration.Duration = 5 * time.Second
			assert.Nil(t, r.Init())

			t1 := time.Now()
			for i := 0; i < len(testCase.Samples)-1; i++ {
				timestamp := t1.Add(time.Duration(i) * testCase.TimeDelta)
				r.Apply(newMetric("foo", nil, testCase.Samples[i], timestamp))
			}

			m := newMetric("foo", nil,
				testCase.Samples[len(testCase.Samples)-1],
				t1.Add(testCase.TimeDelta*time.Duration(len(testCase.Samples)-1)),
			)

			result := r.Apply(m)[0]

			assert.Equal(t, result.Fields(), testCase.Result)
		})
	}
}
