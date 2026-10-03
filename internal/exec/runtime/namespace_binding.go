package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A namespace-level binding connector (`bind a = b;` owned by a package or
// namespace rather than a type) requires its ends to have the same values
// (KerML 1.0 §7.4.6.3): every usage the run's namespace-owned bindings join —
// a member of one equivalence class — denotes the class's one value. That value
// is the one a member declares or a chain end evaluates to (several, pairwise
// equal or refused), or one object materialized for the earliest-declared
// member and classified by every member's types when none declares one.

// namespaceModelIndex is the model's namespace-owned bindings and subsetting
// usages walked once over every document's namespace scopes: which usages a
// binding joins (its equivalence class) and which usages subset a namespace
// usage, both in document-name then declaration order.
type namespaceModelIndex struct {
	subsetters  map[*symbols.Symbol][]*symbols.Symbol
	classes     map[*symbols.Symbol]*namespaceClass
	memberDecls map[ast.Node]*symbols.Symbol
}

// namespaceClass is one equivalence class of namespace usages joined by
// bindings: its members, the ends carrying a value of their own (a valued
// member or an end that names no usage, evaluated where its binding was
// written), and the bindings for their multiplicity refusal.
type namespaceClass struct {
	members  []*symbols.Symbol
	sources  []namespaceSource
	bindings []lower.Binding
}

// namespaceSource is one end carrying a value into its class: a valued usage
// end (usage non-nil) or an end written any other way (usage nil).
type namespaceSource struct {
	binding lower.Binding
	end     int
	usage   *symbols.Symbol
}

// namespaceModelIndex builds the index once per Model, on first need.
func (ctx *Context) namespaceModelIndex() *namespaceModelIndex {
	if ctx.model.namespaceUsageIndex != nil {
		return ctx.model.namespaceUsageIndex
	}
	out := &namespaceModelIndex{
		subsetters:  make(map[*symbols.Symbol][]*symbols.Symbol),
		classes:     make(map[*symbols.Symbol]*namespaceClass),
		memberDecls: make(map[ast.Node]*symbols.Symbol),
	}
	classOf := func(sym *symbols.Symbol) *namespaceClass {
		if class, ok := out.classes[sym]; ok {
			return class
		}
		class := &namespaceClass{}
		out.classes[sym] = class
		return class
	}
	merge := func(a, b *symbols.Symbol) *namespaceClass {
		ca, cb := classOf(a), classOf(b)
		if ca == cb {
			return ca
		}
		ca.members = append(ca.members, cb.members...)
		ca.sources = append(ca.sources, cb.sources...)
		ca.bindings = append(ca.bindings, cb.bindings...)
		for _, member := range cb.members {
			out.classes[member] = ca
		}
		out.classes[b] = ca
		return ca
	}
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		scope.ForEachMember(func(member *symbols.Symbol) bool {
			if member.Scope != nil && member.Scope != scope && namespaceScope(member.Scope) {
				walk(member.Scope)
				return true
			}
			member = ctx.declaredSymbol(member)
			if _, ok := member.Decl.(*ast.Usage); !ok {
				return true
			}
			for _, rel := range relationshipsOfKind(member, ast.RelSubsets) {
				if target := ctx.model.semantics.RelationshipTarget(member, rel); target != nil {
					out.subsetters[target] = append(out.subsetters[target], member)
				}
			}
			return true
		})
		for _, binding := range lower.NamespaceBindings(scope) {
			var class *namespaceClass
			for end := range binding.Ends {
				if u, ok := ctx.namespaceEndUsage(binding, end); ok {
					if class == nil {
						class = classOf(u)
					} else {
						class = merge(class.members[len(class.members)-1], u)
					}
					if !containsSymbol(class.members, u) {
						class.members = append(class.members, u)
					}
				}
			}
			if class == nil {
				continue
			}
			class.bindings = append(class.bindings, binding)
			for end := range binding.Ends {
				u, named := ctx.namespaceEndUsage(binding, end)
				if named {
					if decl, isUsage := u.Decl.(*ast.Usage); isUsage && decl.Value != nil {
						class.sources = append(class.sources, namespaceSource{binding: binding, end: end, usage: u})
					}
				} else {
					class.sources = append(class.sources, namespaceSource{binding: binding, end: end})
				}
			}
		}
	}
	if ctx.model.resolver != nil {
		if idx := ctx.model.resolver.Index(); idx != nil {
			for _, doc := range idx.Documents() {
				walk(idx.DocumentRoot(doc))
			}
		}
	}
	for _, class := range out.classes {
		sort.SliceStable(class.members, func(i, j int) bool { return declaredBefore(class.members[i], class.members[j]) })
		for _, member := range class.members {
			if member.Decl != nil {
				out.memberDecls[member.Decl] = member
			}
		}
	}
	for _, subs := range out.subsetters {
		sort.SliceStable(subs, func(i, j int) bool { return declaredBefore(subs[i], subs[j]) })
	}
	ctx.model.namespaceUsageIndex = out
	return out
}

