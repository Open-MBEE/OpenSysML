package symbols

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An if branch's scope states the imports its body writes, each branch its own.
func TestIfBranchScopeStatesItsImports(t *testing.T) {
	idx := buildIndex(t, map[string]string{"a.sysml": `package Q { part def T; }
package R { part def U; }
package P {
    action def A {
        attribute n : ScalarValues::Integer;
        if n < 3 { private import Q::*; part v : T; } else { private import R::U; part u : U; }
    }
}`})
	p, ok := idx.DocumentRoot("a.sysml").LookupLocal("P")
	if !ok {
		t.Fatal("P not found")
	}
	a, ok := p.Scope.LookupLocal("A")
	if !ok {
		t.Fatal("P::A not found")
	}
	var got []string
	for _, child := range a.Scope.Children() {
		if _, isBranch := child.Node().(*ast.IfBranchNode); !isBranch {
			continue
		}
		imports := child.Imports()
		if len(imports) != 1 {
			t.Fatalf("branch states %d imports, want 1", len(imports))
		}
		var parts []string
		for _, part := range imports[0].Imported.Parts {
			parts = append(parts, part.Text)
		}
		got = append(got, strings.Join(parts, "::"))
	}
	if len(got) != 2 || got[0] != "Q" || got[1] != "R::U" {
		t.Errorf("branch imports = %v, want [Q R::U]", got)
	}
}
