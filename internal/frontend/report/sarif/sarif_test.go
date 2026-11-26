package sarif_test

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif/sariftest"
)

func validate(t *testing.T, document []byte) {
	t.Helper()
	if err := sariftest.Validate(document); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyLogIsValid(t *testing.T) {
	data, err := sarif.Marshal(sarif.NewLog(sarif.Driver{Name: "sysml", Version: "1.2.3"}))
	if err != nil {
		t.Fatal(err)
	}
	validate(t, data)
}

func TestLogWithFixIsValid(t *testing.T) {
	log := sarif.NewLog(sarif.Driver{
		Name:           "sysml",
		InformationURI: "https://github.com/Open-MBEE/OpenSysML",
		Version:        "1.2.3",
		Rules:          []sarif.Rule{{ID: "unused-import", ShortDescription: sarif.Message{Text: "An import nothing refers to."}}},
	})
	log.Runs[0].Results = append(log.Runs[0].Results, sarif.Result{
		RuleID:    "unused-import",
		RuleIndex: 0,
		Level:     sarif.Level("warning"),
		Message:   sarif.Message{Text: "import ScalarValues::* is unused"},
		Locations: sarif.LocationAt("models/a.sysml", 3, 5),
		Fixes: []sarif.Fix{{
			Description: sarif.Message{Text: "Remove the import"},
			ArtifactChanges: []sarif.ArtifactChange{{
				ArtifactLocation: sarif.ArtifactLocation{URI: "models/a.sysml"},
				Replacements: []sarif.Replacement{{
					DeletedRegion: sarif.Region{StartLine: 3, StartColumn: 5, EndLine: 3, EndColumn: 30},
				}},
			}},
		}},
	}, sarif.Result{
		RuleID:    "unused-import",
		Level:     sarif.Level("hint"),
		Message:   sarif.Message{Text: "whole-file note"},
		Locations: sarif.LocationAt("models/b.sysml", 0, 0),
	})
	data, err := sarif.Marshal(log)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, data)
	if !bytes.Contains(data, []byte(`"startLine": 1`)) || bytes.Contains(data, []byte(`"endLine": 0`)) {
		t.Fatalf("whole-file result not placed on line 1 without an end:\n%s", data)
	}
}

func TestLevel(t *testing.T) {
	for severity, want := range map[string]string{"error": "error", "warning": "warning", "info": "note", "hint": "note", "": "note"} {
		if got := sarif.Level(severity); got != want {
			t.Fatalf("Level(%q) = %q, want %q", severity, got, want)
		}
	}
}

func TestSortResultsIsDeterministic(t *testing.T) {
	results := []sarif.Result{
		{RuleID: "b", Locations: sarif.LocationAt("z.sysml", 2, 1)},
		{RuleID: "a", Locations: sarif.LocationAt("a.sysml", 9, 1)},
		{RuleID: "a", Locations: sarif.LocationAt("z.sysml", 2, 1)},
		{RuleID: "c", Locations: sarif.LocationAt("z.sysml", 1, 7)},
	}
	sarif.SortResults(results)
	var order []string
	for _, r := range results {
		order = append(order, r.Locations[0].PhysicalLocation.ArtifactLocation.URI+":"+r.RuleID)
	}
	want := []string{"a.sysml:a", "z.sysml:c", "z.sysml:a", "z.sysml:b"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// TestPathURIIsAURIReference: a path is percent-encoded into the URI reference
// SARIF requires of artifactLocation.uri, and a relative path stays relative.
func TestPathURIIsAURIReference(t *testing.T) {
	for path, want := range map[string]string{
		"":                       "",
		"models/a.sysml":         "models/a.sysml",
		"my dir/b&d #1 é.sysml":  "my%20dir/b&d%20%231%20%C3%A9.sysml",
		"50%/what?.sysml":        "50%25/what%3F.sysml",
		"a:b.sysml":              "./a:b.sysml",
		"/tmp/out side/a.sysml":  "/tmp/out%20side/a.sysml",
		"models/[draft]/a.sysml": "models/%5Bdraft%5D/a.sysml",
	} {
		got := sarif.PathURI(path)
		if got != want {
			t.Errorf("PathURI(%q) = %q, want %q", path, got, want)
		}
		parsed, err := url.Parse(got)
		if err != nil || strings.TrimPrefix(parsed.Path, "./") != path || parsed.Scheme != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			t.Errorf("PathURI(%q) = %q does not read back as that path: %+v, %v", path, got, parsed, err)
		}
	}
}
