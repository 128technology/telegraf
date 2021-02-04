package t128_graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

const (
	//make this configurable?
	DefaultRequestTimeout = time.Second * 5
)

type T128GraphQL struct {
	CollectorName string            `toml:"collector_name"`
	BaseURL       string            `toml:"base_url"`
	EntryPoint    string            `toml:"entry_point"`
	Fields        map[string]string `toml:"extract_fields"`
	Tags          map[string]string `toml:"extract_tags"`

	Client *http.Client
	Query  string
}

var sampleConfig = `
## Collect data using graphQL
[[inputs.t128_graphql]]
## Required. The telegraf collector name
# collector_name = "arp-state"

## Required. GraphQL ports vary across 128T versions
# base_url = "http://localhost:31517"

## The starting point in the graphQL tree for all configured tags and fields
# entry_point = "allRouters[name:RTR_WEST_COMBO]/nodes/nodes[name:combo-west]/nodes/arp/nodes"

## The fields to collect with the desired name as the key (left) and graphQL 
## key as the value (right)
# [inputs.t128_graphql.extract_fields]
#   state = "state"

## The tags for filtering data with the desired name as the key (left) and 
## graphQL key as the value (right)
# [inputs.t128_graphql.extract_tags]
#   networkInterface = "networkInterface"
#   deviceInterface = "deviceInterface"
#   vlan = "vlan"
#   ipAddress = "ipAddress"
#   destinationMac = "destinationMac"
`

func (*T128GraphQL) SampleConfig() string {
	return sampleConfig
}

func (*T128GraphQL) Description() string {
	return "Read data from a graphQL query"
}

func (plugin *T128GraphQL) Init() error {
	if plugin.CollectorName == "" {
		return fmt.Errorf("collector_name is a required configuration field")
	}

	if plugin.BaseURL == "" {
		return fmt.Errorf("base_url is a required configuration field")
	}

	if plugin.BaseURL[len(plugin.BaseURL)-1:] != "/" {
		plugin.BaseURL += "/"
	}

	transport := http.DefaultTransport

	plugin.Client = &http.Client{Transport: transport, Timeout: DefaultRequestTimeout}
	plugin.Query = plugin.buildQuery()

	return nil
}

func (plugin *T128GraphQL) Gather(acc telegraf.Accumulator) error {
	timestamp := time.Now().Round(time.Second)

	plugin.retrieveMetrics(acc, timestamp)

	return nil
}

func (plugin *T128GraphQL) buildQuery() string {
	//allow multiple inputs like names[ComboEast,ComboWest] ?
	var replacer = strings.NewReplacer("[", "(", "]", "\")", "/", "{", ":", ":\"")
	query := "query MyQuery{" + replacer.Replace(plugin.EntryPoint) + "{"

	for _, element := range plugin.Fields {
		query += "\n" + element
	}
	query = strings.TrimSpace(query)
	for _, element := range plugin.Tags {
		query += "\n" + element
	}

	query += "\n" + strings.Repeat("}", strings.Count(query, "{"))

	return query
}

func (plugin *T128GraphQL) retrieveMetrics(acc telegraf.Accumulator, timestamp time.Time) {
	request, err := plugin.createRequest()
	if err != nil {
		acc.AddError(fmt.Errorf("failed to create a request for query %s: %w", plugin.Query, err))
		return
	}

	response, err := plugin.Client.Do(request)
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)

	if err != nil {
		template := "query error for metric %s: %s"
		plugin.DecodeAndReportJsonErrors(buf.Bytes(), acc, template)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		template := fmt.Sprintf("status code %d not OK for metric ", response.StatusCode) + "%s: %s"
		plugin.DecodeAndReportJsonErrors(buf.Bytes(), acc, template)
		return
	}

	//decode json
	jsonParsed, err := gabs.ParseJSON(buf.Bytes())
	if err != nil {
		acc.AddError(fmt.Errorf("invalid json response for metric %s: %w", plugin.CollectorName, err))
		return
	}

	jsonEntryPoint := plugin.BuildJsonEntryPoint()
	jsonObj, err := jsonParsed.JSONPointer(jsonEntryPoint)
	if err != nil {
		template := "unexpected response for metric %s: %s"
		plugin.DecodeAndReportJsonErrors(buf.Bytes(), acc, template)
		return
	}

	//update acc
	jsonChildren, err := jsonObj.Children()
	if err != nil {
		acc.AddError(fmt.Errorf("failed to iterate on response nodes for metric %s: %w", plugin.CollectorName, err))
		return
	}

	//TODO: allow nested fields/tags
	for _, child := range jsonChildren {
		node := child.Data().(map[string]interface{})
		fields := make(map[string]interface{})
		tags := make(map[string]string)

		//allow integer conversion?
		//TODO: fully implement multiple fields
		for fieldRenamed, fieldName := range plugin.Fields {
			if isNil(node[fieldName]) {
				acc.AddError(fmt.Errorf("found empty data for metric %s: field %s", plugin.CollectorName, fieldName))
				continue
			}
			fields[fieldRenamed] = node[fieldName]

			for tagRenamed, tagName := range plugin.Tags {
				if isNil(node[tagName]) {
					tags[tagRenamed] = tagRenamed
					continue
				}
				//gabs module converts data to either string or float64
				strTag, ok := node[tagName].(string)
				if !ok {
					floatTag := node[tagName].(float64)
					strTag = strconv.FormatFloat(floatTag, 'f', -1, 64)
				}
				tags[tagRenamed] = strTag
			}

			acc.AddFields(
				plugin.CollectorName,
				fields,
				tags,
				timestamp,
			)
		}
	}
}

func (plugin *T128GraphQL) createRequest() (*http.Request, error) {
	content := struct {
		Query string `json:"query,omitempty"`
	}{
		plugin.Query,
	}

	//inject env config?
	body, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to create request body for query '%s': %w", plugin.Query, err)
	}

	url := plugin.BaseURL + "api/v1/graphql"
	request, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request for query '%s': %w", plugin.Query, err)
	}

	request.Header.Add("Content-Type", "application/json")

	return request, nil
}

func (plugin *T128GraphQL) BuildJsonEntryPoint() string {
	path := "/data/"
	pathElements := strings.Split(plugin.EntryPoint, "/")
	for idx, element := range pathElements {
		bracketIdx := strings.Index(element, "[")
		if bracketIdx > 0 {
			path += element[:bracketIdx] + "/"
		} else {
			if idx < len(pathElements)-2 {
				path += element + "/0/"
			} else {
				path += element + "/"
			}
		}
	}
	path = strings.TrimRight(path, "/")
	return path
}

func (plugin *T128GraphQL) DecodeAndReportJsonErrors(response []byte, acc telegraf.Accumulator, template string) {
	parsedJSON, err := gabs.ParseJSON(response)
	if err != nil {
		acc.AddError(fmt.Errorf(template, plugin.CollectorName, response))
		return
	}

	jsonObj, err := parsedJSON.JSONPointer("/errors")
	if err != nil {
		acc.AddError(fmt.Errorf(template, plugin.CollectorName, parsedJSON.String()))
		return
	}

	jsonChildren, err := jsonObj.Children()
	if err != nil {
		acc.AddError(fmt.Errorf(template, plugin.CollectorName, parsedJSON.String()))
		return
	}

	for _, child := range jsonChildren {
		errorNode := child.Data().(map[string]interface{})
		acc.AddError(fmt.Errorf(template, plugin.CollectorName, errorNode["message"].(string)))
	}
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

func init() {
	inputs.Add("t128_graphql", func() telegraf.Input {
		return &T128GraphQL{}
	})
}
