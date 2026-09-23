package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// deriveNormativeGraph completes a graph written in the normative element form
// — the toolkit's interchange JSON, which mints the relationship elements this
// mapping collapses — with the collapsed properties the decoder reads, and the
// qualified names the compact form does not carry. Every addition is a triple
// this mapping's own encoder could have written, so graphs already carrying
// them gain nothing.

// owningMembershipLike reports whether a metaclass is one the decoder reads as
// an ownership edge rather than an element: the same set isMembership classifies
// before it consults names, plus the parameter memberships the toolkit writes.
func owningMembershipLike(metaclass string) bool {
	return metaclass == mFeatureValue || metaclass == mParameterMembership ||
		metaclass == "ReturnParameterMembership" ||
		metaclass != "" && ontology.IsAncestorOrSelf(metaclass, mOwningMembership)
}

// firstIRI returns the first IRI object the subject states under any of the
// properties, in argument order.
func firstIRI(graph *rdf.Graph, subject rdf.Term, properties ...string) rdf.Term {
	for _, property := range properties {
		for _, object := range graph.Objects(subject, rdf.SysML+property) {
			if object.IsIRI() {
				return object
			}
		}
	}
	return rdf.Term{}
}

// firstObject returns the first object the subject states under any of the
// properties, IRI or literal, in argument order.
func firstObject(graph *rdf.Graph, subject rdf.Term, properties ...string) rdf.Term {
	for _, property := range properties {
		for _, object := range graph.Objects(subject, rdf.SysML+property) {
			if object.Value != "" {
				return object
			}
		}
	}
	return rdf.Term{}
}

// collapsedOf is the collapsed head property a minted relationship element
// restates, for the owners this mapping collapses them on.
func collapsedOf(metaclass string, ownerHasEndForm, ownerIsSatisfy bool) (string, bool) {
	switch metaclass {
	case mFeatureTyping, mConjugatedPortTyping:
		return "type", true
	case mSubclassification, mSpecialization:
		return "specializes", true
	case mSubsetting:
		return "subsets", true
	case mRedefinition:
		return "redefines", true
	case mReferenceSubsetting:
		if ownerIsSatisfy || ownerHasEndForm {
			return "subsets", true
		}
		return "references", true
	}
	return "", false
}

// collapsedKindsOf are the collapsed properties a materialized element of the
// metaclass may restate targets of — the encoder's relationshipSpec, read the
// other way. A feature's `:>` is stored as specializes but materialized as a
// Subsetting, and a satisfy's or end-form's subsets as a ReferenceSubsetting.
var collapsedKindsOf = map[string][]ast.RelationshipKind{
	mFeatureTyping:        {ast.RelTyping},
	mConjugatedPortTyping: {ast.RelTyping},
	mSubclassification:    {ast.RelSpecializes},
	mSpecialization:       {ast.RelSpecializes},
	mSubsetting:           {ast.RelSubsets, ast.RelSpecializes},
	mRedefinition:         {ast.RelRedefines},
	mReferenceSubsetting:  {ast.RelReferences, ast.RelSubsets},
}

// relationshipLike reports whether a metaclass is a Relationship element the
// collapsed properties materialize: implied when owned directly, a declared
// member when owned through a membership.
func relationshipLike(metaclass string) bool {
	return ontology.IsAncestorOrSelf(metaclass, "Relationship")
}

