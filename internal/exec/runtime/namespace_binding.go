package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A namespace-level binding connector (`bind a = b;` owned by a package or
// namespace rather than a type) requires its two ends to have the same values
// (KerML 1.0 §7.4.6.3): two valueless usages it joins share one object,
// materialized once and registered as an occurrence of the earlier-declared
// end's usage; a valueless usage bound to a feature chain denotes the chain's
// objects; two ends carrying values of their own are a consistency check.

// namespaceBindingsOf is the lowered binding connectors scope owns, taken once per Model.
func (ctx *Context) namespaceBindingsOf(scope *symbols.Scope) []lower.Binding {
	if cached, ok := ctx.model.namespaceBindingIR[scope]; ok {
		return cached
	}
	bindings := lower.NamespaceBindings(scope)
	ctx.model.namespaceBindingIR[scope] = bindings
	return bindings
}

// namespaceEndUsage resolves a binding end written as a name to the usage it
// declares; an end written any other way names no usage at namespace level.
func (ctx *Context) namespaceEndUsage(binding lower.Binding, end int) (*symbols.Symbol, bool) {
	expr := binding.Ends[end].Expr
	if expr == nil {
		return nil, false
	}
	qn := ast.AsQualifiedName(expr)
	if qn == nil {
		return nil, false
	}
	sym, ok := ctx.resolveQualified(binding.Scope, qn)
	if !ok || sym == nil {
		return nil, false
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return nil, false
	}
	return sym, true
}

// namespaceBindingFor is the namespace-owned binding whose end names sym, and
// which end of it sym is.
func (ctx *Context) namespaceBindingFor(sym *symbols.Symbol) (lower.Binding, int, bool) {
	scope := sym.OwnerScope
	if scope == nil || !namespaceScope(scope) {
		return lower.Binding{}, -1, false
	}
	for _, binding := range ctx.namespaceBindingsOf(scope) {
		for end := range binding.Ends {
			if u, ok := ctx.namespaceEndUsage(binding, end); ok && u == sym {
				return binding, end, true
			}
		}
	}
	return lower.Binding{}, -1, false
}

// namespaceBoundObjects is the objects a namespace usage joined by a namespace-owned
// binding denotes. bound reports whether a binding governs the usage at all.
func (ctx *Context) namespaceBoundObjects(sym *symbols.Symbol) (objs []*Instance, bound bool, err error) {
	if val, ok := ctx.namespaceBindings[sym]; ok {
		return ctx.liveInstances(heldObjects(val)), true, nil
	}
	if live, ok := ctx.liveOccurrences(sym); ok {
		return live, true, nil
	}
	binding, _, ok := ctx.namespaceBindingFor(sym)
	if !ok {
		return nil, false, nil
	}
	return ctx.resolveNamespaceBinding(binding, sym)
}

// liveInstances is the live objects ids names, in order.
func (ctx *Context) liveInstances(ids []int64) []*Instance {
	out := make([]*Instance, 0, len(ids))
	for _, id := range ids {
		if inst, live := ctx.instances[id]; live {
			out = append(out, inst)
		}
	}
	return out
}

// resolveNamespaceBinding makes the ends of a namespace-owned binding denote the same
// values, reporting the objects the end asking for (want) denotes. A second call for
// the binding under way answers not-bound so the end being evaluated resolves as usual.
func (ctx *Context) resolveNamespaceBinding(binding lower.Binding, want *symbols.Symbol) ([]*Instance, bool, error) {
	if ctx.resolvingNamespaceBindings[binding.Decl] {
		return nil, false, nil
	}
	if err := ctx.namespaceBindingCounts(binding); err != nil {
		return nil, true, err
	}
	ctx.resolvingNamespaceBindings[binding.Decl] = true
	defer delete(ctx.resolvingNamespaceBindings, binding.Decl)

	valueless := [2]*symbols.Symbol{}
	for end := range binding.Ends {
		sym, ok := ctx.namespaceEndUsage(binding, end)
		if !ok {
			continue
		}
		if decl, isUsage := sym.Decl.(*ast.Usage); isUsage && decl.Value == nil {
			valueless[end] = sym
		}
	}
	switch {
	case valueless[0] != nil && valueless[1] != nil:
		return ctx.bindSharedNamespaceObject(binding, valueless, want)
	case valueless[0] != nil || valueless[1] != nil:
		return ctx.bindNamespaceEnd(binding, valueless, want)
	default:
		return ctx.checkNamespaceBinding(binding, want)
	}
}

// namespaceBindingCounts refuses an end or connector multiplicity other than the one
// link a binding connector declares, as a binding between object features is refused.
func (ctx *Context) namespaceBindingCounts(binding lower.Binding) error {
	for end := range binding.Ends {
		if r, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Ends[end].Multiplicity); ok {
			if n, exact := r.Exactly(); !exact || n != 1 {
				return &UndeterminedBindingError{
					Target:   ctx.bindingText(binding),
					Binding:  ctx.bindingText(binding),
					Endpoint: ctx.bindingEndpointText(binding, end),
					Other:    ctx.bindingEndpointText(binding, 1-end),
				}
			}
		}
	}
	if r, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity); ok {
		if n, exact := r.Exactly(); !exact || n != 1 {
			return fmt.Errorf("%w: `%s` declares %s link(s) but joins namespace usages, each of which denotes one value",
				ErrBindingEnd, ctx.bindingText(binding), r.Text())
		}
	}
	return nil
}

