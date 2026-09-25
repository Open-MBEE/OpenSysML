package diff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func exportReport() *Report {
	return &Report{
		Pilot: "2026-07",
		Roots: []RootReport{
			{
				Name: "kerml-examples",
				Dir:  "examples/pilot-corpora/kerml-examples",
				Files: []FileReport{
					{
						Path:      "clean.kerml",
						Agreement: []Entry{{Line: 3, Severity: "error", Category: "unresolved-reference", Count: 1}},
					},
					{
						Path: "disjoint.kerml",
						PilotOnly: []Entry{{Line: 7, Severity: "error", Category: "unmapped", Count: 2,
							Examples: []string{"pilot: The opposite features do not refer to each other"}}},
					},
				},
			},
			{
				Name: "examples",
				Dir:  "examples",
				Files: []FileReport{
					{
						Path:          "pseudostates-demo.sysml",
						OpenSysMLOnly: []Entry{{Line: 13, Severity: "warning", Category: "nonstandard-notation", Count: 1}},
						SeverityMismatch: []SeverityEntry{{Line: 20, Category: "duplicate-member",
							OpenSysML: "error", Pilot: "warning", Count: 1}},
					},
				},
			},
		},
	}
}

func TestJUnitFileCases(t *testing.T) {
	doc := junitReport(exportReport())
	if len(doc.Suites) != 2 {
		t.Fatalf("suites = %d, want 2", len(doc.Suites))
	}
	if doc.Tests != 3 || doc.Failures != 2 || doc.Errors != 0 || doc.Skipped != 0 {
		t.Fatalf("counts = %d/%d/%d/%d, want 3/2/0/0", doc.Tests, doc.Failures, doc.Errors, doc.Skipped)
	}

	agreeing := doc.Suites[0].Cases[0]
	if agreeing.Failure != nil {
		t.Fatalf("fully agreeing file failed: %+v", agreeing)
	}
	if agreeing.File != "examples/pilot-corpora/kerml-examples/clean.kerml" {
		t.Fatalf("file attribute = %q", agreeing.File)
	}

	disagreeing := doc.Suites[0].Cases[1]
	if disagreeing.Failure == nil || !strings.Contains(disagreeing.Failure.Body, "only the pilot (x2)") {
		t.Fatalf("disagreeing file = %+v", disagreeing)
	}
	mixed := doc.Suites[1].Cases[0]
	if mixed.Failure == nil ||
		!strings.Contains(mixed.Failure.Body, "only OpenSysML (x1)") ||
		!strings.Contains(mixed.Failure.Body, "ours error, pilot warning") {
		t.Fatalf("mixed file = %+v", mixed)
	}
}

func TestSarifReport(t *testing.T) {
	log := sarifReport(exportReport())
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("log shape = %q, %d runs", log.Version, len(log.Runs))
	}
	results := log.Runs[0].Results
	// One per disagreeing group; agreement rows are not findings.
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3:\n%+v", len(results), results)
	}

	// Sorted by URI, then line.
	if results[0].RuleID != "only-pilot" ||
		results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI != "examples/pilot-corpora/kerml-examples/disjoint.kerml" ||
		results[0].Locations[0].PhysicalLocation.Region.StartLine != 7 {
		t.Fatalf("first result = %+v", results[0])
	}
	if !strings.Contains(results[0].Message.Text, "pilot: The opposite features") {
		t.Fatalf("example not carried: %q", results[0].Message.Text)
	}
	if results[1].RuleID != "only-opensysml" || results[1].Level != "warning" {
		t.Fatalf("second result = %+v", results[1])
	}
	if results[2].RuleID != "severity-mismatch" || results[2].Level != "error" ||
		!strings.Contains(results[2].Message.Text, "the pilot warning") {
		t.Fatalf("third result = %+v", results[2])
	}

	data, err := marshalSarif(log)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	again, err := marshalSarif(sarifReport(exportReport()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatal("two renders of the same report differ")
	}
}

func TestSarifLevel(t *testing.T) {
	for severity, want := range map[string]string{"error": "error", "warning": "warning", "info": "note", "": "note"} {
		if got := sarifLevel(severity); got != want {
			t.Fatalf("sarifLevel(%q) = %q, want %q", severity, got, want)
		}
	}
}

func TestSarifWholeFileLine(t *testing.T) {
	locations := sarifLocations("a.sysml", 0)
	if locations[0].PhysicalLocation.Region.StartLine != 1 {
		t.Fatalf("line 0 mapped to %d, want 1", locations[0].PhysicalLocation.Region.StartLine)
	}
}
