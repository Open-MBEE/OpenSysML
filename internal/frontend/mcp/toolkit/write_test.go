package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileKeepsTheModeAndReplacesAtomically(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(tr.root, "sub", "kept.sysml")
	tr.write(t, existing, "package Old;")
	if err := os.Chmod(existing, 0o640); err != nil {
		t.Fatal(err)
	}
	replaced, err := r.WriteFile(existing, []byte("package New;"))
	if err != nil || !replaced {
		t.Fatalf("WriteFile over an existing file = %v, %v; want replaced", replaced, err)
	}
	if got, _ := os.ReadFile(existing); string(got) != "package New;" {
		t.Errorf("content = %q", got)
	}
	if info, err := os.Stat(existing); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, %v; want the existing 0640 kept", info.Mode(), err)
	}
	fresh := filepath.Join(tr.root, "sub", "fresh.sysml")
	replaced, err = r.WriteFile("sub/fresh.sysml", []byte("package Fresh;"))
	if err != nil || replaced {
		t.Fatalf("WriteFile of a new file = %v, %v; want written, not replaced", replaced, err)
	}
	if info, err := os.Stat(fresh); err != nil || info.Mode().Perm() != savedFileMode&^umask(t) {
		t.Errorf("a new file's mode = %v, %v; want %o less the umask", info.Mode(), err, savedFileMode)
	}
	entries, _ := os.ReadDir(filepath.Join(tr.root, "sub"))
	if len(entries) != 2 {
		t.Errorf("the directory holds %d entries; want the two files and no temporary left behind", len(entries))
	}
	if _, err := r.WriteFile("sub", []byte("x")); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("WriteFile over a directory = %v; want a refusal", err)
	}
}

// umask is the process's file creation mask, read by creating a file.
func umask(t *testing.T) os.FileMode {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(probe, nil, 0o777); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(probe)
	if err != nil {
		t.Fatal(err)
	}
	return 0o777 &^ info.Mode().Perm()
}

func TestWriteFileRefusesAPathThatLeavesTheRoot(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tr.outside, "b.sysml")
	for _, path := range []string{"../outside/b.sysml", outside, "link.sysml", "dirlink/b.sysml", "dangle.sysml", "dirlink/new.sysml"} {
		if replaced, err := r.WriteFile(path, []byte("package Hijacked;")); err == nil || !strings.Contains(err.Error(), "outside the workspace root") {
			t.Errorf("WriteFile(%s) = %v, %v; want a refusal naming the root", path, replaced, err)
		}
	}
	if got, _ := os.ReadFile(outside); string(got) != "package B;" {
		t.Errorf("the file outside the root was written: %q", got)
	}
	entries, _ := os.ReadDir(tr.outside)
	if len(entries) != 1 {
		t.Errorf("the outside directory holds %d entries; want only b.sysml", len(entries))
	}
}

func TestWriteFileFollowsALinkRepointedWithinTheRoot(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	// A link that stays inside the root is written through, as a save over a
	// linked model updates the model rather than unlinking it.
	replaced, err := r.WriteFile("copy.sysml", []byte("package Through;"))
	if err != nil || !replaced {
		t.Fatalf("WriteFile through a link inside the root = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(filepath.Join(tr.root, "a.sysml")); string(got) != "package Through;" {
		t.Errorf("the link's target = %q; want it written", got)
	}
	if info, err := os.Lstat(filepath.Join(tr.root, "copy.sysml")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced by a file: %v, %v", info, err)
	}
	if replaced, err := r.WriteFile("dangle-in.sysml", []byte("package Placed;")); err != nil || replaced {
		t.Fatalf("WriteFile through a dangling link inside the root = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(filepath.Join(tr.root, "missing.sysml")); string(got) != "package Placed;" {
		t.Errorf("the dangling link's target = %q; want it created", got)
	}
}

// An existing file in a directory the user cannot add entries to is still
// written, as export.WriteFile writes it: in place, since no temporary file fits.
func TestWriteFileFallsBackWhenTheDirectoryIsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	tr := newTree(t)
	root, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tr.root, "closed")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(path, []byte("package Longer;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	replaced, err := root.WriteFile(path, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "package Q;\n" {
		t.Errorf("file = %q, want the new model with nothing of the old one left", data)
	}
}

// A name near the 255-byte component limit is written: the temporary file
// beside it is named from a shortened stem, as export.WriteFile names its own.
func TestWriteFileWritesANameNearTheComponentLimit(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("m", 249) + ".sysml"
	fresh := filepath.Join(tr.root, long)
	if replaced, err := r.WriteFile(fresh, []byte("package New;")); err != nil || replaced {
		t.Fatalf("WriteFile to a new long name = %v, %v", replaced, err)
	}
	if replaced, err := r.WriteFile(fresh, []byte("package Newer;")); err != nil || !replaced {
		t.Fatalf("WriteFile over the long name = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(fresh); string(got) != "package Newer;" {
		t.Errorf("content = %q", got)
	}
	entries, _ := os.ReadDir(tr.root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("temporary %s left behind", e.Name())
		}
	}
}
