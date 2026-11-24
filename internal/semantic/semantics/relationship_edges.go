package semantics

import (
	"slices"

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
	// SourcePath and TargetPath are a derivation end as written, outermost first and
	// less the features the derivation lies inside (`a.r` is [a r]); nil when unnamed.
	SourcePath, TargetPath []*symbols.Symbol
}

// SameEnds reports an edge from an element to itself reached the same way:
// `a.r` and `b.r` are two requirements though both name one inherited r.
func (e RelationshipEdge) SameEnds() bool {
	return symbols.SameElement(e.Source, e.Target) &&
		slices.EqualFunc(reachedThrough(e.Source, e.SourcePath), reachedThrough(e.Target, e.TargetPath), symbols.SameElement)
}

// reachedThrough is path without each feature that declares the one after it,
// so `p.r` and `r` agree where p declares r; a nil path is element alone.
func reachedThrough(element *symbols.Symbol, path []*symbols.Symbol) []*symbols.Symbol {
	if len(path) == 0 {
		return []*symbols.Symbol{element}
	}
	out := make([]*symbols.Symbol, 0, len(path))
	for i, sym := range path {
		if i+1 < len(path) && path[i+1].OwnerScope != nil && symbols.SameElement(path[i+1].OwnerScope.Owner(), sym) {
			continue
		}
		out = append(out, sym)
	}
	return out
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
	var requirement, subject *symbols.Symbol
	// originals are the requirements a declaring satisfy or verify is typed by
	// or subsets, in declaration order.
	var originals []*symbols.Symbol
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		target, ok := m.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			continue
		}
		if aliased, isAlias := m.resolver.ResolveAliasTarget(target); isAlias && aliased != nil {
			target = aliased
		}
		switch rel.Kind {
		case ast.RelReferences:
			if !usage.DeclaresRequirement {
				requirement = target
			}
		case ast.RelSubsets, ast.RelTyping:
			if usage.DeclaresRequirement && isRequirementKind(target.Kind) {
				originals = append(originals, target)
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
	for _, original := range originals {
		out = append(out, RelationshipEdge{Source: subject, Target: original})
	}
	return out
}

// isRequirementKind reports a requirement definition or usage, or one of the
// viewpoint and concern kinds specializing them.
func isRequirementKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolRequirementDef, symbols.SymbolViewpointDef, symbols.SymbolConcernDef,
		symbols.SymbolRequirementUsage, symbols.SymbolSatisfyRequirementUsage,
		symbols.SymbolViewpointUsage, symbols.SymbolConcernUsage:
		return true
	}
	return false
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
			out = append(out, RelationshipEdge{Source: original.element, Target: target.element, SourcePath: original.path, TargetPath: target.path})
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
	referents []derivationReferent
}

// derivationReferent is a requirement an end refers to and the path naming it.
type derivationReferent struct {
	element *symbols.Symbol
	path    []*symbols.Symbol
}

// derivationPath is the path node is written as in scope, less the leading
// features scope lies inside: there `c.r` names what a bare `r` does.
func (m *Model) derivationPath(scope *symbols.Scope, node ast.Node) []*symbols.Symbol {
	path := m.attachmentPath(scope, node)
	for len(path) > 1 && encloses(path[0], scope) {
		path = path[1:]
	}
	return path
}

func encloses(sym *symbols.Symbol, scope *symbols.Scope) bool {
	for s := scope; s != nil; s = s.Parent() {
		if symbols.SameElement(s.Owner(), sym) {
			return true
		}
	}
	return false
}

func (m *Model) derivationUsageEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, attachment := range m.ConnectorEndAttachments(sym) {
		// A body end is read below with the role its own declaration states.
		if _, clause := attachment.End.(*ast.ConnectorEnd); !clause {
			continue
		}
		end := derivationEnd{role: m.derivationRoleOf(attachment.EndFeature)}
		if attachment.Attachment != nil {
			if target, ok := m.resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment); ok && target != nil {
				end.referents = append(end.referents, derivationReferent{target, m.derivationPath(sym.OwnerScope, attachment.Attachment)})
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
			end.referents = append(end.referents, derivationReferent{target, m.derivationPath(feature.OwnerScope, rel.Target)})
		}
		ends = append(ends, end)
	}
	return ends
}

// derivationDefinitionEnds reads a derivation def's ends: each refers to the
// requirements typing it, and to any requirement usage it subsets or references.
func (m *Model) derivationDefinitionEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, feature := range m.EndFeatures(sym) {
		end := derivationEnd{role: m.derivationRoleOf(feature)}
		for _, typ := range m.requirementTypes(feature) {
			end.referents = append(end.referents, derivationReferent{element: typ})
		}
		for _, rel := range relationshipsOfEnd(feature) {
			if rel.Kind != ast.RelReferences && rel.Kind != ast.RelSubsets {
				continue
			}
			target, ok := m.resolver.ResolveTarget(feature.OwnerScope, rel.Target)
			if !ok || target == nil || m.isDerivationRoleFeature(target) || !m.conformsToLibrary(target, "Requirements::RequirementCheck") {
				continue
			}
			end.referents = append(end.referents, derivationReferent{target, m.derivationPath(feature.OwnerScope, rel.Target)})
		}
		ends = append(ends, end)
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

// isDerivationRoleFeature reports the library's own role features (as declared or as
// `Derivation` redefines them); a model requirement tagged `@original` is a participant.
func (m *Model) isDerivationRoleFeature(target *symbols.Symbol) bool {
	return m.libraryDeclared(target) &&
		(m.conformsToLibrary(target, "DerivationConnections::originalRequirements") ||
			m.conformsToLibrary(target, "DerivationConnections::derivedRequirements"))
}

func splitDerivationEnds(ends []derivationEnd) (originals, derived []derivationReferent) {
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
