package export

// The normative relationship elements the mapping materializes beside the
// collapsed head properties: the FeatureTyping, Subsetting and friends the
// abstract syntax owns for every `: T`, `:>` and `:>>` a head states, the
// Memberships an expression's referent is related through, and the
// MultiplicityRange bounds are held by. The collapsed properties stay; these
// elements are additional, and the decoder verifies they agree rather than
// choosing between them.

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Metaclasses of the materialized relationship elements.
const (
	mFeatureTyping                   = "FeatureTyping"
	mSubclassification               = "Subclassification"
	mSubsetting                      = "Subsetting"
	mSpecialization                  = "Specialization"
	mRedefinition                    = "Redefinition"
	mMultiplicityRange               = "MultiplicityRange"
	mMembership                      = "Membership"
	mConjugatedPortDefinition        = "ConjugatedPortDefinition"
	mConjugatedPortTyping            = "ConjugatedPortTyping"
	mPortConjugation                 = "PortConjugation"
	mRequirementConstraintMembership = "RequirementConstraintMembership"
	mSubjectMembership               = "SubjectMembership"
	mActorMembership                 = "ActorMembership"
	mStakeholderMembership           = "StakeholderMembership"
	mObjectiveMembership             = "ObjectiveMembership"
	mElementFilterMembership         = "ElementFilterMembership"
	mViewRenderingMembership         = "ViewRenderingMembership"
	mFramedConcernMembership         = "FramedConcernMembership"
	mConstraintUsage                 = "ConstraintUsage"
	mReferenceUsage                  = "ReferenceUsage"
	pMultiplicity                    = "multiplicity"
	pCondition                       = "condition"
	pConjugatedPortDefinition        = "conjugatedPortDefinition"
	pPortDefinition                  = "portDefinition"
	pOwnedPortConjugator             = "ownedPortConjugator"
	pOwnedSubjectParameter           = "ownedSubjectParameter"
	pOwnedConstraint                 = "ownedConstraint"
	pOwnedRendering                  = "ownedRendering"
	pOwnedConcern                    = "ownedConcern"
	pKind                            = "kind"
	pTransitionFeature               = "transitionFeature"
	pEffectAction                    = "effectAction"
	pConjugatedType                  = "conjugatedType"
	pOriginalType                    = "originalType"
	pOriginalPortDefinition          = "originalPortDefinition"
)

// materializeNormative emits the first-class relationship elements the
// collapsed head properties stand for, after every member is encoded so the
// collapsed triples are all present. It runs over the subjects existing when
// it starts: the elements it mints state `type`-family properties of their
// own, which are not collapsed properties to materialize.
func (e *encoder) materializeNormative() {
	subjects := e.graph.Subjects()
	for _, subject := range subjects {
		// A node in the expression namespace is addressed by position, so an
		// expression node's collapsed head properties stay collapsed; a feature
		// an expression body declares owns its relationships as any feature does.
		if strings.HasPrefix(subject.Value, rdf.Expression) {
			e.materializeReferentMemberships(subject)
			if !e.bodyDeclarationNode(subject) {
				continue
			}
		}
		e.materializeReferentMemberships(subject)
		e.materializeRelationships(subject)
	}
}

// bodyDeclarationNode reports whether an expression-namespace node is a
// feature an expression body declares rather than a node of an expression tree.
func (e *encoder) bodyDeclarationNode(subject rdf.Term) bool {
	metaclass := e.metaclassOf(subject)
	return !expressionMetaclasses[metaclass] && ontology.IsAncestorOrSelf(metaclass, mFeature)
}

// normativeRelationship is one collapsed property's materialization: the
// metaclass and id suffix of the element minted per target, the properties
// pointing at the element and at each target, and the owned-* properties the
// element gains on its owner.
type normativeRelationship struct {
	metaclass  string
	suffix     string
	sourceEnds []string
	targetEnds []string
	ownedProps []string
}

