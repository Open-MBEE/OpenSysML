package export

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A referenced document's declarations are checked by the subjects collect
// names, not by encoding it: over every model that converts, encoding mints
// each of them, so the check never refuses a model the full conversion writes.
func TestDeclaredSubjectsAreMinted(t *testing.T) {
	var paths []string
	for _, root := range []string{"../../../tests/export/testdata/convert", "../../../examples"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".sysml") && !strings.Contains(path, ".golden.") {
				paths = append(paths, path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	converted := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file := source.New(path, data)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			continue
		}
		e, err := newEncoder(file, root, "", IDQualifiedName)
		if err != nil {
			continue
		}
		declared := subjectIRIs(e.declaredSubjects())
		if err := e.encodeDocument(root); err != nil {
			continue
		}
		converted++
		if extra := without(declared, subjectIRIs(e.minted)); len(extra) > 0 {
			t.Errorf("%s: subjects declared but never minted: %v", path, extra)
		}
	}
	if converted == 0 {
		t.Fatal("no model converted")
	}
	t.Logf("%d of %d models compared", converted, len(paths))
}

func subjectIRIs(minted []mintedSubject) []string {
	out := make([]string, 0, len(minted))
	for _, m := range minted {
		out = append(out, m.iri)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func without(a, b []string) []string {
	var out []string
	for _, s := range a {
		if _, found := slices.BinarySearch(b, s); !found {
			out = append(out, s)
		}
	}
	return out
}
