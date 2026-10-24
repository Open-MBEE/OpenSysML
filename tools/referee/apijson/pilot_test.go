package apijson

import (
	"path/filepath"
	"reflect"
	"testing"
)

// The bridge is told the extra library directories explicitly, after the
// launcher's pinned standard library and before the root's files, so a run
// is reproducible from its command line alone.
func TestPilotArgsNameEveryExtraLibraryBeforeTheFiles(t *testing.T) {
	dir := filepath.Join("repo", "examples")
	got := pilotArgs(dir, "out", []string{filepath.Join("repo", "libs", "OpenSysML Libraries")}, []string{"a.sysml", "sub/b.kerml"})
	want := []string{
		"--root", dir, "--out", "out",
		"--extension-library", filepath.Join("repo", "libs", "OpenSysML Libraries"),
		filepath.Join(dir, "a.sysml"), filepath.Join(dir, "sub", "b.kerml"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := pilotArgs(dir, "out", nil, []string{"a.sysml"}); !reflect.DeepEqual(got, []string{"--root", dir, "--out", "out", filepath.Join(dir, "a.sysml")}) {
		t.Fatalf("without libraries got %q", got)
	}
}