// deriveNormativeGraph runs the whole normalization over the graph and
// returns the completed graph: a copy first, since the sparse form also
// states defaults this mapping never writes, and a stated default would
// print the keyword a graph carrying none reads the same way.
func deriveNormativeGraph(graph *rdf.Graph, metaclasses map[rdf.Term]string) *rdf.Graph {
	meta := func(t rdf.Term) string { return rdf.LocalName(metaclasses[t]) }
	// A graph written in the API element form spells every default, so its
	// stated defaults are dropped and its collapsed properties derived; a
	// graph minted to this mapping states only what it means.
	elementForm := false
	for _, subject := range graph.Subjects() {
		if graph.HasProperty(subject, rdf.SysML+"isImpliedIncluded") {
			elementForm = true
			break
		}
	}
	graph = dropStatedDefaults(graph, meta, elementForm)
	if !elementForm {
		return graph
	}

	// Index the owning memberships: member -> owner, and which memberships own
	// expression parts rather than element members.
	memberOwner := map[string]rdf.Term{}
	memberMembership := map[string]rdf.Term{}
	nodeMember := map[string]bool{}
	nodeOwner := map[string]rdf.Term{}
	membershipSubject := map[string]bool{}
	ownerMembers := map[string][]rdf.Term{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if !owningMembershipLike(m) {
			continue
		}
		membershipSubject[subject.Value] = true
		owner := firstIRI(graph, subject, pMembershipOwningNamespace, pOwningRelatedElement, pOwner)
		member := firstIRI(graph, subject, pMemberElement, pOwnedMemberElement, pOwnedMemberFeature,
			pOwnedVariantUsage, pOwnedResultExpression, pOwnedMemberParameter, pOwnedRelatedElement)
		if owner.Value == "" || member.Value == "" {
			continue
		}
		// Spell out the single ends a sparse graph omits; identical triples
		// dedupe, so graphs carrying them gain nothing.
		graph.Add(subject, rdf.SysMLTerm(pMemberElement), member)
		graph.Add(subject, rdf.SysMLTerm(pOwningRelatedElement), owner)
		switch {
		case m == mFeatureValue:
			graph.Add(subject, rdf.SysMLTerm(pFeatureWithValue), owner)
			graph.Add(subject, rdf.SysMLTerm(pValue), member)
		case m == mParameterMembership || m == "ReturnParameterMembership":
			graph.Add(owner, rdf.SysMLTerm(pOwnedFeatureMembership), subject)
		case m == mSubjectMembership:
			graph.Add(subject, rdf.SysMLTerm(pOwnedSubjectParameter), member)
		}
		switch {
		case m == mResultExpressionMembership:
			// The result expression is a member of the body it closes, not a
			// part of an expression tree.
			memberOwner[member.Value] = owner
			memberMembership[member.Value] = subject
			ownerMembers[owner.Value] = append(ownerMembers[owner.Value], member)
		case m == mFeatureValue:
			nodeMember[member.Value] = true
			nodeOwner[member.Value] = owner
		case expressionMetaclasses[meta(member)]:
			nodeMember[member.Value] = true
			nodeOwner[member.Value] = owner
		case (m == mParameterMembership || m == "ReturnParameterMembership") && expressionMetaclasses[meta(owner)]:
			nodeMember[member.Value] = true
			nodeOwner[member.Value] = owner
		default:
			memberOwner[member.Value] = owner
			memberMembership[member.Value] = subject
			ownerMembers[owner.Value] = append(ownerMembers[owner.Value], member)
		}
	}
	// Order each owner's members the way the owner lists them: the position a
	// member takes among the owner's memberships is its position in
	// ownedRelationship order, import memberships included; a member whose
	// owner lists none takes the order the memberships appear.
	owners := map[string]bool{}
	for _, owner := range memberOwner {
		owners[owner.Value] = true
	}
	for _, owner := range nodeOwner {
		owners[owner.Value] = true
	}
	for subject := range membershipSubject {
		if owner := firstIRI(graph, rdf.IRI(subject), pMembershipOwningNamespace, pOwningRelatedElement, pOwner); owner.IsIRI() {
			owners[owner.Value] = true
		}
	}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !strings.HasSuffix(m, "Membership") && !strings.HasSuffix(m, "Import") {
			continue
		}
		owner := firstIRI(graph, subject, pMembershipOwningNamespace, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			continue
		}
		owners[owner.Value] = true
	}
	for owner := range owners {
		term := rdf.IRI(owner)
		var ordered []rdf.Term
		seen := map[string]bool{}
		appendMember := func(m rdf.Term) {
			if m.Value != "" && !seen[m.Value] {
				seen[m.Value] = true
				ordered = append(ordered, m)
			}
		}
		for _, ms := range graph.Objects(term, rdf.SysML+pOwnedRelationship) {
			if member := firstObject(graph, ms, pMemberElement, "memberFeature", "memberNamespace",
				pOwnedMemberElement, pOwnedMemberFeature, pOwnedVariantUsage, pOwnedResultExpression,
				pOwnedMemberParameter, pOwnedSubjectParameter, pOwnedRelatedElement, pImportedMembership,
				"importedNamespace"); member.Value != "" {
				appendMember(member)
			}
		}
		for _, m := range graph.Objects(term, rdf.SysML+pOwnedMember) {
			appendMember(m)
		}
		for member, o := range memberOwner {
			if o.Value == owner {
				appendMember(rdf.IRI(member))
			}
		}
		ownerMembers[owner] = ordered
	}

	// Collapse the minted relationship elements into the head properties their
	// owner states them as, when the element is owned directly rather than
	// declared through a membership.
	for _, subject := range graph.Subjects() {
		if membershipSubject[subject.Value] || memberOwner[subject.Value] != (rdf.Term{}) {
			continue
		}
		m := meta(subject)
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			continue
		}
		property, ok := collapsedOf(m,
			graph.HasProperty(owner, rdf.OpenSysML+xEndForm),
			meta(owner) == "SatisfyRequirementUsage")
		if !ok {
			continue
		}
		target := firstIRI(graph, subject, relationshipTargetEnds...)
		if target.Value == "" {
			continue
		}
		graph.Add(owner, rdf.SysMLTerm(property), target)
		if m == mConjugatedPortTyping {
			// A conjugated typing types its feature by the original
			// definition with a `~` the head writes before the name.
			graph.Add(owner, rdf.SysMLTerm("isConjugated"), rdf.Bool(true))
		}
	}
	// Literal targets: the minted element's literal ends restate the literal
	// collapsed value the same way; the loop above only reads IRIs, so repeat
	// it for literals.
	for _, subject := range graph.Subjects() {
		if membershipSubject[subject.Value] || memberOwner[subject.Value] != (rdf.Term{}) {
			continue
		}
		m := meta(subject)
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			continue
		}
		property, ok := collapsedOf(m,
			graph.HasProperty(owner, rdf.OpenSysML+xEndForm),
			meta(owner) == "SatisfyRequirementUsage")
		if !ok {
			continue
		}
		for _, end := range relationshipTargetEnds {
			for _, object := range graph.Objects(subject, rdf.SysML+end) {
				if !object.IsIRI() {
					graph.Add(owner, rdf.SysMLTerm(property), object)
				}
			}
		}
		if m == mConjugatedPortTyping {
			graph.Add(owner, rdf.SysMLTerm("isConjugated"), rdf.Bool(true))
		}
	}

	// The Membership relating an expression to its referent states the
	// collapsed property our decoder reads: a FeatureReferenceExpression's
	// referent, a FeatureChainExpression's target feature, an invocation's
	// function.
	for _, subject := range graph.Subjects() {
		if meta(subject) != mMembership {
			continue
		}
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		member, hasMember := graph.Object(subject, rdf.SysML+pMemberElement)
		if owner.Value == "" || !hasMember {
			continue
		}
		var property string
		switch meta(owner) {
		case mFeatureReference:
			property = pReferent
		case mFeatureChain:
			property = pTargetFeature
		case mInvocation:
			property = pFunction
		}
		if property != "" && !graph.HasProperty(owner, rdf.SysML+property) {
			// A referent stated already is the statement; adding the member
			// end's target beside it would mask a disagreement.
			graph.Add(owner, rdf.SysMLTerm(property), member)
		}
	}

	// A MultiplicityRange owned by a feature states the feature's collapsed
	// bounds and the range itself.
	for _, subject := range graph.Subjects() {
		if meta(subject) != mMultiplicityRange {
			continue
		}
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			ms := firstIRI(graph, subject, pOwningRelationship, pOwningMembership)
			if ms.Value != "" {
				owner = firstIRI(graph, ms, pOwningRelatedElement, pMembershipOwningNamespace, pOwner)
			}
		}
		if owner.Value == "" {
			continue
		}
		graph.Add(owner, rdf.SysMLTerm(pMultiplicity), subject)
		var bounds []rdf.Term
		for _, property := range []string{pLowerBound, pUpperBound} {
			bounds = append(bounds, graph.Objects(subject, rdf.SysML+property)...)
		}
		stated := len(bounds)
		if len(bounds) == 0 {
			// The bounds are the range's owned bound expressions, in
			// ownedRelationship order.
			for _, ms := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
				if meta(ms) == "" || !ontology.IsAncestorOrSelf(meta(ms), mOwningMembership) {
					continue
				}
				if bound := firstIRI(graph, ms, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement); bound.Value != "" {
					bounds = append(bounds, bound)
				}
			}
		}
		switch {
		case stated > 0 && !elementForm:
			// Stated bounds are the statement; copying them to the owner
			// would hide a disagreement from verification.
		case len(bounds) == 1:
			graph.Add(owner, rdf.SysMLTerm(pUpperBound), bounds[0])
			graph.Add(subject, rdf.SysMLTerm(pUpperBound), bounds[0])
		case len(bounds) > 1:
			graph.Add(owner, rdf.SysMLTerm(pLowerBound), bounds[0])
			graph.Add(owner, rdf.SysMLTerm(pUpperBound), bounds[len(bounds)-1])
			graph.Add(subject, rdf.SysMLTerm(pLowerBound), bounds[0])
			graph.Add(subject, rdf.SysMLTerm(pUpperBound), bounds[len(bounds)-1])
		}
	}

	// A succession written between members states its ends through unnamed
	// reference features: the member an end targets where the toolkit
	// resolves it, `then` beside the next member where it names nothing.
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !ontology.IsAncestorOrSelf(m, "Succession") {
			continue
		}
		owner := memberOwner[subject.Value]
		if owner.Value == "" {
			continue
		}
		var source, target rdf.Term
		if ends := ownerMembers[subject.Value]; len(ends) >= 2 {
			if t := firstIRI(graph, ends[0], "featureTarget"); t.IsIRI() && t != ends[0] {
				source = t
			}
			last := ends[len(ends)-1]
			if t := firstIRI(graph, last, "featureTarget"); t.IsIRI() && t != last {
				target = t
			}
		}
		if source.Value != "" {
			graph.Add(subject, rdf.SysMLTerm(pSourceFeature), source)
		}
		if target.Value != "" {
			graph.Add(subject, rdf.SysMLTerm(pTargetFeature), target)
		}
		if target.Value != "" || graph.HasProperty(subject, rdf.SysML+pTargetFeature) {
			continue
		}
		members := ownerMembers[owner.Value]
		for i, member := range members {
			if member != subject {
				continue
			}
			for _, next := range members[i+1:] {
				nm := meta(next)
				if nm == "" || relationshipLike(nm) || ontology.IsAncestorOrSelf(nm, "Succession") {
					continue
				}
				graph.Add(subject, rdf.OpenSysMLTerm(xEndForm), rdf.String(formThen))
				graph.Add(subject, rdf.OpenSysMLTerm(xTargetMember), next)
				break
			}
			break
		}
	}

	// An element with members is written with a body; the compact form states
	// no hasBody flag.
	bodied := map[string]bool{}
	for _, owner := range memberOwner {
		bodied[owner.Value] = true
	}
	for _, subject := range graph.Subjects() {
		if bodied[subject.Value] && !graph.HasProperty(subject, rdf.OpenSysML+xHasBody) {
			m := meta(subject)
			// A satisfy's members are its head's subject parameter and the
			// relationships it implies, not a body.
			if m == "SatisfyRequirementUsage" {
				continue
			}
			// A relationship's members that are metadata usages are its `#`
			// prefixes, not a body, so owning only those braces nothing.
			if isRelationship(m) {
				allMetadata := true
				for _, member := range ownerMembers[subject.Value] {
					if meta(member) != "MetadataUsage" {
						allMetadata = false
						break
					}
				}
				if allMetadata {
					continue
				}
			}
			if m != "" && !expressionMetaclasses[m] && !membershipSubject[subject.Value] {
				graph.Add(subject, rdf.OpenSysMLTerm(xHasBody), rdf.Bool(true))
			}
		}
	}

	// A satisfy's subject chain: SubjectMembership -> ReferenceUsage ->
	// FeatureValue -> expression, which the collapsed `subject` states by its
	// referent.
	for _, subject := range graph.Subjects() {
		if meta(subject) != "SatisfyRequirementUsage" {
			continue
		}
		if !graph.HasProperty(subject, rdf.OpenSysML+xEndForm) {
			graph.Add(subject, rdf.OpenSysMLTerm(xEndForm), rdf.String("satisfy"))
		}
		if graph.HasProperty(subject, rdf.SysML+"subject") {
			continue
		}
		for _, ms := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			if meta(ms) != mSubjectMembership {
				continue
			}
			parameter := firstIRI(graph, ms, pOwnedSubjectParameter, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement)
			if parameter.Value == "" {
				continue
			}
			for _, fv := range graph.Objects(parameter, rdf.SysML+pOwnedRelationship) {
				if meta(fv) != mFeatureValue {
					continue
				}
				value := firstIRI(graph, fv, pValue, pMemberElement, pOwnedRelatedElement)
				if value.Value == "" {
					continue
				}
				graph.Add(parameter, rdf.SysMLTerm(pValue), value)
				target := firstIRI(graph, value, pReferent, pTargetFeature)
				if target.Value != "" {
					graph.Add(subject, rdf.SysMLTerm("subject"), target)
				}
			}
		}
	}

	// Qualified names: the compact form carries none, and every name the
	// decoder writes is read from one. An element's is its owner's qualified
	// name plus its declared name, or its position among the owner's members.
	qname := map[string]string{}
	visiting := map[string]bool{}
	var nameOf func(subject rdf.Term) string
	nameOf = func(subject rdf.Term) string {
		if q, ok := qname[subject.Value]; ok {
			return q
		}
		if visiting[subject.Value] {
			// An element owned through a relationship can loop back to itself.
			return ""
		}
		visiting[subject.Value] = true
		defer delete(visiting, subject.Value)
		if q, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok {
			qname[subject.Value] = q
			return q
		}
		owner, owned := memberOwner[subject.Value]
		base := ""
		if owned {
			base = nameOf(owner)
		}
		name, _ := graph.Lexical(subject, rdf.SysML+pDeclaredName)
		if name == "" {
			name, _ = graph.Lexical(subject, rdf.SysML+pDeclaredShortName)
		}
		if name == "" {
			if !owned {
				// An unnamed element no membership owns — the root namespace
				// is the one — contributes no name.
				qname[subject.Value] = ""
				return ""
			}
			index := 0
			for i, member := range ownerMembers[owner.Value] {
				if member == subject {
					index = i
					break
				}
			}
			q := qualify(base, "", index)
			qname[subject.Value] = q
			return q
		}
		q := qualify(base, identity.EscapeName(name), 0)
		qname[subject.Value] = q
		return q
	}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if membershipSubject[subject.Value] || nodeMember[subject.Value] && expressionMetaclasses[m] {
			continue
		}
		if graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
			continue
		}
		if m == "" || expressionMetaclasses[m] {
			continue
		}
		owner, owned := memberOwner[subject.Value]
		if !owned && relationshipLike(m) {
			// An implied relationship is no member and takes no name.
			continue
		}
		if !owned {
			// A root: named by its declared name alone, or left unnamed.
			name, _ := graph.Lexical(subject, rdf.SysML+pDeclaredName)
			if name == "" {
				name, _ = graph.Lexical(subject, rdf.SysML+pDeclaredShortName)
			}
			if name == "" {
				continue
			}
			graph.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(identity.EscapeName(name)))
			qname[subject.Value] = identity.EscapeName(name)
			continue
		}
		_ = owner
		graph.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(nameOf(subject)))
	}

	// A perform's action is the type its head is typed by, written back as
	// the expression the `perform` statement names.
	for _, subject := range graph.Subjects() {
		if meta(subject) != mPerform || graph.HasProperty(subject, rdf.OpenSysML+xExpression) {
			continue
		}
		for _, target := range graph.Objects(subject, rdf.SysML+"type") {
			if target.IsIRI() {
				if name, ok := qname[target.Value]; ok && name != "" {
					graph.Add(subject, rdf.OpenSysMLTerm(xExpression), rdf.String(name))
				}
				continue
			}
			graph.Add(subject, rdf.OpenSysMLTerm(xExpression), target)
		}
	}

	// A ReferenceUsage is the kindless member metaclass — `ref` states the
	// kind when the keyword is written, and the compact form records no
	// keyword for it, so a toolkit ReferenceUsage prints none. A parameter
	// whose membership spells its keyword — a satisfy's `subject` — keeps it.
	for _, subject := range graph.Subjects() {
		if meta(subject) != "ReferenceUsage" {
			continue
		}
		membership := firstIRI(graph, subject, pOwningMembership, pOwningRelationship)
		if mm := meta(membership); mm != "" && mm != "FeatureMembership" && mm != "OwningMembership" {
			continue
		}
		if !graph.HasProperty(subject, rdf.OpenSysML+xDeclaredKeyword) {
			graph.Add(subject, rdf.OpenSysMLTerm(xImplicitKind), rdf.Bool(true))
		}
	}
	return graph
}

