package rdf_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

func TestDerivedPropertyRegistryCoverage(t *testing.T) {
	derived := make(map[string]bool)
	for _, property := range ontology.Properties() {
		if property.Derived {
			derived[property.DefiningClass+"::"+property.Name] = true
		}
	}
	served := make(map[string]bool)
	for _, rule := range metamodel.Rules() {
		key := rule.DefiningClass + "::" + rule.Property
		if served[key] {
			t.Errorf("duplicate evaluator rule for %s", key)
		}
		if (rule.Constraint == "") == (rule.Basis == "") {
			t.Errorf("evaluator rule for %s must have exactly one constraint or basis", key)
		}
		if derived[key] && served[key] {
			t.Errorf("derived property %s has multiple evaluator entries", key)
		}
		served[key] = true
	}
	omitted := make(map[string]bool)
	for _, omission := range metamodel.Omitted() {
		key := omission.DefiningClass + "::" + omission.Property
		if omitted[key] {
			t.Errorf("duplicate evaluator omission for %s", key)
		}
		if omission.Reason == "" {
			t.Errorf("evaluator omission for %s has no reason", key)
		}
		if served[key] {
			t.Errorf("%s is both served and omitted", key)
		}
		if !derived[key] {
			t.Errorf("omitted evaluator property %s is not a derived ontology property", key)
		}
		omitted[key] = true
	}

	for _, property := range ontology.Properties() {
		if !property.Derived {
			continue
		}
		key := property.DefiningClass + "::" + property.Name
		if !served[key] && !omitted[key] {
			t.Errorf("derived ontology property %s is neither served nor omitted", key)
		}
	}
}