// containsSymbol reports whether syms holds sym.
func containsSymbol(syms []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range syms {
		if s == sym {
			return true
		}
	}
	return false
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

// namespaceBoundObjects is the objects a namespace usage joined by a namespace-owned
// binding denotes. bound reports whether a binding governs the usage at all.
func (ctx *Context) namespaceBoundObjects(sym *symbols.Symbol) (objs []*Instance, bound bool, err error) {
	if val, ok := ctx.namespaceBindings[sym]; ok {
		return ctx.liveInstances(heldObjects(val)), true, nil
	}
	class, member := ctx.namespaceClassMember(sym)
	if class == nil {
		if live, ok := ctx.liveOccurrences(sym); ok {
			return live, true, nil
		}
		return nil, false, nil
	}
	return ctx.resolveNamespaceClass(class, member)
}

// namespaceClassMember is the binding class sym belongs to and its member
// declaring sym: a reader may hold a different scope tree's symbol for one
// declaration than the index's tree built the class of, so a missed symbol
// match is retried on the declaration.
func (ctx *Context) namespaceClassMember(sym *symbols.Symbol) (*namespaceClass, *symbols.Symbol) {
	nmi := ctx.namespaceModelIndex()
	if class := nmi.classes[sym]; class != nil {
		return class, sym
	}
	member := nmi.memberDecls[sym.Decl]
	if member == nil {
		return nil, nil
	}
	return nmi.classes[member], member
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

// resolveNamespaceClass makes every member of a binding's equivalence class denote
// the class's one value, reporting the objects the member asking for (want)
// denotes. A second call for the class under way answers not-bound so a member
// being evaluated resolves as usual — its declared value cycles through the
// binding stack — while a chain end reading a member of its own class is the
// cyclic dependency CyclicBindingError reports.
func (ctx *Context) resolveNamespaceClass(class *namespaceClass, want *symbols.Symbol) ([]*Instance, bool, error) {
	if ctx.resolvingNamespaceClasses[class] {
		if ctx.namespaceChainReads[class] {
			return nil, true, &CyclicBindingError{Usage: want, Stated: ctx.qualifiedSymbolName(want)}
		}
		return nil, false, nil
	}
	for _, binding := range class.bindings {
		if err := ctx.namespaceBindingCounts(binding); err != nil {
			return nil, true, err
		}
	}
	ctx.resolvingNamespaceClasses[class] = true
	defer delete(ctx.resolvingNamespaceClasses, class)

	type held struct {
		val  Value
		text string
	}
	var vals []held
	seen := make(map[*symbols.Symbol]bool)
	recorded := func() {
		for _, member := range class.members {
			if seen[member] {
				continue
			}
			if val, ok := ctx.namespaceBindings[member]; ok {
				vals = append(vals, held{val, symbolText(member)})
				seen[member] = true
				continue
			}
			if live, ok := ctx.liveOccurrences(member); ok {
				elements := make([]Value, 0, len(live))
				for _, inst := range live {
					val, err := ctx.objectValue(inst)
					if err != nil {
						return
					}
					elements = append(elements, val)
				}
				val := sequenceOf(elements)
				if len(elements) == 1 {
					val = elements[0]
				}
				vals = append(vals, held{val, symbolText(member)})
				seen[member] = true
			}
		}
	}
	recorded()
	for _, src := range class.sources {
		if src.usage != nil {
			if seen[src.usage] {
				continue
			}
			seen[src.usage] = true
			decl := src.usage.Decl.(*ast.Usage)
			val, err := NewEvalContext(ctx, src.usage.OwnerScope).declaredValue(src.usage, decl.Value)
			if err != nil {
				return nil, true, err
			}
			vals = append(vals, held{val, ctx.bindingEndpointText(src.binding, src.end)})
			continue
		}
		ctx.namespaceChainReads[class] = true
		val, err := NewEvalContext(ctx, src.binding.Scope).Eval(src.binding.Ends[src.end].Expr)
		delete(ctx.namespaceChainReads, class)
		if err != nil {
			return nil, true, err
		}
		vals = append(vals, held{val, ctx.bindingEndpointText(src.binding, src.end)})
	}
	// Members a source's evaluation denoted contribute their value to the class.
	recorded()
	for i := range vals {
		for j := i + 1; j < len(vals); j++ {
			if !ctx.equalValues(vals[i].val, vals[j].val) {
				return nil, true, &BindingConflictError{
					Left:       vals[i].text,
					Right:      vals[j].text,
					LeftValue:  vals[i].val,
					RightValue: vals[j].val,
				}
			}
		}
	}
	if len(vals) == 0 {
		// A class whose members denote no objects has nothing to materialize:
		// a valueless scalar binding leaves each member undetermined.
		objectBearing := false
		for _, member := range class.members {
			if ctx.namesOneObject(member) || ctx.namesObjects(member) ||
				isOccurrenceUsage(member) || objectFeature(member) {
				objectBearing = true
				break
			}
		}
		if !objectBearing {
			return nil, false, nil
		}
		// The class's one value is what the members' own denotations would
		// materialize: the largest lower bound among them, made the way
		// occurrencesOf makes them, so which member is read first does not
		// change it.
		earliest := class.members[0]
		count := 0
		for _, member := range class.members {
			n, err := ctx.lowerBoundCount(ctx.featureMultiplicity(member, ctx.findOwnerType(member)), 0, symbolText(member))
			if err != nil {
				return nil, true, fmt.Errorf("usage %s: %w", symbolText(member), err)
			}
			if n > count {
				count = n
			}
		}
		release := ctx.elementScope()
		if err := ctx.chargeElements(int64(count)); err != nil {
			release()
			return nil, true, err
		}
		mark := len(ctx.created)
		members, err := ctx.materializeMembers(earliest, count, nil, "")
		if err != nil {
			ctx.abandonInstancesSince(mark)
			release()
			return nil, true, fmt.Errorf("usage %s: %w", symbolText(earliest), err)
		}
		elements := make([]Value, 0, len(members))
		for _, inst := range members {
			obj, err := ctx.objectValue(inst)
			if err != nil {
				ctx.abandonInstancesSince(mark)
				release()
				return nil, true, err
			}
			elements = append(elements, obj)
		}
		val := sequenceOf(elements)
		if len(elements) == 1 {
			val = elements[0]
		}
		text := symbolText(earliest)
		if len(class.bindings) > 0 {
			text = ctx.bindingText(class.bindings[0])
		}
		// Recorded before the conform checks, which can start classifier
		// behaviors of the shared objects: one of them reading a member reaches
		// the objects the class already names rather than materializing another.
		for _, member := range class.members {
			ctx.occurrences[member] = heldObjects(val)
			ctx.bindNamespace(member, val)
		}
		fail := func(err error) ([]*Instance, bool, error) {
			for _, member := range class.members {
				delete(ctx.occurrences, member)
				ctx.unbindNamespace(member)
			}
			ctx.abandonInstancesSince(mark)
			release()
			return nil, true, err
		}
		count64 := int64(len(members))
		for _, member := range class.members {
			if msg := ctx.featureMultiplicity(member, ctx.findOwnerType(member)).CountViolation(count64); msg != "" {
				return fail(fmt.Errorf("%w: `%s`: %s", ErrBindingConflict, text, msg))
			}
			v, err := NewEvalContext(ctx, member.OwnerScope).conformDeclared(member, val)
			if err != nil {
				return fail(fmt.Errorf("%w: `%s`: %v", ErrBindingConflict, text, err))
			}
			if member != earliest {
				if err := ctx.classifyHeld(member, val); err != nil {
					return fail(err)
				}
			}
			ctx.occurrences[member] = heldObjects(v)
			ctx.bindNamespace(member, v)
		}
		if err := ctx.startClassifierBehaviorsOf(members, mark); err != nil {
			return fail(err)
		}
		release()
	} else {
		val := vals[0].val
		text := symbolText(class.members[0])
		if len(class.bindings) > 0 {
			text = ctx.bindingText(class.bindings[0])
		}
		for _, member := range class.members {
			if seen[member] {
				continue
			}
			ec := NewEvalContext(ctx, member.OwnerScope)
			v, err := ec.conformDeclared(member, val)
			if err != nil {
				return nil, true, fmt.Errorf("%w: `%s`: %v", ErrBindingConflict, text, err)
			}
			ctx.occurrences[member] = heldObjects(v)
			ctx.bindNamespace(member, v)
		}
	}
	if val, ok := ctx.namespaceBindings[want]; ok {
		return ctx.liveInstances(heldObjects(val)), true, nil
	}
	if live, ok := ctx.liveOccurrences(want); ok {
		return live, true, nil
	}
	return nil, true, nil
}

// namespaceBoundValue is the value a usage a binding connector governs reads as:
// the value resolving its class recorded for it when one was kept, else the live
// objects it denotes — as a read through the binding answers.
func (ctx *Context) namespaceBoundValue(sym *symbols.Symbol) (Value, bool, error) {
	objs, bound, err := ctx.namespaceBoundObjects(sym)
	if err != nil || !bound {
		return Value{}, bound, err
	}
	if val, ok := ctx.namespaceBindings[sym]; ok {
		return val, true, nil
	}
	if _, member := ctx.namespaceClassMember(sym); member != sym {
		if val, ok := ctx.namespaceBindings[member]; ok {
			return val, true, nil
		}
	}
	elements := make([]Value, 0, len(objs))
	for _, inst := range objs {
		obj, err := ctx.objectValue(inst)
		if err != nil {
			return Value{}, true, err
		}
		elements = append(elements, obj)
	}
	val := sequenceOf(elements)
	if len(elements) == 1 {
		val = elements[0]
	}
	return val, true, nil
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
	subs := ctx.namespaceModelIndex().subsetters[sym]
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
