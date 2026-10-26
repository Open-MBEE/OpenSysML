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

func TestDependenciesResolveImportsInTheirOwnScope(t *testing.T) {
	dir := t.TempDir()
	// Nested A::Lib does not satisfy B's import; Q's own Inner satisfies Deep's
	// but not the $:: import beside it.
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package A { package Lib; }\npackage B {\n    private import Lib::*;\n    part w : Widget;\n}\npackage Q {\n    package Inner { part def X; }\n    package Deep { private import Inner::*; }\n    package Global { private import $::Glob::*; }\n    package Glob;\n}\n")
	glob := writeModel(t, filepath.Join(dir, "glob.sysml"), "package Glob {\n    part def G;\n}\n")
	lib := writeModel(t, filepath.Join(dir, "lib.sysml"), "package Lib {\n    part def Widget;\n}\n")
	writeModel(t, filepath.Join(dir, "inner.sysml"), "package Inner {\n    part def Y;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{glob, lib}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

func TestDependenciesFollowSymlinkedDirectories(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "model", "main.sysml"), "package Main {\n    private import Shared::*;\n}\n")
	shared := writeModel(t, filepath.Join(dir, "elsewhere", "shared.sysml"), "package Shared {\n    part def S;\n}\n")
	link := filepath.Join(dir, "model", "vendor")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "model"), filepath.Join(dir, "elsewhere", "back")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got := Dependencies([]string{main}, nil)
	if want := []string{filepath.Join(link, "shared.sysml")}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v (through the link to %s)", got, want, shared)
	}
}

func TestDependenciesIgnoreStdinAndUnreadableFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.sysml")
	if got := Dependencies([]string{"-", missing}, nil); len(got) != 0 {
		t.Errorf("Dependencies = %v; want none", got)
	}
}

// A file in a project reaches what lies under the project's lib/ only through
// the usages: the version the usage selects, not every installed one.
func TestDependenciesReadTheLibraryDirectoryThroughTheUsages(t *testing.T) {
	proj := writeProject(t, filepath.Join(t.TempDir(), "proj"), `{"name": "demo", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "pkg:registry/acme/gears@1.0.0"}]}`)
	main := writeModel(t, filepath.Join(proj, "main.sysml"), "package Main {\n    private import Gears::*;\n    part g : Thing;\n}\n")
	gears1 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.0.0"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	gears1File := writeModel(t, filepath.Join(gears1, "Gears.kerml"), "package Gears {\n    class Thing;\n}\n")
	gears2 := writeProject(t, filepath.Join(proj, "lib", "acme-gears_2.0.0"), `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`)
	writeModel(t, filepath.Join(gears2, "Gears.kerml"), "package Gears {\n    class Thing;\n    class Newer;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{gears1File}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

// A qualified name whose first segment no loaded file declares is followed as
// an import is: the file declaring that root loads, and so do the roots it
// references in turn. The decoy declaring the same member under another root
// is left alone.
func TestDependenciesFollowQualifiedNamesAsTheyFollowImports(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    private import Lib::*;\n    part w : Widget;\n}\n")
	lib := writeModel(t, filepath.Join(dir, "lib.sysml"), "package Lib {\n    part def Widget {\n        attribute kind : Kinds::Kind;\n    }\n}\n")
	kinds := writeModel(t, filepath.Join(dir, "kinds.sysml"), "package Kinds {\n    attribute def Kind { attribute weight : Units::Weight; }\n}\n")
	units := writeModel(t, filepath.Join(dir, "units.kerml"), "package Units {\n    datatype Weight;\n}\n")
	writeModel(t, filepath.Join(dir, "other.sysml"), "package Other {\n    attribute def Kind;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{lib, kinds, units}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

// Two library packages that import each other, one of them naming the other's
// members only by qualified name, both load from a file that imports one of
// them: the qualified names lead to the second as an import would have.
func TestDependenciesFollowAQualifiedNameIntoAMutuallyImportingLibrary(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    private import Method::*;\n    part p : Project;\n}\n")
	method := writeModel(t, filepath.Join(dir, "lib", "Method.sysml"), "library package Method {\n    part def Project {\n        view cover : MethodViews::ProjectView { expose this; }\n    }\n}\n")
	views := writeModel(t, filepath.Join(dir, "lib", "MethodViews.sysml"), "library package MethodViews {\n    private import Method::*;\n    view def ProjectView;\n}\n")

	got := Dependencies([]string{main}, nil)
	if want := []string{method, views}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies = %v, want %v", got, want)
	}
}

// A qualified name an enclosing namespace declares the root of, a feature
// chain and a name the library declares are not looked for.
func TestDependenciesLeaveQualifiedNamesTheFileOrLibraryAnswers(t *testing.T) {
	dir := t.TempDir()
	main := writeModel(t, filepath.Join(dir, "main.sysml"), "package Main {\n    package Local { part def A; }\n    part a : Local::A;\n    part b : ScalarValues::Real;\n    part c = a.x;\n}\n")
	writeModel(t, filepath.Join(dir, "local.sysml"), "package Local {\n    part def B;\n}\n")
	writeModel(t, filepath.Join(dir, "scalars.sysml"), "package ScalarValues {\n    part def Fake;\n}\n")
	writeModel(t, filepath.Join(dir, "a.sysml"), "package a {\n    part def X;\n}\n")

	known := func(name string) bool { return name == "ScalarValues" }
	if got := Dependencies([]string{main}, known); len(got) != 0 {
		t.Errorf("Dependencies = %v, want none", got)
	}
}
