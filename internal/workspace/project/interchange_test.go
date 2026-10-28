package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeProject writes a project interchange file for a project with the given
// identity and usages into dir, which it creates.
func writeProject(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadInterchangeReadsTheSpecifiedProperties(t *testing.T) {
	dir := writeProject(t, filepath.Join(t.TempDir(), "p"), `{
  "name": "Requirements Management",
  "publisher": "ode4hera",
  "description": "Concepts for requirements management.",
  "version": "2.0.1",
  "license": "MIT",
  "maintainer": ["Someone <someone@example.org>"],
  "website": "https://example.org/",
  "topic": ["MBSE"],
  "usage": [
    {"resource": "https://www.omg.org/spec/KerML/20250201/Semantic-Library.kpar", "versionConstraint": "1.0.0"},
    {"resource": "pkg:sysand/mbse4u/sysmod"}
  ]
}`)
	got, err := ReadInterchange(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := &Interchange{
		Dir: dir, Name: "Requirements Management", Publisher: "ode4hera",
		Description: "Concepts for requirements management.", Version: "2.0.1", License: "MIT",
		Usage: []Usage{
			{Resource: "https://www.omg.org/spec/KerML/20250201/Semantic-Library.kpar", VersionConstraint: "1.0.0"},
			{Resource: "pkg:sysand/mbse4u/sysmod"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadInterchange = %+v, want %+v", got, want)
	}
}

func TestReadInterchangeRefusesWhatIsNotAProject(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadInterchange(dir); err == nil {
		t.Error("ReadInterchange of a directory with no project file = nil error")
	}
	if IsProjectDir(dir) {
		t.Error("IsProjectDir = true for a directory with no project file")
	}
	writeProject(t, dir, `{"version": "1.0.0"}`)
	if _, err := ReadInterchange(dir); err == nil {
		t.Error("ReadInterchange of a project with no name = nil error")
	}
	writeProject(t, dir, `[1, 2]`)
	if _, err := ReadInterchange(dir); err == nil {
		t.Error("ReadInterchange of a JSON array = nil error")
	}
	if !IsProjectDir(dir) {
		t.Error("IsProjectDir = false for a directory holding a project file")
	}
}

func TestEnclosingProjectsListsTheAncestorsNearestFirst(t *testing.T) {
	root := t.TempDir()
	outer := writeProject(t, filepath.Join(root, "proj"), `{"name": "outer", "version": "1"}`)
	inner := writeProject(t, filepath.Join(outer, ".sysand", "lib", "acme-dep_1.0.0"), `{"name": "dep", "version": "1"}`)
	file := writeModel(t, filepath.Join(inner, "examples", "a.sysml"), "package A;\n")

	if got := EnclosingProjects(file); !reflect.DeepEqual(got, []string{inner, outer}) {
		t.Errorf("EnclosingProjects(file) = %v, want [%s %s]", got, inner, outer)
	}
	if got := EnclosingProjects(inner); !reflect.DeepEqual(got, []string{inner, outer}) {
		t.Errorf("EnclosingProjects(dir) = %v, want [%s %s]", got, inner, outer)
	}
	if got := EnclosingProjects(filepath.Join(root, "elsewhere", "b.sysml")); len(got) != 0 {
		t.Errorf("EnclosingProjects outside every project = %v, want none", got)
	}
}

func TestUsesMatchesTheResourceByPublisherAndNameSlug(t *testing.T) {
	cases := []struct {
		publisher, name, resource string
		want                      bool
	}{
		{"ode4hera", "Requirements Management", "pkg:sysand/ode4hera/requirements-management", true},
		{"ode4hera", "Requirements Management", "pkg:sysand/ode4hera/requirements-management@2.0.1", true},
		{"esa", "CDF Reference Data Library", "pkg:sysand/esa/cdf-reference-data-library", true},
		{"hugoormo", "FiBO2SysMLv2", "pkg:sysand/hugoormo/fibo2sysmlv2", true},
		{"acme", "Vendored Parts", "https://example.org/projects/acme/vendored-parts", true},
		{"acme", "Vendored Parts", "https://example.org/projects/acme/vendored-parts/", true},
		{"acme", "Vendored Parts", "https://example.org/projects/acme/vendored%20parts.kpar?download=1#top", true},
		{"acme", "Vendored Parts", "https://example.org/projects/other/vendored-parts", false},
		{"acme", "Vendored Parts", "pkg:sysand/acme/vendored", false},
		{"acme", "Vendored Parts", "vendored-parts", false},
		{"", "Kernel Data Type Library", "https://www.omg.org/spec/KerML/20250201/Kernel-Data-Type-Library.kpar", true},
		{"", "Kernel Data Type Library", "https://www.omg.org/spec/KerML/20250201/Data-Type-Library.kpar", false},
		{"acme", "Widgets", "", false},
	}
	for _, c := range cases {
		p := &Interchange{Publisher: c.publisher, Name: c.name}
		if got := p.Uses(Usage{Resource: c.resource}); got != c.want {
			t.Errorf("%s/%s Uses(%q) = %v, want %v", c.publisher, c.name, c.resource, got, c.want)
		}
	}
}

// pick is pickInstalled over a usage whose constraint must parse.
func pick(t *testing.T, installed []*Interchange, u Usage) (*Interchange, []*Interchange) {
	t.Helper()
	r, err := u.versionRange()
	if err != nil {
		t.Fatalf("%q: %v", u.constraint(), err)
	}
	return pickInstalled(installed, u, r)
}

func TestPickInstalledTakesTheHighestVersionInTheRange(t *testing.T) {
	v1 := &Interchange{Dir: "v1", Publisher: "acme", Name: "gears", Version: "1.0.0"}
	v2 := &Interchange{Dir: "v2", Publisher: "acme", Name: "gears", Version: "2.0.0"}
	v10 := &Interchange{Dir: "v10", Publisher: "acme", Name: "gears", Version: "10.0.0"}
	other := &Interchange{Dir: "other", Publisher: "acme", Name: "other", Version: "99.0.0"}
	installed := []*Interchange{v1, v10, v2, other}

	for constraint, want := range map[string]*Interchange{"": v10, "=2.0.0": v2, "^1.0.0": v1, "~2.0": v2, ">=2.0.0 <11.0.0": v10, "1.x": v1, "1.0.0 - 2.0.0": v2, "1.0.0 || 2.0.0": v2, "*": v10} {
		if got, _ := pick(t, installed, Usage{Resource: "pkg:r/acme/gears", VersionConstraint: constraint}); got != want {
			t.Errorf("pick under %q = %v, want %v", constraint, got, want)
		}
	}
	got, named := pick(t, installed, Usage{Resource: "pkg:r/acme/gears", VersionConstraint: "^3.0.0"})
	if got != nil || len(named) != 3 {
		t.Errorf("pick under a range nothing satisfies = %v with %d named, want nil with the three versions named (no fallback to the latest)", got, len(named))
	}
	if got, named := pick(t, installed, Usage{Resource: "pkg:r/acme/missing"}); got != nil || named != nil {
		t.Errorf("pick of an uninstalled project = %v, %v, want nil, nil", got, named)
	}
	if _, err := (Usage{Resource: "pkg:r/acme/gears", VersionConstraint: "latest"}).versionRange(); !errors.Is(err, errBadConstraint) {
		t.Errorf("a constraint outside the grammar must not parse, got %v", err)
	}
}

// fixtureProject writes a project tree: the project at <root>/proj with a
// model importing Widgets and VendoredParts; Widgets installed under
// .sysand/lib, using Base, of which two versions are installed; VendoredParts
// installed under a separate store a library root names; and a hidden
// directory declaring Widgets too, which nothing must read.
func fixtureProject(t *testing.T) (proj, store string) {
	t.Helper()
	root := t.TempDir()
	proj = writeProject(t, filepath.Join(root, "proj"), `{"name": "demo", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "https://www.omg.org/spec/KerML/20250201/Data-Type-Library.kpar"},
  {"resource": "pkg:registry/acme/widgets"},
  {"resource": "https://example.org/projects/acme/vendored-parts"}]}`)
	writeModel(t, filepath.Join(proj, "model", "main.sysml"), "package Demo {\n    private import Widgets::*;\n    private import VendoredParts::*;\n    part w : Widget;\n}\n")
	lib := filepath.Join(proj, ".sysand", "lib")
	widgets := writeProject(t, filepath.Join(lib, "acme-widgets_1.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.0.0", "usage": [{"resource": "pkg:registry/acme/gears"}]}`)
	writeModel(t, filepath.Join(widgets, "Widgets.sysml"), "package Widgets {\n    private import Gears::*;\n    part def Widget :> Thing;\n}\n")
	gears1 := writeProject(t, filepath.Join(lib, "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	writeModel(t, filepath.Join(gears1, "Gears.kerml"), "package Gears {\n    class Thing;\n}\n")
	gears2 := writeProject(t, filepath.Join(lib, "acme-gears_2.0.0"), `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`)
	writeModel(t, filepath.Join(gears2, "Gears.kerml"), "package Gears {\n    class Thing;\n    class Newer;\n}\n")
	writeModel(t, filepath.Join(proj, ".unrelated", "shadow.sysml"), "package Widgets {\n    part def Shadow;\n}\n")
	store = filepath.Join(root, "store")
	vendored := writeProject(t, filepath.Join(store, "acme", "Vendored Parts"), `{"name": "Vendored Parts", "publisher": "acme", "version": "0.3.0"}`)
	writeModel(t, filepath.Join(vendored, "VendoredParts.sysml"), "package VendoredParts {\n    part def VendoredPart;\n}\n")
	return proj, store
}

func projectDirs(projects []*Interchange) []string {
	var out []string
	for _, p := range projects {
		out = append(out, p.Dir)
	}
	return out
}

func TestAroundFollowsTheUsagesThroughTheInstalledProjects(t *testing.T) {
	proj, store := fixtureProject(t)
	lib := filepath.Join(proj, ".sysand", "lib")

	got := Discovery{}.Around([]string{filepath.Join(proj, "model", "main.sysml")})
	if want := []string{proj}; !reflect.DeepEqual(projectDirs(got.Enclosing), want) {
		t.Errorf("Enclosing = %v, want %v", projectDirs(got.Enclosing), want)
	}
	// Widgets through the project's usage, then the latest Gears through
	// Widgets'; the standard-library archive and the uninstalled VendoredParts select nothing.
	if want := []string{filepath.Join(lib, "acme-widgets_1.0.0"), filepath.Join(lib, "acme-gears_2.0.0")}; !reflect.DeepEqual(projectDirs(got.Used), want) {
		t.Errorf("Used = %v, want %v", projectDirs(got.Used), want)
	}

	got = Discovery{Roots: []string{store}}.Around([]string{filepath.Join(proj, "model", "main.sysml")})
	want := []string{filepath.Join(lib, "acme-widgets_1.0.0"), filepath.Join(store, "acme", "Vendored Parts"), filepath.Join(lib, "acme-gears_2.0.0")}
	if !reflect.DeepEqual(projectDirs(got.Used), want) {
		t.Errorf("Used with a library root = %v, want %v", projectDirs(got.Used), want)
	}
}

func TestAroundFindsTheProjectsInstalledBesideAnInstalledOne(t *testing.T) {
	proj, _ := fixtureProject(t)
	lib := filepath.Join(proj, ".sysand", "lib")
	widgets := filepath.Join(lib, "acme-widgets_1.0.0")

	got := Discovery{}.Around([]string{filepath.Join(widgets, "Widgets.sysml")})
	if want := []string{widgets}; !reflect.DeepEqual(projectDirs(got.Enclosing), want) {
		t.Errorf("Enclosing = %v, want %v", projectDirs(got.Enclosing), want)
	}
	if want := []string{filepath.Join(lib, "acme-gears_2.0.0")}; !reflect.DeepEqual(projectDirs(got.Used), want) {
		t.Errorf("Used = %v, want %v: the usages are the installed project's own, found under the enclosing project's library", projectDirs(got.Used), want)
	}
}

func TestAroundOutsideAProjectOffersEveryProjectUnderTheRoots(t *testing.T) {
	_, store := fixtureProject(t)
	lone := writeModel(t, filepath.Join(t.TempDir(), "lone.sysml"), "package Lone;\n")

	if got := (Discovery{}).Around([]string{lone, "-"}); len(got.Enclosing) != 0 || len(got.Used) != 0 {
		t.Errorf("Around with no project and no roots = %+v, want nothing", got)
	}
	got := Discovery{Roots: []string{store, filepath.Join(store, "missing")}}.Around([]string{lone})
	if want := []string{filepath.Join(store, "acme", "Vendored Parts")}; !reflect.DeepEqual(projectDirs(got.Used), want) {
		t.Errorf("Used = %v, want %v", projectDirs(got.Used), want)
	}
}

func TestDependenciesComeFromTheProjectsTheProjectUses(t *testing.T) {
	proj, store := fixtureProject(t)
	lib := filepath.Join(proj, ".sysand", "lib")
	main := filepath.Join(proj, "model", "main.sysml")

	// Widgets answers the import, then Gears answers Widgets' own; the hidden
	// shadow and the older Gears are never read.
	got := Dependencies([]string{main}, nil)
	want := []string{filepath.Join(lib, "acme-widgets_1.0.0", "Widgets.sysml"), filepath.Join(lib, "acme-gears_2.0.0", "Gears.kerml")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}

	got = Discovery{Roots: []string{store}}.Dependencies([]string{main}, nil)
	want = []string{filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml"), want[0], want[1]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies with a library root = %v, want %v", got, want)
	}
}

func TestDependenciesOfAFileInAnInstalledProjectResolveThroughTheEnclosingLibrary(t *testing.T) {
	proj, _ := fixtureProject(t)
	lib := filepath.Join(proj, ".sysand", "lib")

	got := Dependencies([]string{filepath.Join(lib, "acme-widgets_1.0.0", "Widgets.sysml")}, nil)
	if want := []string{filepath.Join(lib, "acme-gears_2.0.0", "Gears.kerml")}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

func TestExpandOfAProjectDirectoryLoadsWhatItUses(t *testing.T) {
	proj, store := fixtureProject(t)
	lib := filepath.Join(proj, ".sysand", "lib")

	got, err := Discovery{Roots: []string{store}}.Expand([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(proj, "model", "main.sysml"),
		filepath.Join(lib, "acme-widgets_1.0.0", "Widgets.sysml"),
		filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml"),
		filepath.Join(lib, "acme-gears_2.0.0", "Gears.kerml"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expand = %v, want %v", got, want)
	}

	// A project directory with no files of its own stands for what it uses; a
	// plain directory still has to hold model files.
	empty := writeProject(t, filepath.Join(t.TempDir(), "empty"), `{"name": "empty", "version": "1", "usage": [{"resource": "pkg:r/acme/vendored-parts"}]}`)
	got, err = Discovery{Roots: []string{store}}.Expand([]string{empty})
	if err != nil || !reflect.DeepEqual(got, []string{filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml")}) {
		t.Errorf("Expand of a project with no files = %v, %v", got, err)
	}
	if _, err := Expand([]string{empty}); err == nil {
		t.Error("Expand of a project with no files and nothing installed = nil error")
	}
	if _, err := Expand([]string{t.TempDir()}); err == nil {
		t.Error("Expand of an empty plain directory = nil error")
	}
}

func TestSplitLibraryRootsDropsEmptyEntries(t *testing.T) {
	list := "a" + string(filepath.ListSeparator) + " " + string(filepath.ListSeparator) + "b c"
	if got := SplitLibraryRoots(list); !reflect.DeepEqual(got, []string{"a", "b c"}) {
		t.Errorf("SplitLibraryRoots = %v", got)
	}
	if got := SplitLibraryRoots(""); got != nil {
		t.Errorf("SplitLibraryRoots(\"\") = %v, want nil", got)
	}
	t.Setenv(LibraryRootsEnvVar, "x")
	if got := EnvLibraryRoots(); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("EnvLibraryRoots = %v", got)
	}
}

func TestNewerVersionFollowsSemanticVersioning(t *testing.T) {
	newer := [][2]string{
		{"2.0.0", "1.9.9"}, {"1.10.0", "1.9.0"}, {"1.0.0", "1.0.0-beta"}, {"1.0.0-rc.1", "1.0.0-beta.2"},
		{"1.0.0-beta.2", "1.0.0-beta.1"}, {"1.0.0-beta.11", "1.0.0-beta.2"}, {"1.0.0-beta", "1.0.0-alpha"},
		{"1.0.0-rc.1", "1.0.0-rc"}, {"1.0.0-beta", "1.0.0-1"}, {"v1.1", "1.0.5"}, {"1.0.0.1", "1.0.0"}, {"b", "a"},
		{"1.0.0-1000000000000000000000", "1.0.0-999999999999999999999"}, {"100000000000000000000.0", "99999999999999999999.0"}, {"1.0.0-010", "1.0.0-9"},
	}
	for _, c := range newer {
		if !newerVersion(c[0], c[1]) {
			t.Errorf("newerVersion(%q, %q) = false", c[0], c[1])
		}
		if newerVersion(c[1], c[0]) {
			t.Errorf("newerVersion(%q, %q) = true", c[1], c[0])
		}
	}
	for _, same := range [][2]string{{"1.0.0", "1.0"}, {"1.0.0+build.7", "1.0.0"}, {"v2.1.0", "2.1.0"}} {
		if newerVersion(same[0], same[1]) || newerVersion(same[1], same[0]) {
			t.Errorf("newerVersion orders %q and %q, want equal", same[0], same[1])
		}
	}
}

func TestUsesKeepsADotInTheNameAndStripsOnlyTheArchiveExtension(t *testing.T) {
	core := &Interchange{Publisher: "acme", Name: "Model Core"}
	if !core.Uses(Usage{Resource: "pkg:registry/acme/model.core"}) {
		t.Error("model.core does not match Model Core: a dot in a name is no extension")
	}
	if !core.Uses(Usage{Resource: "https://example.org/acme/model.core.KPAR"}) {
		t.Error("model.core.KPAR does not match Model Core: only the archive extension is stripped")
	}
	if core.Uses(Usage{Resource: "pkg:registry/acme/model.json"}) {
		t.Error("model.json matches Model Core")
	}
}

func TestExpandOfAProjectIncludesTheNotebooksOfTheProjectsItUses(t *testing.T) {
	proj, store := fixtureProject(t)
	notebook := writeModel(t, filepath.Join(store, "acme", "Vendored Parts", "Guide.ipynb"), "{}")

	got, err := Discovery{Roots: []string{store}}.Expand([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, path := range got {
		found = found || path == notebook
	}
	if !found {
		t.Errorf("Expand = %v, want the used project's notebook %s among them", got, notebook)
	}
	// The import-driven search still reads model files only.
	if deps := (Discovery{Roots: []string{store}}).Dependencies([]string{filepath.Join(proj, "model", "main.sysml")}, nil); len(deps) != 3 {
		t.Errorf("Dependencies = %v, want the three model files", deps)
	}
}

func TestInstalledProjectsFindTheProjectsUnderARootBeforeALinkedTreeSpendsTheBudget(t *testing.T) {
	store := t.TempDir()
	archive := t.TempDir()
	for i := 0; i <= maxLibraryDirs; i++ {
		if err := os.Mkdir(filepath.Join(archive, fmt.Sprintf("d%04d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(archive, filepath.Join(store, "00-archive")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	parts := writeProject(t, filepath.Join(store, "zz-parts"), `{"name": "Parts", "publisher": "acme", "version": "1.0.0"}`)
	if got, _ := installedProjects([]string{store}); len(got) != 1 || got[0].Dir != parts {
		t.Errorf("installedProjects = %v, want the project under the root though an earlier link leads to a tree larger than the budget", projectDirs(got))
	}
}

func TestInstalledProjectsFollowLinksOnceSoACycleTerminates(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "store")
	vendored := writeProject(t, filepath.Join(store, "nested", "acme-parts"), `{"name": "Parts", "publisher": "acme", "version": "1.0.0"}`)
	if err := os.Symlink(store, filepath.Join(store, "nested", "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(vendored, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	elsewhere := writeProject(t, filepath.Join(t.TempDir(), "acme-external"), `{"name": "External", "publisher": "acme", "version": "1.0.0"}`)
	if err := os.Symlink(elsewhere, filepath.Join(store, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got, _ := installedProjects([]string{store, filepath.Join(root, "linked")})
	want := []string{filepath.Join(store, "escape"), vendored}
	if !reflect.DeepEqual(projectDirs(got), want) {
		t.Errorf("installedProjects = %v, want %v: each project once though the store loops and a root links to one, and the project a link leaves the root for", projectDirs(got), want)
	}
	if got, _ := installedProjects([]string{filepath.Join(root, "linked")}); len(got) != 1 || got[0].Dir != filepath.Join(root, "linked") {
		t.Errorf("installedProjects of a root that is a project = %v, want the root itself", projectDirs(got))
	}
}

func TestPickInstalledHonoursTheVersionInTheResourceIRI(t *testing.T) {
	v1 := &Interchange{Dir: "v1", Publisher: "acme", Name: "gears", Version: "1.0.0"}
	v2 := &Interchange{Dir: "v2", Publisher: "acme", Name: "gears", Version: "2.0.0"}
	installed := []*Interchange{v1, v2}
	// A fragment names a sub-package ("@1.0.0#sub" would select a project named
	// sub); see TestUsesMatchesASubpathUsageToTheProjectTheFragmentNames.
	// Build metadata (+build) is outside semver precedence, so 1.0.0+build is 1.0.0.
	for _, resource := range []string{"pkg:r/acme/gears@1.0.0", "pkg:r/acme/gears@v1.0.0", "pkg:r/acme/gears@1.0.0/", "pkg:r/acme/gears@1.0.0?q=1", "pkg:r/acme/gears@1.0.0%2Bbuild"} {
		if got, _ := pick(t, installed, Usage{Resource: resource}); got != v1 {
			t.Errorf("%s picked %v, want 1.0.0", resource, got)
		}
	}
	if got, _ := pick(t, installed, Usage{Resource: "pkg:r/acme/gears@1.0.0", VersionConstraint: "2.0.0"}); got != v2 {
		t.Errorf("versionConstraint must win over the IRI's version; picked %v", got)
	}
	if got, named := pick(t, installed, Usage{Resource: "pkg:r/acme/gears@3.0.0"}); got != nil || len(named) != 2 {
		t.Errorf("an IRI version nothing satisfies selects nothing and names the installed versions; got %v, %v", got, named)
	}
	if got := resourceVersion("git@example.org:acme/gears"); got != "" {
		t.Errorf("a user@host is no version, got %q", got)
	}
}

// A plain lib/ beside .project.json is a well-known library directory, read
// like .sysand/lib; the IRI's @version selects among the versions there.
func TestAroundReadsLibBesideTheProjectFile(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "demo", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:registry/acme/gears@1.0.0"}]}`)
	writeModel(t, filepath.Join(proj, "main.sysml"), "package Demo {\n    private import Gears::*;\n    part g : Thing;\n}\n")
	gears1 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	writeModel(t, filepath.Join(gears1, "Gears.kerml"), "package Gears {\n    class Thing;\n}\n")
	gears2 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_2.0.0"), `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`)
	writeModel(t, filepath.Join(gears2, "Gears.kerml"), "package Gears {\n    class Thing;\n    class Newer;\n}\n")

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if got := projectDirs(found.Used); len(got) != 1 || got[0] != gears1 {
		t.Fatalf("used %v, want lib/acme-gears_1.0.0 alone", got)
	}
	if err := os.WriteFile(filepath.Join(proj, ProjectFile), []byte(`{"name": "demo", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "pkg:registry/acme/gears"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	found = Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if got := projectDirs(found.Used); len(got) != 1 || got[0] != gears2 {
		t.Fatalf("used %v, want the latest, lib/acme-gears_2.0.0", got)
	}
}

// A usage that names no installed project by publisher and name — a
// version-control IRI spells a repository's path — is linked by elimination to
// the one unclaimed project satisfying it, at its highest version.
func TestUsedLinksAnUnmatchedUsageToTheOneUnclaimedProject(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:registry/acme/gears@1.0.0"},
  {"resource": "pkg:git/github.com/MBSE4U/sysmod-sysmlv2"}]}`)
	gears1 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	gears2 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_2.0.0"), `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`)
	writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.0.0"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.0.0"}`)
	sysmod515 := writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5", "usage": [{"resource": "pkg:registry/acme/gears@2.0.0"}]}`)

	// Gears, both versions, is claimed by the first usage, so the second has
	// sysmod alone left, at its highest version; the usages of the project
	// linked by elimination are followed in turn, and the link is said.
	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if want := []string{gears1, sysmod515, gears2}; !reflect.DeepEqual(projectDirs(found.Used), want) {
		t.Fatalf("Used = %v, want %v", projectDirs(found.Used), want)
	}
	want := Warning{
		Path:    filepath.Join(proj, ProjectFile),
		Message: fmt.Sprintf("usage pkg:git/github.com/MBSE4U/sysmod-sysmlv2 names no installed project by publisher and name; linked by elimination to mbse4u/sysmod 5.1.5 (%s), the one unclaimed installed project that satisfies it; end the resource with the project's publisher and name (…/mbse4u/sysmod) to name it", sysmod515),
	}
	if !reflect.DeepEqual(found.Linking, []Warning{want}) {
		t.Fatalf("Linking = %q, want %q", found.Linking, want)
	}
}

func TestUsedLinksByEliminationWithinTheUsageConstraint(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.0.0"}]}`)
	sysmod500 := writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.0.0"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.0.0"}`)
	writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5"}`)
	writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`)

	// Only 5.0.0 satisfies @5.0.0, so Widgets does not make the usage ambiguous.
	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if got := projectDirs(found.Used); !reflect.DeepEqual(got, []string{sysmod500}) {
		t.Fatalf("Used = %v, want sysmod 5.0.0 alone", got)
	}
	if len(found.Linking) != 1 || !strings.Contains(found.Linking[0].Message, "linked by elimination to mbse4u/sysmod 5.0.0 ("+sysmod500+")") {
		t.Fatalf("Linking = %q, want the elimination link to 5.0.0 said once", found.Linking)
	}

	// A constraint nothing satisfies links nothing and says nothing.
	writeProject(t, proj, `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@6.0.0"}]}`)
	found = Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if len(found.Used) != 0 || len(found.Linking) != 0 {
		t.Fatalf("Used = %v, Linking = %v, want nothing", projectDirs(found.Used), found.Linking)
	}
}

func TestUsedLeavesAUsageSeveralUnclaimedProjectsCouldAnswerUnlinked(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/github.com/MBSE4U/sysmod-sysmlv2"}]}`)
	sysmod := writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5"}`)
	widgets := writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`)

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if len(found.Used) != 0 {
		t.Fatalf("Used = %v, want nothing: the usage is ambiguous", projectDirs(found.Used))
	}
	want := Warning{
		Path:    filepath.Join(proj, ProjectFile),
		Message: fmt.Sprintf("usage pkg:git/github.com/MBSE4U/sysmod-sysmlv2 names no installed project by publisher and name, and 2 unclaimed ones could answer it: acme/widgets 1.0.0 (%s), mbse4u/sysmod 5.1.5 (%s); end the resource with the project's publisher and name (…/acme/widgets) to select one", widgets, sysmod),
	}
	if !reflect.DeepEqual(found.Linking, []Warning{want}) {
		t.Fatalf("Linking = %q, want %q", found.Linking, want)
	}
}

// An installed project loaded alongside its consumer still answers the usage by elimination,
// said once, with its own usages followed; loaded alone it is not offered to itself.
func TestAroundSaysTheEliminationLinkWhenTheLinkedProjectIsLoaded(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/github.com/MBSE4U/sysmod-sysmlv2"}]}`)
	sysmod := writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5", "usage": [{"resource": "pkg:registry/acme/gears"}, {"resource": "pkg:git/example.org/other/thing-repo"}]}`)
	gears := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml"), filepath.Join(sysmod, "SYSMOD.sysml")})
	if got := projectDirs(found.Used); !reflect.DeepEqual(got, []string{gears}) {
		t.Fatalf("Used = %v, want Gears, followed from sysmod's usage", got)
	}
	if len(found.Linking) != 1 || !strings.Contains(found.Linking[0].Message, "linked by elimination to mbse4u/sysmod 5.1.5 ("+sysmod+")") {
		t.Fatalf("Linking = %q, want the consumer's elimination link said once", found.Linking)
	}
	found = Discovery{}.Around([]string{filepath.Join(sysmod, "SYSMOD.sysml")})
	if got := projectDirs(found.Used); !reflect.DeepEqual(got, []string{gears}) || len(found.Linking) != 0 {
		t.Fatalf("Used = %v, Linking = %q, want Gears alone and nothing said: sysmod does not answer its own usage", got, found.Linking)
	}
}

