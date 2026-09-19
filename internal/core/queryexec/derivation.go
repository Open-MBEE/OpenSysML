package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Library elements a derivation (a connection conforming to Derivation, its ends
// stating roles by subsetting or metadata) and a refinement dependency are recognized by.
const (
	derivationDefinitionFQN  = "DerivationConnections::Derivation"
	originalRequirementsFQN  = "DerivationConnections::originalRequirements"
	derivedRequirementsFQN   = "DerivationConnections::derivedRequirements"
	originalMetadataFQN      = "RequirementDerivation::OriginalRequirementMetadata"
	derivedMetadataFQN       = "RequirementDerivation::DerivedRequirementMetadata"
	requirementCheckFQN      = "Requirements::RequirementCheck"
	refinementMetadataFQN    = "ModelingMetadata::Refinement"
	metaclassFeatureClient   = "client"
	metaclassFeatureSupplier = "supplier"
)

// derivationRole is the role a derivation end states for its requirements.
type derivationRole int

const (
	roleUnstated derivationRole = iota
	roleOriginal
	roleDerived
)

// derivationEnd is one end of a derivation: its requirements and their stated role.
type derivationEnd struct {
	role      derivationRole
	referents []*symbols.Symbol
}

// scanDerivation records the edges from each original requirement to each derived one:
// a usage relates the requirements its ends attach to, a definition those its ends are typed by.
func (e *executor) scanDerivation(edges *relationshipEdges, sym *symbols.Symbol) {
	derivation := e.librarySymbol(derivationDefinitionFQN)
	if derivation == nil || !e.isDerivation(sym, derivation) {
		return
	}
	var ends []derivationEnd
	switch sym.Decl.(type) {
	case *ast.Usage:
		ends = e.derivationUsageEnds(sym)
	case *ast.Definition:
		ends = e.derivationDefinitionEnds(sym)
	}
	originals, derived := splitDerivationEnds(ends)
	for _, original := range originals {
		for _, target := range derived {
			addEdge(edges, original, target)
		}
	}
}

// isDerivation reports whether sym is a connection conforming to Derivation without being it.
func (e *executor) isDerivation(sym, derivation *symbols.Symbol) bool {
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
	return !symbols.SameElement(sym, derivation) && e.context.Model.Conforms(sym, derivation)
}

// derivationUsageEnds returns a usage's ends in declaration order: the `connect`
// clause's, then its body's `end` features, each with the role its end feature states.
func (e *executor) derivationUsageEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, attachment := range e.context.Model.ConnectorEndAttachments(sym) {
		end := derivationEnd{role: e.derivationRoleOf(attachment.EndFeature)}
		if attachment.Attachment != nil {
			if target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment); ok && target != nil {
				end.referents = append(end.referents, target)
			}
		}
		ends = append(ends, end)
	}
	for _, feature := range bodyEnds(sym) {
		end := derivationEnd{role: e.derivationRoleOf(feature)}
		for _, rel := range relationshipsOfEnd(feature) {
			if rel.Kind != ast.RelReferences && rel.Kind != ast.RelSubsets {
				continue
			}
			target, ok := e.context.Resolver.ResolveTarget(feature.OwnerScope, rel.Target)
			if !ok || target == nil || e.isDerivationRoleFeature(target) {
				continue
			}
			end.referents = append(end.referents, target)
		}
		ends = append(ends, end)
	}
	return ends
}

// derivationDefinitionEnds returns a definition's effective ends — its own, then the
// inherited ones none of them redefines — each standing for its requirement types.
func (e *executor) derivationDefinitionEnds(sym *symbols.Symbol) []derivationEnd {
	var ends []derivationEnd
	for _, feature := range e.context.Model.EndFeatures(sym) {
		ends = append(ends, derivationEnd{
			role:      e.derivationRoleOf(feature),
			referents: e.requirementTypes(feature),
		})
	}
	return ends
}

// requirementTypes is the requirement definitions among an end's effective types,
// declared on it or inherited from the ends it redefines.
func (e *executor) requirementTypes(feature *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, typ := range e.context.Model.FeatureTypeSet(feature) {
		if e.conformsToLibrary(typ, requirementCheckFQN) {
			out = append(out, typ)
		}
	}
	return out
}

// derivationRoleOf is the role an end feature states by its metadata or by conforming
// to the library's role features, directly or through the end it redefines.
func (e *executor) derivationRoleOf(feature *symbols.Symbol) derivationRole {
	if feature == nil {
		return roleUnstated
	}
	switch {
	case e.annotatedBy(feature, originalMetadataFQN), e.conformsToLibrary(feature, originalRequirementsFQN):
		return roleOriginal
	case e.annotatedBy(feature, derivedMetadataFQN), e.conformsToLibrary(feature, derivedRequirementsFQN):
		return roleDerived
	}
	return roleUnstated
}

// isDerivationRoleFeature reports whether subsetting target states a role rather
// than naming a requirement.
func (e *executor) isDerivationRoleFeature(target *symbols.Symbol) bool {
	return e.conformsToLibrary(target, originalRequirementsFQN) || e.conformsToLibrary(target, derivedRequirementsFQN)
}

// splitDerivationEnds applies the stated roles; when no end states the original,
// the first end without a role is the original and the other unstated ends are derived.
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

// bodyEnds returns the `end` features sym's body declares, in declaration order.
func bodyEnds(sym *symbols.Symbol) []*symbols.Symbol {
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

// relationshipsOfEnd returns the declared relationships of an `end` feature that name a target.
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

// scanRefinement records the edges of a dependency annotated Refinement, from each
// client to each supplier; a plain dependency states none.
func (e *executor) scanRefinement(edges *relationshipEdges, sym *symbols.Symbol) {
	if _, ok := sym.Decl.(*ast.Dependency); !ok || !e.annotatedBy(sym, refinementMetadataFQN) {
		return
	}
	clients, _ := e.context.Model.ReflectiveElements(sym, metaclassFeatureClient)
	suppliers, _ := e.context.Model.ReflectiveElements(sym, metaclassFeatureSupplier)
	for _, client := range clients {
		for _, supplier := range suppliers {
			addEdge(edges, client, supplier)
		}
	}
}

// annotatedBy reports whether an annotation of sym is typed by, or specializes, the metadata at fqn.
func (e *executor) annotatedBy(sym *symbols.Symbol, fqn string) bool {
	def := e.librarySymbol(fqn)
	if def == nil {
		return false
	}
	for _, annotation := range e.context.Model.AnnotationFactsOf(sym) {
		for _, actual := range e.context.Index.LookupQualified(annotation.TypeFQN) {
			if symbols.SameElement(actual, def) || e.context.Model.Conforms(actual, def) {
				return true
			}
		}
	}
	return false
}

// conformsToLibrary reports whether sym is, or conforms to, the element at fqn.
func (e *executor) conformsToLibrary(sym *symbols.Symbol, fqn string) bool {
	target := e.librarySymbol(fqn)
	return target != nil && (symbols.SameElement(sym, target) || e.context.Model.Conforms(sym, target))
}

// librarySymbol returns the element at a library qualified name, or nil when it is not loaded.
func (e *executor) librarySymbol(fqn string) *symbols.Symbol {
	for _, match := range e.context.Index.LookupQualified(fqn) {
		if match != nil {
			return match
		}
	}
	return nil
}
