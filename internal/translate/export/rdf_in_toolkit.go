package export

import (
	"fmt"
	"sort"
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
		metaclass == mReturnParameterMembership ||
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

// originalPortDefinition is the PortDefinition a ConjugatedPortTyping's `~P`
// names: its stated portDefinition when that is a definition (the toolkit
// points it at the typing itself), else the conjugate's original.
func originalPortDefinition(graph *rdf.Graph, meta func(rdf.Term) string, typing, conjugated rdf.Term) rdf.Term {
	if stated := firstIRI(graph, typing, pPortDefinition); stated.Value != "" && meta(stated) != mConjugatedPortTyping {
		return stated
	}
	if conjugated.Value == "" || meta(conjugated) != mConjugatedPortDefinition {
		return conjugated
	}
	for _, subject := range graph.Subjects() {
		if meta(subject) == mPortConjugation && firstIRI(graph, subject, pConjugatedType) == conjugated {
			if original := firstIRI(graph, subject, pOriginalPortDefinition, pOriginalType); original.Value != "" {
				return original
			}
		}
		if firstIRI(graph, subject, pConjugatedPortDefinition) == conjugated {
			return subject
		}
	}
	if ms := firstIRI(graph, conjugated, pOwningRelationship, pOwningMembership); ms.Value != "" {
		return firstIRI(graph, ms, pOwningRelatedElement, pMembershipOwningNamespace, pOwner)
	}
	return conjugated
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

// chainOwnerIndex indexes each FeatureChaining element by the chain feature
// its owningRelatedElement (or, unstated, its owner) names.
func chainOwnerIndex(graph *rdf.Graph, meta func(rdf.Term) string) map[string][]rdf.Term {
	index := map[string][]rdf.Term{}
	for _, subject := range graph.Subjects() {
		if meta(subject) != mFeatureChaining {
			continue
		}
		if owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner); owner.Value != "" {
			index[owner.Value] = append(index[owner.Value], subject)
		}
	}
	return index
}

// chainLinksOf is the ordered link list the FeatureChaining elements a chain
// feature owns state: ownedRelationship order first, then indexed leftovers.
func chainLinksOf(graph *rdf.Graph, meta func(rdf.Term) string, index map[string][]rdf.Term, chain rdf.Term) ([]rdf.Term, error) {
	var links []rdf.Term
	seen := map[string]bool{}
	appendLink := func(chaining rdf.Term) error {
		if seen[chaining.Value] {
			return nil
		}
		objects := graph.Objects(chaining, rdf.SysML+pChainingFeature)
		if len(objects) != 1 {
			note := "it states no chainingFeature"
			if len(objects) > 1 {
				note = "it states more than one chainingFeature"
			}
			return &UnsupportedError{
				What: fmt.Sprintf("the FeatureChaining <%s>", chaining.Value),
				Note: note,
			}
		}
		seen[chaining.Value] = true
		links = append(links, objects[0])
		return nil
	}
	for _, chaining := range graph.Objects(chain, rdf.SysML+pOwnedRelationship) {
		if meta(chaining) == mFeatureChaining {
			if err := appendLink(chaining); err != nil {
				return nil, err
			}
		}
	}
	for _, chaining := range index[chain.Value] {
		if err := appendLink(chaining); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// chainFeatureIn reports whether subject is an unnamed Feature owning
// FeatureChainings: the feature chain `a.b` an expression reaches or invokes.
func chainFeatureIn(graph *rdf.Graph, meta func(rdf.Term) string, index map[string][]rdf.Term, subject rdf.Term) (bool, error) {
	_, ok, err := chainTextIn(graph, meta, index, nil, subject)
	return ok, err
}

// chainTextIn writes the chain `a.b.c` an unnamed Feature's FeatureChainings
// spell, as notation: each segment its declared name or, through unresolved,
// the name the writer could not resolve to an element of the document.
func chainTextIn(graph *rdf.Graph, meta func(rdf.Term) string, index map[string][]rdf.Term, unresolved map[string]string, subject rdf.Term) (string, bool, error) {
	if meta(subject) == "" || !ontology.IsAncestorOrSelf(meta(subject), mFeature) {
		return "", false, nil
	}
	if _, named := graph.Lexical(subject, rdf.SysML+pDeclaredName); named {
		return "", false, nil
	}
	features, err := chainLinksOf(graph, meta, index, subject)
	if err != nil {
		return "", false, err
	}
	var segments []string
	for _, feature := range features {
		if feature.IsIRI() {
			if name, ok := graph.Lexical(feature, rdf.SysML+pDeclaredName); ok {
				segments = append(segments, nameText(name))
			} else if name, ok := graph.Lexical(feature, rdf.SysML+pQualifiedName); ok {
				segments = append(segments, lastSegmentText(name))
			} else if name := unresolved[feature.Value[strings.LastIndex(feature.Value, ":")+1:]]; name != "" {
				if _, qualified := source.QualifiedNameSegments(name); qualified {
					segments = append(segments, lastSegmentText(name))
				} else {
					// The writer left a chain unresolved whole; its text is the segment.
					segments = append(segments, name)
				}
			}
		} else if feature.IsLiteral() {
			segments = append(segments, qualifiedNameText(canonicalName(feature.Value)))
		}
	}
	return strings.Join(segments, "."), len(segments) > 0, nil
}

// lastSegmentText writes the last segment of a qualified name as notation.
func lastSegmentText(qname string) string {
	segments := identitySegments(qname)
	return nameText(identityName(segments[len(segments)-1]))
}

// deriveNormativeGraph runs the whole normalization over the graph and
// returns the completed graph: a copy first, since the sparse form also
// states defaults this mapping never writes, and a stated default would
// print the keyword a graph carrying none reads the same way.
func deriveNormativeGraph(graph *rdf.Graph, metaclasses map[rdf.Term]string) (*rdf.Graph, error) {
	meta := func(t rdf.Term) string { return rdf.LocalName(metaclasses[t]) }
	chainIndex := chainOwnerIndex(graph, meta)
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
	graph, err := dropStatedDefaults(graph, meta, elementForm, chainIndex)
	if err != nil {
		return nil, err
	}
	if !elementForm {
		return graph, nil
	}
	n := &normalizer{
		graph:             graph,
		meta:              meta,
		chainIndex:        chainIndex,
		elementForm:       elementForm,
		memberOwner:       map[string]rdf.Term{},
		memberMembership:  map[string]rdf.Term{},
		nodeMember:        map[string]bool{},
		nodeOwner:         map[string]rdf.Term{},
		membershipSubject: map[string]bool{},
		ownerMembers:      map[string][]rdf.Term{},
		qname:             map[string]string{},
		visiting:          map[string]bool{},
	}
	n.indexMemberships()
	n.orderMembers()
	n.collapseRelationships()
	n.collapseLiteralRelationships()
	if err := n.deriveChainingFeatures(); err != nil {
		return nil, err
	}
	if err := n.deriveReferents(); err != nil {
		return nil, err
	}
	n.deriveMultiplicities()
	n.deriveSuccessionEnds()
	n.stateMemberIndices()
	n.markBodies()
	n.deriveSatisfySubjects()
	n.deriveQualifiedNames()
	deriveTransitionHeads(graph, meta)
	n.derivePerformExpressions()
	n.markImplicitKinds()
	return graph, nil
}

// normalizer carries the element-form graph through the derivation passes
// and the membership indexes they share: member -> owner, the membership that
// owns each member, which members are expression-tree nodes rather than body
// members, and each owner's members in positional order.
type normalizer struct {
	graph             *rdf.Graph
	meta              func(rdf.Term) string
	chainIndex        map[string][]rdf.Term
	elementForm       bool
	memberOwner       map[string]rdf.Term
	memberMembership  map[string]rdf.Term
	nodeMember        map[string]bool
	nodeOwner         map[string]rdf.Term
	membershipSubject map[string]bool
	ownerMembers      map[string][]rdf.Term
	qname             map[string]string
	visiting          map[string]bool
	roots             []rdf.Term
}

func (n *normalizer) chainFeature(t rdf.Term) (bool, error) {
	return chainFeatureIn(n.graph, n.meta, n.chainIndex, t)
}

// indexMemberships indexes the owning memberships: member -> owner, and which
// memberships own expression parts rather than element members.
func (n *normalizer) indexMemberships() {
	graph, meta, memberOwner, memberMembership, nodeMember, nodeOwner, membershipSubject, ownerMembers := n.graph, n.meta, n.memberOwner, n.memberMembership, n.nodeMember, n.nodeOwner, n.membershipSubject, n.ownerMembers
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
		case m == mParameterMembership || m == mReturnParameterMembership:
			graph.Add(owner, rdf.SysMLTerm(pOwnedFeatureMembership), subject)
		case m == mSubjectMembership:
			graph.Add(subject, rdf.SysMLTerm(pOwnedSubjectParameter), member)
		}
		switch {
		case m == mSubaction:
			// `entry`/`do`/`exit` is the membership itself, a member of the
			// state that performs its one action.
			delete(membershipSubject, subject.Value)
			if kind, ok := graph.Lexical(subject, rdf.SysML+pKind); ok {
				graph.Add(subject, rdf.OpenSysMLTerm(xSubactionKind), rdf.String(kind))
			}
			memberOwner[subject.Value] = owner
			ownerMembers[owner.Value] = append(ownerMembers[owner.Value], subject)
			memberOwner[member.Value] = subject
			memberMembership[member.Value] = subject
			ownerMembers[subject.Value] = append(ownerMembers[subject.Value], member)
		case m != mResultExpressionMembership && (m == mFeatureValue ||
			expressionMetaclasses[meta(member)] ||
			(m == mParameterMembership || m == mReturnParameterMembership) && expressionMetaclasses[meta(owner)]):
			// A node of an expression tree; a result expression is instead a
			// member of the body it closes.
			nodeMember[member.Value] = true
			nodeOwner[member.Value] = owner
		default:
			memberOwner[member.Value] = owner
			memberMembership[member.Value] = subject
			ownerMembers[owner.Value] = append(ownerMembers[owner.Value], member)
		}
	}
}

// orderMembers orders each owner's members the way the owner lists them: the
// position a member takes among the owner's memberships is its position in
// ownedRelationship order, import memberships included; a member whose owner
// lists none takes the order the memberships appear.
func (n *normalizer) orderMembers() {
	for owner := range n.membershipOwners() {
		n.ownerMembers[owner] = n.orderedMembersOf(owner)
	}
}

// membershipOwners is every element that owns a member, a membership, or an import.
func (n *normalizer) membershipOwners() map[string]bool {
	graph, meta, memberOwner, nodeOwner, membershipSubject := n.graph, n.meta, n.memberOwner, n.nodeOwner, n.membershipSubject
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
	return owners
}

// orderedMembersOf is the owner's members in the order its memberships list them.
func (n *normalizer) orderedMembersOf(owner string) []rdf.Term {
	graph, meta, memberOwner, nodeMember := n.graph, n.meta, n.memberOwner, n.nodeMember
	term := rdf.IRI(owner)
	var ordered []rdf.Term
	seen := map[string]bool{}
	appendMember := func(m rdf.Term) {
		// A feature's `[n]` range and the expressions valuing it are part
		// of its head, not body members that take a position.
		if nodeMember[m.Value] || meta(m) == mMultiplicityRange && !graph.HasProperty(m, rdf.SysML+pDeclaredName) {
			return
		}
		if m.Value != "" && !seen[m.Value] {
			seen[m.Value] = true
			ordered = append(ordered, m)
		}
	}
	appendMembership := func(ms rdf.Term) {
		// Only memberships and imports take a position; a specialization
		// or typing beside them is part of the owner's head.
		if m := meta(ms); m == "" || !strings.HasSuffix(m, "Membership") && !strings.HasSuffix(m, "Import") {
			return
		} else if m == mSubaction {
			appendMember(ms)
			return
		}
		if member := firstObject(graph, ms, pMemberElement, "memberFeature", "memberNamespace",
			pOwnedMemberElement, pOwnedMemberFeature, pOwnedVariantUsage, pOwnedResultExpression,
			pOwnedMemberParameter, pOwnedSubjectParameter, pOwnedRelatedElement, pImportedMembership,
			"importedNamespace"); member.Value != "" {
			appendMember(member)
		}
	}
	for _, ms := range graph.Objects(term, rdf.SysML+pOwnedRelationship) {
		appendMembership(ms)
	}
	for _, ms := range graph.Objects(term, rdf.SysML+pOwnedMembership) {
		appendMembership(ms)
	}
	for _, m := range graph.Objects(term, rdf.SysML+pOwnedMember) {
		appendMember(m)
	}
	for member, o := range memberOwner {
		if o.Value == owner {
			appendMember(rdf.IRI(member))
		}
	}
	return ordered
}

// collapseRelationships collapses the minted relationship elements into the
// head properties their owner states them as, when the element is owned
// directly rather than declared through a membership.
func (n *normalizer) collapseRelationships() {
	graph, meta, memberOwner, membershipSubject := n.graph, n.meta, n.memberOwner, n.membershipSubject
	for _, subject := range graph.Subjects() {
		if membershipSubject[subject.Value] || memberOwner[subject.Value] != (rdf.Term{}) {
			continue
		}
		m := meta(subject)
		owner := firstIRI(graph, subject, pOwningRelatedElement, pOwner)
		if owner.Value == "" {
			continue
		}
		// A verification's reference is the satisfy form's: the RequirementUsage a
		// RequirementVerificationMembership owns subsets the requirement it names.
		verifies := meta(firstIRI(graph, owner, pOwningRelationship, pOwningMembership)) == mRequirementVerificationMembership
		property, ok := collapsedOf(m,
			graph.HasProperty(owner, rdf.OpenSysML+xEndForm),
			meta(owner) == "SatisfyRequirementUsage" || verifies)
		if !ok {
			continue
		}
		target := firstIRI(graph, subject, relationshipTargetEnds...)
		if m == mConjugatedPortTyping {
			// The head writes `~P`: the original definition, the conjugate of
			// which the typing names as its type.
			target = originalPortDefinition(graph, meta, subject, target)
			graph.Add(owner, rdf.OpenSysMLTerm(xConjugatedTyping), rdf.Bool(true))
		}
		if target.Value == "" {
			continue
		}
		graph.Add(owner, rdf.SysMLTerm(property), target)
	}
}

// collapseLiteralRelationships restates the minted element's literal ends as
// the literal collapsed value the same way; collapseRelationships only reads
// IRIs, so the walk is repeated for literals.
func (n *normalizer) collapseLiteralRelationships() {
	graph, meta, memberOwner, membershipSubject := n.graph, n.meta, n.memberOwner, n.membershipSubject
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
					if m == mConjugatedPortTyping {
						object = rdf.String(strings.TrimPrefix(object.Value, "~"))
					}
					graph.Add(owner, rdf.SysMLTerm(property), object)
				}
			}
		}
		if m == mConjugatedPortTyping {
			graph.Add(owner, rdf.OpenSysMLTerm(xConjugatedTyping), rdf.Bool(true))
		}
	}
}

