package passes

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func libraryRootNameDiags(t *testing.T, src string) int {
	t.Helper()
	var count int
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Code == "library-root-name" {
			count++
		}
	}
	return count
}

func TestLibraryRootNameWarns(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want int
	}{
		{"root package named like a library package", `package Requirements {}`, 1},
		{"short name like a library package", `package <Requirements> Reqs {}`, 1},
		{"nested package is not a root", `package P { package Requirements {} }`, 0},
		{"unrelated root name", `package MyReqs {}`, 0},
		{"root alias named like a library package", `package P { part def Pump; } alias Views for P;`, 1},
		{"root alias short name like a library package", `alias <Views> V for ScalarValues;`, 1},
		{"nested alias is not a root", `package P { alias Views for P; }`, 0},
		{"root alias with unrelated name", `alias V for ScalarValues;`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := libraryRootNameDiags(t, tc.src); got != tc.want {
				t.Errorf("got %d library-root-name diagnostics, want %d", got, tc.want)
			}
		})
	}
}

// TestLibraryRootNameCorpus analyzes every committed model under examples/ and
// tests/testdata/ against the standard library and expects no library-root-name
// diagnostic: the warning is a portability hazard a real model should not trip.
// Downloaded corpora are skipped: they are not committed and are covered by
// their own gates.
func TestLibraryRootNameCorpus(t *testing.T) {
	idx := symbols.NewIndex()
	ld := libs.NewLoader(libs.DefaultSource(), nil)
	if err := ld.LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	var files []string
	for _, dir := range []string{"../../../examples", "../../../tests/testdata"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "pilot-corpora", "sysml-v2-training", "pssm":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		root := parser.New(source.New(path, data)).ParseFile()
		idx.AddDocument(path, root)
		idx.ExpandWildcardImports()
		for _, d := range Analyze(path, root, nil, idx) {
			if d.Code == "library-root-name" {
				t.Errorf("%s: %d:%d %s", path, d.Span.Offset, d.Span.Len, d.Message)
			}
		}
	}
}
