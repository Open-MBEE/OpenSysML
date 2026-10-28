package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

const mappingRDFSSubClassOf = "http://www.w3.org/2000/01/rdf-schema#subClassOf"

func mappingTypeConforms(graph *rdf.Graph, subject rdf.Term, ancestor string) bool {
	if graph == nil {
		return false
	}
	for _, typ := range graph.ObjectsView(subject, rdf.RDFType) {
		if mappingTypeIRIConforms(graph, typ.Value, ancestor) {
			return true
		}
	}
	return false
}

func mappingTypeIRIConforms(graph *rdf.Graph, typ, ancestor string) bool {
	seen := map[string]bool{}
	queue := []string{typ}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		class := ontology.LocalName(current)
		if ontology.IsAncestorOrSelf(class, ancestor) {
			return true
		}
		if resolved := apiJSONOwnedAbsenceMetaclassInGraph(graph, current); resolved != "" &&
			ontology.IsAncestorOrSelf(resolved, ancestor) {
			return true
		}
		for _, parent := range graph.ObjectsView(rdf.IRI(current), mappingRDFSSubClassOf) {
			if parent.IsIRI() {
				queue = append(queue, parent.Value)
			}
		}
	}
	return false
}

func mappingHasElementName(graph *rdf.Graph, subject rdf.Term) bool {
	for _, property := range []string{
		pDeclaredName, pDeclaredShortName,
	} {
		if graph.HasProperty(subject, rdf.SysML+property) {
			return true
		}
	}
	return false
}

func mappingHasMemberName(graph *rdf.Graph, subject rdf.Term) bool {
	return graph.HasProperty(subject, rdf.SysML+pMemberName) ||
		graph.HasProperty(subject, rdf.SysML+pMemberShortName)
}

func mappingHasIdentity(graph *rdf.Graph, subject rdf.Term) bool {
	return mappingHasElementName(graph, subject) ||
		graph.HasProperty(subject, rdf.SysML+"declaredKeyword")
}

func mappingFeatureChain(graph *rdf.Graph, subject rdf.Term, context *mappingQualifiedNameContext) bool {
	if !context.conforms(graph, subject, "Feature") || mappingHasElementName(graph, subject) {
		return false
	}
	if graph.HasProperty(subject, rdf.SysML+"chainingFeature") {
		return true
	}
	for _, relationship := range graph.ObjectsView(subject, rdf.SysML+pOwnedRelationship) {
		if context.conforms(graph, relationship, "FeatureChaining") {
			return true
		}
	}
	return false
}

func mappingFilterPackageNodes(graph *rdf.Graph, index *apiJSONGraphIndex) map[string]bool {
	filtered := make(map[string]bool)
	if graph == nil {
		return filtered
	}
	children := make(map[string][]rdf.Term)
	var queue []rdf.Term
	properties := [...]string{
		pOwner, pOwningRelationship, pOwningMembership, pOwningNamespace,
		pMembershipOwningNamespace, pImportOwningNamespace, pOwningRelatedElement,
	}
	for _, subject := range graph.SubjectsView() {
		for _, property := range properties {
			if owner := firstIRI(graph, subject, property); owner.IsIRI() {
				children[owner.Value] = append(children[owner.Value], subject)
			}
		}
		if !index.conforms(graph, subject, "Package") {
			continue
		}
		importer := firstIRI(graph, subject, pOwningRelationship)
		if index.conforms(graph, importer, "Import") {
			if imported, ok := graph.Object(importer, rdf.SysML+pImportedNamespace); ok && imported == subject {
				filtered[subject.Value] = true
				queue = append(queue, subject)
			}
		}
	}
	for head := 0; head < len(queue); head++ {
		for _, child := range children[queue[head].Value] {
			if filtered[child.Value] {
				continue
			}
			filtered[child.Value] = true
			queue = append(queue, child)
		}
	}
	for _, subject := range graph.SubjectsView() {
		if _, ok := filtered[subject.Value]; !ok {
			filtered[subject.Value] = false
		}
	}
	return filtered
}

