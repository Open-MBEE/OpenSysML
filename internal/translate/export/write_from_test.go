package export

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// A produced document is created or replaced whole, and its size reported.
func TestWriteFileFromCreatesAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	pieces := func(parts ...string) func(io.Writer) error {
		return func(w io.Writer) error {
			for _, part := range parts {
				if _, err := io.WriteString(w, part); err != nil {
					return err
				}
			}
			return nil
		}
	}
	replaced, written, err := WriteFileFrom(path, pieces("[\n", "]\n"))
	if err != nil || replaced || written != 4 {
		t.Fatalf("create: replaced %v, written %d, err %v", replaced, written, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	replaced, written, err = WriteFileFrom(path, pieces("a", "bc", "def"))
	if err != nil || !replaced || written != 6 {
		t.Fatalf("replace: replaced %v, written %d, err %v", replaced, written, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "abcdef" {
		t.Fatalf("content %q, err %v", data, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the replaced file's mode is %v (%v), want the 0600 it had", info.Mode().Perm(), err)
	}
}

// Output that fails part-way leaves the destination as it was and nothing beside it,
// and the error is the producer's own, not a failure to write the path.
func TestWriteFileFromLeavesTheDestinationWhenProductionFails(t *testing.T) {
	dir := t.TempDir()
	broken := errors.New("the model does not convert")
	failing := func(w io.Writer) error {
		if _, err := io.WriteString(w, "[\n  {"); err != nil {
			return err
		}
		return broken
	}
	existing := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(existing, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{existing, filepath.Join(dir, "new.json")} {
		replaced, _, err := WriteFileFrom(path, failing)
		if err != broken || replaced {
			t.Errorf("%s: replaced %v, err %v; want the producer's error unwrapped", filepath.Base(path), replaced, err)
		}
	}
	if data, _ := os.ReadFile(existing); string(data) != "kept" {
		t.Errorf("the existing file now holds %q", data)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Errorf("the directory holds %d entries (%v), want the existing file alone", len(entries), err)
	}
}

// A destination that is not a regular file is written in place, and only once the
// whole output was produced.
func TestWriteFileFromWritesANonRegularDestinationInPlace(t *testing.T) {
	if _, err := os.Stat(os.DevNull); err != nil {
		t.Skip("no null device")
	}
	replaced, written, err := WriteFileFrom(os.DevNull, func(w io.Writer) error {
		_, err := io.WriteString(w, "abc")
		return err
	})
	if err != nil || replaced || written != 3 {
		t.Errorf("replaced %v, written %d, err %v", replaced, written, err)
	}
	if info, err := os.Stat(os.DevNull); err != nil || info.Mode().IsRegular() {
		t.Errorf("the null device was replaced by a regular file (%v)", err)
	}
	if _, _, err := WriteFileFrom(t.TempDir(), func(io.Writer) error { return nil }); err == nil {
		t.Error("a directory was accepted as a destination")
	}
}
