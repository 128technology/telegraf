package t128_graphql_test

import (
	"testing"

	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
	"github.com/stretchr/testify/require"
)

var JSONPathFormationTestCases = []struct {
	Name           string
	EntryPoint     string
	Fields         map[string]string
	Tags           map[string]string
	ExpectedOutput *plugin.ParsedEntryPoint
}{
	{
		Name:       "process simple input",
		EntryPoint: "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag": "test-tag"},
		ExpectedOutput: &plugin.ParsedEntryPoint{
			QueryPath: "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(name:\"ComboEast\")":  "allRouters.$predicate",
				"(name:\"east-combo\")": "allRouters.nodes.nodes.$predicate",
			},
			Fields: map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-field": "test-field"},
			Tags:   map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-tag": "test-tag"},
		},
	},
	{
		Name:       "process predicate with list",
		EntryPoint: "allRouters(names:[\"wan\",\"lan\"])/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag": "test-tag"},
		ExpectedOutput: &plugin.ParsedEntryPoint{
			QueryPath: "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(names:[\"wan\",\"lan\"])": "allRouters.$predicate",
				"(name:\"east-combo\")":     "allRouters.nodes.nodes.$predicate",
			},
			Fields: map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-field": "test-field"},
			Tags:   map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-tag": "test-tag"},
		},
	},
	{
		Name:       "process multiple predicates",
		EntryPoint: "allRouters(names:[\"wan\", \"lan\"], key2:\"value2\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag": "test-tag"},
		ExpectedOutput: &plugin.ParsedEntryPoint{
			QueryPath: "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(names:[\"wan\",\"lan\"],key2:\"value2\")": "allRouters.$predicate",
				"(name:\"east-combo\")":                     "allRouters.nodes.nodes.$predicate",
			},
			Fields: map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-field": "test-field"},
			Tags:   map[string]string{"/data/allRouters/nodes/nodes/nodes/arp/nodes/test-tag": "test-tag"},
		},
	},
}

func TestT128GraphqlEntryPointParsing(t *testing.T) {
	for _, testCase := range JSONPathFormationTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			parsedEntryPoint := plugin.ParseEntryPoint(testCase.EntryPoint, testCase.Fields, testCase.Tags)
			require.Equal(t, testCase.ExpectedOutput, parsedEntryPoint)
		})
	}
}
