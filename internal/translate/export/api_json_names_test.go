package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONFeatureChainResolvesInheritedMember(t *testing.T) {
	chain := rdf.IRI(rdf.Expression + "chain")
	first := rdf.IRI(rdf.Expression + "chain-link0")
	second := rdf.IRI(rdf.Expression + "chain-link1")
	previous := rdf.ElementIRIForID("launch-vehicle")
	derived := rdf.ElementIRIForID("LV")
	base := rdf.ElementIRIForID("Base")
	specialization := rdf.ElementIRIForID("LV__sp0")
	member := rdf.ElementIRIForID("Base__instrumentUnit")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: chain, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: first},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: second},
		{Subject: first, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: first, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: first, Predicate: rdf.SysMLTerm(pChainingFeature), Object: previous},
		{Subject: second, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: second, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: second, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("instrumentUnit")},
		{Subject: previous, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: previous, Predicate: rdf.SysMLTerm("type"), Object: derived},
		{Subject: derived, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: derived, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: specialization},
		{Subject: specialization, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("Subclassification")},
		{Subject: specialization, Predicate: rdf.SysMLTerm("superclassifier"), Object: base},
		{Subject: base, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: member, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: member, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("instrumentUnit")},
		{Subject: member, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: base},
	}, nil)

	index := newAPIJSONNameIndex(graph)
	if got, ok := index.chainTargets[second.Value]; !ok || got.reference.ID != rdf.ReferenceID(chain, member) {
		t.Fatalf("inherited chain target = %#v, want reference to %s", got, member.Value)
	}
	targetValue, unresolved := apiJSONResolveNameReference(
		graph, index, second, pTarget, apiJSONNameReference{name: "instrumentUnit"},
	)
	if len(unresolved) != 0 {
		t.Fatalf("FeatureChaining.target unresolved: %v", unresolved)
	}
	target, ok := targetValue.(apiJSONReference)
	if !ok || target.ID != rdf.ReferenceID(second, member) {
		t.Fatalf("FeatureChaining.target = %#v, want reference to %s", targetValue, member.Value)
	}
	relatedValue, unresolved := apiJSONResolveNameReference(graph, index, second, pRelatedElement, []any{
		apiJSONReference{ID: rdf.ReferenceID(second, chain)},
		apiJSONNameReference{name: "instrumentUnit"},
	})
	if len(unresolved) != 0 {
		t.Fatalf("FeatureChaining.relatedElement unresolved: %v", unresolved)
	}
	related, ok := relatedValue.([]any)
	if !ok || len(related) != 2 || related[1] != target {
		t.Fatalf("FeatureChaining.relatedElement = %#v, want chain and inherited target references", relatedValue)
	}
}

