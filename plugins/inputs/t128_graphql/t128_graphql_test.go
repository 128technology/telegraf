package t128_graphql_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

type Endpoint struct {
	URL             string
	Code            int
	ExpectedRequest string
	Response        string
}

const (
	ValidExpectedRequestNoTag     = `{"query":"query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\n}}}}}}}"}`
	ValidQueryNoTag               = "query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\n}}}}}}}"
	ValidExpectedRequestSingleTag = `{"query":"query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\ntest-tag\n}}}}}}}"}`
	ValidQuerySingleTag           = "query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\ntest-tag\n}}}}}}}"
	ValidExpectedRequestDoubleTag = `{"query":"query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\ntest-tag-1\ntest-tag-2\n}}}}}}}"}`
	ValidQueryDoubleTag           = "query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\ntest-tag-1\ntest-tag-2\n}}}}}}}"
	InvalidRouterExpectedRequest  = `{"query":"query {allRouters(name:\"not-a-router\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\n}}}}}}}"}`
	InvalidRouterQuery            = "query {allRouters(name:\"not-a-router\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ntest-field\n}}}}}}}"
	InvalidFieldExpectedRequest   = `{"query":"query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ninvalid-field\n}}}}}}}"}`
	InvalidFieldQuery             = "query {allRouters(name:\"ComboEast\"){nodes{nodes(name:\"east-combo\"){nodes{arp{nodes{\ninvalid-field\n}}}}}}}"
)

