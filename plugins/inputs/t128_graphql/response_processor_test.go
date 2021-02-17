package t128_graphql_test

import (
	"fmt"
	"testing"

	"github.com/Jeffail/gabs"
	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
	"github.com/stretchr/testify/require"
)

//TODO: more unit tests - MON-314
var ResponseProcessingTestCases = []struct {
	Name           string
	Fields         map[string]string
	Tags           map[string]string
	JsonInput      []*gabs.Container
	ExpectedOutput []*plugin.ProcessedResponse
	ExpectedError  error
}{
	{
		Name:   "none value produces error",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": null,
				"test-tag": "test-string"
			}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{},
		ExpectedError:  fmt.Errorf("found empty data for collector test-collector: field test-field"),
	},
	{
		Name:   "converts tag to string if numeric",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
				"test-tag": 128
			}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag": "128"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "uses multiple fields",
		Fields: map[string]string{"test-field-1": "test-field-1", "test-field-2": "test-field-2"},
		Tags:   map[string]string{"test-tag": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field-1": 128,
				"test-field-2": 95,
				"test-tag": "test-string"
	  		}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field-1": 128.0, "test-field-2": 95.0},
				Tags:   map[string]string{"test-tag": "test-string"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "uses multiple tags",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
		  		"test-tag-1": "test-string-1",
		  		"test-tag-2": "test-string-2"
	  		}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag-1": "test-string-1", "test-tag-2": "test-string-2"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "uses multiple tags with some none value",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
		  		"test-tag-1": "test-string-1",
		  		"test-tag-2": null
	  		}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag-1": "test-string-1", "test-tag-2": ""},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "renames tags and fields",
		Fields: map[string]string{"test-field-renamed": "test-field"},
		Tags:   map[string]string{"test-tag-renamed": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
				"test-tag": 128
			}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field-renamed": 128.0},
				Tags:   map[string]string{"test-tag-renamed": "128"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "process response with multiple nodes",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
				"test-tag": "test-string-1"
			}`)),
			generateJsonTestData([]byte(`{
				"test-field": 95,
				"test-tag": "test-string-2"
			}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag": "test-string-1"},
			},
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 95.0},
				Tags:   map[string]string{"test-tag": "test-string-2"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "process response with nested tags",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state/test-tag-2"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
				"test-tag-1": "test-string-1",
			  	"state": {
				  "test-tag-2": "test-string-2"
			  	}
		  	}`)),
			generateJsonTestData([]byte(`{
				"test-field": 95,
				"test-tag-1": "test-string-3",
				"state": {
					"test-tag-2": "test-string-4"
				}
			}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag-1": "test-string-1", "test-tag-2": "test-string-2"},
			},
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 95.0},
				Tags:   map[string]string{"test-tag-1": "test-string-3", "test-tag-2": "test-string-4"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "process response with multi-level nested tags",
		Fields: map[string]string{"test-field": "test-field"},
		Tags:   map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state1/state2/state3/test-tag-2"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field": 128,
				"test-tag-1": "test-string-1",
			  	"state1": {
					"state2": {
						"state3": {
							"test-tag-2": "test-string-2"
						}
					}
			  	}
		  	}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag-1": "test-string-1", "test-tag-2": "test-string-2"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "process response with multi-level nested fields",
		Fields: map[string]string{"test-field-1": "state1/state2/state3/test-field-1", "test-field-2": "test-field-2"},
		Tags:   map[string]string{"test-tag": "test-tag"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-field-2": 128,
				"test-tag": "test-string-1",
			  	"state1": {
					"state2": {
						"state3": {
							"test-field-1": 95
						}
					}
			  	}
		  	}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field-1": 95.0, "test-field-2": 128.0},
				Tags:   map[string]string{"test-tag": "test-string-1"},
			},
		},
		ExpectedError: nil,
	},
	{
		Name:   "process response with mixed nesting",
		Fields: map[string]string{"test-field": "state1/state2/test-field"},
		Tags:   map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state1/state2/state3/test-tag-2"},
		JsonInput: []*gabs.Container{
			generateJsonTestData([]byte(`{
				"test-tag-1": "test-string-1",
			  	"state1": {
					"state2": {
						"test-field": 128,
						"state3": {
							"test-tag-2": "test-string-2"
						}
					}
			  	}
		  	}`)),
		},
		ExpectedOutput: []*plugin.ProcessedResponse{
			&plugin.ProcessedResponse{
				Fields: map[string]interface{}{"test-field": 128.0},
				Tags:   map[string]string{"test-tag-1": "test-string-1", "test-tag-2": "test-string-2"},
			},
		},
		ExpectedError: nil,
	},
}

func generateJsonTestData(data []byte) *gabs.Container {
	gabsData, err := gabs.ParseJSON(data)
	if err != nil {
		panic(err)
	}
	return gabsData
}

func TestT128GraphqlResponseProcessing(t *testing.T) {
	for _, testCase := range ResponseProcessingTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			processedResponse, err := plugin.ProcessResponse(
				testCase.JsonInput,
				"test-collector",
				testCase.Fields,
				testCase.Tags,
			)

			require.Equal(t, testCase.ExpectedError, err)
			require.ElementsMatch(t, testCase.ExpectedOutput, processedResponse)
		})
	}
}
