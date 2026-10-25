package kpar_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// libProject writes a project whose usage names Gears at 1.0.0 by the
// @version of its IRI, with 1.0.0 and 2.0.0 installed under a plain lib/.
func libProject(t *testing.T) string {
	t.Helper()
	proj := filepath.Join(t.TempDir(), "proj")
	files := map[string]string{
		project.ProjectFile: `{"name": "Demo", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "pkg:registry/acme/gears@1.0.0"}]}`,
		"main.sysml":        "package Demo {\n    private import Gears::*;\n    part g : Thing;\n}\n",
		"lib/acme-gears_1.0.0/" + project.ProjectFile: `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`,
		"lib/acme-gears_1.0.0/Gears.kerml":            "package Gears {\n    class Thing;\n}\n",
		"lib/acme-gears_2.0.0/" + project.ProjectFile: `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`,
		"lib/acme-gears_2.0.0/Gears.kerml":            "package Gears {\n    class Thing;\n    class Newer;\n}\n",
	}
	for name, content := range files {
		p := filepath.Join(proj, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return proj
}

// A project whose installed projects lie under lib/ beside .project.json
// loads them as one laid out under .sysand/lib does, the IRI's @version
// choosing among the installed versions.
func TestLibBesideTheProjectFileIsRead(t *testing.T) {
	proj := libProject(t)
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Fatalf("errors:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Loaded, filepath.Join("lib", "acme-gears_1.0.0", "Gears.kerml")) {
		t.Errorf("loaded %q, want lib/acme-gears_1.0.0/Gears.kerml", report.Loaded)
	}
	if containsSubstring(report.Loaded, "acme-gears_2.0.0") {
		t.Errorf("loaded %q: the IRI names 1.0.0, 2.0.0 must stay out", report.Loaded)
	}
	bin := sysmlBinary(t)
	if out, err := run(t, bin, "-validate", proj); err != nil {
		t.Fatalf("-validate: %v\n%s", err, out)
	}
}
