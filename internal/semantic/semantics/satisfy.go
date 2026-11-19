package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// SatisfyEnds is what a satisfy usage relates, as written: the requirement it
// reference-subsets, or the one it declares itself, and its `by` subject.
type SatisfyEnds struct {
	Requirement         ast.Node
	DeclaresRequirement bool
	Satisfier           ast.Node
}

// SatisfyEndsOf reads the ends of a `satisfy` usage — satisfiedRequirement and
// satisfyingFeature — and reports false for any other element, a `verify`
// included. An end left unwritten is nil.
func SatisfyEndsOf(sym *symbols.Symbol) (SatisfyEnds, bool) {
	if sym == nil || sym.Kind != symbols.SymbolSatisfyRequirementUsage {
		return SatisfyEnds{}, false
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Kind != ast.UsageSatisfy || decl.IsVerifiedRequirement() {
		return SatisfyEnds{}, false
	}
	ends := SatisfyEnds{DeclaresRequirement: decl.DeclaresRequirement}
	if rel := decl.ReferenceSubsetting(); rel != nil && !decl.DeclaresRequirement {
		ends.Requirement = rel.Target
	}
	for _, rel := range decl.Relationships {
		if rel != nil && rel.Kind == ast.RelSubject {
			ends.Satisfier = rel.Target
			break
		}
	}
	return ends, true
}

// VerifyEnds is what a `verify` relates: the requirement it references or declares, and the
// verification case whose objective holds it. An end that does not resolve is nil.
type VerifyEnds struct {
	Requirement *symbols.Symbol
	Case        *symbols.Symbol
}

// VerifyEndsOf reads the ends of a `verify` (SysML 8.3.20 RequirementVerificationMembership)
// and reports false for any other element, a `satisfy` included.
func (m *Model) VerifyEndsOf(sym *symbols.Symbol) (VerifyEnds, bool) {
	if m == nil || sym == nil || sym.Implicit != nil || !isVerifiedRequirementMember(sym) {
		return VerifyEnds{}, false
	}
	ends := VerifyEnds{Requirement: m.referenceSubsettingTarget(sym)}
	if ends.Requirement == nil && !referencesRequirement(sym) {
		ends.Requirement = sym
	}
	if objective := sym.OwnerScope.OwningElement(); roleOf(objective) == objectiveRole {
		if owner := objective.OwnerScope.OwningElement(); m.conformsToAny(owner, "VerificationCaseDefinition", "VerificationCaseUsage") {
			ends.Case = owner
		}
	}
	return ends, true
}

// referencesRequirement reports a `verify r;`, which names its requirement, apart from a
// `verify requirement r;`, which declares it.
func referencesRequirement(member *symbols.Symbol) bool {
	if member.Recorded() {
		return !member.Facts.References.IsZero()
	}
	decl, ok := member.Decl.(*ast.Usage)
	return ok && !decl.DeclaresRequirement && decl.ReferenceSubsetting() != nil
}
