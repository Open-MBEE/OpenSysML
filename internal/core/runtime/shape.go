package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EffectiveFeature represents one feature value in a type's flattened schema:
// own + inherited − redefined/masked, carrying type + multiplicity + default.
type EffectiveFeature struct {
	Name         string
	Symbol       *symbols.Symbol // the declaring feature symbol
	OwnerType    *symbols.Symbol // type that declares this feature (may be supertype)
	Type         *symbols.Symbol // resolved type (nil if untyped)
	Multiplicity semantics.Range // declared or inherited (default 1..1)
	DefaultValue ast.Node        // value-binding expression (nil if none)
	DefaultDecl  *symbols.Symbol // feature the DefaultValue was written on (nil if none)
	HoldsSet     bool            // values form a set: a Collection's unordered unique elements
	Unique       bool            // holds no two equal values (KerML isUnique, the default)
}

// Scalar reports whether the feature holds at most one value.
func (f *EffectiveFeature) Scalar() bool {
	return f.Multiplicity.AtMostOne()
}

// DefaultIsFallback reports whether DefaultValue was written with `default`: a
// value the feature holds only when nothing else populates it.
func (f *EffectiveFeature) DefaultIsFallback() bool {
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
	defaultVal := ctx.extractDefaultValue(memberSym)
	defaultDecl := memberSym
	if defaultVal == nil {
		defaultVal, defaultDecl = ctx.redefinedDefault(memberSym, typeSym)
	}
	mult := ctx.featureMultiplicity(memberSym, typeSym)
	return EffectiveFeature{
		Name:         name,
		Symbol:       memberSym,
		OwnerType:    ctx.findOwnerType(memberSym),
		Type:         ctx.extractType(memberSym),
		Multiplicity: mult,
		DefaultValue: defaultVal,
		DefaultDecl:  defaultDecl,
		HoldsSet:     ctx.holdsSet(memberSym, typeSym, mult),
		Unique:       ctx.model.semantics.IsUnique(memberSym),
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

// extractType resolves the type of a feature: the one it declares, or the one
// it inherits from what it redefines or subsets when it restates none
// (KerML 1.0 §7.4.7).
func (ctx *Context) extractType(featureSym *symbols.Symbol) *symbols.Symbol {
	if typ := ctx.declaredType(featureSym); typ != nil {
		return typ
	}
	for _, sup := range ctx.model.semantics.AllSupertypes(featureSym) {
		if typ := ctx.declaredType(sup); typ != nil {
			return typ
		}
	}
	return nil
}

// declaredType resolves the type a feature states itself, ignoring inheritance.
func (ctx *Context) declaredType(featureSym *symbols.Symbol) *symbols.Symbol {
	// Check usage relationships for typing
	rels := semantics.RelationshipsOf(featureSym)
	for _, rel := range rels {
		if rel.Kind == ast.RelTyping && rel.Target != nil {
			// Unwrap FeatureReference if needed
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				if resolved, ok := ctx.resolveQualified(featureSym.OwnerScope, qn); ok {
					return resolved
				}
			}
		}
	}
	return nil
}

// extractMultiplicity returns the multiplicity governing a feature. stated is
// false when it declares none and the assumed 1..1 governs it instead.
func (ctx *Context) extractMultiplicity(featureSym *symbols.Symbol) (r semantics.Range, stated bool) {
	_, stated = ctx.model.semantics.MultiplicityOf(featureSym)
	return ctx.model.semantics.EffectiveMultiplicityOf(featureSym), stated
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

// featureMultiplicity is the multiplicity a feature has on owner: as stated, else
// as inherited from what it redefines or subsets, else the assumed 1..1.
func (ctx *Context) featureMultiplicity(sym, owner *symbols.Symbol) semantics.Range {
	mult, stated := ctx.extractMultiplicity(sym)
	if stated {
		return mult
	}
	if inherited, ok := ctx.inheritedMultiplicity(sym, owner, map[*symbols.Symbol]bool{sym: true}); ok {
		return inherited
	}
	return mult
}

// inheritedMultiplicity intersects the multiplicities a feature declaring none redefines —
// and, if abstract, subsets (KerML 1.0 §8.4.4.12.1); path ends cycles, not shared ancestors.
func (ctx *Context) inheritedMultiplicity(sym, owner *symbols.Symbol, path map[*symbols.Symbol]bool) (semantics.Range, bool) {
	var mult semantics.Range
	found := false
	kinds := []ast.RelationshipKind{ast.RelRedefines}
	if symbols.IsAbstract(sym) {
		kinds = append(kinds, ast.RelSubsets)
	}
	for _, kind := range kinds {
		for _, general := range ctx.relatedFeatures(sym, owner, kind) {
			if path[general] {
				continue
			}
			generalMult, stated := ctx.model.semantics.MultiplicityOf(general)
			if !stated {
				path[general] = true
				inherited, ok := ctx.inheritedMultiplicity(general, owner, path)
				delete(path, general)
				if ok {
					generalMult = inherited
				} else {
					generalMult = semantics.AssumedRange()
				}
			}
			if found {
				mult = mult.Intersect(generalMult)
			} else {
				mult, found = generalMult, true
			}
		}
	}
	return mult, found
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
