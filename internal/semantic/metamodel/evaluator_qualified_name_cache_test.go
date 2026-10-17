package metamodel

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type qualifiedNameCountingStructure struct {
	Structure
	mu                  sync.Mutex
	ownedRelationships  map[ElementKey]int
	owningRelationships map[ElementKey]int
}

type qualifiedNameEffectiveStructure struct {
	Structure
	effectiveNames map[ElementKey]string
}

func (s qualifiedNameEffectiveStructure) EffectiveName(element Element) (string, bool) {
	name, ok := s.effectiveNames[element.Key()]
	return name, ok
}

func (s *qualifiedNameCountingStructure) OwnedRelationships(element Element) ([]Element, bool) {
	s.mu.Lock()
	s.ownedRelationships[element.Key()]++
	s.mu.Unlock()
	return s.Structure.OwnedRelationships(element)
}

func (s *qualifiedNameCountingStructure) OwningRelationship(element Element) (Element, bool, bool) {
	s.mu.Lock()
	s.owningRelationships[element.Key()]++
	s.mu.Unlock()
	return s.Structure.OwningRelationship(element)
}

func (s *qualifiedNameCountingStructure) calls(counts map[ElementKey]int, key ElementKey) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return counts[key]
}

func TestStructureQualifiedNameMemoizesNamespaceAndOwner(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-cache.sysml", `package P {
		part def A;
		part def B;
	}`)
	pkg := testSymbol(t, root, "P")
	first := testSymbol(t, pkg.Scope, "A")
	second := testSymbol(t, pkg.Scope, "B")
	structure := testNamespaceStructure(evaluator, pkg, first, second)
	counting := &qualifiedNameCountingStructure{
		Structure:           structure,
		ownedRelationships:  make(map[ElementKey]int),
		owningRelationships: make(map[ElementKey]int),
	}
	evaluator.options.Structure = counting

	for _, test := range []struct {
		symbol *symbols.Symbol
		want   string
	}{
		{first, "A"},
		{second, "B"},
	} {
		got := testProperty(t, evaluator, ElementOf(test.symbol), "Element", "qualifiedName")
		if got.Kind != StringValue || got.String != test.want {
			t.Errorf("%s qualifiedName = %#v; want %q", test.symbol.Name, got, test.want)
		}
	}

	namespace := ElementOf(pkg).Key()
	if calls := counting.calls(counting.ownedRelationships, namespace); calls != 1 {
		t.Errorf("namespace owned relationships queried %d times; want one sibling-name index", calls)
	}
	if calls := counting.calls(counting.owningRelationships, namespace); calls != 1 {
		t.Errorf("namespace owner queried %d times; want one memoized owner qualifiedName", calls)
	}
}

func TestStructureQualifiedNameEffectiveNameUsesNearestBaseOwner(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-effective-owner.sysml", `package P {}`)
	pkg := testSymbol(t, root, "P")
	first := Element{Aspect: "first-repeated"}
	second := Element{Aspect: "second-repeated"}
	structure := testDocumentStructure(evaluator, root, pkg)
	namespace := ElementOf(pkg)
	addTestOwnedMembership(&structure, namespace, first, "PartDefinition")
	addTestOwnedMembership(&structure, namespace, second, "PartDefinition")
	structure.attributes[testRelatedKey{element: first.Key(), property: "declaredName"}] = stringValue("Repeated")
	structure.attributes[testRelatedKey{element: second.Key(), property: "declaredName"}] = stringValue("Repeated")
	evaluator.options.Structure = qualifiedNameEffectiveStructure{
		Structure: structure,
		effectiveNames: map[ElementKey]string{
			second.Key(): "kind",
		},
	}

	got := testProperty(t, evaluator, second, "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "P::kind" {
		t.Fatalf("effective Element::qualifiedName = %#v; want %q", got, "P::kind")
	}
	base, handled, ok := evaluator.structureQualifiedNameBase(second, make(map[ElementKey]bool))
	if !handled || !ok || base.Kind != NullValue {
		t.Fatalf("base qualifiedName = %#v, %t, %t; want null", base, handled, ok)
	}
}
