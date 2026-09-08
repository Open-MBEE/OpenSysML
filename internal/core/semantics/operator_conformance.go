package semantics

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CastConformance judges `x as T`: sound when a type of x and T specialize one
// another in either direction (KerML validateOperatorExpressionCastConformance).
func (m *Model) CastConformance(scope *symbols.Scope, e *ast.OperatorExpr) Conformance {
	if m == nil || m.resolver == nil || e == nil || e.Operator != ast.OpAs || len(e.Operands) != 1 {
		return conformanceUnknown()
	}
	target := m.namedType(scope, e.TypeRef)
	if target == nil {
		return conformanceUnknown()
	}
	types := m.resultTypes(scope, e.Operands[0])
	if len(types) == 0 {
		return conformanceUnknown()
	}
	for _, typ := range types {
		if m.Classifies(target, typ) || m.Conforms(target, typ) {
			return Conformance{Known: true, Holds: true}
		}
	}
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, leafName(typ.Name))
	}
	return Conformance{Known: true, Found: strings.Join(names, " and ")}
}

// ExprResultTypes is every type an expression's result is declared with, where
// ExprResultType picks one; empty when not statically known.
func (m *Model) ExprResultTypes(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	if m == nil || m.resolver == nil || node == nil {
		return nil
	}
	return m.resultTypes(scope, node)
}

func (m *Model) resultTypes(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	switch n := node.(type) {
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return nil
		}
		return m.featureResultTypes(sym)
	case *ast.InvocationExpr:
		if result := m.invocationResult(scope, n); result != nil {
			return m.featureResultTypes(result)
		}
		return nil
	case *ast.IndexExpr:
		if !n.Bracket {
			return m.indexResultTypes(scope, n)
		}
	case *ast.SelectExpr:
		return m.resultTypes(scope, n.Operand)
	}
	if typ := m.ExprResultType(scope, node); typ != nil {
		return []*symbols.Symbol{typ}
	}
	return nil
}

// indexResultTypes is indexResultType over every type of seq: each Collection
// among them selects an Anything.
func (m *Model) indexResultTypes(scope *symbols.Scope, n *ast.IndexExpr) []*symbols.Symbol {
	seq := m.resultTypes(scope, n.Operand)
	anything, collection := m.libSymbol(fqnAnything), m.libSymbol(fqnCollection)
	var out []*symbols.Symbol
	for _, typ := range seq {
		if collection != nil && m.Conforms(typ, collection) {
			typ = anything
		}
		if typ != nil && !containsElement(out, typ) {
			out = append(out, typ)
		}
	}
	if len(out) == 0 && anything != nil {
		return []*symbols.Symbol{anything}
	}
	return out
}

// featureResultTypes is the types a feature declares or inherits, else those
// its value gives it. A calculation usage named as a value is the calculation,
// not its result (the pilot warns on `calc n : Name; n as String`).
func (m *Model) featureResultTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	if !sym.IsFeature() || !m.generalizationsResolve(sym) {
		return nil
	}
	if defs := m.declaredTypes(sym, map[*symbols.Symbol]bool{}); len(defs) > 0 {
		return defs
	}
	if value := m.typingValue(sym); value != nil && !m.valuing[sym] {
		m.valuing[sym] = true
		defer delete(m.valuing, sym)
		return m.resultTypes(sym.OwnerScope, value)
	}
	if typ := m.featureResultType(sym); typ != nil {
		return []*symbols.Symbol{typ}
	}
	return nil
}

// UnitOperandConformance judges the unit of `x [unit]`, a TensorMeasurementReference
// (SysML validateOperatorExpressionQuantity).
func (m *Model) UnitOperandConformance(scope *symbols.Scope, unit ast.Node) Conformance {
	if m == nil || m.resolver == nil || unit == nil {
		return conformanceUnknown()
	}
	reference := m.libSymbol(fqnTensorMeasurementReference)
	if reference == nil {
		return conformanceUnknown()
	}
	return m.resultConformance(scope, unit, reference)
}

// resultConformance is ExprConformsTo, judging an operator or sequence whose
// declared result is wider than want by its operands (`m * 2` declares a DataValue).
func (m *Model) resultConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol) Conformance {
	var operands []ast.Node
	switch n := node.(type) {
	case *ast.OperatorExpr:
		operands = valueOperands(n)
	case *ast.SequenceExpr:
		operands = n.Elements
	case *ast.IndexExpr:
		// `seq#(i)` is one element of seq, so seq decides.
		if n.Bracket {
			return m.ExprConformsTo(scope, node, want)
		}
		operands = []ast.Node{n.Operand}
	default:
		return m.ExprConformsTo(scope, node, want)
	}
	results := m.resultTypes(scope, node)
	if len(results) == 0 {
		return conformanceUnknown()
	}
	wider, names := false, make([]string, 0, len(results))
	for _, result := range results {
		if m.Conforms(result, want) {
			return Conformance{Known: true, Holds: true}
		}
		wider = wider || m.Conforms(want, result)
		names = append(names, leafName(result.Name))
	}
	found := Conformance{Known: true, Found: "the result of `" + operatorText(node) + "`, typed by " + strings.Join(names, " and ")}
	if !wider {
		return found
	}
	unknown := false
	for _, operand := range operands {
		c := m.resultConformance(scope, operand, want)
		if c.Known && c.Holds {
			return c
		}
		unknown = unknown || !c.Known
	}
	if unknown {
		return conformanceUnknown()
	}
	return found
}

// valueOperands is the operands passed to a function by value: the body of an
// `expr` parameter (a conditional's branches, the fallback of `??`) is not one.
func valueOperands(e *ast.OperatorExpr) []ast.Node {
	switch e.Operator {
	case ast.OpConditional:
		return nil
	case ast.OpNullCoalesce:
		return e.Operands[:1]
	}
	return e.Operands
}

// operatorText spells the operator an expression applies.
func operatorText(node ast.Node) string {
	switch e := node.(type) {
	case *ast.OperatorExpr:
		return e.Operator.String()
	case *ast.IndexExpr:
		return "#"
	}
	return ","
}
