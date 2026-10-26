package kpar_test

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// zipDir archives a project directory as a .kpar under a temporary directory,
// every file under its slash-separated relative path, as sysand build does.
func zipDir(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = f.Write(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), filepath.Base(dir)+project.ArchiveExt)
	if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return out
}

// The directory sysand installed carries the .meta.json sysand wrote, with
// one checksum object per path; archived, it must read and verify as a .kpar.
func TestSysandInstalledDirectoryReadsAsAnArchive(t *testing.T) {
	dir := filepath.Join(sysandProject(t), ".sysand", "lib", "ode4hera-requirements-management_2.0.1")
	a, err := project.OpenArchive(zipDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if a.Project.Name != "Requirements Management" {
		t.Errorf("project name %q", a.Project.Name)
	}
	if len(a.Notes) != 0 {
		t.Errorf("notes = %q, want none: both SHA256 checksums are verifiable", a.Notes)
	}
	for _, member := range []string{"RequirementsManagement.sysml", "examples/01_requirement_attributes.sysml"} {
		sums := a.Meta.Checksum[member]
		if len(sums) != 1 || sums[0].Algorithm != "SHA256" || sums[0].Unrecorded() {
			t.Errorf("%s: checksums %+v, want one recorded SHA256", member, sums)
		}
	}
	if got := len(a.Files()); got != 2 {
		t.Errorf("%d model files, want 2", got)
	}
}

// The .meta.json of MBSE4U's sysmod-sysmlv2 5.1.5 records "NONE" for every
// model file: it reads as metadata, and an archive carrying it reads too.
func TestSysmodMetaWithUnrecordedChecksumsReads(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "mbse4u", "sysmod-5.1.5.meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := project.ParseMeta(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Index) != 4 || len(meta.Checksum) != 4 {
		t.Fatalf("index %d checksums %d, want 4 and 4", len(meta.Index), len(meta.Checksum))
	}
	for member, sums := range meta.Checksum {
		if len(sums) != 1 || !sums[0].Unrecorded() || sums[0].Algorithm != "NONE" {
			t.Errorf("%s: %+v, want one NONE checksum", member, sums)
		}
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	members := map[string]string{
		".project.json": `{"name": "SYSMOD", "version": "5.1.5"}`,
		".meta.json":    string(content),
	}
	for name, file := range meta.Index {
		members[file] = "package " + name + ";\n"
	}
	for name, body := range members {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := project.ParseArchive("mbse4u-sysmod_5.1.5.kpar", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 0 {
		t.Errorf("notes = %q, want none: an unrecorded checksum is not a finding", a.Notes)
	}
	if got := len(a.Files()); got != 4 {
		t.Errorf("%d model files, want 4", got)
	}
}