// deriveChainingFeatures derives a chain feature's collapsed chainingFeature
// list from the FeatureChainings it owns, in ownedRelationship order.
func (n *normalizer) deriveChainingFeatures() error {
	graph, meta, chainIndex := n.graph, n.meta, n.chainIndex
	for _, subject := range graph.Subjects() {
		isChain, err := n.chainFeature(subject)
		if err != nil {
			return err
		}
		if !isChain || graph.HasProperty(subject, rdf.SysML+pChainingFeature) {
			continue
		}
		links, err := chainLinksOf(graph, meta, chainIndex, subject)
		if err != nil {
			return err
		}
		for _, link := range links {
			graph.Add(subject, rdf.SysMLTerm(pChainingFeature), link)
		}
	}
	return nil
}

// deriveReferents states the collapsed property our decoder reads from the
// Membership relating an expression to its referent; a chain the expression
// reaches is a Feature of FeatureChainings, read as the chain's text.
func (n *normalizer) deriveReferents() error {
	graph, meta, chainIndex := n.graph, n.meta, n.chainIndex
	unresolvedID, _ := unresolvedNames(graph, meta)
	// The Membership relating an expression to its referent states the
	// collapsed property our decoder reads; a chain the expression reaches is
	// a Feature of FeatureChainings, read as the chain's text.
	for _, subject := range graph.Subjects() {
		if meta(subject) != mMembership && meta(subject) != mOwningMembership {
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
		case mInvocation, mConstructor:
			property = pFunction
		}
		if meta(subject) == mOwningMembership {
			text, chain, err := chainTextIn(graph, meta, chainIndex, unresolvedID, member)
			if err != nil {
				return err
			}
			if !chain || property == pFunction {
				continue
			}
			member = rdf.TypedLiteral(text, rdf.OpenSysML+dtExpression)
		}
		if property != "" && !graph.HasProperty(owner, rdf.SysML+property) {
			// A referent stated already is the statement; adding the member
			// end's target beside it would mask a disagreement.
			graph.Add(owner, rdf.SysMLTerm(property), member)
		}
	}
	return nil
}

