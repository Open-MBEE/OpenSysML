package kpar_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// writeTree writes the files of a project tree under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// subPackageProject writes a project using Widgets and its sub-package Gears,
// both installed as a package manager installs them: Widgets' tagged tree,
// which carries packages/gears with a project interchange file of its own,
// and Gears' tree beside it.
func subPackageProject(t *testing.T) string {
	t.Helper()
	proj := filepath.Join(t.TempDir(), "proj")
	writeTree(t, proj, map[string]string{
		project.ProjectFile: `{"name": "app", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/git.example.org/acme/widgets@1.0.0"},
  {"resource": "pkg:git/git.example.org/acme/widgets#packages/gears", "versionConstraint": "1.0.0"}]}`,
		"App.sysml": "package App {\n    private import Widgets::*;\n    private import Gears::*;\n    part f : Frame;\n    part g : Gear;\n}\n",
		".sysand/lib/acme-widgets_1.0.0/" + project.ProjectFile:                `{"name": "widgets", "publisher": "acme", "version": "1.0.0", "usage": [{"resource": "pkg:git/git.example.org/acme/widgets#packages/gears"}]}`,
		".sysand/lib/acme-widgets_1.0.0/Widgets.sysml":                         "package Widgets {\n    private import Gears::*;\n    part def Frame {\n        part g : Gear;\n    }\n}\n",
		".sysand/lib/acme-widgets_1.0.0/packages/gears/" + project.ProjectFile: `{"name": "gears", "publisher": "acme", "version": "1.0.0"}`,
		".sysand/lib/acme-widgets_1.0.0/packages/gears/Gears.sysml":            "package Gears {\n    part def Gear;\n}\n",
		".sysand/lib/acme-gears_1.0.0/" + project.ProjectFile:                  `{"name": "gears", "publisher": "acme", "version": "1.0.0"}`,
		".sysand/lib/acme-gears_1.0.0/Gears.sysml":                             "package Gears {\n    part def Gear;\n}\n",
	})
	return proj
}

// reopened reports whether the load said a package was opened by more than one file.
func reopened(report repl.LoadReport) bool {
	for _, lines := range [][]string{report.Loaded, report.Found, report.Declared} {
		if containsSubstring(lines, "opened by more than one loaded file") {
			return true
		}
	}
	return false
}

// A sub-package installed beside the parent whose tagged tree carries it loads
// once, through its own installation: the copy nested in the parent is a
// project of its own and not among the parent's files. Naming the project
// directory and naming the model file load the same files.
func TestSubPackageInstalledBesideItsParentLoadsOnce(t *testing.T) {
	proj := subPackageProject(t)
	for _, input := range []string{proj, filepath.Join(proj, "App.sysml")} {
		s := repl.NewSession()
		report, err := s.LoadPathsReport([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		if report.Errors {
			t.Errorf("%s: errors:\n%s", input, strings.Join(report.Found, "\n"))
		}
		if reopened(report) {
			t.Errorf("%s: Gears was loaded twice:\n%s", input, strings.Join(append(report.Loaded, report.Found...), "\n"))
		}
		if !containsSubstring(report.Loaded, "loaded 3 files:") || !containsSubstring(report.Loaded, filepath.Join("acme-gears_1.0.0", "Gears.sysml")) {
			t.Errorf("%s: loaded %q, want App, Widgets and the installed Gears", input, report.Loaded)
		}
		if containsSubstring(report.Loaded, filepath.Join("packages", "gears")) {
			t.Errorf("%s: loaded %q: the copy nested in Widgets is not Widgets' file", input, report.Loaded)
		}
	}
	bin := sysmlBinary(t)
	out, err := run(t, bin, "-validate", proj)
	if err != nil {
		t.Fatalf("-validate: %v\n%s", err, out)
	}
	if strings.Contains(out, "opened by more than one") {
		t.Errorf("-validate said Gears was loaded twice:\n%s", out)
	}
}

// Two installed projects that both open a package are a genuine duplicate,
// which the load still says so about.
func TestTwoInstalledProjectsOpeningOnePackageAreStillReported(t *testing.T) {
	proj := subPackageProject(t)
	writeTree(t, proj, map[string]string{
		project.ProjectFile: `{"name": "app", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/git.example.org/acme/widgets@1.0.0"},
  {"resource": "pkg:git/git.example.org/acme/widgets#packages/gears", "versionConstraint": "1.0.0"},
  {"resource": "pkg:git/git.example.org/acme/sprockets"}]}`,
		".sysand/lib/acme-sprockets_1.0.0/" + project.ProjectFile: `{"name": "sprockets", "publisher": "acme", "version": "1.0.0"}`,
		".sysand/lib/acme-sprockets_1.0.0/Sprockets.sysml":        "package Gears {\n    part def Gear;\n}\n",
	})
	for _, input := range []string{proj, filepath.Join(proj, "App.sysml")} {
		s := repl.NewSession()
		report, err := s.LoadPathsReport([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		if !reopened(report) {
			t.Errorf("%s: Gears opened by two installed projects was not reported:\n%s", input, strings.Join(append(append(report.Loaded, report.Found...), report.Declared...), "\n"))
		}
	}
}
