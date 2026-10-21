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

type mappingQualifiedNameContext struct {
	packageCache        map[string]bool
	incomingMemberships map[string][]rdf.Term
	elementSubjects     map[string]bool
	membershipElements  map[string]bool
	qualifiedNameOwners map[string]mappingQualifiedNameOwnerResult
	scratchElementSeen  map[string]bool
	scratchOwnerSeen    map[string]bool
	localNames          map[string]mappingQualifiedNameLocalNameResult
	localNameVisiting   map[string]bool
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
		localNames:          make(map[string]mappingQualifiedNameLocalNameResult),
		localNameVisiting:   make(map[string]bool),
	}
}

func mappingQualifiedNamesWithContext(graph *rdf.Graph, context *mappingQualifiedNameContext) map[string]string {
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
		name, hasName := mappingQualifiedNameLocalName(graph, subject, subjectParameters, context)
		index, hasIndex := 0, false
		if hasOwner {
			index, hasIndex = mappingOwnedRelationshipPosition(graph, subject, owner, context)
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

type mappingQualifiedNameLocalNameResult struct {
	name    string
	hasName bool
}

func mappingQualifiedNameLocalName(
	graph *rdf.Graph,
	subject rdf.Term,
	subjectParameters map[string]bool,
	context *mappingQualifiedNameContext,
) (string, bool) {
	if context != nil {
		if result, ok := context.localNames[subject.Value]; ok {
			return result.name, result.hasName
		}
		if context.localNameVisiting[subject.Value] {
			return "", false
		}
		context.localNameVisiting[subject.Value] = true
		defer delete(context.localNameVisiting, subject.Value)
	}

	name, hasName := mappingQualifiedNameLocalNameUncached(graph, subject, subjectParameters, context)
	if context != nil {
		context.localNames[subject.Value] = mappingQualifiedNameLocalNameResult{name: name, hasName: hasName}
	}
	return name, hasName
}

func mappingQualifiedNameLocalNameUncached(
	graph *rdf.Graph,
	subject rdf.Term,
	subjectParameters map[string]bool,
	context *mappingQualifiedNameContext,
) (string, bool) {
	if name, ok := graph.Lexical(subject, rdf.SysML+pDeclaredName); ok {
		return name, true
	}
	if name, ok := graph.Lexical(subject, rdf.SysML+pMemberName); ok {
		return name, true
	}
	if name, ok := graph.Lexical(subject, rdf.SysML+"name"); ok {
		return name, true
	}
	if graph.BoolValue(subject, rdf.SysML+"isLibraryElement") {
		if name, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok {
			return mappingLastWrittenName(name), true
		}
	}
	if graph.BoolValue(subject, rdf.SysML+"isVariant") {
		if target, ok := graph.Object(subject, rdf.SysML+"references"); ok && target.IsIRI() {
			if name, ok := mappingQualifiedNameLocalName(graph, target, subjectParameters, context); ok {
				return name, true
			}
		}
	}
	if subjectParameters[subject.Value] {
		return "_subject", true
	}
	name := mappingRelationshipMemberName(graph, subject)
	if name != "" {
		return name, true
	}
	return mappingNamingFeatureName(graph, subject, subjectParameters, context)
}

func mappingNamingFeatureName(
	graph *rdf.Graph,
	subject rdf.Term,
	subjectParameters map[string]bool,
	context *mappingQualifiedNameContext,
) (string, bool) {
	if mappingNameIsByReference(graph, subject, context) {
		if target, ok := mappingOwnedReferenceTarget(graph, subject); ok {
			return mappingNamingTargetName(graph, target, subjectParameters, context, true)
		}
		if mappingReferenceNameHasNoRedefinition(graph, subject, context) {
			return "", false
		}
	}

	for _, relationship := range mappingOwnedRelationships(graph, subject) {
		if !mappingTypeConforms(graph, relationship, "Redefinition") {
			continue
		}
		target, ok := mappingRelationshipTarget(
			graph, relationship, "redefinedFeature", "subsettedFeature", "general", "target",
		)
		if !ok || mappingFeatureChain(graph, target) ||
			mappingTypeConforms(graph, target, "FeatureChainExpression") {
			return "", false
		}
		return mappingNamingTargetName(graph, target, subjectParameters, context, false)
	}
	return "", false
}

func mappingNameIsByReference(
	graph *rdf.Graph,
	subject rdf.Term,
	context *mappingQualifiedNameContext,
) bool {
	for _, metaclass := range []string{
		"PerformActionUsage", "ExhibitStateUsage", "IncludeUseCaseUsage", "RenderingUsage",
	} {
		if mappingTypeConforms(graph, subject, metaclass) {
			return true
		}
	}
	if graph.BoolValue(subject, rdf.SysML+"isVariant") {
		return true
	}
	memberships := graph.ObjectsView(subject, rdf.SysML+pOwningMembership)
	if context != nil {
		memberships = append(memberships, context.incomingMemberships[subject.Value]...)
	}
	for _, membership := range memberships {
		if mappingTypeConforms(graph, membership, "VariantMembership") ||
			mappingTypeConforms(graph, membership, "RequirementConstraintMembership") ||
			mappingTypeConforms(graph, membership, "RequirementVerificationMembership") ||
			mappingTypeConforms(graph, membership, "StateSubactionMembership") ||
			mappingTypeConforms(graph, membership, "ViewRenderingMembership") ||
			mappingTypeConforms(graph, membership, "FramedConcernMembership") {
			return true
		}
	}
	return false
}

func mappingReferenceNameHasNoRedefinition(
	graph *rdf.Graph,
	subject rdf.Term,
	context *mappingQualifiedNameContext,
) bool {
	memberships := graph.ObjectsView(subject, rdf.SysML+pOwningMembership)
	if context != nil {
		memberships = append(memberships, context.incomingMemberships[subject.Value]...)
	}
	for _, membership := range memberships {
		if mappingTypeConforms(graph, membership, "RequirementConstraintMembership") ||
			mappingTypeConforms(graph, membership, "RequirementVerificationMembership") {
			return true
		}
	}
	return false
}

func mappingOwnedReferenceTarget(graph *rdf.Graph, subject rdf.Term) (rdf.Term, bool) {
	relationships := graph.ObjectsView(subject, rdf.SysML+pOwnedReferenceSubsetting)
	if len(relationships) == 0 {
		for _, relationship := range mappingOwnedRelationships(graph, subject) {
			if mappingTypeConforms(graph, relationship, "ReferenceSubsetting") {
				relationships = append(relationships, relationship)
			}
		}
	}
	for _, relationship := range relationships {
		if target, ok := mappingRelationshipTarget(
			graph, relationship, "referencedFeature", "subsettedFeature", "general", "target",
		); ok {
			return target, true
		}
	}
	return rdf.Term{}, false
}

func mappingOwnedRelationships(graph *rdf.Graph, subject rdf.Term) []rdf.Term {
	relationships := graph.ObjectsView(subject, rdf.SysML+pOwnedRelationship)
	if len(relationships) > 0 {
		return relationships
	}
	if relationships = graph.ObjectsView(subject, rdf.SysML+"ownedRedefinition"); len(relationships) > 0 {
		return relationships
	}
	return graph.ObjectsView(subject, rdf.SysML+pOwnedMembership)
}

func mappingRelationshipTarget(graph *rdf.Graph, relationship rdf.Term, properties ...string) (rdf.Term, bool) {
	for _, property := range properties {
		if target, ok := graph.Object(relationship, rdf.SysML+property); ok {
			return target, true
		}
	}
	return rdf.Term{}, false
}

func mappingNamingTargetName(
	graph *rdf.Graph,
	target rdf.Term,
	subjectParameters map[string]bool,
	context *mappingQualifiedNameContext,
	allowFeatureChain bool,
) (string, bool) {
	if !target.IsIRI() {
		return mappingLastWrittenName(target.Value), target.Value != ""
	}
	if allowFeatureChain && (mappingFeatureChain(graph, target) ||
		mappingTypeConforms(graph, target, "FeatureChainExpression")) {
		links := graph.ObjectsView(target, rdf.SysML+pChainingFeature)
		if len(links) == 0 {
			for _, relationship := range mappingOwnedRelationships(graph, target) {
				if mappingTypeConforms(graph, relationship, "FeatureChaining") {
					if link, ok := mappingRelationshipTarget(
						graph, relationship, "chainingFeature", "target", "general",
					); ok {
						links = append(links, link)
					}
				}
			}
		}
		if len(links) == 0 {
			return "", false
		}
		target = links[len(links)-1]
		if !target.IsIRI() {
			return mappingLastWrittenName(target.Value), target.Value != ""
		}
	}
	return mappingQualifiedNameLocalName(graph, target, subjectParameters, context)
}

func mappingLastWrittenName(name string) string {
	if name == "" {
		return ""
	}
	quoted := false
	last, width := -1, 0
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '\'':
			quoted = !quoted
		case '.':
			if !quoted {
				last, width = i, 1
			}
		case ':':
			if !quoted && i+1 < len(name) && name[i+1] == ':' {
				last, width = i, 2
				i++
			}
		}
	}
	if last >= 0 {
		name = name[last+width:]
	}
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = name[1 : len(name)-1]
	}
	return name
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