// deriveMultiplicities states a feature's collapsed bounds and the range
// itself from the MultiplicityRange it owns.
func (n *normalizer) deriveMultiplicities() {
	graph := n.graph
	for _, subject := range graph.Subjects() {
		if n.meta(subject) != mMultiplicityRange {
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
		bounds, stated := n.rangeBounds(subject)
		switch {
		case stated > 0 && !n.elementForm:
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
}

// rangeBounds is a MultiplicityRange's bounds — the stated ones, or else its
// owned bound expressions in ownedRelationship order — and how many it states.
func (n *normalizer) rangeBounds(subject rdf.Term) ([]rdf.Term, int) {
	graph, meta := n.graph, n.meta
	var bounds []rdf.Term
	for _, property := range []string{pLowerBound, pUpperBound} {
		bounds = append(bounds, graph.Objects(subject, rdf.SysML+property)...)
	}
	stated := len(bounds)
	if stated == 0 {
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
	return bounds, stated
}

// deriveSuccessionEnds states the ends of a succession written between
// members through its unnamed reference features: the member an end targets
// where the toolkit resolves it, `then` beside the next member where it names
// nothing.
func (n *normalizer) deriveSuccessionEnds() {
	graph := n.graph
	for _, subject := range graph.Subjects() {
		m := n.meta(subject)
		if m == "" || !ontology.IsAncestorOrSelf(m, "Succession") {
			continue
		}
		owner := n.memberOwner[subject.Value]
		if owner.Value == "" {
			continue
		}
		source, target := n.successionEnds(subject)
		if source.Value != "" {
			graph.Add(subject, rdf.SysMLTerm(pSourceFeature), source)
		}
		previous, next := n.sequencedNeighbours(owner, subject)
		if source.Value == "" && previous.Value != "" && target.Value != "" {
			graph.Add(subject, rdf.OpenSysMLTerm(xSourceMember), previous)
		}
		switch {
		case target.Value != "" && target != next:
			graph.Add(subject, rdf.SysMLTerm(pTargetFeature), target)
			if source.Value == "" {
				graph.Add(subject, rdf.OpenSysMLTerm(xEndForm), rdf.String(formThen))
			}
		case graph.HasProperty(subject, rdf.SysML+pTargetFeature):
		case next.Value != "":
			graph.Add(subject, rdf.OpenSysMLTerm(xEndForm), rdf.String(formThen))
			graph.Add(subject, rdf.OpenSysMLTerm(xTargetMember), next)
		}
	}
}

// successionEnds is what a succession's end features refer to, falling back
// to the featureTarget of its first and last unnamed ends.
func (n *normalizer) successionEnds(subject rdf.Term) (source, target rdf.Term) {
	graph, meta, ownerMembers := n.graph, n.meta, n.ownerMembers
	if referents := successionEndReferents(graph, meta, subject); len(referents) == 2 {
		source, target = referents[0], referents[1]
	}
	if ends := ownerMembers[subject.Value]; len(ends) >= 2 {
		if t := firstIRI(graph, ends[0], "featureTarget"); source.Value == "" && t.IsIRI() && t != ends[0] {
			source = t
		}
		last := ends[len(ends)-1]
		if t := firstIRI(graph, last, "featureTarget"); target.Value == "" && t.IsIRI() && t != last {
			target = t
		}
	}
	if source.Value != "" {
		graph.Add(subject, rdf.SysMLTerm(pSourceFeature), source)
	}
	return source, target
}

// sequencedNeighbours is the members a `then` is written between: the
// feature (or `first` member) before it, which an empty source end sequences
// from (SysML v2 1.0 § 7.17.4), and the next, which `then` ahead of it targets.
func (n *normalizer) sequencedNeighbours(owner, subject rdf.Term) (previous, next rdf.Term) {
	sequenced := func(t rdf.Term) bool {
		m := n.meta(t)
		return m != "" && !ontology.IsAncestorOrSelf(m, "Succession") &&
			(m == mMembership || !relationshipLike(m))
	}
	members := n.ownerMembers[owner.Value]
	for i, member := range members {
		if member != subject {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if sequenced(members[j]) {
				previous = members[j]
				break
			}
		}
		for _, candidate := range members[i+1:] {
			if sequenced(candidate) {
				next = candidate
				break
			}
		}
		break
	}
	return previous, next
}

// stateMemberIndices states every member's place where the positional order —
// the order the names are derived by — differs from the subject order the
// decoder falls back to: a positional member takes its position, every other
// member keeps the slot it appears in, and a result expression keeps none —
// its trailing place is the absence of an index. An owner whose members state
// an index keeps it: the order it states is the order it takes.
func (n *normalizer) stateMemberIndices() {
	for owner, members := range n.ownerMembers {
		all, merged, children, indexed := n.mergedMemberOrder(owner, members)
		if indexed || sameTerms(all, merged) {
			continue
		}
		for i, m := range merged {
			if !children[m.Value] || n.meta(n.memberMembership[m.Value]) == mResultExpressionMembership {
				continue
			}
			n.indexMember(owner, m, i)
		}
	}
}

// mergedMemberOrder is the owner's members in subject order, the same slots
// with the positional members in their positional order, the members among
// them, and whether one already states an index.
func (n *normalizer) mergedMemberOrder(owner string, members []rdf.Term) (all, merged []rdf.Term, children map[string]bool, indexed bool) {
	positional := map[string]bool{}
	for _, m := range members {
		positional[m.Value] = true
	}
	children = map[string]bool{}
	next := 0
	for _, subject := range n.graph.Subjects() {
		if o, ok := n.memberOwner[subject.Value]; !ok || o.Value != owner {
			continue
		}
		if n.graph.HasProperty(subject, rdf.OpenSysML+xMemberIndex) {
			return nil, nil, nil, true
		}
		children[subject.Value] = true
		all = append(all, subject)
		if positional[subject.Value] {
			merged = append(merged, members[next])
			next++
		} else {
			merged = append(merged, subject)
		}
	}
	merged = append(merged, members[next:]...)
	return all, merged, children, false
}

// indexMember states the member's place on it, its owning membership, and any
// plain Membership of the owner naming it.
func (n *normalizer) indexMember(owner string, m rdf.Term, i int) {
	graph := n.graph
	graph.Add(m, rdf.OpenSysMLTerm(xMemberIndex), rdf.Int(i))
	if ms, ok := n.memberMembership[m.Value]; ok {
		graph.Add(ms, rdf.OpenSysMLTerm(xMemberIndex), rdf.Int(i))
	}
	for _, ms := range graph.Objects(rdf.IRI(owner), rdf.SysML+pOwnedRelationship) {
		if n.meta(ms) == mMembership && firstIRI(graph, ms, pMemberElement) == m {
			graph.Add(ms, rdf.OpenSysMLTerm(xMemberIndex), rdf.Int(i))
		}
	}
}

func sameTerms(a, b []rdf.Term) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// markBodies flags the elements written with a body: those with members,
// the compact form stating no hasBody flag.
func (n *normalizer) markBodies() {
	graph := n.graph
	bodied := n.bodiedOwners()
	for _, subject := range graph.Subjects() {
		if !bodied[subject.Value] || graph.HasProperty(subject, rdf.OpenSysML+xHasBody) {
			continue
		}
		m := n.meta(subject)
		// A satisfy's members are its head's subject parameter and the
		// relationships it implies, not a body.
		if m == "SatisfyRequirementUsage" {
			continue
		}
		// A relationship's members that are metadata usages are its `#`
		// prefixes, not a body, so owning only those braces nothing.
		if isRelationship(m) && n.onlyMetadataMembers(subject) {
			continue
		}
		if m != "" && !expressionMetaclasses[m] && !n.membershipSubject[subject.Value] {
			graph.Add(subject, rdf.OpenSysMLTerm(xHasBody), rdf.Bool(true))
		}
	}
}

// bodiedOwners is every owner with a member written in a body.
func (n *normalizer) bodiedOwners() map[string]bool {
	graph, meta, memberMembership := n.graph, n.meta, n.memberMembership
	bodied := map[string]bool{}
	for member, owner := range n.memberOwner {
		// A subaction's one action and a connector's unnamed ends are written
		// in the head, not in a body.
		if meta(owner) == mSubaction || meta(memberMembership[member]) == mTransitionFeatureMembership {
			continue
		}
		// A transition's `then` succession is its head's target, and the chain
		// of `first a.b` and its EmptyParameterMembers belong to its head too:
		// none is a body.
		if meta(owner) == mTransition {
			m, ms := rdf.IRI(member), meta(memberMembership[member])
			if meta(m) == mSuccession || (meta(m) == mFeature && ms == mOwningMembership) ||
				(ms == mParameterMembership && !graph.HasProperty(m, rdf.SysML+pDeclaredName)) {
				continue
			}
		}
		if m := rdf.IRI(member); meta(memberMembership[member]) == mEndFeatureMembership &&
			!graph.HasProperty(m, rdf.SysML+pDeclaredName) {
			continue
		}
		bodied[owner.Value] = true
	}
	return bodied
}

func (n *normalizer) onlyMetadataMembers(subject rdf.Term) bool {
	for _, member := range n.ownerMembers[subject.Value] {
		if n.meta(member) != "MetadataUsage" {
			return false
		}
	}
	return true
}

// deriveSatisfySubjects follows each satisfy's subject chain —
// SubjectMembership -> ReferenceUsage -> FeatureValue -> expression — which
// the collapsed `subject` states by its referent.
func (n *normalizer) deriveSatisfySubjects() {
	for _, subject := range n.graph.Subjects() {
		if n.meta(subject) == "SatisfyRequirementUsage" {
			n.deriveSatisfySubject(subject)
		}
	}
}

// deriveSatisfySubject is deriveSatisfySubjects for one satisfy.
func (n *normalizer) deriveSatisfySubject(subject rdf.Term) {
	graph, meta := n.graph, n.meta
	// Only the reference form is a satisfy head: unnamed, naming the
	// requirement it satisfies by what it subsets.
	if !graph.HasProperty(subject, rdf.OpenSysML+xEndForm) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredName) &&
		graph.HasProperty(subject, rdf.SysML+"subsets") {
		graph.Add(subject, rdf.OpenSysMLTerm(xEndForm), rdf.String("satisfy"))
	}
	if graph.HasProperty(subject, rdf.SysML+"subject") {
		return
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

// deriveQualifiedNames states the qualified name of every element the
// compact form carries none for, since every name the decoder writes is read
// from one. Members use their owner's position among its members; roots use
// their position among the roots the same way.
func (n *normalizer) deriveQualifiedNames() {
	n.roots = n.collectRoots()
	n.stateQualifiedNames()
}

// collectRoots is the elements no membership owns, in memberIndex order.
func (n *normalizer) collectRoots() []rdf.Term {
	graph, meta, memberOwner, nodeMember, membershipSubject := n.graph, n.meta, n.memberOwner, n.nodeMember, n.membershipSubject
	roots := []rdf.Term{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if membershipSubject[subject.Value] || nodeMember[subject.Value] && expressionMetaclasses[m] {
			continue
		}
		_, owned := memberOwner[subject.Value]
		if m == "" || expressionMetaclasses[m] && !(m == mMembership && owned) {
			continue
		}
		if !owned && relationshipLike(m) && hasOwner(graph, subject) {
			continue
		}
		if !owned {
			roots = append(roots, subject)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return intOf(graph, roots[i], rdf.OpenSysML+xMemberIndex) < intOf(graph, roots[j], rdf.OpenSysML+xMemberIndex)
	})
	return roots
}

// nameOf is the qualified name derived for subject, or empty for an element
// that contributes none.
func (n *normalizer) nameOf(subject rdf.Term) string {
	graph, memberOwner, ownerMembers, qname, visiting, roots := n.graph, n.memberOwner, n.ownerMembers, n.qname, n.visiting, n.roots
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
		base = n.nameOf(owner)
	}
	name, _ := graph.Lexical(subject, rdf.SysML+pDeclaredName)
	if name == "" {
		name, _ = graph.Lexical(subject, rdf.SysML+pDeclaredShortName)
	}
	siblings := roots
	if owned {
		siblings = ownerMembers[owner.Value]
	}
	if name == "" {
		if !owned {
			// An unnamed element no membership owns — the root namespace
			// is the one — contributes no name.
			qname[subject.Value] = ""
			return ""
		}
		index := 0
		for i, sibling := range siblings {
			if sibling == subject {
				index = i
				break
			}
		}
		q := qualify(base, "", index)
		qname[subject.Value] = q
		return q
	}
	q := qualify(base, name, 0)
	// A name an earlier sibling already took is not an identity: the
	// later element is addressed by its position.
	for i, sibling := range siblings {
		if sibling != subject {
			continue
		}
		for _, earlier := range siblings[:i] {
			if prior := n.nameOf(earlier); prior != "" && prior == q {
				q = qualify(base, "", i)
				break
			}
		}
		break
	}
	qname[subject.Value] = q
	return q
}

// stateQualifiedNames writes nameOf onto every element it names.
func (n *normalizer) stateQualifiedNames() {
	graph, meta, memberOwner, nodeMember, membershipSubject := n.graph, n.meta, n.memberOwner, n.nodeMember, n.membershipSubject
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if membershipSubject[subject.Value] || nodeMember[subject.Value] && expressionMetaclasses[m] {
			continue
		}
		if graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
			continue
		}
		_, owned := memberOwner[subject.Value]
		if m == "" || expressionMetaclasses[m] && !(m == mMembership && owned) {
			continue
		}
		if !owned && relationshipLike(m) && hasOwner(graph, subject) {
			// A relationship with an owner is implied by that owner.
			continue
		}
		if !owned {
			name, _ := graph.Lexical(subject, rdf.SysML+pDeclaredName)
			if name == "" {
				name, _ = graph.Lexical(subject, rdf.SysML+pDeclaredShortName)
			}
			if name == "" {
				continue
			}
		}
		graph.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(n.nameOf(subject)))
	}
}

// derivePerformExpressions writes a perform's action — the type its head is
// typed by — back as the expression the `perform` statement names.
func (n *normalizer) derivePerformExpressions() {
	graph := n.graph
	for _, subject := range graph.Subjects() {
		if n.meta(subject) != mPerform || graph.HasProperty(subject, rdf.OpenSysML+xExpression) {
			continue
		}
		// A state's `entry action e : A` and a transition's `do action f : A`
		// are performed actions too, written as usage heads after the keyword.
		ms := firstIRI(graph, subject, pOwningRelationship, pOwningMembership)
		if m := n.meta(ms); m == mSubaction || m == mTransitionFeatureMembership {
			n.statePerformedKeyword(subject, ms, m)
			continue
		}
		for _, target := range graph.Objects(subject, rdf.SysML+"type") {
			if !target.IsIRI() {
				graph.Add(subject, rdf.OpenSysMLTerm(xExpression), target)
				continue
			}
			if name, ok := n.qname[target.Value]; ok && name != "" {
				graph.Add(subject, rdf.OpenSysMLTerm(xExpression), rdf.String(name))
			}
		}
	}
}

// statePerformedKeyword restates a subaction membership's kind as the
// declared keyword of the unnamed performed action it owns.
func (n *normalizer) statePerformedKeyword(subject, ms rdf.Term, m string) {
	graph := n.graph
	if graph.HasProperty(subject, rdf.SysML+pDeclaredName) || !graph.HasProperty(subject, rdf.SysML+pReferences) {
		return
	}
	if kind, ok := graph.Lexical(ms, rdf.SysML+pKind); ok && m == mSubaction {
		graph.Add(subject, rdf.OpenSysMLTerm(xDeclaredKeyword), rdf.String(kind))
	}
}

// markImplicitKinds flags the toolkit ReferenceUsages that print no keyword:
// ReferenceUsage is the kindless member metaclass — `ref` states the kind when
// the keyword is written, and the compact form records no keyword for it. A
// parameter whose membership spells its keyword — a satisfy's `subject` — keeps it.
func (n *normalizer) markImplicitKinds() {
	graph, meta := n.graph, n.meta
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
}

// deriveTransitionHeads states a TransitionUsage's collapsed head from the
// structure SysML v2 1.0 § 8.3.18.9 gives it: the source Membership, the trigger
// AcceptActionUsage, and the SuccessionAsUsage whose second end names the target.
func deriveTransitionHeads(graph *rdf.Graph, meta func(rdf.Term) string) {
	chains := chainOwnerIndex(graph, meta)
	// tail is the feature a chain's last link names; any other term is itself.
	tail := func(term rdf.Term) rdf.Term {
		if !term.IsIRI() || meta(term) != mFeature {
			return term
		}
		links, err := chainLinksOf(graph, meta, chains, term)
		if err != nil || len(links) == 0 {
			links = graph.Objects(term, rdf.SysML+pChainingFeature)
		}
		if len(links) == 0 {
			return term
		}
		return links[len(links)-1]
	}
	for _, subject := range graph.Subjects() {
		if meta(subject) != mTransition {
			continue
		}
		for i, ms := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			member := firstIRI(graph, ms, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement)
			if member.Value == "" {
				continue
			}
			// The first member a transition owns may be its FeatureChainMember
			// owning the chain of `first a.b`: the source is the chain's last
			// link, and the chain is no body member.
			if i == 0 && meta(ms) == mOwningMembership && tail(member) != member {
				if !graph.HasProperty(subject, rdf.SysML+pSource) && !graph.HasProperty(subject, rdf.SysML+pSourceFeature) {
					graph.Add(subject, rdf.SysMLTerm(pSource), tail(member))
				}
				continue
			}
			switch meta(ms) {
			case mMembership:
				if !graph.HasProperty(subject, rdf.SysML+pSource) && !graph.HasProperty(subject, rdf.SysML+pSourceFeature) {
					graph.Add(subject, rdf.SysMLTerm(pSource), member)
				}
			case mTransitionFeatureMembership:
				// The trigger AcceptActionUsage is read by the decoder itself.
				if kind, _ := graph.Lexical(ms, rdf.SysML+pKind); kind == "effect" {
					graph.Add(subject, rdf.OpenSysMLTerm(xEffectMember), member)
					graph.Add(subject, rdf.OpenSysMLTerm(xHasEffect), rdf.Bool(true))
				}
			case mOwningMembership:
				if meta(member) == mSuccession {
					ends := successionEndReferents(graph, meta, member)
					if len(ends) == 2 && ends[1].Value != "" &&
						!graph.HasProperty(subject, rdf.SysML+pTarget) && !graph.HasProperty(subject, rdf.SysML+pTargetFeature) {
						graph.Add(subject, rdf.SysMLTerm(pTarget), tail(ends[1]))
					}
					continue
				}
				graph.Add(subject, rdf.OpenSysMLTerm(xBodyMember), member)
				graph.Add(subject, rdf.OpenSysMLTerm(xHasBody), rdf.Bool(true))
			}
		}
	}
}

