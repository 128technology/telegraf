package t128_graphql

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Jeffail/gabs"
)

func buildQuery(entryPoint string, fields map[string]string, tags map[string]string) string {
	query := "query "

	var buf bytes.Buffer
	w := io.Writer(&buf)
	jsonObj := buildQueryObject(entryPoint, fields, tags)
	traverseQueryObjPreOrder(jsonObj, w)

	query += buf.String()
	return query
}

func traverseQueryObjPreOrder(jsonObj *gabs.Container, w io.Writer) {
	jsonChildren, err := jsonObj.ChildrenMap()
	if err != nil {
		fmt.Println("error")
		return
	}

	//store and sort the keys
	keys := make([]string, len(jsonChildren))
	i := 0
	for k := range jsonChildren {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	children := 0
	for i, key := range keys {
		if strings.HasPrefix(key, "$") {
			w.Write([]byte("(" + key[1:] + ":\"" + jsonChildren[key].Data().(string) + "\"){"))
			continue
		}
		children++
		_, ok := jsonChildren[key].Data().(string)
		if !ok {
			if i == 0 {
				w.Write([]byte("{"))
			}
			w.Write([]byte("\n" + key))
			traverseQueryObjPreOrder(jsonChildren[key], w)
			continue
		}
		if i == 0 {
			w.Write([]byte("{"))
		}
		w.Write([]byte("\n" + key))
	}

	if children > 0 {
		w.Write([]byte("}"))
	}
}

func buildQueryObject(entryPoint string, fields map[string]string, tags map[string]string) *gabs.Container {
	jsonObj := gabs.New()

	//add items in entry point to obj
	path := ""
	pathElements := strings.Split(entryPoint, "/")
	for _, element := range pathElements {
		bracketIdx := strings.Index(element, "[")
		//handle predicates like (name:"ComboEast")
		if bracketIdx > 0 {
			path += element[:bracketIdx]
			predicates := strings.Split(element[bracketIdx+1:len(element)-1], ",")
			for _, predicate := range predicates {
				colonIdx := strings.Index(predicate, ":")
				jsonObj.SetP(predicate[colonIdx+1:], path+".$"+predicate[:colonIdx])
			}
			path += "."
		} else {
			path += element + "."
		}
	}

	//add fields to obj
	for fieldRenamed, field := range fields {
		var replacer = strings.NewReplacer("/", ".")
		partialPath := replacer.Replace(field)
		jsonObj.SetP(fieldRenamed, path+partialPath)

	}

	//add tags to obj
	for tagRenamed, tag := range tags {
		var replacer = strings.NewReplacer("/", ".")
		partialPath := replacer.Replace(tag)
		jsonObj.SetP(tagRenamed, path+partialPath)

	}

	return jsonObj
}

//buildQueryBody creates an intermediary query object that is traversed by buildQueryBody
func buildQueryObject(entryPoint string, fields map[string]string, tags map[string]string) *gabs.Container {
	jsonObj := gabs.New()

	parsedEntryPoint := ParseEntryPoint(entryPoint)

	addToQueryObj(jsonObj, parsedEntryPoint.Predicates, true, false, "")
	addToQueryObj(jsonObj, fields, false, true, parsedEntryPoint.QueryPath)
	addToQueryObj(jsonObj, tags, false, true, parsedEntryPoint.QueryPath)

	return jsonObj
}

func addToQueryObj(jsonObj *gabs.Container, items map[string]string, keyIsPath bool, usesSlash bool, basePath string) {
	var replacer = strings.NewReplacer("/", ".")
	for key, value := range items {
		if keyIsPath {
			if usesSlash {
				partialPath := replacer.Replace(key)
				jsonObj.SetP(value, basePath+partialPath)
			} else {
				jsonObj.SetP(value, basePath+key)
			}
		} else {
			if usesSlash {
				partialPath := replacer.Replace(value)
				jsonObj.SetP(key, basePath+partialPath)
			} else {
				jsonObj.SetP(key, basePath+value)
			}
		}
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
			w.Write(
				[]byte("(" +
					key[1:] + //key example - "$name"
					":\"" +
					jsonChildren[key].Data().(string) + //jsonChildren[key] example - "ComboEast"
					"\"){")) //written output example - (name:"ComboEast")
			continue
		}
		children++
		_, ok := jsonChildren[key].Data().(string)
		if !ok { //means jsonChildren[key] is not a leaf and we need to traverse further
			if i == 0 {
				w.Write([]byte("{"))
			}
			w.Write([]byte("\n" + key))          //visit current node in pre-order traversal
			buildQueryBody(jsonChildren[key], w) //visit child nodes in pre-order traversal
			continue
		}
		if i == 0 {
			w.Write([]byte("{"))
		}
		w.Write([]byte("\n" + key)) //visit a leaf node
	}

	if children > 0 {
		w.Write([]byte("}"))
	}
}
