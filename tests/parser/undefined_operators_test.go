package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestUndefinedOperatorsCoversFixtures checks the parser's `~` record against
// an independent walk of every parse fixture.
func TestUndefinedOperatorsCoversFixtures(t *testing.T) {
	fixtures := filepath.Join("testdata", "parse")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatalf("Failed to read fixtures dir %s: %v", fixtures, err)
	}

	counted := 0
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".sysml" && ext != ".kerml") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(fixtures, name))
			if err != nil {
				t.Fatalf("Failed to read fixture %s: %v", name, err)
			}
			sf := source.New(name, content)
			root := parser.New(sf).ParseFile()

			want := 0
			ast.Inspect(root, func(n ast.Node) bool {
				if e, ok := n.(*ast.OperatorExpr); ok && e.Operator == ast.OpBitNot {
					want++
				}
				return true
			})
			if len(root.UndefinedOperators) != want {
				t.Errorf("UndefinedOperators len = %d, tree walk counts %d", len(root.UndefinedOperators), want)
			}
			for _, e := range root.UndefinedOperators {
				if e == nil || e.Operator != ast.OpBitNot {
					t.Errorf("recorded operator is not a `~` expression: %v", e)
				}
			}
			counted += want
		})
	}
	if counted == 0 {
		t.Error("no fixture exercises a `~` operator; the comparison is vacuous")
	}
}