// successionEndReferents is what each end feature a succession owns through
// EndFeatureMembership refers to, in order; an end that refers to nothing is empty.
func successionEndReferents(graph *rdf.Graph, meta func(rdf.Term) string, succession rdf.Term) []rdf.Term {
	var out []rdf.Term
	for _, ms := range graph.Objects(succession, rdf.SysML+pOwnedRelationship) {
		if meta(ms) != mEndFeatureMembership {
			continue
		}
		end := firstIRI(graph, ms, pMemberElement, pOwnedMemberElement, pOwnedRelatedElement)
		if end.Value == "" {
			continue
		}
		referent := firstObject(graph, end, pReferences)
		for _, rel := range graph.Objects(end, rdf.SysML+pOwnedRelationship) {
			if meta(rel) == mReferenceSubsetting {
				referent = firstObject(graph, rel, pReferencedFeature, pTarget)
			}
		}
		out = append(out, referent)
	}
	return out
}

// dropStatedDefaults copies the graph without the triples the sparse form
// writes where this mapping writes nothing: a stated default reads identically
// to an absent one, and a printed keyword would declare it twice.
func dropStatedDefaults(graph *rdf.Graph, meta func(rdf.Term) string, elementForm bool, chainIndex map[string][]rdf.Term) (*rdf.Graph, error) {
	if !elementForm {
		// Only the element form states defaults and implied restatements; a
		// graph minted to this mapping means every triple it writes.
		return graph, nil
	}
	d, err := newDefaultDropper(graph, meta, chainIndex)
	if err != nil {
		return nil, err
	}
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if d.dropped(triple.Subject.Value) || d.ownsDroppedMember(triple.Subject) {
			continue
		}
		object, keep := triple.Object, true
		if object.IsIRI() {
			var unresolvedName string
			object, unresolvedName, keep = d.iriObject(triple)
			if keep && unresolvedName != "" {
				out.Add(triple.Subject, triple.Predicate, writtenReference(unresolvedName))
				continue
			}
		}
		if keep && !object.IsIRI() {
			object, keep = d.literalObject(triple, object)
		}
		if !keep {
			continue
		}
		triple.Object = object
		out.AddTriple(triple)
	}
	for member, name := range d.unresolvedRef {
		out.Add(rdf.IRI(member), rdf.SysMLTerm(pMemberElement), rdf.String(name))
	}
	return out, nil
}

