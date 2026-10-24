package runtime

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// MemberValue reads the member of the object that name denotes, as every surface
// reads one: a feature value of its shape, else a member the shape leaves out
// through kernelMemberRead. GetFeatureValue alone probes the shape.
func (inst *Instance) MemberValue(ctx *Context, name string) (*FeatureValue, error) {
	fv, err := inst.GetFeatureValue(ctx, name)
	if err == nil || !errors.Is(err, ErrNoSuchFeature) {
		return fv, err
	}
	if _, has := inst.FeatureValues[name]; has {
		return fv, err
	}
	if kfv, answered, kerr := ctx.kernelMemberRead(inst, name, nil); answered {
		return kfv, kerr
	}
	return fv, err
}

// kernelMemberRead answers a read of a member the object's shape leaves out: a
// name that is no member of the object's type is refused naming the type, and a
// Kernel Semantic Library member is answered from what the runtime tracks, the
// library's own declared value, the composition it inverts, or its multiplicity —
// empty where the lower bound is 0, undetermined where it is not. The second
// result is false for a member the rule does not cover, which the caller reports.
// A non-nil open is a model-level read, which makes up no open subsetter's count.
func (ctx *Context) kernelMemberRead(inst *Instance, name string, open *openPopulation) (*FeatureValue, bool, error) {
	if ctx == nil || inst == nil || ctx.model.semantics == nil {
		return nil, false, nil
	}
	sym, found := ctx.model.semantics.LookupMember(inst.Type, name)
	if !found || sym == nil {
		if redefining := ctx.redefiningMember(inst.Type, name); redefining != nil {
			return nil, true, fmt.Errorf("%w: %s has no member %s: %s redefines it", ErrNoSuchFeature, SymbolText(inst.Type), name, ctx.QualifiedName(redefining))
		}
		return nil, true, fmt.Errorf("%w: %s has no member %s", ErrNoSuchFeature, SymbolText(inst.Type), name)
	}
	if !sym.IsFeature() {
		return nil, true, fmt.Errorf("%w: %s has no feature %s (%s is not a feature)", ErrNoSuchFeature, SymbolText(inst.Type), name, name)
	}
	if resultParameter(sym) && ctx.model.semantics.FrameFeature(sym) {
		feat := ctx.effectiveFeature(name, sym, inst.Type)
		fv, err := ctx.returnParameterRead(inst, sym, &feat)
		return fv, true, err
	}
	if !ctx.kernelFrameMember(sym) {
		return nil, false, nil
	}
	feat := ctx.effectiveFeature(name, sym, inst.Type)
	val, stopped, err := ctx.kernelMemberValue(inst, sym, &feat, open)
	if err != nil {
		return nil, true, err
	}
	fv := &FeatureValue{Feature: &feat, Materialized: !stopped}
	if feat.Scalar() || val.Kind == ValUndetermined {
		fv.Value = val
	} else {
		fv.Values = val
	}
	return fv, true, nil
}

// redefiningMember is the member of typ that redefines an inherited feature named
// name under another name (`Items::Item::start :>> startShot`), which masks the
// redefined name (KerML 8.3.3.3.6); nil when none does.
func (ctx *Context) redefiningMember(typ *symbols.Symbol, name string) *symbols.Symbol {
	for _, m := range ctx.model.semantics.MembersOf(typ) {
		for _, redefined := range ctx.redefinedFeatures(m, typ) {
			if redefined != nil && redefined.Name == name {
				return m
			}
		}
	}
	return nil
}

// kernelFrameMember reports a member the shape leaves out for framing the object:
// one the Kernel Semantic Library declares, or a restatement of one of its roots
// (`Parts::Part::self :>> Item::self`); parameters and values held by value are not.
func (ctx *Context) kernelFrameMember(sym *symbols.Symbol) bool {
	sem := ctx.model.semantics
	return sem.FrameFeature(sym) && !semantics.IsParameter(sym) && !sem.HeldByValue(sym)
}

