package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// nonCompositeUsageClasses are the usage metaclasses that are references
// unless written otherwise (SysML § 7.6.3 Usage; the pilot's usage post-processing).
var nonCompositeUsageClasses = []string{
	"AttributeUsage", "BindingConnectorAsUsage", mEventOccurrenceUsage, "ExhibitStateUsage",
	"IncludeUseCaseUsage", "PerformActionUsage", mReferenceUsage, "SuccessionAsUsage",
}

// ownership resolves the membership a usage is owned through and the owner it
// stands under, from whichever side of the edge a graph states it.
type ownership func(subject rdf.Term) (membership, owner rdf.Term)

// graphOwnership reads ownership from the element's own side of the edge.
func graphOwnership(graph *rdf.Graph) ownership {
	return func(subject rdf.Term) (rdf.Term, rdf.Term) {
		membership := firstIRI(graph, subject, pOwningRelationship, pOwningMembership)
		owner := firstIRI(graph, membership, pOwningRelatedElement, pMembershipOwningNamespace, pOwner)
		if !owner.IsIRI() {
			owner = firstIRI(graph, subject, pOwner, pOwningNamespace)
		}
		return membership, owner
	}
}

// expectedFeaturingType is the type a usage is a feature of: the owning type of
// its FeatureMembership, or, for a variant of a variation usage, that usage's own
// (SysML § 7.6.3; the pilot's UsageUtil.getExpectedFeaturingTypeOf).
func expectedFeaturingType(graph *rdf.Graph, meta func(rdf.Term) string, own ownership, subject rdf.Term) rdf.Term {
	membership, owner := own(subject)
	if !membership.IsIRI() || !owner.IsIRI() {
		return rdf.Term{}
	}
	switch {
	case ontology.IsAncestorOrSelf(meta(membership), mFeatureMembership) && ontology.IsAncestorOrSelf(meta(owner), "Type"):
		return owner
	case ontology.IsAncestorOrSelf(meta(owner), "Usage") && graph.BoolValue(owner, rdf.SysML+"isVariation"):
		return expectedFeaturingType(graph, meta, own, owner)
	}
	return rdf.Term{}
}

// usageCompositeByDefault reports whether a SysML usage written with no `ref`
// is composite: one of a compositional kind, not directed, not an end, featured
// by a type other than an attribute, and a port only within a port
// (SysML § 7.6.3 Usage, § 7.11.3 PortUsage; Feature::isComposite is authored,
// Usage::isReference derived as its negation).
func usageCompositeByDefault(graph *rdf.Graph, meta func(rdf.Term) string, own ownership, subject rdf.Term, directed, end bool) bool {
	class := meta(subject)
	if directed || end || !ontology.IsAncestorOrSelf(class, "Usage") {
		return false
	}
	for _, reference := range nonCompositeUsageClasses {
		if ontology.IsAncestorOrSelf(class, reference) {
			return false
		}
	}
	featuring := meta(expectedFeaturingType(graph, meta, own, subject))
	if featuring == "" || ontology.IsAncestorOrSelf(featuring, "AttributeDefinition") || ontology.IsAncestorOrSelf(featuring, "AttributeUsage") {
		return false
	}
	if ontology.IsAncestorOrSelf(class, mPortUsage) {
		return ontology.IsAncestorOrSelf(featuring, "PortDefinition") || ontology.IsAncestorOrSelf(featuring, mPortUsage)
	}
	return true
}
