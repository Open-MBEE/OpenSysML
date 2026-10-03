package semantics

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ownedUsageMetaclasses maps the suffix of SysML's Usage::nested* and
// Definition::owned* derived properties to the metaclass their values conform to.
var ownedUsageMetaclasses = map[string]string{
	"Usage": "Usage", "Reference": "ReferenceUsage", "Attribute": "AttributeUsage",
	"Enumeration": "EnumerationUsage", "Occurrence": "OccurrenceUsage", "Item": "ItemUsage",
	"Part": "PartUsage", "Port": "PortUsage", "Connection": "ConnectorAsUsage",
	"Flow": "FlowUsage", "Interface": "InterfaceUsage", "Allocation": "AllocationUsage",
	"Action": "ActionUsage", "State": "StateUsage", "Transition": "TransitionUsage",
	"Calculation": "CalculationUsage", "Constraint": "ConstraintUsage",
	"Requirement": "RequirementUsage", "Concern": "ConcernUsage", "Case": "CaseUsage",
	"AnalysisCase": "AnalysisCaseUsage", "VerificationCase": "VerificationCaseUsage",
	"UseCase": "UseCaseUsage", "View": "ViewUsage", "Viewpoint": "ViewpointUsage",
	"Rendering": "RenderingUsage", "Metadata": "MetadataUsage",
}

// reflectiveOwnedUsages derives a metaclass's `nested*`/`owned*` property of
// sym: its owned members that are usages of the metaclass the suffix names,
// `nested*` on a usage and `owned*` on a definition (SysML v2 §8.3, the
// Usage/Definition derived properties).
func (m *Model) reflectiveOwnedUsages(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	var prefix string
	switch {
	case sym.DeclaresUsage() || m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage"):
		prefix = "nested"
	case sym.DeclaresDefinition():
		prefix = "owned"
	default:
		return nil, false
	}
	suffix, ok := strings.CutPrefix(feature, prefix)
	if !ok {
		return nil, false
	}
	metaclass, ok := ownedUsageMetaclasses[suffix]
	if !ok {
		return nil, false
	}
	var out []*symbols.Symbol
	for _, member := range ownedMembersOf(sym) {
		if !member.IsFeature() || IsVariant(member) {
			continue
		}
		if m.metaclassConforms(member, sysmlMetaclassPrefix+metaclass) {
			out = append(out, member)
		}
	}
	return out, true
}
