package semantics

import (
	"math"
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

// CollectionValues is how many values a collection value — `xs.{…}`, `xs.?{…}` or a call of a
// ControlFunctions collection function — holds, where the value is one and its size is known.
func (m *Model) CollectionValues(scope *symbols.Scope, node ast.Node) (Range, bool) {
	if m == nil || m.resolver == nil || node == nil {
		return Range{}, false
	}
	if _, ok := m.collectionOf(scope, node); !ok {
		return Range{}, false
	}
	return m.valuesHeldBy(scope, node)
}

// sourcesOf is the sources typing a collection value: `xs.{…}` by its body, `xs.?{…}` by xs, a
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

// heldSourcesOf is the sources of the elements a collection value holds: those typing it, less
// any known to yield nothing — all of them over a collection holding nothing.
func (m *Model) heldSourcesOf(scope *symbols.Scope, node ast.Node) ([]collectionSource, bool) {
	if collection, ok := m.collectionOf(scope, node); ok && m.holdsNothing(scope, collection) {
		return nil, true
	}
	srcs, ok := m.sourcesOf(scope, node)
	if !ok {
		return nil, false
	}
	held := make([]collectionSource, 0, len(srcs))
	for _, src := range srcs {
		if src.result != nil && m.resultHoldsNothing(src.result) || src.result == nil && m.holdsNothing(src.scope, src.node) {
			continue
		}
		held = append(held, src)
	}
	return held, true
}

// resultHoldsNothing reports a result parameter whose multiplicity admits no value.
func (m *Model) resultHoldsNothing(result *symbols.Symbol) bool {
	r, ok := m.governingMultiplicity(result)
	return ok && r.Upper.Known && !r.Upper.Infinite && r.Upper.Value == 0
}

// governingMultiplicity is the multiplicity a feature declares, or inherits from a feature it
// redefines by clause or position, read through an alias; not ok where it declares and inherits none.
func (m *Model) governingMultiplicity(sym *symbols.Symbol) (Range, bool) {
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok && alias != nil {
		sym = alias
	}
	if declared, ok := m.MultiplicityOf(sym); ok {
		return declared, true
	}
	for _, redefined := range m.AllRedefinedFeatures(sym) {
		if inherited, ok := m.MultiplicityOf(redefined); ok {
			return inherited, true
		}
	}
	return Range{}, false
}

func knownRange(r Range, ok bool) (Range, bool) {
	return r, ok && r.Lower.Known && r.Upper.Known
}

// collectionOf is the collection a collection value operates over: the operand of `xs.{…}` or
// `xs.?{…}`, the first argument of a collection function call; not ok for any other value.
func (m *Model) collectionOf(scope *symbols.Scope, node ast.Node) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.CollectExpr:
		return n.Operand, true
	case *ast.SelectExpr:
		return n.Operand, true
	case *ast.InvocationExpr:
		fn := m.invocationCallee(scope, n)
		if fn == nil || !m.isCollectionFunction(fn) {
			return nil, false
		}
		return m.argumentTo(scope, n, fn, 0), true
	}
	return nil, false
}

func (m *Model) isCollectionFunction(fn *symbols.Symbol) bool {
	for _, fqn := range []string{fqnCollect, fqnSelect, fqnReject, fqnSelectOne, fqnReduce} {
		if fn == m.libSymbol(fqn) {
			return true
		}
	}
	return false
}