var (
	// The element collections the mapping reads members from.
	memberCollections = map[string]bool{
		"member": true, "ownedMember": true, "ownedElement": true,
		"ownedFeature": true, "input": true, "output": true,
	}
	// A membership's generic relationship ends restate its member ends; when
	// the member is an expression part they would reference a node.
	membershipEnds = map[string]bool{
		"source": true, "target": true, "relatedElement": true,
	}
	// The collapsed head properties a minted relationship restates.
	collapsedProps = map[string]bool{
		"type": true, "specializes": true, "subsets": true,
		"redefines": true, "references": true,
	}
	// The structural ends an element owns and is owned through: a chain
	// segment stays an element to these, a written name to everything else.
	structuralProps = map[string]bool{
		pOwningRelatedElement: true, pOwner: true, pOwningNamespace: true,
		pOwningRelationship: true, pOwningMembership: true, pOwnedRelationship: true,
		pOwnedRelatedElement: true, "relatedElement": true, pMemberElement: true,
		pOwnedMemberElement: true, pMembershipOwningNamespace: true,
		"owningFeatureMembership": true, "owningFeature": true,
	}
	// The properties whose literal objects are a reference written as a name;
	// each is canonicalized so writing it back quotes it once.
	referenceNameProps = map[string]bool{
		pMemberElement: true, "referent": true, pTargetFeature: true,
		pFunction: true, pClient: true, pSupplier: true, pSource: true,
		pTarget: true, "relatedElement": true, "importedNamespace": true,
		"type": true, "specializes": true, "subsets": true,
		"redefines": true, "references": true,
	}
)

