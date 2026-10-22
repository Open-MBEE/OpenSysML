package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// requirementElements is ReflectiveElements for the derived features of the
// SysML requirement, case and satisfaction metaclasses that their memberships
// imply (SysML 8.3.19 Requirements, 8.3.20 Cases); ok is false for any other
// feature or an element of another metaclass.
func (m *Model) requirementElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	switch feature {
	case "requiredConstraint", "assumedConstraint":
		if !m.conformsToAny(sym, "RequirementDefinition", "RequirementUsage") {
			return nil, false
		}
		kind := "requirement"
		if feature == "assumedConstraint" {
			kind = "assumption"
		}
		return m.requirementConstraints(sym, kind), true
	case "framedConcern":
		if !m.conformsToAny(sym, "RequirementDefinition", "RequirementUsage") {
			return nil, false
		}
		// framedConcern reads featureMembership, so an inherited frame counts.
		out := []*symbols.Symbol{}
		for _, member := range m.membersIncludingAnonymous(sym) {
			if kind, ok := member.UsageKind(); ok && kind == ast.UsageFramedConcern {
				out = append(out, member)
			}
		}
		return out, true
	case "verifiedRequirement":
		if !m.conformsToAny(sym, "VerificationCaseDefinition", "VerificationCaseUsage") {
			return nil, false
		}
		return m.verifiedRequirements(sym), true
	case "actorParameter", "stakeholderParameter":
		if !m.conformsToAny(sym, "RequirementDefinition", "RequirementUsage", "CaseDefinition", "CaseUsage") {
			return nil, false
		}
		kind := ast.UsageActor
		if feature == "stakeholderParameter" {
			kind = ast.UsageStakeholder
		}
		owned, inherited := m.ActorsOf(sym)
		out := []*symbols.Symbol{}
		for _, actor := range append(owned, inherited...) {
			if k, ok := actor.UsageKind(); ok && k == kind {
				out = append(out, actor)
			}
		}
		return out, true
	case "assertedConstraint", "satisfiedRequirement":
		metaclass := "AssertConstraintUsage"
		if feature == "satisfiedRequirement" {
			metaclass = "SatisfyRequirementUsage"
		}
		if !m.reflectiveMetaclassConforms(sym, metaclass) {
			return nil, false
		}
		return m.referencedConstraintOf(sym, feature), true
	case "satisfyingFeature":
		if !m.reflectiveMetaclassConforms(sym, "SatisfyRequirementUsage") {
			return nil, false
		}
		return m.satisfyingFeatureOf(sym), true
	}
	return nil, false
}

// conformsToAny reports whether sym's metaclass conforms to one of the named ones.
func (m *Model) conformsToAny(sym *symbols.Symbol, names ...string) bool {
	for _, name := range names {
		if m.reflectiveMetaclassConforms(sym, name) {
			return true
		}
	}
	return false
}

// requirementConstraints is the ownedConstraint of each RequirementConstraintMembership
// of sym with the given kind, in declaration order (SysML 8.3.19
// deriveRequirementDefinitionRequiredConstraint / AssumedConstraint).
func (m *Model) requirementConstraints(sym *symbols.Symbol, kind string) []*symbols.Symbol {
	out := []*symbols.Symbol{}
	for _, membership := range m.OwnedMemberships(sym) {
		if membership.Implicit == nil {
			continue
		}
		member := membership.Implicit.Member
		if k, ok := requirementConstraintKind(member); ok && k == kind {
			out = append(out, member)
		}
	}
	return out
}

// verifiedRequirements is the verifiedRequirement of each
// RequirementVerificationMembership of the case's objective, in declaration
// order (SysML 8.3.20 VerificationCaseDefinition::verifiedRequirement).
func (m *Model) verifiedRequirements(sym *symbols.Symbol) []*symbols.Symbol {
	out := []*symbols.Symbol{}
	owned, inherited := m.ObjectivesOf(sym)
	objectives := append(owned, inherited...)
	if len(objectives) == 0 || objectives[0] == nil {
		return out
	}
	for _, membership := range m.OwnedMemberships(objectives[0]) {
		if membership.Implicit == nil || !isVerifiedRequirementMember(membership.Implicit.Member) {
			continue
		}
		out = append(out, m.referencedConstraintOf(membership.Implicit.Member, "verifiedRequirement")...)
	}
	return out
}

// satisfyingFeatureOf is the feature a satisfy usage's `by` clause binds its
// subject to (SysML 8.3.19 deriveSatisfyRequirementUsageSatisfyingFeature); a
// satisfaction stating no `by` binds its subject to nothing.
func (m *Model) satisfyingFeatureOf(sym *symbols.Symbol) []*symbols.Symbol {
	out := []*symbols.Symbol{}
	if sym.Recorded() {
		if targets := m.RecordedRelationshipTargets(sym, ast.RelSubject); len(targets) > 0 {
			out = append(out, targets[0])
		}
		return out
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelSubject {
			continue
		}
		if target := m.RelationshipTarget(sym, rel); target != nil {
			out = append(out, target)
		}
		break
	}
	return out
}

// requirementTexts is RequirementDefinition::text / RequirementUsage::text: the
// bodies of the requirement's documentation (SysML 8.3.19
// deriveRequirementDefinitionText `text = documentation.body`).
func (m *Model) requirementTexts(sym *symbols.Symbol) ([]symbols.FilterValue, bool) {
	if !m.conformsToAny(sym, "RequirementDefinition", "RequirementUsage") {
		return nil, false
	}
	bodies := m.DocumentationOf(sym)
	values := make([]symbols.FilterValue, 0, len(bodies))
	for _, body := range bodies {
		values = append(values, symbols.FilterValue{Kind: symbols.FilterValueString, Str: body})
	}
	return values, true
}
