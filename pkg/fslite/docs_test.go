package fslite

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var routerAnnotation = regexp.MustCompile(`(?m)^//\s*@Router\s+(\S+)\s+\[(\w+)\]`)

// The API docs are generated from the @Router annotations (make api-docs):
// every annotation must name a registered route and every API route must be
// annotated (health checks and the docs themselves aside).
func TestAPIDocsMatchTheRoutes(t *testing.T) {
	_, engine := newTestServer(t)
	var registered []string
	for _, r := range engine.Routes() {
		if r.Path != "/health" && r.Path != "/readyz" && !strings.HasPrefix(r.Path, "/swagger") {
			registered = append(registered, r.Method+" "+r.Path)
		}
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var docs []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range routerAnnotation.FindAllStringSubmatch(string(src), -1) {
			docs = append(docs, strings.ToUpper(m[2])+" "+m[1])
		}
	}
	for _, d := range docs {
		if !slices.Contains(registered, d) {
			t.Errorf("documented, not routed: %s", d)
		}
	}
	for _, r := range registered {
		if !slices.Contains(docs, r) {
			t.Errorf("routed, not documented: %s", r)
		}
	}
}
