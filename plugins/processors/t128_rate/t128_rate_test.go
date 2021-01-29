package rename

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/toml"
	"github.com/stretchr/testify/assert"
)

func newMetric(name string, tags map[string]string, fields map[string]interface{}, timestamp time.Time) telegraf.Metric {
	if tags == nil {
		tags = map[string]string{}
	}
	if fields == nil {
		fields = map[string]interface{}{}
	}
	m, _ := metric.New(name, tags, fields, timestamp)
	return m
}

func TestRemovesFirstSample(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{"/my/rate": "/my/rate"}
	assert.Nil(t, r.Init())

	m := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50}, time.Now())

	rate := r.Apply(m)
	assert.Len(t, rate, 1)
	assert.Empty(t, rate[0].FieldList())
}

func TestRemovesExpiredSample(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{"/my/rate": "/my/rate"}
	r.Expiration.Duration = 10 * time.Second
	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(10 * time.Second)
	t3 := t2.Add(10*time.Second + 1*time.Nanosecond)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 60}, t2)
	m3 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 70}, t3)

	r.Apply(m1)
	assert.Len(t, r.Apply(m2)[0].FieldList(), 1)
	assert.Len(t, r.Apply(m3)[0].FieldList(), 0)
}

func TestCalculatesInitialRate(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{
		"/my/rate": "/my/rate",
	}

	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(time.Second * 1)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 60}, t2)

	rate := r.Apply(m1)
	assert.Len(t, rate, 1)
	assert.Empty(t, rate[0].FieldList())

	rate = r.Apply(m2)
	assert.Len(t, rate, 1)

	assert.Equal(t, newMetric("foo", nil, map[string]interface{}{"/my/rate": float64(10)}, t2), rate[0])
}

func TestCalculatesRateAfterExpiration(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{
		"/my/rate": "/my/rate",
	}
	r.Expiration.Duration = time.Second * 1

	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(time.Second * 5)
	t3 := t2.Add(time.Second * 1)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 60}, t2)
	m3 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 65}, t3)

	r.Apply(m1)

	processorResult := r.Apply(m2)
	assert.Len(t, processorResult, 1)
	assert.Empty(t, processorResult[0].FieldList())

	processorResult = r.Apply(m3)
	assert.Len(t, processorResult, 1)
	assert.Equal(t, newMetric("foo", nil, map[string]interface{}{"/my/rate": float64(5)}, t3), processorResult[0])
}

func TestCalculatesRatesOverTime(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{
		"/my/rate":       "/my/rate",
		"/my/other/rate": "/my/other/total",
	}

	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(time.Second * 1)
	t3 := t2.Add(time.Second * 1)
	t4 := t3.Add(time.Second * 1)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 60}, t2)
	m3 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 65}, t3)
	m4 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 65}, t4)

	r.Apply(m1)

	metrics := []telegraf.Metric{m2, m3, m4}
	rates := []float64{10, 5, 0}

	for i := 0; i < 3; i++ {
		processorResult := r.Apply(metrics[i])
		assert.Len(t, processorResult, 1)

		resultMetric := processorResult[0]
		assert.Equal(t, newMetric("foo", nil, map[string]interface{}{"/my/rate": rates[i]}, metrics[i].Time()), resultMetric)
	}
}

func TestLeavesUnmarkedFieldsInTact(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{"/my/rate": "/my/rate"}
	assert.Nil(t, r.Init())

	t1 := time.Now()
	t2 := t1.Add(time.Second * 1)

	m1 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 50, "/unmarked": float64(50)}, t1)
	m2 := newMetric("foo", nil, map[string]interface{}{"/my/rate": 60, "/unmarked": float64(60)}, t2)

	outputs := r.Apply(m1)
	assert.Len(t, outputs, 1)
	assert.Len(t, outputs[0].FieldList(), 1)

	v, ok := outputs[0].GetField("/unmarked")
	assert.True(t, ok)
	assert.Equal(t, float64(50), v)

	outputs = r.Apply(m2)
	assert.Len(t, outputs, 1)

	v, ok = outputs[0].GetField("/my/rate")
	assert.True(t, ok)
	assert.Equal(t, float64(10), v)

	v, ok = outputs[0].GetField("/unmarked")
	assert.True(t, ok)
	assert.Equal(t, float64(60), v)
}

