package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestCaseAndMixedFixturesHaveNoErrorDiagnostics(t *testing.T) {
	files := []string{"case.sysml", "mixed.sysml", "mixed-inherited-actor.sysml"}
	idx := libs.NewModelIndex()
	roots := make(map[string]*ast.RootNamespace, len(files))
	parseDiagnostics := make(map[string][]diag.Diagnostic, len(files))
	for _, file := range files {
		path := filepath.Join("testdata", file)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		p := parser.New(source.New(file, content))
		root := p.ParseFile()
		roots[file] = root
		parseDiagnostics[file] = parser.AsDiagnostics(p.Diagnostics, p.Warnings)
		idx.AddDocument(file, root)
	}
	idx.ExpandWildcardImports()

	for _, file := range files {
		for _, diagnostic := range passes.Analyze(file, roots[file], parseDiagnostics[file], idx) {
			if diagnostic.Severity == diag.SeverityError {
				t.Errorf("%s: %s", file, diagnostic.Message)
			}
		}
	}
}
