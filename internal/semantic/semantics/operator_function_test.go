package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// No index expression, no model: no function, rather than a nil dereference.
func TestIndexFunctionOfNothing(t *testing.T) {
	if fn := (&Model{}).IndexFunction(nil); fn != nil {
		t.Errorf("IndexFunction(nil) = %v, want nil", fn)
	}
	var none *Model
	if fn := none.IndexFunction(nil); fn != nil {
		t.Errorf("nil model's IndexFunction(nil) = %v, want nil", fn)
	}
	if fn := none.IndexFunction(&ast.IndexExpr{Bracket: true}); fn != nil {
		t.Errorf("nil model's IndexFunction = %v, want nil", fn)
	}
}
