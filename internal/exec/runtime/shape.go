package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// EffectiveFeature represents one feature value in a type's flattened schema:
// own + inherited − redefined/masked, carrying type + multiplicity + default.
type EffectiveFeature struct {
	Name         string
	Symbol       *symbols.Symbol // the declaring feature symbol
	OwnerType    *symbols.Symbol // type that declares this feature (may be supertype)
	Type         *symbols.Symbol // resolved type (nil if untyped)
	Multiplicity semantics.Range // declared, inherited or implicit; else the representative one value
	// MultiplicityDefaulted marks a feature that states and inherits no multiplicity, so the
	// usual default 0..* governs it: Multiplicity is representativeRange and judges no count.
	MultiplicityDefaulted bool
	DefaultValue          ast.Node        // value-binding expression (nil if none)
	DefaultDecl           *symbols.Symbol // feature the DefaultValue was written on (nil if none)
	HoldsSet              bool            // values form a set: a Collection's unordered unique elements
	Unique                bool            // holds no two equal values (KerML isUnique, the default)
	// GovernedByChain marks a feature a valued nested chain from a more specific
	// body reaches below, so its bound value does not govern, as with a
	// redefining body.
	GovernedByChain bool
}

// Scalar reports whether the feature holds at most one value.
func (f *EffectiveFeature) Scalar() bool {
	return f.Multiplicity.AtMostOne()
}

// defaultIsFallback reports whether DefaultValue was written with `default`: a
// value the feature holds only when nothing else populates it.
func (f *EffectiveFeature) defaultIsFallback() bool {
	if f.DefaultDecl == nil {
		return false
	}
	usage, ok := f.DefaultDecl.Decl.(*ast.Usage)
	return ok && usage.ValueIsDefault
}

// DefaultScope returns the scope DefaultValue resolves its names in, which for
// an inherited default is where the redefined declaration wrote it.
func (f *EffectiveFeature) DefaultScope() *symbols.Scope {
	if f.DefaultDecl != nil {
		return f.DefaultDecl.OwnerScope
	}
	return f.DeclScope()
}

// DeclScope returns the scope the feature was declared in, which is the scope a
// default value written on it must be evaluated in: an inherited feature's
// default refers to names visible where the supertype was written, not where
// the instantiated type is.
func (f *EffectiveFeature) DeclScope() *symbols.Scope {
	if f.Symbol == nil {
		return nil
	}
	return f.Symbol.OwnerScope
}

// DeclScope returns the scope a declaration's body was written in: the scope the
// declaration owns, in which its own members are visible to each other, falling
// back to the scope it was declared in when it owns none. It is the scope an
// expression written among its members resolves its names against — an
// attribute default, a guard, an assignment in a nested action body — and the
// scope the runtime lowers the declaration in.
func DeclScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// FeaturesOf returns the ordered, deduplicated effective-feature list for the given type symbol.
// Result: own + inherited − redefined/masked, memoized per symbol.
func (ctx *Context) FeaturesOf(typeSym *symbols.Symbol) []EffectiveFeature {
	if typeSym == nil {
		return nil
	}

	// Memoization
	if cached, ok := ctx.model.features[typeSym]; ok {
		return cached
	}

	features := ctx.buildFeatures(typeSym)
	ctx.model.features[typeSym] = features
	return features
}

// buildFeatures constructs the effective-feature list from the semantic shape.
func (ctx *Context) buildFeatures(typeSym *symbols.Symbol) []EffectiveFeature {
	// Redefined features stay in the shape: a redefinition shares its target's
	// feature value, which both names read (see subsetting_test.go).
	shape := ctx.model.semantics.ShapeFeatures(typeSym)
	result := make([]EffectiveFeature, 0, len(shape))
	seenNames := make(map[string]bool, len(shape))
	for _, f := range shape {
		seenNames[f.Name] = true
		result = append(result, ctx.effectiveFeature(f.Name, f.Symbol, typeSym))
	}
	return append(result, ctx.connectorEndFeatures(typeSym, seenNames)...)
}

