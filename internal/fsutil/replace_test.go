package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func TestReplaceOverExistingFile(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "new"), filepath.Join(dir, "old")
	writeFile(t, source, "new\n")
	writeFile(t, target, "old\n")
	if err := Replace(source, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "new\n" {
		t.Errorf("target = %q", got)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Errorf("source remains: %v", err)
	}
}

// A first rename the platform refuses is retried after the target is removed:
// a rename over an empty directory is refused everywhere, standing in for
// Windows refusing one over a file.
func TestReplaceRetriesAfterRemovingTarget(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "new"), filepath.Join(dir, "old")
	writeFile(t, source, "new\n")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Replace(source, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "new\n" {
		t.Errorf("target = %q", got)
	}
}

func TestReplaceKeepsTargetWhenItCannotBeRemoved(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "new"), filepath.Join(dir, "old")
	writeFile(t, source, "new\n")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(target, "kept"), "kept\n")
	if err := Replace(source, target); err == nil {
		t.Fatal("replaced a directory that cannot be removed")
	}
	if got := readFile(t, filepath.Join(target, "kept")); got != "kept\n" {
		t.Errorf("target's content = %q", got)
	}
	if got := readFile(t, source); got != "new\n" {
		t.Errorf("source = %q", got)
	}
}

// A rename that fails for want of its source is not a target in the way: the
// target is kept and the failure reported.
func TestReplaceKeepsTargetWhenSourceIsMissing(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "missing"), filepath.Join(dir, "old")
	writeFile(t, target, "old\n")
	if err := Replace(source, target); !os.IsNotExist(err) {
		t.Fatalf("Replace of a missing source: %v, want its absence reported", err)
	}
	if got := readFile(t, target); got != "old\n" {
		t.Errorf("target = %q", got)
	}
}

func TestReplaceReplacesLinkNotItsTarget(t *testing.T) {
	dir := t.TempDir()
	source, target, elsewhere := filepath.Join(dir, "new"), filepath.Join(dir, "link"), filepath.Join(dir, "elsewhere")
	writeFile(t, source, "new\n")
	writeFile(t, elsewhere, "elsewhere\n")
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := Replace(source, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, elsewhere); got != "elsewhere\n" {
		t.Errorf("the link's target was written: %q", got)
	}
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("target is still a link: %v, %v", info, err)
	}
	if got := readFile(t, target); got != "new\n" {
		t.Errorf("target = %q", got)
	}
}
