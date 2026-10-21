package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// importedMemberAllocs is what enumerating one more member a wildcard import
// brings, re-exported through another namespace's wildcard import, costs in
// allocations: the difference between enumerations of 2n and of n members, per member.
func importedMemberAllocs(t *testing.T, n int) float64 {
	t.Helper()
	enumerate := func(members int) float64 {
		var src strings.Builder
		src.WriteString("package L {\n")
		for i := 0; i < members; i++ {
			fmt.Fprintf(&src, "\tpart def M%d;\n", i)
		}
		src.WriteString("}\npackage Q { public import L::*; }\npackage R { public import Q::*; }\n")
		p := parser.New(source.New("d.sysml", []byte(src.String())))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx := symbols.NewIndexFromDoc("d.sysml", root)
		idx.ExpandWildcardImports()
		r := New(idx)
		r.ResolveDocument("d.sysml", root)
		scope := idx.LookupQualified("R")[0].Scope
		imp := r.scopeImports(scope)[0]
		if got := len(r.importedMembersInto(scope, scope, imp, false)); got != members {
			t.Fatalf("the import brings %d members, want %d", got, members)
		}
		return testing.AllocsPerRun(20, func() {
			r.importedMembersInto(scope, scope, imp, false)
		})
	}
	return (enumerate(2*n) - enumerate(n)) / float64(n)
}

// Enumerating what a wildcard import brings costs each member its place in
// the list, not a walk of the routes that re-exported it: a member no
// namespace hides is admitted without building its route conditions, its
// qualified name or a candidate list of one, so it costs at most the bound.
func TestImportedMemberEnumerationDoesNotWalkRoutesPerMember(t *testing.T) {
	const bound = 3
	got := importedMemberAllocs(t, 64)
	t.Logf("one more imported member costs %.2f allocations", got)
	if got > bound {
		t.Errorf("one more imported member costs %.2f allocations, want at most %d", got, bound)
	}
}
