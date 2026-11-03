package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probeDir writes a project directory without a .meta.json: manifest, models in subdirectories,
// and a README, a hidden directory and an editor backup an archive of it does not carry.
func probeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".project.json":        probeProject,
		"model/Probe.sysml":    probeModel,
		"README.md":            "notes\n",
		".git/config":          "[core]\n",
		".sysand/lib/x.sysml":  "package X;\n",
		"model/Gears.kerml":    "package Gears { classifier Gear; }\n",
		"model/sub/deep.sysml": "package Deep;\n",
		"model/Probe.sysml~":   "package Probe { part def Old; }\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBundleIsDeterministicAndGeneratesMetadata(t *testing.T) {
	dir := probeDir(t)
	first, p, err := Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two bundles of the same directory differ")
	}
	if p.Name != "Probe" {
		t.Errorf("project = %+v", p)
	}
	a, err := ParseArchive("probe.kpar", first)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".meta.json", ".project.json", "model/Gears.kerml", "model/Probe.sysml", "model/sub/deep.sysml"}
	if got := a.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("members = %q, want %q (sorted; the model files alone, without a .meta.json to record more)", got, want)
	}
	if a.Meta.Created != "1980-01-01T00:00:00Z" || a.Meta.Metamodel != MetamodelSysML {
		t.Errorf("meta = %+v, want the archive epoch and the SysML metamodel", a.Meta)
	}
	if a.Meta.Index["Probe"] != "model/Probe.sysml" || a.Meta.Index["Gears"] != "model/Gears.kerml" || a.Meta.Index["Deep"] != "model/sub/deep.sysml" {
		t.Errorf("index = %v", a.Meta.Index)
	}
	for _, member := range []string{".project.json", "model/Gears.kerml", "model/Probe.sysml"} {
		if sums := a.Meta.Checksum[member]; len(sums) != 1 || sums[0].Algorithm != "SHA256" {
			t.Errorf("checksum[%s] = %v, want one SHA256", member, sums)
		}
	}
	if _, ok := a.Meta.Checksum[".meta.json"]; ok {
		t.Error("the metadata file records a checksum of itself")
	}
}

func TestBundleKeepsAndVerifiesAnExistingMetadataFile(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{".project.json": probeProject, "Probe.sysml": probeModel, ".meta.json": probeMeta} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, _, err := Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := ParseArchive("probe.kpar", data)
	if err != nil {
		t.Fatal(err)
	}
	if content, _ := a.Read(".meta.json"); string(content) != probeMeta {
		t.Errorf(".meta.json = %q, want the directory's own kept byte for byte", content)
	}
	if err := os.WriteFile(filepath.Join(dir, "Probe.sysml"), []byte("package Changed;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("Bundle with a stale checksum: %v, want a checksum failure", err)
	}
}

// A .meta.json decides the members: the files it records, whatever else the
// directory holds; one it records that is missing names itself in the error.
func TestBundleArchivesTheFilesTheMetadataRecords(t *testing.T) {
	dir := probeDir(t)
	meta := `{"index": {"Probe": "model/Probe.sysml"}, "created": "2026-01-01T00:00:00Z",
	"checksum": {"model/Probe.sysml": {"value": "4044ca78bfbbcdd72b63cd877ad948c20b758dd30cbb4a0bf8c9e0af30c98bda", "algorithm": "SHA256"}, "README.md": {"value": "", "algorithm": "NONE"}}}`
	if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _, err := Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := ParseArchive("probe.kpar", data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".meta.json", ".project.json", "README.md", "model/Probe.sysml"}
	if got := a.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("members = %q, want the recorded files %q: Gears.kerml and deep.sysml are not recorded", got, want)
	}
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || err.Error() != ".meta.json records README.md, which is not in the directory" {
		t.Errorf("Bundle with a recorded file missing: %v, want the error naming README.md", err)
	}
	if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(strings.Replace(meta, `"README.md"`, `"../README.md"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || !strings.Contains(err.Error(), `records "../README.md"`) {
		t.Errorf("Bundle with a recorded path outside the directory: %v", err)
	}
	// A recorded path through a linked directory leading outside is refused too.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "README.md"), []byte("elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "docs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(strings.Replace(meta, `"README.md"`, `"docs/README.md"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || !strings.Contains(err.Error(), "records docs/README.md, which") || !strings.Contains(err.Error(), "resolves outside") {
		t.Errorf("Bundle with a recorded path through a link leading outside: %v", err)
	}
}

func TestBundleRefusesADirectoryThatIsNotAProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Probe.sysml"), []byte(probeModel), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || !strings.Contains(err.Error(), ".project.json") {
		t.Errorf("Bundle of a plain directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".project.json"), []byte(`{"name": "Probe"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Bundle(dir); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("Bundle with an invalid manifest: %v", err)
	}
}

func TestUnbundleReproducesTheDirectoryAndRefusesToOverwrite(t *testing.T) {
	src := probeDir(t)
	data, _, err := Bundle(src)
	if err != nil {
		t.Fatal(err)
	}
	a, err := ParseArchive("probe.kpar", data)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if err := Unbundle(a, out); err != nil {
		t.Fatal(err)
	}
	for _, member := range a.Names() {
		want, _ := a.Read(member)
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(member)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: %v, bytes equal %v", member, err, bytes.Equal(got, want))
		}
	}
	if _, err := os.Stat(filepath.Join(out, ".git")); err == nil {
		t.Error("the hidden directory travelled with the archive")
	}
	if err := Unbundle(a, out); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("Unbundle into a directory with files: %v", err)
	}
	if err := Unbundle(a, filepath.Join(out, "model", "Probe.sysml")); err == nil {
		t.Error("Unbundle onto a file succeeded")
	}
	again, _, err := Bundle(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, data) {
		t.Error("bundling the unbundled directory gives different bytes")
	}
}

// A project's lib/ holds what it uses, not its own files: the archive leaves it
// out, so two installed versions declaring one namespace do not stop -bundle.
func TestBundleLeavesTheLibraryDirectoryOut(t *testing.T) {
	dir := probeDir(t)
	for _, version := range []string{"1.0.0", "2.0.0"} {
		gears := filepath.Join(dir, "lib", "acme-gears_"+version)
		if err := os.MkdirAll(gears, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gears, ProjectFile), []byte(`{"name": "Gears", "publisher": "acme", "version": "`+version+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gears, "Gears.kerml"), []byte("package Gears {\n    class Thing;\n}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, _, err := Bundle(dir)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	a, err := ParseArchive("probe.kpar", data)
	if err != nil {
		t.Fatal(err)
	}
	for name := range a.members {
		if strings.HasPrefix(name, "lib/") {
			t.Errorf("archive holds %s; the installed projects are not the project's files", name)
		}
	}
}