func TestAPIJSONFeatureChainPreservesExpressionTargetForLaterLink(t *testing.T) {
	owner := rdf.ElementIRIForID("P")
	chain := rdf.ElementIRIForID("chain")
	firstLink := rdf.ExpressionIRI(chain, "pfc0")
	secondLink := rdf.ExpressionIRI(chain, "pfc1")
	expressionFeature := rdf.ExpressionIRI(chain, "pfeature")
	vehicle := rdf.ElementIRIForID("Vehicle")
	wheel := rdf.ElementIRIForID("Vehicle__wheel")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: chain, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: firstLink},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: secondLink},
		{Subject: chain, Predicate: rdf.SysMLTerm(pChainingFeature), Object: expressionFeature},
		{Subject: chain, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("wheel")},
		{Subject: firstLink, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: firstLink, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: firstLink, Predicate: rdf.SysMLTerm(pChainingFeature), Object: expressionFeature},
		{Subject: firstLink, Predicate: rdf.SysMLTerm(pTarget), Object: expressionFeature},
		{Subject: secondLink, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: secondLink, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: secondLink, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("wheel")},
		{Subject: secondLink, Predicate: rdf.SysMLTerm(pTarget), Object: rdf.String("wheel")},
		{Subject: expressionFeature, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: expressionFeature, Predicate: rdf.SysMLTerm("type"), Object: vehicle},
		{Subject: vehicle, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: wheel, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: wheel, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("wheel")},
		{Subject: wheel, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: vehicle},
	}, nil)
	index := newAPIJSONNameIndex(graph)

	resolved, unresolved := apiJSONResolveFeatureChain(graph, index, chain, []any{
		apiJSONReference{ID: rdf.ReferenceID(chain, expressionFeature)},
		apiJSONNameReference{name: "wheel"},
	})
	if len(unresolved) != 0 {
		t.Fatalf("expression-target chain unresolved: %v", unresolved)
	}
	for i, target := range []rdf.Term{expressionFeature, wheel} {
		reference, ok := resolved[i].(apiJSONReference)
		if !ok || reference.ID != rdf.ReferenceID(chain, target) {
			t.Fatalf("chain link %d = %#v, want @id for %s", i, resolved[i], target.Value)
		}
	}

	elements, err := apiJSONElementsWithOptions(graph, APIJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	memberValue := func(element apiJSONObject, key string) (any, bool) {
		for _, member := range element {
			if member.key == key {
				return member.value, true
			}
		}
		return nil, false
	}
	findElement := func(subject rdf.Term) apiJSONObject {
		id, ok := rdf.SubjectID(subject)
		if !ok {
			t.Fatalf("subject %s has no API JSON id", subject.Value)
		}
		for _, element := range elements {
			if got, present := memberValue(element, "@id"); present && got == id {
				return element
			}
		}
		t.Fatalf("subject %s has no API JSON element", subject.Value)
		return nil
	}
	aggregate, present := memberValue(findElement(chain), pChainingFeature)
	aggregateLinks, ok := aggregate.([]any)
	if !present || !ok || len(aggregateLinks) != 2 {
		t.Fatalf("aggregate chainingFeature = %#v, want two references", aggregate)
	}
	for i, target := range []rdf.Term{expressionFeature, wheel} {
		expectedID := rdf.ReferenceID(chain, target)
		aggregateReference, ok := aggregateLinks[i].(apiJSONReference)
		if !ok || aggregateReference.ID != expectedID {
			t.Fatalf("aggregate link %d = %#v, want %s", i, aggregateLinks[i], expectedID)
		}
		relationshipTarget, present := memberValue(findElement([]rdf.Term{firstLink, secondLink}[i]), pTarget)
		relationshipReferences, ok := relationshipTarget.([]any)
		if !present || !ok || len(relationshipReferences) != 1 {
			t.Fatalf("relationship target %d = %#v, want aggregate reference %s",
				i, relationshipTarget, expectedID)
		}
		relationshipReference, ok := relationshipReferences[0].(apiJSONReference)
		if !ok || relationshipReference.ID != expectedID {
			t.Fatalf("relationship target %d = %#v, want aggregate reference %s",
				i, relationshipTarget, expectedID)
		}
	}
}

