package t128_graphql

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Jeffail/gabs"
)

const (
	predicateTag = "$"
)

/*
BuildQuery first creates an intermediary query object that is traversed by buildQueryBody() in pre-order

Args:
	entryPoint example - "allRouters(name:\"ComboEast\)/nodes/nodes(name:\"combo-east\")/nodes/arp/nodes"
	fields example - map[string]string{"enabled": "enabled"}
	tags example - map[string]string{
			"name": "name",
			"admin-status":  "state/adminStatus",
		}

Example:
	For the example input above, buildQueryObject() will produce the following query object

	{
		"allRouters": {
			"$predicate": "(name:\"ComboEast\")",
			"nodes": {
				"nodes": {
					"$predicate":"(name:\"east-combo\")",
					"nodes": {
						"arp": {
							"nodes": {
								"enabled":"enabled",
								"name":"name",
								"state": {
									"adminStatus":
									"admin-status"
								}
							}
						}
					}
				}
			}
		}
	}

	And buildQueryBody() will build the following

	{
	allRouters(name:"ComboEast"){
	nodes{
	nodes(name:"combo-east"){
	nodes{
	arp{
	nodes{
	enabled
	name
	state{
	adminStatus}}}}}}}}
*/
func BuildQuery(config *Config) string {
	query := "query "

	var buf bytes.Buffer
	w := io.Writer(&buf)
	jsonObj := buildQueryObject(config)
	buildQueryBody(jsonObj, w)

	query += buf.String()
	return query
}

//buildQueryBody creates an intermediary query object that is traversed by buildQueryBody
func buildQueryObject(config *Config) *gabs.Container {
	jsonObj := gabs.New()

	//parsedEntryPoint := ParseEntryPoint(entryPoint, fields, tags)

	addToQueryObj(jsonObj, config.Predicates, "")
	addToQueryObj(jsonObj, formatPaths(config.RawFields), config.QueryPath)
	addToQueryObj(jsonObj, formatPaths(config.RawTags), config.QueryPath)

	return jsonObj
}

// the config uses a "/" json syntax but json helpers call for "." syntax
func formatPaths(predicates map[string]string) map[string]string {
	newMap := make(map[string]string)
	var replacer = strings.NewReplacer("/", ".")

	for key, val := range predicates {
		newMap[key] = replacer.Replace(val)
	}
	return newMap
}

func addToQueryObj(jsonObj *gabs.Container, items map[string]string, basePath string) {
	for key, value := range items {
		jsonObj.SetP(key, basePath+value)
	}
}

//buildQueryBody builds the graphql query body by traversing jsonObj in pre-order and writing to the provided writer
func buildQueryBody(jsonObj *gabs.Container, w io.Writer) {
	jsonChildren, err := jsonObj.ChildrenMap()
	if err != nil {
		fmt.Println("error")
		return
	}

	//sort the keys for testing and to handle "$" syntax for predicates
	keys := make([]string, len(jsonChildren))
	i := 0
	for k := range jsonChildren {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	children := 0
	for i, key := range keys {
		//add predicates like (name:"ComboEast") to the query
		if strings.HasPrefix(key, predicateTag) {
			writePredicate(w, fmt.Sprintf("%v", jsonChildren[key].Data())+"{")
			continue
		}
		children++
		_, ok := jsonChildren[key].Data().(string)
		if !ok { //means jsonChildren[key] is not a leaf and we need to traverse further
			if i == 0 {
				startSection(w)
			}
			writeElement(w, key)                 //visit current node in pre-order traversal
			buildQueryBody(jsonChildren[key], w) //visit child nodes in pre-order traversal
			continue
		}
		if i == 0 {
			startSection(w)
		}
		writeElement(w, key) //visit a leaf node
	}

	if children > 0 {
		endSection(w)
	}
}

func startSection(w io.Writer) {
	w.Write([]byte("{"))
}

func endSection(w io.Writer) {
	w.Write([]byte("}"))
}

func writePredicate(w io.Writer, pred string) {
	w.Write([]byte(pred))
}

func writeElement(w io.Writer, elem string) {
	w.Write([]byte("\n" + elem))
}
