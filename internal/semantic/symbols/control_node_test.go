package symbols

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// actionBodyScope returns the scope of the sole action def in src.
func actionBodyScope(t *testing.T, src string) *Scope {
	t.Helper()
	file := parser.New(source.New("test", []byte(src))).ParseFile()
	idx := NewIndex()
	idx.AddDocument("test", file)
	root := idx.DocumentRoot("test")
	if root == nil || len(root.Children()) == 0 {
		t.Fatal("no document root")
	}
	pkg := root.Children()[0]
	if len(pkg.Children()) == 0 {
		t.Fatal("no action def scope")
	}
	return pkg.Children()[0]
}

// TestControlNodeSymbols covers the four control nodes: a named one is a member
// of the action body, recorded as an action usage at its name's span.
func TestControlNodeSymbols(t *testing.T) {
	src := `package P {
	action def F {
		fork Jump;
		join Land;
		merge M;
		decide D;
	}
}`
	scope := actionBodyScope(t, src)
	for _, name := range []string{"Jump", "Land", "M", "D"} {
		sym, _ := scope.LookupLocal(name)
		if sym == nil {
			t.Errorf("%s not registered", name)
			continue
		}
		if sym.Kind != SymbolActionUsage {
			t.Errorf("%s has kind %v, want %v", name, sym.Kind, SymbolActionUsage)
		}
		if want := strings.Index(src, name+";"); sym.NameSpan.Offset != want {
			t.Errorf("%s NameSpan at %d, want %d", name, sym.NameSpan.Offset, want)
		}
	}
}

// TestUnnamedControlNodesRegisterNothing covers unnamed control nodes: they
// declare no name, so the action body gains no member for them.
func TestUnnamedControlNodesRegisterNothing(t *testing.T) {
	src := `package P {
	action def F {
		fork;
		join;
		merge;
		decide;
	}
}`
	if names := actionBodyScope(t, src).MemberNames(); len(names) != 0 {
		t.Errorf("unnamed control nodes registered %v", names)
	}
}

// TestSuccessionBodyScope covers the body an action target succession carries:
// its declarations are members of the anonymous succession usage, not of the action.
func TestSuccessionBodyScope(t *testing.T) {
	scope := actionBodyScope(t, `package P {
	action def F {
		action prep;
		action starting;
		first prep;
		then starting;
		then starting { attribute wait; }
	}
}`)
	if _, ok := scope.LookupLocal("wait"); ok {
		t.Error("wait is a member of the action body, want a member of the succession body only")
	}
	var bodies []*Scope
	for _, child := range scope.Children() {
		if _, ok := child.Node().(*ast.SuccessionEdge); ok {
			bodies = append(bodies, child)
		}
	}
	if len(bodies) != 1 {
		t.Fatalf("succession body scopes = %d, want 1 (a bodiless succession owns none)", len(bodies))
	}
	body := bodies[0]
	if owner := body.Owner(); owner == nil || owner.Kind != SymbolSuccessionUsage || owner.OwnerScope != scope {
		t.Errorf("succession body scope owner = %v, want an anonymous succession usage of the action", owner)
	}
	sym, ok := body.LookupLocal("wait")
	if !ok || sym.OwnerScope != body || sym.Kind != SymbolAttributeUsage {
		t.Errorf("wait in succession body = %v, %v; want an attribute usage owned by the body", sym, ok)
	}
}
