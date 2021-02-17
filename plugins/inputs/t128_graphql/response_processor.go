package t128_graphql

import (
	"fmt"
	"strings"

	"github.com/Jeffail/gabs"
)

//ProcessedResponse stores the processed fields and tags for injection into telegraf accumulator
type ProcessedResponse struct {
	Fields map[string]interface{}
	Tags   map[string]string
}

// ProcessResponse takes in a query response, pulls out the desired data and stores it in a struct
func ProcessResponse(jsonChildren []*gabs.Container, collectorName string, fields map[string]string, tags map[string]string) ([]*ProcessedResponse, error) {
	var processedResponses []*ProcessedResponse

	//TODO: support queries needed by mist agent - MON-315
	for _, child := range jsonChildren {
		//TODO: safely type-asert if keeping - MON-315
		node := child.Data().(map[string]interface{})
		//TODO: move to init if keeping - MON-315
		processedResponse := &ProcessedResponse{Fields: map[string]interface{}{}, Tags: map[string]string{}}

		for fieldRenamed, fieldName := range fields {
			data := node[fieldName]

			if strings.Contains(fieldName, "/") {
				//TODO: clean this up, it's the second time JSONPointer() is called on the response - MON-315
				nestedObj, err := child.JSONPointer("/" + fieldName)
				if err != nil {
					return nil, fmt.Errorf("unexpected response for collector %s: field %s", collectorName, fieldName)
				}
				data = nestedObj.Data()
			}

			if isNil(data) {
				return nil, fmt.Errorf("found empty data for collector %s: field %s", collectorName, fieldName)
			}
			processedResponse.Fields[fieldRenamed] = data
		}

		for tagRenamed, tagName := range tags {
			data := node[tagName]

			if strings.Contains(tagName, "/") {
				nestedObj, err := child.JSONPointer("/" + tagName)
				if err != nil {
					return nil, fmt.Errorf("unexpected response for collector %s: tag %s", collectorName, tagName)
				}
				data = nestedObj.Data()
			}

			if isNil(data) {
				processedResponse.Tags[tagRenamed] = ""
				continue
			}

			processedResponse.Tags[tagRenamed] = fmt.Sprintf("%v", data)
		}

		processedResponses = append(processedResponses, processedResponse)
	}

	return processedResponses, nil
}