// kernelMemberValue is the value of the Kernel member sym on inst, in the order
// the rule tries: tracked, declared by the library, inverted through composition,
// then what the multiplicity alone says. The second result reports a model-level
// read stopped by an open subsetter, recorded in open.
func (ctx *Context) kernelMemberValue(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature, open *openPopulation) (Value, bool, error) {
	if val, stopped, ok, err := ctx.trackedKernelValue(inst, sym, feat, open); ok {
		return val, stopped, err
	}
	if context := ctx.performanceContextOf(inst, sym); context != nil {
		return Value{Kind: ValInstance, Instance: context.ID}, false, nil
	}
	if val, ok, err := ctx.kernelDeclaredValue(inst, feat); ok {
		return val, false, nil
	} else if err != nil {
		// A declared value the evaluator cannot run leaves the member open, not empty.
		return undeterminedFeatureValue(fmt.Sprintf("%s declares a value the evaluator cannot run: %v", sym.Name, err), feat.Multiplicity, sym), false, nil
	}
	if val, ok := ctx.kernelInverseValue(inst, sym, feat); ok {
		return val, false, nil
	}
	if lower := feat.Multiplicity.Lower; lower.Known && !lower.Infinite && lower.Value == 0 {
		if feat.Scalar() {
			return Value{Kind: ValNull}, false, nil
		}
		return ctx.collectionOf(feat, nil), false, nil
	}
	return unsetFeatureValue(sym.Name, feat.Multiplicity, sym), false, nil
}

// trackedKernelValue answers the members the runtime tracks for every object:
// `self` is the object itself (Base.kerml `Anything::self`), and a collection
// `self` subsets — `timeSlices`, `spaceSlices`, `portions`, `timeEnclosedOccurrences`, …
// (Occurrences.kerml) — holds the object, since an occurrence the runtime has recorded
// no portions of is its own only portion, unioned with what the shape features
// subsetting the collection hold (`ownedPorts :> timeEnclosedOccurrences`, KerML 1.0
// §7.3.4.4): the object first, then the contributions in shape order, each once.
// Transfers are tracked under no Kernel name, so none is here.
func (ctx *Context) trackedKernelValue(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature, open *openPopulation) (val Value, stopped, ok bool, err error) {
	self, found := ctx.model.semantics.LookupMember(inst.Type, "self")
	if !found || self == nil {
		return Value{}, false, false, nil
	}
	if sym == self || ctx.model.semantics.IsSelf(sym) {
		val, err := ctx.objectValue(inst)
		return val, false, true, err
	}
	if feat.Scalar() {
		if val, ok, err := ctx.shotValue(inst, sym, feat); ok {
			return val, false, true, err
		}
		val, ok, err := ctx.transferEndValue(inst, sym)
		return val, false, ok, err
	}
	if val, ok := ctx.timeOrderOpen(inst, sym, feat); ok {
		return val, false, true, nil
	}
	var held []Value
	if ctx.subsetsTransitively(self, inst.Type, sym) {
		val, err := ctx.objectValue(inst)
		if err != nil {
			return Value{}, false, true, err
		}
		held = append(held, val)
	}
	var shots []Value
	if ctx.shotsCollection(sym) {
		if shots, err = ctx.happenedShots(inst); err != nil {
			return Value{}, false, true, err
		}
		if len(held) == 0 && len(shots) == 0 {
			return undeterminedFeatureValue(fmt.Sprintf("%s has not started: no snapshot of it has happened", SymbolText(inst.Type)), feat.Multiplicity, sym), false, true, nil
		}
	} else if fired, ok, err := ctx.recordedValues(inst, sym); ok {
		if err != nil {
			return Value{}, false, true, err
		}
		held = append(held, fired...)
		if len(held) == 0 {
			return ctx.collectionOf(feat, nil), false, true, nil
		}
	} else if len(held) == 0 {
		return Value{}, false, false, nil
	}
	values, stopped, err := ctx.shapeContributions(inst, sym, feat, held, open)
	if err != nil {
		return Value{}, false, true, err
	}
	// The snapshots follow the object and what its shape features hold.
	for _, shot := range shots {
		if !slices.ContainsFunc(values, func(v Value) bool { return v.Kind == ValInstance && v.Instance == shot.Instance }) {
			values = append(values, shot)
		}
	}
	if ctx.shotsCollection(sym) {
		// Its startShot and endShot [1] exist and have not happened (Occurrences.kerml `snapshots`).
		switch l := ctx.lives[inst.ID]; {
		case l.began == 0:
			return openKernelCollection(feat, sym, values, fmt.Sprintf("%s has not started: its startShot has not happened", SymbolText(inst.Type))), stopped, true, nil
		case len(shots) > 0 && l.alive():
			return openKernelCollection(feat, sym, values, fmt.Sprintf("%s has not ended: its endShot has not happened", SymbolText(inst.Type))), stopped, true, nil
		}
	}
	return ctx.collectionOf(feat, values), stopped, true, nil
}

// openKernelCollection is a collection member the run has not closed: undetermined,
// certainly holding known and at least one value more.
func openKernelCollection(feat *EffectiveFeature, sym *symbols.Symbol, known []Value, reason string) Value {
	count := feat.Multiplicity
	if fewest := int64(len(known)) + 1; fewest > fewestOf(count) {
		count.Lower = semantics.Bound{Value: fewest, Known: true}
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{reason: reason, count: count, known: known, positions: known, feature: sym}}
}

