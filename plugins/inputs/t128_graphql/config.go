package t128_graphql

import (
	"strings"
)

//Config stores paths and paths to fields, tags and predicates used by queryBuilder and responseProcessor
type Config struct {
	QueryPath  string
	Predicates map[string]string
	Fields     map[string]string
	Tags       map[string]string
	RawFields  map[string]string //Fields is used with RawFields to build the query
	RawTags    map[string]string
}

//LoadConfig converts an entry point into a corresponding responsePath, queryPath and predicates
func LoadConfig(entryPoint string, fieldsIn map[string]string, tagsIn map[string]string) *Config {
	config := newConfig()
	responsePath := "/data/"
	queryPath := ""
	predicates := map[string]string{}

	pathElements := strings.Split(entryPoint, "/")
	for idx, element := range pathElements {
		parenIdx := strings.Index(element, "(")
		if parenIdx > 0 {
			queryPath += element[:parenIdx]
			predicatePath := queryPath + "." + predicateTag + "predicate"
			predicates[parsePredicate(element[parenIdx:])] = predicatePath
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

	config.QueryPath = queryPath
	config.Predicates = predicates
	config.Fields = fields
	config.Tags = tags
	config.RawFields = fieldsIn
	config.RawTags = tagsIn

	return config
}

func parsePredicate(predicate string) string {
	var replacer = strings.NewReplacer(" ", "")
	return replacer.Replace(predicate)
}

func newConfig() *Config {
	return &Config{
		QueryPath:  "",
		Predicates: map[string]string{},
		Fields:     map[string]string{},
		Tags:       map[string]string{},
		RawFields:  map[string]string{},
		RawTags:    map[string]string{},
	}
}