var ResponseProcessingTestCases = []struct {
	Name            string
	EntryPoint      string
	Fields          map[string]string
	Tags            map[string]string
	Query           string
	Endpoint        Endpoint
	ExpectedMetrics []*testutil.Metric
	ExpectedErrors  []string
}{
	{
		Name:            "empty query produces no request or metrics",
		EntryPoint:      "",
		Fields:          nil,
		Tags:            nil,
		Query:           "",
		Endpoint:        Endpoint{},
		ExpectedMetrics: nil,
		ExpectedErrors:  nil,
	},
	{
		Name:            "empty response produces error",
		EntryPoint:      "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:          map[string]string{"test-field": "test-field"},
		Tags:            map[string]string{},
		Query:           ValidQueryNoTag,
		Endpoint:        Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestNoTag, "{}"},
		ExpectedMetrics: nil,
		ExpectedErrors: []string{
			"unexpected response for collector test-collector: {}",
		},
	},
	{
		Name:       "none value produces error",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{},
		Query:      ValidQueryNoTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestNoTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": null
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: nil,
		ExpectedErrors: []string{
			"found empty data for collector test-collector: field test-field",
		},
	},
	{
		Name:       "converts tag to string if numeric",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag": "test-tag"},
		Query:      ValidQuerySingleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestSingleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": 128,
								  		"test-tag": 128
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag": "128"},
				Fields:      map[string]interface{}{"test-field": 128.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:       "uses multiple fields",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field-1": "test-field-1", "test-field-2": "test-field-2"},
		Tags:       map[string]string{},
		Query:      ValidQueryDoubleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestDoubleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field-1": 128,
										"test-field-2": 95
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{},
				Fields:      map[string]interface{}{"test-field-1": 128.0, "test-field-2": 95.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:       "uses multiple tags",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		Query:      ValidQueryDoubleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestDoubleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": 128,
										"test-tag-1": "test-string-1",
										"test-tag-2": "test-string-2"
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag-1": "test-string-1", "test-tag-2": "test-string-2"},
				Fields:      map[string]interface{}{"test-field": 128.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:       "uses multiple tags with some none value",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		Query:      ValidQueryDoubleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestDoubleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": 128,
										"test-tag-1": "test-string-1",
										"test-tag-2": null
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag-1": "test-string-1", "test-tag-2": ""},
				Fields:      map[string]interface{}{"test-field": 128.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:       "renames tags and fields",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field-renamed": "test-field"},
		Tags:       map[string]string{"test-tag-renamed": "test-tag"},
		Query:      ValidQuerySingleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestSingleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": 128,
								  		"test-tag": "test-string"
									}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag-renamed": "test-string"},
				Fields:      map[string]interface{}{"test-field-renamed": 128.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:       "processes response with multiple nodes",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag": "test-tag"},
		Query:      ValidQuerySingleTag,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestSingleTag, `{
			"data": {
				"allRouters": {
				  	"nodes": [{
					  	"nodes": {
							"nodes": [{
								"arp": {
							  		"nodes": [{
								  		"test-field": 128,
								  		"test-tag": "test-string-1"
									},
									{
										"test-field": 95,
										"test-tag": "test-string-2"
								  	}]
								}
						  	}]
					  	}
					}]
				}
			}
		}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag": "test-string-1"},
				Fields:      map[string]interface{}{"test-field": 128.0},
			},
			{
				Measurement: "test-collector",
				Tags:        map[string]string{"test-tag": "test-string-2"},
				Fields:      map[string]interface{}{"test-field": 95.0},
			},
		},
		ExpectedErrors: nil,
	},
	{
		Name:            "propogates not found error to accumulator",
		EntryPoint:      "allRouters[name:not-a-router]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:          map[string]string{"test-field": "test-field"},
		Tags:            map[string]string{},
		Query:           InvalidRouterQuery,
		Endpoint:        Endpoint{"/api/v1/graphql/", 404, InvalidRouterExpectedRequest, `it's not right`},
		ExpectedMetrics: nil,
		ExpectedErrors: []string{
			"status code 404 not OK for collector test-collector: it's not right",
		},
	},
	{
		Name:            "propogates invalid json error to accumulator",
		EntryPoint:      "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:          map[string]string{"test-field": "test-field"},
		Tags:            map[string]string{},
		Query:           ValidQueryNoTag,
		Endpoint:        Endpoint{"/api/v1/graphql/", 200, ValidExpectedRequestNoTag, `{"test": }`},
		ExpectedMetrics: nil,
		ExpectedErrors:  []string{"invalid json response for collector test-collector: invalid character '}' looking for beginning of value"},
	},
	{
		Name:       "propogates graphQL error to accumulator",
		EntryPoint: "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:     map[string]string{"invalid-field": "invalid-field"},
		Tags:       map[string]string{},
		Query:      InvalidFieldQuery,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, InvalidFieldExpectedRequest, `
		{
			"errors": [{
				"name": "GraphQLError",
				"message": "Cannot query field \"invalid-field\" on type \"ArpEntryType\".",
				"locations": [{
					"line": 2,
					"column": 1
				}]
			}]
		  }`},
		ExpectedMetrics: nil,
		ExpectedErrors:  []string{"unexpected response for collector test-collector: Cannot query field \"invalid-field\" on type \"ArpEntryType\"."},
	},
}

var QueryFormationTestCases = []struct {
	Name          string
	EntryPoint    string
	Fields        map[string]string
	Tags          map[string]string
	ExpectedQuery string
}{
	{
		Name:          "convert simple query no tag",
		EntryPoint:    "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{},
		ExpectedQuery: ValidQueryNoTag,
	},
	{
		Name:          "convert simple query single tag",
		EntryPoint:    "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{"test-tag": "test-tag"},
		ExpectedQuery: ValidQuerySingleTag,
	},
	{
		Name:          "convert simple query double tag",
		EntryPoint:    "allRouters[name:ComboEast]/nodes/nodes[name:east-combo]/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		ExpectedQuery: ValidQueryDoubleTag,
	},
}

func TestT128GraphqlResponseProcessing(t *testing.T) {
	for _, testCase := range ResponseProcessingTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fakeServer := createTestServer(t, testCase.Endpoint)
			defer fakeServer.Close()

			plugin := &plugin.T128GraphQL{
				CollectorName: "test-collector",
				BaseURL:       fakeServer.URL + "/api/v1/graphql",
				EntryPoint:    testCase.EntryPoint,
				Fields:        testCase.Fields,
				Tags:          testCase.Tags,
			}

			var acc testutil.Accumulator

			if plugin.EntryPoint == "" {
				require.Error(t, plugin.Init())
				return
			} else {
				require.NoError(t, plugin.Init())
			}

			plugin.Query = testCase.Query
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

func TestT128GraphqlQueryFormation(t *testing.T) {
	for _, testCase := range QueryFormationTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			plugin := &plugin.T128GraphQL{
				CollectorName: "test-collector",
				BaseURL:       "/api/v1/graphql",
				EntryPoint:    testCase.EntryPoint,
				Fields:        testCase.Fields,
				Tags:          testCase.Tags,
			}

			require.NoError(t, plugin.Init())
			require.Equal(t, testCase.ExpectedQuery, plugin.Query)
		})
	}
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

	plugin := &plugin.T128GraphQL{
		CollectorName: "test-collector",
		BaseURL:       fakeServer.URL + "/api/v1/graphql",
		EntryPoint:    "fake/entry/point",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{},
		Timeout:       internal.Duration{Duration: 1 * time.Millisecond},
	}

	var acc testutil.Accumulator
	require.NoError(t, plugin.Init())

	require.NoError(t, plugin.Gather(&acc))
	done <- struct{}{}

	require.Len(t, acc.Errors, 1)
	require.EqualError(
		t,
		acc.Errors[0],
		fmt.Sprintf("failed to make graphQL request for collector test-collector: Post \"%s/api/v1/graphql/\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)", fakeServer.URL))

	fakeServer.Close()
}

func createTestServer(t *testing.T, endpoint Endpoint) *httptest.Server {
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "POST", r.Method)

		if endpoint.URL != r.URL.Path {
			fmt.Printf("There isn't an endpoint for: %v\n", r.URL.Path)
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