func TestAPIJSONFeatureChainDoesNotFallBackToCatalog(t *testing.T) {
	owner := rdf.ElementIRIForID("P")
	feature := rdf.ElementIRIForID("host")
	vehicle := rdf.ElementIRIForID("Vehicle")
	v := rdf.ElementIRIForID("v")
	first := rdf.ElementIRIForID("host-chain0")
	second := rdf.ElementIRIForID("host-chain1")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: feature, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: feature, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: feature, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: first},
		{Subject: feature, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: second},
		{Subject: first, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: first, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: feature},
		{Subject: first, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("v")},
		{Subject: second, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: second, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: feature},
		{Subject: second, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("done")},
		{Subject: vehicle, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: vehicle, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: v, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: v, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("v")},
		{Subject: v, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: v, Predicate: rdf.SysMLTerm("type"), Object: vehicle},
	}, nil)
	index := newAPIJSONNameIndex(graph)

	var libraryDone rdf.Term
	if element, ok := index.catalog.ElementNamed("done"); ok && element.ID != "" {
		libraryDone = rdf.ElementIRIForID(element.ID)
	}
	if libraryDone.Value == "" {
		candidates := index.catalogNames.byShortName["done"]
		if len(candidates) == 0 {
			t.Fatal("library catalog has no done member")
		}
		libraryDone = candidates[0]
	}
	catalogNames := *index.catalogNames
	catalogNames.byShortName = map[string][]rdf.Term{"done": {libraryDone}}
	index.catalogNames = &catalogNames
	if got := apiJSONCatalogResolveName(graph, index, feature, "done"); got != libraryDone {
		t.Fatalf("unique library done lookup = %s, want %s", got.Value, libraryDone.Value)
	}

	members := []any{
		apiJSONNameReference{name: "v"},
		apiJSONNameReference{name: "done"},
	}
	resolved, unresolved := apiJSONResolveFeatureChain(graph, index, feature, members)
	if len(unresolved) != 1 || unresolved[0] != "done" {
		t.Fatalf("unresolved chain links = %v, want [done]", unresolved)
	}
	if _, ok := resolved[1].(apiJSONNameReference); !ok {
		t.Fatalf("later chain link = %#v, want unresolved name", resolved[1])
	}

	var omissions []APIJSONOmission
	_, keep := apiJSONResolveUnresolvedProperty(
		graph, index, second, mFeatureChaining, pChainingFeature,
		apiJSONNameReference{name: "done"},
		APIJSONOptions{Omitted: func(omission APIJSONOmission) {
			omissions = append(omissions, omission)
		}},
	)
	if keep {
		t.Fatal("unresolved FeatureChaining.chainingFeature was kept")
	}
	if len(omissions) != 1 || omissions[0].Reason.Category != metamodel.ReasonUnresolvedReference ||
		omissions[0].Reason.Detail != "done" {
		t.Fatalf("omissions = %#v, want unresolved-reference reason for done", omissions)
	}
}

