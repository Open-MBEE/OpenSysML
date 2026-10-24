package runtime

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The footprint of an expression a run re-reads on every pass — a loop's
// condition, a branch's guard — is computed once per model: the second request
// comes from the memo, allocates nothing, and is what the lowering computes.
func TestExpressionFootprintIsMemoizedPerModel(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		private import ScalarValues::*;
		part def P {
			attribute a : Integer = 1;
			attribute b : Integer = 2;
			attribute sum : Integer = a + b;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := typedModel(semantics.NewModel(resolver), resolver)
	sum := idx.LookupQualified("test::P::sum")[0]
	scope := idx.LookupQualified("test::P")[0].Scope
	expr := sum.Decl.(*ast.Usage).Value

	want := lower.ExpressionFootprint(resolver, scope, expr)
	if len(want.Reads) != 2 {
		t.Fatalf("the footprint of a + b reads %d places, want 2", len(want.Reads))
	}
	got := model.expressionFootprint(scope, expr)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expressionFootprint = %+v, want %+v", got, want)
	}
	var again lower.Footprint
	if allocs := testing.AllocsPerRun(50, func() {
		again = model.expressionFootprint(scope, expr)
	}); allocs != 0 {
		t.Errorf("a repeated expressionFootprint allocates %.0f objects, want 0", allocs)
	}
	if !reflect.DeepEqual(again, want) {
		t.Errorf("expressionFootprint again = %+v, want %+v", again, want)
	}
}
