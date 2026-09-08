package semantics

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Kernel Function Library collection functions whose result the arguments specialize.
const (
	fqnCollect   = "ControlFunctions::collect"
	fqnSelect    = "ControlFunctions::select"
	fqnReject    = "ControlFunctions::reject"
	fqnSelectOne = "ControlFunctions::selectOne"
	fqnReduce    = "ControlFunctions::reduce"
)

// collectionResultTypes is the types a call of a ControlFunctions collection function results in:
// collect and reduce those of the body applied, select/reject/selectOne those of the collection.
func (m *Model) collectionResultTypes(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol) []*symbols.Symbol {
	if fn == nil || e == nil {
		return nil
	}
	var types []*symbols.Symbol
	switch fn {
	case m.libSymbol(fqnCollect), m.libSymbol(fqnReduce):
		types = m.appliedResultTypes(scope, m.argumentTo(scope, e, fn, 1))
	case m.libSymbol(fqnSelect), m.libSymbol(fqnReject), m.libSymbol(fqnSelectOne):
		if collection := m.argumentTo(scope, e, fn, 0); collection != nil {
			types = m.resultTypes(scope, collection)
		}
	}
	return informativeTypes(types)
}

// collectResultTypes is the types `xs.{ in x; … }` results in: its body's; empty when unknown.
func (m *Model) collectResultTypes(scope *symbols.Scope, e *ast.CollectExpr) []*symbols.Symbol {
	body, ok := e.Body.(*ast.BodyExpr)
	if !ok {
		return nil
	}
	return informativeTypes(m.bodyResultTypes(scope, body))
}

// argumentTo is the expression e passes to the i-th input parameter of fn: the receiver of
// `xs->fn(…)` binds the first, then positional arguments in order, then named ones by name.
func (m *Model) argumentTo(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol, i int) ast.Node {
	positional := make([]ast.Node, 0, len(e.Args)+1)
	if e.Operand != nil {
		positional = append(positional, e.Operand)
	}
	positional = append(positional, e.Args...)
	if i < len(positional) {
		return positional[i]
	}
	if len(e.NamedArgs) == 0 {
		return nil
	}
	sig := m.signatureOf(fn)
	for _, arg := range e.NamedArgs {
		if arg.Name != nil && m.parameterIndex(scope, sig, arg.Name) == i {
			return arg.Value
		}
	}
	return nil
}

// appliedResultTypes is the types the expression applied to each element results in: a body
// `{ in x; … }` by its result, a function or expression named by its result parameter.
func (m *Model) appliedResultTypes(scope *symbols.Scope, applied ast.Node) []*symbols.Symbol {
	switch n := applied.(type) {
	case nil:
		return nil
	case *ast.BodyExpr:
		return m.bodyResultTypes(scope, n)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return nil
		}
		if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = alias
		}
		if result := m.ResultParameterOf(sym); result != nil {
			return m.featureResultTypes(result)
		}
	}
	return nil
}

// bodyResultTypes is the types the result of a body `{ in x; … }` is declared with, resolved
// among its parameters; a sequence result contributes each element, so every element type.
func (m *Model) bodyResultTypes(scope *symbols.Scope, body *ast.BodyExpr) []*symbols.Symbol {
	if body == nil || body.Result == nil || m.resolver == nil {
		return nil
	}
	return m.elementResultTypes(symbols.BodyExprScope(scope, body), body.Result)
}

// elementResultTypes is resultTypes over the elements a value contributes: those of a sequence
// `(a, b)`, nested ones flattened, else the expression itself; empty when any is unknown.
func (m *Model) elementResultTypes(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	seq, ok := node.(*ast.SequenceExpr)
	if !ok {
		return m.resultTypes(scope, node)
	}
	var out []*symbols.Symbol
	for _, element := range seq.Elements {
		types := m.elementResultTypes(scope, element)
		if len(types) == 0 {
			return nil
		}
		for _, typ := range types {
			if !containsElement(out, typ) {
				out = append(out, typ)
			}
		}
	}
	return out
}

// informativeTypes is types unless every one is Anything, which says nothing; then nil.
func informativeTypes(types []*symbols.Symbol) []*symbols.Symbol {
	for _, typ := range types {
		if !isAnything(typ) {
			return types
		}
	}
	return nil
}

// typesConformance judges a value typed by every one of types: it conforms when one does.
func (m *Model) typesConformance(types []*symbols.Symbol, want *symbols.Symbol) Conformance {
	if len(types) == 0 {
		return conformanceUnknown()
	}
	names := make([]string, 0, len(types))
	for _, typ := range types {
		if m.Conforms(typ, want) {
			return Conformance{Known: true, Holds: true}
		}
		names = append(names, leafName(typ.Name))
	}
	return Conformance{Known: true, Found: strings.Join(names, " and ")}
}