// effectiveFeature is the member memberSym of typeSym as name, with the value it
// states or inherits from what it redefines.
func (ctx *Context) effectiveFeature(name string, memberSym, typeSym *symbols.Symbol) EffectiveFeature {
	// A satisfy usage's `by` is its subject's most specific value (SysML v2 §7.21.4),
	// over a value or default the subject it binds declares or inherits.
	defaultVal, defaultDecl := ctx.satisfiedByValue(memberSym, typeSym)
	if defaultVal == nil {
		defaultVal, defaultDecl = ctx.extractDefaultValue(memberSym), memberSym
	}
	if defaultVal == nil {
		defaultVal, defaultDecl = ctx.redefinedDefault(memberSym, typeSym)
	}
	if defaultVal == nil {
		defaultVal, defaultDecl = ctx.roleRedefiningDefault(memberSym, typeSym)
	}
	mult, _, defaulted := ctx.featureRange(memberSym, typeSym)
	return EffectiveFeature{
		Name:                  name,
		Symbol:                memberSym,
		OwnerType:             ctx.findOwnerType(memberSym),
		Type:                  ctx.extractType(memberSym),
		Multiplicity:          mult,
		MultiplicityDefaulted: defaulted,
		DefaultValue:          defaultVal,
		DefaultDecl:           defaultDecl,
		HoldsSet:              ctx.holdsSet(memberSym, typeSym, mult),
		Unique:                ctx.model.semantics.IsUnique(memberSym),
	}
}

// parameterFeatures are the input parameters of typeSym no object carries as a
// feature value, in member order, the most specific declaration of each name.
func (ctx *Context) parameterFeatures(typeSym *symbols.Symbol) []EffectiveFeature {
	var order []string
	byName := make(map[string]*symbols.Symbol)
	for _, member := range ctx.model.semantics.MembersOfIncludingRedefined(typeSym) {
		if member.Name == "" || !isInputParameter(member) || semantics.IsShapeFeature(member) && !ctx.model.semantics.FrameFeature(member) {
			continue
		}
		if _, seen := byName[member.Name]; !seen {
			order = append(order, member.Name)
		}
		byName[member.Name] = member
	}
	out := make([]EffectiveFeature, 0, len(order))
	for _, name := range order {
		out = append(out, ctx.effectiveFeature(name, byName[name], typeSym))
	}
	return out
}

// isInputParameter reports an `in` or `inout` parameter usage.
func isInputParameter(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && !usage.IsResult && (usage.Direction == ast.DirIn || usage.Direction == ast.DirInOut)
}

// extractType resolves a feature's type: the one it states, or the single more specific
// effective type that stands for the conjunction (KerML 1.0 §7.3.4.3–§7.3.4.4).
func (ctx *Context) extractType(featureSym *symbols.Symbol) *symbols.Symbol {
	stated := ctx.statedType(featureSym)
	if types := ctx.effectiveTypes(featureSym); len(types) == 1 && types[0] != stated && ctx.model.semantics.Conforms(types[0], stated) {
		return types[0]
	}
	return stated
}

// statedType is the type a feature declares, or the first one a feature it
// specializes declares when it declares none itself (KerML 1.0 §7.4.7).
func (ctx *Context) statedType(featureSym *symbols.Symbol) *symbols.Symbol {
	if types := ctx.statedTypes(featureSym); len(types) > 0 {
		return types[0]
	}
	return nil
}

// statedTypes is every type a feature declares (`part x : A, B`), or those of
// the first feature it specializes that declares any when it declares none.
func (ctx *Context) statedTypes(featureSym *symbols.Symbol) []*symbols.Symbol {
	if types := ctx.declaredTypes(featureSym); len(types) > 0 {
		return types
	}
	for _, sup := range ctx.model.semantics.AllDeclaredSupertypes(featureSym) {
		if types := ctx.declaredTypes(sup); len(types) > 0 {
			return types
		}
	}
	return nil
}