// dropStatedDefaults copies the graph without the triples the sparse form
// writes where this mapping writes nothing: a stated default reads identically
// to an absent one, and a printed keyword would declare it twice.
func dropStatedDefaults(graph *rdf.Graph, meta func(rdf.Term) string, elementForm bool) *rdf.Graph {
	if !elementForm {
		// Only the element form states defaults and implied restatements; a
		// graph minted to this mapping means every triple it writes.
		return graph
	}
	// An element owned by a membership — an annotation of the membership, or
	// an implied-include membership nested under one — is no member of the
	// model; the compact form writes it as the memberElement literal alone.
	membershipOwned := map[string]bool{}
	if elementForm {
		for _, subject := range graph.Subjects() {
			owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner, pMembershipOwningNamespace, pOwningNamespace)
			if owner.Value == "" {
				if ms := firstIRI(graph, subject, pOwningRelationship, pOwningMembership); ms.Value != "" {
					owner = firstIRI(graph, ms, pOwningRelatedElement, pOwner, pMembershipOwningNamespace)
				}
			}
			if m := meta(owner); m != "" && (strings.HasSuffix(m, "Membership") || strings.HasSuffix(m, "Import")) {
				membershipOwned[subject.Value] = true
			}
		}
	}
	// The collections a derived property lists count expression parts —
	// a MultiplicityRange or a bound is part of its owner's structure, not a
	// member — as members of the element collections the mapping reads.
	nodeOwned := map[string]bool{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !(owningMembershipLike(m) || strings.HasSuffix(m, "Membership") || strings.HasSuffix(m, "Import")) {
			continue
		}
		member := firstIRI(graph, subject, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement)
		if member.Value == "" {
			continue
		}
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner, pMembershipOwningNamespace)
		if expressionMetaclasses[meta(member)] || m == mFeatureValue ||
			(m == mParameterMembership || m == "ReturnParameterMembership") && expressionMetaclasses[meta(owner)] {
			nodeOwned[member.Value] = true
		}
	}
	memberCollections := map[string]bool{
		"member": true, "ownedMember": true, "ownedElement": true,
		"ownedFeature": true, "input": true, "output": true,
	}
	// A membership's generic relationship ends restate its member ends; when
	// the member is an expression part they would reference a node.
	membershipEnds := map[string]bool{
		"source": true, "target": true, "relatedElement": true,
	}
	// The collapsed head properties a minted relationship restates.
	collapsedProps := map[string]bool{
		"type": true, "specializes": true, "subsets": true,
		"redefines": true, "references": true,
	}
	// The structural ends an element owns and is owned through: a chain
	// segment stays an element to these, a written name to everything else.
	structuralProps := map[string]bool{
		pOwningRelatedElement: true, pOwner: true, pOwningNamespace: true,
		pOwningRelationship: true, pOwningMembership: true, pOwnedRelationship: true,
		pOwnedRelatedElement: true, "relatedElement": true, pMemberElement: true,
		pOwnedMemberElement: true, pMembershipOwningNamespace: true,
		"owningFeatureMembership": true, "owningFeature": true,
	}
	// The properties whose literal objects are a reference written as a name;
	// the written spelling is unescaped so writing it back escapes it once.
	referenceNameProps := map[string]bool{
		pMemberElement: true, "referent": true, pTargetFeature: true,
		pFunction: true, pClient: true, pSupplier: true, pSource: true,
		pTarget: true, "relatedElement": true, "importedNamespace": true,
		"type": true, "specializes": true, "subsets": true,
		"redefines": true, "references": true,
	}
	membershipSubjects := map[string]bool{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m != "" && (owningMembershipLike(m) || strings.HasSuffix(m, "Membership") || strings.HasSuffix(m, "Import")) {
			membershipSubjects[subject.Value] = true
		}
	}
	// The full form restates each declared relationship as a derived property
	// on its owner; only an edge a minted relationship element carries is a
	// statement. A derived edge unbacked by one is dropped — the toolkit marks
	// the elements it derives these for with isImpliedIncluded.
	backed := map[string]map[string]bool{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !relationshipLike(m) {
			continue
		}
		if graph.BoolValue(subject, rdf.SysML+"isImplied") {
			// An implied element restates a derivation its owner did not
			// declare; it backs no collapsed statement either.
			continue
		}
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			continue
		}
		for _, end := range relationshipTargetEnds {
			for _, object := range graph.Objects(subject, rdf.SysML+end) {
				if backed[owner.Value] == nil {
					backed[owner.Value] = map[string]bool{}
				}
				backed[owner.Value][object.Value] = true
			}
		}
	}
	// A memberElement the compact form writes as a `{"@ref"}` name the full
	// form resolves to a library IRI and annotates with the name it could not
	// resolve; the name is what the member states.
	unresolvedRef := map[string]string{}
	for _, subject := range graph.Subjects() {
		if meta(subject) != "TextualRepresentation" {
			continue
		}
		language, _ := graph.Lexical(subject, rdf.SysML+"language")
		body, hasBody := graph.Lexical(subject, rdf.SysML+"body")
		if language != "x-sysmlv2-unresolved-reference" || !hasBody {
			continue
		}
		owner := firstIRI(graph, subject, "representedElement", "annotatedElement", pOwner)
		if owner.Value != "" {
			unresolvedRef[owner.Value] = unescapeName(body)
		}
	}
	// The same reference resolved to a library uuid only in the full form is
	// still the name it could not resolve: uuid5(OID, "unresolved:"+name).
	unresolvedID := map[string]string{}
	unresolvedTR := map[string]bool{}
	for _, subject := range graph.Subjects() {
		if meta(subject) != "TextualRepresentation" {
			continue
		}
		language, _ := graph.Lexical(subject, rdf.SysML+"language")
		body, hasBody := graph.Lexical(subject, rdf.SysML+"body")
		if language != "x-sysmlv2-unresolved-reference" || !hasBody {
			continue
		}
		unresolvedID[identity.UnresolvedElementID(body)] = unescapeName(body)
		// The annotation exists only to carry the written name; the compact
		// form mints no such element.
		unresolvedTR[subject.Value] = true
	}
	// An unnamed Feature a relationship relates is a feature chain's segment:
	// `a.b.c` is the chain of features its FeatureChaining elements name, so a
	// reference to it is written as that chain's text, not as the element.
	chainSegment := map[string]string{}
	for _, subject := range graph.Subjects() {
		if meta(subject) == "" || !ontology.IsAncestorOrSelf(meta(subject), "Feature") {
			continue
		}
		if _, named := graph.Lexical(subject, rdf.SysML+pDeclaredName); named {
			continue
		}
		var segments []string
		for _, chain := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			if meta(chain) != "FeatureChaining" {
				continue
			}
			feature := firstIRI(graph, chain, "chainingFeature")
			if feature.IsIRI() {
				if name, ok := graph.Lexical(feature, rdf.SysML+pDeclaredName); ok {
					segments = append(segments, name)
				} else if name, ok := graph.Lexical(feature, rdf.SysML+pQualifiedName); ok {
					segments = append(segments, name[strings.LastIndex(name, "::")+2:])
				}
			} else if name, ok := graph.Lexical(chain, rdf.SysML+"chainingFeature"); ok {
				segments = append(segments, unescapeName(name))
			}
		}
		if len(segments) > 0 {
			chainSegment[subject.Value] = strings.Join(segments, ".")
		}
	}
	// The element owning an unresolved membership is annotated the same way
	// through its derived referent properties.
	unresolvedOwner := map[string]bool{}
	for member := range unresolvedRef {
		if owner := firstIRI(graph, rdf.IRI(member), pOwningRelatedElement, pOwner); owner.Value != "" {
			unresolvedOwner[owner.Value] = true
		}
	}
	// memberOwnerOf is the element a membership-owned member belongs to.
	memberOwnerOf := map[string]string{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !(owningMembershipLike(m) || strings.HasSuffix(m, "Membership") || strings.HasSuffix(m, "Import")) {
			continue
		}
		member := firstIRI(graph, subject, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement)
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner, pMembershipOwningNamespace)
		if member.Value != "" && owner.Value != "" {
			memberOwnerOf[member.Value] = owner.Value
		}
	}
	// An implied element restates a derivation, not a statement: the extra
	// specialization a minted element does not declare is dropped whole.
	implied := map[string]bool{}
	for _, triple := range graph.Triples() {
		if rdf.LocalName(triple.Predicate.Value) == "isImplied" &&
			!triple.Object.IsIRI() && triple.Object.Value == "true" {
			implied[triple.Subject.Value] = true
		}
	}
	dropped := func(v string) bool {
		return membershipOwned[v] || implied[v] || unresolvedTR[v]
	}
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if dropped(triple.Subject.Value) {
			continue
		}
		if elementForm && membershipSubjects[triple.Subject.Value] {
			// The membership exists only to own an artifact the copy drops;
			// a member the name restore already handled stays literal.
			if member := firstIRI(graph, triple.Subject, pMemberElement, pOwnedMemberElement); member.Value != "" &&
				unresolvedTR[member.Value] {
				continue
			}
		}
		object := triple.Object
		if object.IsIRI() {
			local := rdf.LocalName(triple.Predicate.Value)
			if _, segment := chainSegment[object.Value]; segment && !structuralProps[local] {
				object = rdf.String(chainSegment[object.Value])
				triple.Object = object
			}
			if tail := object.Value[strings.LastIndex(object.Value, ":")+1:]; unresolvedID[tail] != "" && meta(object) == "" {
				out.Add(triple.Subject, triple.Predicate, rdf.String(unresolvedID[tail]))
				continue
			}
			if _, unresolved := unresolvedRef[triple.Subject.Value]; unresolved && local == pMemberElement {
				continue
			}
			if elementForm && (nodeOwned[object.Value] || dropped(object.Value)) &&
				(memberCollections[local] ||
					membershipSubjects[triple.Subject.Value] && membershipEnds[local]) {
				continue
			}
			if elementForm {
				if membershipSubjects[triple.Subject.Value] && membershipEnds[local] {
					// A membership's generic ends restate its member ends; a
					// member an expression owns by node is not among them.
					continue
				}
				if membershipEnds[local] && relationshipLike(meta(triple.Subject)) &&
					meta(triple.Subject) != "ReferenceSubsetting" {
					// A relationship's generic ends restate its member ends, the
					// client and supplier it owns by name aside.
					continue
				}
				if local == pAnnotatedElement && memberOwnerOf[triple.Subject.Value] == object.Value {
					// An annotating member's annotated element is its owner,
					// restated: the compact form states ownership alone.
					continue
				}
			}
			if unresolvedOwner[triple.Subject.Value] && meta(object) == "" {
				switch local {
				case "referent", pTargetFeature, pFunction:
					continue
				}
			}
			if elementForm && nodeOwned[triple.Subject.Value] && (local == pOwningNamespace || local == pOwner) {
				// A node's namespace is its owning membership's, which the
				// full form also states directly.
				continue
			}
			switch rdf.LocalName(triple.Predicate.Value) {
			case "type", "specializes", "subsets", "redefines", "references":
				if m := meta(triple.Subject); !relationshipLike(m) &&
					!backed[triple.Subject.Value][object.Value] &&
					graph.HasProperty(triple.Subject, rdf.SysML+"isImpliedIncluded") {
					continue
				}
			}
		}
		if !object.IsIRI() {
			local := rdf.LocalName(triple.Predicate.Value)
			if strings.HasPrefix(triple.Predicate.Value, rdf.AnnotationJSON) &&
				(memberCollections[local] || membershipEnds[local] ||
					collapsedProps[local] && !relationshipLike(meta(triple.Subject)) &&
						graph.HasProperty(triple.Subject, rdf.SysML+"isImpliedIncluded")) {
				// A collection restated under json: agrees with the typed
				// triples only while none is dropped; the derived list is
				// dropped whole instead.
				continue
			}
			if referenceNameProps[local] && !strings.HasPrefix(triple.Predicate.Value, rdf.AnnotationJSON) {
				object = rdf.String(unescapeName(object.Value))
			}
			drop := false
			switch {
			case local == pQualifiedName && graph.HasProperty(triple.Subject, rdf.SysML+"isImpliedIncluded") &&
				!graph.HasProperty(triple.Subject, rdf.SysML+pDeclaredName) &&
				!graph.HasProperty(triple.Subject, rdf.SysML+pDeclaredShortName):
				// The full form derives a qualified name for the unnamed too;
				// the compact form leaves the name to be derived.
				drop = true
			case local == pElementID && strings.HasSuffix(triple.Subject.Value, object.Value):
				// An elementId identical to the IRI's id is derivable.
				drop = true
			case local == "isReference" && object.Value == "true" &&
				graph.HasProperty(triple.Subject, rdf.SysML+"isImpliedIncluded"):
				// A derived reference usage writes no `ref` of its own.
				drop = true
			case (local == "mayTimeVary" || local == "isVariable") && object.Value == "true" &&
				graph.HasProperty(triple.Subject, rdf.SysML+"isImpliedIncluded"):
				// A feature is variable and time-varying without a keyword.
				drop = true
			case local == pVisibility && object.Value == "public":
				// Public is the visibility a member writes nothing for.
				drop = true
			case local == "isComposite" && object.Value == "true":
				switch meta(triple.Subject) {
				case "PartUsage", "PortUsage", "ItemUsage", "ConstraintUsage":
					// Compositional usages are composite without a keyword.
					drop = true
				}
			}
			if drop {
				continue
			}
			triple.Object = object
		}
		out.AddTriple(triple)
	}
	for member, name := range unresolvedRef {
		out.Add(rdf.IRI(member), rdf.SysMLTerm(pMemberElement), rdf.String(name))
	}
	return out
}

// unescapeName returns a written qualified name in its canonical spelling:
// segments split on `::` outside quotes, each requoted and escaped once.
func unescapeName(name string) string {
	segments, ok := source.QualifiedNameSegments(name)
	if !ok {
		return source.UnescapedName(strings.Trim(name, "'"))
	}
	for i, segment := range segments {
		segments[i] = source.UnescapedName(segment)
	}
	return source.QualifiedNameOf(segments)
}
