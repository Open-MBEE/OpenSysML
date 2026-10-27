// Package kpar_test reads the committed project interchange archives (KerML
// 1.0 §10.3) through every path a project directory is read: as a named
// input, as an installed project under a library root, and as the library
// that stands in for the bundled one. See testdata/README.md for the pins.
package kpar_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// omgArchives lists the OMG release archives under testdata/omg.
func omgArchives(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "omg", "*"+project.ArchiveExt))
	if err != nil || len(paths) != 10 {
		t.Fatalf("testdata/omg holds %d archives (err %v), want the 10 of the release", len(paths), err)
	}
	return paths
}

// sysandProject is the committed project laid out as sysand installs one.
func sysandProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "sysand"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOMGArchivesAreValidProjectInterchangeArchives(t *testing.T) {
	for _, path := range omgArchives(t) {
		a, err := project.OpenArchive(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if a.Project.Name == "" || a.Project.Version == "" || a.Meta.Created == "" || len(a.Meta.Index) == 0 || len(a.Meta.Checksum) == 0 {
			t.Errorf("%s: name %q version %q created %q index %d checksums %d", path, a.Project.Name, a.Project.Version, a.Meta.Created, len(a.Meta.Index), len(a.Meta.Checksum))
		}
		if len(a.Notes) != 0 {
			t.Errorf("%s: notes %q, want none: every checksum of the release is verifiable", path, a.Notes)
		}
		if files := a.Project.Files(); len(files) == 0 {
			t.Errorf("%s: no model files", path)
		}
	}
}

// omittingPrefix is a library source without the files under one directory.
type omittingPrefix struct {
	libs.Source
	prefix string
}

func (s omittingPrefix) List() []string {
	var out []string
	for _, name := range s.Source.List() {
		if !strings.HasPrefix(name, s.prefix) {
			out = append(out, name)
		}
	}
	return out
}

// The OMG archives hold the published text of the release the bundled library
// embeds, so indexing them in its place yields the same qualified names as the
// published files alone. The bundled source adds this project's extension
// libraries (under "OpenSysML Libraries/") and the declared errata, neither of
// which is in the release, so those are left out of the comparison.
func TestOMGArchivesIndexTheBundledLibrary(t *testing.T) {
	fromArchives, _ := libs.FrozenLibraryFrom(libs.NewArchiveSource(omgArchives(t)...))
	fromEmbedded, _ := libs.FrozenLibraryFrom(omittingPrefix{libs.EmbeddedSource(), "OpenSysML Libraries/"})

	got, want := fromArchives.FQNs(), fromEmbedded.FQNs()
	sort.Strings(got)
	sort.Strings(want)
	if len(got) < 1000 {
		t.Fatalf("archive index holds %d qualified names; the standard library has thousands", len(got))
	}
	if diff := setDiff(got, want); diff != "" {
		t.Errorf("qualified names differ between the OMG archives and the embedded release:\n%s", diff)
	}
	docs := fromArchives.Documents()
	if len(docs) != 94 {
		t.Errorf("archive index holds %d documents, want the 94 model files of the release", len(docs))
	}
	// Each archived file carries the tier of the library its archive is,
	// which is the tier the embedded copy of the same file has.
	tiers := map[string]symbols.LibraryTier{}
	for _, doc := range fromEmbedded.Documents() {
		tiers[filepath.Base(doc)] = fromEmbedded.DocumentLibraryTier(doc)
	}
	for _, doc := range docs {
		if got, want := fromArchives.DocumentLibraryTier(doc), tiers[filepath.Base(doc)]; got != want {
			t.Errorf("%s: tier %v, want %v as the embedded copy has", doc, got, want)
		}
	}
	for doc, want := range map[string]symbols.LibraryTier{
		"Kernel_Semantic_Library-1.1.0-dev.20260901.kpar/Base.kerml":          symbols.TierKernelSemantic,
		"SysML_Systems_Library-2.1.0-dev.20260901.kpar/Items.sysml":           symbols.TierSystems,
		"SysML_Quantities_and_Units_Library-2.1.0-dev.20260901.kpar/SI.sysml": symbols.TierDomain,
	} {
		if got := fromArchives.DocumentLibraryTier(filepath.FromSlash(doc)); got != want {
			t.Errorf("%s: tier %v, want %v", doc, got, want)
		}
	}
}

// setDiff lists the names only one of two sorted lists holds.
func setDiff(got, want []string) string {
	var b strings.Builder
	i, j := 0, 0
	for i < len(got) || j < len(want) {
		switch {
		case j == len(want) || (i < len(got) && got[i] < want[j]):
			b.WriteString("+ " + got[i] + "\n")
			i++
		case i == len(got) || want[j] < got[i]:
			b.WriteString("- " + want[j] + "\n")
			j++
		default:
			i++
			j++
		}
	}
	return b.String()
}

// Named on the command line, an archive loads its model files, members
// named under the archive's path.
func TestArchiveNamedAsInputLoadsItsFiles(t *testing.T) {
	archive := filepath.Join(sysandProject(t), ".sysand", "lib", "mbse4u-sysmod_5.1.4"+project.ArchiveExt)
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{archive})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Errorf("errors loading %s:\n%s", archive, strings.Join(report.Found, "\n"))
	}
	for _, line := range report.Found {
		if strings.Contains(line, "warning") {
			t.Errorf("warning loading %s: %s", archive, line)
		}
	}
	for _, member := range []string{"SYSMOD.sysml", "SYSMOD4AI.sysml", "SYSMODRequirementsBoilerPlates.sysml"} {
		if !containsSubstring(report.Loaded, archive+"/"+member) {
			t.Errorf("loaded %q, want %s", report.Loaded, member)
		}
	}
	if containsSubstring(report.Loaded, "README.md") {
		t.Errorf("loaded %q: the archive's README is not a model file", report.Loaded)
	}
}