// effectiveTypes returns the most specific of the stated types conjoined with the types
// of each subsetted, redefined or referenced feature (a positional parameter redefinition only when none is stated).
// Computed once per feature; callers must not modify the returned slice.
func (ctx *Context) effectiveTypes(featureSym *symbols.Symbol) []*symbols.Symbol {
	if types, ok := ctx.model.effectiveTypes[featureSym]; ok {
		return types
	}
	types := ctx.computeEffectiveTypes(featureSym)
	if ctx.model.effectiveTypes == nil {
		ctx.model.effectiveTypes = map[*symbols.Symbol][]*symbols.Symbol{}
	}
	ctx.model.effectiveTypes[featureSym] = types
	return types
}

func (ctx *Context) computeEffectiveTypes(featureSym *symbols.Symbol) []*symbols.Symbol {
	sem := ctx.model.semantics
	stated := ctx.statedTypes(featureSym)
	if len(stated) == 0 {
		return nil
	}
	types := slices.Clone(stated)
	add := func(candidates []*symbols.Symbol) {
		for _, typ := range candidates {
			if typ != nil && !slices.Contains(types, typ) {
				types = append(types, typ)
			}
		}
	}
	implicit := sem.ImplicitParameterRedefinitions(featureSym)
	for _, sup := range sem.DeclaredSupertypes(featureSym) {
		if sup.IsFeature() && !slices.Contains(implicit, sup) {
			add(sem.FeatureTypeSet(sup))
		}
	}
	if ref := sem.ReferencedFeature(featureSym); ref != nil && ref != featureSym {
		add(sem.FeatureTypeSet(ref))
	}
	return slices.DeleteFunc(types, func(typ *symbols.Symbol) bool {
		return slices.ContainsFunc(types, func(other *symbols.Symbol) bool {
			return other != typ && sem.Conforms(other, typ)
		})
	})
}

// conjoinedTypes returns the effective types that typ, the type the feature's objects
// are made as, does not stand for: `Panel :> frames : Framed` holds both (§7.3.4.3).
func (ctx *Context) conjoinedTypes(featureSym, typ *symbols.Symbol) []*symbols.Symbol {
	types := slices.Clone(ctx.effectiveTypes(featureSym))
	for _, base := range ctx.implicitFeatureBases(featureSym) {
		if !slices.Contains(types, base) {
			types = append(types, base)
		}
	}
	return slices.DeleteFunc(types, func(other *symbols.Symbol) bool {
		return other == typ || ctx.model.semantics.Conforms(typ, other)
	})
}

// implicitFeatureBases returns the bases the kinds of a feature and of the features it
// specializes imply (KerML §7.3.4): `:>> vertices` redefining `item :>> vertices` holds Items.
func (ctx *Context) implicitFeatureBases(featureSym *symbols.Symbol) []*symbols.Symbol {
	sem := ctx.model.semantics
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	var walk func(*symbols.Symbol)
	walk = func(f *symbols.Symbol) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		for _, base := range sem.ImplicitBases(f) {
			if base != nil && !slices.Contains(out, base) {
				out = append(out, base)
			}
		}
		implicit := sem.ImplicitParameterRedefinitions(f)
		for _, sup := range sem.DeclaredSupertypes(f) {
			if sup.IsFeature() && !slices.Contains(implicit, sup) {
				walk(sup)
			}
		}
	}
	walk(featureSym)
	return out
}

// withConjoinedTypes classifies the object by every type its feature conjoins and adds
// their members to its shape; a member two types declare is one feature.
func (ctx *Context) withConjoinedTypes(inst, owner *Instance, feature string, features []EffectiveFeature) []EffectiveFeature {
	if owner == nil || feature == "" {
		return features
	}
	fv := owner.FeatureValues[feature]
	if fv == nil || fv.Feature == nil || fv.Feature.Symbol == nil {
		return features
	}
	extra := ctx.conjoinedTypes(fv.Feature.Symbol, inst.Type)
	if len(extra) == 0 {
		return features
	}
	held := make(map[string]bool, len(features))
	for i := range features {
		held[features[i].Name] = true
	}
	features = slices.Clone(features)
	for _, typ := range extra {
		inst.classifiers = append(inst.classifiers, typ)
		for _, member := range ctx.FeaturesOf(typ) {
			if !held[member.Name] {
				held[member.Name] = true
				features = append(features, member)
			}
		}
	}
	return features
}

