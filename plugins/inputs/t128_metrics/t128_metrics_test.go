package t128_metrics_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_metrics"
	"github.com/influxdata/telegraf/testutil"
	"github.com/influxdata/toml"
	"github.com/stretchr/testify/require"
)

type Endpoint struct {
	URL             string
	Code            int
	ExpectedRequest string
	Response        string
}

var ResponseProcessingTestCases = []struct {
	Name              string
	ConfiguredMetrics []plugin.ConfiguredMetric
	Endpoints         []Endpoint
	ExpectedMetrics   []*testutil.Metric
	ExpectedErrors    []string
}{
	{
		Name:              "empty configured metrics produce no requests or metrics",
		ConfiguredMetrics: []plugin.ConfiguredMetric{},
		Endpoints:         []Endpoint{},
		ExpectedMetrics:   nil,
		ExpectedErrors:    nil,
	},
	{
		Name: "empty results produce no metrics",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints:       []Endpoint{{"/stats/test", 200, "{}", "[]"}},
		ExpectedMetrics: nil,
		ExpectedErrors:  nil,
	},
	{
		Name: "none value produces no metric",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [],
				"value": null
			}]
		}]`}},
		ExpectedMetrics: nil,
		ExpectedErrors:  nil,
	},
	{
		Name: "forms string value if it is non numeric",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [],
				"value": "test-string"
			}]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"test-field": "test-string"},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "forms int value if it is numeric",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [],
				"value": "50"
			}]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"test-field": 50},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "forms float value if it is numeric",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [],
				"value": "50.5"
			}]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"test-field": 50.5},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "adds permutation parameters to metrics",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [
					{
						"name": "node",
						"value": "node1"
					},
					{
						"name": "interface",
						"value": "intf1"
					}
				],
				"value": "0"
			}]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{"node": "node1", "interface": "intf1"},
				Fields:      map[string]interface{}{"test-field": 0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "produces multiple metrics for multiple permutations",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test",
			"permutations": [
				{
					"parameters": [
						{
							"name": "node",
							"value": "node1"
						}
					],
					"value": "897"
				},
				{
					"parameters": [
						{
							"name": "node",
							"value": "node2"
						}
					],
					"value": "306"
				}
			]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{"node": "node1"},
				Fields:      map[string]interface{}{"test-field": 897},
			},
			{
				Measurement: "test-metric",
				Tags:        map[string]string{"node": "node2"},
				Fields:      map[string]interface{}{"test-field": 306},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "hits multiple endpoints",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}, {
			"another-test-metric",
			map[string]string{"another-test-field": "stats/another/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", `[{
			"id": "/stats/test-metric",
			"permutations": [{
				"parameters": [],
				"value": "50"
			}]
		}]`}, {
			"/stats/another/test", 200, "{}", `[{
			"id": "/stats/another/test",
			"permutations": [{
				"parameters": [],
				"value": "60"
			}]
		}]`}},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"test-field": 50},
			},
			{
				Measurement: "another-test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"another-test-field": 60},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name: "propogates errors to accumulator",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"404",
			map[string]string{"field": "stats/404"},
			map[string][]string{},
		}, {
			"300",
			map[string]string{"field": "stats/300"},
			map[string][]string{},
		}, {
			"invalid-json",
			map[string]string{"field": "stats/invalid-json"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{
			{"/stats/404", 404, "{}", `it's not right`},
			{"/stats/300", 300, "{}", `it's not right`},
			{"/stats/invalid-json", 200, "{}", `{"test": }`},
		},
		ExpectedMetrics: nil,
		ExpectedErrors: []string{
			"status code 404 not OK for metric stats/404: it's not right",
			"status code 300 not OK for metric stats/300: it's not right",
			"failed to decode response for metric stats/invalid-json: invalid character '}' looking for beginning of value",
		},
	},
	{
		Name: "mixes errors and valid results",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"404",
			map[string]string{"field": "stats/404"},
			map[string][]string{},
		}, {
			"test-metric",
			map[string]string{"field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{
			{"/stats/404", 404, "{}", `it's not right`},
			{"/stats/test", 200, "{}", `[{
				"id": "/stats/test-metric",
				"permutations": [{
					"parameters": [],
					"value": "50"
				}]
			}]`},
		},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-metric",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"field": 50},
			},
		},
		ExpectedErrors: []string{
			"status code 404 not OK for metric stats/404: it's not right",
		},
	},
}

