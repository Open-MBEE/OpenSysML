package libs

import (
	"os"
	"path/filepath"
	"testing"
)

// The source SharedLibrary hands out serves the bytes its index was built from:
// editing an override file afterwards must change neither, or the index's spans
// would address text they were not computed over.
func TestSharedLibraryServesTheBytesItsIndexWasBuiltFrom(t *testing.T) {
	dir := writeLibrary(t, bodyLibrary)
	t.Setenv(LibraryPathEnvVar, dir)

	idx, src := SharedLibrary()
	names := src.List()
	if len(names) != 1 {
		t.Fatalf("List() = %v, want the one file of the fixture", names)
	}
	name := names[0]
	before, err := src.Read(name)
	if err != nil {
		t.Fatalf("Read(%q): %v", name, err)
	}
	if string(before) != bodyLibrary {
		t.Fatalf("Read(%q) = %q, want the fixture text", name, before)
	}
	if !idx.IsLibraryDocument(name) {
		t.Fatalf("%q is not a library document of the shared index", name)
	}

	edited := "package Lib { datatype Edited; }\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Added.kerml"), []byte("package Added;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	again, src2 := SharedLibrary()
	if again != idx || src2 != src {
		t.Fatal("SharedLibrary() returned another library for the same path")
	}
	after, err := src.Read(name)
	if err != nil {
		t.Fatalf("Read(%q) after the edit: %v", name, err)
	}
	if string(after) != bodyLibrary {
		t.Errorf("Read(%q) after the edit = %q, want the text the index was built from", name, after)
	}
	if got := src.List(); len(got) != 1 || got[0] != name {
		t.Errorf("List() after adding a file = %v, want %v", got, names)
	}
	if _, err := src.Read("Added.kerml"); err == nil {
		t.Error("Read of a file added after the index was built succeeded, want an error")
	}
	if len(idx.LookupQualified("Lib::Edited")) != 0 {
		t.Error("the shared index picked up the edit")
	}
}
