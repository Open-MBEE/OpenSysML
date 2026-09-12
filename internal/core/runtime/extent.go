package runtime

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evalExtent evaluates `all T` (KerML 1.0 §7.4.9.2, BaseFunctions::'all'): the instances of T
// this run enumerates, in declaration order; objects materialize lazily, so the extent is the run's.
func (ec *EvalContext) evalExtent(n *ast.OperatorExpr) (Value, error) {
	qn := semantics.ExtentTypeName(n)
	if qn == nil {
		return Value{}, fmt.Errorf("%w: 'all' requires the name of a type", ErrTypeMismatch)
	}
	sem := ec.ctx.model.semantics
	target, ok := ec.ctx.extentOperand(ec.scope, qn)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType, qualifiedNameToString(qn))
	}
	if !semantics.IsType(target) {
		return Value{}, fmt.Errorf("%w: 'all' requires a type, %s is a %s",
			ErrTypeMismatch, qualifiedNameToString(qn), target.Notation())
	}
	switch {
	case target.Kind == symbols.SymbolEnumerationDef:
		return ec.literalValues(sem.LiteralsOf(target))
	case sem.IsVariationFeature(target):
		return ec.variantValues(target, sem.VariantsOf(target))
	case sem.IsDataType(target):
		// A data value is not created by a run, so what a run's attributes hold is not the extent.
		return Value{}, fmt.Errorf("%w: %s is a data type, whose values are not enumerated (only an enumeration's literals are)",
			ErrUnboundedExtent, qualifiedNameToString(qn))
	}
	roots, err := ec.extentRoots(target)
	if err != nil {
		return Value{}, err
	}
	return ec.ctx.objectsOf(roots, target)
}

// extentOperand is what `all T` names, as semantics.ExtentOperand finds it: T through an alias to its
// target, recorded by what the name denoted and what the type declares for the binding being made.
func (ctx *Context) extentOperand(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	sym, ok := ctx.resolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil, false
	}
	if alias, ok := ctx.resolveAliasTarget(sym); ok && alias != nil {
		sym = alias
	}
	ctx.noteDeclarationRead(sym)
	return sym, true
}

// literalValues is the sequence of the values an enumeration's literals stand for.
func (ec *EvalContext) literalValues(literals []*symbols.Symbol) (Value, error) {
	values := make([]Value, 0, len(literals))
	for _, literal := range literals {
		val, err := ec.enumLiteralValue(literal)
		if err != nil {
			return Value{}, err
		}
		values = append(values, val)
	}
	return ec.newSequence(values)
}

// variantValues is what a variation's variants stand for: the value each declares, or an
// object of it, materialized once for the object evaluating, as a selection is.
func (ec *EvalContext) variantValues(variation *symbols.Symbol, variants []*symbols.Symbol) (Value, error) {
	owner := int64(0)
	if ec.self != nil {
		owner = ec.self.ID
	}
	values := make([]Value, 0, len(variants))
	for _, variant := range variants {
		val, err := ec.ctx.variantValue(variation, variant, owner)
		if err != nil {
			return Value{}, err
		}
		values = append(values, val)
	}
	return ec.newSequence(values)
}

// objectsOf is this run's objects of target under roots, each then its features in declaration
// order: a feature that may hold one is read (a failing read ends the extent) unless it would create
// an object of a declaration already on the path; what a feature holds is walked regardless.
func (ctx *Context) objectsOf(roots []*Instance, target *symbols.Symbol) (Value, error) {
	var values []Value
	seen := make(map[int64]bool)
	path := make(map[*symbols.Symbol]int)
	through := func(inst *Instance, of ObjectFeature) (*FeatureValue, error) {
		if !ctx.mayHold(of.Feature.Symbol, target, make(map[*symbols.Symbol]bool)) {
			return nil, nil
		}
		// A value whose every possible type is on the path is not read; any other read tells by what it made.
		types := ctx.createdTypes(of.Feature)
		recursive := len(types) > 0
		for _, typ := range types {
			if path[typ] == 0 {
				recursive = false
			}
		}
		if recursive {
			return nil, nil
		}
		return ctx.readUnlessRecursive(inst, of.Name, target, path)
	}
	var descend func(inst *Instance) error
	descend = func(inst *Instance) error {
		if inst == nil || seen[inst.ID] {
			return nil
		}
		seen[inst.ID] = true
		if ctx.isOf(inst, target) {
			val, err := ctx.objectValue(inst)
			if err != nil {
				return err
			}
			values = append(values, val)
		}
		declared := ctx.declarationsOf(inst)
		for _, decl := range declared {
			path[decl]++
		}
		defer func() {
			for _, decl := range declared {
				path[decl]--
			}
		}()
		children, err := ctx.heldObjectsOf(inst, through, true)
		if err != nil {
			return fmt.Errorf("object of %s: %w", symbolText(inst.Type), err)
		}
		for _, child := range children {
			if err := descend(child.instance); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := descend(root); err != nil {
			return Value{}, err
		}
	}
	return ctx.newSequence(values)
}

// readUnlessRecursive reads a feature of inst whose objects only the read reveals: kept when none
// it created is of a declaration on the path, else undone and nil as if unread. A failing read is
// reported, as is a value making an object on the path together with one that may lead to target.
func (ctx *Context) readUnlessRecursive(inst *Instance, name string, target *symbols.Symbol, path map[*symbols.Symbol]int) (*FeatureValue, error) {
	commit, rollback := ctx.beginJournal()
	mark := len(ctx.created)
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		rollback()
		return nil, err
	}
	held := make(map[int64]bool)
	for _, id := range heldObjects(fv.HeldValue()) {
		held[id] = true
	}
	var recursive, reached *Instance
	for _, id := range ctx.created[mark:] {
		made, live := ctx.instances[id]
		if !live {
			continue
		}
		if decl := ctx.onPath(made, path); decl != nil {
			recursive = made
		} else if held[id] && ctx.mayReach(made, target) {
			reached = made
		}
	}
	if recursive == nil {
		commit()
		return fv, nil
	}
	rollback()
	if reached != nil {
		return nil, fmt.Errorf("%w: the value of %s makes an object of %s, already on the path, together with one of %s, which the extent cannot reach without it",
			ErrExtentUnavailable, name, symbolText(ctx.onPath(recursive, path)), symbolText(reached.Type))
	}
	return nil, nil
}