func TestAPIJSONGraphNameResolutionUsesShortNames(t *testing.T) {
	for _, property := range []string{pDeclaredShortName, pMemberShortName} {
		t.Run(property, func(t *testing.T) {
			owner := rdf.ElementIRIForID("P")
			subject := rdf.ElementIRIForID("reference")
			engine := rdf.ElementIRIForID("Engine")
			triples := []rdf.Triple{
				{Subject: subject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
				{Subject: subject, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
				{Subject: engine, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
				{Subject: engine, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("Engine")},
				{Subject: engine, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
			}
			if property == pDeclaredShortName {
				triples = append(triples, rdf.Triple{
					Subject: engine, Predicate: rdf.SysMLTerm(property), Object: rdf.String("motor"),
				})
			} else {
				membership := rdf.ElementIRIForID("motor-membership")
				triples = append(triples,
					rdf.Triple{Subject: membership, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mMembership)},
					rdf.Triple{Subject: membership, Predicate: rdf.SysMLTerm(pMembershipOwningNamespace), Object: owner},
					rdf.Triple{Subject: membership, Predicate: rdf.SysMLTerm(pMemberElement), Object: engine},
					rdf.Triple{Subject: membership, Predicate: rdf.SysMLTerm(property), Object: rdf.String("motor")},
				)
			}
			graph := rdf.NewGraphOf(triples, nil)
			index := newAPIJSONNameIndex(graph)
			value, unresolved := apiJSONResolveNameReference(
				graph, index, subject, pTarget, apiJSONNameReference{name: "motor"},
			)
			if len(unresolved) != 0 {
				t.Fatalf("short name unresolved: %v", unresolved)
			}
			reference, ok := value.(apiJSONReference)
			if !ok || reference.ID != rdf.ReferenceID(subject, engine) {
				t.Fatalf("short name result = %#v, want @id %q", value, rdf.ReferenceID(subject, engine))
			}
		})
	}
}

func TestAPIJSONFeatureChainResolvesLibraryMemberInType(t *testing.T) {
	index := newAPIJSONNameIndex(rdf.NewGraph())
	var libraryType, libraryMember rdf.Term
	var memberName string
	for _, member := range index.catalog.Elements() {
		if member.Symbol == nil || member.Symbol.OwnerScope == nil || member.Symbol.Name == "" {
			continue
		}
		owner := member.Symbol.OwnerScope.Owner()
		if owner == nil {
			continue
		}
		typeElement, ok := index.catalog.ElementForSymbol(owner)
		if !ok || typeElement.ID == "" {
			continue
		}
		typeTerm := rdf.ElementIRIForID(typeElement.ID)
		memberTerm := rdf.ElementIRIForID(member.ID)
		matches := apiJSONCatalogNamedTargetsInType(index, typeTerm, member.Symbol.Name)
		if len(matches) == 1 && matches[0] == memberTerm {
			libraryType = typeTerm
			libraryMember = memberTerm
			memberName = member.Symbol.Name
			break
		}
	}
	if libraryType.Value == "" {
		t.Fatal("library catalog has no uniquely named member in a scoped type")
	}
	if got := apiJSONGraphNamedTargetInType(rdf.NewGraph(), index, libraryType, memberName); got != libraryMember {
		t.Fatalf("library type member = %s, want %s", got.Value, libraryMember.Value)
	}
}

func TestAPIJSONFeatureChainSearchesAllEffectiveTypes(t *testing.T) {
	firstType := rdf.ElementIRIForID("TypeOne")
	secondType := rdf.ElementIRIForID("TypeTwo")
	firstMember := rdf.ElementIRIForID("TypeOne::done")
	secondMember := rdf.ElementIRIForID("TypeTwo::done")
	triples := []rdf.Triple{
		{Subject: firstMember, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: firstMember, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("done")},
		{Subject: firstMember, Predicate: rdf.SysMLTerm(pOwner), Object: firstType},
	}
	graph := rdf.NewGraphOf(triples, nil)
	index := newAPIJSONNameIndex(graph)
	if got := apiJSONGraphNamedTargetInTypes(graph, index, []rdf.Term{firstType, secondType}, "done"); got != firstMember {
		t.Fatalf("unique member across effective types = %s, want %s", got.Value, firstMember.Value)
	}

	graph = rdf.NewGraphOf(append(triples,
		rdf.Triple{Subject: secondMember, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		rdf.Triple{Subject: secondMember, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("done")},
		rdf.Triple{Subject: secondMember, Predicate: rdf.SysMLTerm(pOwner), Object: secondType},
	), nil)
	index = newAPIJSONNameIndex(graph)
	if got := apiJSONGraphNamedTargetInTypes(graph, index, []rdf.Term{firstType, secondType}, "done"); got.Value != "" {
		t.Fatalf("ambiguous member across effective types = %s, want unresolved", got.Value)
	}
}

func TestAPIJSONFeatureChainUsesCatalogTypeForLibraryStub(t *testing.T) {
	catalogIndex := newAPIJSONNameIndex(rdf.NewGraph())
	part, ok := catalogIndex.catalog.ElementNamed("Parts::Part")
	if !ok || part.ID == "" {
		t.Fatal("library catalog has no Parts::Part")
	}
	partTerm := rdf.ElementIRIForID(part.ID)
	starts := apiJSONCatalogNamedTargetsInType(catalogIndex, partTerm, "start")
	dones := apiJSONCatalogNamedTargetsInType(catalogIndex, partTerm, "done")
	if len(starts) != 1 || len(dones) != 1 {
		t.Fatalf("Parts::Part start/done = %v/%v, want one catalog member each", starts, dones)
	}
	start, done := starts[0], dones[0]

	owner := rdf.ElementIRIForID("P")
	chain := rdf.ElementIRIForID("chain")
	p := rdf.ElementIRIForID("p")
	links := []rdf.Term{
		rdf.ElementIRIForID("chain-link0"),
		rdf.ElementIRIForID("chain-link1"),
		rdf.ElementIRIForID("chain-link2"),
	}
	triples := []rdf.Triple{
		{Subject: chain, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: p, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: p, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("p")},
		{Subject: p, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		{Subject: p, Predicate: rdf.SysMLTerm("type"), Object: partTerm},
	}
	for i, name := range []string{"p", "start", "done"} {
		triples = append(triples,
			rdf.Triple{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: links[i]},
			rdf.Triple{Subject: links[i], Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
			rdf.Triple{Subject: links[i], Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
			rdf.Triple{Subject: links[i], Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String(name)},
		)
	}
	graph := rdf.NewGraphOf(triples, nil)
	index := newAPIJSONNameIndex(graph)
	for _, subject := range graph.SubjectsView() {
		if subject == start {
			t.Fatalf("middle library member %s unexpectedly has a graph description", start.Value)
		}
	}

	resolved, unresolved := apiJSONResolveFeatureChain(graph, index, chain, []any{
		apiJSONNameReference{name: "p"},
		apiJSONNameReference{name: "start"},
		apiJSONNameReference{name: "done"},
	})
	if len(unresolved) != 0 {
		t.Fatalf("feature chain unresolved links = %v", unresolved)
	}
	for i, target := range []rdf.Term{p, start, done} {
		reference, ok := resolved[i].(apiJSONReference)
		if !ok || reference.ID != rdf.ReferenceID(chain, target) {
			t.Fatalf("feature-chain link %d = %#v, want @id reference to %s", i, resolved[i], target.Value)
		}
	}
}

func TestAPIJSONFeatureChainAggregateMatchesRelationshipTargets(t *testing.T) {
	catalogIndex := newAPIJSONNameIndex(rdf.NewGraph())
	part, ok := catalogIndex.catalog.ElementNamed("Parts::Part")
	if !ok || part.ID == "" {
		t.Fatal("library catalog has no Parts::Part")
	}
	partTerm := rdf.ElementIRIForID(part.ID)
	starts := apiJSONCatalogNamedTargetsInType(catalogIndex, partTerm, "start")
	dones := apiJSONCatalogNamedTargetsInType(catalogIndex, partTerm, "done")
	if len(starts) != 1 || len(dones) != 1 {
		t.Fatalf("Parts::Part start/done = %v/%v, want one catalog member each", starts, dones)
	}
	start, done := starts[0], dones[0]
	startElement, ok := catalogIndex.catalog.Element(rdf.LocalName(start.Value))
	if !ok || startElement.FQN == "" {
		t.Fatalf("catalog has no FQN for start feature %s", start.Value)
	}

	owner := rdf.ElementIRIForID("P")
	chain := rdf.ElementIRIForID("chain")
	relationships := []rdf.Term{
		rdf.ExpressionIRI(chain, "pfc0"),
		rdf.ExpressionIRI(chain, "pfc1"),
	}
	names := []string{startElement.FQN, "done"}
	targets := []rdf.Term{start, done}
	makeGraph := func(lastName string) *rdf.Graph {
		triples := []rdf.Triple{
			{Subject: chain, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
			{Subject: chain, Predicate: rdf.SysMLTerm(pOwner), Object: owner},
		}
		for _, name := range []string{names[0], lastName} {
			triples = append(triples, rdf.Triple{
				Subject: chain, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String(name),
			})
		}
		for i, relationship := range relationships {
			name := names[i]
			if i == len(names)-1 {
				name = lastName
			}
			triples = append(triples,
				rdf.Triple{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: relationship},
				rdf.Triple{Subject: chain, Predicate: rdf.SysMLTerm("ownedFeatureChaining"), Object: relationship},
				rdf.Triple{Subject: relationship, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
				rdf.Triple{Subject: relationship, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
				rdf.Triple{Subject: relationship, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String(name)},
				rdf.Triple{Subject: relationship, Predicate: rdf.SysMLTerm(pTarget), Object: rdf.String(name)},
				rdf.Triple{Subject: relationship, Predicate: rdf.SysMLTerm(pRelatedElement), Object: chain},
				rdf.Triple{Subject: relationship, Predicate: rdf.SysMLTerm(pRelatedElement), Object: rdf.String(name)},
			)
		}
		return rdf.NewGraphOf(triples, nil)
	}

	graph := makeGraph(names[len(names)-1])
	index := newAPIJSONNameIndex(graph)
	resolved, unresolved := apiJSONResolveFeatureChain(graph, index, chain, []any{
		apiJSONNameReference{name: names[0]},
		apiJSONNameReference{name: names[1]},
	})
	if len(unresolved) != 0 {
		t.Fatalf("catalog-only first-link chain unresolved: %v", unresolved)
	}
	ordered := apiJSONOrderedFeatureChain(graph, index, chain)
	aggregate, err := apiJSONValues(chain, pChainingFeature, ordered, true)
	if err != nil {
		t.Fatalf("aggregate feature-chain values: %v", err)
	}
	aggregateValues, ok := aggregate.([]any)
	if !ok || len(aggregateValues) != len(targets) {
		t.Fatalf("aggregate feature-chain values = %#v, want %d references", aggregate, len(targets))
	}
	for i, target := range targets {
		reference, ok := resolved[i].(apiJSONReference)
		if !ok || reference.ID != rdf.ReferenceID(chain, target) {
			t.Fatalf("aggregate link %d = %#v, want @id reference to %s", i, resolved[i], target.Value)
		}
		aggregateReference, ok := aggregateValues[i].(apiJSONReference)
		if !ok || aggregateReference.ID != reference.ID {
			t.Fatalf("aggregate output link %d = %#v, want %s", i, aggregateValues[i], reference.ID)
		}
		relationshipTarget, unresolved := apiJSONResolveNameReference(
			graph, index, relationships[i], pTarget, apiJSONNameReference{name: names[i]},
		)
		if len(unresolved) != 0 {
			t.Fatalf("FeatureChaining %d target unresolved: %v", i, unresolved)
		}
		targetReference, ok := relationshipTarget.(apiJSONReference)
		if !ok || targetReference.ID != rdf.ReferenceID(relationships[i], target) {
			t.Fatalf("FeatureChaining %d target = %#v, want @id reference to %s", i, relationshipTarget, target.Value)
		}
		if targetReference.ID != reference.ID {
			t.Fatalf("FeatureChaining %d target id = %s, want aggregate id %s",
				i, targetReference.ID, reference.ID)
		}
		chainingFeature, unresolved := apiJSONResolveNameReference(
			graph, index, relationships[i], pChainingFeature, apiJSONNameReference{name: names[i]},
		)
		chainingFeatureReference, ok := chainingFeature.(apiJSONReference)
		if len(unresolved) != 0 || !ok || chainingFeatureReference.ID != reference.ID {
			t.Fatalf("FeatureChaining %d chainingFeature = %#v, unresolved %v, want %s",
				i, chainingFeature, unresolved, reference.ID)
		}
		related, unresolved := apiJSONResolveNameReference(graph, index, relationships[i], pRelatedElement, []any{
			apiJSONReference{ID: rdf.ReferenceID(relationships[i], chain)},
			apiJSONNameReference{name: names[i]},
		})
		if len(unresolved) != 0 {
			t.Fatalf("FeatureChaining %d relatedElement unresolved: %v", i, unresolved)
		}
		relatedValues, ok := related.([]any)
		if !ok || len(relatedValues) != 2 || relatedValues[1] != targetReference {
			t.Fatalf("FeatureChaining %d relatedElement = %#v, want owner and target references", i, related)
		}
		if got, ok := index.chainTargets[relationships[i].Value]; !ok || got.reference.ID != reference.ID {
			t.Fatalf("chain target %d = %#v, want reference %s", i, got, reference.ID)
		}
	}
	elements, err := apiJSONElementsWithOptions(graph, APIJSONOptions{})
	if err != nil {
		t.Fatalf("serialize resolved chain: %v", err)
	}
	subjectID := func(subject rdf.Term) string {
		id, ok := rdf.SubjectID(subject)
		if !ok {
			t.Fatalf("subject %s has no API JSON ID", subject.Value)
		}
		return id
	}
	findElement := func(id string) (apiJSONObject, bool) {
		for _, element := range elements {
			for _, member := range element {
				if member.key == "@id" && member.value == id {
					return element, true
				}
			}
		}
		return nil, false
	}
	memberValue := func(element apiJSONObject, key string) (any, bool) {
		for _, member := range element {
			if member.key == key {
				return member.value, true
			}
		}
		return nil, false
	}
	singleReference := func(value any) (string, bool) {
		if reference, ok := value.(apiJSONReference); ok {
			return reference.ID, true
		}
		values, ok := value.([]any)
		if !ok || len(values) != 1 {
			return "", false
		}
		reference, ok := values[0].(apiJSONReference)
		return reference.ID, ok
	}
	chainElement, ok := findElement(subjectID(chain))
	if !ok {
		t.Fatal("resolved feature chain was not serialized")
	}
	emittedChain, ok := memberValue(chainElement, pChainingFeature)
	emittedLinks, okLinks := emittedChain.([]any)
	if !ok || !okLinks || len(emittedLinks) != len(targets) {
		t.Fatalf("serialized aggregate chain = %#v, want %d references", emittedChain, len(targets))
	}
	for i, target := range targets {
		expectedID := rdf.ReferenceID(chain, target)
		aggregateReference, ok := emittedLinks[i].(apiJSONReference)
		if !ok || aggregateReference.ID != expectedID {
			t.Fatalf("serialized aggregate link %d = %#v, want %s", i, emittedLinks[i], expectedID)
		}
		relationshipElement, ok := findElement(subjectID(relationships[i]))
		if !ok {
			t.Fatalf("resolved FeatureChaining %d was not serialized", i)
		}
		targetValue, ok := memberValue(relationshipElement, pTarget)
		targetID, okTarget := singleReference(targetValue)
		if !ok || !okTarget || targetID != expectedID {
			t.Fatalf("serialized FeatureChaining %d target = %#v, want a reference to %s", i, targetValue, target.Value)
		}
		chainingValue, ok := memberValue(relationshipElement, pChainingFeature)
		chainingID, okChaining := singleReference(chainingValue)
		if !ok || !okChaining || chainingID != expectedID {
			t.Fatalf("serialized FeatureChaining %d chainingFeature = %#v, want %s",
				i, chainingValue, expectedID)
		}
		relatedValue, ok := memberValue(relationshipElement, pRelatedElement)
		relatedElements, okRelated := relatedValue.([]any)
		if !ok || !okRelated || len(relatedElements) != 2 {
			t.Fatalf("serialized FeatureChaining %d relatedElement = %#v, want owner and target references", i, relatedValue)
		}
		relatedReference, ok := relatedElements[1].(apiJSONReference)
		if !ok || relatedReference.ID != targetID {
			t.Fatalf("serialized FeatureChaining %d related target = %#v, want %s", i, relatedElements[1], targetID)
		}
	}

	graph = makeGraph("not-a-member")
	index = newAPIJSONNameIndex(graph)
	_, unresolved = apiJSONResolveFeatureChain(graph, index, chain, []any{
		apiJSONNameReference{name: names[0]},
		apiJSONNameReference{name: "not-a-member"},
	})
	if len(unresolved) != 1 || unresolved[0] != "not-a-member" {
		t.Fatalf("unresolved chain links = %v, want [not-a-member]", unresolved)
	}
	if got, ok := index.chainTargets[relationships[0].Value]; !ok ||
		got.reference.ID != rdf.ReferenceID(chain, start) {
		t.Fatalf("resolved prefix target = %#v, want reference to catalog feature %s", got, start.Value)
	}
	if _, ok := index.chainTargets[relationships[1].Value]; ok {
		t.Fatalf("unresolved FeatureChaining kept target %s", relationships[1].Value)
	}
	resolvedTarget, unresolved := apiJSONResolveNameReference(
		graph, index, relationships[0], pTarget, apiJSONNameReference{name: names[0]},
	)
	if len(unresolved) != 0 {
		t.Fatalf("resolved FeatureChaining target unresolved: %v", unresolved)
	}
	targetReference, ok := resolvedTarget.(apiJSONReference)
	if !ok || targetReference.ID != rdf.ReferenceID(relationships[0], start) {
		t.Fatalf("resolved FeatureChaining target = %#v, want reference to %s", resolvedTarget, start.Value)
	}
	resolvedChainingFeature, unresolved := apiJSONResolveNameReference(
		graph, index, relationships[0], pChainingFeature, apiJSONNameReference{name: names[0]},
	)
	chainingReference, ok := resolvedChainingFeature.(apiJSONReference)
	if len(unresolved) != 0 || !ok || chainingReference.ID != targetReference.ID {
		t.Fatalf("resolved FeatureChaining chainingFeature = %#v, unresolved %v, want %s",
			resolvedChainingFeature, unresolved, targetReference.ID)
	}
	unresolvedTarget, unresolved := apiJSONResolveNameReference(
		graph, index, relationships[1], pTarget, apiJSONNameReference{name: "not-a-member"},
	)
	if len(unresolved) != 1 || unresolved[0] != "not-a-member" {
		t.Fatalf("unresolved FeatureChaining target = %#v, unresolved names %v", unresolvedTarget, unresolved)
	}
	var omissions []APIJSONOmission
	var diagnostics []string
	elements, err = apiJSONElementsWithOptions(graph, APIJSONOptions{
		Omitted: func(omission APIJSONOmission) {
			omissions = append(omissions, omission)
		},
		Diagnostic: func(diagnostic string) {
			diagnostics = append(diagnostics, diagnostic)
		},
	})
	if err != nil {
		t.Fatalf("serialize unresolved chain: %v", err)
	}
	chainElement, ok = findElement(subjectID(chain))
	if !ok {
		t.Fatal("unresolved owner feature was not serialized")
	}
	if _, present := memberValue(chainElement, pChainingFeature); present {
		t.Fatal("unresolved owner feature kept its aggregate chainingFeature")
	}
	for _, property := range []string{pOwnedRelationship, "ownedFeatureChaining"} {
		value, present := memberValue(chainElement, property)
		if !present {
			t.Fatalf("unresolved owner is missing %s", property)
		}
		references, ok := value.([]any)
		if !ok || len(references) != len(relationships) {
			t.Fatalf("unresolved owner %s = %#v, want both FeatureChaining references", property, value)
		}
		for i, relationship := range relationships {
			reference, ok := references[i].(apiJSONReference)
			if !ok || reference.ID != rdf.ReferenceID(chain, relationship) {
				t.Fatalf("unresolved owner %s reference %d = %#v, want %s",
					property, i, references[i], rdf.ReferenceID(chain, relationship))
			}
		}
	}
	for _, relationship := range relationships {
		if _, ok := findElement(subjectID(relationship)); !ok {
			t.Fatalf("FeatureChaining %s was omitted from the output", relationship.Value)
		}
	}
	resolvedRelationship, _ := findElement(subjectID(relationships[0]))
	for _, property := range []string{pTarget, pChainingFeature} {
		value, present := memberValue(resolvedRelationship, property)
		if !present {
			t.Fatalf("resolved FeatureChaining is missing %s", property)
		}
		referenceID, ok := singleReference(value)
		if !ok || referenceID != rdf.ReferenceID(relationships[0], start) {
			t.Fatalf("resolved FeatureChaining %s = %#v, want %s",
				property, value, rdf.ReferenceID(relationships[0], start))
		}
	}
	unresolvedRelationship, _ := findElement(subjectID(relationships[1]))
	unresolvedTarget, present := memberValue(unresolvedRelationship, pTarget)
	if present {
		t.Fatalf("unresolved FeatureChaining target = %#v, want omission", unresolvedTarget)
	}
	mappingValues, present := memberValue(unresolvedRelationship, "sysx:mappingValues")
	mappingProperties, ok := mappingValues.(apiJSONObject)
	if !present || !ok {
		t.Fatalf("unresolved FeatureChaining mappingValues = %#v, want target graph value", mappingValues)
	}
	var preservedTarget bool
	for _, property := range mappingProperties {
		if property.key != pTarget {
			continue
		}
		values, ok := property.value.([]any)
		preservedTarget = ok && len(values) == 1 && values[0] == "not-a-member"
	}
	if !preservedTarget {
		t.Fatalf("unresolved FeatureChaining mappingValues = %#v, want target [not-a-member]", mappingValues)
	}
	for _, expected := range []struct {
		subject   string
		metaclass string
		property  string
	}{
		{subjectID(chain), mFeature, pChainingFeature},
		{subjectID(relationships[1]), mFeatureChaining, pTarget},
	} {
		found := false
		for _, omission := range omissions {
			if omission.Subject == expected.subject &&
				omission.Metaclass == expected.metaclass &&
				omission.Property == expected.property &&
				omission.Reason.Category == metamodel.ReasonUnresolvedReference &&
				omission.Reason.Detail == "not-a-member" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing unresolved-reference omission for %s.%s on %s: %#v",
				expected.metaclass, expected.property, expected.subject, omissions)
		}
	}
	if !slices.Contains(diagnostics, "unresolved API JSON reference FeatureChaining.target: not-a-member") {
		t.Fatalf("unresolved FeatureChaining target diagnostic not emitted: %v", diagnostics)
	}
}
