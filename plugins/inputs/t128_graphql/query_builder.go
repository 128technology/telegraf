package t128_graphql

import "strings"

func buildQuery(entryPoint string, fields map[string]string, tags map[string]string) string {
	var replacer = strings.NewReplacer("[", "(", "]", "\")", "/", "{", ":", ":\"")
	query := "query {" + replacer.Replace(entryPoint) + "{"

	for _, element := range fields {
		query += "\n" + element
	}
	query = strings.TrimSpace(query)
	for _, element := range tags {
		query += "\n" + element
	}

	query += "\n" + strings.Repeat("}", strings.Count(query, "{"))

	return query
}
