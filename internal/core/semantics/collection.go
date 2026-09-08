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

// collectionSource is what the elements of a collection function's result are: an
// expression in its scope — the result of the body applied, or the collection kept —
// or the result parameter of the function applied by name.
type collectionSource struct {
	scope  *symbols.Scope
	node   ast.Node
	result *symbols.Symbol
}

// invocationSource is the source of a call of a ControlFunctions collection function: collect
// and reduce the body or function applied, select/reject/selectOne the collection; not ok
// where the arguments say nothing and the declaration alone decides.
func (m *Model) invocationSource(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol) (collectionSource, bool) {
	if fn == nil || e == nil {
		return collectionSource{}, false
	}
	switch fn {
	case m.libSymbol(fqnCollect), m.libSymbol(fqnReduce):
		return m.appliedSource(scope, m.argumentTo(scope, e, fn, 1))
	case m.libSymbol(fqnSelect), m.libSymbol(fqnReject), m.libSymbol(fqnSelectOne):
		if collection := m.argumentTo(scope, e, fn, 0); collection != nil {
			return collectionSource{scope: scope, node: collection}, true
		}
	}
	return collectionSource{}, false
}

// collectSource is the source of `xs.{ in x; … }`: its body's result.
func (m *Model) collectSource(scope *symbols.Scope, e *ast.CollectExpr) (collectionSource, bool) {
	body, ok := e.Body.(*ast.BodyExpr)
	if !ok {
		return collectionSource{}, false
	}
	return m.bodySource(scope, body)
}

// appliedSource is the source of the expression applied to each element: a body
// `{ in x; … }` by its result, a function or expression named by its result parameter.
func (m *Model) appliedSource(scope *symbols.Scope, applied ast.Node) (collectionSource, bool) {
	switch n := applied.(type) {
	case *ast.BodyExpr:
		return m.bodySource(scope, n)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		if result := m.appliedResult(scope, n); result != nil {
			return collectionSource{result: result}, true
		}
	}
	return collectionSource{}, false
}

// bodySource is the result of a body `{ in x; … }`, resolved among its parameters.
func (m *Model) bodySource(scope *symbols.Scope, body *ast.BodyExpr) (collectionSource, bool) {
	if body == nil || body.Result == nil || m.resolver == nil {
		return collectionSource{}, false
	}
	return collectionSource{scope: symbols.BodyExprScope(scope, body), node: body.Result}, true
}

