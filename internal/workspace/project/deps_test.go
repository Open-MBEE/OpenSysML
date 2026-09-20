package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeModel(t *testing.T, path, text string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDependenciesFollowImportsThroughSiblings(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    private import Lib::*;\n    part w : Widget;\n}\n")
	lib := writeModel(t, filepath.Join(dir, "parts", "lib.sysml"), "package Lib {\n    import Base::*;\n    part def Widget :> Thing;\n}\n")
	base := writeModel(t, filepath.Join(dir, "base.kerml"), "package Base {\n    class Thing;\n}\n")
	writeModel(t, filepath.Join(dir, "other.sysml"), "package Other {\n    part def Unused;\n}\n")
	writeModel(t, filepath.Join(dir, ".hidden", "shadow.sysml"), "package Lib {\n    part def Widget;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{lib, base}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

func TestDependenciesSkipWhatIsDeclaredOrKnown(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    private import ScalarValues::*;\n    private import Local::*;\n    package Local { part def A; }\n    private import Named::*;\n}\n")
	writeModel(t, filepath.Join(dir, "scalars.sysml"), "package ScalarValues {\n    part def Fake;\n}\n")
	writeModel(t, filepath.Join(dir, "local.sysml"), "package Local {\n    part def B;\n}\n")
	named := writeModel(t, filepath.Join(dir, "named.sysml"), "package Named {\n    part def C;\n}\n")

	known := func(name string) bool { return name == "ScalarValues" }
	got := Dependencies([]string{main, named}, known)
	if len(got) != 0 {
		t.Errorf("Dependencies = %v, want none: the library, the file itself and a named file declare every import", got)
	}
}

func TestDependenciesLoadEveryFileDeclaringTheRoot(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    private import P::*;\n}\n")
	a := writeModel(t, filepath.Join(dir, "a.sysml"), "package P {\n    part def Wheel;\n}\n")
	b := writeModel(t, filepath.Join(dir, "b.sysml"), "package P {\n    part def Axle;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{a, b}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

func TestDependenciesIgnoreStdinAndUnreadableFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.sysml")
	if got := Dependencies([]string{"-", missing}, nil); len(got) != 0 {
		t.Errorf("Dependencies = %v; want none", got)
	}
}
