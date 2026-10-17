package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

type apiJSONNameIndex struct {
	declared      map[string][]rdf.Term
	qualified     map[string][]rdf.Term
	chainTargets  map[string]rdf.Term
	chainOwners   map[string][]rdf.Term
	subjectsByID  map[string]rdf.Term
	expressionIDs map[string]bool
	catalog       *identity.Catalog
}

func newAPIJSONNameIndex(graph *rdf.Graph) *apiJSONNameIndex {
	index := &apiJSONNameIndex{
		declared:      make(map[string][]rdf.Term),
		qualified:     make(map[string][]rdf.Term),
		catalog:       identity.LibraryCatalogWithOpenSysML(libs.SharedBase()),
		subjectsByID:  make(map[string]rdf.Term),
		expressionIDs: make(map[string]bool),
	}
	for _, subject := range graph.SubjectsView() {
		id, ok := rdf.SubjectID(subject)
		if !ok {
			continue
		}
		index.subjectsByID[id] = subject
		if strings.HasPrefix(subject.Value, rdf.Expression) {
			index.expressionIDs[id] = true
		}
		if name, ok := graph.Lexical(subject, rdf.SysML+pDeclaredName); ok {
			index.declared[name] = append(index.declared[name], subject)
		}
		if name, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok {
			index.qualified[name] = append(index.qualified[name], subject)
		}
	}
	index.chainOwners = apiJSONChainOwnerIndex(graph)
	index.chainTargets = apiJSONFeatureChainTargets(graph, index)
	return index
}

func apiJSONChainOwnerIndex(graph *rdf.Graph) map[string][]rdf.Term {
	owners := make(map[string][]rdf.Term)
	for _, relationship := range graph.SubjectsView() {
		if rdf.LocalName(graph.Type(relationship)) != mFeatureChaining {
			continue
		}
		owner := firstIRI(graph, relationship, pOwningRelatedElement, pOwner)
		if owner.Value != "" {
			owners[owner.Value] = append(owners[owner.Value], relationship)
		}
	}
	return owners
}

func apiJSONFeatureChainTargets(graph *rdf.Graph, index *apiJSONNameIndex) map[string]rdf.Term {
	resolved := make(map[string]rdf.Term)
	for ownerID, relationships := range index.chainOwners {
		owner := rdf.IRI(ownerID)
		ordered := make([]rdf.Term, 0, len(relationships))
		seen := make(map[string]bool, len(relationships))
		for _, relationship := range graph.ObjectsView(owner, rdf.SysML+pOwnedRelationship) {
			if rdf.LocalName(graph.Type(relationship)) == mFeatureChaining {
				ordered = append(ordered, relationship)
				seen[relationship.Value] = true
			}
		}
		for _, relationship := range relationships {
			if !seen[relationship.Value] {
				ordered = append(ordered, relationship)
			}
		}
		var previous rdf.Term
		for _, relationship := range ordered {
			targets := graph.ObjectsView(relationship, rdf.SysML+pChainingFeature)
			if len(targets) != 1 {
				previous = rdf.Term{}
				continue
			}
			target := targets[0]
			if target.IsIRI() {
				resolved[relationship.Value] = target
				previous = target
				continue
			}
			if !target.IsLiteral() {
				previous = rdf.Term{}
				continue
			}
			if previous.Value == "" {
				previous = apiJSONGraphResolveName(graph, index, owner, target.Value)
			} else {
				feature := previous
				previous = rdf.Term{}
				if featureType := apiJSONFeatureType(graph, feature); featureType.Value != "" {
					previous = apiJSONGraphNamedTargetInType(graph, index, featureType, target.Value)
				}
			}
			if previous.Value != "" {
				resolved[relationship.Value] = previous
			}
		}
	}
	return resolved
}

