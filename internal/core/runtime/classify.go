package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// instanceConforms reports whether an object is an instance of typ by its declaration or by
// a feature it was held as a value of (KerML 1.0 §7.3.4.1: a feature's values are instances of its types).
func (ctx *Context) instanceConforms(inst *Instance, typ *symbols.Symbol) bool {
	// All of the object's types at once: a difference reads the types it subtracts too.
	return ctx.model.semantics.ClassifiesTypes(inst.types(), typ) == semantics.ClassifiesAll
}

// isDirectTypeOf reports whether typ is already a direct type of an object: one it was
// declared by or held by, or a type one of those is typed by when typ declares no more.
func (ctx *Context) isDirectTypeOf(inst *Instance, typ *symbols.Symbol) bool {
	direct := ctx.directType(typ)
	return slices.ContainsFunc(inst.types(), func(t *symbols.Symbol) bool {
		return symbols.SameElement(t, typ) || (direct == typ && ctx.directType(t) == direct)
	})
}

// canClassify reports whether an object may be held by a feature typed by typ: it is one
// already, or every type classifying it is comparable with typ, so holding it narrows it.
func (ctx *Context) canClassify(inst *Instance, typ *symbols.Symbol) bool {
	if ctx.instanceConforms(inst, typ) {
		return true
	}
	if !ctx.comparableTypes(inst.Type, typ, map[*symbols.Symbol]bool{}) {
		return false
	}
	for _, c := range inst.classifiers {
		if !ctx.comparableTypes(c, typ, map[*symbols.Symbol]bool{}) {
			return false
		}
	}
	return true
}

// comparableTypes reports whether one of typ and other specializes the other. A
// feature stands for the types it is typed by, implicit base included.
func (ctx *Context) comparableTypes(typ, other *symbols.Symbol, seen map[*symbols.Symbol]bool) bool {
	if typ == nil || other == nil || ctx.model.semantics.Conforms(typ, other) || ctx.model.semantics.Conforms(other, typ) {
		return true
	}
	if !semantics.IsShapeFeature(typ) || seen[typ] {
		return false
	}
	seen[typ] = true
	for _, super := range ctx.model.semantics.DirectSupertypes(typ) {
		if !ctx.comparableTypes(super, other, seen) {
			return false
		}
	}
	return true
}

// classifyHeld classifies every object held as a value of feature by the feature itself,
// so it carries the features its type and body declare; the caller has checked each may be held.
// The value is classified whole: one object refused leaves every object as it was.
func (ctx *Context) classifyHeld(feature *symbols.Symbol, val Value) error {
	if feature == nil {
		return nil
	}
	commit, rollback := ctx.beginJournal()
	for _, el := range elementsOf(val) {
		id, ok := el.Object()
		if !ok {
			continue
		}
		inst, ok := ctx.instances[id]
		if !ok || inst == nil || inst.Type == nil {
			continue
		}
		if err := ctx.classify(inst, feature); err != nil {
			rollback()
			return err
		}
	}
	commit()
	return nil
}

// classify records typ as a classifier of inst with the features and behaviors it adds; a
// type the object already conforms to adds nothing and is recorded as a direct type alone.
// It is one transaction: a failure, or a probe rolling it back, leaves the object, what its
// behaviors wrote, the bus and the objects they made as they were.
func (ctx *Context) classify(inst *Instance, typ *symbols.Symbol) error {
	if ctx.isDirectTypeOf(inst, typ) {
		return nil
	}
	inherited := ctx.instanceConforms(inst, typ)
	commit, rollback := ctx.beginJournal()
	classifiers, values, running := inst.classifiers, maps.Clone(inst.FeatureValues), len(inst.behaviors)
	ctx.noteProbeUndo(func() {
		if len(inst.behaviors) > running {
			ctx.forgetBehaviors(inst.behaviors[running:])
		}
		inst.classifiers, inst.FeatureValues = classifiers, values
	})
	inst.classifiers = append(inst.classifiers, typ)
	if inherited {
		commit()
		return nil
	}
	carried := make(map[string]bool, len(inst.FeatureValues))
	for name := range inst.FeatureValues {
		carried[name] = true
	}
	features := ctx.FeaturesOf(typ)
	for i := range features {
		feat := &features[i]
		if !carried[feat.Name] {
			inst.FeatureValues[feat.Name] = ctx.newFeatureValue(inst, feat)
			continue
		}
		if err := ctx.refineFeatureValue(inst, inst.FeatureValues[feat.Name], feat, typ); err != nil {
			rollback()
			return err
		}
	}
	ctx.unfoldSubsettedDefaults(inst, typ, features)
	if err := ctx.aliasRedefinedFeatureValuesOf(inst, typ, carried); err != nil {
		rollback()
		return err
	}
	if err := ctx.startClassifierBehaviors(inst, len(ctx.created)); err != nil {
		rollback()
		return err
	}
	commit()
	return nil
}

