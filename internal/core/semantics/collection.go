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

// CollectionResultTypes is the types every element of a collection value — `xs.{…}` or a call
// of a ControlFunctions collection function — has, and whether its arguments decide the value
// rather than the declared result alone; nil types where they are unknown or say nothing.
func (m *Model) CollectionResultTypes(scope *symbols.Scope, node ast.Node) ([]*symbols.Symbol, bool) {
	if m == nil || m.resolver == nil || node == nil {
		return nil, false
	}
	srcs, ok := m.sourcesOf(scope, node)
	if !ok {
		return nil, false
	}
	return m.sourcesTypes(srcs), true
}

// sourcesOf is the sources of a collection value: `xs.{…}` by its body, `xs.?{…}` by xs, a
// collection function call by its arguments; not ok for any other value.
func (m *Model) sourcesOf(scope *symbols.Scope, node ast.Node) ([]collectionSource, bool) {
	switch n := node.(type) {
	case *ast.CollectExpr:
		return m.collectSources(scope, n)
	case *ast.SelectExpr:
		return m.keptSources(scope, n.Operand)
	case *ast.InvocationExpr:
		return m.invocationSources(scope, n, m.invocationCallee(scope, n))
	}
	return nil, false
}

// invocationSources are the sources of a call of a ControlFunctions collection function: collect
// the body or function applied, select/reject/selectOne the collection, reduce the reducer and,
// unless the collection holds two or more, the one element it returns unreduced; none over a
// collection known to hold nothing, which nothing is applied to. Not ok where the arguments say
// nothing and the declaration alone decides.
func (m *Model) invocationSources(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol) ([]collectionSource, bool) {
	if fn == nil || e == nil {
		return nil, false
	}
	switch fn {
	case m.libSymbol(fqnCollect):
		if m.holdsNothing(scope, m.argumentTo(scope, e, fn, 0)) {
			return nil, true
		}
		return m.appliedSources(scope, m.argumentTo(scope, e, fn, 1))
	case m.libSymbol(fqnReduce):
		collection := m.argumentTo(scope, e, fn, 0)
		if m.holdsNothing(scope, collection) {
			return nil, true
		}
		srcs, ok := m.appliedSources(scope, m.argumentTo(scope, e, fn, 1))
		if !ok {
			return nil, false
		}
		if collection != nil && !m.holdsAtLeastTwo(scope, collection) {
			srcs = append(srcs, collectionSource{scope: scope, node: collection})
		}
		return srcs, true
	case m.libSymbol(fqnSelect), m.libSymbol(fqnReject), m.libSymbol(fqnSelectOne):
		return m.keptSources(scope, m.argumentTo(scope, e, fn, 0))
	}
	return nil, false
}

// keptSources is the source of a selection: the collection whose elements it keeps.
func (m *Model) keptSources(scope *symbols.Scope, collection ast.Node) ([]collectionSource, bool) {
	if collection == nil {
		return nil, false
	}
	return []collectionSource{{scope: scope, node: collection}}, true
}

// holdsAtLeastTwo reports a collection statically known to hold two elements or more.
func (m *Model) holdsAtLeastTwo(scope *symbols.Scope, collection ast.Node) bool {
	r, ok := m.valuesHeldBy(scope, collection)
	return ok && (r.Lower.Infinite || r.Lower.Value >= 2)
}

// holdsNothing reports a collection statically known to hold no element: `()`, or a feature
// whose multiplicity admits none.
func (m *Model) holdsNothing(scope *symbols.Scope, collection ast.Node) bool {
	r, ok := m.valuesHeldBy(scope, collection)
	return ok && !r.Upper.Infinite && r.Upper.Value == 0
}

// valuesHeldBy is how many values a collection expression holds: `()` none, a literal one, a
// sequence the sum over its elements, a feature or chain the multiplicity governing it, a chain
// holding through each value of its operand the values of its last feature; not ok where unknown.
func (m *Model) valuesHeldBy(scope *symbols.Scope, node ast.Node) (Range, bool) {
	switch n := node.(type) {
	case *ast.NullExpr:
		return exactly(0), true
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralString, *ast.LiteralBool:
		return exactly(1), true
	case *ast.SequenceExpr:
		sum := exactly(0)
		for _, element := range n.Elements {
			r, ok := m.valuesHeldBy(scope, element)
			if !ok {
				return Range{}, false
			}
			sum = Range{Lower: addBounds(sum.Lower, r.Lower), Upper: addBounds(sum.Upper, r.Upper)}
		}
		return sum, true
	case *ast.FeatureReference, *ast.QualifiedName:
		return m.valuesHeldByFeature(scope, n)
	case *ast.FeatureChainExpr:
		last, ok := m.valuesHeldByFeature(scope, n)
		if !ok {
			return Range{}, false
		}
		through, ok := m.valuesHeldBy(scope, n.Operand)
		if !ok {
			return Range{}, false
		}
		return Range{Lower: mulBounds(through.Lower, last.Lower), Upper: mulBounds(through.Upper, last.Upper)}, true
	}
	return Range{}, false
}

