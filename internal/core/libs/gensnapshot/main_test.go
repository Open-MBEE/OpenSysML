package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const committed = "../stdlib.snapshot"

// TestCommittedSnapshotIsCurrent is the drift gate `make stdlib-snapshot-check` runs,
// in-process: the embedded library must regenerate to the committed snapshot byte for byte.
func TestCommittedSnapshotIsCurrent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-check", "-out", committed}, &stdout, &stderr); code != 0 {
		t.Fatalf("run -check = %d, want 0\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("a current snapshot should print nothing, got %q / %q", stdout.String(), stderr.String())
	}
}

func TestRunWritesTheSnapshot(t *testing.T) {
	out := filepath.Join(t.TempDir(), "stdlib.snapshot")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0\n%s", code, stderr.String())
	}
	want, err := os.ReadFile(committed)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("written snapshot (%d bytes) differs from the committed one (%d bytes)", len(got), len(want))
	}
	if !strings.HasPrefix(stdout.String(), "wrote "+out+" (") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestCheckRejectsAStaleOrMissingSnapshot(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "stale.snapshot")
	if err := os.WriteFile(stale, []byte("not a snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{stale, filepath.Join(dir, "missing.snapshot")} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"-check", "-out", out}, &stdout, &stderr); code != 1 {
			t.Fatalf("run -check -out %s = %d, want 1", out, code)
		}
		if !strings.Contains(stderr.String(), out+" is stale") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	}
}

func TestRunReportsAnUnwritableOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	out := filepath.Join(t.TempDir(), "missing", "stdlib.snapshot")
	if code := run([]string{"-out", out}, &stdout, &stderr); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "gensnapshot: ") {
		t.Fatalf("stderr = %q", stderr.String())
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