func mappingFilterPackageNode(graph *rdf.Graph, subject rdf.Term, context *mappingQualifiedNameContext) bool {
	var cache map[string]bool
	if context != nil {
		cache = context.packageCache
	}
	if cache != nil {
		if filtered, ok := cache[subject.Value]; ok {
			return filtered
		}
	}
	queue := []rdf.Term{subject}
	seen := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if !current.IsIRI() || seen[current.Value] {
			continue
		}
		seen[current.Value] = true
		if context.conforms(graph, current, "Package") {
			importer := firstIRI(graph, current, pOwningRelationship)
			if context.conforms(graph, importer, "Import") {
				if imported, ok := graph.Object(importer, rdf.SysML+pImportedNamespace); ok && imported == current {
					if cache != nil {
						cache[subject.Value] = true
					}
					return true
				}
			}
		}
		for _, property := range []string{
			pOwner, pOwningRelationship, pOwningMembership, pOwningNamespace,
			pMembershipOwningNamespace, pImportOwningNamespace, pOwningRelatedElement,
		} {
			if owner := firstIRI(graph, current, property); owner.IsIRI() {
				queue = append(queue, owner)
			}
		}
	}
	if cache != nil {
		cache[subject.Value] = false
	}
	return false
}

// MappingElementSubject reports whether the subject is emitted as an Element
// rather than a reified membership edge.
func MappingElementSubject(graph *rdf.Graph, subject rdf.Term) bool {
	return mappingElementSubjectWithContext(graph, subject, map[string]bool{}, nil)
}

// mappingElementSubjectWithContext is MappingElementSubject, memoized in the
// context when there is one.
func mappingElementSubjectWithContext(
	graph *rdf.Graph,
	subject rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) bool {
	if seen[subject.Value] {
		return false
	}
	if context != nil {
		if result, ok := context.elementSubjects[subject.Value]; ok {
			return result
		}
	}
	_, ok := rdf.SubjectID(subject)
	if graph == nil || !ok ||
		!context.conforms(graph, subject, "Element") {
		return false
	}
	seen[subject.Value] = true
	result := mappingElementSubjectUncached(graph, subject, seen, context)
	if context != nil {
		context.elementSubjects[subject.Value] = result
	}
	return result
}

func mappingElementSubjectUncached(
	graph *rdf.Graph,
	subject rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) bool {
	if mappingFilterPackageNode(graph, subject, context) {
		return false
	}
	if context.conforms(graph, subject, "FeatureChaining") || mappingFeatureChain(graph, subject, context) {
		return false
	}
	if context.conforms(graph, subject, "Membership") {
		return context.conforms(graph, subject, "ElementFilterMembership") ||
			context.conforms(graph, subject, "StateSubactionMembership") ||
			mappingHasIdentity(graph, subject) ||
			graph.HasProperty(subject, rdf.OpenSysML+"memberIndex")
	}
	if mappingExpressionType(graph, subject) &&
		!mappingHasIdentity(graph, subject) &&
		!graph.HasProperty(subject, rdf.OpenSysML+"memberIndex") &&
		!mappingHasOwnerFact(graph, subject) {
		return false
	}
	for _, owner := range []rdf.Term{
		firstIRI(graph, subject, pOwningMembership),
		firstIRI(graph, subject, pOwningRelationship),
	} {
		if context.conforms(graph, owner, "Membership") {
			return mappingMembershipMemberIsElementWithContext(graph, owner, subject, seen, context)
		}
	}
	for _, typ := range graph.ObjectsView(subject, rdf.RDFType) {
		if (impliedRelationshipMetaclasses[ontology.LocalName(typ.Value)] ||
			derivedSatelliteMetaclasses[ontology.LocalName(typ.Value)]) &&
			!mappingHasIdentity(graph, subject) {
			return false
		}
	}
	return true
}

func mappingHasOwnerFact(graph *rdf.Graph, subject rdf.Term) bool {
	for _, property := range []string{
		pOwningMembership, pOwningRelationship, pMembershipOwningNamespace,
		pOwningRelatedElement, pOwningNamespace, pOwner,
	} {
		if graph.HasProperty(subject, rdf.SysML+property) {
			return true
		}
	}
	return false
}

func mappingExpressionBodyMemberSubject(graph *rdf.Graph, subject rdf.Term) bool {
	if graph == nil || !strings.HasPrefix(subject.Value, rdf.Expression) {
		return false
	}
	owner := firstIRI(graph, subject, pOwner)
	if !owner.IsIRI() {
		return false
	}
	for _, member := range graph.ObjectsView(owner, rdf.OpenSysML+"bodyMember") {
		if member == subject {
			return mappingTypeConforms(graph, subject, "Feature") &&
				!mappingExpressionType(graph, subject) &&
				!graph.HasProperty(subject, rdf.SysML+"direction")
		}
	}
	return false
}

// MappingMembershipMemberIsElement reports whether the forward mapping emits a
// membership's member as an Element rather than as an expression part.
func MappingMembershipMemberIsElement(graph *rdf.Graph, membership, member rdf.Term) bool {
	return mappingMembershipMemberIsElementWithContext(graph, membership, member, map[string]bool{}, nil)
}