func apiJSONResolveNameReference(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	key string,
	value any,
) (any, []string) {
	if key == pChainingFeature {
		if members, ok := value.([]any); ok {
			return apiJSONResolveFeatureChain(graph, index, subject, members)
		}
	}
	if rdf.LocalName(graph.Type(subject)) == mFeatureChaining {
		switch key {
		case pTarget:
			if name, ok := value.(apiJSONNameReference); ok {
				target := index.chainTargets[subject.Value]
				if id, resolved := apiJSONTermReferenceID(subject, target); resolved {
					return apiJSONReference{ID: id}, nil
				}
				return name, []string{name.name}
			}
		case pRelatedElement:
			if members, ok := value.([]any); ok {
				target := index.chainTargets[subject.Value]
				id, resolved := apiJSONTermReferenceID(subject, target)
				result := make([]any, len(members))
				var unresolved []string
				for i, member := range members {
					if name, ok := member.(apiJSONNameReference); ok {
						if resolved {
							result[i] = apiJSONReference{ID: id}
						} else {
							result[i] = name
							unresolved = append(unresolved, name.name)
						}
						continue
					}
					result[i] = member
				}
				return result, unresolved
			}
		}
	}
	switch value := value.(type) {
	case apiJSONNameReference:
		target := rdf.Term{}
		if key == pChainingFeature && rdf.LocalName(graph.Type(subject)) == mFeatureChaining {
			target = index.chainTargets[subject.Value]
		} else {
			target = apiJSONGraphResolveName(graph, index, subject, value.name)
		}
		if target.Value == "" &&
			!(key == pChainingFeature && rdf.LocalName(graph.Type(subject)) == mFeatureChaining) {
			target = apiJSONCatalogResolveName(graph, index, subject, value.name)
		}
		id, ok := apiJSONTermReferenceID(subject, target)
		if !ok {
			return value, []string{value.name}
		}
		return apiJSONReference{ID: id}, nil
	case []any:
		resolved := make([]any, len(value))
		var unresolved []string
		for i, member := range value {
			encoded, names := apiJSONResolveNameReference(graph, index, subject, key, member)
			resolved[i] = encoded
			unresolved = append(unresolved, names...)
		}
		return resolved, unresolved
	default:
		return value, nil
	}
}

func apiJSONResolveFeatureChain(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	members []any,
) ([]any, []string) {
	resolved := make([]any, len(members))
	var previous rdf.Term
	var unresolved []string
	for i, member := range members {
		switch value := member.(type) {
		case string:
			member = apiJSONNameReference{name: value}
		}
		if reference, ok := member.(apiJSONReference); ok {
			resolved[i] = reference
			previous = rdf.ReferenceIRI(subject, reference.ID)
			continue
		}
		name, isName := member.(apiJSONNameReference)
		if !isName {
			resolved[i] = member
			continue
		}
		target := rdf.Term{}
		if previous.Value == "" {
			target = apiJSONGraphResolveName(graph, index, subject, name.name)
		} else if featureType := apiJSONFeatureType(graph, previous); featureType.Value != "" {
			target = apiJSONGraphNamedTargetInType(graph, index, featureType, name.name)
		}
		if target.Value == "" {
			target = apiJSONCatalogResolveName(graph, index, subject, name.name)
		}
		id, ok := apiJSONTermReferenceID(subject, target)
		if !ok {
			resolved[i] = member
			unresolved = append(unresolved, name.name)
			previous = rdf.Term{}
			continue
		}
		resolved[i] = apiJSONReference{ID: id}
		previous = target
	}
	return resolved, unresolved
}

func apiJSONFeatureType(graph *rdf.Graph, feature rdf.Term) rdf.Term {
	for _, relationship := range graph.ObjectsView(feature, rdf.SysML+pOwnedRelationship) {
		if rdf.LocalName(graph.Type(relationship)) != mFeatureTyping {
			continue
		}
		for _, property := range []string{"type", pGeneral, pTarget} {
			if target, ok := graph.Object(relationship, rdf.SysML+property); ok && target.IsIRI() {
				return target
			}
		}
	}
	if target, ok := graph.Object(feature, rdf.SysML+"type"); ok && target.IsIRI() {
		return target
	}
	return rdf.Term{}
}

func apiJSONGraphResolveName(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	name string,
) rdf.Term {
	var scopes []rdf.Term
	seen := make(map[string]bool)
	for scope := subject; scope.Value != "" && !seen[scope.Value]; scope = apiJSONNameOwner(graph, scope) {
		seen[scope.Value] = true
		scopes = append(scopes, scope)
	}
	for _, scope := range scopes {
		matches := apiJSONGraphOwnedNames(graph, index, scope, name)
		if len(matches) == 1 {
			return matches[0]
		}
		if len(matches) > 1 {
			return rdf.Term{}
		}
	}
	for _, scope := range scopes {
		matches := apiJSONImportedNames(graph, index, scope, name)
		if len(matches) == 1 {
			return matches[0]
		}
		if len(matches) > 1 {
			return rdf.Term{}
		}
	}
	return rdf.Term{}
}

