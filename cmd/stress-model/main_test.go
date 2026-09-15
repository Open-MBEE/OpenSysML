package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/stressmodel"
)

func TestWriteSplitDropsOnlyWhatALargerGenerationWrote(t *testing.T) {
	dir := t.TempDir()
	// A model of the user's own, one of them under a name a plane could take.
	for _, name := range []string{"notes.sysml", "plane009.sysml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package Notes;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 8, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	// A directory now standing where the first generation put a plane.
	if err := os.Remove(filepath.Join(dir, "plane007.sysml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "plane007.sysml"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 4, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{
		manifestName, "constellation.sysml", "library.sysml", "notes.sysml",
		"plane000.sysml", "plane001.sysml", "plane002.sysml", "plane003.sysml", "plane007.sysml", "plane009.sysml",
	}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("after regenerating with four planes the directory holds %v, want %v", got, want)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "plane009.sysml")); err != nil || string(got) != "package Notes;\n" {
		t.Errorf("the user's plane009.sysml reads %q, %v; want it untouched", got, err)
	}
}

// A first generation into a directory holding files at generated names writes
// nothing: it names every one of them, leaves each as it was and makes no manifest.
func TestWriteSplitWritesNothingOverFilesItDidNotWrite(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"plane000.sysml", "plane005.sysml", "library.sysml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package Mine;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}, dir)
	if err == nil {
		t.Fatal("generating over the user's library.sysml and plane000.sysml should fail")
	}
	for _, name := range []string{"library.sysml", "plane000.sysml"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error %q does not name %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "plane005") {
		t.Errorf("the error %q names plane005.sysml, which no generation of two planes writes", err)
	}
	if got := listing(t, dir); !slices.Equal(got, []string{"library.sysml", "plane000.sysml", "plane005.sysml"}) {
		t.Errorf("the refused generation left %v; want the user's three files alone", got)
	}
	for _, name := range []string{"plane000.sysml", "plane005.sysml", "library.sysml"} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != "package Mine;\n" {
			t.Errorf("%s reads %q, %v; want it untouched", name, got, err)
		}
	}
}

// A later generation replaces only what the last one wrote and nothing has
// changed since: a plane the user edited, a directory at a plane's name and a
// file of the user's at one all stop it before it writes a byte, and are named
// together; with them out of the way the same generation goes through.
func TestWriteSplitReplacesOnlyItsOwnUnchangedOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.sysml"), []byte("package Notes;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	firstListing, firstManifest := listing(t, dir), recordedNames(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "plane001.sysml"), []byte("package Edited;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "plane002.sysml"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plane003.sysml"), []byte("package Mine;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := append(slices.Clone(firstListing), "plane002.sysml", "plane003.sysml")
	slices.Sort(before)
	_, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 4, Satellites: 1}, dir)
	if err == nil {
		t.Fatal("generating over an edited plane, a directory and the user's file should fail")
	}
	for _, name := range []string{"plane001.sysml", "plane002.sysml", "plane003.sysml"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error %q does not name %s", err, name)
		}
	}
	if got := listing(t, dir); !slices.Equal(got, before) {
		t.Errorf("the refused generation left %v, want %v: nothing written, nothing staged", got, before)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "plane001.sysml")); err != nil || string(got) != "package Edited;\n" {
		t.Errorf("the edited plane001.sysml reads %q, %v; want the edit kept", got, err)
	}
	if got := recordedNames(t, dir); !slices.Equal(got, firstManifest) {
		t.Errorf("the manifest reads %v after the refused generation, want %v as before", got, firstManifest)
	}

	for _, name := range []string{"plane001.sysml", "plane002.sysml", "plane003.sysml"} {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 4, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{manifestName, "constellation.sysml", "library.sysml", "notes.sysml", "plane000.sysml", "plane001.sysml", "plane002.sysml", "plane003.sysml"}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("regenerating with four planes left %v, want %v", got, want)
	}
}