// valuesHeldByFeature is the multiplicity governing the feature a name or chain resolves to:
// the one it declares, or inherits from a feature it redefines; one where it declares none.
func (m *Model) valuesHeldByFeature(scope *symbols.Scope, node ast.Node) (Range, bool) {
	sym, ok := m.resolver.ResolveTarget(scope, node)
	if !ok || sym == nil {
		return Range{}, false
	}
	r := AssumedRange()
	if declared, ok := m.MultiplicityOf(sym); ok {
		r = declared
	} else {
		for _, redefined := range m.redefinedTransitively(sym) {
			if inherited, ok := m.MultiplicityOf(redefined); ok {
				r = inherited
				break
			}
		}
	}
	return r, r.Lower.Known && r.Upper.Known
}

func exactly(n int64) Range {
	b := Bound{Value: n, Known: true}
	return Range{Lower: b, Upper: b}
}

func addBounds(a, b Bound) Bound {
	if a.Infinite || b.Infinite {
		return Bound{Infinite: true, Known: true}
	}
	return Bound{Value: a.Value + b.Value, Known: true}
}

// mulBounds is the values held through a values, each holding b: none through none.
func mulBounds(a, b Bound) Bound {
	if (!a.Infinite && a.Value == 0) || (!b.Infinite && b.Value == 0) {
		return Bound{Known: true}
	}
	if a.Infinite || b.Infinite {
		return Bound{Infinite: true, Known: true}
	}
	return Bound{Value: a.Value * b.Value, Known: true}
}

// collectSources is the source of `xs.{ in x; … }`: its body's result; none where xs is known
// to hold nothing.
func (m *Model) collectSources(scope *symbols.Scope, e *ast.CollectExpr) ([]collectionSource, bool) {
	body, ok := e.Body.(*ast.BodyExpr)
	if !ok {
		return nil, false
	}
	if m.holdsNothing(scope, e.Operand) {
		return nil, true
	}
	return m.bodySources(scope, body)
}

// appliedSources is the source of the expression applied to each element: a body
// `{ in x; … }` by its result, a function or expression named by its result parameter.
func (m *Model) appliedSources(scope *symbols.Scope, applied ast.Node) ([]collectionSource, bool) {
	switch n := applied.(type) {
	case *ast.BodyExpr:
		return m.bodySources(scope, n)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		if result := m.appliedResult(scope, n); result != nil {
			return []collectionSource{{result: result}}, true
		}
	}
	return nil, false
}

// bodySources is the result of a body `{ in x; … }`, resolved among its parameters.
func (m *Model) bodySources(scope *symbols.Scope, body *ast.BodyExpr) ([]collectionSource, bool) {
	if body == nil || body.Result == nil || m.resolver == nil {
		return nil, false
	}
	return []collectionSource{{scope: symbols.BodyExprScope(scope, body), node: body.Result}}, true
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
	srcs, ok := m.invocationSources(scope, e, fn)
	if !ok {
		return nil
	}
	return m.sourcesTypes(srcs)
}

// collectResultTypes is the types every element of `xs.{ in x; … }` has; nil when unknown.
func (m *Model) collectResultTypes(scope *symbols.Scope, e *ast.CollectExpr) []*symbols.Symbol {
	srcs, ok := m.collectSources(scope, e)
	if !ok {
		return nil
	}
	return m.sourcesTypes(srcs)
}

// CollectionElement is one element a collection value may hold: the expression producing it,
// in its scope, or nil where a function's result parameter does; and its types, nil where unknown.
type CollectionElement struct {
	Scope *symbols.Scope
	Node  ast.Node
	Types []*symbols.Symbol
}

