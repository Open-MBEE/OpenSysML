package export

import (
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

type apiJSONNameIndex struct {
	subjectSlots        map[string]int
	subjects            []rdf.Term
	chainTargets        map[string]apiJSONChainTarget
	chainTargetsBySlot  []apiJSONChainTarget
	chainTargetSet      []bool
	chainOwners         [][]rdf.Term
	externalChainOwners map[string][]rdf.Term
	catalogTypeMu       sync.Mutex
	catalogModel        *semantics.Model
	catalogTypes        map[string][]rdf.Term
	subjectsByID        map[string]rdf.Term
	expressionIDs       map[string]bool
	catalog             *identity.Catalog
	catalogNames        *apiJSONCatalogNameIndex
	owners              []rdf.Term
	externalOwners      map[string]rdf.Term
	scopes              [][]rdf.Term
	scopeSet            []bool
	externalScopes      map[string][]rdf.Term
	ownedNames          map[apiJSONScopeName][]rdf.Term
	imports             [][]rdf.Term
	importedNames       map[apiJSONScopeName][]rdf.Term
}

type apiJSONChainTarget struct {
	reference apiJSONReference
	target    rdf.Term
}

type apiJSONScopeName struct {
	scope string
	name  string
}

type apiJSONCatalogNameIndex struct {
	byShortName       map[string][]rdf.Term
	importedDirect    map[string]map[string][]rdf.Term
	importedRecursive map[string]map[string][]rdf.Term
	importedQualified map[string]map[string][]rdf.Term
}

var apiJSONCatalogNameIndexes sync.Map

func newAPIJSONNameIndex(graph *rdf.Graph) *apiJSONNameIndex {
	return newAPIJSONNameIndexWithSubjectSlots(graph, nil)
}

func newAPIJSONNameIndexWithSubjectSlots(graph *rdf.Graph, subjectSlots map[string]int) *apiJSONNameIndex {
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	var subjects []rdf.Term
	if graph != nil {
		subjects = graph.SubjectsView()
	}
	if subjectSlots == nil {
		subjectSlots = make(map[string]int, len(subjects))
		for position, subject := range subjects {
			subjectSlots[subject.Value] = position
		}
	}
	index := &apiJSONNameIndex{
		subjectSlots:       subjectSlots,
		subjects:           subjects,
		chainTargets:       make(map[string]apiJSONChainTarget),
		chainTargetsBySlot: make([]apiJSONChainTarget, len(subjects)),
		chainTargetSet:     make([]bool, len(subjects)),
		chainOwners:        make([][]rdf.Term, len(subjects)),
		catalog:            catalog,
		catalogNames:       apiJSONCatalogNames(catalog),
		catalogTypes:       make(map[string][]rdf.Term),
		subjectsByID:       make(map[string]rdf.Term),
		expressionIDs:      make(map[string]bool),
		owners:             make([]rdf.Term, len(subjects)),
		scopes:             make([][]rdf.Term, len(subjects)),
		scopeSet:           make([]bool, len(subjects)),
		ownedNames:         make(map[apiJSONScopeName][]rdf.Term),
		imports:            make([][]rdf.Term, len(subjects)),
		importedNames:      make(map[apiJSONScopeName][]rdf.Term),
	}
	for _, subject := range graph.SubjectsView() {
		position := subjectSlots[subject.Value]
		index.owners[position] = apiJSONNameOwner(graph, subject)
		index.imports[position] = append(graph.ObjectsView(subject, rdf.SysML+pOwnedRelationship),
			graph.ObjectsView(subject, rdf.SysML+pOwnedImport)...)
		id, ok := rdf.SubjectID(subject)
		if !ok {
			continue
		}
		index.subjectsByID[id] = subject
		if strings.HasPrefix(subject.Value, rdf.Expression) {
			index.expressionIDs[id] = true
		}
		if name, ok := graph.Lexical(subject, rdf.SysML+pDeclaredName); ok {
			index.addOwnedName(subject, name)
		}
		if name, ok := graph.Lexical(subject, rdf.SysML+pDeclaredShortName); ok {
			index.addOwnedName(subject, name)
		}
		if name, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok {
			index.addOwnedName(subject, name)
		}
	}
	for _, membership := range graph.SubjectsView() {
		name, ok := graph.Lexical(membership, rdf.SysML+pMemberShortName)
		if !ok {
			continue
		}
		member, ok := graph.Object(membership, rdf.SysML+pMemberElement)
		if ok && member.IsIRI() {
			index.addOwnedNameInScope(index.ownerForSubject(membership), member, name)
		}
	}
	index.chainOwners, index.externalChainOwners = apiJSONChainOwnerIndex(graph, subjectSlots, len(subjects))
	index.chainTargetsBySlot, index.chainTargetSet, index.chainTargets = apiJSONFeatureChainTargets(graph, index)
	return index
}

func (index *apiJSONNameIndex) ownerOf(graph *rdf.Graph, subject rdf.Term) rdf.Term {
	if owner, ok := index.ownerForSubjectOK(subject); ok {
		return owner
	}
	if owner, ok := index.externalOwners[subject.Value]; ok {
		return owner
	}
	owner := apiJSONNameOwner(graph, subject)
	if index.externalOwners == nil {
		index.externalOwners = make(map[string]rdf.Term)
	}
	index.externalOwners[subject.Value] = owner
	return owner
}

func (index *apiJSONNameIndex) addOwnedName(subject rdf.Term, name string) {
	owner := index.ownerForSubject(subject)
	index.addOwnedNameInScope(owner, subject, name)
}

func (index *apiJSONNameIndex) ownerForSubject(subject rdf.Term) rdf.Term {
	owner, _ := index.ownerForSubjectOK(subject)
	return owner
}

func (index *apiJSONNameIndex) ownerForSubjectOK(subject rdf.Term) (rdf.Term, bool) {
	position, ok := index.subjectSlots[subject.Value]
	if !ok || position >= len(index.owners) {
		return rdf.Term{}, false
	}
	return index.owners[position], true
}

func (index *apiJSONNameIndex) chainTarget(subject rdf.Term) (apiJSONChainTarget, bool) {
	position, ok := index.subjectSlots[subject.Value]
	if ok && position < len(index.chainTargetSet) && index.chainTargetSet[position] {
		return index.chainTargetsBySlot[position], true
	}
	target, ok := index.chainTargets[subject.Value]
	return target, ok
}

func (index *apiJSONNameIndex) chainRelationships(owner rdf.Term) []rdf.Term {
	position, ok := index.subjectSlots[owner.Value]
	if ok && position < len(index.chainOwners) {
		return index.chainOwners[position]
	}
	return index.externalChainOwners[owner.Value]
}

func (index *apiJSONNameIndex) scopePath(owner rdf.Term) ([]rdf.Term, bool) {
	position, ok := index.subjectSlots[owner.Value]
	if ok && position < len(index.scopeSet) && index.scopeSet[position] {
		return index.scopes[position], true
	}
	if !ok || position >= len(index.scopeSet) {
		ownerScopes, exists := index.externalScopes[owner.Value]
		return ownerScopes, exists
	}
	return nil, false
}

func (index *apiJSONNameIndex) setScopePath(owner rdf.Term, ownerScopes []rdf.Term) {
	position, ok := index.subjectSlots[owner.Value]
	if ok && position < len(index.scopes) {
		index.scopes[position] = ownerScopes
		index.scopeSet[position] = true
		return
	}
	if index.externalScopes == nil {
		index.externalScopes = make(map[string][]rdf.Term)
	}
	index.externalScopes[owner.Value] = ownerScopes
}

func (index *apiJSONNameIndex) importsFor(scope rdf.Term) []rdf.Term {
	position, ok := index.subjectSlots[scope.Value]
	if !ok || position >= len(index.imports) {
		return nil
	}
	return index.imports[position]
}

func (index *apiJSONNameIndex) addOwnedNameInScope(owner, target rdf.Term, name string) {
	if name == "" || owner.Value == "" || !target.IsIRI() {
		return
	}
	key := apiJSONScopeName{scope: owner.Value, name: name}
	index.ownedNames[key] = appendUniqueTerm(index.ownedNames[key], target)
}

func apiJSONCatalogNames(catalog *identity.Catalog) *apiJSONCatalogNameIndex {
	if cached, ok := apiJSONCatalogNameIndexes.Load(catalog); ok {
		return cached.(*apiJSONCatalogNameIndex)
	}
	index := &apiJSONCatalogNameIndex{
		byShortName:       make(map[string][]rdf.Term),
		importedDirect:    make(map[string]map[string][]rdf.Term),
		importedRecursive: make(map[string]map[string][]rdf.Term),
		importedQualified: make(map[string]map[string][]rdf.Term),
	}
	for _, element := range catalog.Elements() {
		if element.Symbol != nil && element.Symbol.Name != "" {
			index.byShortName[element.Symbol.Name] = appendUniqueTerm(
				index.byShortName[element.Symbol.Name], rdf.ElementIRIForID(element.ID),
			)
		}
		parts := strings.Split(element.FQN, "::")
		for i := 1; i < len(parts); i++ {
			namespace := strings.Join(parts[:i], "::")
			relative := strings.Join(parts[i:], "::")
			target := rdf.ElementIRIForID(element.ID)
			index.importedQualified[namespace] = appendAPIJSONCatalogName(
				index.importedQualified[namespace], element.FQN, target,
			)
			index.importedRecursive[namespace] = appendAPIJSONCatalogName(
				index.importedRecursive[namespace], relative, target,
			)
			if !strings.Contains(relative, "::") {
				index.importedDirect[namespace] = appendAPIJSONCatalogName(
					index.importedDirect[namespace], relative, target,
				)
			}
		}
	}
	actual, _ := apiJSONCatalogNameIndexes.LoadOrStore(catalog, index)
	return actual.(*apiJSONCatalogNameIndex)
}

func appendAPIJSONCatalogName(index map[string][]rdf.Term, name string, target rdf.Term) map[string][]rdf.Term {
	if index == nil {
		index = make(map[string][]rdf.Term)
	}
	index[name] = appendUniqueTerm(index[name], target)
	return index
}

func apiJSONChainOwnerIndex(
	graph *rdf.Graph,
	subjectSlots map[string]int,
	subjectCount int,
) ([][]rdf.Term, map[string][]rdf.Term) {
	owners := make([][]rdf.Term, subjectCount)
	var externalOwners map[string][]rdf.Term
	for _, relationship := range graph.SubjectsView() {
		if rdf.LocalName(graph.Type(relationship)) != mFeatureChaining {
			continue
		}
		owner := firstIRI(graph, relationship, pOwningRelatedElement, pOwner)
		if owner.Value != "" {
			if position, ok := subjectSlots[owner.Value]; ok && position < len(owners) {
				owners[position] = append(owners[position], relationship)
			} else {
				if externalOwners == nil {
					externalOwners = make(map[string][]rdf.Term)
				}
				externalOwners[owner.Value] = append(externalOwners[owner.Value], relationship)
			}
		}
	}
	return owners, externalOwners
}

func apiJSONFeatureChainTargets(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
) ([]apiJSONChainTarget, []bool, map[string]apiJSONChainTarget) {
	resolved := make([]apiJSONChainTarget, len(index.subjects))
	resolvedSet := make([]bool, len(index.subjects))
	resolvedBySubject := make(map[string]apiJSONChainTarget)
	resolveOwner := func(owner rdf.Term, relationships []rdf.Term) {
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
		members := make([]any, len(ordered))
		for i, relationship := range ordered {
			targets := graph.ObjectsView(relationship, rdf.SysML+pChainingFeature)
			if len(targets) != 1 {
				members[i] = apiJSONNameReference{}
				continue
			}
			target := targets[0]
			if target.IsIRI() {
				members[i] = apiJSONReference{ID: rdf.ReferenceID(owner, target)}
				continue
			}
			if target.IsLiteral() {
				members[i] = apiJSONNameReference{name: target.Value}
			} else {
				members[i] = apiJSONNameReference{}
			}
		}
		chain, targets, _ := apiJSONResolveFeatureChainWithTargets(graph, index, owner, members)
		for i, member := range chain {
			reference, ok := member.(apiJSONReference)
			if !ok || i >= len(targets) || targets[i].Value == "" {
				continue
			}
			if position, ok := index.subjectSlots[ordered[i].Value]; ok && position < len(resolved) {
				target := apiJSONChainTarget{reference: reference, target: targets[i]}
				resolved[position] = target
				resolvedSet[position] = true
				resolvedBySubject[ordered[i].Value] = target
			}
		}
	}
	for position, relationships := range index.chainOwners {
		if len(relationships) != 0 {
			resolveOwner(index.subjects[position], relationships)
		}
	}
	for ownerID, relationships := range index.externalChainOwners {
		resolveOwner(rdf.IRI(ownerID), relationships)
	}
	return resolved, resolvedSet, resolvedBySubject
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
			if !apiJSONFeatureChainNeedsResolution(members) {
				return value, nil
			}
			return apiJSONResolveFeatureChain(graph, index, subject, members)
		}
	}
	if rdf.LocalName(graph.Type(subject)) == mFeatureChaining {
		switch key {
		case pTarget:
			if name, ok := value.(apiJSONNameReference); ok {
				if target, resolved := index.chainTarget(subject); resolved {
					return target.reference, nil
				}
				return name, []string{name.name}
			}
		case pRelatedElement:
			if members, ok := value.([]any); ok {
				if !apiJSONValuesHaveNameReferences(members) {
					return value, nil
				}
				target, resolved := index.chainTarget(subject)
				result := make([]any, len(members))
				var unresolved []string
				for i, member := range members {
					if name, ok := member.(apiJSONNameReference); ok {
						if resolved {
							result[i] = target.reference
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
		if key == pChainingFeature && rdf.LocalName(graph.Type(subject)) == mFeatureChaining {
			if target, resolved := index.chainTarget(subject); resolved {
				return target.reference, nil
			}
			return value, []string{value.name}
		}
		target := rdf.Term{}
		target = apiJSONGraphResolveName(graph, index, subject, value.name)
		if target.Value == "" {
			target = apiJSONCatalogResolveName(graph, index, subject, value.name)
		}
		id, ok := apiJSONTermReferenceID(subject, target)
		if !ok {
			return value, []string{value.name}
		}
		return apiJSONReference{ID: id}, nil
	case []any:
		if !apiJSONValuesHaveNameReferences(value) {
			return value, nil
		}
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

func apiJSONValuesHaveNameReferences(values []any) bool {
	for _, value := range values {
		switch member := value.(type) {
		case apiJSONNameReference:
			return true
		case []any:
			if apiJSONValuesHaveNameReferences(member) {
				return true
			}
		}
	}
	return false
}

func apiJSONFeatureChainNeedsResolution(members []any) bool {
	for _, member := range members {
		switch value := member.(type) {
		case string, apiJSONNameReference:
			return true
		case []any:
			if apiJSONFeatureChainNeedsResolution(value) {
				return true
			}
		}
	}
	return false
}

func apiJSONResolveFeatureChain(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	members []any,
) ([]any, []string) {
	resolved, _, unresolved := apiJSONResolveFeatureChainWithTargets(graph, index, subject, members)
	return resolved, unresolved
}

func apiJSONResolveFeatureChainWithTargets(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	members []any,
) ([]any, []rdf.Term, []string) {
	resolved := make([]any, len(members))
	targets := make([]rdf.Term, len(members))
	var previous rdf.Term
	var unresolved []string
	for i, member := range members {
		switch value := member.(type) {
		case string:
			member = apiJSONNameReference{name: value}
		}
		if reference, ok := member.(apiJSONReference); ok {
			resolved[i] = reference
			previous = apiJSONFeatureChainReferenceTarget(index, subject, reference.ID)
			targets[i] = previous
			continue
		}
		name, isName := member.(apiJSONNameReference)
		if !isName {
			resolved[i] = member
			continue
		}
		target := rdf.Term{}
		if i == 0 {
			target = apiJSONGraphResolveName(graph, index, subject, name.name)
			if target.Value == "" {
				target = apiJSONCatalogResolveName(graph, index, subject, name.name)
			}
		} else if previous.Value != "" {
			if featureTypes := apiJSONFeatureTypes(graph, index, previous); len(featureTypes) > 0 {
				target = apiJSONGraphNamedTargetInTypes(graph, index, featureTypes, name.name)
			}
		}
		id, ok := apiJSONTermReferenceID(subject, target)
		if !ok {
			resolved[i] = member
			unresolved = append(unresolved, name.name)
			previous = rdf.Term{}
			continue
		}
		resolved[i] = apiJSONReference{ID: id}
		targets[i] = target
		previous = target
	}
	return resolved, targets, unresolved
}

func apiJSONFeatureChainReferenceTarget(index *apiJSONNameIndex, subject rdf.Term, id string) rdf.Term {
	if index != nil {
		if target, ok := index.subjectsByID[id]; ok {
			return target
		}
		if index.catalog != nil {
			if element, ok := index.catalog.Element(id); ok && element.ID != "" {
				return rdf.ElementIRIForID(element.ID)
			}
		}
	}
	return rdf.ReferenceIRI(subject, id)
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

func apiJSONFeatureTypes(graph *rdf.Graph, index *apiJSONNameIndex, feature rdf.Term) []rdf.Term {
	if featureType := apiJSONFeatureType(graph, feature); featureType.Value != "" {
		return []rdf.Term{featureType}
	}
	return apiJSONCatalogFeatureTypes(index, feature)
}

func apiJSONCatalogFeatureTypes(index *apiJSONNameIndex, feature rdf.Term) []rdf.Term {
	if index == nil || index.catalog == nil {
		return nil
	}
	id, ok := rdf.SubjectID(feature)
	if !ok {
		return nil
	}

	index.catalogTypeMu.Lock()
	defer index.catalogTypeMu.Unlock()
	if types, ok := index.catalogTypes[id]; ok {
		return append([]rdf.Term(nil), types...)
	}
	element, ok := index.catalog.Element(id)
	if !ok || element.Symbol == nil || !element.Symbol.IsFeature() {
		index.catalogTypes[id] = nil
		return nil
	}
	if index.catalogModel == nil {
		index.catalogModel = semantics.NewModel(resolve.New(libs.SharedBase()))
	}
	var types []rdf.Term
	for _, symbol := range index.catalogModel.FeatureTypeSet(element.Symbol) {
		typeElement, ok := index.catalog.ElementForSymbol(symbol)
		if !ok {
			typeElement, ok = index.catalog.ElementNamed(symbols.FQNOf(symbol))
		}
		if ok && typeElement.ID != "" {
			types = appendUniqueTerm(types, rdf.ElementIRIForID(typeElement.ID))
		}
	}
	index.catalogTypes[id] = append([]rdf.Term(nil), types...)
	return types
}

func apiJSONGraphResolveName(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	subject rdf.Term,
	name string,
) rdf.Term {
	scopes := []rdf.Term{subject}
	owner := index.ownerOf(graph, subject)
	if owner.Value != "" {
		ownerScopes, ok := index.scopePath(owner)
		if !ok {
			seen := make(map[string]bool)
			for scope := owner; scope.Value != "" && !seen[scope.Value]; scope = index.ownerOf(graph, scope) {
				seen[scope.Value] = true
				ownerScopes = append(ownerScopes, scope)
			}
			index.setScopePath(owner, ownerScopes)
		}
		for _, scope := range ownerScopes {
			if scope.Value != subject.Value {
				scopes = append(scopes, scope)
			}
		}
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
	key := apiJSONScopeName{scope: scope.Value, name: name}
	if matches, ok := index.importedNames[key]; ok {
		return matches
	}
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
	addAll := func(targets []rdf.Term) {
		for _, target := range targets {
			add(target)
		}
	}
	relations := index.importsFor(scope)
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
			addAll(index.catalogNames.importedQualified[root.FQN][name])
			if recursive {
				addAll(index.catalogNames.importedRecursive[root.FQN][name])
			} else {
				addAll(index.catalogNames.importedDirect[root.FQN][name])
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
	index.importedNames[key] = matches
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
	_ *rdf.Graph,
	index *apiJSONNameIndex,
	owner rdf.Term,
	name string,
) []rdf.Term {
	return index.ownedNames[apiJSONScopeName{scope: owner.Value, name: name}]
}

func apiJSONGraphNamedTargetInType(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	typ rdf.Term,
	name string,
) rdf.Term {
	return apiJSONGraphNamedTargetInTypes(graph, index, []rdf.Term{typ}, name)
}

func apiJSONGraphNamedTargetInTypes(
	graph *rdf.Graph,
	index *apiJSONNameIndex,
	types []rdf.Term,
	name string,
) rdf.Term {
	frontier := append([]rdf.Term(nil), types...)
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
			for _, target := range apiJSONCatalogNamedTargetsInType(index, scope, name) {
				matches = appendUniqueTerm(matches, target)
			}
			for _, parent := range apiJSONGraphSuperTypes(graph, scope) {
				if !seen[parent.Value] {
					next = appendUniqueTerm(next, parent)
				}
			}
			for _, parent := range apiJSONCatalogSuperTypes(index, scope) {
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

func apiJSONCatalogNamedTargetsInType(index *apiJSONNameIndex, typ rdf.Term, name string) []rdf.Term {
	if index.catalog == nil || !typ.IsIRI() || name == "" {
		return nil
	}
	element, ok := index.catalog.Element(rdf.LocalName(typ.Value))
	if !ok || element.Symbol == nil || element.Symbol.Scope == nil {
		return nil
	}
	var matches []rdf.Term
	for _, member := range element.Symbol.Scope.Members() {
		if member.Name != name && member.ShortName != name {
			continue
		}
		if target, ok := index.catalog.ElementForSymbol(member); ok && target.ID != "" {
			matches = appendUniqueTerm(matches, rdf.ElementIRIForID(target.ID))
		}
	}
	return matches
}

func apiJSONCatalogSuperTypes(index *apiJSONNameIndex, typ rdf.Term) []rdf.Term {
	if index.catalog == nil || !typ.IsIRI() {
		return nil
	}
	element, ok := index.catalog.Element(rdf.LocalName(typ.Value))
	if !ok || element.Symbol == nil || element.Symbol.Facts == nil {
		return nil
	}
	var parents []rdf.Term
	for _, super := range element.Symbol.Facts.Supers {
		if parent, ok := index.catalog.ElementNamed(super.FQN); ok && parent.ID != "" {
			parents = appendUniqueTerm(parents, rdf.ElementIRIForID(parent.ID))
		}
	}
	return parents
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
		for _, target := range index.catalogNames.byShortName[name] {
			if !target.IsIRI() {
				continue
			}
			found := false
			for _, prior := range matches {
				if prior == target {
					found = true
					break
				}
			}
			if !found {
				matches = append(matches, target)
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
