package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model of several files whose validation crosses them: a metadata body to
// link, a reference into another file, and an error to report in a third.
var loadModel = map[string]string{
	"a.sysml": "package A {\n\tmetadata def M { attribute n; }\n\tpart def X { @M { n = 1; } }\n}\n",
	"b.sysml": "package B { part x : A::X; part y : Missing; }\n",
	"c.sysml": "package C { part z : A::X { @A::M { n = 2; } } }\n",
}

func writeLoadModel(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(loadModel))
	for _, name := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(loadModel[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

// TestJobsGovernLoads: -jobs and OPENSYSML_JOBS bound how many files of a load
// are parsed and validated at once; -validate reports the same at any count, and a
// bad value is rejected before any file is read, in every mode that loads a model.
func TestJobsGovernLoads(t *testing.T) {
	binary := buildCLI(t)
	paths := writeLoadModel(t)
	validate := func(env []string, args ...string) runOutcome {
		return checkPathsEnv(t, binary, env, append(append([]string{"-validate"}, args...), paths...)...)
	}
	want := validate(nil)
	if want.status != 2 || !strings.Contains(want.output(), "unresolved reference: Missing") {
		t.Fatalf("the default run should report b.sysml's unresolved name and exit 2, got %d:\n%s", want.status, want.output())
	}

	for _, jobs := range []string{"1", "2", "8"} {
		if got := validate(nil, "-jobs", jobs); got.status != want.status || got.output() != want.output() {
			t.Errorf("-jobs %s reported %d\n%s\nwant the default's %d\n%s", jobs, got.status, got.output(), want.status, want.output())
		}
		if got := validate([]string{"OPENSYSML_JOBS=" + jobs}); got.status != want.status || got.output() != want.output() {
			t.Errorf("OPENSYSML_JOBS=%s reported %d\n%s\nwant the default's %d\n%s", jobs, got.status, got.output(), want.status, want.output())
		}
	}

	for _, bad := range []string{"0", "-3", "two", "1.5"} {
		got := validate(nil, "-jobs", bad)
		if got.status != 2 || !strings.Contains(got.output(), `-jobs="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "Missing") {
			t.Errorf("-jobs %s: status %d\n%s", bad, got.status, got.output())
		}
		got = validate([]string{"OPENSYSML_JOBS=" + bad})
		if got.status != 2 || !strings.Contains(got.output(), `OPENSYSML_JOBS="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "Missing") {
			t.Errorf("OPENSYSML_JOBS=%s: status %d\n%s", bad, got.status, got.output())
		}
	}

	if got := validate([]string{"OPENSYSML_JOBS=nope"}, "-jobs", "2"); got.status != want.status || got.output() != want.output() {
		t.Errorf("-jobs 2 under OPENSYSML_JOBS=nope reported %d\n%s\nwant the default's\n%s", got.status, got.output(), want.output())
	}

	// Every mode that loads a model reads the setting before loading.
	for _, mode := range [][]string{
		{"-satisfy"},
		{"-query", `sysml:name="X"`},
		{"-render", "A::X"},
		{"-render-all", t.TempDir()},
		{"-compile", "A::X", "-o", filepath.Join(t.TempDir(), "x")},
	} {
		got := checkPathsEnv(t, binary, []string{"OPENSYSML_JOBS=0"}, append(mode, paths...)...)
		if got.status != 2 || !strings.Contains(got.output(), `OPENSYSML_JOBS="0" is not a positive integer`) || strings.Contains(got.output(), "Missing") {
			t.Errorf("%s under OPENSYSML_JOBS=0: status %d\n%s", mode[0], got.status, got.output())
		}
	}
}
