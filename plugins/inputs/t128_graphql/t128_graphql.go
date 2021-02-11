package t128_graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/inputs"
)

const (
	// DefaultRequestTimeout is the request timeout if none is configured
	DefaultRequestTimeout = time.Second * 5
)

// T128GraphQL is an input for metrics of a 128T router instance
type T128GraphQL struct {
	CollectorName string            `toml:"collector_name"`
	BaseURL       string            `toml:"base_url"`
	UnixSocket    string            `toml:"unix_socket"`
	EntryPoint    string            `toml:"entry_point"`
	Fields        map[string]string `toml:"extract_fields"`
	Tags          map[string]string `toml:"extract_tags"`
	Timeout       internal.Duration `toml:"timeout"`

	Query          string
	JSONEntryPoint string
	requestBody    []byte
	client         *http.Client
}

// SampleConfig returns the default configuration of the Input
func (*T128GraphQL) SampleConfig() string {
	return sampleConfig
}

// Description returns a one-sentence description on the Input
func (*T128GraphQL) Description() string {
	return "Make a 128T GraphQL query and return the data"
}

// Init sets up the input to be ready for action
func (plugin *T128GraphQL) Init() error {

	//check config
	err := plugin.checkConfig()
	if err != nil {
		return err
	}

	//build query, json path to data and request body
	plugin.Query = buildQuery(plugin.EntryPoint, plugin.Fields, plugin.Tags)
	plugin.JSONEntryPoint = buildJSONPathFromEntryPoint(plugin.EntryPoint)

	content := struct {
		Query string `json:"query,omitempty"`
	}{
		plugin.Query,
	}

	body, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to create request body for query '%s': %w", plugin.Query, err)
	}
	plugin.requestBody = body

	//setup client
	transport := http.DefaultTransport

	if plugin.UnixSocket != "" {
		transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", plugin.UnixSocket)
			},
		}
	}

	plugin.client = &http.Client{Transport: transport, Timeout: plugin.Timeout.Duration}

	return nil
}

func (plugin *T128GraphQL) checkConfig() error {
	if plugin.CollectorName == "" {
		return fmt.Errorf("collector_name is a required configuration field")
	}

	if plugin.BaseURL == "" {
		return fmt.Errorf("base_url is a required configuration field")
	}

	if !strings.HasSuffix(plugin.BaseURL, "/") {
		plugin.BaseURL += "/"
	}

	if plugin.EntryPoint == "" {
		return fmt.Errorf("entry_point is a required configuration field")
	}

	if plugin.Fields == nil {
		return fmt.Errorf("extract_fields is a required configuration field")
	}

	if plugin.Tags == nil {
		return fmt.Errorf("extract_tags is a required configuration field")
	}

	return nil
}

// Gather takes in an accumulator and adds the metrics that the Input gathers
func (plugin *T128GraphQL) Gather(acc telegraf.Accumulator) error {
	request, err := plugin.createRequest()
	if err != nil {
		acc.AddError(fmt.Errorf("failed to create a request for query %s: %w", plugin.Query, err))
		return nil
	}

	response, err := plugin.client.Do(request)
	if err != nil {
		acc.AddError(fmt.Errorf("failed to make graphQL request for collector %s: %w", plugin.CollectorName, err))
		return nil
	}
	defer response.Body.Close()

	message, err := ioutil.ReadAll(response.Body)
	if err != nil {
		message = []byte("")
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		template := fmt.Sprintf("status code %d not OK for collector ", response.StatusCode) + plugin.CollectorName + ": %s"
		for _, err := range decodeAndReportJSONErrors(message, template) {
			acc.AddError(err)
		}
		return nil
	}

	//decode json
	jsonParsed, err := gabs.ParseJSON(message)
	if err != nil {
		acc.AddError(fmt.Errorf("invalid json response for collector %s: %w", plugin.CollectorName, err))
		return nil
	}

	//TODO: move all of this into ProcessResponse() - MON-315
	jsonObj, err := jsonParsed.JSONPointer(plugin.JSONEntryPoint)
	if err != nil {
		template := "unexpected response for collector " + plugin.CollectorName + ": %s"
		for _, err := range decodeAndReportJSONErrors(message, template) {
			acc.AddError(err)
		}
		return nil
	}

	//update acc
	jsonChildren, err := jsonObj.Children()
	if err != nil {
		acc.AddError(fmt.Errorf("failed to iterate on response nodes for collector %s: %w", plugin.CollectorName, err))
		return nil
	}

	plugin.processResponse(jsonChildren, acc)
	return nil
}

func (plugin *T128GraphQL) processResponse(jsonChildren []*gabs.Container, acc telegraf.Accumulator) {
	for _, child := range jsonChildren {
		node := child.Data().(map[string]interface{})
		fields := make(map[string]interface{})
		tags := make(map[string]string)

		for fieldRenamed, fieldName := range plugin.Fields {
			data := node[fieldName]

			if strings.Contains(fieldName, "/") {
				nestedObj, err := child.JSONPointer("/" + fieldName)
				if err != nil {
					acc.AddError(fmt.Errorf("unexpected response for collector %s: field %s", plugin.CollectorName, fieldName))
					continue
				}
				data = nestedObj.Data()
			}

			if isNil(data) {
				acc.AddError(fmt.Errorf("found empty data for collector %s: field %s", plugin.CollectorName, fieldName))
				continue
			}
			fields[fieldRenamed] = data
		}

		for tagRenamed, tagName := range plugin.Tags {
			data := node[tagName]

			if strings.Contains(tagName, "/") {
				nestedObj, err := child.JSONPointer("/" + tagName)
				if err != nil {
					acc.AddError(fmt.Errorf("unexpected response for collector %s: tag %s", plugin.CollectorName, tagName))
					continue
				}
				data = nestedObj.Data()
			}

			if isNil(data) {
				tags[tagRenamed] = ""
				continue
			}
			tags[tagRenamed] = fmt.Sprintf("%v", data)
		}

	for _, processedResponse := range processedResponses {
		acc.AddFields(
			plugin.CollectorName,
			processedResponse.Fields,
			processedResponse.Tags,
		)
	}
	return nil
}

func (plugin *T128GraphQL) createRequest() (*http.Request, error) {
	request, err := http.NewRequest("POST", plugin.BaseURL, bytes.NewReader(plugin.requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request for query '%s': %w", plugin.Query, err)
	}

	request.Header.Add("Content-Type", "application/json")

	return request, nil
}

func buildJSONPathFromEntryPoint(entryPoint string) string {
	path := "/data/"
	pathElements := strings.Split(entryPoint, "/")
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

func decodeAndReportJSONErrors(response []byte, template string) []error {
	var errors []error

	parsedJSON, err := gabs.ParseJSON(response)
	if err != nil {
		errors = append(errors, fmt.Errorf(template, response))
		return errors
	}

	jsonObj, err := parsedJSON.JSONPointer("/errors")
	if err != nil {
		errors = append(errors, fmt.Errorf(template, parsedJSON.String()))
		return errors
	}

	jsonChildren, err := jsonObj.Children()
	if err != nil {
		errors = append(errors, fmt.Errorf(template, parsedJSON.String()))
		return errors
	}

	for _, child := range jsonChildren {
		errorNode := child.Data().(map[string]interface{})
		errors = append(errors, fmt.Errorf(template, errorNode["message"].(string)))
	}
	return errors
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
		return &T128GraphQL{
			Timeout: internal.Duration{Duration: DefaultRequestTimeout},
		}
	})
}
