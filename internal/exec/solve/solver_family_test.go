package solve

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// A solver behind a wrapper or under another name is given its family's
// arguments by what it answers to --version, not by its file name.
func TestNewSolverReadsTheFamilyOfAnUnrecognisedName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in solvers are shell scripts")
	}
	cases := []struct {
		name, banner string
		want         []string
	}{
		{"mysolver", "Z3 version 4.13.0 - 64 bit", []string{"-smt2", "-in"}},
		{"wrapper", "This is cvc5 version 1.2.0", []string{"--lang=smt2", "--incremental"}},
		{"other", "some other tool", nil},
	}
	for _, tc := range cases {
		path := filepath.Join(t.TempDir(), tc.name)
		script := "#!/bin/sh\n[ \"$1\" = --version ] && echo '" + tc.banner + "'\n"
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil { // #nosec G306 -- a test's own executable
			t.Fatal(err)
		}
		if got := newSolver(path).Args; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s (%s): arguments %v, want %v", tc.name, tc.banner, got, tc.want)
		}
	}
	if got := newSolver("/opt/bin/z3-4.13").Args; !reflect.DeepEqual(got, []string{"-smt2", "-in"}) {
		t.Errorf("a solver named for its family got %v", got)
	}
}
