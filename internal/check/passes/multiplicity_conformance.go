package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// conformanceMultiplicity is the declared range or the implicit [1..1] for an
// end feature or eligible usage with no relationship to a type-owned feature.
func (cc *constraintChecker) conformanceMultiplicity(sym *symbols.Symbol) (semantics.Range, bool) {
	if rng, ok := cc.model.MultiplicityOf(sym); ok {
		return rng, true
	}
	if declaresEndFeature(sym) {
		return semantics.AssumedRange(), true
	}
	if !cc.model.ImplicitMultiplicityApplies(sym) {
		return semantics.Range{}, false
	}
	return semantics.AssumedRange(), true
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
