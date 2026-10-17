package export

import (
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
	if got := index.chainTargets[second.Value]; got != member {
		t.Fatalf("inherited chain target = %s, want %s", got.Value, member.Value)
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
