package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestCategorizePilot(t *testing.T) {
	cases := []struct {
		message string
		want    Category
	}{
		{"Couldn't resolve reference to Type 'Real'.", CategoryUnresolved},
		{"mismatched input 'import' expecting '}'", CategorySyntax},
		{"no viable alternative at input '::'", CategorySyntax},
		{"An attribute must be typed by attribute definitions.", CategoryKindMismatch},
		{"Subsetting/redefining feature should not have larger multiplicity upper bound", CategoryMultiplicity},
		{"Must have at least two related elements", CategoryMultiplicity},
		{"Duplicate of other owned member name", CategoryUnmapped},
		{"Must invoke a behavior or a behavioral feature", CategoryKindMismatch},
		// We word these two the same way the reference does, and map our copies
		// to kind-mismatch; mapping the reference's copies keeps the pair in one
		// category instead of reporting a divergence that does not exist.
		{"Must be an accessible feature (use dot notation for nesting)", CategoryKindMismatch},
		{"Must be model-level evaluable", CategoryKindMismatch},
		// Neither side maps this one: no category above describes a flow end.
		{"Cannot identify flow end (use dot notation)", CategoryUnmapped},
	}
	for _, c := range cases {
		if got := categorizePilot(c.message); got != c.want {
			t.Errorf("categorizePilot(%q) = %q, want %q", c.message, got, c.want)
		}
	}
}

func TestCategorizeOpenSysML(t *testing.T) {
	cases := []struct {
		code, pass, message string
		want                Category
	}{
		{"unresolved", "nameres", "unresolved reference: length", CategoryUnresolved},
		{"syntax", "syntax", `"on" is a reserved keyword`, CategorySyntax},
		{"subsetting-multiplicity", "type", "upper bound exceeds subsetted cap", CategoryMultiplicity},
		{"", "type", "incommensurable units mm and s", CategoryUnits},
		{"specialization-cycle", "type", "A participates in a specialization cycle", CategoryUnmapped},
		{"specialization-cycle", "type", "C3 participates in a specialization cycle", CategoryUnmapped},
		{"invocation-not-behavior", "type", "Must invoke a behavior or a behavioral feature", CategoryKindMismatch},
		{"bound-feature-types", "type", "Bound features should have conforming types", CategoryKindMismatch},
	}
	for _, c := range cases {
		if got := categorizeOpenSysML(c.code, c.pass, c.message); got != c.want {
			t.Errorf("categorizeOpenSysML(%q, %q, %q) = %q, want %q", c.code, c.pass, c.message, got, c.want)
		}
	}
}

// A tuple reported more often by one side stays a disagreement for the surplus.
func TestCompareFileBucketsMultisets(t *testing.T) {
	ours := []diagnostic{
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "a"},
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "b"},
		{Line: 9, Severity: "error", Category: CategoryMultiplicity, Message: "c"},
	}
	theirs := []diagnostic{
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "x"},
		{Line: 9, Severity: "warning", Category: CategoryMultiplicity, Message: "y"},
	}

	got := compareFile("m.sysml", ours, theirs)
	if len(got.Agreement) != 1 || got.Agreement[0].Count != 1 || got.Agreement[0].Line != 3 {
		t.Fatalf("agreement = %+v", got.Agreement)
	}
	if len(got.OpenSysMLOnly) != 1 || got.OpenSysMLOnly[0].Line != 3 {
		t.Fatalf("openSysMLOnly = %+v", got.OpenSysMLOnly)
	}
	// The line 9 pair matches on line and category but not severity.
	if len(got.PilotOnly) != 0 {
		t.Fatalf("pilotOnly = %+v", got.PilotOnly)
	}
	if len(got.SeverityMismatch) != 1 {
		t.Fatalf("severityMismatch = %+v", got.SeverityMismatch)
	}
	if sm := got.SeverityMismatch[0]; sm.Line != 9 || sm.OpenSysML != "error" || sm.Pilot != "warning" || sm.Count != 1 {
		t.Errorf("severityMismatch[0] = %+v", sm)
	}
}

