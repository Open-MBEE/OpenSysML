package project

import (
	"path/filepath"
	"testing"
)

// TestExpandStdinIsNeitherStattedNorGlobbed checks that a lone "-" survives
// expansion as the stream, even where a file of that name sits beside it.
func TestExpandStdinIsNeitherStattedNorGlobbed(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "-"), "package Dash { }\n")
	t.Chdir(dir)

	got, err := Expand([]string{Stdin})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 1 || got[0] != Stdin {
		t.Fatalf("Expand = %v, want [%s]", got, Stdin)
	}
}

// TestExpandKeepsStdinApartFromAFileNamedDash checks that naming both the stream
// and the file called "-" beside it loads both, neither taken for the other.
func TestExpandKeepsStdinApartFromAFileNamedDash(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "-"), "package Dash { }\n")
	t.Chdir(dir)

	got, err := Expand([]string{Stdin, "./-"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 2 || got[0] != Stdin || got[1] != "./-" {
		t.Fatalf("Expand = %v, want [%s ./-]", got, Stdin)
	}
}

// TestReadFileNamesWhatItRead checks that a file is reported under the path that
// named it, which is what diagnostics about it are counted from.
func TestReadFileNamesWhatItRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.sysml")
	write(t, file, "package A { }\n")

	name, data, err := ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if name != file {
		t.Errorf("name = %q, want %q", name, file)
	}
	if string(data) != "package A { }\n" {
		t.Errorf("data = %q", data)
	}
}

// TestIsStdin checks that only a lone "-" names the stream, so a file whose name
// merely starts with one is still read from disk.
func TestIsStdin(t *testing.T) {
	for _, path := range []string{"-"} {
		if !IsStdin(path) {
			t.Errorf("IsStdin(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"./-", "-.sysml", "--", "", "a-"} {
		if IsStdin(path) {
			t.Errorf("IsStdin(%q) = true, want false", path)
		}
	}
}
