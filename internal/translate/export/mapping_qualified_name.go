package export

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

var (
	mappingRelationshipPredicates = [...]string{rdf.SysML + pOwningRelationship, rdf.SysML + pOwningMembership}
	mappingOwnerPredicates        = [...]string{
		rdf.SysML + pMembershipOwningNamespace, rdf.SysML + pOwningRelatedElement,
		rdf.SysML + pOwningNamespace, rdf.SysML + pOwner,
		rdf.SysML + pOwningRelationship, rdf.SysML + pOwningMembership,
	}
	mappingOwnedRelationshipPredicates = [...]string{
		rdf.SysML + pOwnedRelatedElement, rdf.SysML + pMemberElement,
		rdf.SysML + pOwnedMemberElement, rdf.SysML + pOwnedMemberFeature,
		rdf.SysML + pOwnedMemberParameter, rdf.SysML + pOwnedResultExpression,
		rdf.SysML + pOwnedMember,
	}
	mappingIncomingMembershipPredicates = [...]string{
		rdf.SysML + pMemberElement, rdf.SysML + pOwnedMemberElement,
		rdf.SysML + pOwnedMemberFeature, rdf.SysML + pOwnedMemberParameter,
		rdf.SysML + pOwnedResultExpression, rdf.SysML + pOwnedRelatedElement,
	}
)

func mappingQualifiedNames(graph *rdf.Graph) map[string]string {
	return mappingQualifiedNamesWithContext(graph, newMappingQualifiedNameContext(graph))
}

func mappingQualifiedNamesForAPIJSONImport(graph *rdf.Graph, context *mappingQualifiedNameContext) map[string]string {
	return mappingQualifiedNamesWithPositionRule(graph, context, true)
}

type mappingQualifiedNameContext struct {
	packageCache        map[string]bool
	incomingMemberships map[string][]rdf.Term
	elementSubjects     map[string]bool
	membershipElements  map[string]bool
	qualifiedNameOwners map[string]mappingQualifiedNameOwnerResult
	scratchElementSeen  map[string]bool
	scratchOwnerSeen    map[string]bool
}

type mappingQualifiedNameOwnerResult struct {
	owner    rdf.Term
	hasOwner bool
}

func newMappingQualifiedNameContext(graph *rdf.Graph) *mappingQualifiedNameContext {
	return newMappingQualifiedNameContextWithMemberships(graph, nil)
}

func newMappingQualifiedNameContextWithMemberships(graph *rdf.Graph, incoming map[string][]rdf.Term) *mappingQualifiedNameContext {
	if graph == nil {
		return &mappingQualifiedNameContext{}
	}
	if incoming == nil {
		incoming = mappingIncomingMemberships(graph)
	}
	return &mappingQualifiedNameContext{
		packageCache:        mappingFilterPackageNodes(graph),
		incomingMemberships: incoming,
		elementSubjects:     make(map[string]bool),
		membershipElements:  make(map[string]bool),
		qualifiedNameOwners: make(map[string]mappingQualifiedNameOwnerResult),
		scratchElementSeen:  make(map[string]bool),
		scratchOwnerSeen:    make(map[string]bool),
	}
}

func mappingQualifiedNamesWithContext(graph *rdf.Graph, context *mappingQualifiedNameContext) map[string]string {
	return mappingQualifiedNamesWithPositionRule(graph, context, false)
}

