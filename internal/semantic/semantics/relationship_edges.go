package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// RelationshipKind names a relationship represented by resolved source-target edges.
type RelationshipKind string

const (
	RelationshipConnection   RelationshipKind = "connection"
	RelationshipAllocation   RelationshipKind = "allocation"
	RelationshipSatisfaction RelationshipKind = "satisfaction"
	RelationshipVerification RelationshipKind = "verification"
	RelationshipDerivation   RelationshipKind = "derivation"
	RelationshipRefinement   RelationshipKind = "refinement"
	RelationshipDependency   RelationshipKind = "dependency"
)

// RelationshipEdge is one resolved source-to-target relationship.
type RelationshipEdge struct {
	Source *symbols.Symbol
	Target *symbols.Symbol
}

// RelationshipEdgesOf returns the resolved edges sym's declaration states, in declaration order.
func (m *Model) RelationshipEdgesOf(sym *symbols.Symbol, kind RelationshipKind) []RelationshipEdge {
	if m == nil || sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	switch kind {
	case RelationshipConnection, RelationshipAllocation:
		return m.connectorEdges(sym, kind)
	case RelationshipSatisfaction, RelationshipVerification:
		return m.satisfactionEdges(sym, kind)
	case RelationshipDerivation:
		return m.derivationEdges(sym)
	case RelationshipRefinement:
		return m.refinementEdges(sym)
	case RelationshipDependency:
		return m.dependencyEdges(sym)
	}
	return nil
}

func (m *Model) connectorEdges(sym *symbols.Symbol, kind RelationshipKind) []RelationshipEdge {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !connectorRelationship(usage.Kind, kind) || !m.IsConnectorUsage(sym) {
		return nil
	}
	var ends []*symbols.Symbol
	for _, attachment := range m.ConnectorEndAttachments(sym) {
		if attachment.Attachment == nil {
			continue
		}
		target, ok := m.resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment)
		if ok && target != nil {
			ends = append(ends, target)
		}
	}
	if len(ends) < 2 {
		return nil
	}
	out := make([]RelationshipEdge, 0, len(ends)-1)
	for _, target := range ends[1:] {
		out = append(out, RelationshipEdge{Source: ends[0], Target: target})
	}
	return out
}

func connectorRelationship(usage ast.UsageKind, kind RelationshipKind) bool {
	switch kind {
	case RelationshipConnection:
		return usage == ast.UsageConnection || usage == ast.UsageConnector || usage == ast.UsageInterface
	case RelationshipAllocation:
		return usage == ast.UsageAllocation
	}
	return false
}

func (m *Model) satisfactionEdges(sym *symbols.Symbol, kind RelationshipKind) []RelationshipEdge {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy || (usage.Keyword == "verify") != (kind == RelationshipVerification) {
		return nil
	}
	var requirement, definition, subject *symbols.Symbol
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		target, ok := m.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets:
			if !usage.DeclaresRequirement {
				requirement = target
			}
		case ast.RelTyping:
			if usage.DeclaresRequirement && target.Kind == symbols.SymbolRequirementDef {
				definition = target
			}
		case ast.RelSubject:
			subject = target
		}
	}
	if usage.DeclaresRequirement {
		requirement = sym
	}
	if subject == nil && sym.OwnerScope != nil {
		subject = sym.OwnerScope.Owner()
		if kind == RelationshipVerification && subject != nil && isObjectiveUsage(subject.Decl) && subject.OwnerScope != nil {
			subject = subject.OwnerScope.Owner()
		}
	}
	if subject == nil || requirement == nil {
		return nil
	}
	out := []RelationshipEdge{{Source: subject, Target: requirement}}
	if definition != nil {
		out = append(out, RelationshipEdge{Source: subject, Target: definition})
	}
	return out
}

func isObjectiveUsage(decl ast.Node) bool {
	usage, ok := decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageObjective
}

func (m *Model) derivationEdges(sym *symbols.Symbol) []RelationshipEdge {
	derivation := m.librarySymbol("DerivationConnections::Derivation")
	if derivation == nil || !m.isDerivation(sym, derivation) {
		return nil
	}
	var ends []derivationEnd
	switch sym.Decl.(type) {
	case *ast.Usage:
		ends = m.derivationUsageEnds(sym)
	case *ast.Definition:
		ends = m.derivationDefinitionEnds(sym)
	}
	originals, derived := splitDerivationEnds(ends)
	var out []RelationshipEdge
	for _, original := range originals {
		for _, target := range derived {
			out = append(out, RelationshipEdge{Source: original, Target: target})
		}
	}
	return out
}

func (m *Model) isDerivation(sym, derivation *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefConnection {
			return false
		}
	case *ast.Usage:
		if decl.Kind != ast.UsageConnection {
			return false
		}
	default:
		return false
	}
	return !symbols.SameElement(sym, derivation) && m.Conforms(sym, derivation)
}

