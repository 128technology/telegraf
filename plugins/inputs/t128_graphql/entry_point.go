package t128_graphql

import (
	"strings"
)

//ParsedEntryPoint stores paths and paths to fields, tags and predicates used by queryBuilder and responseProcessor
type ParsedEntryPoint struct {
	ResponsePath string
	QueryPath    string
	Predicates   map[string]string
}

//ParseEntryPoint converts an entry point into a corresponding responsePath, queryPath and predicates
func ParseEntryPoint(entryPoint string) *ParsedEntryPoint {
	responsePath := "/data/"
	queryPath := ""
	predicateMap := map[string]string{}

	pathElements := strings.Split(entryPoint, "/")
	for idx, element := range pathElements {
		bracketIdx := strings.Index(element, "[")
		colonIdx := strings.Index(element, ":")
		if bracketIdx > 0 {
			queryPath += element[:bracketIdx]
			//TODO: support more complex predicates like (metric: SESSION_COUNT, transform: AVERAGE) and (names:["wan", "lan"], key2:"value2") - MON-315
			predicatePath := queryPath + "." + predicateTag + element[bracketIdx+1:colonIdx]
			predicateMap[predicatePath] = element[colonIdx+1 : len(element)-1]
			queryPath += "."
			responsePath += element[:bracketIdx] + "/"
		} else {
			queryPath += element + "."
			if idx < len(pathElements)-2 {
				responsePath += element + "/0/"
			} else {
				responsePath += element + "/"
			}
		}
	}
	responsePath = strings.TrimRight(responsePath, "/")
	return &ParsedEntryPoint{ResponsePath: responsePath, QueryPath: queryPath, Predicates: predicateMap}
}