// bindSharedNamespaceObject makes two valueless namespace usages denote one object:
// the objects either already denotes, or one materialized once for the
// earlier-declared end's usage and classified by the other as its value.
func (ctx *Context) bindSharedNamespaceObject(binding lower.Binding, ends [2]*symbols.Symbol, want *symbols.Symbol) ([]*Instance, bool, error) {
	earlier, later := ends[0], ends[1]
	if !declaredBefore(earlier, later) {
		earlier, later = later, earlier
	}
	if live, ok := ctx.liveOccurrences(earlier); ok {
		return live, true, nil
	}
	if live, ok := ctx.liveOccurrences(later); ok {
		return live, true, nil
	}
	ctx.bindingStack = append(ctx.bindingStack, ends[0], ends[1])
	defer func() { ctx.bindingStack = ctx.bindingStack[:len(ctx.bindingStack)-2] }()
	inst, err := ctx.occurrenceOf(earlier)
	if err != nil {
		return nil, true, fmt.Errorf("usage %s: %w", symbolText(earlier), err)
	}
	val, err := ctx.objectValue(inst)
	if err != nil {
		return nil, true, err
	}
	if err := ctx.classifyHeld(later, val); err != nil {
		return nil, true, err
	}
	ctx.bindNamespace(later, val)
	objs, _, err := ctx.namespaceBoundObjects(want)
	return objs, true, err
}

// bindNamespaceEnd makes a valueless namespace usage denote the objects the binding's
// other end already denotes: that end's own value, evaluated and conformed to the
// usage's declaration (KerML 1.0 §7.4.6.3).
func (ctx *Context) bindNamespaceEnd(binding lower.Binding, ends [2]*symbols.Symbol, want *symbols.Symbol) ([]*Instance, bool, error) {
	uEnd := 0
	if ends[1] != nil {
		uEnd = 1
	}
	u := ends[uEnd]
	ctx.bindingStack = append(ctx.bindingStack, u)
	defer func() { ctx.bindingStack = ctx.bindingStack[:len(ctx.bindingStack)-1] }()
	val, err := ctx.namespaceBindingEndValue(binding, 1-uEnd)
	if err != nil {
		return nil, true, err
	}
	ec := NewEvalContext(ctx, u.OwnerScope)
	val, err = ec.conformDeclared(u, val)
	if err != nil {
		return nil, true, fmt.Errorf("%w: `%s`: %v", ErrBindingConflict, ctx.bindingText(binding), err)
	}
	ctx.occurrences[u] = heldObjects(val)
	ctx.bindNamespace(u, val)
	objs, _, err := ctx.namespaceBoundObjects(want)
	return objs, true, err
}

