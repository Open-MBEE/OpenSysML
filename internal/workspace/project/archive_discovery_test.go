package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeArchive(t *testing.T, path string, members ...[2]string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, zipOf(t, members...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLibraryRootsAcceptAnArchive(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, filepath.Join(dir, "store", "probe.kpar"),
		[2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta})
	lone := writeModel(t, filepath.Join(dir, "lone.sysml"), "package Lone;\n")

	got := Discovery{Roots: []string{archive}}.Around([]string{lone})
	if len(got.Used) != 1 || got.Used[0].Dir != archive {
		t.Errorf("Around with an archive as a library root: used = %v, want the archive's project", projectDirs(got.Used))
	}
}

const (
	siblingsProject = `{"name": "Siblings", "version": "1.0.0"}`
	siblingsMeta    = `{"index": {"Main": "Main.sysml", "Lib": "lib/Lib.sysml"}, "created": "2026-01-01T00:00:00Z"}`
	siblingsMain    = "package Main {\n    private import Lib::*;\n    part w : Widget;\n}\n"
	siblingsLib     = "package Lib {\n    private import Main::*;\n    part def Widget;\n}\n"
)

func TestDependenciesOfAnArchiveMemberAreItsSiblings(t *testing.T) {
	archive := writeArchive(t, filepath.Join(t.TempDir(), "siblings.kpar"),
		[2]string{"Main.sysml", siblingsMain}, [2]string{"lib/Lib.sysml", siblingsLib},
		[2]string{".project.json", siblingsProject}, [2]string{".meta.json", siblingsMeta})
	main := MemberPath(archive, "Main.sysml")
	lib := MemberPath(archive, "lib/Lib.sysml")

	if got := Dependencies([]string{main}, nil); !reflect.DeepEqual(got, []string{lib}) {
		t.Errorf("Dependencies of a top-level member = %v, want its sibling %v", got, []string{lib})
	}
	if got := Dependencies([]string{lib}, nil); !reflect.DeepEqual(got, []string{main}) {
		t.Errorf("Dependencies of a nested member = %v, want its sibling %v", got, []string{main})
	}
}

func TestDependenciesOfAnArchiveMemberIncludeTheProjectsTheArchiveUses(t *testing.T) {
	dir := t.TempDir()
	gears := writeProject(t, filepath.Join(dir, "store", "acme-gears"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	gearsModel := writeModel(t, filepath.Join(gears, "Gears.sysml"), "package Gears {\n    part def Gear;\n}\n")
	outer := writeProject(t, filepath.Join(dir, "outer"), `{"name": "Outer", "version": "1.0.0"}`)
	archive := writeArchive(t, filepath.Join(outer, "vendor", "probe.kpar"),
		[2]string{"Probe.sysml", "package Probe {\n    private import Gears::*;\n    part g : Gear;\n}\n"},
		[2]string{".project.json", probeProject},
		[2]string{".meta.json", `{"index": {"Probe": "Probe.sysml"}, "created": "2026-01-01T00:00:00Z"}`})

	got := Discovery{Roots: []string{filepath.Join(dir, "store")}}.Dependencies([]string{MemberPath(archive, "Probe.sysml")}, nil)
	if !reflect.DeepEqual(got, []string{gearsModel}) {
		t.Errorf("Dependencies of an archive member inside another project = %v, want the file of the project the archive uses %v", got, []string{gearsModel})
	}
}

func TestAProjectDirectoryNamedLikeAnArchiveIsReadAsADirectory(t *testing.T) {
	proj := writeProject(t, filepath.Join(t.TempDir(), "probe.kpar"), probeProject)
	main := writeModel(t, filepath.Join(proj, "Probe.sysml"), probeModel)

	got := Discovery{}.Around([]string{main})
	if len(got.Enclosing) != 1 || got.Enclosing[0].Dir != proj || got.Enclosing[0].Name != "Probe" {
		t.Errorf("Around in a project directory named like an archive: enclosing = %v, want the directory's project", projectDirs(got.Enclosing))
	}
}
