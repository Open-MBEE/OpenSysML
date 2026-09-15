package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/stressmodel"
)

func TestWriteSplitDropsThePlanesOfALargerGeneration(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "notes.sysml")
	if err := os.WriteFile(keep, []byte("package Notes;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 8, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSplit(stressmodel.SatelliteNetwork{Planes: 4, Satellites: 1}, dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := []string{
		"constellation.sysml", "library.sysml", "notes.sysml",
		"plane000.sysml", "plane001.sysml", "plane002.sysml", "plane003.sysml",
	}
	if !slices.Equal(got, want) {
		t.Errorf("after regenerating with four planes the directory holds %v, want %v", got, want)
	}
}

func TestIsPlaneFileMatchesOnlyTheGeneratorsNames(t *testing.T) {
	for _, name := range []string{"plane000.sysml", "plane031.sysml", "plane1000.sysml"} {
		if !stressmodel.IsPlaneFile(name) {
			t.Errorf("IsPlaneFile(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"plane.sysml", "plane01.sysml", "plane-01.sysml", "plane001.kerml", "planet001.sysml", "myplane001.sysml", "library.sysml"} {
		if stressmodel.IsPlaneFile(name) {
			t.Errorf("IsPlaneFile(%q) = true, want false", name)
		}
	}
}
