package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Folded is the constant the expression folds to under an Integer budget of maxBits,
// false for one only evaluation settles: the fold the semantic model makes of its syntax.
func (e *ExprEntry) Folded(m *semantics.Model, maxBits int64) (semantics.Value, bool) {
	if e == nil {
		return semantics.Value{}, false
	}
	return m.EvalWithin(e.Tree, maxBits)
}

// Dimension is the quantity dimension the declarations the expression reads in scope fix
// for its value, false where they fix none.
func (e *ExprEntry) Dimension(m *semantics.Model, scope *symbols.Scope) (semantics.Dimension, bool) {
	if e == nil {
		return semantics.Dimension{}, false
	}
	return m.DimensionOfExpr(scope, e.Tree)
}

// Held is how many values the expression holds wherever it is evaluated in scope, as far
// as the declarations it reads fix that.
func (e *ExprEntry) Held(m *semantics.Model, scope *symbols.Scope) (semantics.Range, bool) {
	if e == nil {
		return semantics.Range{}, false
	}
	return m.ValuesHeldBy(scope, e.Tree)
}

// Classifies is the verdict of the classification `@T`/`@@T` the expression writes on
// elem, its type read in scope: the one a filter condition writing the same test reaches.
func (e *ExprEntry) Classifies(m *semantics.Model, scope *symbols.Scope, elem *symbols.Symbol) (bool, error) {
	var written *ast.OperatorExpr
	if e != nil {
		written, _ = e.Tree.(*ast.OperatorExpr)
	}
	return m.EvalClassification(scope, written, elem)
}

// Unit reads the expression as the unit a quantity carries, its names read in scope.
func (e *ExprEntry) Unit(m *semantics.Model, scope *symbols.Scope) (semantics.Unit, error) {
	var written ast.Node
	if e != nil {
		written = e.Tree
	}
	return m.UnitOfExpr(scope, written)
}
