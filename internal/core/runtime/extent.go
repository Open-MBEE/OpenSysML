package runtime

import (
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
	target := sem.ExtentType(ec.scope, n)
	if target == nil {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType, qualifiedNameToString(qn))
	}
	switch {
	case target.Kind == symbols.SymbolEnumerationDef:
		return ec.literalValues(sem.LiteralsOf(target))
	case sem.IsVariationFeature(target):
		return ec.variantValues(target, sem.VariantsOf(target))
	case sem.IsDataType(target):
		return Value{}, fmt.Errorf("%w: %s declares no values to enumerate",
			ErrUnboundedExtent, qualifiedNameToString(qn))
	}
	roots, err := ec.extentRoots(target)
	if err != nil {
		return Value{}, err
	}
	return ec.ctx.objectsOf(roots, target)
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
	return sequenceOf(values), nil
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
	return sequenceOf(values), nil
}

// objectsOf is this run's objects of target under roots: each root, then what its features
// hold in declaration order; reading a feature materializes it, so unread usages are reached too.
func (ctx *Context) objectsOf(roots []*Instance, target *symbols.Symbol) (Value, error) {
	var values []Value
	seen := make(map[int64]bool)
	path := make(map[*symbols.Symbol]bool)
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
		if inst.Type != nil {
			if path[inst.Type] {
				return nil
			}
			path[inst.Type] = true
			defer delete(path, inst.Type)
		}
		for _, child := range ctx.nestedObjects(inst) {
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
	return sequenceOf(values), nil
}

// extentRoots is the objects an extent is searched from, in declaration order: those the run
// materialized standing on their own (not held, not read through), the outermost holder of the
// object evaluating, and the occurrences the enclosing namespace declares that may hold a
// target — materialized now, as reading them would be.
func (ec *EvalContext) extentRoots(target *symbols.Symbol) ([]*Instance, error) {
	ctx := ec.ctx
	var roots []*Instance
	seen := make(map[int64]bool)
	add := func(inst *Instance) {
		if inst != nil && !seen[inst.ID] {
			seen[inst.ID] = true
			roots = append(roots, inst)
		}
	}
	for top := ec.self; top != nil; top = top.owner {
		if top.owner == nil {
			add(top)
		}
	}
	for _, sym := range ctx.namespaceOccurrences(ec.scope) {
		if !ctx.mayHold(sym, target, make(map[*symbols.Symbol]bool)) {
			continue
		}
		inst, err := ctx.occurrenceOf(sym)
		if err != nil {
			return nil, fmt.Errorf("usage %s: %w", symbolText(sym), err)
		}
		add(inst)
	}
	held := ctx.heldObjectIDs()
	for _, id := range ctx.created {
		inst, live := ctx.instances[id]
		if !live || held[id] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		add(inst)
	}
	sort.SliceStable(roots, func(i, j int) bool { return declaredBefore(roots[i].Type, roots[j].Type) })
	return roots, nil
}

// namespaceOccurrences is the usages denoting one object each that the namespace enclosing
// scope declares, in declaration order — the objects a value written there reaches by name.
func (ctx *Context) namespaceOccurrences(scope *symbols.Scope) []*symbols.Symbol {
	for scope != nil && typeScope(scope) {
		scope = scope.Parent()
	}
	if scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if ctx.namesOneObject(sym) {
			out = append(out, sym)
		}
		return true
	})
	return out
}

// typeScope reports whether scope is the body of a definition or usage rather than a namespace.
func typeScope(scope *symbols.Scope) bool {
	if scope.Owner() == nil {
		return false
	}
	switch scope.Owner().Decl.(type) {
	case *ast.Definition, *ast.Usage:
		return true
	}
	return false
}

// mayHold reports whether an object of typ, or one a feature of it holds however deep, may be
// of target. Each type is descended into once, so recursive composition ends.
func (ctx *Context) mayHold(typ, target *symbols.Symbol, visited map[*symbols.Symbol]bool) bool {
	if typ == nil || visited[typ] {
		return false
	}
	visited[typ] = true
	if ctx.model.semantics.Conforms(typ, target) {
		return true
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
		if ctx.model.semantics.Conforms(typ, target) {
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