// kernelDeclaredValue evaluates the value the library declares for the member —
// `localClock default universalClock`, `this default self` — on the object through
// the evaluator, as a shape feature's default is; false where it declares none,
// and the error where the evaluator cannot run it.
func (ctx *Context) kernelDeclaredValue(inst *Instance, feat *EffectiveFeature) (Value, bool, error) {
	if feat.DefaultValue == nil {
		return Value{}, false, nil
	}
	val, err := ctx.evalFeatureValueDefault(inst, &FeatureValue{Feature: feat}, feat.Name)
	if err != nil {
		return Value{}, false, err
	}
	if val.Kind == ValInvalid {
		return Value{}, false, fmt.Errorf("no value")
	}
	return val, true, nil
}

// The Kernel collections whose holders redefine the context feature `this` of what they hold.
const (
	ownedPerformancesFQN = "Objects::Object::ownedPerformances"
	subperformancesFQN   = "Performances::Performance::subperformances"
)

// performanceContextOf is the object the context feature `this` of a performance denotes
// when the feature holding it redefines the default: the holder for one held under
// `ownedPerformances` (Objects.kerml `this default that`), the holder's own context for one
// held under `subperformances` (Performances.kerml `this default (that as Performance).this`);
// nil for any other member or an occurrence held neither way, whose `this` defaults to self.
func (ctx *Context) performanceContextOf(inst *Instance, sym *symbols.Symbol) *Instance {
	if ctx.model.resolver == nil || !ctx.model.resolver.IsOccurrenceThis(sym) {
		return nil
	}
	return ctx.performanceContext(inst, map[*Instance]bool{})
}

func (ctx *Context) performanceContext(inst *Instance, seen map[*Instance]bool) *Instance {
	owner, held := inst.Owner()
	if owner == nil || seen[inst] {
		return nil
	}
	seen[inst] = true
	holding, ok := owner.FeatureValues[held]
	if !ok || holding.Feature == nil || holding.Feature.Symbol == nil {
		return nil
	}
	if sub := ctx.librarySymbol(subperformancesFQN); sub != nil && ctx.subsetsTransitively(holding.Feature.Symbol, owner.Type, sub) {
		if context := ctx.performanceContext(owner, seen); context != nil {
			return context
		}
		return owner
	}
	if owned := ctx.librarySymbol(ownedPerformancesFQN); owned != nil && ctx.subsetsTransitively(holding.Feature.Symbol, owner.Type, owned) {
		return owner
	}
	return nil
}

// kernelInverseValue answers a member declared `inverse of` a collection X from
// what the runtime knows holds the object in X: the object itself when `self`
// subsets X (`portionOf inverse of portions`, since `self :> timeSlices :> portions`),
// and the object holding inst when X is composite and the holding feature subsets
// it (`superoccurrence inverse of suboccurrences`). A member whose X is composite
// and holds the object nowhere is null; anything else is left to the multiplicity.
func (ctx *Context) kernelInverseValue(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, bool) {
	var values []Value
	composite := false
	owner, held := inst.Owner()
	self, hasSelf := ctx.model.semantics.LookupMember(inst.Type, "self")
	for _, rel := range relationshipsOfKind(sym, ast.RelInverseOf) {
		target := ctx.model.semantics.RelationshipTarget(sym, rel)
		if target == nil {
			continue
		}
		if hasSelf && ctx.subsetsTransitively(self, inst.Type, target) {
			values = append(values, Value{Kind: ValInstance, Instance: inst.ID})
		}
		if occurrence, isShot := ctx.shotOccurrence(inst.ID); isShot && ctx.shotsCollection(target) {
			// A shot is a snapshot, time slice and portion of the occurrence it was handed out for.
			values = append(values, Value{Kind: ValInstance, Instance: occurrence})
		}
		if !compositeFeature(target) {
			continue
		}
		composite = true
		if owner == nil {
			continue
		}
		if holding, ok := owner.FeatureValues[held]; ok && holding.Feature != nil && holding.Feature.Symbol != nil &&
			ctx.subsetsTransitively(holding.Feature.Symbol, owner.Type, target) {
			values = append(values, Value{Kind: ValInstance, Instance: owner.ID})
		}
	}
	switch {
	case len(values) > 0 && feat.Scalar():
		return values[0], true
	case len(values) > 0:
		return ctx.collectionOf(feat, values), true
	case composite && feat.Scalar():
		return Value{Kind: ValNull}, true
	case composite:
		return ctx.collectionOf(feat, nil), true
	}
	return Value{}, false
}

// compositeFeature reports a feature declared `composite`, whose values an
// object holds as its parts.
func compositeFeature(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsComposite
}
