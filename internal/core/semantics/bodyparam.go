package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Kernel Function Library functions applying a body to each element of a collection,
// beside those collectionResultTypes specializes the result of.
const (
	fqnForAll   = "ControlFunctions::forAll"
	fqnExists   = "ControlFunctions::exists"
	fqnMinimize = "ControlFunctions::minimize"
	fqnMaximize = "ControlFunctions::maximize"
)

// bodyApplication is the operation a body expression is the argument of
// (`xs.{…}`, `xs.?{…}`, `xs->f {…}`) and the scope that operation resolves in.
type bodyApplication struct {
	scope *symbols.Scope
	op    ast.Node
}

// assumedSupertypes answers a symbol's supertypes under the resolver frame at depth,
// cutting every query it answers short so none is memoized.
type assumedSupertypes struct {
	types []*symbols.Symbol
	depth int
}

// BodyParameterElementTypes is the element types of the collection a collection function
// binds an untyped body parameter to (KerML 8.3.4.8); nothing where those cannot be typed,
// nor for a reducer's first parameter unless the reducer's result feeds back as an element.
func (m *Model) BodyParameterElementTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || m.resolver == nil || sym == nil || sym.OwnerScope == nil {
		return nil
	}
	body, ok := sym.Decl.(*ast.BodyExpr)
	if !ok {
		return nil
	}
	position := bodyParameterPosition(body, sym.Name)
	if position < 0 {
		return nil
	}
	app, ok := m.bodyApplicationOf(sym.OwnerScope, body)
	if !ok {
		return nil
	}
	collection, bound, ok := m.appliedOver(app.scope, app.op, body)
	if !ok || position >= bound || collection == nil {
		return nil
	}
	elements := m.sourcesTypes([]collectionSource{{scope: app.scope, node: collection}})
	if len(elements) == 0 || position == 0 && bound == 2 && !m.reducerFeedsBack(sym, app.scope, body, elements) {
		return nil
	}
	return elements
}

// reducerFeedsBack reports whether the reducer's result, typed with its first parameter
// assumed to hold the elements, conforms to every element type: the fold's fixed point.
func (m *Model) reducerFeedsBack(first *symbols.Symbol, scope *symbols.Scope, body *ast.BodyExpr, elements []*symbols.Symbol) bool {
	if body.Result == nil {
		return false
	}
	if _, assuming := m.assumedSupers[first]; assuming {
		return false
	}
	m.assumedSupers[first] = assumedSupertypes{types: elements, depth: m.resolver.Enter()}
	results := m.resultTypes(symbols.BodyExprScope(scope, body), body.Result)
	delete(m.assumedSupers, first)
	m.resolver.Leave()
	for _, element := range elements {
		if !m.anyConforms(results, element) {
			return false
		}
	}
	return true
}

// bodyParameterPosition is the position of body's parameter named name, or -1 where it
// declares a type or generalization of its own.
func bodyParameterPosition(body *ast.BodyExpr, name string) int {
	for i := range body.Params {
		p := &body.Params[i]
		if p.Name != name {
			continue
		}
		if p.Type != nil {
			return -1
		}
		for _, rel := range p.Relationships {
			if rel != nil && GeneralizationKind(rel.Kind) {
				return -1
			}
		}
		return i
	}
	return -1
}

// appliedOver is the collection op binds body's leading parameters to the elements of, and
// how many it binds: one, or two for reduce. Not ok where op applies no body over a collection.
func (m *Model) appliedOver(scope *symbols.Scope, op ast.Node, body *ast.BodyExpr) (ast.Node, int, bool) {
	switch n := op.(type) {
	case *ast.CollectExpr:
		if n.Body == body {
			return n.Operand, 1, true
		}
	case *ast.SelectExpr:
		if n.Body == body {
			return n.Operand, 1, true
		}
	case *ast.InvocationExpr:
		fn := m.invocationCallee(scope, n)
		if fn == nil || !m.appliesOverElements(fn) || m.argumentTo(scope, n, fn, 1) != body {
			return nil, 0, false
		}
		bound := 1
		if fn == m.libSymbol(fqnReduce) {
			bound = 2
		}
		return m.argumentTo(scope, n, fn, 0), bound, true
	}
	return nil, 0, false
}

// appliesOverElements reports a library function applying its second parameter to each
// element of its first.
func (m *Model) appliesOverElements(fn *symbols.Symbol) bool {
	for _, fqn := range []string{fqnCollect, fqnSelect, fqnReject, fqnSelectOne, fqnReduce, fqnForAll, fqnExists, fqnMinimize, fqnMaximize} {
		if fn == m.libSymbol(fqn) {
			return true
		}
	}
	return false
}

// bodyApplicationOf is the operation body is the argument of, indexing the document holding
// bodyScope on first query: no scope records the expression a body sits in.
func (m *Model) bodyApplicationOf(bodyScope *symbols.Scope, body *ast.BodyExpr) (bodyApplication, bool) {
	if app, ok := m.bodyApplications[body]; ok {
		return app, true
	}
	root := bodyScope
	for root.Parent() != nil {
		root = root.Parent()
	}
	if m.bodyIndexed[root] {
		return bodyApplication{}, false
	}
	m.bodyIndexed[root] = true
	doc, ok := root.Node().(*ast.RootNamespace)
	if !ok {
		return bodyApplication{}, false
	}
	var w symbols.ExprWalker
	w = symbols.ExprWalker{
		Body:    symbols.BodyExprScope,
		Members: func(scope *symbols.Scope, members []ast.Node) { w.WalkMembers(scope, members) },
		Applied: func(scope *symbols.Scope, op ast.Node, applied *ast.BodyExpr) {
			m.bodyApplications[applied] = bodyApplication{scope: scope, op: op}
		},
	}
	w.WalkMembers(root, doc.Members)
	app, ok := m.bodyApplications[body]
	return app, ok
}
