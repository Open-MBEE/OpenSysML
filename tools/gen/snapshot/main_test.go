package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// fresh is the snapshot the bundled library builds to, which every output of
// the generator must equal byte for byte.
func fresh(t *testing.T) []byte {
	t.Helper()
	data, err := libs.BuildSnapshot(libs.BundledSource())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunWritesTheSnapshot(t *testing.T) {
	out := filepath.Join(t.TempDir(), "stdlib.snapshot")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0\n%s", code, stderr.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := fresh(t); !bytes.Equal(got, want) {
		t.Fatalf("written snapshot (%d bytes) differs from a fresh build (%d bytes)", len(got), len(want))
	}
	if !strings.HasPrefix(stdout.String(), "wrote "+out+" (") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

// TestRunWritesWherePackageLibsEmbeds: `go run -C tools` starts the generator
// inside the tools module, yet without -out it writes the file the embed
// directive of package libs reads, under the repository root — what every
// build target runs before compiling.
func TestRunWritesWherePackageLibsEmbeds(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, filepath.FromSlash(snapshotPath))
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0\n%s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "wrote "+want+" (") {
		t.Fatalf("stdout = %q, want the snapshot written to %s", stdout.String(), want)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fresh(t)) {
		t.Fatalf("%s is not a fresh snapshot after run", want)
	}
}

// TestRunWritesAnAbsoluteOutputFromOutsideTheRepository: with -out absolute the
// generator never looks for the repository, so it runs from any directory.
func TestRunWritesAnAbsoluteOutputFromOutsideTheRepository(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	out := filepath.Join(dir, "stdlib.snapshot")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("run -out %s from %s = %d, want 0\n%s", out, dir, code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestRunReportsAnUnwritableOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	out := filepath.Join(t.TempDir(), "missing", "stdlib.snapshot")
	if code := run([]string{"-out", out}, &stdout, &stderr); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "snapshot: ") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPrintsUsageForHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run -h = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage of snapshot:") || !strings.Contains(stderr.String(), "-out") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("help should write nothing to stdout, got %q", stdout.String())
	}
}

func TestRunRejectsAnUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -bogus") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