// declaredTypes resolves every type a feature states itself, in declaration
// order, ignoring inheritance.
func (ctx *Context) declaredTypes(featureSym *symbols.Symbol) []*symbols.Symbol {
	var types []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(featureSym) {
		if rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			if resolved, ok := ctx.resolveQualified(featureSym.OwnerScope, qn); ok && !slices.Contains(types, resolved) {
				types = append(types, resolved)
			}
		}
	}
	return types
}

// representativeRange is the range the runtime holds a feature to that states and
// inherits no multiplicity: one value, the representative of the usual default 0..*
// (KerML 1.1 §7.4.12), against which no count is judged (MultiplicityDefaulted).
func representativeRange() semantics.Range {
	return semantics.AssumedRange()
}

// extractMultiplicity returns the multiplicity a feature declares, or the
// representative range with stated false when it declares none.
func (ctx *Context) extractMultiplicity(featureSym *symbols.Symbol) (r semantics.Range, stated bool) {
	r, stated = ctx.model.semantics.MultiplicityOf(featureSym)
	if !stated {
		return representativeRange(), false
	}
	if (!r.Lower.Known || !r.Upper.Known) && featureSym.OwnerScope != nil {
		// A bound naming a valued feature is model-level evaluable in the scope declaring it.
		if scoped, ok := ctx.model.semantics.RangeIn(featureSym.OwnerScope, semantics.UsageMultiplicityOf(featureSym)); ok {
			r = semantics.Range{Lower: knownBound(r.Lower, scoped.Lower), Upper: knownBound(r.Upper, scoped.Upper)}
		}
	}
	return r, stated
}

// knownBound is plain, or scoped where plain is unknown.
func knownBound(plain, scoped semantics.Bound) semantics.Bound {
	if plain.Known {
		return plain
	}
	return scoped
}

// extractDefaultValue returns the default-value expression for a feature (nil if none).
func (ctx *Context) extractDefaultValue(featureSym *symbols.Symbol) ast.Node {
	if oc, ok := ast.OwnedConstraintOf(featureSym.Decl); ok {
		return oc.Value
	}
	switch decl := featureSym.Decl.(type) {
	case *ast.Usage:
		return decl.Value // nil if no default
	case *ast.SubjectMember:
		return decl.BindingExpr
	}
	return nil
}

// featureMultiplicity is the multiplicity a feature has on owner: a parameter's
// effective range (§7.6.3), else as stated, else as inherited from what it
// redefines or subsets, else the representative range.
func (ctx *Context) featureMultiplicity(sym, owner *symbols.Symbol) semantics.Range {
	mult, _, _ := ctx.featureRange(sym, owner)
	return mult
}

// featureRange is featureMultiplicity reporting whether a declaration, inheritance or the
// parameter rule states the range, and whether it is defaulted: none of those nor the
// implicit [1..1] (SysML v2 §7.6.3, end features) states it, so the usual default 0..*
// judges no count against it.
func (ctx *Context) featureRange(sym, owner *symbols.Symbol) (mult semantics.Range, stated, defaulted bool) {
	if semantics.IsParameter(sym) {
		return ctx.model.semantics.EffectiveParameterRange(sym), true, false
	}
	mult, stated = ctx.extractMultiplicity(sym)
	if stated {
		return mult, true, false
	}
	if unstated, unstatedStated, ok := ctx.unstatedMultiplicity(sym, owner, map[*symbols.Symbol]bool{sym: true}, false); ok {
		mult, stated = unstated, unstatedStated
	}
	return mult, stated, !stated && !ctx.model.semantics.ImplicitlyDeclaresOne(sym)
}

// unstatedMultiplicity is the multiplicity of a feature declaring none: inherited from what it
// redefines (and, abstract or on a subsetting walk, subsets), else, valued, bound by what it subsets.
func (ctx *Context) unstatedMultiplicity(sym, owner *symbols.Symbol, path map[*symbols.Symbol]bool, subsetting bool) (mult semantics.Range, stated, found bool) {
	if inherited, ok, inheritedStated := ctx.inheritedMultiplicity(sym, owner, path, subsetting || symbols.IsAbstract(sym)); ok {
		return inherited, inheritedStated, true
	}
	return ctx.subsettedBound(sym, owner, path)
}