var RequestFormationTestCases = []struct {
	Name              string
	ConfiguredMetrics []plugin.ConfiguredMetric
	Endpoints         []Endpoint
}{
	{
		Name: "empty request body with no parameters",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, "{}", "[]"}},
	},
	{
		Name: "itemizes with no filter values for empty list",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{"interface": {}},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, `{"parameters": [{"name": "interface", "itemize": true}]}`, "[]"}},
	},
	{
		Name: "includes parameter filter values",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{"interface": {"intf1", "intf2"}},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, `{
				"parameters": [
					{"name": "interface", "values": ["intf1", "intf2"], "itemize": true}
				]
			}`,
			"[]"}},
	},
	{
		Name: "includes multiple parameter filters",
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{
				"interface": {"intf1", "intf2"},
				"node":      {"node1", "node2"},
				"other":     {}},
		}},
		Endpoints: []Endpoint{{"/stats/test", 200, `{
				"parameters": [
					{"name": "interface", "values": ["intf1", "intf2"], "itemize": true},
					{"name": "node", "values": ["node1", "node2"], "itemize": true},
					{"name": "other", "itemize": true}
				]
			}`,
			"[]"}},
	},
}

func TestT128MetricsResponseProcessing(t *testing.T) {
	for _, testCase := range ResponseProcessingTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fakeServer := createTestServer(t, testCase.Endpoints)
			defer fakeServer.Close()

			plugin := &plugin.T128Metrics{
				BaseURL:                 fakeServer.URL,
				MaxSimultaneousRequests: 20,
				ConfiguredMetrics:       testCase.ConfiguredMetrics,
			}

			var acc testutil.Accumulator
			require.NoError(t, plugin.Init())

			plugin.Gather(&acc)

			var errorStrings []string = nil
			for _, err := range acc.Errors {
				errorStrings = append(errorStrings, err.Error())
			}

			require.ElementsMatch(t, testCase.ExpectedErrors, errorStrings)

			// Timestamps aren't important, but need to match
			for _, m := range acc.Metrics {
				m.Time = time.Time{}
			}

			// Avoid specifying this unused type for each field
			for _, m := range testCase.ExpectedMetrics {
				m.Type = telegraf.Untyped
			}

			require.ElementsMatch(t, testCase.ExpectedMetrics, acc.Metrics)
		})
	}
}

func TestT128MetricsRequestFormation(t *testing.T) {
	for _, testCase := range RequestFormationTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fakeServer := createTestServer(t, testCase.Endpoints)
			defer fakeServer.Close()

			plugin := &plugin.T128Metrics{
				BaseURL:                 fakeServer.URL,
				MaxSimultaneousRequests: 20,
				ConfiguredMetrics:       testCase.ConfiguredMetrics,
			}

			var acc testutil.Accumulator
			require.NoError(t, plugin.Init())

			require.NoError(t, acc.GatherError(plugin.Gather))
		})
	}
}

