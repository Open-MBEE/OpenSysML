package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// BenchmarkNewRootFrame is the frame an action execution starts from: its held
// index takes every feature of the action, inherited ones included (61 for a
// plain action def), read from the Model's memoized shape after the first run.
func BenchmarkNewRootFrame(b *testing.B) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<bench>", parser.New(source.New("<bench>", []byte(`package bench {
		private import ScalarValues::*;
		action def Proc {
			in a : Integer; in b : Integer; out c : Integer;
			attribute d : Integer = 1; attribute e : Integer = 2;
			assign c := a + b + d + e;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 1_000_000)
	exec, err := newActionExecutor(ctx, idx.LookupQualified("bench::Proc")[0], nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if root := exec.newRootFrame(exec.performanceShape()); len(root.declared) == 0 {
			b.Fatal("root frame holds no features")
		}
	}
}
