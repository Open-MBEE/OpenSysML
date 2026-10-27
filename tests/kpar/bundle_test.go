package kpar_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// recordedProject writes a project laid out as a repository is: the
// .meta.json records the two units and the LICENSE, and the tree also holds
// what a repository accumulates and no archive carries — a README, a build
// script with its __pycache__, an editor backup and version-control metadata.
func recordedProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "widgets")
	writeTree(t, dir, map[string]string{
		project.ProjectFile: `{"name": "Widgets", "publisher": "acme", "version": "1.0.0"}`,
		project.MetaFile: `{
  "index": {"Gears": "libraries/Gears.kerml", "Widgets": "libraries/Widgets.sysml"},
  "created": "2026-01-01T00:00:00Z",
  "metamodel": "https://www.omg.org/spec/SysML/20250201",
  "checksum": {
    "LICENSE": {"value": "4e8f3e2c5a1a5e4c0d5f6a1a0dc3b7a8a1b4d1ad8e4c5f1d61d1bb4b8c2e9d4b", "algorithm": "SHA256"},
    "libraries/Gears.kerml": {"value": "b4f6d2e2a4f7b8b4bb5d0b2a6b2c6a71f5d1a3f1c6cbe0f4b1d4a7c1b2e3d4c5", "algorithm": "SHA256"},
    "libraries/Widgets.sysml": {"value": "f0f3d1c1e4e4a8f1d7a0e7f2b4c9b6e2a5d8c3f1b0e9d4c7a6b5f3e2d1c0b9a8", "algorithm": "SHA256"}
  }
}
`,
		"LICENSE":                 "Apache License 2.0\n",
		"libraries/Gears.kerml":   "package Gears { class Gear; }\n",
		"libraries/Widgets.sysml": "package Widgets { private import Gears::*; part def Widget { part gear : Gear; } }\n",
		"README.md":               "# Widgets\n",
		"CONTRIBUTING.md":         "Send patches.\n",
		"scripts/build.py":        "print('build')\n",
		"scripts/__pycache__/build.cpython-312.pyc": "\x00\x00pyc",
		"libraries/Widgets.sysml~":                  "package Widgets;\n",
		".git/HEAD":                                 "ref: refs/heads/main\n",
		".github/workflows/ci.yml":                  "on: push\n",
	})
	return dir
}

// recordChecksums rewrites the project's .meta.json with the real SHA-256 of
// each recorded file, as its publisher's tooling would.
func recordChecksums(t *testing.T, dir string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, project.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := project.ParseMeta(content)
	if err != nil {
		t.Fatal(err)
	}
	for name := range meta.Checksum {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		meta.Checksum[name] = project.Checksums{{Value: hex.EncodeToString(sum[:]), Algorithm: "SHA256"}}
	}
	out, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, project.MetaFile), out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A directory with a .meta.json bundles into the two manifests and exactly the
// files the metadata records, whatever else the tree holds; each member's
// bytes are the file's.
func TestBundleArchivesTheFilesTheMetadataRecordsAlone(t *testing.T) {
	dir := recordedProject(t)
	recordChecksums(t, dir)
	bin := sysmlBinary(t)
	out := filepath.Join(t.TempDir(), "widgets"+project.ArchiveExt)
	output, err := run(t, bin, "-bundle", dir, "-o", out)
	if err != nil {
		t.Fatalf("-bundle: %v\n%s", err, output)
	}
	if !strings.Contains(output, "wrote "+out+" (Widgets 1.0.0, 5 files)") {
		t.Errorf("-bundle wrote:\n%s\nwant the 5 recorded files counted", output)
	}
	a, err := project.OpenArchive(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".meta.json", ".project.json", "LICENSE", "libraries/Gears.kerml", "libraries/Widgets.sysml"}
	if got := a.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("members = %q, want %q", got, want)
	}
	for _, name := range want {
		file, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if member, err := a.Read(name); err != nil || string(member) != string(file) {
			t.Errorf("%s: member differs from the file (%v)", name, err)
		}
	}
}

// A recorded file missing from the tree refuses the bundle, naming the file.
func TestBundleRefusesATreeMissingARecordedFile(t *testing.T) {
	dir := recordedProject(t)
	recordChecksums(t, dir)
	if err := os.Remove(filepath.Join(dir, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	output, err := run(t, sysmlBinary(t), "-bundle", dir, "-o", filepath.Join(t.TempDir(), "widgets"+project.ArchiveExt))
	if err == nil || !strings.Contains(output, "cannot bundle "+dir+": .meta.json records LICENSE, which is not in the directory") {
		t.Errorf("-bundle with LICENSE missing: err %v\n%s", err, output)
	}
}

// Without a .meta.json the archive holds what KerML 10.3 says it holds: the
// project file and the model files, with a .meta.json written from them;
// the README, scripts, caches, backups and version-control metadata stay out.
func TestBundleWithoutMetadataArchivesTheModelFiles(t *testing.T) {
	dir := recordedProject(t)
	if err := os.Remove(filepath.Join(dir, project.MetaFile)); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "widgets"+project.ArchiveExt)
	if output, err := run(t, sysmlBinary(t), "-bundle", dir, "-o", out); err != nil {
		t.Fatalf("-bundle: %v\n%s", err, output)
	}
	a, err := project.OpenArchive(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".meta.json", ".project.json", "libraries/Gears.kerml", "libraries/Widgets.sysml"}
	if got := a.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("members = %q, want %q", got, want)
	}
	roots := make([]string, 0, len(a.Meta.Index))
	for root := range a.Meta.Index {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	if strings.Join(roots, ",") != "Gears,Widgets" || a.Meta.Index["Gears"] != "libraries/Gears.kerml" {
		t.Errorf("generated index = %v, want the two roots at their units", a.Meta.Index)
	}
}