func mappingQualifiedNamesWithPositionRule(
	graph *rdf.Graph,
	context *mappingQualifiedNameContext,
	memberPositionsOnly bool,
) map[string]string {
	names := make(map[string]string)
	if graph == nil {
		return names
	}

	candidates := make(map[string]rdf.Term)
	subjectParameters := make(map[string]bool)
	seenElements := context.scratchElementSeen
	for _, subject := range graph.SubjectsView() {
		clear(seenElements)
		isElement := mappingElementSubjectWithContext(graph, subject, seenElements, context)
		context.elementSubjects[subject.Value] = isElement
		if isElement {
			candidates[subject.Value] = subject
		}
		for _, parameter := range graph.ObjectsView(subject, rdf.SysML+pOwnedSubjectParameter) {
			subjectParameters[parameter.Value] = true
		}
	}
	incomingMemberships := context.incomingMemberships

	infos := make(map[string]mappingQualifiedNameInfo, len(candidates))
	siblingsByName := make(map[string][]mappingQualifiedNameSibling)
	seenOwners := context.scratchOwnerSeen
	for _, subject := range candidates {
		clear(seenOwners)
		owner, hasOwner := mappingQualifiedNameOwnerWithContext(
			graph, subject, incomingMemberships, seenOwners, context,
		)
		context.qualifiedNameOwners[subject.Value] = mappingQualifiedNameOwnerResult{
			owner: owner, hasOwner: hasOwner,
		}
		name, hasName := mappingQualifiedNameLocalName(graph, subject, subjectParameters)
		index, hasIndex := 0, false
		if hasOwner {
			if memberPositionsOnly {
				index, hasIndex = mappingOwnedMemberRelationshipPosition(graph, subject, owner)
			} else {
				index, hasIndex = mappingOwnedRelationshipPosition(graph, subject, owner)
			}
		} else if memberIndex, ok := graph.Lexical(subject, rdf.OpenSysML+"memberIndex"); ok {
			if parsed, err := strconv.Atoi(memberIndex); err == nil {
				index, hasIndex = parsed, true
			}
		}
		infos[subject.Value] = mappingQualifiedNameInfo{
			owner: owner, hasOwner: hasOwner, name: name, hasName: hasName, index: index, hasIndex: hasIndex,
		}
		if hasName && hasIndex {
			ownerID := ""
			if hasOwner {
				ownerID = owner.Value
			}
			key := ownerID + "\x00" + name
			siblingsByName[key] = append(siblingsByName[key], mappingQualifiedNameSibling{
				subject: subject, index: index,
			})
		}
	}
	positionalDuplicates := make(map[string]bool)
	for _, siblings := range siblingsByName {
		if len(siblings) < 2 {
			continue
		}
		sort.SliceStable(siblings, func(i, j int) bool { return siblings[i].index < siblings[j].index })
		if siblings[0].index == siblings[1].index {
			continue
		}
		for _, sibling := range siblings[1:] {
			positionalDuplicates[sibling.subject.Value] = true
		}
	}

	resolved := make(map[string]string, len(candidates))
	known := make(map[string]bool, len(candidates))
	visiting := make(map[string]bool, len(candidates))
	var nameOf func(rdf.Term) (string, bool)
	nameOf = func(subject rdf.Term) (string, bool) {
		if known[subject.Value] {
			return resolved[subject.Value], resolved[subject.Value] != ""
		}
		if visiting[subject.Value] {
			return "", false
		}
		visiting[subject.Value] = true
		defer delete(visiting, subject.Value)

		if name, ok := graph.Lexical(subject, rdf.OpenSysML+"mappingQualifiedName"); ok {
			resolved[subject.Value] = name
			known[subject.Value] = true
			return name, name != ""
		}

		info := infos[subject.Value]
		owner, hasOwner := info.owner, info.hasOwner
		ownerName := ""
		if hasOwner {
			if candidate, ok := candidates[owner.Value]; ok {
				owner = candidate
			}
			ownerName, _ = nameOf(owner)
		}
		if !hasOwner && graph.BoolValue(subject, rdf.SysML+"isLibraryElement") {
			known[subject.Value] = true
			return "", false
		}

		name, hasName := info.name, info.hasName
		if positionalDuplicates[subject.Value] {
			name, hasName = "", false
		}
		index, hasIndex := info.index, info.hasIndex
		if !hasName && !hasIndex {
			known[subject.Value] = true
			return "", false
		}

		value := qualify(ownerName, name, index)
		if hasName && mappingTypeConforms(graph, subject, "ConjugatedPortDefinition") &&
			strings.HasPrefix(name, "~") {
			value = strings.TrimSuffix(value, identitySegment(name)) + "'" + name + "'"
		}
		resolved[subject.Value] = value
		known[subject.Value] = true
		return value, true
	}

	for _, subject := range candidates {
		id, ok := rdf.SubjectID(subject)
		if !ok {
			continue
		}
		if name, ok := nameOf(subject); ok {
			names[id] = name
		}
	}
	return names
}