// subsettedBound bounds a valued feature declaring none by the upper bound of the type-owned feature
// it subsets, references or crosses: no default of one is declared for it (SysML v2 §7.6.3).
func (ctx *Context) subsettedBound(sym, owner *symbols.Symbol, path map[*symbols.Symbol]bool) (semantics.Range, bool, bool) {
	if !bindsValue(sym) || ctx.model.semantics.ImplicitMultiplicityApplies(sym) {
		return semantics.Range{}, false, false
	}
	inherited, ok, stated := ctx.inheritedMultiplicity(sym, owner, path, true)
	if !ok {
		return semantics.Range{}, false, false
	}
	return semantics.Range{Lower: semantics.Bound{Known: true}, Upper: inherited.Upper}, stated, true
}

// bindsValue reports a feature whose declaration binds it a value (`= v`, `default v`).
func bindsValue(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModValued)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Value != nil
}

// statedMultiplicity is the multiplicity a feature states, itself or as inherited from the
// declarations it redefines or, abstract, subsets; a parameter stating none holds the assumed
// one value (KerML 1.0 §7.4.5), and stated is false only for a non-parameter none on that walk bounds.
func (ctx *Context) statedMultiplicity(sym *symbols.Symbol) (semantics.Range, bool) {
	if mult, stated := ctx.extractMultiplicity(sym); stated {
		return mult, true
	}
	var mult semantics.Range
	stated := false
	if owner := ctx.findOwnerType(sym); owner != nil {
		mult, _, stated = ctx.inheritedMultiplicity(sym, owner, map[*symbols.Symbol]bool{sym: true}, symbols.IsAbstract(sym))
	}
	if !stated && semantics.IsParameter(sym) {
		return ctx.model.semantics.EffectiveParameterRange(sym), true
	}
	return mult, stated
}

// inheritedMultiplicity intersects what a feature declaring none redefines and, when subsetting is
// followed, subsets (KerML 1.0 §8.4.4.12.1); path ends cycles, stated reports a general stating one.
func (ctx *Context) inheritedMultiplicity(sym, owner *symbols.Symbol, path map[*symbols.Symbol]bool, subsetting bool) (mult semantics.Range, found, stated bool) {
	generals := append(ctx.relatedFeatures(sym, owner, ast.RelRedefines),
		ctx.model.semantics.ImplicitParameterRedefinitions(sym)...)
	generals = append(generals, ctx.model.semantics.ImplicitRoleRedefinitions(sym)...)
	if subsetting {
		for _, kind := range [...]ast.RelationshipKind{ast.RelSubsets, ast.RelReferences, ast.RelCrosses} {
			generals = append(generals, ctx.relatedFeatures(sym, owner, kind)...)
		}
	}
	for _, general := range generals {
		if path[general] {
			continue
		}
		generalMult, generalStated := ctx.extractMultiplicity(general)
		if !generalStated && ctx.model.semantics.ImplicitlyDeclaresOne(general) {
			generalMult, generalStated = semantics.AssumedRange(), true
		} else if !generalStated {
			path[general] = true
			unstated, unstatedStated, ok := ctx.unstatedMultiplicity(general, owner, path, subsetting)
			delete(path, general)
			generalStated = unstatedStated
			if ok {
				generalMult = unstated
			} else {
				generalMult = representativeRange()
			}
		}
		stated = stated || generalStated
		if found {
			mult = mult.Intersect(generalMult)
		} else {
			mult, found = generalMult, true
		}
	}
	return mult, found, stated
}

