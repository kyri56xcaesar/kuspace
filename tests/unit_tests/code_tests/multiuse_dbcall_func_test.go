package test

import (
	"fmt"
	"log"
	"strings"
	"testing"
)

func Get(sel, table, by, byvalue string, limit int) {
	if sel == "" {
		sel = "*"
	}
	query := fmt.Sprintf("SELECT %s FROM %s", sel, table)

	var placeholderStr string
	var args []any
	for _, arg := range strings.Split(strings.TrimSpace(byvalue), ",") {
		args = append(args, arg)
	}
	if by != "" && byvalue != "" {
		by_values_split := strings.Split(strings.TrimSpace(byvalue), ",")
		placeholders := make([]string, len(by_values_split))
		for i := range by_values_split {
			placeholders[i] = "?"
		}
		placeholderStr = strings.Join(placeholders, ",")

		query = fmt.Sprintf("%s WHERE %s (%s)", query, by, placeholderStr)
	}

	if limit > 0 {
		limit_str := fmt.Sprintf("LIMIT %d", limit)
		query = fmt.Sprintf("%s %s", query, limit_str)
	}

	log.Printf("Query: %s\nArgs: %v", query, args)

}

// Example usage
func TestGet(t *testing.T) {
	log.Printf("test1 ")
	Get("name, age", "users", "id IN", "1,2,3", 10)

	log.Printf("test2 ")
	Get("", "users", "", "", 0)

	log.Printf("test3 ")
	Get("name, age", "users", "id", "", 0)

	log.Printf("test4 ")
	Get("name, age", "users", "", "1,2,3", 0)

	log.Printf("test5 ")
	Get("name, age", "users", "id IN", "1,2,3", 0)

	log.Printf("test6")
	Get("*", "jobs", "rid IN", "10, 24,    65", 5)

	log.Printf("test7")
	Get("*", "jobs", "name LIKE", "Johanesburg", 5)
}

// Output:
// Query: SELECT name, age FROM users WHERE id (?,?)
// Args: [1 2]
// Query: SELECT * FROM users
// Args: []
// Query: SELECT name, age FROM users
// Args: []
// Query: SELECT name, age FROM users
// Args: []