func mappingQualifiedNameOwnerWithContext(
	graph *rdf.Graph,
	subject rdf.Term,
	incomingMemberships map[string][]rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) (rdf.Term, bool) {
	var seenElements map[string]bool
	if context != nil {
		seenElements = context.scratchElementSeen
	} else {
		seenElements = make(map[string]bool)
	}
	isElement := func(subject rdf.Term) bool {
		clear(seenElements)
		return mappingElementSubjectWithContext(graph, subject, seenElements, context)
	}
	for _, property := range mappingRelationshipPredicates {
		if relationship, ok := graph.Object(subject, property); ok && relationship.IsIRI() {
			if isElement(relationship) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithContext(graph, relationship, seen, context); ok {
				return owner, true
			}
		}
	}
	if !graph.BoolValue(subject, rdf.SysML+"isLibraryElement") {
		for _, relationship := range incomingMemberships[subject.Value] {
			if isElement(relationship) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithContext(graph, relationship, seen, context); ok {
				return owner, true
			}
		}
	}
	return mappingQualifiedNameAncestorWithContext(graph, subject, seen, context)
}

func mappingQualifiedNameAncestorWithContext(
	graph *rdf.Graph,
	subject rdf.Term,
	seen map[string]bool,
	context *mappingQualifiedNameContext,
) (rdf.Term, bool) {
	var seenElements map[string]bool
	if context != nil {
		seenElements = context.scratchElementSeen
	} else {
		seenElements = make(map[string]bool)
	}
	isElement := func(subject rdf.Term) bool {
		clear(seenElements)
		return mappingElementSubjectWithContext(graph, subject, seenElements, context)
	}
	if seen[subject.Value] {
		return rdf.Term{}, false
	}
	seen[subject.Value] = true
	for _, property := range mappingOwnerPredicates {
		for _, owner := range graph.ObjectsView(subject, property) {
			if !owner.IsIRI() {
				continue
			}
			if isElement(owner) {
				return owner, true
			}
			if ancestor, ok := mappingQualifiedNameAncestorWithContext(graph, owner, seen, context); ok {
				return ancestor, true
			}
		}
	}
	return rdf.Term{}, false
}

func mappingIncomingMemberships(graph *rdf.Graph) map[string][]rdf.Term {
	incoming := make(map[string][]rdf.Term)
	seenMembers := make(map[string]bool)
	for _, subject := range graph.SubjectsView() {
		if !mappingTypeConforms(graph, subject, "Membership") {
			continue
		}
		clear(seenMembers)
		for _, property := range mappingIncomingMembershipPredicates {
			for _, member := range graph.ObjectsView(subject, property) {
				if member.IsIRI() && !seenMembers[member.Value] {
					seenMembers[member.Value] = true
					incoming[member.Value] = append(incoming[member.Value], subject)
				}
			}
		}
	}
	return incoming
}

type mappingQualifiedNameInfo struct {
	owner    rdf.Term
	hasOwner bool
	name     string
	hasName  bool
	index    int
	hasIndex bool
}

type mappingQualifiedNameSibling struct {
	subject rdf.Term
	index   int
}

func mappingQualifiedNameLocalName(graph *rdf.Graph, subject rdf.Term, subjectParameters map[string]bool) (string, bool) {
	if name, ok := graph.Lexical(subject, rdf.SysML+pDeclaredName); ok {
		return name, true
	}
	if name, ok := graph.Lexical(subject, rdf.SysML+pMemberName); ok {
		return name, true
	}
	if graph.BoolValue(subject, rdf.SysML+"isVariant") {
		if target, ok := graph.Object(subject, rdf.SysML+"references"); ok && target.IsIRI() {
			if name, ok := graph.Lexical(target, rdf.SysML+pDeclaredName); ok {
				return name, true
			}
			if name, ok := graph.Lexical(target, rdf.SysML+pMemberName); ok {
				return name, true
			}
		}
	}
	if subjectParameters[subject.Value] {
		return "_subject", true
	}
	name := mappingRelationshipMemberName(graph, subject)
	return name, name != ""
}

func mappingQualifiedNameOwner(
	graph *rdf.Graph,
	subject rdf.Term,
	incomingMemberships map[string][]rdf.Term,
	seen map[string]bool,
) (rdf.Term, bool) {
	return mappingQualifiedNameOwnerWithCache(graph, subject, incomingMemberships, seen, nil)
}

