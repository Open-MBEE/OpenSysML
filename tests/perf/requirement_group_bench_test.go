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

// BenchmarkAnalyzeRequirementGroup validates a requirement that requires n
// sibling requirements, each `require` member named by its referent: the
// time must grow linearly with n.
func BenchmarkAnalyzeRequirementGroup(b *testing.B) {
	for _, n := range []int{8, 16, 32, 64} {
		var src strings.Builder
		src.WriteString("package P {\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, "\trequirement r%d;\n", i)
		}
		src.WriteString("\trequirement grp {\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, "\t\trequire r%d;\n", i)
		}
		src.WriteString("\t}\n}\n")
		model := []byte(src.String())
		b.Run(fmt.Sprintf("members=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				root := parser.New(source.New("m.sysml", model)).ParseFile()
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
				if diags := passes.Analyze("m.sysml", root, nil, idx); len(diags) != 0 {
					b.Fatalf("diagnostics: %v", diags)
				}
			}
		})
	}
}
