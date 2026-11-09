package derived

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

func TestCensusRootCountsEveryServedRuleOverTheProbe(t *testing.T) {
	census, err := censusRoot("testdata", []string{"probe.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if census.Files != 1 || len(census.Rejected) != 0 || census.Elements == 0 {
		t.Fatalf("census = %+v", census)
	}
	if census.Implied == 0 {
		t.Error("the walk reached no implied relationship; the semantic model's synthesized relationships are part of the census")
	}
	// Six usages carry Feature::type; every one is typed, so every one answers a value.
	typed := census.Rules["Feature::type"]
	if typed.Carriers < 6 || typed.Value+typed.Empty+typed.NotDerived != typed.Carriers {
		t.Errorf("Feature::type = %+v", typed)
	}
	// The one import carries Import::importedElement.
	if imported := census.Rules["Import::importedElement"]; imported.Carriers != 1 {
		t.Errorf("Import::importedElement = %+v, want one carrier", imported)
	}
	for name, counts := range census.Rules {
		if counts.Value+counts.Empty+counts.NotDerived != counts.Carriers {
			t.Errorf("%s = %+v: the classes do not partition the carriers", name, counts)
		}
	}
	for name := range census.Rules {
		if !served(name) {
			t.Errorf("%s is counted but not served", name)
		}
	}
}

func TestCensusRootIsDeterministic(t *testing.T) {
	first, err := censusRoot("testdata", []string{"probe.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := censusRoot("testdata", []string{"probe.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Elements != second.Elements || len(first.Rules) != len(second.Rules) {
		t.Fatalf("two runs differ: %d/%d elements, %d/%d rules", first.Elements, second.Elements, len(first.Rules), len(second.Rules))
	}
	for name, counts := range first.Rules {
		if second.Rules[name] != counts {
			t.Errorf("%s: %+v then %+v", name, counts, second.Rules[name])
		}
	}
}

func TestCensusRootRecordsRejectedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(filepath.Join(dir, "bad.sysml"), "package {"); err != nil {
		t.Fatal(err)
	}
	census, err := censusRoot(dir, []string{"bad.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(census.Rejected) != 1 || census.Elements != 0 {
		t.Errorf("census = %+v, want the one file rejected and nothing counted", census)
	}
}

func TestPilotElementClassifiesPresentEmptyAndAbsent(t *testing.T) {
	classes := newMetaclasses()
	census := &PilotCensus{Rules: map[string]Counts{}, seen: map[string]bool{}}
	rules := metamodel.Rules()
	calc := map[string]any{
		"@id": "calc", "@type": "CalculationDefinition", "result": map[string]any{"@id": "x"}, "step": []any{}, "isImplied": false,
	}
	countPilotElement(calc, rules, classes, census)
	countPilotElement(calc, rules, classes, census)
	countPilotElement(map[string]any{"@id": "lib", "@type": "Package", "isLibraryElement": true, "name": "ScalarValues"}, rules, classes, census)
	countPilotElement(map[string]any{"@id": "sub", "@type": "Subclassification", "isImplied": true}, rules, classes, census)
	if census.Elements != 2 || census.Implied != 1 || census.Duplicates != 1 {
		t.Errorf("elements %d implied %d duplicates %d, want 2, 1 and 1 (the library element is not counted, the copy once)",
			census.Elements, census.Implied, census.Duplicates)
	}
	if got := census.Rules["Function::result"]; got != (Counts{Carriers: 1, Value: 1}) {
		t.Errorf("Function::result = %+v", got)
	}
	if got := census.Rules["Behavior::step"]; got != (Counts{Carriers: 1, Empty: 1}) {
		t.Errorf("Behavior::step = %+v", got)
	}
	if got := census.Rules["Function::expression"]; got != (Counts{Carriers: 1, NotDerived: 1}) {
		t.Errorf("Function::expression = %+v", got)
	}
	if got := census.Rules["Subclassification::owningClassifier"]; got.Carriers != 1 {
		t.Errorf("Subclassification::owningClassifier = %+v, want the implied Subclassification counted", got)
	}
	if got := census.Rules["Package::filterCondition"]; got.Carriers != 0 {
		t.Errorf("Package::filterCondition = %+v, want the library package left out", got)
	}
}

func TestSilentIsOursNeverAValueWhereThePilotAnswers(t *testing.T) {
	cases := []struct {
		name string
		row  RuleCounts
		want bool
	}{
		{"empty everywhere, pilot answers", RuleCounts{Ours: Counts{Carriers: 3, Empty: 3}, Pilot: Counts{Carriers: 3, Value: 1}}, true},
		{"not derived everywhere, pilot answers", RuleCounts{Ours: Counts{Carriers: 3, NotDerived: 3}, Pilot: Counts{Carriers: 3, Value: 3}}, true},
		{"answers once", RuleCounts{Ours: Counts{Carriers: 3, Value: 1, Empty: 2}, Pilot: Counts{Carriers: 3, Value: 3}}, false},
		{"pilot empty too", RuleCounts{Ours: Counts{Carriers: 3, Empty: 3}, Pilot: Counts{Carriers: 3, Empty: 3}}, false},
		{"no carriers", RuleCounts{Pilot: Counts{Carriers: 3, Value: 3}}, false},
	}
	for _, c := range cases {
		if got := c.row.Silent(); got != c.want {
			t.Errorf("%s: Silent = %v, want %v", c.name, got, c.want)
		}
	}
	recorded := &Report{Silent: []string{"A::a", "B::b"}}
	fresh := &Report{Silent: []string{"B::b", "C::c"}}
	if got := newlySilent(recorded, fresh); len(got) != 1 || got[0] != "C::c" {
		t.Errorf("newlySilent = %v, want [C::c]", got)
	}
}

func TestReportFinishClassifiesRules(t *testing.T) {
	report := &Report{Roots: []RootReport{
		{Name: "one", Rules: map[string]RuleCounts{
			"Feature::type":               {Ours: Counts{Carriers: 2, Value: 2}, Pilot: Counts{Carriers: 2, Value: 2}},
			"Feature::ownedTypeFeaturing": {Ours: Counts{Carriers: 2, Empty: 2}, Pilot: Counts{Carriers: 2, Value: 1, Empty: 1}},
			"Import::importedElement":     {Ours: Counts{Carriers: 1, NotDerived: 1}, Pilot: Counts{Carriers: 1, Value: 1}},
		}},
		{Name: "two", Rules: map[string]RuleCounts{
			"Feature::type": {Ours: Counts{Carriers: 1, Empty: 1}, Pilot: Counts{Carriers: 1, Value: 1}},
		}},
	}}
	report.finish()
	if report.Totals["Feature::type"].Ours != (Counts{Carriers: 3, Value: 2, Empty: 1}) {
		t.Errorf("totals = %+v", report.Totals["Feature::type"])
	}
	if strings.Join(report.Silent, ",") != "Feature::ownedTypeFeaturing,Import::importedElement" {
		t.Errorf("silent = %v", report.Silent)
	}
	s := report.Summary
	if s.Served != len(metamodel.Rules()) || s.Answering != 1 || s.Empty != 1 || s.NotDerived != 1 || s.Silent != 2 || s.PilotAnswering != 3 ||
		s.Uncarried != s.Served-3 {
		t.Errorf("summary = %+v", s)
	}
	if report.Structure == "" || len(report.Policy) == 0 {
		t.Error("the report names no structure or policy")
	}
}

func TestProvenanceNamesTheBridgeAndTheRoots(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	libraries := filepath.Join(root, "internal", "workspace", "libs", "stdlib", "OpenSysML Libraries")
	record, err := provenance(root, libraries)
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	for _, tool := range record.Tools {
		tools = append(tools, tool.Name)
	}
	if strings.Join(tools, ",") != bridgeName {
		t.Errorf("tools = %v", tools)
	}
	var inputs []string
	for _, input := range record.Inputs {
		inputs = append(inputs, input.Name)
	}
	for _, want := range []string{"examples", "testdata", "manual-examples"} {
		if !contains(inputs, want) {
			t.Errorf("inputs %v lack %s", inputs, want)
		}
	}
}

func served(name string) bool {
	for _, rule := range metamodel.Rules() {
		if ruleName(rule) == name {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
