package kpar_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

const (
	extensionsUsage   = "pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library@1.0.0"
	extensionsInstall = "open-mbee-opensysml-extensions-library_1.0.0"
)

var (
	vendoredUnits     = filepath.Join("..", "..", "internal", "workspace", "libs", "stdlib", "OpenSysML Libraries")
	vendoredManifests = filepath.Join("..", "..", "internal", "workspace", "libs", "extensions-project")
)

// extensionsProject is the Extensions Library as its tagged tree holds it:
// the vendored units under libraries/, with the manifests and the LICENSE the
// pin carries, so the .meta.json checksums verify. Name replaces the project's
// name when not empty.
func extensionsProject(t *testing.T, name string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, manifest := range []string{project.ProjectFile, project.MetaFile, "LICENSE"} {
		content, err := os.ReadFile(filepath.Join(vendoredManifests, manifest))
		if err != nil {
			t.Fatal(err)
		}
		files[manifest] = content
	}
	entries, err := os.ReadDir(vendoredUnits)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !project.IsModelFile(e.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(vendoredUnits, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files["libraries/"+e.Name()] = content
	}
	if name != "" {
		var manifest map[string]any
		if err := json.Unmarshal(files[project.ProjectFile], &manifest); err != nil {
			t.Fatal(err)
		}
		manifest["name"] = name
		if files[project.ProjectFile], err = json.Marshal(manifest); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// unitsOf lists the model files among files, as the install's relative paths.
func unitsOf(files map[string][]byte) []string {
	var units []string
	for name := range files {
		if project.IsModelFile(name) {
			units = append(units, name)
		}
	}
	sort.Strings(units)
	return units
}

// zipFiles archives files under prefix ("" for the top level), as -bundle and
// a release job do when prefix is empty, and as an archive of the project's
// folder does otherwise.
func zipFiles(t *testing.T, files map[string][]byte, prefix string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := w.Create(prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// extensionsConsumer writes a project using the Extensions Library by its
// repository IRI, with the library installed under lib/ as the tree or the
// archive given, beside an installed project no usage claims: the library is
// found by its name, not as the only candidate left.
func extensionsConsumer(t *testing.T, tree map[string][]byte, archive []byte) string {
	t.Helper()
	proj := filepath.Join(t.TempDir(), "proj")
	writeTree(t, proj, map[string]string{
		project.ProjectFile: `{"name": "Consumer", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "` + extensionsUsage + `"}]}`,
		"Consumer.sysml":    "package Consumer {\n    private import MOSA::*;\n    private import StateMachines::*;\n}\n",
		"lib/acme-widgets_1.0.0/" + project.ProjectFile: `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`,
		"lib/acme-widgets_1.0.0/Widgets.sysml":          "package Widgets;\n",
	})
	install := filepath.Join(proj, "lib", extensionsInstall)
	if archive != nil {
		if err := os.WriteFile(install+project.ArchiveExt, archive, 0o600); err != nil {
			t.Fatal(err)
		}
		return proj
	}
	for name, content := range tree {
		p := filepath.Join(install, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return proj
}

// loadedFromInstall checks that every unit of the install, and nothing of the
// unclaimed Widgets, is among the files the load read.
func loadedFromInstall(t *testing.T, report repl.LoadReport, install string, units []string) {
	t.Helper()
	for _, unit := range units {
		if !containsSubstring(report.Loaded, filepath.Join(install, filepath.FromSlash(unit))) {
			t.Errorf("loaded %q, want %s from the install", report.Loaded, unit)
		}
	}
	if containsSubstring(report.Loaded, "acme-widgets_1.0.0") {
		t.Errorf("loaded %q: Widgets, which no usage names, was loaded", report.Loaded)
	}
}

// The library installed under lib/ from the repository IRI's usage loads its
// sixteen packages from the install, linked by publisher and name — an
// elimination or ambiguity would be a warning — as a directory and as the
// archive of the same tree, whose checksums verify.
func TestExtensionsLibraryInstallIsLinkedByNameAndLoadsItsPackages(t *testing.T) {
	tree := extensionsProject(t, "")
	units := unitsOf(tree)
	if len(units) != 16 {
		t.Fatalf("the vendored copy holds %d units, want the 16 extension libraries", len(units))
	}
	for _, form := range []struct {
		name    string
		archive []byte
	}{{"directory", nil}, {"archive", zipFiles(t, tree, "")}} {
		t.Run(form.name, func(t *testing.T) {
			proj := extensionsConsumer(t, tree, form.archive)
			install := extensionsInstall
			if form.archive != nil {
				install += project.ArchiveExt
			}
			s := repl.NewSession()
			report, err := s.LoadPathsReport([]string{proj})
			if err != nil {
				t.Fatal(err)
			}
			if report.Errors || len(report.Found) != 0 {
				t.Fatalf("found:\n%s", strings.Join(report.Found, "\n"))
			}
			if !containsSubstring(report.Loaded, "loaded 17 files:") {
				t.Errorf("loaded %q, want Consumer and the 16 units", report.Loaded)
			}
			loadedFromInstall(t, report, install, units)
			out, err := run(t, sysmlBinary(t), "-warnings-as-errors", "-validate", proj)
			if err != nil {
				t.Fatalf("-warnings-as-errors -validate: %v\n%s", err, out)
			}
		})
	}
}

// A unit changed after the manifests were written fails the archive's
// checksum: the archive is a warning on its path and nothing of it loads.
func TestExtensionsLibraryArchiveWithAChangedUnitIsRefused(t *testing.T) {
	tree := extensionsProject(t, "")
	tree["libraries/MOSA.sysml"] = append(tree["libraries/MOSA.sysml"], []byte("\n// changed after release\n")...)
	proj := extensionsConsumer(t, nil, zipFiles(t, tree, ""))
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(proj, "lib", extensionsInstall+project.ArchiveExt) + `: not a project interchange archive: .meta.json: checksum of "libraries/MOSA.sysml" does not match`
	if !containsSubstring(report.Found, want) {
		t.Errorf("found %q, want %q", report.Found, want)
	}
	if containsSubstring(report.Loaded, extensionsInstall) {
		t.Errorf("loaded %q from an archive whose checksum failed", report.Loaded)
	}
}

// A project whose name does not slug to the repository's last path segment is
// not matched by name; it is linked by elimination when it is the one
// candidate, and the load says so for the exit policy to judge.
func TestExtensionsLibraryUnderAnotherNameIsLinkedByEliminationWithAWarning(t *testing.T) {
	tree := extensionsProject(t, "OpenSysML Extension Libraries")
	proj := extensionsConsumer(t, tree, nil)
	if err := os.RemoveAll(filepath.Join(proj, "lib", "acme-widgets_1.0.0")); err != nil {
		t.Fatal(err)
	}
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	loadedFromInstall(t, report, extensionsInstall, unitsOf(tree))
	want := filepath.Join(proj, project.ProjectFile) + ": usage " + extensionsUsage + " names no installed project by publisher and name; linked by elimination to open-mbee/opensysml-extension-libraries 1.0.0 (" + filepath.Join(proj, "lib", extensionsInstall) + "), the one unclaimed installed project that satisfies it; end the resource with the project's publisher and name (…/open-mbee/opensysml-extension-libraries) to name it"
	if !containsSubstring(report.Found, want) {
		t.Errorf("found %q, want %q", report.Found, want)
	}
	bin := sysmlBinary(t)
	if out, err := run(t, bin, "-validate", proj); err != nil || !strings.Contains(out, "warning: "+want) {
		t.Errorf("-validate: %v\n%s\nwant the warning %q", err, out, want)
	}
	if out, err := run(t, bin, "-warnings-as-errors", "-validate", proj); err == nil {
		t.Errorf("-warnings-as-errors -validate passed over the elimination warning:\n%s", out)
	}
}

// An archive of the project's folder rather than of its contents, as the OMG
// library releases are laid out, is read as the project inside the folder;
// an index entry naming no member — the OMG Requirement Derivation release
// indexes RequirementsDerivation.sysml for RequirementDerivation.sysml — is
// noted, and the unit is found by what it declares.
func TestArchiveOfTheProjectsFolderWithAMisnamedIndexEntryLoadsWithNotes(t *testing.T) {
	tree := extensionsProject(t, "")
	var meta map[string]any
	if err := json.Unmarshal(tree[project.MetaFile], &meta); err != nil {
		t.Fatal(err)
	}
	index := meta["index"].(map[string]any)
	delete(index, "MOSA")
	index["MOSAs"] = "libraries/MOSAs.sysml"
	var err error
	if tree[project.MetaFile], err = json.Marshal(meta); err != nil {
		t.Fatal(err)
	}
	proj := extensionsConsumer(t, nil, zipFiles(t, tree, "OpenSysML Extensions Library/"))
	s := repl.NewSession()
	report, err := s.LoadPathsReport([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Fatalf("errors:\n%s", strings.Join(report.Found, "\n"))
	}
	archive := filepath.Join(proj, "lib", extensionsInstall+project.ArchiveExt)
	loadedFromInstall(t, report, extensionsInstall+project.ArchiveExt, unitsOf(tree))
	for _, want := range []string{
		archive + ": the archive holds the folder OpenSysML Extensions Library/ with the project inside it rather than the project at its top level; read as the project in that folder",
		archive + `: .meta.json: index entry "MOSAs" names "libraries/MOSAs.sysml", which is not a member of the archive; the members' own declarations are read instead`,
	} {
		if !containsSubstring(report.Found, want) {
			t.Errorf("found %q, want %q", report.Found, want)
		}
	}
	if len(report.Found) != 2 {
		t.Errorf("found %d lines, want the two notes alone:\n%s", len(report.Found), strings.Join(report.Found, "\n"))
	}
}
