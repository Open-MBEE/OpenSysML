package export

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	return mappingQualifiedNamesWithContext(graph, newMappingQualifiedNameContext(graph, newAPIJSONGraphIndex(graph, nil)))
}

type mappingQualifiedNameContext struct {
	index               *apiJSONGraphIndex
	packageCache        map[string]bool
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

func newMappingQualifiedNameContext(graph *rdf.Graph, index *apiJSONGraphIndex) *mappingQualifiedNameContext {
	return &mappingQualifiedNameContext{
		index:               index,
		packageCache:        mappingFilterPackageNodes(graph, index),
		elementSubjects:     make(map[string]bool),
		membershipElements:  make(map[string]bool),
		qualifiedNameOwners: make(map[string]mappingQualifiedNameOwnerResult),
		scratchElementSeen:  make(map[string]bool),
		scratchOwnerSeen:    make(map[string]bool),
		localNames:          make(map[string]mappingQualifiedNameLocalNameResult),
		localNameVisiting:   make(map[string]bool),
	}
}

// conforms is mappingTypeConforms, through the shared index when there is one.
func (context *mappingQualifiedNameContext) conforms(graph *rdf.Graph, subject rdf.Term, ancestor string) bool {
	if context == nil {
		return mappingTypeConforms(graph, subject, ancestor)
	}
	return context.index.conforms(graph, subject, ancestor)
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
	incomingMemberships := context.index.incomingMemberships

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
		if hasName && context.conforms(graph, subject, "ConjugatedPortDefinition") &&
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
		if !context.conforms(graph, relationship, "Redefinition") {
			continue
		}
		target, ok := mappingRelationshipTarget(
			graph, relationship, "redefinedFeature", "subsettedFeature", "general", "target",
		)
		if !ok || mappingFeatureChain(graph, target, context) ||
			context.conforms(graph, target, "FeatureChainExpression") {
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
		if context.conforms(graph, subject, metaclass) {
			return true
		}
	}
	if graph.BoolValue(subject, rdf.SysML+"isVariant") {
		return true
	}
	memberships := graph.ObjectsView(subject, rdf.SysML+pOwningMembership)
	if context != nil {
		memberships = append(memberships, context.index.incomingMemberships[subject.Value]...)
	}
	for _, membership := range memberships {
		if context.conforms(graph, membership, "VariantMembership") ||
			context.conforms(graph, membership, "RequirementConstraintMembership") ||
			context.conforms(graph, membership, "RequirementVerificationMembership") ||
			context.conforms(graph, membership, "StateSubactionMembership") ||
			context.conforms(graph, membership, "ViewRenderingMembership") ||
			context.conforms(graph, membership, "FramedConcernMembership") {
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
		memberships = append(memberships, context.index.incomingMemberships[subject.Value]...)
	}
	for _, membership := range memberships {
		if context.conforms(graph, membership, "RequirementConstraintMembership") ||
			context.conforms(graph, membership, "RequirementVerificationMembership") {
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
	var relationships []rdf.Term
	seen := make(map[rdf.Term]struct{})
	for _, property := range []string{pOwnedRelationship, "ownedRedefinition", pOwnedMembership} {
		for _, relationship := range graph.ObjectsView(subject, rdf.SysML+property) {
			if _, ok := seen[relationship]; ok {
				continue
			}
			seen[relationship] = struct{}{}
			relationships = append(relationships, relationship)
		}
	}
	return relationships
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
	if allowFeatureChain && (mappingFeatureChain(graph, target, context) ||
		context.conforms(graph, target, "FeatureChainExpression")) {
		links := graph.ObjectsView(target, rdf.SysML+pChainingFeature)
		if len(links) == 0 {
			for _, relationship := range mappingOwnedRelationships(graph, target) {
				if context.conforms(graph, relationship, "FeatureChaining") {
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
	endName := source.ReferenceEndNames(name)
	if segments, ok := source.MemberPathSegments(endName); ok && len(segments) > 0 {
		segment := segments[len(segments)-1]
		if len(endName) > 0 && endName[0] == '\'' {
			return source.Unescape(segment)
		}
		return segment
	}
	qualified := identitySegments(name)
	if len(qualified) == 0 {
		return ""
	}
	segment := qualified[len(qualified)-1]
	if segments, ok := source.MemberPathSegments(segment); ok && len(segments) > 0 {
		segment = segments[len(segments)-1]
		if len(qualified[len(qualified)-1]) > 0 && qualified[len(qualified)-1][0] == '\'' {
			return source.Unescape(segment)
		}
	}
	return identityName(segment)
}

func mappingQualifiedNameOwner(
	graph *rdf.Graph,
	subject rdf.Term,
	incomingMemberships map[string][]rdf.Term,
	seen map[string]bool,
) (rdf.Term, bool) {
	return mappingQualifiedNameOwnerWithCache(graph, subject, incomingMemberships, seen)
}

func mappingQualifiedNameOwnerWithCache(
	graph *rdf.Graph,
	subject rdf.Term,
	incomingMemberships map[string][]rdf.Term,
	seen map[string]bool,
) (rdf.Term, bool) {
	for _, property := range mappingRelationshipPredicates {
		if relationship, ok := graph.Object(subject, property); ok && relationship.IsIRI() {
			if mappingElementSubjectWithContext(graph, relationship, make(map[string]bool), nil) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithCache(graph, relationship, seen); ok {
				return owner, true
			}
		}
	}
	if !graph.BoolValue(subject, rdf.SysML+"isLibraryElement") {
		for _, relationship := range incomingMemberships[subject.Value] {
			if mappingElementSubjectWithContext(graph, relationship, make(map[string]bool), nil) {
				return relationship, true
			}
			if owner, ok := mappingQualifiedNameAncestorWithCache(graph, relationship, seen); ok {
				return owner, true
			}
		}
	}
	return mappingQualifiedNameAncestorWithCache(graph, subject, seen)
}

func mappingQualifiedNameAncestorWithCache(
	graph *rdf.Graph,
	subject rdf.Term,
	seen map[string]bool,
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
			if mappingElementSubjectWithContext(graph, owner, make(map[string]bool), nil) {
				return owner, true
			}
			if ancestor, ok := mappingQualifiedNameAncestorWithCache(graph, owner, seen); ok {
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
	positions := context.index.ownerPositions[owner.Value]
	if positions == nil {
		positions = mappingOwnedRelationshipPositions(graph, owner, context)
		context.index.ownerPositions[owner.Value] = positions
	}
	position, found := 0, false
	consider := func(index map[rdf.Term]int, term rdf.Term) {
		if p, ok := index[term]; ok && (!found || p < position) {
			position, found = p, true
		}
	}
	consider(positions.relationships, subject)
	consider(positions.members, subject)
	if owningRelationship, _ := graph.Object(subject, rdf.SysML+pOwningRelationship); owningRelationship.IsIRI() {
		consider(positions.relationships, owningRelationship)
	}
	if owningMembership, _ := graph.Object(subject, rdf.SysML+pOwningMembership); owningMembership.IsIRI() {
		consider(positions.relationships, owningMembership)
	}
	return position, found
}

func mappingOwnedRelationshipPositions(
	graph *rdf.Graph,
	owner rdf.Term,
	context *mappingQualifiedNameContext,
) *mappingOwnerPositions {
	relationships := graph.ObjectsView(owner, rdf.SysML+pOwnedRelationship)
	if len(relationships) == 0 {
		relationships = graph.ObjectsView(owner, rdf.SysML+pOwnedMembership)
	}
	positions := &mappingOwnerPositions{
		relationships: make(map[rdf.Term]int, len(relationships)),
		members:       make(map[rdf.Term]int, len(relationships)),
	}
	position := 0
	for _, relationship := range relationships {
		if !mappingRelationshipContributesMemberPosition(graph, relationship, context) {
			continue
		}
		if _, ok := positions.relationships[relationship]; !ok {
			positions.relationships[relationship] = position
		}
		for _, property := range mappingOwnedRelationshipPredicates {
			for _, member := range graph.ObjectsView(relationship, property) {
				if _, ok := positions.members[member]; !ok {
					positions.members[member] = position
				}
			}
		}
		position++
	}
	return positions
}

// collect indexes kept source members; see rdf_out.go:969–992 and 1209–1217.
func mappingRelationshipContributesMemberPosition(
	graph *rdf.Graph,
	relationship rdf.Term,
	context *mappingQualifiedNameContext,
) bool {
	if context.conforms(graph, relationship, "Membership") ||
		context.conforms(graph, relationship, "Import") {
		return true
	}
	if mappingPositionElementSubject(graph, relationship, context) {
		return true
	}
	if !context.conforms(graph, relationship, "Annotation") {
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
