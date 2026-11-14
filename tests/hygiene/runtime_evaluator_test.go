package hygiene

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// TestEvaluatorsDoNotImportTheSyntaxTree keeps the evaluator and the calc compiler
// on lowered expression records: neither file names a syntax-tree type.
func TestEvaluatorsDoNotImportTheSyntaxTree(t *testing.T) {
	const tree = "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	for _, name := range []string{"eval.go", "compile.go"} {
		path := filepath.Join("..", "..", "internal", "exec", "runtime", name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, spec := range file.Imports {
			if imported, _ := strconv.Unquote(spec.Path.Value); imported == tree {
				t.Errorf("internal/exec/runtime/%s imports %s", name, tree)
			}
		}
	}
}