// The standard library's usages are the bundled library's to answer: one left
// unmatched does not take the one unrelated project installed.
func TestUsedLeavesTheStandardLibrarysUsagesToTheBundledLibrary(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "https://www.omg.org/spec/KerML/20250201/Data-Type-Library.kpar", "versionConstraint": "1.0.0"},
  {"resource": "https://www.omg.org/spec/SysML/20250201/Systems-Library.kpar"}]}`)
	writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`)

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if len(found.Used) != 0 || len(found.Linking) != 0 {
		t.Fatalf("Used = %v, Linking = %v, want nothing", projectDirs(found.Used), found.Linking)
	}
}

// Two usages of one repository at two versions: the project the first is
// linked to by elimination answers the second at its own version, whichever
// is written first.
func TestUsedLinksEachVersionOfARepositoryOnceItsProjectIsLearnt(t *testing.T) {
	for _, order := range [][2]string{{"1.0.0", "2.0.0"}, {"2.0.0", "1.0.0"}} {
		t.Run(order[0]+" first", func(t *testing.T) {
			root := t.TempDir()
			proj := writeProject(t, filepath.Join(root, "proj"), fmt.Sprintf(`{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/example.org/acme/gears-repo@%s"},
  {"resource": "pkg:git/example.org/acme/gears-repo@%s"}]}`, order[0], order[1]))
			gears1 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
			gears2 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_2.0.0"), `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`)
			writeProject(t, filepath.Join(proj, "lib", "acme-widgets_3.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "3.0.0"}`)

			found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
			want := []string{gears1, gears2}
			if order[0] == "2.0.0" {
				want = []string{gears2, gears1}
			}
			if !reflect.DeepEqual(projectDirs(found.Used), want) {
				t.Fatalf("Used = %v, want %v", projectDirs(found.Used), want)
			}
			// The repository's project is learnt from the first link: the
			// elimination is said once, the second version takes it silently.
			if len(found.Linking) != 1 || !strings.Contains(found.Linking[0].Message, "linked by elimination to acme/gears "+order[0]) {
				t.Fatalf("Linking = %q, want the elimination link to %s said once", found.Linking, order[0])
			}
		})
	}
}

// A deferred usage whose candidates later usages all claim selects nothing and
// says nothing, rather than failing on an emptied candidate list; the two links are said.
func TestUsedSurvivesADeferredUsageWhoseCandidatesAreAllClaimedLater(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/example.org/acme/either-repo"},
  {"resource": "pkg:git/example.org/acme/gears-repo@1.0.0"},
  {"resource": "pkg:git/example.org/acme/widgets-repo@2.0.0"}]}`)
	gears := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	widgets := writeProject(t, filepath.Join(proj, "lib", "acme-widgets_2.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "2.0.0"}`)

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
	if want := []string{gears, widgets}; !reflect.DeepEqual(projectDirs(found.Used), want) {
		t.Fatalf("Used = %v, want %v", projectDirs(found.Used), want)
	}
	if len(found.Linking) != 2 || !strings.Contains(found.Linking[0].Message, "linked by elimination to acme/gears 1.0.0") || !strings.Contains(found.Linking[1].Message, "linked by elimination to acme/widgets 2.0.0") {
		t.Fatalf("Linking = %q, want the two elimination links alone: both candidates went to the usages that name them", found.Linking)
	}
}

