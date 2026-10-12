package codegen

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const stateActivityModel = `package test {
	private import ScalarValues::*;
	private import StateActivity::*;
	state def M {
		entry; then a;
		state a;
	}
	state m : M;
	calc def Reads { in x : Boolean; return r : Boolean = x and m.a.isActive; }
	calc def Plain { in x : Boolean; return r : Boolean = x and true; }
}
`

// A calc reading a state's activity is refused with a typed error naming the
// read: only the state machine running the state answers it, so no compiled
// program can.
func TestStateActivityReadIsNotCompilable(t *testing.T) {
	const path = "state_activity.sysml"
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, []byte(stateActivityModel))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	pkg, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	compiler := New(model, resolver)
	for _, target := range []Target{TargetGo, TargetC} {
		t.Run(string(target), func(t *testing.T) {
			plain, _ := pkg.Scope.LookupLocal("Plain")
			if _, err := compiler.Compile(plain, target); err != nil {
				t.Errorf("Plain: %v; want it compiled", err)
			}
			reads, _ := pkg.Scope.LookupLocal("Reads")
			_, err := compiler.Compile(reads, target)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) || !errors.Is(err, ErrUnsupported) ||
				!strings.Contains(err.Error(), "m.a.isActive: a read of a state's activity (StateActivity::isActive)") {
				t.Errorf("Reads: %v; want a typed refusal naming the isActive read", err)
			}
		})
	}
}