// appliedResult is the result parameter of the function or expression the name applied denotes.
func (m *Model) appliedResult(scope *symbols.Scope, applied ast.Node) *symbols.Symbol {
	if m.resolver == nil {
		return nil
	}
	sym, ok := m.resolver.ResolveTarget(scope, applied)
	if !ok || sym == nil {
		return nil
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	return m.ResultParameterOf(sym)
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

// collectionResultTypes is the types every element of a collection function call's result
// has; nil where they are unknown or say nothing.
func (m *Model) collectionResultTypes(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol) []*symbols.Symbol {
	src, ok := m.invocationSource(scope, e, fn)
	if !ok {
		return nil
	}
	return m.sourceTypes(src)
}

// collectResultTypes is the types every element of `xs.{ in x; … }` has; nil when unknown.
func (m *Model) collectResultTypes(scope *symbols.Scope, e *ast.CollectExpr) []*symbols.Symbol {
	src, ok := m.collectSource(scope, e)
	if !ok {
		return nil
	}
	return m.sourceTypes(src)
}

func (m *Model) sourceTypes(src collectionSource) []*symbols.Symbol {
	if src.result != nil {
		return informativeTypes(m.featureResultTypes(src.result))
	}
	return informativeTypes(m.elementResultTypes(src.scope, src.node))
}

// elementResultTypes is the types every element a value contributes has: the expression's own,
// or for a sequence `(a, b)` those each element conforms to; nil when any element is unknown.
func (m *Model) elementResultTypes(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	seq, ok := node.(*ast.SequenceExpr)
	if !ok {
		return m.resultTypes(scope, node)
	}
	var common []*symbols.Symbol
	for i, element := range seq.Elements {
		types := m.elementResultTypes(scope, element)
		if len(types) == 0 {
			return nil
		}
		if i > 0 {
			types = m.sharedTypes(common, types)
		}
		if common = types; len(common) == 0 {
			return nil
		}
	}
	return common
}

// sharedTypes is the types of either list a value of each list conforms to.
func (m *Model) sharedTypes(a, b []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, list := range [][]*symbols.Symbol{a, b} {
		for _, typ := range list {
			if !containsElement(out, typ) && m.anyConforms(a, typ) && m.anyConforms(b, typ) {
				out = append(out, typ)
			}
		}
	}
	return out
}

func (m *Model) anyConforms(types []*symbols.Symbol, want *symbols.Symbol) bool {
	for _, typ := range types {
		if m.Conforms(typ, want) {
			return true
		}
	}
	return false
}

// collectionConformance judges the value of a collection function call by its source: every
// element must conform. Not ok where the source is unknown and the declaration decides.
func (m *Model) collectionConformance(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	src, ok := m.invocationSource(scope, e, fn)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.sourceConformance(src, want, byUnit)
}

// collectConformance judges the value of `xs.{ in x; … }` by its body's result: every element must conform.
func (m *Model) collectConformance(scope *symbols.Scope, e *ast.CollectExpr, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	src, ok := m.collectSource(scope, e)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.sourceConformance(src, want, byUnit)
}

func (m *Model) sourceConformance(src collectionSource, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	judged := m.judgeSource(src,
		func(scope *symbols.Scope, node ast.Node) Conformance {
			return m.elementConformance(scope, node, want, byUnit)
		},
		func(result *symbols.Symbol) Conformance { return m.featureConformance(result, want) })
	return decided(everyHolds(judged))
}

// collectionCastConformance judges `xs.{…} as T` or a collection function call cast by its
// source: the cast is sound when some element's types and T specialize one another. Not ok
// where the source is unknown and the declaration decides.
func (m *Model) collectionCastConformance(scope *symbols.Scope, operand ast.Node, target *symbols.Symbol) (Conformance, bool) {
	var src collectionSource
	var ok bool
	switch n := operand.(type) {
	case *ast.CollectExpr:
		src, ok = m.collectSource(scope, n)
	case *ast.InvocationExpr:
		src, ok = m.invocationSource(scope, n, m.invocationCallee(scope, n))
	}
	if !ok {
		return conformanceUnknown(), false
	}
	judged := m.judgeSource(src,
		func(scope *symbols.Scope, node ast.Node) Conformance { return m.castConformance(scope, node, target) },
		func(result *symbols.Symbol) Conformance {
			return m.castTypesConformance(m.featureResultTypes(result), target)
		})
	return decided(anyHolds(judged))
}

// judgeSource judges each element of a source: a result parameter as the feature, an
// expression by itself, a sequence `(a, b)` element by element.
func (m *Model) judgeSource(src collectionSource, byNode func(*symbols.Scope, ast.Node) Conformance, byResult func(*symbols.Symbol) Conformance) []Conformance {
	if src.result != nil {
		return []Conformance{byResult(src.result)}
	}
	return elementJudgements(src.scope, src.node, byNode)
}

func elementJudgements(scope *symbols.Scope, node ast.Node, judge func(*symbols.Scope, ast.Node) Conformance) []Conformance {
	seq, ok := node.(*ast.SequenceExpr)
	if !ok {
		return []Conformance{judge(scope, node)}
	}
	var out []Conformance
	for _, element := range seq.Elements {
		out = append(out, elementJudgements(scope, element, judge)...)
	}
	return out
}

// elementConformance judges one element; a value typed Anything alone says nothing of it.
func (m *Model) elementConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol, byUnit bool) Conformance {
	c := m.exprConformance(scope, node, want, byUnit)
	if c.Known && !c.Holds && m.typedAnything(scope, node) {
		return conformanceUnknown()
	}
	return c
}

// typedAnything reports a value whose only type is Anything.
func (m *Model) typedAnything(scope *symbols.Scope, node ast.Node) bool {
	types := m.resultTypes(scope, node)
	return len(types) > 0 && len(informativeTypes(types)) == 0
}

// everyHolds holds when every judgement does, fails naming each known not to, and is
// unknown while none fails and one is unknown or untyped.
func everyHolds(judged []Conformance) Conformance {
	if len(judged) == 0 {
		return conformanceUnknown()
	}
	var found []string
	unknown := false
	for _, c := range judged {
		switch {
		case !c.Known || c.Untyped:
			unknown = true
		case !c.Holds:
			found = appendUnique(found, c.Found)
		}
	}
	switch {
	case len(found) > 0:
		return Conformance{Known: true, Found: strings.Join(found, " and ")}
	case unknown:
		return conformanceUnknown()
	}
	return Conformance{Known: true, Holds: true}
}

// anyHolds holds when any judgement does, fails naming each when every one is known not
// to, and is unknown while none holds and one is unknown or untyped.
func anyHolds(judged []Conformance) Conformance {
	if len(judged) == 0 {
		return conformanceUnknown()
	}
	var found []string
	for _, c := range judged {
		switch {
		case c.Known && c.Holds:
			return c
		case !c.Known || c.Untyped:
			return conformanceUnknown()
		}
		found = appendUnique(found, c.Found)
	}
	return Conformance{Known: true, Found: strings.Join(found, " and ")}
}

// decided is c with whether it decides anything: an unknown or untyped value leaves the
// declared result to decide.
func decided(c Conformance) (Conformance, bool) {
	return c, c.Known && !c.Untyped
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

func appendUnique(list []string, s string) []string {
	for _, item := range list {
		if item == s {
			return list
		}
	}
	return append(list, s)
}