// A usage that names a project is matched by name alone: with that project
// missing, the one unrelated installed project is not taken in its place.
func TestUsedLeavesAMissingNamedUsageUnresolved(t *testing.T) {
	for _, resource := range []string{"pkg:registry/acme/gears", "pkg:sysand/acme/gears@1.0.0", "pkg:npm/acme/gears"} {
		t.Run(resource, func(t *testing.T) {
			root := t.TempDir()
			proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "`+resource+`"}]}`)
			writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`)

			found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
			if len(found.Used) != 0 || len(found.Linking) != 0 {
				t.Fatalf("Used = %v, Linking = %v, want nothing: the usage names a project, not a location", projectDirs(found.Used), found.Linking)
			}
		})
	}
}

func TestSpellsLocation(t *testing.T) {
	for resource, want := range map[string]bool{
		"pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.5": true,
		"pkg:svn/example.org/repo/trunk":                 true,
		"pkg:hg/example.org/repo":                        true,
		"https://example.org/acme/gears.git":             true,
		"ssh://git@example.org/acme/gears.git@1.0.0":     true,
		"git@example.org:acme/gears.git":                 true,
		"file:///srv/projects/gears":                     true,
		"pkg:registry/acme/gears":                        false,
		"pkg:sysand/acme/gears@1.0.0":                    false,
		"pkg:github/acme/gears":                          false,
		"acme/gears":                                     false,
	} {
		if got := (Usage{Resource: resource}).spellsLocation(); got != want {
			t.Errorf("spellsLocation(%s) = %v, want %v", resource, got, want)
		}
	}
}