// A pattern that matches archives loads them as naming them would, and
// LoadFile, the single-path load, expands an archive too.
func TestArchivesMatchedByAPatternLoad(t *testing.T) {
	store := filepath.Join(sysandProject(t), ".sysand", "lib")
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{filepath.Join(store, "*"+project.ArchiveExt)})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors || !containsSubstring(report.Loaded, project.ArchiveExt+"/SYSMOD.sysml") {
		t.Errorf("errors %v, loaded %q", report.Errors, report.Loaded)
	}
	lines, err := repl.NewSession().LoadFile(filepath.Join(store, "mbse4u-sysmod_5.1.4"+project.ArchiveExt))
	if err != nil {
		t.Fatal(err)
	}
	if !containsSubstring(lines, "SYSMOD4AI") {
		t.Errorf("LoadFile of the archive reported %q, want its packages", lines)
	}
	summary, err := repl.NewSession().LoadFilesSummary([]string{filepath.Join(store, "mbse4u-sysmod_5.1.4"+project.ArchiveExt)})
	if err != nil {
		t.Fatal(err)
	}
	if !containsSubstring(summary, "SYSMOD4AI") {
		t.Errorf("LoadFilesSummary of the archive reported %q, want its packages", summary)
	}
	// The archive beside one of its own members, and beside itself, is one load.
	archive := filepath.Join(store, "mbse4u-sysmod_5.1.4"+project.ArchiveExt)
	overlap, err := repl.NewSession().LoadFilesSummary([]string{archive, project.MemberPath(archive, "SYSMOD.sysml"), archive})
	if err != nil {
		t.Fatal(err)
	}
	if len(overlap) != len(summary) {
		t.Errorf("summary of the archive with its member named too has %d lines, the archive alone %d:\n%s", len(overlap), len(summary), strings.Join(overlap, "\n"))
	}
}

