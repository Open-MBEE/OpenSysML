package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// unitOf is the unit operand of the quantity `x [unit]` that is the value of
// the attribute T::name, with the scope the value is written in.
func unitOf(t *testing.T, idx *symbols.Index, name string) (*symbols.Scope, ast.Node) {
	t.Helper()
	sym := dimensionSymbol(t, idx, "T::"+name)
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("%s is declared by %T, want *ast.Usage", name, sym.Decl)
	}
	quantity, ok := usage.Value.(*ast.IndexExpr)
	if !ok || !quantity.Bracket {
		t.Fatalf("the value of %s is %T, want a bracket quantity", name, usage.Value)
	}
	return sym.OwnerScope, quantity.Index
}

// TestUnitOperandConformanceIsOrderIndependent: an operand whose type is not
// known neither hides a sibling that is a unit nor is hidden by one that is not.
func TestUnitOperandConformanceIsOrderIndependent(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(`package T {
		private import SI::*;
		attribute unitFirst = 10 [m * nope];
		attribute unitLast = 10 [nope * m];
		attribute scalarFirst = 10 [3 * nope];
		attribute scalarLast = 10 [nope * 3];
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	m := semantics.NewModel(resolve.New(idx))
	for _, name := range []string{"unitFirst", "unitLast"} {
		scope, unit := unitOf(t, idx, name)
		if c := m.UnitOperandConformance(scope, unit); !c.Known || !c.Holds {
			t.Errorf("%s: = %+v, want it to hold", name, c)
		}
	}
	for _, name := range []string{"scalarFirst", "scalarLast"} {
		scope, unit := unitOf(t, idx, name)
		if c := m.UnitOperandConformance(scope, unit); c.Known {
			t.Errorf("%s: = %+v, want it unknown", name, c)
		}
	}
}