// A versionConstraint is an npm-style semver range on both paths: the project a
// usage names (slug match) and the one it reaches by elimination.
func TestUsedHonoursARangeConstraintOnBothPaths(t *testing.T) {
	cases := map[string]string{"^1.0.0": "1.1.0", ">=1.0.0": "2.0.0", "~1.1.0": "1.1.0", "1.1.x": "1.1.0", ">=1.0.0 <2.0.0": "1.1.0", "1.0.0 - 1.1.0": "1.1.0", "1.0.0 || 2.0.0": "2.0.0", "=1.1.0": "1.1.0", "v2.0.0": "2.0.0", "<1.0.0": "1.0.0-rc.1", ">=1.0.0-rc.1 <1.0.0": "1.0.0-rc.1", "": "2.0.0"}
	for _, path := range []string{"pkg:sysand/acme/widgets", "pkg:hg/hg.example.org/models/hg-widgets"} {
		for constraint, want := range cases {
			t.Run(path+" "+constraint, func(t *testing.T) {
				root := t.TempDir()
				proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "`+path+`", "versionConstraint": "`+constraint+`"}]}`)
				var dirs []string
				for _, v := range []string{"1.0.0-rc.1", "1.0.0", "1.1.0", "2.0.0"} {
					dirs = append(dirs, writeProject(t, filepath.Join(proj, "lib", "acme-widgets_"+v), `{"name": "Widgets", "publisher": "acme", "version": "`+v+`"}`))
				}
				found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
				if got := projectDirs(found.Used); len(got) != 1 || !strings.HasSuffix(got[0], "acme-widgets_"+want) {
					t.Fatalf("Used = %v, want acme-widgets_%s alone (of %d installed)", got, want, len(dirs))
				}
				// The elimination path says its link; the name match says nothing.
				if said := strings.HasPrefix(path, "pkg:hg/"); len(found.Linking) != map[bool]int{true: 1, false: 0}[said] || (said && !strings.Contains(found.Linking[0].Message, "linked by elimination to acme/widgets "+want)) {
					t.Fatalf("Linking = %q, want %v", found.Linking, map[bool]string{true: "the elimination link said once", false: "nothing"}[said])
				}
			})
		}
	}
}

// A constraint no installed version satisfies, or one outside the grammar, is
// a warning at the consumer's .project.json and links nothing — never "any".
func TestUsedReportsAnUnsatisfiedOrMalformedConstraint(t *testing.T) {
	for _, path := range []string{"pkg:sysand/acme/widgets", "pkg:hg/hg.example.org/models/hg-widgets"} {
		for constraint, want := range map[string]string{"^3.0.0": "none of the installed versions satisfies: 1.1.0 (", "latest": "malformed version constraint", "1.x.3": "malformed version constraint"} {
			t.Run(path+" "+constraint, func(t *testing.T) {
				root := t.TempDir()
				proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "`+path+`", "versionConstraint": "`+constraint+`"}]}`)
				writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.1.0"), `{"name": "Widgets", "publisher": "acme", "version": "1.1.0"}`)
				found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml")})
				if len(found.Used) != 0 {
					t.Fatalf("Used = %v, want nothing", projectDirs(found.Used))
				}
				if constraint == "^3.0.0" && strings.HasPrefix(path, "pkg:hg/") {
					// By elimination nothing in range is a candidate: unresolved, as for any usage nothing answers.
					if len(found.Linking) != 0 {
						t.Fatalf("Linking = %v, want nothing", found.Linking)
					}
					return
				}
				if len(found.Linking) != 1 || found.Linking[0].Path != filepath.Join(proj, ProjectFile) || !strings.Contains(found.Linking[0].Message, want) {
					t.Fatalf("Linking = %v, want one at %s containing %q", found.Linking, ProjectFile, want)
				}
			})
		}
	}
}

