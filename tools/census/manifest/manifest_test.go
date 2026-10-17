package manifest

import (
	"strings"
	"testing"
)

func fixture() *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion, PilotTag: "t", PilotCommit: "c", PilotArtifact: "a",
		Specifications: map[string]Specification{
			"kerml": {Name: "KerML", Version: "1.0", Document: "formal/26-03-01", URL: "https://example.test", SHA256: "abc"},
			"sysml": {Name: "SysML", Version: "2.0", Document: "formal/26-03-02", URL: "https://example.test", SHA256: "def"},
		},
		FormalExtraction: &FormalExtractionRecord{Method: "m", Constraints: map[string]int{"kerml": 2, "sysml": 1}},
		Constraints: []Constraint{
			{Name: "validateA", Source: "kerml", Kind: "validate", Metaclass: "Element",
				Spec:   &SpecRef{Document: "kerml", Clause: "8.3.2.1.2", Name: "validateA"},
				Checks: "a", Implementation: "internal/check/passes/a.go:APass.Run", Message: "—", NegativeCase: "none",
				Codes: []string{"a"}, Passes: []string{"constraint"}, Status: StatusFaithful},
			{Name: "validateB", Source: "sysml", Kind: "validate", Metaclass: "Element",
				Spec:   &SpecRef{Document: "sysml", Clause: "8.3.2.1", Name: "validateBFormal"},
				Checks: "b", Implementation: "—", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: StatusUnknown, Rationale: "no pass yet"},
			{Name: "validateC", Source: "both", Kind: "validate", Metaclass: "Element",
				Checks: "c", Implementation: "—", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: StatusDeliberate, Rationale: "declined"},
			{Name: "checkD", Source: SourceSpec, Kind: "check", Metaclass: "Type",
				Spec:   &SpecRef{Document: "kerml", Clause: "8.3.3.1.2", Name: "checkD"},
				Checks: "d", OCL: "specializesFromLibrary('Base::Anything')", Implementation: "—", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: StatusUnknown, Rationale: "not yet assessed"},
			{Name: "deriveE", Source: SourceSpec, Kind: "derive", Metaclass: "Type",
				Spec:   &SpecRef{Document: "kerml", Clause: "8.3.3.1.2", Name: "deriveE"},
				Checks: "e", Implementation: "internal/semantic/e.go:E", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: StatusApproximate, Rationale: "computed lazily"},
		},
		Extensions: []Extension{}, Errata: []Erratum{}, SpecIssues: []SpecIssue{},
	}
}

