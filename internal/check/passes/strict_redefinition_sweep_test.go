package passes

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestStrictRedefinitionCommittedModelSweep keeps the strict-only finding off
// committed models; the intentional repro fixture alone is excluded.
func TestStrictRedefinitionCommittedModelSweep(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "..", "examples"),
		filepath.Join("..", "..", "..", "tests", "testdata"),
	}
	wantFile := filepath.Join("..", "..", "..", "tests", "testdata", "passes", "strict_redefinition_resolves_to_nothing.sysml")
	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || (filepath.Ext(path) != ".sysml" && filepath.Ext(path) != ".kerml") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(path)
			doc := parser.New(source.New(name, data)).ParseFile()
			idx := newTestIndexFromDoc(name, doc)
			idx.ExpandWildcardImports()
			ctx := NewContextWithOptions(name, source.KindOf(name), idx, nil, Options{Conformance: diag.ConformanceStrict})
			pass := StrictRedefinitionPass{}
			for _, dg := range pass.Run(ctx, name, doc) {
				if dg.Code != CodeRedefinitionResolvesToNothing {
					continue
				}
				if path == wantFile {
					continue
				}
				t.Errorf("%s: unexpected %s: %s", path, dg.Code, dg.Message)
			}
			checked++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked == 0 {
		t.Fatal("no models were swept")
	}
	t.Logf("swept %d models", checked)
}