// materializeRelationships emits one relationship element per target of each
// collapsed head property subject states, in the property's stated order.
func (e *encoder) materializeRelationships(subject rdf.Term) {
	// A membership is no feature: its collapsed head properties stay collapsed,
	// as an expression node's do.
	if ontology.IsAncestorOrSelf(e.metaclassOf(subject), "Membership") {
		return
	}
	for _, kind := range []ast.RelationshipKind{
		ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelReferences,
	} {
		targets := e.graph.Objects(subject, rdf.SysML+relationshipProperty[kind])
		for i, target := range targets {
			spec, ok := e.relationshipSpec(subject, kind, len(targets), i)
			if !ok {
				continue
			}
			if kind == ast.RelTyping && e.graph.BoolValue(subject, rdf.OpenSysML+xConjugatedTyping) {
				e.emitConjugatedPortTyping(subject, target, spec)
				continue
			}
			e.emitRelationship(subject, target, spec)
		}
	}
}

// relationshipSpec picks the materialization of one collapsed property on
// subject: the relationship class a target takes depends on what the element
// is, and a `subsets` a satisfy or other end-binding head states is written
// as an OwnedReferenceSubsetting (SysML.xtext).
func (e *encoder) relationshipSpec(subject rdf.Term, kind ast.RelationshipKind, count, i int) (normativeRelationship, bool) {
	index := fmt.Sprintf("%d", i)
	switch kind {
	case ast.RelTyping:
		return normativeRelationship{mFeatureTyping, "_ft" + index,
			[]string{"typedFeature", pOwningFeature, pSpecific, pSource, pOwningRelatedElement},
			[]string{"type", pGeneral, pTarget},
			[]string{"ownedTyping", pOwnedSpecialization, pOwnedRelationship}}, true
	case ast.RelSpecializes:
		metaclass := e.metaclassOf(subject)
		switch {
		case ontology.IsAncestorOrSelf(metaclass, "Classifier"):
			return normativeRelationship{mSubclassification, "_sc" + index,
				[]string{"subclassifier", "owningClassifier", pSpecific, pSource, pOwningRelatedElement},
				[]string{"superclassifier", pGeneral, pTarget},
				[]string{"ownedSubclassification", pOwnedSpecialization, pOwnedRelationship}}, true
		case ontology.IsAncestorOrSelf(metaclass, mFeature):
			// A feature's specializes materializes as a Subsetting like its
			// subsets, under a distinct suffix so the two kinds never merge.
			return normativeRelationship{mSubsetting, "_sp" + index,
				[]string{pSubsettingFeature, pOwningFeature, pSpecific, pSource, pOwningRelatedElement},
				[]string{pSubsettedFeature, pGeneral, pTarget},
				[]string{pOwnedSubsetting, pOwnedSpecialization, pOwnedRelationship}}, true
		default:
			return normativeRelationship{mSpecialization, "_sp" + index,
				[]string{pSpecific, pOwningType, pSource, pOwningRelatedElement},
				[]string{pGeneral, pTarget},
				[]string{pOwnedSpecialization, pOwnedRelationship}}, true
		}
	case ast.RelSubsets:
		if e.referenceSubsettingForm(subject) {
			return e.referenceSubsettingSpec(count, i), true
		}
		return normativeRelationship{mSubsetting, "_ss" + index,
			[]string{pSubsettingFeature, pOwningFeature, pSpecific, pSource, pOwningRelatedElement},
			[]string{pSubsettedFeature, pGeneral, pTarget},
			[]string{pOwnedSubsetting, pOwnedSpecialization, pOwnedRelationship}}, true
	case ast.RelRedefines:
		return normativeRelationship{mRedefinition, "_rd" + index,
			[]string{"redefiningFeature", pSubsettingFeature, pOwningFeature, pSpecific, pSource, pOwningRelatedElement},
			[]string{"redefinedFeature", pSubsettedFeature, pGeneral, pTarget},
			[]string{"ownedRedefinition", pOwnedSubsetting, pOwnedSpecialization, pOwnedRelationship}}, true
	case ast.RelReferences:
		return e.referenceSubsettingSpec(count, i), true
	}
	return normativeRelationship{}, false
}

// referenceSubsettingForm reports whether a `subsets` subject states is an
// OwnedReferenceSubsetting: the satisfy head writes the requirement it subsets
// bare, and the other end-binding forms (`perform`, `exhibit`, `include`,
// `assert`, `satisfy`) state their targets through `references`.
func (e *encoder) referenceSubsettingForm(subject rdf.Term) bool {
	return e.metaclassOf(subject) == "SatisfyRequirementUsage" ||
		e.graph.HasProperty(subject, rdf.OpenSysML+xEndForm)
}

