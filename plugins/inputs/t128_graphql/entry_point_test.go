package t128_graphql_test

import (
	"testing"

	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
	"github.com/stretchr/testify/require"
)

//TODO: more unit tests - MON-314
var JSONPathFormationTestCases = []struct {
	Name           string
	EntryPoint     string
	ExpectedOutput *plugin.ParsedEntryPoint
}{
	{
		Name:       "build arp state json path",
		EntryPoint: "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		ExpectedOutput: &plugin.ParsedEntryPoint{
			ResponsePath: "/data/allRouters/nodes/0/nodes/nodes/0/arp/nodes",
			QueryPath:    "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(name:\"ComboEast\")":  "allRouters.$predicate",
				"(name:\"east-combo\")": "allRouters.nodes.nodes.$predicate",
			},
		},
	},
	{
		Name:       "process predicate with list",
		EntryPoint: "allRouters(names:[\"wan\",\"lan\"])/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		ExpectedOutput: &plugin.ParsedEntryPoint{
			ResponsePath: "/data/allRouters/nodes/0/nodes/nodes/0/arp/nodes",
			QueryPath:    "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(names:[\"wan\",\"lan\"])": "allRouters.$predicate",
				"(name:\"east-combo\")":     "allRouters.nodes.nodes.$predicate",
			},
		},
	},
	{
		Name:       "process multiple sub-predicates",
		EntryPoint: "allRouters(names:[\"wan\", \"lan\"], key2:\"value2\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		ExpectedOutput: &plugin.ParsedEntryPoint{
			ResponsePath: "/data/allRouters/nodes/0/nodes/nodes/0/arp/nodes",
			QueryPath:    "allRouters.nodes.nodes.nodes.arp.nodes.",
			Predicates: map[string]string{
				"(names:[\"wan\",\"lan\"],key2:\"value2\")": "allRouters.$predicate",
				"(name:\"east-combo\")":                     "allRouters.nodes.nodes.$predicate",
			},
		},
	},
}

func TestT128GraphqlEntryPointParsing(t *testing.T) {
	for _, testCase := range JSONPathFormationTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			parsedEntryPoint := plugin.ParseEntryPoint(testCase.EntryPoint)
			require.Equal(t, testCase.ExpectedOutput, parsedEntryPoint)
		})
	}
}