// derivedCollections reports the derived collections the full form restates
// over owned structure: the owned relationships state them, so the derived
// list cannot contradict them.
func derivedCollections(local string) bool {
	switch local {
	case pChainingFeature, "relatedFeature", "relatedType", "associationEnd", "connectorEnd":
		return true
	}
	return local == pArgument || local == "definition" || strings.HasSuffix(local, "Definition")
}

// defaultDropper indexes what the element form restates or derives, so the
// copy can tell a statement from a restatement triple by triple.
type defaultDropper struct {
	graph              *rdf.Graph
	meta               func(rdf.Term) string
	membershipOwned    map[string]bool
	nodeOwned          map[string]bool
	membershipSubjects map[string]bool
	backed             map[string]map[string]bool
	unresolvedRef      map[string]string
	unresolvedID       map[string]string
	unresolvedTR       map[string]bool
	chainSegment       map[string]string
	unresolvedOwner    map[string]bool
	memberOwnerOf      map[string]string
	implied            map[string]bool
}

func newDefaultDropper(graph *rdf.Graph, meta func(rdf.Term) string, chainIndex map[string][]rdf.Term) (*defaultDropper, error) {
	unresolvedID, unresolvedTR := unresolvedNames(graph, meta)
	chainSegment, err := chainSegmentsIn(graph, meta, chainIndex, unresolvedID)
	if err != nil {
		return nil, err
	}
	unresolvedRef := unresolvedRefsIn(graph, meta)
	return &defaultDropper{
		graph:              graph,
		meta:               meta,
		membershipOwned:    membershipOwnedIn(graph, meta),
		nodeOwned:          nodeOwnedIn(graph, meta),
		membershipSubjects: membershipSubjectsIn(graph, meta),
		backed:             backedIn(graph, meta),
		unresolvedRef:      unresolvedRef,
		unresolvedID:       unresolvedID,
		unresolvedTR:       unresolvedTR,
		chainSegment:       chainSegment,
		unresolvedOwner:    unresolvedOwnersIn(graph, unresolvedRef),
		memberOwnerOf:      memberOwnersIn(graph, meta),
		implied:            impliedIn(graph),
	}, nil
}

// dropped reports whether every triple of the subject is dropped: an element
// a membership owns, an implied one, or an unresolved-name annotation.
func (d *defaultDropper) dropped(v string) bool {
	return d.membershipOwned[v] || d.implied[v] || d.unresolvedTR[v]
}