func TestValidateAcceptsTheFixture(t *testing.T) {
	if err := fixture().Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestValidateHoldsSpecSourcedRowsToTheirFields: each rule a spec-sourced row
// must satisfy, and the ordering rule between the two groups, fails with the
// row named.
func TestValidateHoldsSpecSourcedRowsToTheirFields(t *testing.T) {
	cases := []struct {
		name string
		edit func(m *Manifest)
		want string
	}{
		{"spec row before pilot rows", func(m *Manifest) { m.Constraints[0], m.Constraints[3] = m.Constraints[3], m.Constraints[0] }, "out of order"},
		{"spec rows unsorted", func(m *Manifest) { m.Constraints[3], m.Constraints[4] = m.Constraints[4], m.Constraints[3] }, "checkD is out of order"},
		{"spec row without a clause", func(m *Manifest) { m.Constraints[3].Spec = nil }, "checkD is spec-sourced but cites no clause"},
		{"spec row under another name", func(m *Manifest) { m.Constraints[3].Spec.Name = "checkOther" }, "checkD is spec-sourced but cites no clause under its own name"},
		{"spec row without text", func(m *Manifest) { m.Constraints[3].Checks = " " }, "checkD is spec-sourced but states no text"},
		{"spec row with a pilot identifier", func(m *Manifest) { m.Constraints[3].Raw = "checkD_0" }, "records a pilot identifier"},
		{"pilot row with OCL", func(m *Manifest) { m.Constraints[0].OCL = "x" }, "validateA records OCL"},
		{"kind the name does not state", func(m *Manifest) { m.Constraints[3].Kind = "derive" }, `checkD has kind "derive", which its name does not state`},
		{"unknown kind", func(m *Manifest) {
			m.Constraints[3].Kind = "assert"
			m.Constraints[3].Name = "assertD"
			m.Constraints[3].Spec.Name = "assertD"
		}, "is not a normalized constraint name"},
		{"unknown source", func(m *Manifest) { m.Constraints[3].Source = "formal" }, `checkD has source "formal"`},
		{"spec name that is no constraint name", func(m *Manifest) { m.Constraints[1].Spec.Name = "Element" }, `validateB cites the spec name "Element"`},
		{"extraction record without a method", func(m *Manifest) { m.FormalExtraction.Method = "" }, "states no method"},
		{"extraction record counting an undeclared document", func(m *Manifest) { m.FormalExtraction.Constraints["uml"] = 1 }, `counts specification "uml"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fixture()
			tc.edit(m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

// TestCountsAndTableCountOnlyThePilotRows: the census figures and table are
// the pilot's rows; the spec-sourced rows are tallied by Coverage instead.
func TestCountsAndTableCountOnlyThePilotRows(t *testing.T) {
	m := fixture()
	c := m.Counts()
	if c.Total != 3 || c.Implemented() != 1 || c.ByState[StatusUnknown] != 1 || c.ByState[StatusDeliberate] != 1 || c.ByState[StatusApproximate] != 0 {
		t.Fatalf("counts %+v", c)
	}
	if got := m.Summary(); !strings.HasPrefix(got, "**Census:** 1 of 3 named constraints") {
		t.Fatalf("summary %q", got)
	}
	table := m.RenderTable()
	if strings.Contains(table, "checkD") || strings.Contains(table, "deriveE") || !strings.Contains(table, "`validateC`") {
		t.Fatalf("table:\n%s", table)
	}
	if got, want := len(m.PilotRows()), 3; got != want {
		t.Fatalf("%d pilot rows, want %d", got, want)
	}
	if got, want := len(m.SpecRows()), 2; got != want {
		t.Fatalf("%d spec rows, want %d", got, want)
	}
}

// TestCoverageTalliesTheDocumentsConstraints: per document, the constraints
// stated are the spec-sourced rows plus the pilot rows citing a document name,
// by kind, with the implemented and unknown spec-sourced ones apart.
func TestCoverageTalliesTheDocumentsConstraints(t *testing.T) {
	m := fixture()
	coverage := m.Coverage()
	if len(coverage) != 2 || coverage[0].Document != "kerml" || coverage[1].Document != "sysml" {
		t.Fatalf("coverage %+v", coverage)
	}
	kerml, sysml := coverage[0], coverage[1]
	if kerml.Stated != 3 || kerml.PilotNamed != 1 || kerml.SpecSourced != 2 || kerml.Implemented != 1 || kerml.Unknown != 1 ||
		kerml.ByKind["check"] != 1 || kerml.ByKind["derive"] != 1 || kerml.ByKind["validate"] != 1 {
		t.Fatalf("kerml %+v", kerml)
	}
	if sysml.Stated != 1 || sysml.PilotNamed != 1 || sysml.SpecSourced != 0 || sysml.ByKind["validate"] != 1 {
		t.Fatalf("sysml %+v", sysml)
	}
	rendered := m.RenderCoverage()
	for _, want := range []string{
		"| KerML 1.0 (`formal/26-03-01`) | 1 | 1 | 1 | 3 | 1 | 2 | 1 | 1 |",
		"| SysML 2.0 (`formal/26-03-02`) | 0 | 0 | 1 | 1 | 1 | 0 | 0 | 0 |",
		"state 4 constraints in the Constraints subsections of their abstract-syntax clauses — 1 `check*`, 1 `derive*` and 2 `validate*`. 2 of them are named by a pilot validator and counted in the census above; 2 are recorded from the documents alone, outside the census, of which 1 are implemented and 1 remain ❔ unknown.",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered coverage lacks %q:\n%s", want, rendered)
		}
	}
}
