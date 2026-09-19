package parser

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var update = flag.Bool("update", false, "update golden files")

func runGolden(t *testing.T, name string) {
	t.Helper()
	srcPath := filepath.Join("..", "..", "..", "tests", "testdata", "parse", name+".sysml")
	goldenPath := filepath.Join("..", "..", "..", "tests", "testdata", "parse", name+".golden")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read %s: %v", srcPath, err)
	}
	p := New(source.New(name+".sysml", data))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics parsing %s: %+v", name, p.Diagnostics)
	}
	got := strings.TrimSpace(ast.Dump(root)) + "\n"

	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", goldenPath, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenNamespaces(t *testing.T)  { runGolden(t, "namespaces") }
func TestGoldenExpressions(t *testing.T) { runGolden(t, "expressions") }