// mayReach reports whether inst, or an object a feature of it holds however deep, may be of target.
func (ctx *Context) mayReach(inst *Instance, target *symbols.Symbol) bool {
	if ctx.isOf(inst, target) {
		return true
	}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if of.Name != "" && holdsObjects(of.Feature) && ctx.mayHold(of.Feature.Symbol, target, make(map[*symbols.Symbol]bool)) {
			return true
		}
	}
	return false
}

// onPath is the declaration on the path inst is of, if any.
func (ctx *Context) onPath(inst *Instance, path map[*symbols.Symbol]int) *symbols.Symbol {
	for _, decl := range ctx.declarationsOf(inst) {
		if path[decl] > 0 {
			return decl
		}
	}
	return nil
}

// declarationsOf is the declarations an object is of: the types it was created as and since
// classified by, and the types each usage among them is written with.
func (ctx *Context) declarationsOf(inst *Instance) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	var add func(sym *symbols.Symbol)
	add = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		out = append(out, sym)
		if sym.IsFeature() {
			for _, typ := range ctx.model.semantics.DeclaredFeatureTypes(sym) {
				add(typ)
			}
		}
	}
	for _, typ := range inst.types() {
		add(typ)
	}
	return out
}

// extentRoots is the objects an extent is searched from, in declaration order: the run's free-standing
// objects, the evaluating object's outermost holder, and what the enclosing namespaces' usages denote.
func (ec *EvalContext) extentRoots(target *symbols.Symbol) ([]*Instance, error) {
	ctx := ec.ctx
	var roots []*Instance
	declared := make(map[int64]*symbols.Symbol)
	add := func(inst *Instance, decl *symbols.Symbol) {
		if inst != nil && declared[inst.ID] == nil {
			declared[inst.ID] = decl
			roots = append(roots, inst)
		}
	}
	for top := ec.self; top != nil; top = top.owner {
		if top.owner == nil {
			add(top, top.Type)
		}
	}
	for _, sym := range ctx.namespaceUsages(ec.scope) {
		if !ctx.mayHold(sym, target, make(map[*symbols.Symbol]bool)) {
			continue
		}
		if namespaceObjectUsage(sym) {
			if ctx.binding(sym) {
				continue
			}
			bound, err := ec.boundObjects(sym)
			if err != nil {
				return nil, err
			}
			for _, inst := range bound {
				add(inst, sym)
			}
			continue
		}
		denoted, err := ctx.denotedObjects(sym)
		if err != nil {
			return nil, err
		}
		for _, inst := range denoted {
			add(inst, sym)
		}
	}
	held := ctx.heldObjectIDs()
	for _, id := range ctx.created {
		inst, live := ctx.instances[id]
		if !live || held[id] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		add(inst, inst.Type)
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return declaredBefore(declared[roots[i].ID], declared[roots[j].ID])
	})
	return roots, nil
}

// boundObjects is the objects a namespace-level usage's value binds it to, read once for the run;
// a value depending on a usage still being bound yields none yet.
func (ec *EvalContext) boundObjects(sym *symbols.Symbol) ([]*Instance, error) {
	val, err := NewEvalContext(ec.ctx, sym.OwnerScope).declaredValue(sym, sym.Decl.(*ast.Usage).Value)
	if err != nil {
		var cycle *CyclicBindingError
		if errors.As(err, &cycle) && ec.ctx.binding(cycle.Usage) {
			return nil, nil
		}
		return nil, fmt.Errorf("usage %s: %w", symbolText(sym), err)
	}
	var out []*Instance
	for _, id := range heldObjects(val) {
		if inst, live := ec.ctx.instances[id]; live {
			out = append(out, inst)
		}
	}
	return out, nil
}

