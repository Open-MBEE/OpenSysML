package ontology_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// TestPropertyOf checks the defining-class disambiguation: the declaration a
// metaclass inherits from an ancestor, the more specific one when several
// qualify, and the recorded multiplicity.
func TestPropertyOf(t *testing.T) {
	cases := []struct {
		metaclass, name string
		wantIRI         string
		wantMany        bool
	}{
		{"PartUsage", "type", "https://www.omg.org/spec/SysML#Feature_type", true},
		{"FeatureTyping", "type", "https://www.omg.org/spec/SysML#FeatureTyping_type", false},
		{"Package", "ownedRelationship", "https://www.omg.org/spec/SysML#Element_ownedRelationship", true},
		{"Element", "owningRelationship", "https://www.omg.org/spec/SysML#Element_owningRelationship", false},
	}
	for _, c := range cases {
		got, ok := ontology.PropertyOf(c.metaclass, c.name)
		if !ok {
			t.Errorf("PropertyOf(%q, %q) not found", c.metaclass, c.name)
			continue
		}
		if got.IRI != c.wantIRI {
			t.Errorf("PropertyOf(%q, %q).IRI = %s, want %s", c.metaclass, c.name, got.IRI, c.wantIRI)
		}
		if got.Many != c.wantMany {
			t.Errorf("PropertyOf(%q, %q).Many = %t, want %t", c.metaclass, c.name, got.Many, c.wantMany)
		}
	}
	if _, ok := ontology.PropertyOf("Package", "nonsense"); ok {
		t.Error("PropertyOf(Package, nonsense) found, want not found")
	}
}

// TestManyAgreed checks the agreement fallback an extension element needs:
// consensus names report their shared multiplicity, ambiguous or undeclared
// names report none.
func TestManyAgreed(t *testing.T) {
	cases := []struct {
		name       string
		wantMany   bool
		wantAgreed bool
	}{
		{"ownedRelationship", true, true},
		{"type", false, false},
		{"nonsense", false, false},
	}
	for _, c := range cases {
		many, agreed := ontology.ManyAgreed(c.name)
		if many != c.wantMany || agreed != c.wantAgreed {
			t.Errorf("ManyAgreed(%q) = (%t, %t), want (%t, %t)",
				c.name, many, agreed, c.wantMany, c.wantAgreed)
		}
	}
}
