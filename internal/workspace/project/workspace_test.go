package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestReferencedRootsListsImportsAndQualifiedNamesNoEnclosingNamespaceDeclares(t *testing.T) {
	src := `private import Lib::*;
package Mine {
    private import ISQ::mass;
    import $::Global::*;
    part def Car { attribute m : ISQ::MassValue; part w : Wheels::Wheel; ref r = Mine::Car; attribute n = car.mass; }
    part car : Car;
    part other : Other::Thing;
    private import Parts::*[@Rules::Approved];
}
`
	root := parser.New(source.New("t.sysml", []byte(src))).ParseFile()
	got := ReferencedRoots(root)
	want := []string{"Lib", "ISQ", "Global", "Wheels", "Other", "Parts", "Rules"}
	if !slices.Equal(got, want) {
		t.Errorf("ReferencedRoots = %v, want %v", got, want)
	}
}

func TestWorkspaceIndexFindsTheFilesDeclaringANameAndFollowsChanges(t *testing.T) {
	dir := t.TempDir()
	lib := writeModel(t, filepath.Join(dir, "lib.sysml"), "package Lib { attribute a = 1; }\n")
	writeModel(t, filepath.Join(dir, "sub", "more.kerml"), "package More;\npackage Lib;\n")
	writeModel(t, filepath.Join(dir, ".hidden", "lib.sysml"), "package Lib;\n")
	writeModel(t, filepath.Join(dir, "notes.txt"), "package Lib;\n")

	ix := Discovery{}.NewWorkspaceIndex(dir)
	got := ix.Declaring([]string{"Lib", "More", "Nope"}, nil)
	if !slices.Equal(got["Lib"], []string{lib, filepath.Join(dir, "sub", "more.kerml")}) || !slices.Equal(got["More"], []string{filepath.Join(dir, "sub", "more.kerml")}) {
		t.Errorf("Declaring = %v", got)
	}
	if _, ok := got["Nope"]; ok {
		t.Errorf("a name no file declares is present: %v", got)
	}
	got = ix.Declaring([]string{"Lib"}, func(path string) bool { return path == lib })
	if !slices.Equal(got["Lib"], []string{filepath.Join(dir, "sub", "more.kerml")}) {
		t.Errorf("Declaring with lib.sysml excluded = %v", got)
	}

	// A rewritten file is read again; one removed drops out.
	writeModel(t, lib, "package Renamed { attribute a = 1; }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(lib, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "sub", "more.kerml")); err != nil {
		t.Fatal(err)
	}
	got = ix.Declaring([]string{"Lib", "Renamed", "More"}, nil)
	if _, ok := got["Lib"]; ok || !slices.Equal(got["Renamed"], []string{lib}) {
		t.Errorf("after the rewrite, Declaring = %v", got)
	}
	if _, ok := got["More"]; ok {
		t.Errorf("a removed file still declares: %v", got)
	}
}

func TestWorkspaceIndexReadsAFileOnceWhileItsStampHolds(t *testing.T) {
	dir := t.TempDir()
	lib := writeModel(t, filepath.Join(dir, "lib.sysml"), "package Lib;\n")
	ix := Discovery{}.NewWorkspaceIndex(dir)
	ix.Declaring([]string{"Lib"}, nil)
	before := ix.entries[lib]
	ix.Declaring([]string{"Lib"}, nil)
	if ix.entries[lib] != before {
		t.Error("an unchanged file was read again")
	}
}

func TestWorkspaceIndexHonorsTheProjectsTheRootUses(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, filepath.Join(dir, ProjectFile), `{"name": "App", "version": "1.0.0", "usage": [{"resource": "pkg:sysand/acme/gears"}]}`)
	writeModel(t, filepath.Join(dir, ".sysand", "lib", "gears", ProjectFile), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	gears := writeModel(t, filepath.Join(dir, ".sysand", "lib", "gears", "gears.sysml"), "package Gears { part def Gear; }\n")
	writeModel(t, filepath.Join(dir, ".sysand", "lib", "unused", ProjectFile), `{"name": "Unused", "publisher": "acme", "version": "1.0.0"}`)
	writeModel(t, filepath.Join(dir, ".sysand", "lib", "unused", "unused.sysml"), "package Unused;\n")

	ix := Discovery{}.NewWorkspaceIndex(dir)
	got := ix.Declaring([]string{"Gears", "Unused"}, nil)
	if !slices.Equal(got["Gears"], []string{gears}) {
		t.Errorf("Gears declared by %v, want %s", got["Gears"], gears)
	}
	if _, ok := got["Unused"]; ok {
		t.Errorf("a project the root does not use was searched: %v", got)
	}
}

func TestRelativeSpellsAPathUnderTheRootRelativeToIt(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ws")
	if got := Relative(root, filepath.Join(root, "sub", "a.sysml")); got != filepath.Join("sub", "a.sysml") {
		t.Errorf("Relative = %q", got)
	}
	outside := filepath.Join(string(filepath.Separator), "elsewhere", "a.sysml")
	if got := Relative(root, outside); got != outside {
		t.Errorf("Relative outside = %q", got)
	}
}