// referenceSubsettingSpec is the materialization of a `references` target or
// an end-form `subsets` one: a ReferenceSubsetting per target, `_rs` for a
// single one and `_rs<i>` when several share the suffix.
func (e *encoder) referenceSubsettingSpec(count, i int) normativeRelationship {
	suffix := "_rs"
	if count > 1 {
		suffix = fmt.Sprintf("_rs%d", i)
	}
	return normativeRelationship{mReferenceSubsetting, suffix,
		[]string{pReferencingFeature, pSubsettingFeature, pOwningFeature, pSpecific, pSource, pOwningRelatedElement},
		[]string{pReferencedFeature, pSubsettedFeature, pGeneral, pTarget},
		[]string{pOwnedReferenceSubsetting, pOwnedSubsetting, pOwnedSpecialization, pOwnedRelationship}}
}

// emitConjugatedPortTyping materializes `: ~P` as a ConjugatedPortTyping whose
// type is P's ConjugatedPortDefinition and whose portDefinition is P itself
// (SysML v2 1.0 § 8.3.12 Ports); an external P is named `~P` by the literal.
func (e *encoder) emitConjugatedPortTyping(subject, target rdf.Term, spec normativeRelationship) {
	spec.metaclass = mConjugatedPortTyping
	spec.targetEnds = append([]string{pConjugatedPortDefinition}, spec.targetEnds...)
	conjugated := rdf.String("~" + target.Value)
	if target.IsIRI() {
		conjugated = e.ids.minted(rdf.RelationshipIRI(target, "_conjugated"), target, "_conjugated")
	}
	e.emitRelationship(subject, conjugated, spec)
	relation := e.ids.minted(rdf.RelationshipIRI(subject, spec.suffix), subject, spec.suffix)
	e.graph.Add(relation, e.sysml(pPortDefinition), target)
}

// emitRelationship mints the relationship element between subject and target
// and wires its ends, its identity and its owner's owned-* properties.
func (e *encoder) emitRelationship(subject, target rdf.Term, spec normativeRelationship) {
	e.emitRelationshipAt(subject, target, spec,
		e.ids.minted(rdf.RelationshipIRI(subject, spec.suffix), subject, spec.suffix))
}