// ownsDroppedMember reports a membership existing only to own an artifact the
// copy drops; a member the name restore already handled stays literal.
func (d *defaultDropper) ownsDroppedMember(subject rdf.Term) bool {
	if !d.membershipSubjects[subject.Value] {
		return false
	}
	member := firstIRI(d.graph, subject, pMemberElement, pOwnedMemberElement)
	return member.Value != "" && d.unresolvedTR[member.Value]
}

func (d *defaultDropper) impliedIncluded(subject rdf.Term) bool {
	return d.graph.HasProperty(subject, rdf.SysML+"isImpliedIncluded")
}

// iriObject is the object an IRI-valued triple is copied with — a chain
// segment becomes the chain's text — the name an unresolved reference was
// written as, and whether the triple is kept at all.
func (d *defaultDropper) iriObject(triple rdf.Triple) (object rdf.Term, unresolvedName string, keep bool) {
	object = triple.Object
	local := rdf.LocalName(triple.Predicate.Value)
	if text, segment := d.chainSegment[object.Value]; segment && !structuralProps[local] {
		object = rdf.TypedLiteral(text, rdf.OpenSysML+dtExpression)
	}
	// A reference the writer could not resolve becomes the name it wrote,
	// once the derived edges restating it are dropped.
	if tail := object.Value[strings.LastIndex(object.Value, ":")+1:]; d.unresolvedID[tail] != "" && d.meta(object) == "" {
		unresolvedName = d.unresolvedID[tail]
	}
	if _, unresolved := d.unresolvedRef[triple.Subject.Value]; unresolved && local == pMemberElement {
		return object, unresolvedName, false
	}
	if d.dropsRestatedEnd(triple.Subject, local, object) {
		return object, unresolvedName, false
	}
	if d.unresolvedOwner[triple.Subject.Value] && d.meta(object) == "" && unresolvedName == "" {
		switch local {
		case "referent", pTargetFeature, pFunction:
			return object, unresolvedName, false
		}
	}
	if d.dropsDerivedEdge(triple.Subject, local, object) {
		return object, unresolvedName, false
	}
	return object, unresolvedName, true
}

// dropsRestatedEnd reports an IRI edge that restates ownership or a member end
// the copy already carries.
func (d *defaultDropper) dropsRestatedEnd(subject rdf.Term, local string, object rdf.Term) bool {
	membership := d.membershipSubjects[subject.Value]
	switch {
	case (d.nodeOwned[object.Value] || d.dropped(object.Value)) &&
		(memberCollections[local] || membership && membershipEnds[local]):
		return true
	case membership && membershipEnds[local]:
		// A membership's generic ends restate its member ends; a member an
		// expression owns by node is not among them.
		return true
	case membershipEnds[local] && relationshipLike(d.meta(subject)) && d.meta(subject) != "ReferenceSubsetting":
		// A relationship's generic ends restate its member ends, the client
		// and supplier it owns by name aside.
		return true
	case local == pAnnotatedElement && d.memberOwnerOf[subject.Value] == object.Value:
		// An annotating member's annotated element is its owner, restated:
		// the compact form states ownership alone.
		return true
	case d.nodeOwned[subject.Value] && (local == pOwningNamespace || local == pOwner):
		// A node's namespace is its owning membership's, which the full
		// form also states directly.
		return true
	}
	return false
}

// dropsDerivedEdge reports a derived head property or argument list no
// declared relationship backs.
func (d *defaultDropper) dropsDerivedEdge(subject rdf.Term, local string, object rdf.Term) bool {
	switch local {
	case "type", "specializes", "subsets", "redefines", "references":
		return !relationshipLike(d.meta(subject)) && !d.backed[subject.Value][object.Value] && d.impliedIncluded(subject)
	case pArgument:
		// The argument list is derived from the parameters the expression
		// owns; the owned structure is what is stated.
		return d.impliedIncluded(subject) && ownsParameterMembership(d.graph, d.meta, subject)
	}
	return false
}

// literalObject is the object a literal-valued triple is copied with, and
// whether it is kept at all.
func (d *defaultDropper) literalObject(triple rdf.Triple, object rdf.Term) (rdf.Term, bool) {
	local := rdf.LocalName(triple.Predicate.Value)
	json := strings.HasPrefix(triple.Predicate.Value, rdf.AnnotationJSON)
	if json && (memberCollections[local] || membershipEnds[local] ||
		derivedCollections(local) && d.impliedIncluded(triple.Subject) ||
		collapsedProps[local] && !relationshipLike(d.meta(triple.Subject)) && d.impliedIncluded(triple.Subject)) {
		// A collection restated under json: agrees with the typed triples
		// only while none is dropped; the derived list is dropped whole instead.
		return object, false
	}
	if referenceNameProps[local] && object.Datatype != rdf.OpenSysML+dtExpression && !json {
		object = rdf.String(canonicalName(object.Value))
	}
	if local == pQualifiedName && !json {
		object = rdf.String(plainQualifiedName(object.Value))
	}
	return object, !d.dropsLiteral(triple.Subject, local, object)
}

// dropsLiteral reports a literal that states a default or a derivable value.
func (d *defaultDropper) dropsLiteral(subject rdf.Term, local string, object rdf.Term) bool {
	graph, meta := d.graph, d.meta
	switch {
	case local == pQualifiedName && d.impliedIncluded(subject) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredName) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredShortName):
		// The full form derives a qualified name for the unnamed too;
		// the compact form leaves the name to be derived.
		return true
	case local == pQualifiedName && underSubaction(graph, meta, subject):
		// This mapping positions a subaction's action under its
		// membership (`S::@0::ops`); the full form's name skips it.
		return true
	case local == pElementID && strings.HasSuffix(subject.Value, object.Value):
		// An elementId identical to the IRI's id is derivable.
		return true
	case local == "isReference" && object.Value == "true" && d.impliedIncluded(subject):
		// A derived reference usage writes no `ref` of its own.
		return true
	case (local == "mayTimeVary" || local == "isVariable") && object.Value == "true" && d.impliedIncluded(subject):
		// A feature is variable and time-varying without a keyword.
		return true
	case local == "isConstant" && object.Value == "true" &&
		graph.BoolValue(subject, rdf.SysML+"isEnd") && d.impliedIncluded(subject):
		// KerML Feature: `isEnd and isVariable implies isConstant`, so a
		// variable end is constant without the keyword.
		return true
	case local == pVisibility && object.Value == "public":
		// Public is the visibility a member writes nothing for.
		return true
	case local == "isComposite" && object.Value == "true":
		return d.compositeByDefault(subject)
	}
	return false
}

// compositeByDefault reports a usage that is composite without a keyword.
func (d *defaultDropper) compositeByDefault(subject rdf.Term) bool {
	graph, meta := d.graph, d.meta
	switch meta(subject) {
	case "PartUsage", mPortUsage, "ItemUsage", "ConstraintUsage":
		// Compositional usages are composite without a keyword.
		return true
	}
	// A usage nested in a type is composite unless `ref` (SysML
	// Usage::isComposite); only one elsewhere writes `composite`.
	owner := firstIRI(graph, firstIRI(graph, subject, pOwningRelationship), pOwningRelatedElement, pOwner)
	return ontology.IsAncestorOrSelf(meta(subject), "Usage") && owner.IsIRI() &&
		ontology.IsAncestorOrSelf(meta(owner), "Type")
}

