package resolve

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var update = flag.Bool("update", false, "update resolve golden files")

func runResolveGolden(t *testing.T, name string) {
	t.Helper()
	base := filepath.Join("..", "..", "..", "tests", "testdata", "resolve", name)
	srcBytes, err := os.ReadFile(base + ".sysml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sf := source.New(name+".sysml", srcBytes)
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name+".sysml", root)
	r := New(idx)
	r.ResolveDocument(name+".sysml", root)

	diags := append([]Diagnostic(nil), r.Diagnostics...)
	sort.Slice(diags, func(i, j int) bool {
		return diags[i].Span.Offset < diags[j].Span.Offset
	})
	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("(no diagnostics)\n")
	}
	for _, d := range diags {
		pos := sf.Lines().PosAt(d.Span.Offset)
		fmt.Fprintf(&b, "%d:%d %s\n", pos.Line, pos.Col, d.Message)
	}
	got := b.String()

	goldenPath := base + ".golden"
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestResolveGoldenBasic(t *testing.T)  { runResolveGolden(t, "basic") }
func TestResolveGoldenErrors(t *testing.T) { runResolveGolden(t, "errors") }