// redefinedDefault returns the value a feature takes from the feature it
// redefines, by clause or by parameter position, and the declaration that wrote
// it: a redefining feature is the redefined feature declared again (KerML 1.0 §7.3.4.5).
func (ctx *Context) redefinedDefault(sym, owner *symbols.Symbol) (ast.Node, *symbols.Symbol) {
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		targets := ctx.relatedFeatures(cur, owner, ast.RelRedefines)
		targets = append(targets, ctx.model.semantics.ImplicitParameterRedefinitions(cur)...)
		targets = append(targets, ctx.redefinedNamespaceUsages(cur)...)
		for _, redefined := range targets {
			if seen[redefined] {
				continue
			}
			seen[redefined] = true
			if val := ctx.extractDefaultValue(redefined); val != nil {
				return val, redefined
			}
			queue = append(queue, redefined)
		}
	}
	return nil, nil
}

// satisfiedByValue is the `by` operand of the satisfy usage typeSym when sym is its subject.
func (ctx *Context) satisfiedByValue(sym, typeSym *symbols.Symbol) (ast.Node, *symbols.Symbol) {
	if typeSym == nil || typeSym.Decl == nil {
		return nil, nil
	}
	if by := satisfiedBy(typeSym); by != nil && ctx.model.semantics.SubjectParameterOf(typeSym) == sym {
		return by, typeSym
	}
	return nil, nil
}

// roleRedefiningDefault is the value typeSym binds to sym under no name of its own, a
// bare `subject = x;` (SysML v2 §7.21.2, §8.3.21.11), and its declaration.
func (ctx *Context) roleRedefiningDefault(sym, typeSym *symbols.Symbol) (ast.Node, *symbols.Symbol) {
	if typeSym == nil || typeSym.Decl == nil {
		return nil, nil
	}
	scope := bodyScope(typeSym, typeSym.OwnerScope)
	for _, node := range unwrappedDeclMembers(typeSym.Decl) {
		member := memberSymbol(scope, node)
		if member == nil || member == sym || member.Name != "" {
			continue
		}
		val := ctx.extractDefaultValue(member)
		if val == nil || !slices.Contains(ctx.redefinedFeatures(member, typeSym), sym) {
			continue
		}
		return val, member
	}
	return nil, nil
}

// redefinedNamespaceUsages are the usages sym redefines that no type features — package-level
// attributes such as `ShapeItems::xoffset` — whose value a redefining feature inherits as it
// inherits any redefined feature's (KerML 1.0 §7.3.4.5), though no object carries them.
func (ctx *Context) redefinedNamespaceUsages(sym *symbols.Symbol) []*symbols.Symbol {
	var usages []*symbols.Symbol
	for _, rel := range relationshipsOfKind(sym, ast.RelRedefines) {
		resolved := ctx.model.semantics.RelationshipTarget(sym, rel)
		if resolved == nil || resolved == sym {
			continue
		}
		if owner := ctx.findOwnerType(resolved); owner != nil && owner.Kind != symbols.SymbolPackage && owner.Kind != symbols.SymbolNamespace {
			continue
		}
		if _, usage := resolved.Decl.(*ast.Usage); usage {
			usages = append(usages, resolved)
		}
	}
	return usages
}

// findOwnerType walks up the scope chain to find the type symbol that owns the feature's declaration.
func (ctx *Context) findOwnerType(featureSym *symbols.Symbol) *symbols.Symbol {
	// Start from the feature's owner scope (the scope that contains the declaration)
	ownerScope := featureSym.OwnerScope
	if ownerScope == nil {
		return nil
	}

	// The owner scope's node is the definition/usage that contains the feature
	ownerNode := ownerScope.Node()
	if ownerNode == nil {
		return nil
	}

	// The scope records the symbol declaring it; a scope re-owned by another
	// declaration (a metadata body owned by its definition) is searched below.
	if owner := ownerScope.Owner(); owner != nil && owner.Decl == ownerNode {
		return owner
	}

	// Look up the symbol for the owner node in the parent scope
	parentScope := ownerScope.Parent()
	if parentScope == nil {
		return nil
	}

	// Find the symbol in the parent scope that declares the owner node
	for _, name := range parentScope.MemberNames() {
		syms := parentScope.LookupLocalAll(name)
		for _, sym := range syms {
			if sym.Decl == ownerNode {
				return sym
			}
		}
	}

	return nil
}