func apiJSONImportedNames(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	scope rdf.Term,
	name string,
) []rdf.Term {
	var matches []rdf.Term
	add := func(target rdf.Term) {
		if !target.IsIRI() {
			return
		}
		for _, match := range matches {
			if match == target {
				return
			}
		}
		matches = append(matches, target)
	}
	relations := append(graph.ObjectsView(scope, rdf.SysML+pOwnedRelationship),
		graph.ObjectsView(scope, rdf.SysML+pOwnedImport)...)
	seen := make(map[string]bool, len(relations))
	for _, relation := range relations {
		if seen[relation.Value] {
			continue
		}
		seen[relation.Value] = true
		switch rdf.LocalName(graph.Type(relation)) {
		case mNamespaceImport, "NamespaceExpose":
			namespace, ok := graph.Object(relation, rdf.SysML+pImportedNamespace)
			if !ok || !namespace.IsIRI() {
				continue
			}
			for _, target := range apiJSONGraphOwnedNames(graph, index, namespace, name) {
				add(target)
			}
			if index.catalog == nil {
				continue
			}
			root, ok := index.catalog.Element(rdf.LocalName(namespace.Value))
			if !ok || root.FQN == "" {
				continue
			}
			recursive := graph.BoolValue(relation, rdf.SysML+pIsRecursive) ||
				graph.BoolValue(relation, rdf.OpenSysML+pIsRecursive)
			for _, element := range index.catalog.Elements() {
				if !strings.HasPrefix(element.FQN, root.FQN+"::") {
					continue
				}
				relative := strings.TrimPrefix(element.FQN, root.FQN+"::")
				if name == element.FQN ||
					((recursive || !strings.Contains(relative, "::")) && name == relative) {
					add(rdf.ElementIRIForID(element.ID))
				}
			}
		case "MembershipImport", "MembershipExpose":
			membership, ok := graph.Object(relation, rdf.SysML+pImportedMembership)
			if !ok || !membership.IsIRI() {
				continue
			}
			alias, hasAlias := graph.Lexical(relation, rdf.SysML+pDeclaredName)
			if hasAlias && alias == name && index.catalog != nil {
				if element, ok := index.catalog.OwningMembership(rdf.LocalName(membership.Value)); ok {
					add(rdf.ElementIRIForID(element.ID))
				}
				continue
			}
			target, ok := graph.Object(membership, rdf.SysML+pMemberElement)
			if ok && target.IsIRI() && apiJSONNameMatches(graph, target, name) {
				add(target)
				continue
			}
			if index.catalog != nil {
				element, ok := index.catalog.OwningMembership(rdf.LocalName(membership.Value))
				if !ok {
					continue
				}
				if name == element.FQN ||
					(element.Symbol != nil && name == element.Symbol.Name) {
					add(rdf.ElementIRIForID(element.ID))
				}
			}
		}
	}
	return matches
}

func apiJSONNameMatches(graph *rdf.Graph, subject rdf.Term, name string) bool {
	if value, ok := graph.Lexical(subject, rdf.SysML+pDeclaredName); ok && value == name {
		return true
	}
	if value, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok && value == name {
		return true
	}
	return false
}

