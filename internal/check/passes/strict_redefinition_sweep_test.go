package passes

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
		filepath.Join("..", "..", "..", "internal", "workspace", "libs", "stdlib"),
	}
	wantFile := filepath.Join("..", "..", "..", "tests", "testdata", "passes", "strict_redefinition_resolves_to_nothing.sysml")
	corpusRoots := []string{
		filepath.Join(roots[0], "pilot-corpora"),
		filepath.Join(roots[0], "sysml-v2-training"),
	}
	type model struct {
		name string
		doc  *ast.RootNamespace
	}
	var models []model
	corpusChecked := 0
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
			models = append(models, model{name, parser.New(source.New(name, data)).ParseFile()})
			for _, cr := range corpusRoots {
				if strings.HasPrefix(path, cr+string(filepath.Separator)) {
					corpusChecked++
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(models) == 0 {
		t.Fatal("no models were swept")
	}
	downloaded := false
	for _, cr := range corpusRoots {
		if st, err := os.Stat(cr); err == nil && st.IsDir() {
			downloaded = true
		}
	}
	if downloaded && corpusChecked < 20 {
		t.Fatalf("only %d OMG corpus models were swept, want at least 20", corpusChecked)
	}
	// One index for every document: cross-file types must resolve as they do in a workspace.
	idx := newTestIndex()
	for _, m := range models {
		idx.AddDocument(m.name, m.doc)
	}
	idx.ExpandWildcardImports()
	for _, m := range models {
		ctx := NewContextWithOptions(m.name, source.KindOf(m.name), idx, nil, Options{Conformance: diag.ConformanceStrict})
		pass := StrictRedefinitionPass{}
		for _, dg := range pass.Run(ctx, m.name, m.doc) {
			if dg.Code != CodeRedefinitionResolvesToNothing {
				continue
			}
			if m.name == wantFile {
				continue
			}
			t.Errorf("%s: unexpected %s: %s", m.name, dg.Code, dg.Message)
		}
	}
	t.Logf("swept %d models (%d OMG corpus)", len(models), corpusChecked)
}
