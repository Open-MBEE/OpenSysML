package hygiene

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// scopeRow matches one row of the scopes table in docs/reference/sysml-mcp.md:
// the scope, what it grants, and the tools that require it.
var scopeRow = regexp.MustCompile("(?m)^\\| `(sysml:[a-z]+)` \\| [^|]+ \\| ([^|]+) \\|$")

// The scopes table of the reference lists under each scope exactly the
// catalog's tools of that scope, so the page cannot say a tool needs less or more.
func TestMCPReferenceScopesTableMatchesTheCatalog(t *testing.T) {
	page := readRepoFile(t, "docs/reference/sysml-mcp.md")
	documented := map[catalog.Scope][]string{}
	for _, m := range scopeRow.FindAllStringSubmatch(page, -1) {
		scope := catalog.Scope(m[1])
		if !scope.Known() {
			t.Errorf("the scopes table names %s, which the catalog does not know", scope)
		}
		for _, tool := range strings.Split(m[2], ",") {
			documented[scope] = append(documented[scope], strings.Trim(strings.TrimSpace(tool), "`"))
		}
	}
	for _, scope := range catalog.Scopes() {
		var want []string
		for _, e := range catalog.Tools {
			if e.Scope == scope {
				want = append(want, e.Name)
			}
		}
		slices.Sort(want)
		got := documented[scope]
		switch {
		case scope == catalog.ScopeAdmin:
			if got != nil {
				t.Errorf("the scopes table lists tools under %s, which gates no tool: %v", scope, got)
			}
		case !slices.Equal(got, want):
			t.Errorf("the scopes table lists under %s:\n%v\nthe catalog requires it of:\n%v", scope, got, want)
		}
	}
	if !strings.Contains(page, "`sysml:admin`") {
		t.Error("docs/reference/sysml-mcp.md does not explain sysml:admin")
	}
}