func containsSubstring(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

// A sysand project's usage closure reaches an installed project directory and,
// through it, an installed archive, all under the well-known .sysand/lib. The
// installed Requirements Management 2.0.1 ships an example that imports
// SYSMOD::System, which sysmod 5.1.4 (the version its constraint selects)
// does not declare; those two references are the only diagnostics.
func TestSysandProjectLoadsItsInstalledArchives(t *testing.T) {
	proj := sysandProject(t)
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range report.Found {
		if !strings.Contains(line, "error:") && !strings.Contains(line, "warning") {
			continue
		}
		if !strings.Contains(line, "examples/01_requirement_attributes.sysml") || !strings.Contains(line, "System") {
			t.Errorf("unexpected diagnostic loading %s: %s", proj, line)
		}
	}
	if !containsSubstring(report.Loaded, "RequirementsManagement.sysml") {
		t.Errorf("loaded %q, want the installed directory's files", report.Loaded)
	}
	if !containsSubstring(report.Loaded, "mbse4u-sysmod_5.1.4"+project.ArchiveExt+"/SYSMOD.sysml") {
		t.Errorf("loaded %q, want the installed archive's members", report.Loaded)
	}
}

// A model outside any project finds an installed archive through a library
// root: the import names a package only the archive declares.
func TestArchiveUnderLibraryRootAnswersAnImport(t *testing.T) {
	store := filepath.Join(sysandProject(t), ".sysand", "lib")
	a, err := project.OpenArchive(filepath.Join(store, "mbse4u-sysmod_5.1.4"+project.ArchiveExt))
	if err != nil {
		t.Fatal(err)
	}
	var pkg string
	for name := range a.Meta.Index {
		if pkg == "" || name < pkg {
			pkg = name
		}
	}
	if pkg == "" {
		t.Fatal("the archive's .meta.json index is empty")
	}
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte("package Probe {\n\tprivate import "+pkg+"::*;\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	load := func(roots []string) repl.LoadReport {
		s := repl.NewSession()
		s.SetLibraryRoots(roots)
		report, err := s.LoadPathsReport([]string{model})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := load(nil); !report.Errors {
		t.Fatalf("import of %s resolved with no library root; the probe proves nothing", pkg)
	}
	report := load([]string{store})
	if report.Errors {
		t.Errorf("errors with the store as a library root:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Loaded, project.ArchiveExt+"/") {
		t.Errorf("loaded %q, want the archive's members pulled in by the import", report.Loaded)
	}
}

// A file that is not an archive under a root is a warning on its path, and
// the model still loads.
func TestMalformedArchiveUnderLibraryRootIsAWarning(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken"+project.ArchiveExt)
	if err := os.WriteFile(bad, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte("package Probe { part def P; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := repl.NewSession()
	s.SetLibraryRoots([]string{dir})
	report, err := s.LoadPathsReport([]string{model})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Errorf("errors:\n%s", strings.Join(report.Found, "\n"))
	}
	if !containsSubstring(report.Found, bad+": not a project interchange archive") {
		t.Errorf("found %q, want a warning on %s", report.Found, bad)
	}
}

// sysmlBinary builds cmd/sysml once for the CLI tests.
func sysmlBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "sysml")
	build := exec.Command("go", gobuild.Args(out)...)
	build.Dir = filepath.Join("..", "..", "cmd", "sysml")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return out
}

func run(t *testing.T, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "OPENSYSML_LIBRARY_ROOTS=")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// -bundle writes the same bytes on every run, -unbundle writes back the files the archive records
// (those the .meta.json names, not every file), and the two refuse to overwrite or to run with a model.
func TestBundleIsDeterministicAndUnbundleReproducesTheRecordedFiles(t *testing.T) {
	bin := sysmlBinary(t)
	src := filepath.Join(sysandProject(t), ".sysand", "lib", "ode4hera-requirements-management_2.0.1")
	work := t.TempDir()
	first, second := filepath.Join(work, "a"+project.ArchiveExt), filepath.Join(work, "b"+project.ArchiveExt)
	for _, out := range []string{first, second} {
		if output, err := run(t, bin, "-bundle", src, "-o", out); err != nil {
			t.Fatalf("-bundle: %v\n%s", err, output)
		}
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two -bundle runs differ (%d and %d bytes)", len(a), len(b))
	}

	dir := filepath.Join(work, "out")
	if output, err := run(t, bin, "-unbundle", first, "-o", dir); err != nil {
		t.Fatalf("-unbundle: %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-unbundle", first, "-o", dir); err == nil {
		t.Errorf("-unbundle into a directory that holds files succeeded:\n%s", output)
	}
	compareRecorded(t, src, dir)

	third := filepath.Join(work, "c"+project.ArchiveExt)
	if output, err := run(t, bin, "-bundle", dir, "-o", third); err != nil {
		t.Fatalf("-bundle of the unbundled directory: %v\n%s", err, output)
	}
	if c, _ := os.ReadFile(third); !bytes.Equal(a, c) {
		t.Errorf("re-bundling the unbundled directory gives different bytes")
	}

	// An archive another tool wrote round-trips too, and the result validates;
	// the README and license it carried unrecorded are not members of the
	// re-bundled archive, since its .meta.json records the three units alone.
	sysmod := filepath.Join(filepath.Dir(src), "mbse4u-sysmod_5.1.4"+project.ArchiveExt)
	unpacked := filepath.Join(work, "sysmod")
	if output, err := run(t, bin, "-unbundle", sysmod, "-o", unpacked); err != nil {
		t.Fatalf("-unbundle %s: %v\n%s", sysmod, err, output)
	}
	repacked := filepath.Join(work, "sysmod"+project.ArchiveExt)
	if output, err := run(t, bin, "-bundle", unpacked, "-o", repacked); err != nil {
		t.Fatalf("-bundle of the unbundled sysmod: %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-validate", repacked); err != nil {
		t.Errorf("-validate of the re-bundled archive: %v\n%s", err, output)
	}
	if a, err := project.OpenArchive(repacked); err != nil || len(a.Names()) != 5 {
		t.Errorf("re-bundled sysmod: %d members, err %v; want the two manifests and the three recorded units", len(a.Names()), err)
	}
	if output, err := run(t, bin, "-bundle", src, "-unbundle", first); err == nil || !strings.Contains(output, "one per run") {
		t.Errorf("-bundle with -unbundle: err %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-bundle", src, "-o", third, filepath.Join(src, "RequirementsManagement.sysml")); err == nil || !strings.Contains(output, "on their own") {
		t.Errorf("-bundle with a model: err %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-bundle", work, "-o", third); err == nil || !strings.Contains(output, ".project.json") {
		t.Errorf("-bundle of a directory with no .project.json: err %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-bundle", dir, "-o", filepath.Join(dir, "self"+project.ArchiveExt)); err == nil || !strings.Contains(output, "lies inside") {
		t.Errorf("-bundle into the project itself: err %v\n%s", err, output)
	}
	if output, err := run(t, bin, "-bundle", dir, "-o", filepath.Join(dir, ".project.json")); err == nil || !strings.Contains(output, "lies inside") {
		t.Errorf("-bundle over the project's manifest: err %v\n%s", err, output)
	}
	if manifest, err := os.ReadFile(filepath.Join(dir, ".project.json")); err != nil || !bytes.HasPrefix(manifest, []byte("{")) {
		t.Errorf("the manifest was overwritten: %q, %v", manifest, err)
	}
}

// compareRecorded fails unless got holds exactly the files want's .meta.json records, with the
// two manifests, byte for byte; what want holds unrecorded (README, LICENSES) is not the archive's.
func compareRecorded(t *testing.T, want, got string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(want, project.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := project.ParseMeta(content)
	if err != nil {
		t.Fatal(err)
	}
	recorded := map[string]bool{project.ProjectFile: true, project.MetaFile: true}
	for name := range meta.Checksum {
		recorded[name] = true
	}
	for _, name := range meta.Index {
		recorded[name] = true
	}
	for name := range recorded {
		w, err := os.ReadFile(filepath.Join(want, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		g, err := os.ReadFile(filepath.Join(got, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s missing from the unbundled directory: %v", name, err)
		} else if !bytes.Equal(w, g) {
			t.Errorf("%s differs after the round trip", name)
		}
	}
	err = filepath.Walk(got, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if rel, _ := filepath.Rel(got, path); !recorded[filepath.ToSlash(rel)] {
			t.Errorf("%s appeared in the unbundled directory; the archive records no such file", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
