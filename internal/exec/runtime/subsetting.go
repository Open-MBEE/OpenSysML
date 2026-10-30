package runtime

import (
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	key := relatedKey{sym, owner, kind}
	if features, ok := ctx.model.related[key]; ok {
		return features
	}
	features := slices.Clip(ctx.collectRelatedFeatures(sym, owner, kind))
	ctx.model.related[key] = features
	return features
}

// relatedKey identifies the features of one owner a feature's relationships of a kind name.
type relatedKey struct {
	sym, owner *symbols.Symbol
	kind       ast.RelationshipKind
}

func (ctx *Context) collectRelatedFeatures(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []*symbols.Symbol {
	var features []*symbols.Symbol
	for _, rel := range ctx.relationshipsOfKind(sym, kind) {
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
			if own, declared := ctx.ownDeclarationNamed(owner, sym, relationshipTargetLastName(rel.Target), resolved.Name, resolved.ShortName); declared && !ctx.redefinesTransitively(own, sym) {
				features = append(features, own)
			} else {
				features = append(features, resolved)
			}
			continue
		}
		if qn == nil || len(qn.Parts) != 1 {
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

// relationshipsKey identifies the relationships of one kind a symbol declares.
type relationshipsKey struct {
	sym  *symbols.Symbol
	kind ast.RelationshipKind
}

// relationshipsOfKind returns sym's relationships of the given kind that name a target,
// memoized per symbol and kind; callers read the shared slice and never append to it.
func (ctx *Context) relationshipsOfKind(sym *symbols.Symbol, kind ast.RelationshipKind) []*ast.Relationship {
	key := relationshipsKey{sym, kind}
	if rels, ok := ctx.model.relationships[key]; ok {
		return rels
	}
	rels := declaredRelationshipsOfKind(sym, kind)
	ctx.model.relationships[key] = rels
	return rels
}

// declaredRelationshipsOfKind collects sym's relationships of the given kind that name a target.
func declaredRelationshipsOfKind(sym *symbols.Symbol, kind ast.RelationshipKind) []*ast.Relationship {
	var rels []*ast.Relationship
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind || rel.Target == nil {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil && len(qn.Parts) > 0 {
			rels = append(rels, rel)
			continue
		}
		// A feature-chain target names a feature too: its last segment, which
		// RelationshipTarget resolves — the semantics layer decides whether it does.
		if _, ok := rel.Target.(*ast.FeatureChainExpr); ok {
			rels = append(rels, rel)
		}
	}
	return rels
}

// relationshipTargetLastName returns the final feature name a relationship
// target spells: a qualified name's last part, or a feature chain's member's.
func relationshipTargetLastName(target ast.Node) string {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	for {
		switch node := target.(type) {
		case *ast.FeatureReference:
			target = node.Name
		case *ast.FeatureChainExpr:
			target = node.Member
		case *ast.QualifiedName:
			if len(node.Parts) == 0 {
				return ""
			}
			return node.Parts[len(node.Parts)-1].Text
		default:
			return ""
		}
	}
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

// aliasRedefinedFeatureValuesOf makes every name a redefinition chain gives one feature, across
// types sharing a name, read one feature value: a carried one, else the most specific valued declaration's.
func (ctx *Context) aliasRedefinedFeatureValuesOf(inst *Instance, types []*symbols.Symbol, carried map[string]bool, features []EffectiveFeature) error {
	byName := make(map[string]*EffectiveFeature, len(features))
	for i := range features {
		byName[features[i].Name] = &features[i]
	}
	var chains []redefinitionChains
	for _, typ := range types {
		if groups := ctx.redefinitionGroups(typ); len(groups) > 0 {
			chains = append(chains, ctx.redefinitionChainsOf(typ, groups, byName))
		}
	}
	for _, names := range mergedRedefinitionGroups(chains) {
		chosen, in, err := ctx.sharedRedefinitionName(inst, byName, chains, names)
		if err != nil {
			return err
		}
		fv := inst.FeatureValues[chosen]
		for _, name := range names {
			if carried[name] {
				fv = inst.FeatureValues[name]
				if err := ctx.refineFeatureValue(inst, fv, byName[chosen], in); err != nil {
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

// redefinitionChains is one type's redefinition groups with the declarations
// its features canonically have and the names a nested chain overrode.
type redefinitionChains struct {
	typ             *symbols.Symbol
	groups          [][]string
	canonicalByName map[string]*EffectiveFeature
	overridden      map[string]bool
}

// redefinitionChainsOf notes which names of typ's groups a nested chain replaced the
// effective feature of: that chain's declaration is the most specific, so it leads.
func (ctx *Context) redefinitionChainsOf(typ *symbols.Symbol, groups [][]string, byName map[string]*EffectiveFeature) redefinitionChains {
	chains := redefinitionChains{typ: typ, groups: groups}
	canonical := ctx.FeaturesOf(typ)
	chains.canonicalByName = make(map[string]*EffectiveFeature, len(canonical))
	for i := range canonical {
		chains.canonicalByName[canonical[i].Name] = &canonical[i]
		feat, ok := byName[canonical[i].Name]
		if ok && feat.Symbol != nil && feat.Symbol != canonical[i].Symbol {
			if chains.overridden == nil {
				chains.overridden = make(map[string]bool)
			}
			chains.overridden[canonical[i].Name] = true
		}
	}
	return chains
}

// mergedRedefinitionGroups joins the groups of every type that share a name, since a
// name is one feature whichever type declares it; first-appearance order is kept.
func mergedRedefinitionGroups(chains []redefinitionChains) [][]string {
	leader := map[string]string{}
	var find func(string) string
	find = func(name string) string {
		next := leader[name]
		if next == name {
			return name
		}
		root := find(next)
		leader[name] = root
		return root
	}
	var order []string
	for _, c := range chains {
		for _, names := range c.groups {
			for _, name := range names {
				if _, ok := leader[name]; !ok {
					leader[name] = name
					order = append(order, name)
				}
				if a, b := find(names[0]), find(name); a != b {
					leader[b] = a
				}
			}
		}
	}
	var merged [][]string
	index := map[string]int{}
	for _, name := range order {
		root := find(name)
		at, ok := index[root]
		if !ok {
			at = len(merged)
			index[root] = at
			merged = append(merged, nil)
		}
		merged[at] = append(merged[at], name)
	}
	return merged
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
	byName := make(map[string]*EffectiveFeature, len(features))
	for i := range features {
		if features[i].Symbol != nil {
			byName[features[i].Name] = &features[i]
		}
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
		for _, redefined := range ctx.redefinedFeatures(feat.Symbol, typ) {
			slot, held := byName[redefined.Name]
			if !held || slot.Name == feat.Name || !ctx.slotHolds(slot, redefined, typ) {
				continue
			}
			if a, b := find(feat.Name), find(slot.Name); a != b {
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

// slotHolds reports whether slot's feature is feature itself or in its redefinition
// chain; a namesake no redefinition relates to it is another feature.
func (ctx *Context) slotHolds(slot *EffectiveFeature, feature, typ *symbols.Symbol) bool {
	return slot.Symbol == feature ||
		slices.Contains(ctx.redefinedFeatures(slot.Symbol, typ), feature) ||
		slices.Contains(ctx.redefinedFeatures(feature, typ), slot.Symbol)
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

// sharedRedefinitionName returns the name whose feature value a merged group shares, the
// first valued one in most-specific-first order else the most specific, and its type.
func (ctx *Context) sharedRedefinitionName(inst *Instance, byName map[string]*EffectiveFeature, chains []redefinitionChains, names []string) (string, *symbols.Symbol, error) {
	in := map[string]bool{}
	for _, name := range names {
		in[name] = true
	}
	for _, c := range chains {
		for _, group := range c.groups {
			if !in[group[0]] {
				continue
			}
			valued, err := ctx.valuedRedefinitionName(inst, byName, c.canonicalByName, group, c.overridden)
			if err != nil {
				return "", nil, err
			}
			if valued != "" {
				return valued, c.typ, nil
			}
		}
	}
	for _, c := range chains {
		for _, group := range c.groups {
			if group[0] == names[0] {
				return names[0], c.typ, nil
			}
		}
	}
	return names[0], chains[0].typ, nil
}

// valuedRedefinitionName returns the first name of one type's group, overridden
// names first, whose declaration values it, or "" when none does.
func (ctx *Context) valuedRedefinitionName(inst *Instance, byName, canonicalByName map[string]*EffectiveFeature, names []string, overridden map[string]bool) (string, error) {
	valued := ""
	var valuedBy *symbols.Scope
	for _, name := range overriddenFirst(names, overridden) {
		feat, ok := byName[name]
		if !ok || feat.Symbol == nil {
			continue
		}
		for _, cand := range valueCandidates(name, feat, canonicalByName, overridden) {
			if !ctx.declarationValues(cand) {
				continue
			}
			if valued == "" {
				valued, valuedBy = name, cand.Symbol.OwnerScope
				continue
			}
			if cand.Symbol.OwnerScope == valuedBy {
				return "", fmt.Errorf("%w: %s values %s and %s, which redefinition makes one feature",
					errConflictingRedefinition, inst.Type.Name, valued, name)
			}
		}
	}
	return valued, nil
}

// overriddenFirst orders the names with the overridden ones first, each part
// in the given order.
func overriddenFirst(names []string, overridden map[string]bool) []string {
	if len(overridden) == 0 {
		return names
	}
	ordered := make([]string, 0, 2*len(names))
	for _, name := range names {
		if overridden[name] {
			ordered = append(ordered, name)
		}
	}
	for _, name := range names {
		if !overridden[name] {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

// valueCandidates lists the declarations whose value counts for a name: the
// declaration a chain replaced still counts its value, so two names one body
// values conflict however specifically each reads.
func valueCandidates(name string, feat *EffectiveFeature, canonicalByName map[string]*EffectiveFeature, overridden map[string]bool) []*EffectiveFeature {
	if overridden[name] {
		if canon := canonicalByName[name]; canon != nil && canon.Symbol != feat.Symbol {
			return []*EffectiveFeature{canon, feat}
		}
	}
	return []*EffectiveFeature{feat}
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
		// A connector end, a behavior or step parameter, and a subject or objective redefine by
		// position or role without naming it (KerML 1.0 §7.3.4.5): the two names read one feature value.
		redefines := append(ctx.relatedFeatures(cur, owner, ast.RelRedefines),
			ctx.model.semantics.ImplicitEndRedefinitions(cur)...)
		redefines = append(redefines, ctx.model.semantics.ImplicitParameterRedefinitions(cur)...)
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
	key := featureOfType{feature: sym, owner: owner}
	if names, ok := ctx.model.subsetted[key]; ok {
		return names
	}
	names := ctx.relatedFeatureNames(sym, owner, ast.RelSubsets)
	for _, redefined := range ctx.redefinedFeatures(sym, owner) {
		names = append(names, ctx.relatedFeatureNames(redefined, owner, ast.RelSubsets)...)
		names = append(names, ctx.metadataSubsettingNames(redefined, owner)...)
	}
	// A nested usage also subsets the feature of its owner its kind nests under
	// (SysML v2 §8.3): a part in an item one of its `subparts`.
	names = append(names, ctx.implicitSubsettingNames(sym, owner)...)
	names = append(names, ctx.metadataSubsettingNames(sym, owner)...)
	ctx.model.subsetted[key] = names
	return names
}

// metadataSubsettingNames returns the owner features sym subsets through its semantic metadata
// (SysML v2 §7.27.4, `#cause` for `causes`): the base when owner carries it, else its redefinition.
func (ctx *Context) metadataSubsettingNames(sym, owner *symbols.Symbol) []string {
	var names []string
	for _, base := range ctx.model.semantics.SemanticMetadataBases(sym) {
		if base == nil || base == sym || !base.IsFeature() {
			continue
		}
		if ctx.isFeatureOf(owner, base, sym) {
			names = append(names, base.Name)
			continue
		}
		for _, member := range ctx.model.semantics.MembersOf(owner) {
			if member != sym && member.Name != "" && ctx.redefinesTransitively(member, base) {
				names = append(names, member.Name)
				break
			}
		}
	}
	return names
}

// implicitSubsettingNames returns the names of the owner features sym and the
// features it redefines implicitly subset by nesting (SysML v2 §8.3); only
// model-declared usages contribute theirs.
func (ctx *Context) implicitSubsettingNames(sym, owner *symbols.Symbol) []string {
	var names []string
	for _, sub := range ctx.model.semantics.ImplicitSubsettings(sym) {
		names = append(names, sub.Name)
	}
	for _, redefined := range ctx.redefinedFeatures(sym, owner) {
		for _, sub := range ctx.model.semantics.ImplicitSubsettings(redefined) {
			names = append(names, sub.Name)
		}
	}
	return names
}

// declaredSubsettedNames is subsettedNames without the implicit owner-context
// subsettings, for readers that follow only declared edges.
func (ctx *Context) declaredSubsettedNames(sym, owner *symbols.Symbol) []string {
	key := featureOfType{feature: sym, owner: owner}
	if names, ok := ctx.model.declaredSubsetted[key]; ok {
		return names
	}
	names := ctx.collectDeclaredSubsettedNames(sym, owner)
	ctx.model.declaredSubsetted[key] = names
	return names
}

func (ctx *Context) collectDeclaredSubsettedNames(sym, owner *symbols.Symbol) []string {
	names := ctx.subsettedNames(sym, owner)
	implicit := ctx.implicitSubsettingNames(sym, owner)
	if len(implicit) == 0 {
		return names
	}
	skip := make(map[string]int, len(implicit))
	for _, name := range implicit {
		skip[name]++
	}
	// names is the memoized slice subsettedNames shares; copy, never write it.
	out := make([]string, 0, len(names)-len(implicit))
	for _, name := range names {
		if skip[name] > 0 {
			skip[name]--
			continue
		}
		out = append(out, name)
	}
	return out
}

// SubsettingFeatures returns the features of typ subsetting the named feature under any
// of its redefinition names, in declaration order, reading nothing; inst is nil for a type alone.
func (ctx *Context) SubsettingFeatures(inst *Instance, typ *symbols.Symbol, name string) []EffectiveFeature {
	ofType := ctx.subsettersOf(typ, name)
	if inst == nil {
		return ofType
	}
	for i, feat := range ofType {
		if _, ok := inst.FeatureValues[feat.Name]; ok {
			continue
		}
		// The first feature inst does not hold: the rest is filtered into a slice of its own.
		subsetting := append(make([]EffectiveFeature, 0, len(ofType)-1), ofType[:i]...)
		for _, feat := range ofType[i+1:] {
			if _, ok := inst.FeatureValues[feat.Name]; ok {
				subsetting = append(subsetting, feat)
			}
		}
		return subsetting
	}
	return ofType
}

// subsettersOf is SubsettingFeatures for the type alone, memoized per type and name;
// callers read the shared slice and never append to it.
func (ctx *Context) subsettersOf(typ *symbols.Symbol, name string) []EffectiveFeature {
	byName, ok := ctx.model.subsetters[typ]
	if !ok {
		byName = make(map[string][]EffectiveFeature)
		ctx.model.subsetters[typ] = byName
	}
	if subsetting, ok := byName[name]; ok {
		return subsetting
	}
	aliases := ctx.redefinitionAliases(typ, name)
	var subsetting []EffectiveFeature
	for _, feat := range ctx.FeaturesOf(typ) {
		if aliases[feat.Name] || feat.Symbol == nil {
			continue
		}
		for _, subsetted := range ctx.subsettedNames(feat.Symbol, typ) {
			if aliases[subsetted] {
				subsetting = append(subsetting, feat)
				break
			}
		}
	}
	subsetting = slices.Clip(subsetting)
	byName[name] = subsetting
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
	var seen map[int64]bool
	err := ctx.eachSubsetterOf(inst, name, func(feat *EffectiveFeature) error {
		// A subsetter drawing its values from this feature adds none to it.
		if ctx.drawingSubsetters[featureValueRef{instance: inst.ID, feature: feat.Name}] {
			return nil
		}
		sub, err := inst.GetFeatureValue(ctx, feat.Name)
		if err != nil {
			return err
		}
		held, err := ctx.HeldElements(sub.HeldValue())
		if err != nil {
			return err
		}
		values, seen = appendUniqueInstances(values, held, seen)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if values, err = ctx.endContributions(inst, name, values, seen); err != nil {
		return nil, err
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, err
	}
	return values, nil
}

// endContributions appends the attached ends of a connector object whose end feature subsets the
// named feature: an end's value is a value of what the end subsets (KerML 1.0 §7.3.4.4).
func (ctx *Context) endContributions(inst *Instance, name string, values []Value, seen map[int64]bool) ([]Value, error) {
	for _, end := range inst.Ends {
		if end.Feature == nil || !slices.Contains(ctx.subsettedNamesOf(inst, end.Feature), name) {
			continue
		}
		held, err := ctx.HeldElements(end.Value)
		if err != nil {
			return nil, fmt.Errorf("end %s of %s: %w", endName(end), name, err)
		}
		values, seen = appendUniqueInstances(values, held, seen)
	}
	return values, nil
}

// endName names an end in a report: its own name, else the feature it attaches.
func endName(end ConnectorEnd) string {
	if end.Name != "" {
		return end.Name
	}
	return FormatValue(end.Value)
}

// openSubsettingContributions is subsettingContributions for a model-level read: an open
// subsetter is not made up, contributing what it certainly holds and its fewest as atLeast.
func (ctx *Context) openSubsettingContributions(inst *Instance, name string) (values []Value, atLeast int64, err error) {
	var seen map[int64]bool
	err = ctx.eachSubsetterOf(inst, name, func(feat *EffectiveFeature) error {
		sub, open, err := inst.openFeatureValue(ctx, feat.Name)
		if err != nil {
			return err
		}
		if !open.Stopped {
			held, err := ctx.HeldElements(sub.HeldValue())
			if err != nil {
				return err
			}
			values, seen = appendUniqueInstances(values, held, seen)
			return nil
		}
		values = append(values, open.Contributed...)
		atLeast = max(atLeast, open.AtLeast, fewestOf(feat.Multiplicity))
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if values, err = ctx.endContributions(inst, name, values, seen); err != nil {
		return nil, 0, err
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, 0, err
	}
	return values, atLeast, nil
}

// appendUniqueInstances appends els to out, dropping an object already seen:
// a subsetted set holds each instance once, however many subsetting paths reach it.
// The seen set is made on the first object and returned.
func appendUniqueInstances(out, els []Value, seen map[int64]bool) ([]Value, map[int64]bool) {
	for _, v := range els {
		if v.Kind == ValInstance {
			if seen[v.Instance] {
				continue
			}
			if seen == nil {
				seen = make(map[int64]bool)
			}
			seen[v.Instance] = true
		}
		out = append(out, v)
	}
	return out, seen
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

// hasImpliedSubsetter reports whether a feature subsets the named feature of inst
// only through subsetting implied by nesting: reading the collection would
// materialize objects an eager fill has no business creating.
func (ctx *Context) hasImpliedSubsetter(inst *Instance, name string) bool {
	for _, feat := range ctx.subsettingFeaturesOf(inst, name) {
		if !slices.Contains(ctx.declaredSubsettedNamesOf(inst, feat.Symbol), name) {
			return true
		}
	}
	return false
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
		!ctx.model.semantics.IsConnectorObjectUsage(feat.Symbol) && ctx.CompositeTypeOf(feat) != nil
}

// materializeSubsettedCollections reads the collections an optional feature subsets before the
// feature itself, so what they make up for their lower bounds is held by it whichever is read first.
func (ctx *Context) materializeSubsettedCollections(inst *Instance, fv *FeatureValue) error {
	if !ctx.fillsFromSubsetted(fv.Feature) && !ctx.connectorFillsFromSubsetted(fv.Feature) {
		return nil
	}
	key := featureValueRef{instance: inst.ID, feature: fv.Feature.Name}
	if ctx.readingSubsetted[key] {
		return nil
	}
	ctx.readingSubsetted[key] = true
	defer delete(ctx.readingSubsetted, key)
	for _, name := range ctx.declaredSubsettedNamesOf(inst, fv.Feature.Symbol) {
		sub, ok := inst.FeatureValues[name]
		if !ok || sub.Materialized || sub.Feature.Scalar() ||
			ctx.collectingSubsets[featureValueRef{instance: inst.ID, feature: name}] ||
			ctx.readingSubsetted[featureValueRef{instance: inst.ID, feature: name}] ||
			ctx.hasImpliedSubsetter(inst, name) {
			continue
		}
		if _, err := inst.GetFeatureValue(ctx, name); err != nil {
			return fmt.Errorf("subsetted feature %s of %s: %w", name, fv.Feature.Name, err)
		}
	}
	return nil
}

// subsetterRoom is how many objects the optional subsetters fillOptionalSubsetters fills
// could still take for the collection name: math.MaxInt64 when one takes any number.
func (ctx *Context) subsetterRoom(inst *Instance, name string) int64 {
	var room int64
	for _, feat := range ctx.subsettingFeaturesOf(inst, name) {
		if !ctx.fillsFromSubsetted(&feat) {
			continue
		}
		upper := feat.Multiplicity.Upper
		if !upper.Known || upper.Infinite {
			return math.MaxInt64
		}
		heldValue := inst.FeatureValues[feat.Name].HeldValue()
		if spare := upper.Value - elementCount(&heldValue); spare > 0 {
			if room > math.MaxInt64-spare {
				return math.MaxInt64
			}
			room += spare
		}
	}
	return room
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
		spare := n
		if upper := feat.Multiplicity.Upper; upper.Known && !upper.Infinite {
			if spare = int(upper.Value - ElementCount(fv.HeldValue())); spare > n {
				spare = n
			}
		}
		if spare <= 0 {
			continue
		}
		held, err := ctx.HeldElements(fv.HeldValue())
		if err != nil {
			return made, undo, err
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
		fill.fv.Materialized, fill.fv.intrinsic = true, false
		ctx.invalidateDependents(fill.fv)
	}
	return made, undo, nil
}

// referencedFeatureName is the feature of inst that sym reference-subsets (`ref part y ::> x`),
// or "" when it names no feature this object holds.
func (ctx *Context) referencedFeatureName(inst *Instance, sym *symbols.Symbol) string {
	target := ctx.model.semantics.ReferencedFeature(sym)
	if target == nil || target == sym {
		return ""
	}
	fv, held := inst.FeatureValues[target.Name]
	if !held {
		return ""
	}
	// A redefinition under another name (`part renamed :>> own`) keeps the name it
	// redefines as an alias of its own slot, which the types no longer look up.
	if fv.Feature.Symbol == target || ctx.redefinesTransitively(fv.Feature.Symbol, target) {
		return target.Name
	}
	for _, typ := range inst.types() {
		if ctx.isFeatureOf(typ, target, sym) {
			return target.Name
		}
	}
	return ""
}

// materializeReferenced holds in fv the values of the feature of the same object it
// reference-subsets: a reference subsetting is a subsetting (KerML 1.0 §8.3.3.3.9), so the
// values of `ref part y ::> x` are values of x (§7.3.4.4). Reports whether it held any.
func (inst *Instance) materializeReferenced(ctx *Context, fv *FeatureValue, name string) (bool, error) {
	source := ctx.referencedFeatureName(inst, fv.Feature.Symbol)
	if source == "" || source == name {
		return false, nil
	}
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.readingSubsetted[key] || ctx.readingSubsetted[featureValueRef{instance: inst.ID, feature: source}] {
		return false, nil
	}
	ctx.readingSubsetted[key] = true
	defer delete(ctx.readingSubsetted, key)
	sfv, err := inst.GetFeatureValue(ctx, source)
	if err != nil {
		return false, fmt.Errorf("referenced feature %s of %s: %w", source, name, err)
	}
	// The reference reads the source for as long as it holds it: a write to the
	// source unmaterializes it, and the next read follows the source again.
	ctx.listRead(sfv, fv)
	val := sfv.HeldValue()
	if val.Kind == ValInvalid {
		return false, nil
	}
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return false, err
	}
	if val, err = ctx.holdDeclared(inst, fv, val); err != nil {
		return false, err
	}
	if fv.Feature.Scalar() {
		fv.Value = val
	} else {
		fv.Values = val
	}
	fv.Materialized = true
	return true, nil
}

// materializeFromSubsetted holds in a feature stating no value the values of what it subsets (KerML
// 1.0 §7.3.4.4): its bindings' witnesses, then what the lower bound needs; false where none names any.
func (inst *Instance) materializeFromSubsetted(ctx *Context, fv *FeatureValue, name string, composite *symbols.Symbol) (bool, error) {
	mult := fv.Feature.Multiplicity
	if ctx.model.semantics.IsConnectorObjectUsage(fv.Feature.Symbol) {
		return false, nil
	}
	subsetted := ctx.declaredSubsettedNamesOf(inst, fv.Feature.Symbol)
	key := featureValueRef{instance: inst.ID, feature: name}
	if len(subsetted) == 0 || ctx.drawingSubsetters[key] {
		return false, nil
	}
	ctx.drawingSubsetters[key] = true
	defer delete(ctx.drawingSubsetters, key)

	pool, pooled, err := ctx.subsettedPool(inst, fv, subsetted, composite)
	if err != nil {
		return false, err
	}
	taken := ctx.siblingSubsetterValues(inst, fv, subsetted)
	chosen, seen, err := ctx.bindingWitnesses(inst, name, pool, pooled, taken)
	if err != nil {
		return false, err
	}
	want := fewestOf(mult)
	if mult.Upper.Known && !mult.Upper.Infinite && int64(len(chosen)) > mult.Upper.Value {
		chosen = chosen[:mult.Upper.Value]
	}
	if pooled && want > 0 {
		chosen, _ = pickFromPool(chosen, seen, pool, taken, want)
	} else if len(chosen) == 0 {
		return false, nil
	}
	mark := len(ctx.created)
	if int64(len(chosen)) < want && !pooled {
		if err := ctx.chargeElements(want - int64(len(chosen))); err != nil {
			return false, err
		}
		made, err := ctx.materializeMembers(composite, int(want)-len(chosen), inst, name)
		if err != nil {
			ctx.abandonInstancesSince(mark)
			return false, err
		}
		for _, child := range made {
			chosen = append(chosen, Value{Kind: ValInstance, Instance: child.ID})
		}
		if _, err := inst.holdContributed(ctx, fv, name, chosen); err != nil {
			ctx.abandonInstancesSince(mark)
			return false, err
		}
		fv.Assumed = mult.AdmitsMore(int64(len(chosen)))
		return true, ctx.startClassifierBehaviorsOf(made, mark)
	}
	if _, err := inst.holdContributed(ctx, fv, name, chosen); err != nil {
		return false, err
	}
	fv.Assumed = mult.AdmitsMore(int64(len(chosen)))
	return true, nil
}

// subsettedPool is the objects of fv's type every subsetted feature holds by its own declaration (a
// value, a write or a binding, not a `default`); pooled is false where none does and they hold what fv makes.
func (ctx *Context) subsettedPool(inst *Instance, fv *FeatureValue, subsetted []string, composite *symbols.Symbol) (pool []Value, pooled bool, err error) {
	for _, sub := range subsetted {
		subFV, ok := inst.FeatureValues[sub]
		if !ok || !((ctx.valueBinds(subFV.Feature) && !subFV.Feature.defaultIsFallback()) || subFV.Written || subFV.BindingDerived) {
			continue
		}
		subKey := featureValueRef{instance: inst.ID, feature: sub}
		if !subFV.Materialized {
			if ctx.collectingSubsets[subKey] || ctx.readingSubsetted[subKey] || ctx.drawingSubsetters[subKey] {
				continue
			}
			if _, err := inst.GetFeatureValue(ctx, sub); err != nil {
				return nil, false, fmt.Errorf("subsetted feature %s of %s: %w", sub, fv.Feature.Name, err)
			}
		}
		held, err := ctx.HeldElements(subFV.HeldValue())
		if err != nil {
			return nil, false, fmt.Errorf("subsetted feature %s of %s: %w", sub, fv.Feature.Name, err)
		}
		// A write to the subsetted feature unmaterializes fv, to draw again from what it then holds.
		ctx.listRead(subFV, fv)
		if !pooled {
			pool, _ = appendUniqueInstances(nil, held, nil)
			pooled = true
			continue
		}
		pool = instancesAlsoIn(pool, held)
	}
	// A subsetter with a body is its own type; its values are of the types it declares.
	types := []*symbols.Symbol{composite}
	if composite.IsFeature() {
		types = ctx.directTypes(composite)
	}
	pool = slices.DeleteFunc(pool, func(v Value) bool {
		obj := ctx.instances[v.Instance]
		return obj == nil || slices.ContainsFunc(types, func(t *symbols.Symbol) bool { return !ctx.canClassify(obj, t) })
	})
	return pool, pooled, nil
}

// instancesAlsoIn keeps, in order, the objects of pool that els also holds.
func instancesAlsoIn(pool, els []Value) []Value {
	in := make(map[int64]bool, len(els))
	for _, v := range els {
		if v.Kind == ValInstance {
			in[v.Instance] = true
		}
	}
	return slices.DeleteFunc(slices.Clone(pool), func(v Value) bool { return v.Kind != ValInstance || !in[v.Instance] })
}

// siblingSubsetterValues is the objects the other features subsetting what fv subsets already
// hold on inst: a value drawn for fv is one of them only where the subsetted feature has no other.
func (ctx *Context) siblingSubsetterValues(inst *Instance, fv *FeatureValue, subsetted []string) map[int64]bool {
	var taken map[int64]bool
	for _, sub := range subsetted {
		for _, feat := range ctx.subsettingFeaturesOf(inst, sub) {
			sibling, ok := inst.FeatureValues[feat.Name]
			if !ok || sibling == fv || !sibling.Materialized {
				continue
			}
			for _, v := range elementsOf(sibling.HeldValue()) {
				if v.Kind == ValInstance {
					if taken == nil {
						taken = map[int64]bool{}
					}
					taken[v.Instance] = true
				}
			}
		}
	}
	return taken
}

// bindingWitnesses draws, per binding constraining the named feature, as many of its other end's
// values as the binding links, from pool where the subsetted features fix one (KerML 1.0 §7.4.6).
func (ctx *Context) bindingWitnesses(inst *Instance, name string, pool []Value, pooled bool, taken map[int64]bool) ([]Value, map[int64]bool, error) {
	draw := witnessDraw{fv: inst.FeatureValues[name], pooled: pooled, pool: pool, taken: taken}
	// A binding of an owner naming the feature through its path constrains it as well.
	for current, path := inst, name; current != nil; {
		if ctx.declaresBindings(current) {
			for _, binding := range ctx.bindingsOf(current, path) {
				if err := ctx.drawBindingWitnesses(&draw, inst, current, binding, path); err != nil {
					return nil, nil, err
				}
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" || !ctx.ownersDeclareBindings(owner) {
			break
		}
		path = ownerFeature + "." + path
		current = owner
	}
	return draw.chosen, draw.seen, nil
}

// ownersDeclareBindings reports whether inst or an owner of it declares a binding: where none
// does, a walk up from inst spells out no feature path, as no binding could name it.
func (ctx *Context) ownersDeclareBindings(inst *Instance) bool {
	for current := inst; current != nil; {
		if ctx.declaresBindings(current) {
			return true
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			return false
		}
		current = owner
	}
	return false
}

// declaresBindings reports whether any type of inst declares a binding, so that a walk over
// its owners spells out a feature path only where a binding could name it.
func (ctx *Context) declaresBindings(inst *Instance) bool {
	if inst.Type != nil && len(ctx.objectBindings(inst.Type)) != 0 {
		return true
	}
	for _, typ := range inst.classifiers {
		if len(ctx.objectBindings(typ)) != 0 {
			return true
		}
	}
	return false
}

// witnessDraw is one bindingWitnesses draw: the feature drawn for, the pool it may draw
// from, what sibling subsetters hold, and what it has chosen so far.
type witnessDraw struct {
	fv     *FeatureValue
	pooled bool
	pool   []Value
	inPool map[int64]bool
	taken  map[int64]bool
	chosen []Value
	seen   map[int64]bool
}

// admitsPooled reports whether a pooled draw may choose v: the pool index is built on the
// first candidate, so a walk that finds no binding never pays for it.
func (d *witnessDraw) admitsPooled(v Value) bool {
	if !d.pooled {
		return true
	}
	if d.inPool == nil {
		d.inPool = make(map[int64]bool, len(d.pool))
		for _, p := range d.pool {
			d.inPool[p.Instance] = true
		}
	}
	return d.inPool[v.Instance]
}

// drawBindingWitnesses draws from the other end of binding, when it constrains path at
// current, as many of that end's values as the binding links.
func (ctx *Context) drawBindingWitnesses(draw *witnessDraw, inst, current *Instance, binding lower.Binding, path string) error {
	end := bindingEndForPath(binding, path)
	if end < 0 {
		return nil
	}
	constrains, err := ctx.constrainingBinding(current, binding)
	if err != nil || !constrains {
		return err
	}
	other, err := ctx.resolveBindingEndpoint(current, binding, 1-end)
	if err != nil {
		return err
	}
	// An end drawing its own values meanwhile has none to share yet; nor, for a binding an
	// owner declares, one that will draw them from what it subsets when read.
	if slices.ContainsFunc(other.locations, func(loc bindingLocation) bool {
		return ctx.drawingSubsetters[featureValueRef{instance: loc.instance.ID, feature: loc.name}] ||
			(current != inst && ctx.drawsFromSubsetted(loc))
	}) {
		return nil
	}
	for _, loc := range other.locations {
		if src, ok := loc.instance.FeatureValues[loc.name]; ok && src != draw.fv {
			ctx.listRead(src, draw.fv)
		}
	}
	// An end that cannot be read shares nothing here; validating the binding reports why.
	val, found, err := ctx.bindingEndpointValue(other, current, true)
	if err != nil || !found {
		return nil
	}
	candidates, err := ctx.HeldElements(val)
	if err != nil {
		return nil
	}
	links, _ := ctx.bindingLinks(binding)
	draw.pick(candidates, links)
	return nil
}

// pick draws up to links admitted candidates, those no sibling subsetter holds first.
func (d *witnessDraw) pick(candidates []Value, links int64) {
	admits := func(v Value) bool {
		return v.Kind == ValInstance && d.admitsPooled(v) && !d.seen[v.Instance]
	}
	for n := int64(0); n < links; n++ {
		pick := slices.IndexFunc(candidates, func(v Value) bool { return admits(v) && !d.taken[v.Instance] })
		if pick < 0 {
			pick = slices.IndexFunc(candidates, admits)
		}
		if pick < 0 {
			return
		}
		d.chosen, d.seen = appendUniqueInstances(d.chosen, candidates[pick:pick+1], d.seen)
	}
}

// drawsFromSubsetted reports a feature at loc that holds no value yet and states none of its
// own, so that it draws its values from the features it subsets when read.
func (ctx *Context) drawsFromSubsetted(loc bindingLocation) bool {
	fv, ok := loc.instance.FeatureValues[loc.name]
	return ok && !fv.Materialized && !fv.Written && !fv.BindingDerived && !ctx.valueBinds(fv.Feature) &&
		len(ctx.declaredSubsettedNamesOf(loc.instance, fv.Feature.Symbol)) != 0
}

// pickFromPool fills chosen to want values from pool, those no sibling subsetter holds first.
func pickFromPool(chosen []Value, seen map[int64]bool, pool []Value, taken map[int64]bool, want int64) ([]Value, map[int64]bool) {
	for _, fresh := range []bool{true, false} {
		for _, v := range pool {
			if int64(len(chosen)) >= want {
				return chosen, seen
			}
			if seen[v.Instance] || (fresh && taken[v.Instance]) {
				continue
			}
			chosen, seen = appendUniqueInstances(chosen, []Value{v}, seen)
		}
	}
	return chosen, seen
}
