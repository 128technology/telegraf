package t128_graphql_test

import (
	"strings"
	"testing"

	plugin "github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
	"github.com/stretchr/testify/require"
)

const (
	ValidQueryDoubleTag   = "query {\nallRouters(name:\"ComboEast\"){\nnodes{\nnodes(name:\"east-combo\"){\nnodes{\narp{\nnodes{\ntest-field\ntest-tag-1\ntest-tag-2}}}}}}}"
	ValidQueryDoubleField = "query {\nallRouters(name:\"ComboEast\"){\nnodes{\nnodes(name:\"east-combo\"){\nnodes{\narp{\nnodes{\ntest-field-1\ntest-field-2\ntest-tag}}}}}}}"
	ValidQueryNestedTag   = "query {\nallRouters(name:\"ComboEast\"){\nnodes{\nnodes(name:\"east-combo\"){\nnodes{\narp{\nnodes{\nstate{\ntest-tag-2}\ntest-field\ntest-tag-1}}}}}}}"
)

var QueryFormationTestCases = []struct {
	Name          string
	EntryPoint    string
	Fields        map[string]string
	Tags          map[string]string
	ExpectedQuery string
}{
	{
		Name:          "build simple query single tag",
		EntryPoint:    "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{"test-tag": "test-tag"},
		ExpectedQuery: ValidQuerySingleTag,
	},
	{
		Name:          "build simple query double tag",
		EntryPoint:    "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "test-tag-2"},
		ExpectedQuery: ValidQueryDoubleTag,
	},
	{
		Name:          "build simple query double field",
		EntryPoint:    "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:        map[string]string{"test-field-1": "test-field-1", "test-field-2": "test-field-2"},
		Tags:          map[string]string{"test-tag": "test-tag"},
		ExpectedQuery: ValidQueryDoubleField,
	},
	{
		Name:          "build query nested tag",
		EntryPoint:    "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:        map[string]string{"test-field": "test-field"},
		Tags:          map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state/test-tag-2"},
		ExpectedQuery: ValidQueryNestedTag,
	},
	{
		Name:       "build query multi-level-nested tag",
		EntryPoint: "allRouters(name:\"ComboEast\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state1/state2/state3/test-tag-2"},
		ExpectedQuery: strings.ReplaceAll(`query {
			allRouters(name:"ComboEast"){
			nodes{
			nodes(name:"east-combo"){
			nodes{
			arp{
			nodes{
			state1{
			state2{
			state3{
			test-tag-2}}}
			test-field
			test-tag-1}}}}}}}`, "\t", ""),
	},
	{
		Name:       "build query list predicate",
		EntryPoint: "allRouters(names:[\"wan\",\"lan\"])/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state1/state2/state3/test-tag-2"},
		ExpectedQuery: strings.ReplaceAll(`query {
			allRouters(names:["wan","lan"]){
			nodes{
			nodes(name:"east-combo"){
			nodes{
			arp{
			nodes{
			state1{
			state2{
			state3{
			test-tag-2}}}
			test-field
			test-tag-1}}}}}}}`, "\t", ""),
	},
	{
		Name:       "build query multiple predicates",
		EntryPoint: "allRouters(names:[\"wan\", \"lan\"], key2:\"value2\")/nodes/nodes(name:\"east-combo\")/nodes/arp/nodes",
		Fields:     map[string]string{"test-field": "test-field"},
		Tags:       map[string]string{"test-tag-1": "test-tag-1", "test-tag-2": "state1/state2/state3/test-tag-2"},
		ExpectedQuery: strings.ReplaceAll(`query {
			allRouters(names:["wan","lan"],key2:"value2"){
			nodes{
			nodes(name:"east-combo"){
			nodes{
			arp{
			nodes{
			state1{
			state2{
			state3{
			test-tag-2}}}
			test-field
			test-tag-1}}}}}}}`, "\t", ""),
	},
}

func TestT128GraphqlQueryFormation(t *testing.T) {
	for _, testCase := range QueryFormationTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			query := plugin.BuildQuery(testCase.EntryPoint, testCase.Fields, testCase.Tags)
			require.Equal(t, testCase.ExpectedQuery, query)
		})
	}
}