// CollectionElements is each element a collection value may hold, and whether the value is
// one whose arguments decide it rather than its declared result.
func (m *Model) CollectionElements(scope *symbols.Scope, node ast.Node) ([]CollectionElement, bool) {
	if m == nil || m.resolver == nil || node == nil {
		return nil, false
	}
	srcs, ok := m.sourcesOf(scope, node)
	if !ok {
		return nil, false
	}
	return m.sourcesElements(srcs), true
}

// sourcesTypes is the types every element of every source conforms to; nil when unknown or
// when only Anything, which says nothing.
func (m *Model) sourcesTypes(srcs []collectionSource) []*symbols.Symbol {
	var lists [][]*symbols.Symbol
	for _, element := range m.sourcesElements(srcs) {
		lists = append(lists, element.Types)
	}
	return informativeTypes(m.sharedAmong(lists))
}

// sourcesElements is each element of every source: a result parameter by its own types, a
// sequence `(a, b)` each element in turn; types nil where they say nothing.
func (m *Model) sourcesElements(srcs []collectionSource) []CollectionElement {
	var out []CollectionElement
	for _, src := range srcs {
		if src.result != nil {
			out = append(out, CollectionElement{Types: informativeTypes(m.featureResultTypes(src.result))})
			continue
		}
		out = append(out, m.elementsOf(src.scope, src.node)...)
	}
	return out
}

func (m *Model) elementsOf(scope *symbols.Scope, node ast.Node) []CollectionElement {
	seq, ok := node.(*ast.SequenceExpr)
	if !ok {
		return []CollectionElement{{Scope: scope, Node: node, Types: informativeTypes(m.resultTypes(scope, node))}}
	}
	var out []CollectionElement
	for _, element := range seq.Elements {
		out = append(out, m.elementsOf(scope, element)...)
	}
	return out
}

// sharedAmong is the types a value of every list conforms to; nil when there is no list or
// one is empty.
func (m *Model) sharedAmong(lists [][]*symbols.Symbol) []*symbols.Symbol {
	var common []*symbols.Symbol
	for i, types := range lists {
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

// collectionConformance judges the value of a collection function call by its sources: every
// element must conform. Not ok where the sources are unknown and the declaration decides.
func (m *Model) collectionConformance(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	srcs, ok := m.invocationSources(scope, e, fn)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.sourcesConformance(srcs, want, byUnit)
}

// collectConformance judges the value of `xs.{ in x; … }` by its body's result: every element must conform.
func (m *Model) collectConformance(scope *symbols.Scope, e *ast.CollectExpr, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	srcs, ok := m.collectSources(scope, e)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.sourcesConformance(srcs, want, byUnit)
}

// sourcesConformance judges a collection value by its sources: every element must conform. A
// value with no source holds nothing, an empty value typed Anything as `null` is.
func (m *Model) sourcesConformance(srcs []collectionSource, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	if len(srcs) == 0 {
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = "an empty value over a collection holding nothing, typed Anything"
			c.Untyped = true
		}
		return c, true
	}
	judged := m.judgeSources(srcs,
		func(scope *symbols.Scope, node ast.Node) Conformance {
			return m.elementConformance(scope, node, want, byUnit)
		},
		func(result *symbols.Symbol) Conformance { return m.featureConformance(result, want) })
	return decided(everyHolds(judged))
}

// collectionCastConformance judges `xs.{…} as T` or a collection function call cast by its
// sources: the cast is sound when some element's types and T specialize one another. Not ok
// where the sources are unknown and the declaration decides.
func (m *Model) collectionCastConformance(scope *symbols.Scope, operand ast.Node, target *symbols.Symbol) (Conformance, bool) {
	srcs, ok := m.sourcesOf(scope, operand)
	if !ok {
		return conformanceUnknown(), false
	}
	judged := m.judgeSources(srcs,
		func(scope *symbols.Scope, node ast.Node) Conformance { return m.castConformance(scope, node, target) },
		func(result *symbols.Symbol) Conformance {
			return m.castTypesConformance(m.featureResultTypes(result), target)
		})
	return decided(anyHolds(judged))
}

// judgeSources judges each element of every source: a result parameter as the feature, an
// expression by itself, a sequence `(a, b)` element by element.
func (m *Model) judgeSources(srcs []collectionSource, byNode func(*symbols.Scope, ast.Node) Conformance, byResult func(*symbols.Symbol) Conformance) []Conformance {
	var out []Conformance
	for _, src := range srcs {
		if src.result != nil {
			out = append(out, byResult(src.result))
			continue
		}
		out = append(out, elementJudgements(src.scope, src.node, byNode)...)
	}
	return out
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