func mappingQualifiedNameOwnerWithCache(
	graph *rdf.Graph,
	subject rdf.Term,
	incomingMemberships map[string][]rdf.Term,
	seen, packageCache map[string]bool,
) (rdf.Term, bool) {
	for _, property := range mappingRelationshipPredicates {
		if relationship, ok := graph.Object(subject, property); ok && relationship.IsIRI() {
			if mappingElementSubjectWithCache(graph, relationship, make(map[string]bool), packageCache) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithCache(graph, relationship, seen, packageCache); ok {
				return owner, true
			}
		}
	}
	if !graph.BoolValue(subject, rdf.SysML+"isLibraryElement") {
		for _, relationship := range incomingMemberships[subject.Value] {
			if mappingElementSubjectWithCache(graph, relationship, make(map[string]bool), packageCache) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithCache(graph, relationship, seen, packageCache); ok {
				return owner, true
			}
		}
	}
	return mappingQualifiedNameAncestorWithCache(graph, subject, seen, packageCache)
}

func mappingQualifiedNameAncestorWithCache(
	graph *rdf.Graph,
	subject rdf.Term,
	seen, packageCache map[string]bool,
) (rdf.Term, bool) {
	if seen[subject.Value] {
		return rdf.Term{}, false
	}
	seen[subject.Value] = true
	for _, property := range mappingOwnerPredicates {
		for _, owner := range graph.ObjectsView(subject, property) {
			if !owner.IsIRI() {
				continue
			}
			if mappingElementSubjectWithCache(graph, owner, make(map[string]bool), packageCache) {
				return owner, true
			}
			if ancestor, ok := mappingQualifiedNameAncestorWithCache(graph, owner, seen, packageCache); ok {
				return ancestor, true
			}
		}
	}
	return rdf.Term{}, false
}

func mappingRelationshipMemberName(graph *rdf.Graph, subject rdf.Term) string {
	for _, property := range mappingRelationshipPredicates {
		if relationship, ok := graph.Object(subject, property); ok && relationship.IsIRI() {
			if name, ok := graph.Lexical(relationship, rdf.SysML+pMemberName); ok {
				return name
			}
		}
	}
	return ""
}

func mappingOwnedRelationshipPosition(graph *rdf.Graph, subject, owner rdf.Term) (int, bool) {
	relationships := graph.ObjectsView(owner, rdf.SysML+pOwnedRelationship)
	if len(relationships) == 0 {
		relationships = graph.ObjectsView(owner, rdf.SysML+pOwnedMembership)
	}
	owningRelationship, _ := graph.Object(subject, rdf.SysML+pOwningRelationship)
	owningMembership, _ := graph.Object(subject, rdf.SysML+pOwningMembership)
	for index, relationship := range relationships {
		if relationship.Equal(subject) ||
			owningRelationship.IsIRI() && relationship.Equal(owningRelationship) ||
			owningMembership.IsIRI() && relationship.Equal(owningMembership) {
			return index, true
		}
		for _, property := range mappingOwnedRelationshipPredicates {
			for _, member := range graph.ObjectsView(relationship, property) {
				if member.Equal(subject) {
					return index, true
				}
			}
		}
	}
	return 0, false
}

func mappingOwnedMemberRelationshipPosition(graph *rdf.Graph, subject, owner rdf.Term) (int, bool) {
	relationships := graph.ObjectsView(owner, rdf.SysML+pOwnedRelationship)
	if len(relationships) == 0 {
		relationships = graph.ObjectsView(owner, rdf.SysML+pOwnedMembership)
	}
	owningRelationship, _ := graph.Object(subject, rdf.SysML+pOwningRelationship)
	owningMembership, _ := graph.Object(subject, rdf.SysML+pOwningMembership)
	position := 0
	for _, relationship := range relationships {
		if !mappingTypeConforms(graph, owner, "Relationship") &&
			!mappingTypeConforms(graph, relationship, "Membership") &&
			!mappingTypeConforms(graph, relationship, "Import") {
			continue
		}
		if relationship.Equal(subject) ||
			owningRelationship.IsIRI() && relationship.Equal(owningRelationship) ||
			owningMembership.IsIRI() && relationship.Equal(owningMembership) {
			return position, true
		}
		for _, property := range mappingOwnedRelationshipPredicates {
			for _, member := range graph.ObjectsView(relationship, property) {
				if member.Equal(subject) {
					return position, true
				}
			}
		}
		position++
	}
	return 0, false
}

func graphWithoutQualifiedNames(graph *rdf.Graph) *rdf.Graph {
	if graph == nil {
		return rdf.NewGraph()
	}
	triples := make([]rdf.Triple, 0, graph.Len())
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.SysML+pQualifiedName {
			triples = append(triples, triple)
		}
	}
	return rdf.NewGraphOf(triples, graph.Prefixes)
}
