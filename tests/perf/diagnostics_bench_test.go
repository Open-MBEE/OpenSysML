package perf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// BenchmarkAnalyzeManyDiagnostics analyzes a model whose n usages each name an unresolved
// type, so the diagnostic collection and dedup paths run: the time must grow linearly with n.
func BenchmarkAnalyzeManyDiagnostics(b *testing.B) {
	for _, n := range []int{100, 400, 1600} {
		var src strings.Builder
		src.WriteString("package P {\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, "\tpart p%d : Missing%d;\n", i, i)
		}
		src.WriteString("}\n")
		root := parser.New(source.New("m.sysml", []byte(src.String()))).ParseFile()
		b.Run(fmt.Sprintf("diagnostics=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
				if diags := passes.Analyze("m.sysml", root, nil, idx); len(diags) < n {
					b.Fatalf("%d diagnostics, want at least %d", len(diags), n)
				}
			}
		})
	}
}

// Analysis of a ring of packages that each import the next one's unresolved T by
// name and its members by wildcard, the one after privately (repeat times), and
// the third from a nested package: every lookup of T goes round the ring.
func BenchmarkAnalyzeImportRing(b *testing.B) {
	for _, c := range []struct{ ring, repeat int }{{5, 12}, {6, 1}, {6, 12}} {
		var src strings.Builder
		for i := 0; i < c.ring; i++ {
			fmt.Fprintf(&src, "package C%d { public import C%d::T; public import C%d::*;", i, (i+1)%c.ring, (i+1)%c.ring)
			for j := 0; j < c.repeat; j++ {
				fmt.Fprintf(&src, " private import C%d;", (i+2)%c.ring)
			}
			fmt.Fprintf(&src, " package In { public import C%d::*; } }\n", (i+3)%c.ring)
		}
		fmt.Fprintf(&src, "package C%d { part def T; }\npackage U { private import C0::*; part u : T; }\n", c.ring)
		root := parser.New(source.New("m.sysml", []byte(src.String()))).ParseFile()
		b.Run(fmt.Sprintf("ring=%d/repeat=%d", c.ring, c.repeat), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
				idx.ExpandWildcardImports()
				if diags := passes.Analyze("m.sysml", root, nil, idx); len(diags) < c.ring {
					b.Fatalf("%d diagnostics, want at least %d", len(diags), c.ring)
				}
			}
		})
	}
}
