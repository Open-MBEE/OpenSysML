package metamodel

import (
	"encoding/json"
	"os"
	"testing"
)

// TestDerivedCensusBaselineKeysServedRules holds the recorded census to the
// rules it measures: every rule it counts or names silent is served, and no
// served rule is missing from it. tools/cmd/derived-census -update re-records it.
func TestDerivedCensusBaselineKeysServedRules(t *testing.T) {
	content, err := os.ReadFile("derived_census_baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline struct {
		Structure string                     `json:"structure"`
		Totals    map[string]json.RawMessage `json:"totals"`
		Silent    []string                   `json:"silent"`
		Summary   struct {
			Served int `json:"served"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(content, &baseline); err != nil {
		t.Fatal(err)
	}
	served := map[string]bool{}
	for _, rule := range Rules() {
		served[rule.DefiningClass+"::"+rule.Property] = true
	}
	if baseline.Summary.Served != len(served) {
		t.Errorf("baseline records %d served rules, rules.go serves %d; re-record with go run -C tools ./cmd/derived-census -update",
			baseline.Summary.Served, len(served))
	}
	for name := range baseline.Totals {
		if !served[name] {
			t.Errorf("baseline counts %s, which rules.go does not serve", name)
		}
	}
	for _, name := range baseline.Silent {
		if !served[name] {
			t.Errorf("baseline names %s silent, which rules.go does not serve", name)
		}
		if _, counted := baseline.Totals[name]; !counted {
			t.Errorf("baseline names %s silent without counting it", name)
		}
	}
	if baseline.Structure == "" {
		t.Error("baseline does not name the Structure the evaluator read")
	}
}
