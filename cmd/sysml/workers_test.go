package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model of several files whose validation crosses them: a metadata body to
// link, a reference into another file, and an error to report in a third.
var workersModel = map[string]string{
	"a.sysml": "package A {\n\tmetadata def M { attribute n; }\n\tpart def X { @M { n = 1; } }\n}\n",
	"b.sysml": "package B { part x : A::X; part y : Missing; }\n",
	"c.sysml": "package C { part z : A::X { @A::M { n = 2; } } }\n",
}

func writeWorkersModel(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(workersModel))
	for _, name := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(workersModel[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

// TestWorkersFlagAndEnvironment checks that -workers and OPENSYSML_WORKERS take a
// positive integer, that the flag wins over the variable, that either rejected
// at startup loads nothing, and that the count does not change what -validate
// reports over a model of several files.
func TestWorkersFlagAndEnvironment(t *testing.T) {
	binary := buildCLI(t)
	paths := writeWorkersModel(t)
	validate := func(env []string, args ...string) runOutcome {
		return checkPathsEnv(t, binary, env, append(append([]string{"-validate"}, args...), paths...)...)
	}
	want := validate(nil)
	if want.status != 2 || !strings.Contains(want.output(), "unresolved reference: Missing") {
		t.Fatalf("the default run should report b.sysml's unresolved name and exit 2, got %d:\n%s", want.status, want.output())
	}

	for _, workers := range []string{"1", "2", "8"} {
		if got := validate(nil, "-workers", workers); got.status != want.status || got.output() != want.output() {
			t.Errorf("-workers %s reported %d\n%s\nwant the default's %d\n%s", workers, got.status, got.output(), want.status, want.output())
		}
		if got := validate([]string{"OPENSYSML_WORKERS=" + workers}); got.status != want.status || got.output() != want.output() {
			t.Errorf("OPENSYSML_WORKERS=%s reported %d\n%s\nwant the default's %d\n%s", workers, got.status, got.output(), want.status, want.output())
		}
	}

	for _, bad := range []string{"0", "-3", "two", "1.5"} {
		got := validate(nil, "-workers", bad)
		if got.status != 2 || !strings.Contains(got.output(), `-workers="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "Missing") {
			t.Errorf("-workers %s: status %d\n%s", bad, got.status, got.output())
		}
		got = validate([]string{"OPENSYSML_WORKERS=" + bad})
		if got.status != 2 || !strings.Contains(got.output(), `OPENSYSML_WORKERS="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "Missing") {
			t.Errorf("OPENSYSML_WORKERS=%s: status %d\n%s", bad, got.status, got.output())
		}
	}

	if got := validate([]string{"OPENSYSML_WORKERS=nope"}, "-workers", "2"); got.status != want.status || got.output() != want.output() {
		t.Errorf("-workers 2 under OPENSYSML_WORKERS=nope reported %d\n%s\nwant the default's\n%s", got.status, got.output(), want.output())
	}
}