// The downloaded OMG corpora live under examples/, which is a root of its own,
// so they must be compared once each rather than twice.
func TestDefaultRootsDoNotOverlap(t *testing.T) {
	dirs := map[string]string{}
	for _, root := range defaultRoots {
		if other, ok := dirs[root.Dir]; ok {
			t.Errorf("roots %s and %s share the directory %s", other, root.Name, root.Dir)
		}
		dirs[root.Dir] = root.Name
	}
	for _, root := range defaultRoots {
		for dir, name := range dirs {
			if name == root.Name || !strings.HasPrefix(dir, root.Dir+"/") {
				continue
			}
			nested := strings.TrimPrefix(dir, root.Dir+"/")
			if !slices.ContainsFunc(root.Skip, func(skip string) bool {
				return nested == skip || strings.HasPrefix(nested, skip+"/")
			}) {
				t.Errorf("root %s does not skip %s, which root %s compares", root.Name, nested, name)
			}
		}
	}
}

func TestCollectFilesSkipsNestedRoots(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{
		"examples/demo.sysml",
		"examples/pilot-corpora/sysml-examples/Model.sysml",
		"examples/pilot-corpora/kerml-examples/Model.kerml",
		"examples/sysml-v2-training/Training.sysml",
	} {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package P;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectFiles(repo, corpusRoot{
		Name: "examples", Dir: "examples",
		Skip: []string{"sysml-v2-training", "pilot-corpora"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"demo.sysml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("collectFiles() = %v, want %v", got, want)
	}
}

// F34: a root carries both languages, so collection is extension-agnostic and
// the language is decided per file.
func TestCollectFilesBothLanguages(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{
		"models/Example.kerml",
		"models/Example.sysml",
		"models/notes.md",
		"models/nested/Other.kerml",
	} {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package P;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectFiles(repo, corpusRoot{Name: "models", Dir: "models"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Example.kerml", "Example.sysml", "nested/Other.kerml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectFiles() = %v, want %v", got, want)
	}
}

// F34: each language is one batch, so the reference still resolves cross-file
// references within it, and SysML is compared first.
func TestBatchByLanguage(t *testing.T) {
	files := []string{"a.kerml", "b.sysml", "nested/c.kerml", "nested/d.sysml"}
	got := batchByLanguage(files)
	want := []languageBatch{
		{Kind: source.KindSysML, Files: []string{"b.sysml", "nested/d.sysml"}},
		{Kind: source.KindKerML, Files: []string{"a.kerml", "nested/c.kerml"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("batchByLanguage() = %+v, want %+v", got, want)
	}

	if got := batchByLanguage([]string{"only.sysml"}); len(got) != 1 || got[0].Kind != source.KindSysML {
		t.Errorf("batchByLanguage() = %+v, want the SysML batch alone", got)
	}
	if got := batchByLanguage([]string{"only.kerml"}); len(got) != 1 || got[0].Kind != source.KindKerML {
		t.Errorf("batchByLanguage() = %+v, want the KerML batch alone", got)
	}
}

func TestPilotLineParsing(t *testing.T) {
	tests := []struct {
		name, line, path, lineNo, severity, message string
	}{
		{
			name:     "SysML",
			line:     "Part Definition Example.sysml:12:3: warning: Bound features should have conforming types",
			path:     "Part Definition Example.sysml",
			lineNo:   "12",
			severity: "warning",
			message:  "Bound features should have conforming types",
		},
		{
			name:     "KerML nested path",
			line:     "Nested Folder/Example.kerml:7:4: error: unresolved reference",
			path:     "Nested Folder/Example.kerml",
			lineNo:   "7",
			severity: "error",
			message:  "unresolved reference",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match := pilotLine.FindStringSubmatch(test.line)
			if match == nil {
				t.Fatalf("no match for %q", test.line)
			}
			if match[1] != test.path || match[2] != test.lineNo || match[4] != test.severity {
				t.Errorf("match = %q", match[1:])
			}
			if match[5] != test.message {
				t.Errorf("message = %q", match[5])
			}
		})
	}
}

// F6: both validators are single-batch and report paths relative to --root.
func TestPilotDiagnosticsAttribution(t *testing.T) {
	repo := t.TempDir()
	rel := "Nested Space/Example.kerml"
	file := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := filepath.Join(repo, "bridge.sh")
	if err := os.WriteFile(bridge, []byte("#!/bin/sh\necho \"Nested Space/Example.kerml:7:4: error: unresolved reference\" >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := pilotDiagnostics(bridge, repo, ".", []string{rel}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[rel]) != 1 {
		t.Fatalf("diagnostics[%q] = %+v", rel, got[rel])
	}
	if got[rel][0].File != rel || got[rel][0].Line != 7 || got[rel][0].Message != "unresolved reference" {
		t.Errorf("diagnostics[%q] = %+v", rel, got[rel])
	}
}
