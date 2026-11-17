package hierarchy

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

func TestNewOrdersParentsFirstAndLinearizesDiamonds(t *testing.T) {
	h, err := New([]ontology.Class{
		{Name: "Leaf", Parents: []string{"Left", "Right"}},
		{Name: "Right", Parents: []string{"Base"}},
		{Name: "Left", Parents: []string{"Base"}},
		{Name: "Base", Parents: []string{"Element"}},
		{Name: "Element"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order := make([]string, len(h.Order))
	for i, class := range h.Order {
		order[i] = class.Name
	}
	if got := strings.Join(order, ","); got != "Element,Base,Left,Right,Leaf" {
		t.Fatalf("order = %s", got)
	}
	if got := strings.Join(h.MRO["Leaf"], ","); got != "Leaf,Left,Right,Base,Element" {
		t.Fatalf("Leaf MRO = %s", got)
	}
	if !h.IsAncestorOrSelf("Leaf", "Base") || !h.IsAncestorOrSelf("Leaf", "Leaf") || h.IsAncestorOrSelf("Base", "Leaf") {
		t.Fatal("IsAncestorOrSelf disagrees with the linearization")
	}
}

func TestNewRefusesUnsoundHierarchies(t *testing.T) {
	for want, classes := range map[string][]ontology.Class{
		"metaclass A is declared more than once": {{Name: "A"}, {Name: "A"}},
		"A has unknown metaclass parent Missing": {{Name: "A", Parents: []string{"Missing"}}},
		"metaclass inheritance contains a cycle": {{Name: "A", Parents: []string{"B"}}, {Name: "B", Parents: []string{"A"}}},
		"C3 linearization failed for Z": {
			{Name: "O"}, {Name: "X", Parents: []string{"O"}}, {Name: "Y", Parents: []string{"O"}},
			{Name: "A", Parents: []string{"X", "Y"}}, {Name: "B", Parents: []string{"Y", "X"}},
			{Name: "Z", Parents: []string{"A", "B"}},
		},
	} {
		if _, err := New(classes); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("New error = %v, want substring %q", err, want)
		}
	}
}
