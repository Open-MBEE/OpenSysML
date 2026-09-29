package migrate_test

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// TestDocKeySortsAsTheDocumentationQuery checks the planner's documentation key
// orders elements as the query's documentation property orders the doc comments
// the migration writes, single-line and multiline alike.
func TestDocKeySortsAsTheDocumentationQuery(t *testing.T) {
	texts := []string{"A\nZ", "A", "B", "A  \n  b"}
	var src strings.Builder
	src.WriteString("package P {\n")
	for i, text := range texts {
		fmt.Fprintf(&src, "part p%d {\ndoc %s\n}\n", i, strings.Join(migrate.CommentLines(text), "\n"))
	}
	src.WriteString("}\n")
	s := repl.NewSession()
	s.Submit(src.String())
	lines, err := s.Query(`oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name&oslc.orderBy=+sysml:documentation`)
	if err != nil {
		t.Fatal(err)
	}
	var queried []string
	for _, line := range lines {
		queried = append(queried, line[strings.LastIndex(line, "=")+1:])
	}
	planned := make([]int, len(texts))
	for i := range planned {
		planned[i] = i
	}
	sort.SliceStable(planned, func(a, b int) bool {
		return migrate.DocKey(texts[planned[a]]) < migrate.DocKey(texts[planned[b]])
	})
	var want []string
	for _, i := range planned {
		want = append(want, fmt.Sprintf("p%d", i))
	}
	if !slices.Equal(queried, want) {
		t.Errorf("query orders %q, docKey orders %q", queried, want)
	}
}
