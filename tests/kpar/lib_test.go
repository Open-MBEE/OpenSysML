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

// vcsProject writes a consumer whose usage spells a repository path, as a
// version-control IRI does, with the project installed under lib/ by its own
// publisher and name — which the path does not spell.
func vcsProject(t *testing.T, resource string) string {
	t.Helper()
	proj := filepath.Join(t.TempDir(), "proj")
	files := map[string]string{
		project.ProjectFile:                              `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "` + resource + `"}]}`,
		"consumer/Consumer.sysml":                        "package Consumer {\n    private import SYSMOD::*;\n    part sys : SystemContext;\n}\n",
		"lib/mbse4u-sysmod_5.1.5/" + project.ProjectFile: `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5"}`,
		"lib/mbse4u-sysmod_5.1.5/SYSMOD.sysml":           "package SYSMOD {\n    part def SystemContext;\n}\n",
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

// A usage whose IRI spells a repository path rather than the project's
// publisher and name (pkg:git/github.com/MBSE4U/sysmod-sysmlv2 for mbse4u/sysmod)
// is linked to the one installed project no other usage claims.
func TestUsageSpellingARepositoryPathLinksToTheUnclaimedProject(t *testing.T) {
	for _, resource := range []string{"pkg:git/github.com/MBSE4U/sysmod-sysmlv2", "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.5"} {
		t.Run(resource, func(t *testing.T) {
			proj := vcsProject(t, resource)
			s := repl.NewSession()
			report, err := s.LoadPathsReport([]string{proj})
			if err != nil {
				t.Fatal(err)
			}
			if report.Errors {
				t.Fatalf("errors:\n%s", strings.Join(report.Found, "\n"))
			}
			if !containsSubstring(report.Loaded, filepath.Join("lib", "mbse4u-sysmod_5.1.5", "SYSMOD.sysml")) {
				t.Errorf("loaded %q, want lib/mbse4u-sysmod_5.1.5/SYSMOD.sysml", report.Loaded)
			}
			bin := sysmlBinary(t)
			if out, err := run(t, bin, "-validate", proj); err != nil {
				t.Fatalf("-validate: %v\n%s", err, out)
			}
		})
	}
}

// An unmatched usage nothing unclaimed answers leaves the project as it was:
// Gears, claimed by its own usage, is not offered to it.
func TestAnUnmatchedUsageDoesNotDisturbTheClaimedProjects(t *testing.T) {
	proj := libProject(t)
	pf := filepath.Join(proj, project.ProjectFile)
	if err := os.WriteFile(pf, []byte(`{"name": "Demo", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "pkg:registry/acme/gears@1.0.0"}, {"resource": "pkg:git/example.org/other/thing-repo"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors || len(report.Found) != 0 {
		t.Fatalf("found:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Loaded, "acme-gears_1.0.0") || containsSubstring(report.Loaded, "acme-gears_2.0.0") {
		t.Errorf("loaded %q, want 1.0.0 alone", report.Loaded)
	}
}

// A usage several unclaimed projects could answer is linked to none and
// reported once, naming each candidate.
func TestAUsageSeveralUnclaimedProjectsCouldAnswerIsReported(t *testing.T) {
	proj := vcsProject(t, "pkg:git/github.com/MBSE4U/sysmod-sysmlv2")
	widgets := filepath.Join(proj, "lib", "acme-widgets_1.0.0")
	if err := os.MkdirAll(widgets, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(widgets, project.ProjectFile), []byte(`{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Errors {
		t.Fatalf("no errors: SYSMOD must stay unresolved while the usage is ambiguous\n%s", strings.Join(report.Found, "\n"))
	}
	want := "warning: " + filepath.Join(proj, project.ProjectFile) + ": usage pkg:git/github.com/MBSE4U/sysmod-sysmlv2 names no installed project by publisher and name, and 2 unclaimed ones could answer it: acme/widgets 1.0.0 (" + widgets + "), mbse4u/sysmod 5.1.5 (" + filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5") + "); end the resource with the project's publisher and name (…/acme/widgets) to select one"
	if !containsSubstring(report.Found, want) {
		t.Errorf("found %q, want %q", report.Found, want)
	}
	bin := sysmlBinary(t)
	out, err := run(t, bin, "-validate", proj)
	if err == nil {
		t.Fatalf("-validate passed:\n%s", out)
	}
	if !strings.Contains(out, want) {
		t.Errorf("-validate wrote:\n%s\nwant %q", out, want)
	}
}

// A model that analyses cleanly beside an ambiguous usage still carries the
// warning: -warnings-as-errors ends the run on it, and -json reports it as data.
func TestAnAmbiguousUsageIsAWarningTheExitPolicyJudges(t *testing.T) {
	proj := vcsProject(t, "pkg:git/github.com/MBSE4U/sysmod-sysmlv2")
	widgets := filepath.Join(proj, "lib", "acme-widgets_1.0.0")
	if err := os.MkdirAll(widgets, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(widgets, project.ProjectFile), []byte(`{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "consumer", "Consumer.sysml"), []byte("package Consumer;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := sysmlBinary(t)
	want := filepath.Join(proj, project.ProjectFile) + ": usage pkg:git/github.com/MBSE4U/sysmod-sysmlv2 names no installed project by publisher and name"
	out, err := run(t, bin, "-validate", proj)
	if err != nil {
		t.Fatalf("-validate: %v\n%s", err, out)
	}
	if strings.Count(out, want) != 1 {
		t.Errorf("-validate wrote the warning %d times, want once:\n%s", strings.Count(out, want), out)
	}
	out, err = run(t, bin, "-warnings-as-errors", "-validate", proj)
	if err == nil {
		t.Fatalf("-warnings-as-errors -validate passed:\n%s", out)
	}
	if !strings.Contains(out, "warning: "+want) || !strings.Contains(out, "has warnings, which -warnings-as-errors judges errors") {
		t.Errorf("-warnings-as-errors -validate wrote:\n%s", out)
	}
	out, err = run(t, bin, "-json", "-validate", proj)
	if err != nil {
		t.Fatalf("-json -validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"severity": "warning"`) || !strings.Contains(out, `"code": "project.load"`) {
		t.Errorf("-json -validate reports no project.load warning:\n%s", out)
	}
	// Located at the project file, the warning is a finding of that file in
	// JUnit, as analysis's warnings are, not an error of the run.
	out, err = run(t, bin, "-format", "junit", "-validate", proj)
	if err != nil {
		t.Fatalf("-format junit -validate: %v\n%s", err, out)
	}
	if strings.Contains(out, "<error") || !strings.Contains(out, "project.load") {
		t.Errorf("-format junit -validate wrote:\n%s", out)
	}
	// A baseline that records the warning knows it: -warnings-as-errors passes
	// and the suppression is counted.
	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if out, err = run(t, bin, "-json", "-o", baseline, "-validate", proj); err != nil {
		t.Fatalf("-json -o: %v\n%s", err, out)
	}
	out, err = run(t, bin, "-baseline", baseline, "-warnings-as-errors", "-validate", proj)
	if err != nil {
		t.Fatalf("-baseline -warnings-as-errors -validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 known finding(s) suppressed by "+baseline) || strings.Contains(out, "warning: ") {
		t.Errorf("-baseline -warnings-as-errors -validate wrote:\n%s", out)
	}
}

// -quiet asks for errors alone: the warning that explains them is kept off the
// text, while the run still ends on the errors.
func TestQuietKeepsTheUsageWarningOffTheText(t *testing.T) {
	proj := vcsProject(t, "pkg:git/github.com/MBSE4U/sysmod-sysmlv2")
	widgets := filepath.Join(proj, "lib", "acme-widgets_1.0.0")
	if err := os.MkdirAll(widgets, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(widgets, project.ProjectFile), []byte(`{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := sysmlBinary(t)
	out, err := run(t, bin, "-quiet", "-validate", proj)
	if err == nil {
		t.Fatalf("-quiet -validate passed:\n%s", out)
	}
	if strings.Contains(out, "warning: ") || !strings.Contains(out, "unresolved reference: SYSMOD") {
		t.Errorf("-quiet -validate wrote:\n%s", out)
	}
}
