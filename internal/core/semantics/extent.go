package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// IsExtentExpr reports whether node is an extent expression `all T` (KerML 1.0 §7.4.9.2).
func IsExtentExpr(node ast.Node) bool {
	n, ok := node.(*ast.OperatorExpr)
	return ok && n.Operator == ast.OpAll
}

// ExtentTypeName is the name an extent expression `all T` names its type by: its one
// operand, which the grammar makes a type reference, not a value. Nil for any other node.
func ExtentTypeName(node ast.Node) *ast.QualifiedName {
	if !IsExtentExpr(node) {
		return nil
	}
	n := node.(*ast.OperatorExpr)
	if len(n.Operands) != 1 {
		return nil
	}
	switch operand := n.Operands[0].(type) {
	case *ast.QualifiedName:
		return operand
	case *ast.FeatureReference:
		return operand.Name
	}
	return nil
}

// ExtentOperand is the element an extent expression `all T` names, an alias followed,
// whatever it is; false where node is no extent expression or the name resolves to nothing.
func (m *Model) ExtentOperand(scope *symbols.Scope, node ast.Node) (*symbols.Symbol, bool) {
	qn := ExtentTypeName(node)
	if m == nil || m.resolver == nil || qn == nil {
		return nil, false
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil, false
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok && alias != nil {
		sym = alias
	}
	return sym, true
}

// ExtentType is the type an extent expression `all T` names. Nil where the name
// resolves to nothing or to an element that is no type, such as a package.
func (m *Model) ExtentType(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	sym, ok := m.ExtentOperand(scope, node)
	if !ok || !IsType(sym) {
		return nil
	}
	return sym
}

// IsType reports whether sym declares a KerML Type, which alone has an extent: a
// classifier or a feature (KerML 1.0 §8.3.3), never a package, relationship or comment.
func IsType(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind.IsDefinition() || sym.IsFeature())
}

// extentTypes are the types every element of `all T` is of; nil for any other node.
func (m *Model) extentTypes(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	sym := m.ExtentType(scope, node)
	if sym == nil {
		return nil
	}
	return m.instanceTypes(sym)
}

// instanceTypes are the types every instance of sym is of: sym for a classifier; for a
// feature the types it is typed by, since its values are instances of them (KerML 1.0 §7.3.4.1).
func (m *Model) instanceTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.IsFeature() {
		return m.featureResultTypes(sym)
	}
	return []*symbols.Symbol{sym}
}

// ExtentRange is how many elements an extent holds: any number, as
// BaseFunctions::'all' declares its result `Object[0..*]`.
func ExtentRange() Range {
	return Range{
		Lower: Bound{Value: 0, Known: true},
		Upper: Bound{Infinite: true, Known: true},
	}
}