func mappingOwnedRelationshipPosition(
	graph *rdf.Graph,
	subject, owner rdf.Term,
	context *mappingQualifiedNameContext,
) (int, bool) {
	relationships := graph.ObjectsView(owner, rdf.SysML+pOwnedRelationship)
	if len(relationships) == 0 {
		relationships = graph.ObjectsView(owner, rdf.SysML+pOwnedMembership)
	}
	owningRelationship, _ := graph.Object(subject, rdf.SysML+pOwningRelationship)
	owningMembership, _ := graph.Object(subject, rdf.SysML+pOwningMembership)
	position := 0
	for _, relationship := range relationships {
		if !mappingRelationshipContributesMemberPosition(graph, relationship, context) {
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

// collect indexes kept source members; see rdf_out.go:969–992 and 1209–1217.
func mappingRelationshipContributesMemberPosition(
	graph *rdf.Graph,
	relationship rdf.Term,
	context *mappingQualifiedNameContext,
) bool {
	if mappingTypeConforms(graph, relationship, "Membership") ||
		mappingTypeConforms(graph, relationship, "Import") {
		return true
	}
	if mappingPositionElementSubject(graph, relationship, context) {
		return true
	}
	if !mappingTypeConforms(graph, relationship, "Annotation") {
		return false
	}
	for _, property := range []string{
		rdf.SysML + pOwnedRelatedElement,
		rdf.SysML + pAnnotatingElement,
	} {
		for _, member := range graph.ObjectsView(relationship, property) {
			if mappingPositionElementSubject(graph, member, context) {
				return true
			}
		}
	}
	return false
}

func mappingPositionElementSubject(
	graph *rdf.Graph,
	subject rdf.Term,
	context *mappingQualifiedNameContext,
) bool {
	if context == nil {
		return MappingElementSubject(graph, subject)
	}
	clear(context.scratchElementSeen)
	return mappingElementSubjectWithContext(graph, subject, context.scratchElementSeen, context)
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