func apiJSONGraphOwnedNames(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	owner rdf.Term,
	name string,
) []rdf.Term {
	candidates := append(index.declared[name], index.qualified[name]...)
	var matches []rdf.Term
	for _, candidate := range candidates {
		if apiJSONNameOwner(graph, candidate) != owner {
			continue
		}
		found := false
		for _, prior := range matches {
			if prior == candidate {
				found = true
				break
			}
		}
		if !found {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func apiJSONGraphNamedTarget(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	scope rdf.Term,
	name string,
) rdf.Term {
	matches := apiJSONGraphNamedTargets(graph, index, scope, name)
	if len(matches) == 1 {
		return matches[0]
	}
	return rdf.Term{}
}

func apiJSONGraphNamedTargetInType(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	typ rdf.Term,
	name string,
) rdf.Term {
	frontier := []rdf.Term{typ}
	seen := make(map[string]bool)
	for len(frontier) > 0 {
		var matches []rdf.Term
		var next []rdf.Term
		for _, scope := range frontier {
			if seen[scope.Value] {
				continue
			}
			seen[scope.Value] = true
			for _, target := range apiJSONGraphNamedTargets(graph, index, scope, name) {
				matches = appendUniqueTerm(matches, target)
			}
			for _, parent := range apiJSONGraphSuperTypes(graph, scope) {
				if !seen[parent.Value] {
					next = appendUniqueTerm(next, parent)
				}
			}
		}
		if len(matches) == 1 {
			return matches[0]
		}
		if len(matches) > 1 {
			return rdf.Term{}
		}
		frontier = next
	}
	return rdf.Term{}
}

func apiJSONGraphNamedTargets(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	scope rdf.Term,
	name string,
) []rdf.Term {
	matches := apiJSONGraphOwnedNames(graph, index, scope, name)
	if len(matches) > 0 {
		return matches
	}
	return apiJSONImportedNames(graph, index, scope, name)
}

func apiJSONGraphSuperTypes(graph *rdf.Graph, typ rdf.Term) []rdf.Term {
	var parents []rdf.Term
	add := func(target rdf.Term) {
		if target.IsIRI() {
			parents = appendUniqueTerm(parents, target)
		}
	}
	for _, property := range []string{pGeneral, "superclassifier"} {
		for _, target := range graph.ObjectsView(typ, rdf.SysML+property) {
			add(target)
		}
	}
	for _, relationship := range graph.ObjectsView(typ, rdf.SysML+pOwnedRelationship) {
		if !ontology.IsAncestorOrSelf(rdf.LocalName(graph.Type(relationship)), mSpecialization) {
			continue
		}
		for _, property := range []string{pGeneral, "superclassifier"} {
			for _, target := range graph.ObjectsView(relationship, rdf.SysML+property) {
				add(target)
			}
		}
	}
	return parents
}

func appendUniqueTerm(terms []rdf.Term, candidate rdf.Term) []rdf.Term {
	for _, term := range terms {
		if term == candidate {
			return terms
		}
	}
	return append(terms, candidate)
}

func apiJSONNameOwner(graph *rdf.Graph, subject rdf.Term) rdf.Term {
	for _, property := range []string{
		pOwner, pOwningRelatedElement, pMembershipOwningNamespace, pOwningNamespace, pImportOwningNamespace,
	} {
		if owner, ok := graph.Object(subject, rdf.SysML+property); ok && owner.IsIRI() {
			return owner
		}
	}
	for _, relationshipProperty := range []string{pOwningMembership, pOwningRelationship} {
		if relationship, ok := graph.Object(subject, rdf.SysML+relationshipProperty); ok && relationship.IsIRI() {
			for _, property := range []string{
				pMembershipOwningNamespace, pOwningRelatedElement, pOwningNamespace, pOwner,
			} {
				if owner, ok := graph.Object(relationship, rdf.SysML+property); ok && owner.IsIRI() {
					return owner
				}
			}
		}
	}
	return rdf.Term{}
}

func apiJSONCatalogResolveName(
	_ *rdf.Graph,
	index *apiJSONNameIndex,
	_ rdf.Term,
	name string,
) rdf.Term {
	if index.catalog == nil || name == "" {
		return rdf.Term{}
	}
	var matches []rdf.Term
	add := func(element *identity.LibraryElement) {
		if element == nil || element.ID == "" {
			return
		}
		target := rdf.ElementIRIForID(element.ID)
		for _, prior := range matches {
			if prior == target {
				return
			}
		}
		matches = append(matches, target)
	}
	if element, ok := index.catalog.ElementNamed(name); ok {
		add(element)
	}
	if !strings.Contains(name, "::") {
		for _, element := range index.catalog.Elements() {
			if element.Symbol != nil && element.Symbol.Name == name {
				add(element)
			}
		}
	}
	if len(matches) != 1 {
		return rdf.Term{}
	}
	return matches[0]
}

func apiJSONTermReferenceID(subject, target rdf.Term) (string, bool) {
	if !subject.IsIRI() || subject.Value == "" || !target.IsIRI() || target.Value == "" {
		return "", false
	}
	return rdf.ReferenceID(subject, target), true
}
