package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	msgCrossFeatureType            = "Cross feature must have same type as feature"
	msgCrossSubsettingChain        = "Cross subsetting must chain through an opposite end feature"
	msgCrossSubsettingOwner        = "Cross subsetting must be owned by one of two or more end features"
	msgCrossSpecialization         = "Cross feature must specialized redefined-end cross features"
	msgCrossSubsettingAtMostOne    = "At most one cross subsetting is allowed"
	codeCrossSubsettingOwner       = "cross-subsetting-crossing-feature"
	codeCrossSubsettingAtMostOne   = "cross-subsetting-at-most-one"
	codeCrossSubsettingChain       = "cross-subsetting-chain"
	codeCrossFeatureType           = "cross-feature-type"
	codeCrossFeatureSpecialization = "cross-feature-specialization"
	msgMustBeCrossFeature          = "Must be the cross feature"
)

// checkW10BCrossFeatures checks the cross subsettings a feature declares: at most
// one, owned by an end of a type with two or more ends, chaining through an
// opposite end to a same-typed feature that specializes redefined ends' crosses.
func (cc *constraintChecker) checkW10BCrossFeatures(sym *symbols.Symbol) {
	crosses := semantics.CrossSubsettings(sym)
	if len(crosses) == 0 {
		cc.checkOwnedCrossFeatureType(sym)
		return
	}
	cc.checkDeclaredCrossFeature(sym)
	for _, rel := range crosses[1:] {
		cc.addRedefineDiag(rel.Target, msgCrossSubsettingAtMostOne, codeCrossSubsettingAtMostOne)
	}
	owner := w10bOwningType(sym)
	ends := cc.model.EndFeatures(owner)
	isEnd := w10bIsEndFeature(sym) && owner != nil
	var cross *symbols.Symbol
	for i, rel := range crosses {
		if !isEnd || len(ends) < 2 {
			cc.addRedefineDiag(rel.Target, msgCrossSubsettingOwner, codeCrossSubsettingOwner)
		}
		if !isEnd {
			continue
		}
		base, crossed, isChain := cc.model.CrossedFeatureChain(sym, rel)
		// Only a two-ended owner pins the chain root (KerML validateCrossSubsettingCrossedFeature).
		if !isChain || len(ends) == 2 && !w10bIsOppositeEnd(ends, sym, base) {
			cc.addRedefineDiag(rel.Target, msgCrossSubsettingChain, codeCrossSubsettingChain)
		}
		if i == 0 {
			cross = crossed
		}
	}
	// Feature::crossFeature is single-valued: only the first clause defines it.
	rel := crosses[0]
	if cross == nil {
		return
	}
	if !cc.w10bSameTypes(cross, sym) {
		cc.addRedefineDiag(rel.Target, msgCrossFeatureType, codeCrossFeatureType)
	}
	if !cc.w10bSpecializesRedefinedCross(sym, cross) {
		cc.addRedefineDiag(rel.Target, msgCrossSpecialization, codeCrossFeatureSpecialization)
	}
}

// checkOwnedCrossFeatureType reports an end whose body-declared cross feature has
// types other than the end's own.
func (cc *constraintChecker) checkOwnedCrossFeatureType(end *symbols.Symbol) {
	cross := cc.model.OwnedCrossFeature(end)
	if cross == nil || cross.Decl == nil || cc.w10bSameTypes(cross, end) {
		return
	}
	cc.addRedefineDiag(cross.Decl, msgCrossFeatureType, codeCrossFeatureType)
}

// checkDeclaredCrossFeature reports an end that declares its cross feature inline
// and also crosses another: Feature::crossFeature is single-valued.
func (cc *constraintChecker) checkDeclaredCrossFeature(end *symbols.Symbol) {
	u, ok := end.Decl.(*ast.Usage)
	if !ok || u.CrossFeature == nil {
		return
	}
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     u.CrossFeature.Span(),
		Message:  msgMustBeCrossFeature,
		Code:     "declared-cross-feature",
		Source:   "constraint",
	})
}

// w10bOwningType returns the type whose body declares sym, or nil for a
// symbol declared at namespace level.
func w10bOwningType(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	switch owner.Decl.(type) {
	case *ast.Definition, *ast.Usage:
		return owner
	}
	return nil
}

// w10bIsOppositeEnd reports whether base is one of ends other than end itself.
func w10bIsOppositeEnd(ends []*symbols.Symbol, end, base *symbols.Symbol) bool {
	if base == nil || symbols.SameElement(base, end) {
		return false
	}
	for _, candidate := range ends {
		if candidate != nil && symbols.SameElement(candidate, base) {
			return true
		}
	}
	return false
}

// w10bSameTypes reports whether a cross feature and its end have the same
// types (KerML Feature::type), an untyped feature counting as typed by its kind's base.
func (cc *constraintChecker) w10bSameTypes(cross, end *symbols.Symbol) bool {
	crossTypes := cc.model.FeatureTypeSet(cross)
	endTypes := cc.model.FeatureTypeSet(end)
	if len(crossTypes) == 0 || len(endTypes) == 0 {
		return true // nothing resolved to compare
	}
	return w10bCoversTypes(crossTypes, endTypes) && w10bCoversTypes(endTypes, crossTypes)
}

func w10bCoversTypes(types, want []*symbols.Symbol) bool {
	for _, w := range want {
		found := false
		for _, t := range types {
			if symbols.SameElement(t, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// w10bSpecializesRedefinedCross reports whether cross specializes the cross
// feature of every end sym redefines, by a `redefines` clause or by occupying
// its position in a specialized connector.
func (cc *constraintChecker) w10bSpecializesRedefinedCross(sym, cross *symbols.Symbol) bool {
	redefined := append([]*symbols.Symbol(nil), cc.model.RedefinedFeatures(sym)...)
	redefined = append(redefined, cc.model.ImplicitEndRedefinitions(sym)...)
	for _, end := range redefined {
		inherited := cc.model.CrossFeature(end)
		if inherited == nil || symbols.SameElement(inherited, cross) {
			continue
		}
		if !cc.model.Conforms(cross, inherited) {
			return false
		}
	}
	return true
}
