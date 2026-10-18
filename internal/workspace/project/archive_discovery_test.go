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