// A generation interrupted between recording a file and moving it in leaves
// the manifest naming whatever stood at that name; a later generation must
// not take that for its own. Neither may it take a recorded file the user
// has since edited.
func TestWriteSplitRemovesOnlyFilesThatReadAsRecorded(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 3, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	// The user's plane007.sysml, recorded by an interrupted larger generation
	// that never moved its own plane007 over it.
	if err := os.WriteFile(filepath.Join(dir, "plane007.sysml"), []byte("package Mine;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	interrupted := digest([]byte("package Plane7;\n")) + " plane007.sysml\n"
	manifest, err := os.OpenFile(filepath.Join(dir, manifestName), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.WriteString(interrupted); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Close(); err != nil {
		t.Fatal(err)
	}
	// The user's edit of a plane the generator did write.
	if err := os.WriteFile(filepath.Join(dir, "plane002.sysml"), []byte("package Edited;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 1, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{manifestName, "constellation.sysml", "library.sysml", "plane000.sysml", "plane002.sysml", "plane007.sysml"}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("regenerating with one plane left %v, want %v: plane001 removed, the user's files kept", got, want)
	}
	for name, content := range map[string]string{"plane002.sysml": "package Edited;\n", "plane007.sysml": "package Mine;\n"} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != content {
			t.Errorf("%s reads %q, %v; want it untouched", name, got, err)
		}
	}
	if recorded := recordedNames(t, dir); !slices.Equal(recorded, []string{"library.sysml", "plane000.sysml", "constellation.sysml"}) {
		t.Errorf("the manifest reads %v; want only the last generation's files", recorded)
	}
}

// After a generation interrupted between recording and moving its files in,
// names hold the new content or the old; the next run replaces both.
func TestWriteSplitRetriesAnInterruptedGeneration(t *testing.T) {
	dir := t.TempDir()
	first := stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}
	second := stressmodel.SatelliteNetwork{Planes: 2, Satellites: 2}
	if _, err := writeSplit(first, dir); err != nil {
		t.Fatal(err)
	}
	files, _ := second.Split()
	if files[1].Name != "plane000.sysml" || files[2].Name != "plane001.sysml" {
		t.Fatalf("the split writes %s, %s second and third; the test wants the two planes", files[1].Name, files[2].Name)
	}
	// The second generation recorded both planes and moved only the first in.
	if err := os.WriteFile(filepath.Join(dir, files[1].Name), []byte(files[1].Source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := interruptManifest(dir, files[1:3]); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(second, dir); err != nil {
		t.Fatalf("the interrupted generation cannot be retried: %v", err)
	}
	for _, f := range files {
		if got, err := os.ReadFile(filepath.Join(dir, f.Name)); err != nil || string(got) != f.Source {
			t.Errorf("%s does not read as the retried generation writes it (%v)", f.Name, err)
		}
	}
	if recorded := recordedNames(t, dir); !slices.Equal(recorded, []string{"library.sysml", "plane000.sysml", "plane001.sysml", "constellation.sysml"}) {
		t.Errorf("the manifest reads %v; want each of the retried generation's files once", recorded)
	}
}

// A file an interrupted generation recorded and moved in is still the
// generator's: a smaller generation afterwards removes it like any other.
func TestWriteSplitRemovesWhatAnInterruptedGenerationMovedIn(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	files, _ := stressmodel.SatelliteNetwork{Planes: 2, Satellites: 2}.Split()
	if err := os.WriteFile(filepath.Join(dir, files[2].Name), []byte(files[2].Source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := interruptManifest(dir, files[1:3]); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 1, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{manifestName, "constellation.sysml", "library.sysml", "plane000.sysml"}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("regenerating with one plane left %v, want %v", got, want)
	}
}

// interruptManifest records files in dir's manifest the way a generation does
// before moving them in, and stops there.
func interruptManifest(dir string, files []stressmodel.File) error {
	previous, err := readManifest(dir)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(dir, ".stress-model-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	return replaceManifest(dir, staging, intended(previous, files))
}

// The manifest that stood while a generation ran is replaced whole, never
// cut short: a reader holding it sees it unchanged.
func TestWriteSplitReplacesTheManifestWholeInsteadOfTruncatingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a renamed-over file cannot be held open on Windows")
	}
	dir := t.TempDir()
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 1, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	held, err := io.ReadAll(old)
	if err != nil {
		t.Fatal(err)
	}
	if string(held) != string(first) {
		t.Errorf("the manifest held open through the second generation reads:\n%s\nwant the first manifest unchanged:\n%s", held, first)
	}
	if string(second) == string(first) {
		t.Error("the second generation did not replace the manifest")
	}
	if recorded := recordedNames(t, dir); !slices.Equal(recorded, []string{"library.sysml", "plane000.sysml", "constellation.sysml"}) {
		t.Errorf("the manifest reads %v; want only the second generation's files", recorded)
	}
}

func recordedNames(t *testing.T, dir string) []string {
	t.Helper()
	records, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range records {
		names = append(names, r.Name)
	}
	return names
}

func listing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