// membershipOwnedIn is every element owned by a membership — an annotation of
// the membership, or an implied-include membership nested under one — which is
// no member of the model; the compact form writes it as the memberElement
// literal alone.
func membershipOwnedIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]bool {
	membershipOwned := map[string]bool{}
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
	return membershipOwned
}

// nodeOwnedIn is every expression part the collections a derived property
// lists count — a MultiplicityRange or a bound is part of its owner's
// structure, not a member — as a member of the element collections the
// mapping reads.
func nodeOwnedIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]bool {
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
			(m == mParameterMembership || m == mReturnParameterMembership) && expressionMetaclasses[meta(owner)] {
			nodeOwned[member.Value] = true
		}
	}
	return nodeOwned
}

// membershipSubjectsIn is every membership and import.
func membershipSubjectsIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]bool {
	membershipSubjects := map[string]bool{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m != "" && (owningMembershipLike(m) || strings.HasSuffix(m, "Membership") || strings.HasSuffix(m, "Import")) {
			membershipSubjects[subject.Value] = true
		}
	}
	return membershipSubjects
}

// backedIn is owner -> target for every declared relationship: the full form
// restates each as a derived property on its owner; only an edge a minted
// relationship element carries is a statement. A derived edge unbacked by one
// is dropped — the toolkit marks the elements it derives these for with
// isImpliedIncluded.
func backedIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]map[string]bool {
	backed := map[string]map[string]bool{}
	for _, subject := range graph.Subjects() {
		m := meta(subject)
		if m == "" || !relationshipLike(m) {
			continue
		}
		if graph.BoolValue(subject, rdf.SysML+pIsImplied) {
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
				if m == mConjugatedPortTyping && object.IsIRI() {
					// The head writes `~P`; the derived type `~P` is not what it states.
					object = originalPortDefinition(graph, meta, subject, object)
				}
				if backed[owner.Value] == nil {
					backed[owner.Value] = map[string]bool{}
				}
				backed[owner.Value][object.Value] = true
			}
		}
	}
	return backed
}

// unresolvedRefsIn is member -> name for every memberElement the compact form
// wrote as a `{"@ref"}` name the full form resolved to a library IRI and
// annotated with the name it could not resolve; the name is what the member states.
func unresolvedRefsIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]string {
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
			unresolvedRef[owner.Value] = canonicalName(body)
		}
	}
	return unresolvedRef
}

// chainSegmentsIn is the text of every unnamed Feature a relationship relates
// as a feature chain's segment: `a.b.c` is the chain of features its
// FeatureChaining elements name, so a reference to it is written as that
// chain's text, not as the element.
func chainSegmentsIn(graph *rdf.Graph, meta func(rdf.Term) string, chainIndex map[string][]rdf.Term, unresolvedID map[string]string) (map[string]string, error) {
	chainSegment := map[string]string{}
	for _, subject := range graph.Subjects() {
		text, ok, err := chainTextIn(graph, meta, chainIndex, unresolvedID, subject)
		if err != nil {
			return nil, err
		}
		if ok {
			chainSegment[subject.Value] = text
		}
	}
	// The element owning an unresolved membership is annotated the same way
	// through its derived referent properties.
	return chainSegment, nil
}

// unresolvedOwnersIn is every element owning an unresolved membership, which is
// annotated the same way through its derived referent properties.
func unresolvedOwnersIn(graph *rdf.Graph, unresolvedRef map[string]string) map[string]bool {
	unresolvedOwner := map[string]bool{}
	for member := range unresolvedRef {
		if owner := firstIRI(graph, rdf.IRI(member), pOwningRelatedElement, pOwner); owner.Value != "" {
			unresolvedOwner[owner.Value] = true
		}
	}
	// memberOwnerOf is the element a membership-owned member belongs to.
	return unresolvedOwner
}

// memberOwnersIn is member -> the element a membership-owned member belongs to.
func memberOwnersIn(graph *rdf.Graph, meta func(rdf.Term) string) map[string]string {
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
	return memberOwnerOf
}

// impliedIn is every implied element, which restates a derivation rather than
// stating one: the extra specialization a minted element does not declare is
// dropped whole.
func impliedIn(graph *rdf.Graph) map[string]bool {
	implied := map[string]bool{}
	for _, triple := range graph.Triples() {
		if rdf.LocalName(triple.Predicate.Value) == "isImplied" &&
			!triple.Object.IsIRI() && triple.Object.Value == "true" {
			implied[triple.Subject.Value] = true
		}
	}
	return implied
}

// ownsParameterMembership reports whether subject owns a ParameterMembership.
func ownsParameterMembership(graph *rdf.Graph, meta func(rdf.Term) string, subject rdf.Term) bool {
	for _, rel := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
		if meta(rel) == mParameterMembership {
			return true
		}
	}
	return false
}

// writtenReference is the literal for a reference written as name: a qualified
// name stays a name, a feature chain is expression text so it is not re-quoted.
func writtenReference(name string) rdf.Term {
	if _, ok := source.QualifiedNameSegments(name); ok {
		return rdf.String(name)
	}
	return rdf.TypedLiteral(name, rdf.OpenSysML+dtExpression)
}

// unresolvedNames indexes the references the writer could not resolve: the
// uuid5(OID, "unresolved:"+name) id the full form mints -> the name written, and
// the TextualRepresentation elements carrying those names.
func unresolvedNames(graph *rdf.Graph, meta func(rdf.Term) string) (map[string]string, map[string]bool) {
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
		unresolvedID[identity.UnresolvedElementID(body)] = canonicalName(body)
		unresolvedTR[subject.Value] = true
	}
	return unresolvedID, unresolvedTR
}

// plainQualifiedName spells the identity form of a qualified name.
func plainQualifiedName(name string) string {
	segments := identitySegments(name)
	for i, segment := range segments {
		segments[i] = identitySegment(identityName(segment))
	}
	return strings.Join(segments, "::")
}

// canonicalName returns a written qualified name in its canonical spelling:
// segments split on `::` outside quotes, each requoted with its escapes kept.
func canonicalName(name string) string {
	segments, ok := source.QualifiedNameSegments(name)
	if !ok {
		return name
	}
	return source.QualifiedNameOf(segments)
}

// underSubaction reports whether a StateSubactionMembership owns subject or
// one of the elements it is nested in.
func underSubaction(graph *rdf.Graph, meta func(rdf.Term) string, subject rdf.Term) bool {
	for seen := map[string]bool{}; subject.IsIRI() && !seen[subject.Value]; {
		seen[subject.Value] = true
		ms := firstIRI(graph, subject, pOwningRelationship)
		if meta(ms) == mSubaction {
			return true
		}
		subject = firstIRI(graph, ms, pOwningRelatedElement, pOwner)
	}
	return false
}
