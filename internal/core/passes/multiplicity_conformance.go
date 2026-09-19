package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Messages of the two multiplicity-conformance rules, worded as the reference
// implementation words them (KerML validateSubsetting/RedefinitionMultiplicityConformance).
const (
	msgSubsettingMultiplicityConformance   = "Subsetting/redefining feature should not have larger multiplicity upper bound"
	msgRedefinitionMultiplicityConformance = "Redefining feature should not have smaller multiplicity lower bound"
)

// checkMultiplicityConformance implements KerML 1.0 §7.4.9's multiplicity
// conformance for subsetting and redefinition (validateSubsettingMultiplicity-
// Conformance, validateRedefinitionMultiplicityConformance): a subsetting or
// redefining feature may not admit a larger upper bound than the feature it
// specializes, and a redefining feature may not weaken its lower bound. Both are
// warnings, the lower-bound rule applies to redefinitions of non-end features
// only, and both features must be ends or both not ends.
func (cc *constraintChecker) checkMultiplicityConformance(sym *symbols.Symbol) {
	subRange, ok := cc.conformanceMultiplicity(sym)
	if !ok {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		redefines := rel.Kind == ast.RelRedefines
		if !redefines && rel.Kind != ast.RelSubsets {
			continue
		}
		target := cc.resolveRelationshipTarget(sym, rel)
		if target == nil || target == sym {
			continue
		}
		supRange, ok := cc.conformanceMultiplicity(target)
		if !ok || declaresEndFeature(sym) != declaresEndFeature(target) {
			continue
		}
		if redefines && !declaresEndFeature(sym) && lowerBoundWeakened(subRange, supRange) {
			cc.diags = append(cc.diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     rel.Target.Span(),
				Message:  msgRedefinitionMultiplicityConformance,
				Code:     "redefinition-multiplicity",
				Source:   "constraint",
			})
		}
		if upperBoundWidened(subRange, supRange) {
			cc.diags = append(cc.diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     rel.Target.Span(),
				Message:  msgSubsettingMultiplicityConformance,
				Code:     "subsetting-multiplicity",
				Source:   "constraint",
			})
		}
	}
}

// lowerBoundWeakened reports whether sub's lower bound is below sup's. An
// unbounded lower conforms only to an unbounded or zero one.
func lowerBoundWeakened(sub, sup semantics.Range) bool {
	if !sub.Lower.Known || !sup.Lower.Known || sup.Lower.Infinite {
		return false
	}
	if sub.Lower.Infinite {
		return sup.Lower.Value > 0
	}
	return sub.Lower.Value < sup.Lower.Value
}

// upperBoundWidened reports whether sub's upper bound exceeds sup's.
func upperBoundWidened(sub, sup semantics.Range) bool {
	if !sub.Upper.Known || !sup.Upper.Known || sup.Upper.Infinite {
		return false
	}
	if sub.Upper.Infinite {
		return true
	}
	return sub.Upper.Value > sup.Upper.Value
}

// conformanceMultiplicity is the multiplicity a feature is held to by the
// conformance rules: the declared one, or the implicit 1..1 the reference gives
// an end feature and an attribute, item, part or port usage that is owned by a
// type and subsets no feature that a type owns. Other usages have none, and the
// rules then have nothing to compare (SysML v2 §7.9.2 default multiplicity).
func (cc *constraintChecker) conformanceMultiplicity(sym *symbols.Symbol) (semantics.Range, bool) {
	if rng, ok := cc.model.MultiplicityOf(sym); ok {
		return rng, true
	}
	if declaresEndFeature(sym) {
		return semantics.AssumedRange(), true
	}
	if !defaultMultiplicityKind(sym) || !ownedByType(sym) {
		return semantics.Range{}, false
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		if subsetted := cc.resolveRelationshipTarget(sym, rel); subsetted != nil && ownedByType(subsetted) {
			return semantics.Range{}, false
		}
	}
	return semantics.AssumedRange(), true
}

// defaultMultiplicityKind reports whether a usage's kind is one the reference
// gives a default multiplicity of 1..1 (AttributeUsage, ItemUsage — which
// PartUsage specializes — and PortUsage).
func defaultMultiplicityKind(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	u, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage {
		return false
	}
	// The keyword, not the kind: KerML's `feature` parses to an attribute usage
	// and the reference gives it no default multiplicity.
	switch u.Keyword {
	case "attribute", "item", "part", "port":
		return true
	}
	return false
}

// ownedByType reports whether a feature is owned by a definition or usage rather
// than by a package or namespace.
func ownedByType(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	switch owner.Kind {
	case symbols.SymbolPackage, symbols.SymbolNamespace:
		return false
	}
	return true
}

// declaresEndFeature reports whether a feature is a connector end or carries the
// `end` modifier.
func declaresEndFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		return true
	case *ast.Usage:
		return d.IsEnd
	}
	return false
}

// resolveRelationshipTarget resolves the feature a relationship of sym names,
// following an alias to its target.
func (cc *constraintChecker) resolveRelationshipTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if sym == nil || rel == nil || ast.AsQualifiedName(rel.Target) == nil {
		return nil
	}
	return cc.model.RelationshipTarget(sym, rel)
}