func mappingMembershipMemberIsElementWithContext(
	graph *rdf.Graph,
	membership, member rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) bool {
	if context == nil {
		return mappingMembershipMemberIsElementUncached(graph, membership, member, seen, nil)
	}
	cacheKey := membership.Value + "\x00" + member.Value
	if result, ok := context.membershipElements[cacheKey]; ok {
		return result
	}
	result := mappingMembershipMemberIsElementUncached(graph, membership, member, seen, context)
	context.membershipElements[cacheKey] = result
	return result
}

func mappingMembershipMemberIsElementUncached(
	graph *rdf.Graph,
	membership, member rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) bool {
	if graph == nil {
		return false
	}
	if _, ok := rdf.SubjectID(member); !ok ||
		!context.conforms(graph, member, "Element") {
		return false
	}
	if mappingExpressionType(graph, member) &&
		graph.BoolValue(member, rdf.OpenSysML+"hasBody") &&
		!mappingHasIdentity(graph, member) {
		return false
	}
	owner := firstIRI(graph, membership, pMembershipOwningNamespace)
	if !owner.IsIRI() {
		owner = firstIRI(graph, membership, pOwningRelatedElement)
	}
	if !owner.IsIRI() {
		owner = firstIRI(graph, membership, pOwner)
	}
	resultExpression := context.conforms(graph, membership, "ResultExpressionMembership")
	ownerIsElement := owner.IsIRI() &&
		mappingElementSubjectWithContext(graph, owner, seen, context)
	if mappingExpressionType(graph, owner) && !ownerIsElement {
		return false
	}
	if mappingExpressionBodyMemberSubject(graph, owner) {
		return false
	}
	if owner.IsIRI() && !ownerIsElement && !resultExpression {
		return mappingHasElementName(graph, member) || mappingHasMemberName(graph, membership)
	}
	if context.conforms(graph, membership, "TransitionFeatureMembership") {
		for _, typ := range graph.ObjectsView(member, rdf.RDFType) {
			if expressionMetaclasses[ontology.LocalName(typ.Value)] {
				return false
			}
		}
	}
	parameter := context.conforms(graph, membership, "ParameterMembership") ||
		context.conforms(graph, membership, "ReturnParameterMembership")
	if resultExpression {
		return true
	}
	if !parameter && context.conforms(graph, membership, "FeatureMembership") {
		if graph.BoolValue(member, rdf.SysML+"isEnd") &&
			context.conforms(graph, membership, "EndFeatureMembership") {
			return false
		}
		if mappingHasElementName(graph, member) || mappingHasMemberName(graph, membership) {
			return true
		}
		for _, typ := range graph.ObjectsView(member, rdf.RDFType) {
			if ontology.IsAncestorOrSelf(ontology.LocalName(typ.Value), "Feature") {
				return true
			}
		}
		return false
	}
	for _, subjectParameter := range graph.ObjectsView(membership, rdf.SysML+pOwnedSubjectParameter) {
		if subjectParameter == member {
			return true
		}
	}
	if mappingHasElementName(graph, member) || mappingHasMemberName(graph, membership) {
		return true
	}
	if !graph.HasProperty(member, rdf.OpenSysML+"memberIndex") || mappingExpressionType(graph, member) {
		return false
	}
	if (context.conforms(graph, membership, "ParameterMembership") ||
		context.conforms(graph, membership, "ReturnParameterMembership")) &&
		mappingExpressionType(graph, owner) && mappingHasExactType(graph, member, "Feature") {
		return false
	}
	return true
}

func mappingHasExactType(graph *rdf.Graph, subject rdf.Term, expected string) bool {
	for _, typ := range graph.ObjectsView(subject, rdf.RDFType) {
		if ontology.LocalName(typ.Value) == expected {
			return true
		}
	}
	return false
}

func mappingExpressionType(graph *rdf.Graph, subject rdf.Term) bool {
	if mappingHasExactType(graph, subject, "Expression") {
		return true
	}
	for _, typ := range graph.ObjectsView(subject, rdf.RDFType) {
		if expressionMetaclasses[ontology.LocalName(typ.Value)] {
			return true
		}
	}
	return false
}

// MappingMembershipEdgeSubject reports whether the subject is a membership
// edge rather than a relationship emitted as its own element.
func MappingMembershipEdgeSubject(graph *rdf.Graph, subject rdf.Term) bool {
	return mappingTypeConforms(graph, subject, "Membership") && !MappingElementSubject(graph, subject)
}