// namespaceBindingEndValue is the value a binding end carries: a valued usage's
// declared value, or its expression evaluated where the binding was written.
func (ctx *Context) namespaceBindingEndValue(binding lower.Binding, end int) (Value, error) {
	if sym, ok := ctx.namespaceEndUsage(binding, end); ok {
		if val, ok := ctx.namespaceBindings[sym]; ok {
			return val, nil
		}
		if decl, isUsage := sym.Decl.(*ast.Usage); isUsage && decl.Value != nil {
			return NewEvalContext(ctx, sym.OwnerScope).declaredValue(sym, decl.Value)
		}
		if live, ok := ctx.liveOccurrences(sym); ok {
			elements := make([]Value, 0, len(live))
			for _, inst := range live {
				val, err := ctx.objectValue(inst)
				if err != nil {
					return Value{}, err
				}
				elements = append(elements, val)
			}
			return sequenceOf(elements), nil
		}
		return Value{}, nil
	}
	return NewEvalContext(ctx, binding.Scope).Eval(binding.Ends[end].Expr)
}

// checkNamespaceBinding is the consistency check for a binding whose ends each carry
// a value of their own: the two must be equal, as a bound feature's is.
func (ctx *Context) checkNamespaceBinding(binding lower.Binding, want *symbols.Symbol) ([]*Instance, bool, error) {
	var vals [2]Value
	for end := range binding.Ends {
		val, err := ctx.namespaceBindingEndValue(binding, end)
		if err != nil {
			return nil, true, err
		}
		vals[end] = val
	}
	if !ctx.equalValues(vals[0], vals[1]) {
		return nil, true, &BindingConflictError{
			Left:       ctx.bindingEndpointText(binding, 0),
			Right:      ctx.bindingEndpointText(binding, 1),
			LeftValue:  vals[0],
			RightValue: vals[1],
		}
	}
	var wantEnd int
	for end := range binding.Ends {
		if sym, ok := ctx.namespaceEndUsage(binding, end); ok && sym == want {
			wantEnd = end
		}
	}
	return ctx.liveInstances(heldObjects(vals[wantEnd])), true, nil
}

