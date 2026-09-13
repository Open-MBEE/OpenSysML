package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// This file materializes the two ways a feature of an object takes its values
// from another feature of the same object.
//
// Subsetting: the values of a subsetting feature are always values of the
// feature it subsets (KerML 1.0 §7.3.4.4), so a nested part declared
// `part a : Sub :> subsystem` is one of the objects the `subsystem` collection
// holds. That is what a roll-up over an unbounded feature sums over.
//
// Redefinition: a redefining feature is the same feature as the one it
// redefines, seen through a new declaration (KerML 1.0 §7.3.4.5), so
// the two names read one feature value rather than two — `part subsystems :>> Subsystems`
// makes `Subsystems.mass` read the values held under `subsystems`.

// relatedFeatures returns the features of owner that sym's relationships of the given kind name:
// the resolved target, or owner's own same-named declaration masking it; an unresolved name is looked up on owner.
// A declaration that itself restates sym (`:>> elements` on an object, walked past
// its library chain) masks nothing on that walk: the target stays the resolved one.
func (ctx *Context) relatedFeatures(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []*symbols.Symbol {
	var features []*symbols.Symbol
	for _, rel := range relationshipsOfKind(sym, kind) {
		qn := ast.AsQualifiedName(rel.Target)
		resolved := ctx.model.semantics.RelationshipTarget(sym, rel)
		if resolved != nil && resolved != sym {
			if ctx.isFeatureOf(owner, resolved, sym) {
				features = append(features, resolved)
				continue
			}
			if !ctx.inheritsDeclaration(owner, resolved) {
				continue
			}
			if own, declared := ctx.ownDeclarationNamed(owner, sym, qn.Parts[len(qn.Parts)-1].Text, resolved.Name, resolved.ShortName); declared && !ctx.redefinesTransitively(own, sym) {
				features = append(features, own)
			} else {
				features = append(features, resolved)
			}
			continue
		}
		if len(qn.Parts) != 1 {
			continue
		}
		if member, found := ctx.model.semantics.LookupMember(owner, qn.Parts[0].Text); found && member != nil && member != sym {
			features = append(features, member)
		}
	}
	return features
}

// redefinesTransitively reports whether sym redefines feature, directly or through
// the features those redefine.
func (ctx *Context) redefinesTransitively(sym, feature *symbols.Symbol) bool {
	for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(sym) {
		if redefined == feature {
			return true
		}
	}
	return false
}

// inheritsDeclaration reports whether owner takes its members from the type declaring feature.
func (ctx *Context) inheritsDeclaration(owner, feature *symbols.Symbol) bool {
	if feature.OwnerScope == nil {
		return false
	}
	for _, src := range ctx.model.semantics.MemberSources(owner) {
		if src.Scope == feature.OwnerScope {
			return true
		}
	}
	return false
}

// ownDeclarationNamed returns the first member other than sym that owner itself
// declares under one of names (a feature's written, primary or short name), not one it inherits.
func (ctx *Context) ownDeclarationNamed(owner, sym *symbols.Symbol, names ...string) (*symbols.Symbol, bool) {
	for _, name := range names {
		if name == "" {
			continue
		}
		member, found := ctx.model.semantics.LookupMember(owner, name)
		if !found || member == nil || member == sym {
			continue
		}
		if contributed, ok := ctx.model.semantics.LookupContributedMember(owner, name); ok && contributed == member {
			continue
		}
		return member, true
	}
	return nil, false
}

// relationshipsOfKind returns sym's relationships of the given kind that name a target.
func relationshipsOfKind(sym *symbols.Symbol, kind ast.RelationshipKind) []*ast.Relationship {
	var rels []*ast.Relationship
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind || rel.Target == nil {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil && len(qn.Parts) > 0 {
			rels = append(rels, rel)
		}
	}
	return rels
}

// isFeatureOf reports whether owner carries feature under its name, as its own
// declaration or through what it inherits. A declaration restating a feature
// (`attribute :>> own`) masks the feature it restates under that name, so the
// masked feature is still a feature of the owner and masking names it.
func (ctx *Context) isFeatureOf(owner, feature, masking *symbols.Symbol) bool {
	carries := func(member *symbols.Symbol, ok bool) bool {
		return ok && (member == feature || member == masking)
	}
	if carries(ctx.model.semantics.LookupMember(owner, feature.Name)) {
		return true
	}
	return carries(ctx.model.semantics.LookupContributedMember(owner, feature.Name))
}

// relatedFeatureNames is relatedFeatures by name.
func (ctx *Context) relatedFeatureNames(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []string {
	features := ctx.relatedFeatures(sym, owner, kind)
	names := make([]string, 0, len(features))
	for _, feat := range features {
		names = append(names, feat.Name)
	}
	return names
}

// aliasRedefinedFeatureValuesOf makes every name a redefinition chain gives one feature read one
// feature value (the most specific valued declaration's); two valued names of it is an error.
// A value carried under one of the names is kept, refined by that declaration.
func (ctx *Context) aliasRedefinedFeatureValuesOf(inst *Instance, typ *symbols.Symbol, carried map[string]bool) error {
	features := ctx.FeaturesOf(typ)
	byName := make(map[string]*EffectiveFeature, len(features))
	for i := range features {
		byName[features[i].Name] = &features[i]
	}

	for _, names := range ctx.redefinitionGroups(typ) {
		chosen, err := ctx.sharedRedefinitionName(inst, byName, names)
		if err != nil {
			return err
		}
		fv := inst.FeatureValues[chosen]
		for _, name := range names {
			if carried[name] {
				fv = inst.FeatureValues[name]
				if err := ctx.refineFeatureValue(inst, fv, byName[chosen], typ); err != nil {
					return err
				}
				break
			}
		}
		for _, name := range names {
			inst.FeatureValues[name] = fv
		}
	}
	return nil
}

// redefinitionGroups groups the names of typ's features by the feature they name,
// in feature order, keeping only names that share a feature with another. Each
// group's names are ordered from the most specific declaration to the least,
// which is the order the features are computed in. The answer is memoized per
// type; callers read it and never write it.
func (ctx *Context) redefinitionGroups(typ *symbols.Symbol) [][]string {
	if groups, ok := ctx.model.redefGroups[typ]; ok {
		return groups
	}
	features := ctx.FeaturesOf(typ)
	declared := make(map[string]bool, len(features))
	for _, feat := range features {
		declared[feat.Name] = true
	}
	leader := map[string]string{}
	var find func(string) string
	find = func(name string) string {
		next, ok := leader[name]
		if !ok || next == name {
			leader[name] = name
			return name
		}
		root := find(next)
		leader[name] = root
		return root
	}
	for _, feat := range features {
		if feat.Symbol == nil {
			continue
		}
		for _, redefined := range ctx.redefinedNames(feat.Symbol, typ) {
			if redefined == feat.Name || !declared[redefined] {
				continue
			}
			if a, b := find(feat.Name), find(redefined); a != b {
				leader[b] = a
			}
		}
	}

	var groups [][]string
	index := map[string]int{}
	for _, feat := range features {
		if _, ok := leader[feat.Name]; !ok {
			continue
		}
		root := find(feat.Name)
		if at, ok := index[root]; ok {
			groups[at] = append(groups[at], feat.Name)
			continue
		}
		index[root] = len(groups)
		groups = append(groups, []string{feat.Name})
	}
	ctx.model.redefGroups[typ] = groups
	return groups
}

// redefinitionAliases returns the names under which typ's features read the
// feature called name: name itself and every name redefinition makes one with it.
func (ctx *Context) redefinitionAliases(typ *symbols.Symbol, name string) map[string]bool {
	aliases := map[string]bool{name: true}
	for _, names := range ctx.redefinitionGroups(typ) {
		for _, n := range names {
			if n != name {
				continue
			}
			for _, alias := range names {
				aliases[alias] = true
			}
			return aliases
		}
	}
	return aliases
}

// sharedRedefinitionName returns the name in a group of names of one feature
// whose feature value the group shares: the first name, in most-specific-first order,
// whose own declaration values it — by a value, or by a body valuing the features
// of the value it inherits — and otherwise the most specific name.
// Two names valued by one declaration are ErrConflictingRedefinition.
func (ctx *Context) sharedRedefinitionName(inst *Instance, byName map[string]*EffectiveFeature, names []string) (string, error) {
	valued := ""
	var valuedBy *symbols.Scope
	for _, name := range names {
		feat, ok := byName[name]
		if !ok || feat.Symbol == nil || !ctx.declarationValues(feat) {
			continue
		}
		if valued == "" {
			valued, valuedBy = name, feat.Symbol.OwnerScope
			continue
		}
		if feat.Symbol.OwnerScope == valuedBy {
			return "", fmt.Errorf("%w: %s values %s and %s, which redefinition makes one feature",
				ErrConflictingRedefinition, inst.Type.Name, valued, name)
		}
	}
	if valued != "" {
		return valued, nil
	}
	return names[0], nil
}

// declarationValues reports whether a feature's own declaration values it: it
// writes a value, or its body governs the value it inherits (`datum :>> coordinateFrame
// { :>> mRefs = (mm, mm, mm); }` over `coordinateFrame default universal…`).
func (ctx *Context) declarationValues(feat *EffectiveFeature) bool {
	return ctx.extractDefaultValue(feat.Symbol) != nil || ctx.bodyGovernsInheritedValue(feat)
}

// redefinedNames returns the names of every feature of owner sym redefines,
// directly or through a redefinition of a redefinition: a restated redefinition
// still declares the feature at the end of the chain (KerML 1.0 §7.3.4.5).
func (ctx *Context) redefinedNames(sym, owner *symbols.Symbol) []string {
	redefined := ctx.redefinedFeatures(sym, owner)
	names := make([]string, 0, len(redefined))
	for _, feat := range redefined {
		names = append(names, feat.Name)
	}
	return names
}

// redefinedFeatures returns every feature of owner sym redefines, directly or
// through a redefinition of a redefinition, in breadth-first order.
func (ctx *Context) redefinedFeatures(sym, owner *symbols.Symbol) []*symbols.Symbol {
	key := featureOfType{feature: sym, owner: owner}
	if features, ok := ctx.model.redefined[key]; ok {
		return features
	}
	features := ctx.collectRedefinedFeatures(sym, owner)
	ctx.model.redefined[key] = features
	return features
}

func (ctx *Context) collectRedefinedFeatures(sym, owner *symbols.Symbol) []*symbols.Symbol {
	var features []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		// An end of a connector redefines the end at its position of each
		// connector its owner specializes, and a subject or objective redefines
		// the one its owner inherits, without naming it: the two names read one
		// feature value as an explicit redefinition's do.
		redefines := append(ctx.relatedFeatures(cur, owner, ast.RelRedefines),
			ctx.model.semantics.ImplicitEndRedefinitions(cur)...)
		redefines = append(redefines, ctx.model.semantics.ImplicitRoleRedefinitions(cur)...)
		for _, redefined := range redefines {
			if seen[redefined] {
				continue
			}
			seen[redefined] = true
			features = append(features, redefined)
			queue = append(queue, redefined)
		}
	}
	return features
}

// subsettedNames returns the features of owner sym subsets, itself or through a feature it
// redefines, which subsets what the redefined one subsets (KerML 1.0 §7.3.4.5).
func (ctx *Context) subsettedNames(sym, owner *symbols.Symbol) []string {
	names := ctx.relatedFeatureNames(sym, owner, ast.RelSubsets)
	for _, redefined := range ctx.redefinedFeatures(sym, owner) {
		names = append(names, ctx.relatedFeatureNames(redefined, owner, ast.RelSubsets)...)
	}
	return names
}

// SubsettingFeatures returns the features of typ subsetting the named feature under any
// of its redefinition names, in declaration order, reading nothing; inst is nil for a type alone.
func (ctx *Context) SubsettingFeatures(inst *Instance, typ *symbols.Symbol, name string) []EffectiveFeature {
	aliases := ctx.redefinitionAliases(typ, name)
	var subsetting []EffectiveFeature
	for _, feat := range ctx.FeaturesOf(typ) {
		if aliases[feat.Name] || feat.Symbol == nil {
			continue
		}
		if inst != nil {
			if _, ok := inst.FeatureValues[feat.Name]; !ok {
				continue
			}
		}
		for _, subsetted := range ctx.subsettedNames(feat.Symbol, typ) {
			if aliases[subsetted] {
				subsetting = append(subsetting, feat)
				break
			}
		}
	}
	return subsetting
}

// subsettingContributions returns the values the features subsetting the named
// feature contribute to it, in declaration order. A redefinition shares one feature value
// under two names, so membership is decided by the feature value a subsetted name reads
// rather than by the name this collection was read under. Reading a subsetting
// feature materializes it, so a cycle between subsetting features is reported as
// ErrCyclicFeatureValue rather than recursing until the step budget runs out.
func (ctx *Context) subsettingContributions(inst *Instance, name string) ([]Value, error) {
	var values []Value
	err := ctx.eachSubsetterOf(inst, name, func(feat *EffectiveFeature) error {
		sub, err := inst.GetFeatureValue(ctx, feat.Name)
		if err != nil {
			return err
		}
		values = append(values, elementsOf(sub.HeldValue())...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, err
	}
	return values, nil
}

// openSubsettingContributions is subsettingContributions for a model-level read: an open
// subsetter is not made up, contributing what it certainly holds and its fewest as atLeast.
func (ctx *Context) openSubsettingContributions(inst *Instance, name string) (values []Value, atLeast int64, err error) {
	err = ctx.eachSubsetterOf(inst, name, func(feat *EffectiveFeature) error {
		sub, open, err := inst.openFeatureValue(ctx, feat.Name)
		if err != nil {
			return err
		}
		if !open.Stopped {
			values = append(values, elementsOf(sub.HeldValue())...)
			return nil
		}
		values = append(values, open.Contributed...)
		atLeast = max(atLeast, open.AtLeast, fewestOf(feat.Multiplicity))
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, 0, err
	}
	return values, atLeast, nil
}

// eachSubsetterOf reads each feature subsetting the named feature of inst through
// read, in declaration order; a cycle between subsetting features is an error.
func (ctx *Context) eachSubsetterOf(inst *Instance, name string, read func(feat *EffectiveFeature) error) error {
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.collectingSubsets[key] {
		return fmt.Errorf("%w: %s.%s subsets itself", ErrCyclicFeatureValue, inst.Type.Name, name)
	}
	ctx.collectingSubsets[key] = true
	defer delete(ctx.collectingSubsets, key)

	for _, feat := range ctx.subsettingFeaturesOf(inst, name) {
		if err := read(&feat); err != nil {
			return fmt.Errorf("subsetting feature %s of %s: %w", feat.Name, name, err)
		}
	}
	return nil
}

// fewestOf is the fewest values a feature of multiplicity mult holds.
func fewestOf(mult semantics.Range) int64 {
	if mult.Lower.Known && !mult.Lower.Infinite {
		return mult.Lower.Value
	}
	return 0
}

// fillsFromSubsetted reports whether feat may hold objects a collection it
// subsets makes up: it is optional, holds objects, and states no value.
func (ctx *Context) fillsFromSubsetted(feat *EffectiveFeature) bool {
	lower := feat.Multiplicity.Lower
	return lower.Known && !lower.Infinite && lower.Value == 0 &&
		!ctx.model.semantics.IsConnectorUsage(feat.Symbol) && ctx.CompositeTypeOf(feat) != nil
}

// materializeSubsettedCollections reads the collections an optional feature subsets before the
// feature itself, so what they make up for their lower bounds is held by it whichever is read first.
func (ctx *Context) materializeSubsettedCollections(inst *Instance, fv *FeatureValue) error {
	if !ctx.fillsFromSubsetted(fv.Feature) {
		return nil
	}
	key := featureValueRef{instance: inst.ID, feature: fv.Feature.Name}
	if ctx.readingSubsetted[key] {
		return nil
	}
	ctx.readingSubsetted[key] = true
	defer delete(ctx.readingSubsetted, key)
	for _, name := range ctx.subsettedNamesOf(inst, fv.Feature.Symbol) {
		sub, ok := inst.FeatureValues[name]
		if !ok || sub.Materialized || sub.Feature.Scalar() ||
			ctx.collectingSubsets[featureValueRef{instance: inst.ID, feature: name}] ||
			ctx.readingSubsetted[featureValueRef{instance: inst.ID, feature: name}] {
			continue
		}
		if _, err := inst.GetFeatureValue(ctx, name); err != nil {
			return fmt.Errorf("subsetted feature %s of %s: %w", name, fv.Feature.Name, err)
		}
	}
	return nil
}

// fillOptionalSubsetters makes up to n objects for the optional features subsetting the named
// collection, in declaration order and within their upper bounds; undo puts every filled feature
// back, as does the journal under way (see noteProbeWrite).
func (ctx *Context) fillOptionalSubsetters(inst *Instance, name string, n int) (made []*Instance, undo func(), err error) {
	type filled struct {
		fv     *FeatureValue
		before FeatureValue
		held   []Value
	}
	var fills []filled
	undo = func() {
		for _, fill := range fills {
			*fill.fv = fill.before
		}
	}
	for _, feat := range ctx.subsettingFeaturesOf(inst, name) {
		if n == 0 {
			break
		}
		if !ctx.fillsFromSubsetted(&feat) {
			continue
		}
		fv := inst.FeatureValues[feat.Name]
		held := elementsOf(fv.HeldValue())
		spare := n
		if upper := feat.Multiplicity.Upper; upper.Known && !upper.Infinite {
			if spare = int(upper.Value) - len(held); spare > n {
				spare = n
			}
		}
		if spare <= 0 {
			continue
		}
		composite := ctx.CompositeTypeOf(&feat)
		for i := 0; i < spare; i++ {
			obj, err := ctx.materialize(composite, 0, inst, feat.Name)
			if err != nil {
				return made, undo, err
			}
			made = append(made, obj)
			held = append(held, Value{Kind: ValInstance, Instance: obj.ID})
		}
		fills = append(fills, filled{fv: fv, before: *fv, held: held})
		n -= spare
	}
	for _, fill := range fills {
		ctx.noteProbeWrite(fill.fv)
		if fill.fv.Feature.Scalar() {
			fill.fv.Value = fill.held[0]
		} else {
			fill.fv.Values = ctx.collectionOf(fill.fv.Feature, fill.held)
		}
		fill.fv.Materialized = true
		ctx.invalidateDependents(fill.fv)
	}
	return made, undo, nil
}
