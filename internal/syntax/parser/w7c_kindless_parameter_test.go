package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A declaration with no kind keyword is one thing wherever it is written
// (SysML.xtext DefaultReferenceUsage, SysML v2 §7.6), so a parameter reads the
// same kind as the same declaration outside a parameter list.
func TestKindlessParameterReadsSameKindAsKindlessMember(t *testing.T) {
	src := `action def A { in x : Real; out y : Real; attribute z : Real; w : Real; }`
	p := New(source.New("kindless.sysml", []byte(src)))
	f := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Fatalf("unexpected error: %s", d.Message)
	}

	dump := ast.Dump(f)
	for _, name := range []string{"x", "y", "w"} {
		want := `(Usage kind="attribute" name="` + name + `"`
		if !strings.Contains(dump, want) {
			t.Errorf("no %s in\n%s", want, dump)
		}
	}
}