// emitRelationshipAt is emitRelationship with the relationship element's IRI
// already minted: a chain's FeatureChaining is an expression node, named by
// position under the chain rather than a `_`-suffixed id, which a `_fc` escape
// could not spell.
func (e *encoder) emitRelationshipAt(subject, target rdf.Term, spec normativeRelationship, relation rdf.Term) {
	if prior, taken := e.claim(relation.Value, "the "+spec.metaclass+" of "+relation.Value); taken && e.idErr == nil {
		e.idErr = &UnsupportedError{
			What: fmt.Sprintf("the %s <%s>", spec.metaclass, relation.Value),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	e.typed(relation, spec.metaclass)
	e.graph.Add(relation, e.sysml(pElementID), rdf.String(rdf.LocalName(relation.Value)))
	for _, property := range spec.sourceEnds {
		e.graph.Add(relation, e.sysml(property), subject)
	}
	for _, property := range spec.targetEnds {
		e.graph.Add(relation, e.sysml(property), target)
	}
	e.graph.Add(relation, e.sysml(pOwner), subject)
	e.graph.Add(relation, e.sysml(pRelatedElement), subject)
	e.graph.Add(relation, e.sysml(pRelatedElement), target)
	for _, property := range spec.ownedProps {
		e.graph.Add(subject, e.sysml(property), relation)
	}
	// A chain feature a head relationship targets is that relationship's
	// owned chain (OwnedRedefinition/OwnedSubsetting/OwnedReferenceSubsetting).
	if e.isChainFeature(target) && !e.graph.HasProperty(target, rdf.SysML+pOwningRelationship) {
		e.graph.Add(relation, e.sysml(pOwnedRelatedElement), target)
		e.graph.Add(target, e.sysml(pOwningRelationship), relation)
		e.graph.Add(target, e.sysml(pOwner), subject)
	}
}

// isChainFeature reports whether term is a feature chain: an unnamed Feature
// owning FeatureChaining links or stating the derived chainingFeature list.
func (e *encoder) isChainFeature(term rdf.Term) bool {
	return e.metaclassOf(term) == mFeature &&
		(e.graph.HasProperty(term, rdf.SysML+pChainingFeature) || e.hasFeatureChaining(term))
}

// hasFeatureChaining reports whether subject owns a FeatureChaining element.
func (e *encoder) hasFeatureChaining(subject rdf.Term) bool {
	for _, rel := range e.graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
		if e.metaclassOf(rel) == mFeatureChaining {
			return true
		}
	}
	return false
}

// featureChainings relates a chain feature to its ordered links: one
// FeatureChaining relationship per link (KerML Feature::ownedFeatureChaining),
// plus the derived chainingFeature list this mapping also states.
func (e *encoder) featureChainings(chain rdf.Term, links []rdf.Term) {
	spec := normativeRelationship{
		metaclass:  mFeatureChaining,
		sourceEnds: []string{"featureChained", pSource, pOwningRelatedElement},
		targetEnds: []string{pChainingFeature, pTarget},
		ownedProps: []string{"ownedFeatureChaining", pOwnedRelationship},
	}
	for i, link := range links {
		position := fmt.Sprintf("fc%d", i)
		e.emitRelationshipAt(chain, link, spec,
			e.ids.mintedNode(rdf.ExpressionIRI(chain, position), chain, position))
		e.graph.Add(chain, e.sysml(pChainingFeature), link)
	}
}

// materializeReferentMemberships emits the plain Membership through which an
// expression node is related to the element it reaches: a FeatureReference-
// Expression's referent and a FeatureChainExpression's target feature.
func (e *encoder) materializeReferentMemberships(subject rdf.Term) {
	var property, slot string
	switch e.metaclassOf(subject) {
	case mFeatureReference:
		property, slot = pReferent, "referent"
	case mFeatureChain:
		property, slot = pTargetFeature, "targetFeature"
	default:
		return
	}
	target, ok := e.graph.Object(subject, rdf.SysML+property)
	if !ok {
		return
	}
	// A body expression the node owns is already its member through the
	// FeatureMembership that owns it, as in the pilot's XMI.
	if owner, owned := e.graph.Object(target, rdf.SysML+pOwner); owned && owner == subject {
		return
	}
	var membership rdf.Term
	if strings.HasPrefix(subject.Value, rdf.Expression) {
		e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
		membership = e.ids.mintedNode(rdf.ExpressionIRI(subject, slot), subject, slot)
	} else {
		membership = e.ids.minted(rdf.RelationshipIRI(subject, "_"+slot), subject, "_"+slot)
		if prior, taken := e.claim(membership.Value, "the "+slot+" membership of "+membership.Value); taken && e.idErr == nil {
			e.idErr = &UnsupportedError{
				What: fmt.Sprintf("the %s membership <%s>", slot, membership.Value),
				Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
			}
		}
	}
	e.typed(membership, mMembership)
	e.graph.Add(membership, e.sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
	e.graph.Add(membership, e.sysml(pMemberElement), target)
	e.graph.Add(membership, e.sysml(pOwner), subject)
	e.graph.Add(membership, e.sysml(pOwningRelatedElement), subject)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
}

// conjugatedPortDefinition emits the conjugate of a port definition: the
// `~P` the conjugated typing of a port usage names, which the metamodel owns
// as a ConjugatedPortDefinition of P with a PortConjugation between them.
func (e *encoder) conjugatedPortDefinition(subject rdf.Term, fqn string, n *ast.Definition) {
	conjugated := e.ids.minted(rdf.RelationshipIRI(subject, "_conjugated"), subject, "_conjugated")
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(conjugated), conjugated, rdf.OwningMembershipSuffix)
	conjugation := e.ids.minted(rdf.RelationshipIRI(conjugated, "_pc"), conjugated, "_pc")
	for _, c := range []struct{ iri, standsFor string }{
		{conjugated.Value, "the conjugated port definition of " + rdf.LocalName(subject.Value)},
		{membership.Value, "the owning membership of the conjugated port definition"},
		{conjugation.Value, "the port conjugation of " + rdf.LocalName(subject.Value)},
	} {
		if prior, taken := e.claim(c.iri, c.standsFor); taken && e.idErr == nil {
			e.idErr = &UnsupportedError{
				What: c.standsFor,
				Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
			}
		}
	}
	e.typed(conjugated, mConjugatedPortDefinition)
	e.graph.Add(conjugated, e.sysml(pElementID), rdf.String(rdf.LocalName(conjugated.Value)))
	// The conjugate is named by position: references read a qualifiedName,
	// which the derived `~` name does not give them.
	e.graph.Add(conjugated, e.sysml(pQualifiedName), rdf.String(fqn+"::"+identity.EscapeName("~"+n.Ident.Name)))
	e.graph.Add(conjugated, e.sysml(pDeclaredName), rdf.String("~"+n.Ident.Name))
	if n.Ident.ShortName != "" {
		e.graph.Add(conjugated, e.sysml(pDeclaredShortName), rdf.String("~"+n.Ident.ShortName))
	}
	e.emitMembershipCore(membership, conjugated, subject, mOwningMembership, true)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(subject, e.sysml(pOwnedMember), conjugated)
	e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
	e.graph.Add(subject, e.sysml(pConjugatedPortDefinition), conjugated)
	// The PortConjugation is a relationship the conjugated definition owns
	// directly, as Conjugation is owned by the conjugated type (KerML).
	e.typed(conjugation, mPortConjugation)
	e.graph.Add(conjugation, e.sysml(pElementID), rdf.String(rdf.LocalName(conjugation.Value)))
	e.graph.Add(conjugation, e.sysml(pConjugatedType), conjugated)
	e.graph.Add(conjugation, e.sysml(pOriginalType), subject)
	e.graph.Add(conjugation, e.sysml(pOriginalPortDefinition), subject)
	e.graph.Add(conjugation, e.sysml(pOwner), conjugated)
	e.graph.Add(conjugation, e.sysml(pOwningRelatedElement), conjugated)
	e.graph.Add(conjugation, e.sysml(pRelatedElement), conjugated)
	e.graph.Add(conjugation, e.sysml(pRelatedElement), subject)
	e.graph.Add(conjugation, e.sysml(pSource), conjugated)
	e.graph.Add(conjugation, e.sysml(pTarget), subject)
	e.graph.Add(conjugated, e.sysml(pOwnedRelationship), conjugation)
	e.graph.Add(conjugated, e.sysml(pOwnedPortConjugator), conjugation)
}

// subjectParameter emits the `by` subject of a satisfy usage: an unnamed
// ReferenceUsage owned through a SubjectMembership whose FeatureValue is a
// FeatureReferenceExpression of the subject — the shape `subject = <expr>`
// has, since a satisfy's `by` is the subject it evaluates the requirement for.
func (e *encoder) subjectParameter(subject rdf.Term, owner string, target ast.Node) error {
	usage := e.ids.minted(rdf.RelationshipIRI(subject, "_subject"), subject, "_subject")
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(usage), usage, rdf.OwningMembershipSuffix)
	for _, c := range []struct{ iri, standsFor string }{
		{usage.Value, "the subject parameter of " + rdf.LocalName(subject.Value)},
		{membership.Value, "the subject membership of " + rdf.LocalName(subject.Value)},
	} {
		if prior, taken := e.claim(c.iri, c.standsFor); taken && e.idErr == nil {
			e.idErr = &UnsupportedError{
				What: c.standsFor,
				Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
			}
		}
	}
	e.typed(usage, mReferenceUsage)
	e.graph.Add(usage, e.sysml(pElementID), rdf.String(rdf.LocalName(usage.Value)))
	e.graph.Add(usage, e.sysml(pQualifiedName), rdf.String(owner+"::_subject"))
	e.emitMembershipCore(membership, usage, subject, mSubjectMembership, true)
	e.graph.Add(membership, e.sysml(pOwnedSubjectParameter), usage)
	e.graph.Add(membership, e.sysml(pOwnedMemberParameter), usage)
	e.graph.Add(membership, e.sysml(pOwnedMemberFeature), usage)
	e.graph.Add(membership, e.sysml(pOwningType), subject)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(subject, e.sysml(pOwnedMember), usage)
	e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
	return e.featureValue(usage, owner, target, false, false)
}

// subjectParameters emits a subject parameter per `by` target the head states
// (SysML.xtext SatisfyRequirementUsage `subject`); the collapsed `subject`
// property the relationships pass wrote stays beside it.
func (e *encoder) subjectParameters(subject rdf.Term, owner string, rels []*ast.Relationship) error {
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelSubject || rel.Target == nil {
			continue
		}
		if err := e.subjectParameter(subject, owner, rel.Target); err != nil {
			return err
		}
	}
	return nil
}