type derivationRole int

const (
	roleUnstated derivationRole = iota
	roleOriginal
	roleDerived
)

type derivationEnd struct {
	role      derivationRole
	referents []*symbols.Symbol
}

func (m *Model) derivationUsageEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, attachment := range m.ConnectorEndAttachments(sym) {
		end := derivationEnd{role: m.derivationRoleOf(attachment.EndFeature)}
		if attachment.Attachment != nil {
			if target, ok := m.resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment); ok && target != nil {
				end.referents = append(end.referents, target)
			}
		}
		ends = append(ends, end)
	}
	for _, feature := range relationshipBodyEnds(sym) {
		end := derivationEnd{role: m.derivationRoleOf(feature)}
		for _, rel := range relationshipsOfEnd(feature) {
			if rel.Kind != ast.RelReferences && rel.Kind != ast.RelSubsets {
				continue
			}
			target, ok := m.resolver.ResolveTarget(feature.OwnerScope, rel.Target)
			if !ok || target == nil || m.isDerivationRoleFeature(target) {
				continue
			}
			end.referents = append(end.referents, target)
		}
		ends = append(ends, end)
	}
	return ends
}

func (m *Model) derivationDefinitionEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, feature := range m.EndFeatures(sym) {
		ends = append(ends, derivationEnd{role: m.derivationRoleOf(feature), referents: m.requirementTypes(feature)})
	}
	return ends
}

func (m *Model) requirementTypes(feature *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, typ := range m.FeatureTypeSet(feature) {
		if m.conformsToLibrary(typ, "Requirements::RequirementCheck") {
			out = append(out, typ)
		}
	}
	return out
}

func (m *Model) derivationRoleOf(feature *symbols.Symbol) derivationRole {
	if feature == nil {
		return roleUnstated
	}
	switch {
	case m.annotatedBy(feature, "RequirementDerivation::OriginalRequirementMetadata"),
		m.conformsToLibrary(feature, "DerivationConnections::originalRequirements"):
		return roleOriginal
	case m.annotatedBy(feature, "RequirementDerivation::DerivedRequirementMetadata"),
		m.conformsToLibrary(feature, "DerivationConnections::derivedRequirements"):
		return roleDerived
	}
	return roleUnstated
}

func (m *Model) isDerivationRoleFeature(target *symbols.Symbol) bool {
	return m.conformsToLibrary(target, "DerivationConnections::originalRequirements") ||
		m.conformsToLibrary(target, "DerivationConnections::derivedRequirements")
}

func splitDerivationEnds(ends []derivationEnd) (originals, derived []*symbols.Symbol) {
	hasOriginal := false
	for _, end := range ends {
		if end.role == roleOriginal {
			hasOriginal = true
			break
		}
	}
	for _, end := range ends {
		switch {
		case end.role == roleOriginal:
			originals = append(originals, end.referents...)
		case end.role == roleDerived:
			derived = append(derived, end.referents...)
		case !hasOriginal:
			hasOriginal = true
			originals = append(originals, end.referents...)
		default:
			derived = append(derived, end.referents...)
		}
	}
	return originals, derived
}

func relationshipBodyEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var ends []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.IsEnd && !seen[member] {
			seen[member] = true
			ends = append(ends, member)
		}
		return true
	})
	return ends
}

func relationshipsOfEnd(feature *symbols.Symbol) []*ast.Relationship {
	usage, ok := feature.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	var out []*ast.Relationship
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Target != nil {
			out = append(out, rel)
		}
	}
	return out
}

func (m *Model) refinementEdges(sym *symbols.Symbol) []RelationshipEdge {
	if _, ok := sym.Decl.(*ast.Dependency); !ok || !m.annotatedBy(sym, "ModelingMetadata::Refinement") {
		return nil
	}
	return dependencyEnds(m, sym)
}

func (m *Model) dependencyEdges(sym *symbols.Symbol) []RelationshipEdge {
	if _, ok := sym.Decl.(*ast.Dependency); !ok {
		return nil
	}
	return dependencyEnds(m, sym)
}

func dependencyEnds(m *Model, sym *symbols.Symbol) []RelationshipEdge {
	clients, _ := m.ReflectiveElements(sym, "client")
	suppliers, _ := m.ReflectiveElements(sym, "supplier")
	var out []RelationshipEdge
	for _, client := range clients {
		for _, supplier := range suppliers {
			out = append(out, RelationshipEdge{Source: client, Target: supplier})
		}
	}
	return out
}

func (m *Model) conformsToLibrary(sym *symbols.Symbol, fqn string) bool {
	target := m.librarySymbol(fqn)
	return target != nil && (symbols.SameElement(sym, target) || m.Conforms(sym, target))
}