func TestRemoveOriginalAndRename(t *testing.T) {
	testCases := []struct {
		Name            string
		Fields          map[string]string
		LastSample      map[string]interface{}
		CurrentSample   map[string]interface{}
		TimeDelta       time.Duration
		RemoveOriginal  bool
		RemainingFields []string
	}{
		{
			Name:            "remove-matching-new",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{},
		},
		{
			Name:            "remove-matching-existing",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{"/rate": 45},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{"/rate"},
		},
		{
			Name:            "remove-matching-expired",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{"/rate": 45},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       6 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{},
		},
		{
			Name:            "remove-mismatching-new",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{},
		},
		{
			Name:            "remove-mismatching-existing",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{"/non-rate": 45},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{"/rate"},
		},
		{
			Name:            "remove-mismatching-expired",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{"/non-rate": 45},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       6 * time.Second,
			RemoveOriginal:  true,
			RemainingFields: []string{},
		},

		{
			Name:            "leave-matching-new",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{},
		},
		{
			Name:            "leave-matching-existing",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{"/rate": 45},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{"/rate"},
		},
		{
			Name:            "leave-matching-expired",
			Fields:          map[string]string{"/rate": "/rate"},
			LastSample:      map[string]interface{}{"/rate": 45},
			CurrentSample:   map[string]interface{}{"/rate": 50},
			TimeDelta:       6 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{},
		},
		{
			Name:            "leave-mismatching-new",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{"/non-rate"},
		},
		{
			Name:            "leave-mismatching-existing",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{"/non-rate": 45},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       5 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{"/non-rate", "/rate"},
		},
		{
			Name:            "leave-mismatching-expired",
			Fields:          map[string]string{"/rate": "/non-rate"},
			LastSample:      map[string]interface{}{"/non-rate": 45},
			CurrentSample:   map[string]interface{}{"/non-rate": 50},
			TimeDelta:       6 * time.Second,
			RemoveOriginal:  false,
			RemainingFields: []string{"/non-rate"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			r := newRate()
			r.Fields = testCase.Fields
			r.RemoveOriginal = testCase.RemoveOriginal
			r.Expiration.Duration = 5 * time.Second
			assert.Nil(t, r.Init())

			t1 := time.Now()
			m1 := newMetric("foo", nil, testCase.LastSample, t1)
			m2 := newMetric("foo", nil, testCase.CurrentSample, t1.Add(testCase.TimeDelta))

			r.Apply(m1)
			result := r.Apply(m2)[0]

			assert.Len(t, result.FieldList(), len(testCase.RemainingFields))

			for _, field := range testCase.RemainingFields {
				_, exists := result.GetField(field)
				assert.Truef(t, exists, "the field '%v' doesn't exist", field)
			}
		})
	}
}

func TestFailsOnConflictingFieldMappings(t *testing.T) {
	r := newRate()
	r.Fields = map[string]string{
		"/my/rate":       "/my/total",
		"/my/other/rate": "/my/total",
	}

	assert.EqualError(t, r.Init(), "both '/my/other/rate' and '/my/rate' are configured to be calculated from '/my/total'")
}

func TestLoadsFromToml(t *testing.T) {

	plugin := &T128Rate{}
	exampleConfig := []byte(`
		expiration = "10s"

		[fields]
			"/my/rate" = "/my/total"
			"/other/rate" = "/other/total"
	`)

	assert.NoError(t, toml.Unmarshal(exampleConfig, plugin))
	assert.Equal(t, map[string]string{"/my/rate": "/my/total", "/other/rate": "/other/total"}, plugin.Fields)
	assert.Equal(t, plugin.Expiration.Duration, 10*time.Second)
}
