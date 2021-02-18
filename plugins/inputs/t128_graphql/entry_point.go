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
		parenIdx := strings.Index(element, "(")
		if parenIdx > 0 {
			queryPath += element[:parenIdx]
			predicatePath := queryPath + "." + predicateTag + "predicate"
			predicateMap[parsePredicate(element[parenIdx:])] = predicatePath
			queryPath += "."
			responsePath += element[:parenIdx] + "/"
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

func parsePredicate(predicate string) string {
	//TODO: switch back brackets and parens
	var replacer = strings.NewReplacer(" ", "")
	return replacer.Replace(predicate)
}
