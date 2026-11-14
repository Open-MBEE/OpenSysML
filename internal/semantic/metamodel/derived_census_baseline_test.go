package metamodel

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestDerivedCensusBaselineKeysServedRules holds the recorded census to the rules it
// measures: the records count every served rule and only those, under header roots.
func TestDerivedCensusBaselineKeysServedRules(t *testing.T) {
	content, err := os.ReadFile("derived_census_baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline struct {
		Structure string `json:"structure"`
		Roots     []struct {
			Name string `json:"name"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(content, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Structure == "" {
		t.Error("baseline does not name the Structure the evaluator read")
	}
	roots := map[string]int{}
	for i, root := range baseline.Roots {
		roots[root.Name] = i + 1
	}
	served := map[string]bool{}
	for _, rule := range Rules() {
		served[rule.DefiningClass+"::"+rule.Property] = true
	}
	records, err := os.ReadFile("derived_census_records.tsv")
	if err != nil {
		t.Fatal(err)
	}
	counted := map[string]bool{}
	previousRule, previousRoot := "", 0
	for number, line := range strings.Split(string(records), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 11 {
			t.Fatalf("derived_census_records.tsv:%d: want 11 fields, got %d", number+1, len(fields))
		}
		if !served[fields[0]] {
			t.Errorf("derived_census_records.tsv:%d: counts %s, which rules.go does not serve", number+1, fields[0])
		}
		if roots[fields[1]] == 0 {
			t.Errorf("derived_census_records.tsv:%d: names root %s, which the baseline header does not", number+1, fields[1])
		}
		if fields[0] < previousRule || fields[0] == previousRule && roots[fields[1]] <= previousRoot {
			t.Errorf("derived_census_records.tsv:%d: %s %s is out of order; records are sorted by rule, then by the header's root order", number+1, fields[0], fields[1])
		}
		previousRule, previousRoot = fields[0], roots[fields[1]]
		counted[fields[0]] = true
	}
	if len(counted) == 0 {
		t.Fatal("derived_census_records.tsv holds no records")
	}
	for name := range served {
		if !counted[name] {
			t.Errorf("derived_census_records.tsv does not count %s, which rules.go serves; re-record with go run -C tools ./cmd/derived-census -update", name)
		}
	}
}
