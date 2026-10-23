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
