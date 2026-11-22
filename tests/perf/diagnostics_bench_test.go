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