// undenotedUsage refuses an extent for a namespace usage the run denotes no object of: a
// port, which stands for an interaction point of an object the namespace has none of.
func (ctx *Context) undenotedUsage(sym *symbols.Symbol) error {
	return fmt.Errorf("%w: usage %s is a %s at namespace level, which the run denotes no object of",
		ErrExtentUnavailable, symbolText(sym), sym.Notation())
}

// namespaceUsages is the object usages the namespaces enclosing scope declare, innermost first,
// whether the run denotes an object of each or not; a variation stands for none.
func (ctx *Context) namespaceUsages(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	for ; scope != nil; scope = scope.Parent() {
		if !namespaceScope(scope) {
			continue
		}
		ctx.noteNamespaceRead(scope)
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if ctx.model.semantics.IsVariationFeature(sym) {
				return true
			}
			if objectFeature(sym) || ctx.namesOneObject(sym) {
				out = append(out, sym)
			}
			return true
		})
	}
	return out
}

// namespaceObjectUsage reports whether sym is an object-holding usage (ports included) a
// namespace declares with a value: the value binds it to the objects it stands for.
func namespaceObjectUsage(sym *symbols.Symbol) bool {
	if !objectFeature(sym) || sym.OwnerScope == nil || !namespaceScope(sym.OwnerScope) {
		return false
	}
	return sym.Decl.(*ast.Usage).Value != nil
}

// namespaceScope reports whether scope is a package, a namespace or a document root, rather
// than the body of a type or a body's locals.
func namespaceScope(scope *symbols.Scope) bool {
	if scope.BodyLocal() {
		return false
	}
	if scope.Owner() == nil {
		return scope.Parent() == nil
	}
	switch scope.Owner().Decl.(type) {
	case *ast.Package, *ast.Namespace:
		return true
	}
	return false
}

// givenValue is the value an object usage is given, whose objects are what it yields.
func (ctx *Context) givenValue(sym *symbols.Symbol) (ast.Node, bool) {
	if !objectFeature(sym) {
		return nil, false
	}
	value := sym.Decl.(*ast.Usage).Value
	return value, value != nil
}

// createdTypes is the declarations the objects an unread feature comes to hold would be of: the
// composite it materializes, or the types its value results in (its declared type, where unknown).
func (ctx *Context) createdTypes(feature *EffectiveFeature) []*symbols.Symbol {
	ctx.noteTypeRead(feature.Symbol)
	if composite := ctx.CompositeTypeOf(feature); composite != nil {
		return []*symbols.Symbol{composite}
	}
	value, valued := ctx.givenValue(feature.Symbol)
	if !valued {
		return nil
	}
	if types := ctx.model.semantics.ExprResultTypes(feature.Symbol.OwnerScope, value); len(types) > 0 {
		return types
	}
	if declared := ctx.extractType(feature.Symbol); declared != nil {
		return []*symbols.Symbol{declared}
	}
	return nil
}

// mayHold reports whether an object of typ, or one a feature of it holds however deep, may be
// of target. Each type is descended into once, so recursive composition ends.
func (ctx *Context) mayHold(typ, target *symbols.Symbol, visited map[*symbols.Symbol]bool) bool {
	if typ == nil || visited[typ] {
		return false
	}
	visited[typ] = true
	if ctx.modelConforms(typ, target) {
		return true
	}
	if value, valued := ctx.givenValue(typ); valued {
		declared := ctx.extractType(typ)
		if declared != nil && ctx.modelConforms(target, declared) {
			return true
		}
		types := ctx.model.semantics.ExprResultTypes(typ.OwnerScope, value)
		if len(types) == 0 {
			return true
		}
		for _, valueType := range types {
			if ctx.modelConforms(target, valueType) || ctx.mayHold(valueType, target, visited) {
				return true
			}
		}
		if declared == nil {
			return false
		}
	}
	for _, member := range ctx.model.semantics.MembersOf(typ) {
		if objectFeature(member) && ctx.mayHold(member, target, visited) {
			return true
		}
	}
	return false
}

// isOf reports whether a type inst is of, or was classified by, conforms to target.
func (ctx *Context) isOf(inst *Instance, target *symbols.Symbol) bool {
	for _, typ := range inst.types() {
		if ctx.modelConforms(typ, target) {
			return true
		}
	}
	return false
}

// declaredBefore orders declarations as their documents, then their positions in them, do.
func declaredBefore(a, b *symbols.Symbol) bool {
	if a == nil || b == nil {
		return a == nil && b != nil
	}
	if a.DocName != b.DocName {
		return a.DocName < b.DocName
	}
	return a.DeclSpan.Offset < b.DeclSpan.Offset
}