// namespacedSubsetObjects is the objects a namespace usage takes from the usages
// subsetting it, as a composite collection takes its subsetting features' values
// (KerML 1.0 §7.3.4.4): their objects are members, optional subsetters with room and
// then anonymous objects make up the lower bound, and an abstract usage holds its
// concrete specializations' values alone. ok reports whether any usage subsets sym.
func (ctx *Context) namespacedSubsetObjects(sym *symbols.Symbol) ([]*Instance, bool, error) {
	if sym == nil || sym.OwnerScope == nil || !namespaceScope(sym.OwnerScope) {
		return nil, false, nil
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Value != nil || ctx.model.semantics.IsVariationFeature(sym) {
		return nil, false, nil
	}
	if !isOccurrenceUsage(sym) && !objectFeature(sym) {
		return nil, false, nil
	}
	subs := ctx.namespaceSubsetters(sym)
	if len(subs) == 0 {
		return nil, false, nil
	}
	if live, ok := ctx.liveOccurrences(sym); ok {
		return live, true, nil
	}
	if ctx.namespaceCollecting[sym] {
		return nil, true, fmt.Errorf("%w: usage %s subsets itself", ErrCyclicFeatureValue, symbolText(sym))
	}
	ctx.namespaceCollecting[sym] = true
	defer delete(ctx.namespaceCollecting, sym)

	release := ctx.elementScope()
	var ids []int64
	fail := func(err error) ([]*Instance, bool, error) {
		release()
		return nil, true, err
	}

	var contributed []*Instance
	var optional []*symbols.Symbol
	for _, sub := range subs {
		if ctx.optionalValueless(sub) {
			optional = append(optional, sub)
			continue
		}
		objs, err := ctx.denotedSubsetObjects(sub)
		if err != nil {
			return fail(fmt.Errorf("subsetting usage %s of %s: %w", symbolText(sub), symbolText(sym), err))
		}
		for _, inst := range objs {
			if !containsInstance(contributed, inst.ID) {
				contributed = append(contributed, inst)
				ids = append(ids, inst.ID)
			}
		}
	}

	mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym))
	abstract := symbols.IsAbstract(sym)
	if !abstract {
		count, err := ctx.lowerBoundCount(mult, len(contributed), fmt.Sprintf("usage %s", symbolText(sym)))
		if err != nil {
			return fail(err)
		}
		if err := ctx.chargeElements(int64(count)); err != nil {
			return fail(err)
		}
		mark := len(ctx.created)
		var newObjs []*Instance
		for _, sub := range optional {
			if count == 0 {
				break
			}
			upper := ctx.featureMultiplicity(sub, ctx.findOwnerType(sub)).Upper
			spare := count
			if upper.Known && !upper.Infinite && int(upper.Value) < spare {
				spare = int(upper.Value)
			}
			var filled []int64
			for i := 0; i < spare; i++ {
				inst, err := ctx.materialize(sub, 0, nil, "")
				if err != nil {
					ctx.abandonInstancesSince(mark)
					return fail(err)
				}
				filled = append(filled, inst.ID)
				newObjs = append(newObjs, inst)
				ids = append(ids, inst.ID)
				count--
			}
			if len(filled) > 0 {
				ctx.occurrences[sub] = filled
			}
		}
		made, err := ctx.materializeMembers(sym, count, nil, "")
		if err != nil {
			ctx.abandonInstancesSince(mark)
			return fail(err)
		}
		newObjs = append(newObjs, made...)
		for _, inst := range newObjs {
			contributed = append(contributed, inst)
			ids = append(ids, inst.ID)
		}
		if err := ctx.startClassifierBehaviorsOf(newObjs, mark); err != nil {
			ctx.abandonInstancesSince(mark)
			return fail(err)
		}
	}
	if msg := mult.CountViolation(int64(len(contributed))); msg != "" {
		return fail(fmt.Errorf("usage %s: %w: %s", symbolText(sym), ErrMultiplicityViolation, msg))
	}
	ctx.occurrences[sym] = ids
	release()
	return contributed, true, nil
}

// denotedSubsetObjects is the objects a usage subsetting a namespace usage
// contributes: what it denotes, or what its declared value binds it to.
func (ctx *Context) denotedSubsetObjects(sym *symbols.Symbol) ([]*Instance, error) {
	if namespaceObjectUsage(sym) {
		ec := NewEvalContext(ctx, sym.OwnerScope)
		val, err := ec.declaredValue(sym, sym.Decl.(*ast.Usage).Value)
		if err != nil {
			return nil, err
		}
		return ctx.liveInstances(heldObjects(val)), nil
	}
	return ctx.denotedObjects(sym)
}

// containsInstance reports whether objs holds the object id names.
func containsInstance(objs []*Instance, id int64) bool {
	for _, inst := range objs {
		if inst.ID == id {
			return true
		}
	}
	return false
}

// namespaceSubsetters is the usages any namespace of the model declares that
// subset sym, in document-name then declaration order.
func (ctx *Context) namespaceSubsetters(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		scope.ForEachMember(func(member *symbols.Symbol) bool {
			if member.Scope != nil && member.Scope != scope && namespaceScope(member.Scope) {
				walk(member.Scope)
				return true
			}
			member = ctx.declaredSymbol(member)
			if member == sym {
				return true
			}
			if _, ok := member.Decl.(*ast.Usage); !ok {
				return true
			}
			for _, rel := range relationshipsOfKind(member, ast.RelSubsets) {
				if ctx.model.semantics.RelationshipTarget(member, rel) == sym {
					out = append(out, member)
					break
				}
			}
			return true
		})
	}
	if ctx.model.resolver != nil {
		if idx := ctx.model.resolver.Index(); idx != nil {
			for _, doc := range idx.Documents() {
				walk(idx.DocumentRoot(doc))
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return declaredBefore(out[i], out[j]) })
	return out
}
