package gobuild

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Args only reads the environment: the runtime's variable is whatever it was before the call.
func TestArgsChangesNoEnvironment(t *testing.T) {
	t.Setenv(DirEnv, t.TempDir())
	t.Setenv(runtimeDirEnv, "as it was")
	args := Args("out")
	if !slices.Contains(args, "-cover") {
		t.Errorf("a coverage run builds without -cover: %v", args)
	}
	if got := os.Getenv(runtimeDirEnv); got != "as it was" {
		t.Errorf("Args set %s to %q", runtimeDirEnv, got)
	}

	t.Setenv(DirEnv, "")
	if args := Args("out", "-trimpath"); !slices.Equal(args, []string{"build", "-trimpath", "-o", "out", "."}) {
		t.Errorf("outside a coverage run: %v", args)
	}
}

// The counter directory reaches the runtime's variable through exportDir alone.
func TestExportDirHandsTheCounterDirectoryToTheRuntime(t *testing.T) {
	t.Setenv(DirEnv, "")
	t.Setenv(runtimeDirEnv, "as it was")
	if err := exportDir(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(runtimeDirEnv); got != "as it was" {
		t.Errorf("outside a coverage run %s became %q", runtimeDirEnv, got)
	}
	dir := t.TempDir()
	t.Setenv(DirEnv, dir)
	if err := exportDir(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(runtimeDirEnv); got != dir {
		t.Errorf("%s = %q, want %q", runtimeDirEnv, got, dir)
	}
}

// A Scratch is one directory however often it is asked for, and Remove leaves nothing behind.
func TestScratchIsRemoved(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var unused Scratch
	if err := unused.Remove(); err != nil {
		t.Errorf("removing a scratch never used: %v", err)
	}
	scratch := Scratch{Pattern: "gobuild-scratch"}
	dir, err := scratch.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := scratch.Dir(); again != dir {
		t.Errorf("a second directory %s beside %s", again, dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "built"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scratch.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s is still there: %v", dir, err)
	}
}