// invocationSources type a collection function call: collect by what it applies, select/reject/
// selectOne by the collection, reduce by the reducer plus the one element it may return unreduced.
func (m *Model) invocationSources(scope *symbols.Scope, e *ast.InvocationExpr, fn *symbols.Symbol) ([]collectionSource, bool) {
	if fn == nil || e == nil {
		return nil, false
	}
	switch fn {
	case m.libSymbol(fqnCollect):
		return m.appliedSources(scope, m.argumentTo(scope, e, fn, 1))
	case m.libSymbol(fqnReduce):
		srcs, ok := m.appliedSources(scope, m.argumentTo(scope, e, fn, 1))
		if !ok {
			return nil, false
		}
		collection := m.argumentTo(scope, e, fn, 0)
		if collection != nil && !m.holdsAtLeastTwo(scope, collection) && !m.holdsNothing(scope, collection) {
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
// holding through each value of its operand the values of its last feature, a collection
// operation what it maps to, keeps or reduces; not ok where unknown.
func (m *Model) valuesHeldBy(scope *symbols.Scope, node ast.Node) (Range, bool) {
	switch n := node.(type) {
	case *ast.NullExpr:
		return exactly(0), true
	case *ast.CollectExpr:
		return m.valuesMappedBy(scope, n.Operand, n.Body)
	case *ast.SelectExpr:
		return m.valuesKeptFrom(scope, n.Operand, Bound{Infinite: true, Known: true})
	case *ast.InvocationExpr:
		return m.valuesHeldByCall(scope, n)
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralString, *ast.LiteralBool:
		return exactly(1), true
	case *ast.SequenceExpr:
		sum := exactly(0)
		for _, element := range n.Elements {
			r, ok := m.valuesHeldBy(scope, element)
			if !ok {
				return Range{}, false
			}
			sum = addRanges(sum, r)
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
		return mulRanges(through, last), true
	}
	return Range{}, false
}

// valuesHeldByCall is how many values a call holds: collect what it maps to, select/reject up to
// all, selectOne up to one, reduce one unless the collection holds none, any other function what
// its result parameter declares.
func (m *Model) valuesHeldByCall(scope *symbols.Scope, e *ast.InvocationExpr) (Range, bool) {
	fn := m.invocationCallee(scope, e)
	if fn == nil {
		return Range{}, false
	}
	switch fn {
	case m.libSymbol(fqnCollect):
		return m.valuesMappedBy(scope, m.argumentTo(scope, e, fn, 0), m.argumentTo(scope, e, fn, 1))
	case m.libSymbol(fqnSelect), m.libSymbol(fqnReject):
		return m.valuesKeptFrom(scope, m.argumentTo(scope, e, fn, 0), Bound{Infinite: true, Known: true})
	case m.libSymbol(fqnSelectOne):
		return m.valuesKeptFrom(scope, m.argumentTo(scope, e, fn, 0), Bound{Value: 1, Known: true})
	case m.libSymbol(fqnReduce):
		through, ok := m.valuesHeldBy(scope, m.argumentTo(scope, e, fn, 0))
		if !ok {
			return Range{}, false
		}
		return Range{Lower: minBound(through.Lower, Bound{Value: 1, Known: true}), Upper: minBound(through.Upper, Bound{Value: 1, Known: true})}, true
	}
	if result := m.ResultParameterOf(fn); result != nil {
		return m.valuesGoverning(result)
	}
	return Range{}, false
}

// valuesMappedBy is how many values mapping each of a collection's values through applied
// yields: as many as the body's result or the function's result parameter holds per value.
func (m *Model) valuesMappedBy(scope *symbols.Scope, collection, applied ast.Node) (Range, bool) {
	if collection == nil {
		return Range{}, false
	}
	through, ok := m.valuesHeldBy(scope, collection)
	if !ok {
		return Range{}, false
	}
	per := Range{Lower: Bound{Known: true}, Upper: Bound{Infinite: true, Known: true}}
	switch a := applied.(type) {
	case *ast.BodyExpr:
		if a.Result != nil {
			if r, ok := m.valuesHeldBy(symbols.BodyExprScope(scope, a), a.Result); ok {
				per = r
			}
		}
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		if result := m.appliedResult(scope, a); result != nil {
			if r, ok := m.valuesGoverning(result); ok {
				per = r
			}
		}
	}
	return mulRanges(through, per), true
}

// valuesKeptFrom is how many values a selection keeps: none up to the collection's, capped at most.
func (m *Model) valuesKeptFrom(scope *symbols.Scope, collection ast.Node, most Bound) (Range, bool) {
	if collection == nil {
		return Range{}, false
	}
	through, ok := m.valuesHeldBy(scope, collection)
	if !ok {
		return Range{}, false
	}
	return Range{Lower: Bound{Known: true}, Upper: minBound(through.Upper, most)}, true
}

// valuesHeldByFeature is how many values the feature a name or chain resolves to holds.
func (m *Model) valuesHeldByFeature(scope *symbols.Scope, node ast.Node) (Range, bool) {
	sym, ok := m.resolver.ResolveTarget(scope, node)
	if !ok || sym == nil {
		return Range{}, false
	}
	return m.valuesGoverning(sym)
}

// valuesGoverning is how many values a feature holds: the multiplicity governing it, through an
// alias, declared or inherited by redefinition — one where it has none; not ok while a bound
// it declares is not evaluable.
func (m *Model) valuesGoverning(sym *symbols.Symbol) (Range, bool) {
	if r, ok := m.governingMultiplicity(sym); ok {
		return knownRange(r, true)
	}
	return AssumedRange(), true
}

func exactly(n int64) Range {
	b := Bound{Value: n, Known: true}
	return Range{Lower: b, Upper: b}
}

// addRanges is the values two collections hold together; a sum past int64 is unbounded above
// and at least MaxInt64 below.
func addRanges(a, b Range) Range {
	return Range{Lower: addBounds(a.Lower, b.Lower, mostFinite), Upper: addBounds(a.Upper, b.Upper, unbounded)}
}

// mulRanges is the values held through each value of a, each holding b; a product past int64
// is unbounded above and at least MaxInt64 below.
func mulRanges(a, b Range) Range {
	return Range{Lower: mulBounds(a.Lower, b.Lower, mostFinite), Upper: mulBounds(a.Upper, b.Upper, unbounded)}
}

var (
	unbounded  = Bound{Infinite: true, Known: true}
	mostFinite = Bound{Value: math.MaxInt64, Known: true}
)

// addBounds is a + b, or past where int64 reaches.
func addBounds(a, b, past Bound) Bound {
	if a.Infinite || b.Infinite {
		return unbounded
	}
	if a.Value > math.MaxInt64-b.Value {
		return past
	}
	return Bound{Value: a.Value + b.Value, Known: true}
}

func minBound(a, b Bound) Bound {
	switch {
	case a.Infinite:
		return b
	case b.Infinite || a.Value <= b.Value:
		return a
	}
	return b
}

// mulBounds is a × b, none through none, or past where int64 reaches.
func mulBounds(a, b, past Bound) Bound {
	if (!a.Infinite && a.Value == 0) || (!b.Infinite && b.Value == 0) {
		return Bound{Known: true}
	}
	if a.Infinite || b.Infinite {
		return unbounded
	}
	if a.Value > math.MaxInt64/b.Value {
		return past
	}
	return Bound{Value: a.Value * b.Value, Known: true}
}

// collectSources is the source of `xs.{ in x; … }`: its body's result.
func (m *Model) collectSources(scope *symbols.Scope, e *ast.CollectExpr) ([]collectionSource, bool) {
	body, ok := e.Body.(*ast.BodyExpr)
	if !ok {
		return nil, false
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

// CollectionElements is each element a collection value may hold (a nested collection value by
// its own elements), and whether the value's arguments decide it rather than its declaration.
func (m *Model) CollectionElements(scope *symbols.Scope, node ast.Node) ([]CollectionElement, bool) {
	if m == nil || m.resolver == nil || node == nil {
		return nil, false
	}
	srcs, ok := m.heldSourcesOf(scope, node)
	if !ok {
		return nil, false
	}
	return m.heldElements(srcs), true
}

// CollectionHeldTypes is the types every element a collection value holds has, and whether the
// value's arguments decide it; nil types where it holds nothing or they are unknown.
func (m *Model) CollectionHeldTypes(scope *symbols.Scope, node ast.Node) ([]*symbols.Symbol, bool) {
	elements, ok := m.CollectionElements(scope, node)
	if !ok {
		return nil, false
	}
	lists := make([][]*symbols.Symbol, 0, len(elements))
	for _, element := range elements {
		lists = append(lists, element.Types)
	}
	return informativeTypes(m.sharedAmong(lists)), true
}

// heldElements is each element the sources hold, a collection-valued one by its elements in turn.
func (m *Model) heldElements(srcs []collectionSource) []CollectionElement {
	var out []CollectionElement
	for _, element := range m.sourcesElements(srcs) {
		if element.Node == nil {
			out = append(out, element)
			continue
		}
		if m.holdsNothing(element.Scope, element.Node) {
			continue
		}
		inner, ok := m.heldSourcesOf(element.Scope, element.Node)
		if !ok {
			out = append(out, element)
			continue
		}
		out = append(out, m.heldElements(inner)...)
	}
	return out
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

// collectionConformance judges `xs.{…}` or a collection function call by the elements it
// holds: every one must conform. Not ok where they are unknown and the declaration decides.
func (m *Model) collectionConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol, byUnit bool) (Conformance, bool) {
	srcs, ok := m.heldSourcesOf(scope, node)
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

// collectionCastConformance judges a collection value cast to T by the elements it holds: sound
// when some element and T specialize one another, or none is held (as `null as T` is).
func (m *Model) collectionCastConformance(scope *symbols.Scope, operand ast.Node, target *symbols.Symbol) (Conformance, bool) {
	srcs, ok := m.heldSourcesOf(scope, operand)
	if !ok {
		return conformanceUnknown(), false
	}
	if len(srcs) == 0 {
		return Conformance{Known: true, Holds: true}, true
	}
	judged := m.judgeSources(srcs,
		func(scope *symbols.Scope, node ast.Node) Conformance { return m.castConformance(scope, node, target) },
		func(result *symbols.Symbol) Conformance {
			return m.castTypesConformance(m.featureResultTypes(result), target)
		})
	return decided(anyHolds(judged))
}

// judgeSources judges each element of every source: a result parameter as the feature, an
// expression by itself, a sequence `(a, b)` element by element, one holding nothing not at all.
func (m *Model) judgeSources(srcs []collectionSource, byNode func(*symbols.Scope, ast.Node) Conformance, byResult func(*symbols.Symbol) Conformance) []Conformance {
	var out []Conformance
	for _, src := range srcs {
		if src.result != nil {
			out = append(out, byResult(src.result))
			continue
		}
		out = append(out, m.elementJudgements(src.scope, src.node, byNode)...)
	}
	return out
}

func (m *Model) elementJudgements(scope *symbols.Scope, node ast.Node, judge func(*symbols.Scope, ast.Node) Conformance) []Conformance {
	if m.holdsNothing(scope, node) {
		return nil
	}
	seq, ok := node.(*ast.SequenceExpr)
	if !ok {
		return []Conformance{judge(scope, node)}
	}
	var out []Conformance
	for _, element := range seq.Elements {
		out = append(out, m.elementJudgements(scope, element, judge)...)
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
	unknown := false
	for _, c := range judged {
		switch {
		case c.Known && c.Holds:
			return c
		case !c.Known || c.Untyped:
			unknown = true
		default:
			found = appendUnique(found, c.Found)
		}
	}
	if unknown {
		return conformanceUnknown()
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
		if !IsAnything(typ) {
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