func TestT128MetricsRequestLimiting(t *testing.T) {
	inFlight := int32(0)
	const limit = 3
	const metricCount = 20

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		currentInFlight := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)

		fmt.Println(currentInFlight)
		require.LessOrEqual(t, currentInFlight, int32(limit))

		time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
		w.Write([]byte("[]"))
	}))

	configuredMetrics := make([]plugin.ConfiguredMetric, 0, metricCount)
	for i := 0; i < metricCount; i++ {
		configuredMetrics = append(configuredMetrics, plugin.ConfiguredMetric{
			fmt.Sprintf("test-metric-%d", i),
			map[string]string{"test-field": fmt.Sprintf("stats/test/%d", i)},
			map[string][]string{},
		})
	}

	plugin := &plugin.T128Metrics{
		BaseURL:                 fakeServer.URL,
		MaxSimultaneousRequests: limit,
		ConfiguredMetrics:       configuredMetrics,
	}

	var acc testutil.Accumulator
	require.NoError(t, plugin.Init())

	require.NoError(t, acc.GatherError(plugin.Gather))
}

func TestTimoutUsedForRequests(t *testing.T) {
	done := make(chan struct{}, 1)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}

		w.Write([]byte("[]"))
	}))

	plugin := &plugin.T128Metrics{
		BaseURL: fakeServer.URL,
		ConfiguredMetrics: []plugin.ConfiguredMetric{{
			"test-metric",
			map[string]string{"test-field": "stats/test"},
			map[string][]string{},
		}},
		Timeout:                 internal.Duration{Duration: 1 * time.Millisecond},
		MaxSimultaneousRequests: 1,
	}

	var acc testutil.Accumulator
	require.NoError(t, plugin.Init())

	require.NoError(t, plugin.Gather(&acc))
	done <- struct{}{}

	require.Len(t, acc.Errors, 1)
	require.EqualError(
		t,
		acc.Errors[0],
		fmt.Sprintf("failed to retrieve metric stats/test: Post %s/stats/test: net/http: request canceled (Client.Timeout exceeded while awaiting headers)", fakeServer.URL))

	fakeServer.Close()
}

func TestEmptyBaseURLIsInvalid(t *testing.T) {
	plugin := &plugin.T128Metrics{}
	err := plugin.Init()

	require.Errorf(t, err, "base_url is a require configuration field")
}

func TestZeroMaxSimultaneousRequestsIsInvalid(t *testing.T) {
	plugin := &plugin.T128Metrics{BaseURL: "/example"}
	err := plugin.Init()

	require.Errorf(t, err, "max_simultaneous_requests must be greater than 0")
}

func TestLoadsFromToml(t *testing.T) {
	expectedMetrics := []plugin.ConfiguredMetric{{
		Name:       "cpu",
		Fields:     map[string]string{"my_field": "field_value"},
		Parameters: map[string][]string{"my_parameter": {"value1", "value2"}},
	}}

	plugin := &plugin.T128Metrics{}
	exampleConfig := []byte(`
	base_url                    = "example/base/url/"
	unix_socket                 = "example.sock"
	max_simultaneous_requests   = 15
	timeout                     = "500ms"
	[[metric]]
	  name = "cpu"
	  [metric.fields]
	    my_field = "field_value"
	  [metric.parameters]
	    my_parameter = ["value1", "value2"]
	`)

	require.NoError(t, toml.Unmarshal(exampleConfig, plugin))
	require.Equal(t, "example/base/url/", plugin.BaseURL)
	require.Equal(t, "example.sock", plugin.UnixSocket)
	require.Equal(t, 500*time.Millisecond, plugin.Timeout.Duration)
	require.Equal(t, expectedMetrics, plugin.ConfiguredMetrics)
}

func createTestServer(t *testing.T, e []Endpoint) *httptest.Server {
	endpoints := make(map[string]Endpoint)
	for _, endpoint := range e {
		endpoints[endpoint.URL] = endpoint
	}

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "POST", r.Method)

		endpoint, ok := endpoints[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			return
		}

		if endpoint.ExpectedRequest != "" {
			contents, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				return
			}

			require.JSONEq(t, endpoint.ExpectedRequest, string(contents), "Unexpected request body for endpoint %s", endpoint.URL)
		}

		w.WriteHeader(endpoint.Code)
		w.Write([]byte(endpoint.Response))
	}))

	return fakeServer
}
