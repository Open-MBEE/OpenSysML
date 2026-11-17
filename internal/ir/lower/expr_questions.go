package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// UnitTerm reads the expression as a unit reduced to the base units it is made of, its names read in scope.
func (e *ExprEntry) UnitTerm(m *semantics.Model, scope *symbols.Scope) (semantics.UnitTerm, error) {
	var written ast.Node
	if e != nil {
		written = e.Tree
	}
	return m.UnitTermOfExpr(scope, written)
}

// Written names the syntax the expression was lowered from, for a message refusing a
// form the expression records carry no kind for.
func (e *ExprEntry) Written() string {
	if e == nil {
		return fmt.Sprintf("%T", nil)
	}
	return fmt.Sprintf("%T", e.Tree)
}

// Reference reports whether the expression is a name read as a feature reference or a
// chain of them, rather than a bare qualified name standing where an expression does.
func (e *ExprEntry) Reference() bool {
	if e == nil {
		return false
	}
	switch e.Tree.(type) {
	case *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	}
	return false
}

// Segments are the names a reference or a feature chain steps through, false for
// any other expression or one naming nothing.
func (e *ExprEntry) Segments() ([]string, bool) {
	if e == nil {
		return nil, false
	}
	switch e.Kind {
	case program.ExprRef:
		return e.spelling()
	case program.ExprChain:
		if len(e.Operands) != 1 || len(e.Chain) == 0 {
			return nil, false
		}
		base, ok := e.Operands[0].Segments()
		if !ok {
			return nil, false
		}
		return append(base, names(e.Chain)...), true
	}
	return nil, false
}

// Invoked are the names an invocation's function is written with, false for any
// other expression or one naming nothing; a chain never steps through them.
func (e *ExprEntry) Invoked() ([]string, bool) {
	if e == nil || e.Kind != program.ExprInvoke {
		return nil, false
	}
	return e.spelling()
}

// spelling is the segments of the name e is written with, false when it names nothing.
// The empty unrestricted name is a name: only a reference written nowhere names nothing.
func (e *ExprEntry) spelling() ([]string, bool) {
	if len(e.Path) > 0 {
		return names(e.Path), true
	}
	if e.Ref.Name == "" && e.Ref.Span == (source.Span{}) {
		return nil, false
	}
	return []string{e.Ref.Name}, true
}

// names are the names refs spell, in order.
func names(refs []program.SymbolRef) []string {
	out := make([]string, len(refs))
	for i := range refs {
		out[i] = refs[i].Name
	}
	return out
}

// Reads visits each name and whole feature chain the expression reads a value through, a quantity's unit
// aside, with the names it steps through and false for one naming nothing; it stops where visit answers false.
func (e *ExprEntry) Reads(visit func(segments []string, chain, ok bool) bool) {
	if e == nil || e.Tree == nil {
		return
	}
	going := true
	var walk func(n ast.Node) bool
	walk = func(n ast.Node) bool {
		if !going {
			return false
		}
		switch ref := n.(type) {
		case *ast.IndexExpr:
			if ref.Bracket {
				ast.Inspect(ref.Operand, walk)
				return false
			}
		case *ast.FeatureChainExpr:
			segments, ok := writtenSegments(ref)
			going = visit(segments, true, ok)
			return false
		case *ast.FeatureReference:
			segments, ok := writtenSegments(ref)
			going = visit(segments, false, ok)
			return false
		}
		return going
	}
	ast.Inspect(e.Tree, walk)
}

// writtenSegments are the names the reference or chain n steps through, false for any other syntax.
func writtenSegments(n ast.Node) ([]string, bool) {
	var qn *ast.QualifiedName
	var base []string
	switch n := n.(type) {
	case *ast.FeatureReference:
		qn = n.Name
	case *ast.QualifiedName:
		qn = n
	case *ast.FeatureChainExpr:
		var ok bool
		if base, ok = writtenSegments(n.Operand); !ok {
			return nil, false
		}
		qn = n.Member
	default:
		return nil, false
	}
	if qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	for _, part := range qn.Parts {
		base = append(base, part.Text)
	}
	return base, true
}

// Declared is the declaration of sym standing where an expression is read: where a
// message about it points, and what the dimension its type fixes is read from.
func Declared(sym *symbols.Symbol) *ExprEntry {
	if sym == nil || sym.Decl == nil {
		return nil
	}
	entry := &ExprEntry{Tree: sym.Decl, Scope: sym.OwnerScope}
	entry.Origin = program.Origin{Doc: sym.DocName, Span: sym.Decl.Span()}
	return entry
}
