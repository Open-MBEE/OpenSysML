package main

import (
	"os"
	"testing"
)

// The kernel resolves a cell's unresolved imports against its working
// directory, which Jupyter sets to the notebook's directory when it starts the
// kernel.
func TestTheSessionIsRootedAtTheWorkingDirectory(t *testing.T) {
	sess, err := newSession(build())
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := sess.WorkspaceRoot(); got != cwd {
		t.Errorf("WorkspaceRoot() = %q, want the working directory %q", got, cwd)
	}
}