// A claim that enqueues nothing — the claimed project is an input path too, so
// already selected — still narrows a deferred usage to its one remaining candidate.
func TestUsedRevisitsADeferredUsageAfterAClaimOfASelectedProject(t *testing.T) {
	root := t.TempDir()
	proj := writeProject(t, filepath.Join(root, "proj"), `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/example.org/acme/either-repo"},
  {"resource": "pkg:git/example.org/acme/gears-repo@1.0.0"}]}`)
	gears := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	widgets := writeProject(t, filepath.Join(proj, "lib", "acme-widgets_2.0.0"), `{"name": "Widgets", "publisher": "acme", "version": "2.0.0"}`)

	found := Discovery{}.Around([]string{filepath.Join(proj, "main.sysml"), filepath.Join(gears, "Gears.sysml")})
	if want := []string{widgets}; !reflect.DeepEqual(projectDirs(found.Used), want) {
		t.Fatalf("Used = %v, want %v (Gears is enclosing already)", projectDirs(found.Used), want)
	}
	if len(found.Linking) != 2 || !strings.Contains(found.Linking[0].Message, "linked by elimination to acme/gears 1.0.0") || !strings.Contains(found.Linking[1].Message, "linked by elimination to acme/widgets 2.0.0") {
		t.Fatalf("Linking = %q, want the two elimination links, no ambiguity", found.Linking)
	}
}
