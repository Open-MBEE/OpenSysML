package main

import (
	"os"
	"path/filepath"
	"slices"
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
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 4, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{
		manifestName, "constellation.sysml", "library.sysml", "notes.sysml",
		"plane000.sysml", "plane001.sysml", "plane002.sysml", "plane003.sysml", "plane009.sysml",
	}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("after regenerating with four planes the directory holds %v, want %v", got, want)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "plane009.sysml")); err != nil || string(got) != "package Notes;\n" {
		t.Errorf("the user's plane009.sysml reads %q, %v; want it untouched", got, err)
	}
}

func TestWriteSplitIntoAnUnknownDirectoryRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"plane000.sysml", "plane005.sysml", "library.sysml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package Mine;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 2, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{manifestName, "constellation.sysml", "library.sysml", "plane000.sysml", "plane001.sysml", "plane005.sysml"}
	if got := listing(t, dir); !slices.Equal(got, want) {
		t.Errorf("a first generation into a directory left %v, want %v", got, want)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "plane005.sysml")); err != nil || string(got) != "package Mine;\n" {
		t.Errorf("plane005.sysml, which no generation wrote, reads %q, %v; want it untouched", got, err)
	}
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