// refineFeatureValue makes a carried feature value read the classifier's declaration when it redefines the
// one read (KerML 1.0 §7.3.4.5), or the classifier specializes the type declaring it and so masks it (§7.3.2.1).
func (ctx *Context) refineFeatureValue(inst *Instance, fv *FeatureValue, feat *EffectiveFeature, typ *symbols.Symbol) error {
	have := fv.Feature
	if feat.Symbol == nil || have.Symbol == nil || feat.Symbol == have.Symbol {
		return nil
	}
	if !slices.Contains(ctx.redefinedFeatures(feat.Symbol, typ), have.Symbol) &&
		(!ctx.model.semantics.Conforms(typ, have.OwnerType) || slices.Contains(ctx.redefinedFeatures(have.Symbol, have.OwnerType), feat.Symbol)) {
		return nil
	}
	ctx.noteProbeWrite(fv)
	if !fv.Materialized || (!fv.Written && feat.DefaultValue != have.DefaultValue) {
		ctx.invalidateDependents(fv)
		ctx.initFeatureValue(inst, fv, feat)
		return nil
	}
	how := admitDeclared
	if fv.Written {
		how = admitWritten
	}
	held := fv.HeldValue()
	if err := ctx.checkAdmits(feat, fmt.Sprintf("feature value %s.%s", inst.Type.Name, feat.Name), &held, how); err != nil {
		return err
	}
	val, err := ctx.admitted(feat, held, how)
	if err != nil {
		return err
	}
	fv.Feature, fv.Value, fv.Values = feat, Value{}, Value{}
	if feat.Scalar() {
		fv.Value = val
	} else {
		fv.Values = val
	}
	return nil
}

// declaredBy gathers what each of an object's types declares for it, in type order and once
// per declaring scope: a classifier adds only what the types before it do not declare.
func declaredBy[T any](ctx *Context, types []*symbols.Symbol, of func(*symbols.Symbol) []T, scopeOf func(T) *symbols.Scope) []T {
	if len(types) == 1 {
		return of(types[0])
	}
	covered := map[*symbols.Scope]bool{}
	var out []T
	for _, typ := range types {
		for _, rel := range of(typ) {
			if scope := scopeOf(rel); scope == nil || !covered[scope] {
				out = append(out, rel)
			}
		}
		covered[declScope(typ)] = true
		for _, sup := range ctx.model.semantics.AllSupertypes(typ) {
			covered[declScope(sup)] = true
		}
	}
	return out
}

// bindingsOf returns the binding connectors of inst's types involving the named feature.
func (ctx *Context) bindingsOf(inst *Instance, name string) []lower.Binding {
	return declaredBy(ctx, inst.types(),
		func(typ *symbols.Symbol) []lower.Binding { return ctx.bindingsForFeature(typ, name) },
		func(b lower.Binding) *symbols.Scope { return b.Scope })
}

// connectionsOf returns the connections inst owns through each of its types.
func (ctx *Context) connectionsOf(inst *Instance) []lower.Connection {
	return declaredBy(ctx, inst.types(), ctx.objectConnections,
		func(c lower.Connection) *symbols.Scope { return c.Scope })
}

// anonymousConnectorsOf returns the unnamed connector usages an object of types owns.
func (ctx *Context) anonymousConnectorsOf(types []*symbols.Symbol) []*symbols.Symbol {
	return declaredBy(ctx, types, ctx.anonymousConnectors,
		func(sym *symbols.Symbol) *symbols.Scope { return sym.OwnerScope })
}

// subsettingFeaturesOf returns the features of inst subsetting the named one through any
// of its types, in declaration order, each once.
func (ctx *Context) subsettingFeaturesOf(inst *Instance, name string) []EffectiveFeature {
	types := inst.types()
	if len(types) == 1 {
		return ctx.SubsettingFeatures(inst, types[0], name)
	}
	seen := map[string]bool{}
	var out []EffectiveFeature
	for _, typ := range types {
		for _, feat := range ctx.SubsettingFeatures(inst, typ, name) {
			if !seen[feat.Name] {
				seen[feat.Name] = true
				out = append(out, feat)
			}
		}
	}
	return out
}

// subsettedNamesOf returns the features of inst that its feature sym subsets, through any of its types.
func (ctx *Context) subsettedNamesOf(inst *Instance, sym *symbols.Symbol) []string {
	types := inst.types()
	if len(types) == 1 {
		return ctx.subsettedNames(sym, types[0])
	}
	seen := map[string]bool{}
	var out []string
	for _, typ := range types {
		for _, name := range ctx.subsettedNames(sym, typ) {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}
