package t128_graphql

import (
	"strings"
)

//ParsedEntryPoint stores paths and paths to fields, tags and predicates used by queryBuilder and responseProcessor
//TODO: rename tags and fields
//TODO: rename file
type ParsedEntryPoint struct {
	QueryPath  string
	Predicates map[string]string
	Fields     map[string]string
	Tags       map[string]string
}

//ParseEntryPoint converts an entry point into a corresponding responsePath, queryPath and predicates
func ParseEntryPoint(entryPoint string, fieldsIn map[string]string, tagsIn map[string]string) *ParsedEntryPoint {
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
				responsePath += element + "/"
			} else {
				responsePath += element + "/"
			}
		}
	}

	fields := map[string]string{}
	tags := map[string]string{}
	for fieldRenamed, path := range fieldsIn {
		fields[responsePath+path] = fieldRenamed
	}
	for tagRenamed, path := range tagsIn {
		tags[responsePath+path] = tagRenamed
	}

	return &ParsedEntryPoint{QueryPath: queryPath, Predicates: predicateMap, Fields: fields, Tags: tags}
}

func parsePredicate(predicate string) string {
	var replacer = strings.NewReplacer(" ", "")
	return replacer.Replace(predicate)
}
