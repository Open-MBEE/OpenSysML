package kpar_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// canonicalSysmod is the usage SysPI writes for MBSE4U/sysmod-sysmlv2 at 5.1.6.
const canonicalSysmod = "pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6"

// syspiProject copies the committed project laid out as SysPI installs one
// into a temporary directory, so a test may take its library away.
func syspiProject(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "syspi"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "app")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	return dst
}

// eliminationWarning is the warning the usage linked by elimination leaves on
// the consumer's .project.json: the same for every spelling of the repository.
func eliminationWarning(proj, resource, installed string) string {
	return filepath.Join(proj, project.ProjectFile) + ": usage " + resource + " names no installed project by publisher and name; linked by elimination to mbse4u/sysmod 5.1.6 (" + installed + "), the one unclaimed installed project that satisfies it; end the resource with the project's publisher and name (…/mbse4u/sysmod) to name it"
}

// A project written by SysPI — the usage an ECMA-427 package URL whose vcs_url
// qualifier carries the repository — resolves its import from lib/, linked by
// elimination as the pkg:git spelling is, and loses it once lib/ is gone.
func TestSysPIProjectResolvesItsImportFromLib(t *testing.T) {
	proj := syspiProject(t)
	installed := filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.6")
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Fatalf("errors:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Loaded, filepath.Join("lib", "mbse4u-sysmod_5.1.6", "SYSMOD.sysml")) {
		t.Errorf("loaded %q, want lib/mbse4u-sysmod_5.1.6/SYSMOD.sysml", report.Loaded)
	}
	if want := "warning: " + eliminationWarning(proj, canonicalSysmod, installed); !containsSubstring(report.Found, want) {
		t.Errorf("found %q, want %q", report.Found, want)
	}
	bin := sysmlBinary(t)
	out, err := run(t, bin, "-validate", proj)
	if err != nil {
		t.Fatalf("-validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, eliminationWarning(proj, canonicalSysmod, installed)) {
		t.Errorf("-validate wrote:\n%s\nwant the elimination warning", out)
	}

	if err := os.RemoveAll(filepath.Join(proj, "lib")); err != nil {
		t.Fatal(err)
	}
	out, err = run(t, bin, "-validate", proj)
	if err == nil {
		t.Fatalf("-validate without lib/ passed:\n%s", out)
	}
	if !strings.Contains(out, "unresolved reference: SYSMOD") {
		t.Errorf("-validate without lib/ wrote:\n%s\nwant SYSMOD unresolved", out)
	}
}

// The canonical spelling, the pkg:git alias and the bare vcs_url link the
// same installed project with the same warning but for the resource text.
func TestEverySpellingOfTheRepositoryLinksTheSameProject(t *testing.T) {
	for _, resource := range []string{canonicalSysmod, "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.6", "git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6"} {
		t.Run(resource, func(t *testing.T) {
			proj := syspiProject(t)
			installed := filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.6")
			if err := os.WriteFile(filepath.Join(proj, project.ProjectFile), []byte(`{"name": "app", "version": "0.1.0", "usage": [{"resource": "`+resource+`"}]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			s := repl.NewSession()
			report, err := s.LoadPathsReport([]string{proj})
			if err != nil {
				t.Fatal(err)
			}
			if report.Errors {
				t.Fatalf("errors:\n%s", strings.Join(report.Found, "\n"))
			}
			if !containsSubstring(report.Loaded, filepath.Join("lib", "mbse4u-sysmod_5.1.6", "SYSMOD.sysml")) {
				t.Errorf("loaded %q, want lib/mbse4u-sysmod_5.1.6/SYSMOD.sysml", report.Loaded)
			}
			if want := "warning: " + eliminationWarning(proj, resource, installed); !containsSubstring(report.Found, want) {
				t.Errorf("found %q, want %q", report.Found, want)
			}
		})
	}
}

// A #subpath in the vcs_url names a sub-package within the repository: the
// usage is answered by that project's own installation, by name.
func TestASubpathInTheVCSURLNamesTheSubPackage(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "proj")
	files := map[string]string{
		project.ProjectFile: `{"name": "app", "version": "0.1.0", "usage": [{"resource": "pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%401.0.0%23packages%2Fgears"}]}`,
		"App.sysml":         "package App {\n    private import Gears::*;\n    part g : Gear;\n}\n",
		"lib/acme-gears_1.0.0/" + project.ProjectFile:   `{"name": "gears", "publisher": "acme", "version": "1.0.0"}`,
		"lib/acme-gears_1.0.0/Gears.sysml":              "package Gears {\n    part def Gear;\n}\n",
		"lib/acme-widgets_1.0.0/" + project.ProjectFile: `{"name": "widgets", "publisher": "acme", "version": "1.0.0"}`,
		"lib/acme-widgets_1.0.0/Widgets.sysml":          "package Widgets {\n    part def Frame;\n}\n",
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
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors || len(report.Found) != 0 {
		t.Fatalf("found:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Loaded, "acme-gears_1.0.0") || containsSubstring(report.Loaded, "acme-widgets_1.0.0") {
		t.Errorf("loaded %q, want Gears alone: the fragment names the sub-package", report.Loaded)
	}
}

// A vcs_url that does not decode to a repository URL is a warning on the
// consumer's .project.json; the usage selects nothing and the import stays unresolved.
func TestAMalformedVCSURLIsAWarningAndLinksNothing(t *testing.T) {
	proj := syspiProject(t)
	resource := "pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com"
	if err := os.WriteFile(filepath.Join(proj, project.ProjectFile), []byte(`{"name": "app", "version": "0.1.0", "usage": [{"resource": "`+resource+`"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(proj, project.ProjectFile) + ": usage " + resource + `: the vcs_url qualifier "git+https://github.com" is not a <vcs>+<transport>://<host>/<path> URL (vcs: git, svn or hg) (a package URL of type generic names a repository as pkg:generic/<name>?vcs_url=<vcs>%2B<transport>:%2F%2F<host>%2F<path>[%40<version>][%23<subpath>], the qualifier percent-encoded)`
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Errors {
		t.Fatalf("no errors: SYSMOD must stay unresolved\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Found, "warning: "+want) {
		t.Errorf("found %q, want %q", report.Found, want)
	}
	if containsSubstring(report.Loaded, "mbse4u-sysmod_5.1.6") {
		t.Errorf("loaded %q: a usage that names no location selects nothing", report.Loaded)
	}
	bin := sysmlBinary(t)
	out, err := run(t, bin, "-validate", proj)
	if err == nil {
		t.Fatalf("-validate passed:\n%s", out)
	}
	if !strings.Contains(out, want) || !strings.Contains(out, "unresolved reference: SYSMOD") {
		t.Errorf("-validate wrote:\n%s\nwant %q and SYSMOD unresolved", out, want)
	}
}
