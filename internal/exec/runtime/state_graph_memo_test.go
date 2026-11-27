package runtime

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestStateGraphIsLoweredOncePerModel pins that every execution of a machine on one
// Model reads one lowered graph, while each holds its own attribute values.
func TestStateGraphIsLoweredOncePerModel(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		private import ScalarValues::*;
		state def Machine {
			attribute count : Integer = 0;
			entry; then idle;
			state idle;
			transition idle_to_busy first idle if count >= 0 do assign count := count + 1 then busy;
			state busy;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	syms := idx.LookupQualified("test::Machine")
	if len(syms) != 1 {
		t.Fatalf("test::Machine resolves to %d symbols", len(syms))
	}
	machine := syms[0]
	model := typedModel(semantics.NewModel(resolver), resolver)

	var execs []*StateExecutor
	for run := 0; run < 2; run++ {
		ctx := NewContext(model, 1_000_000)
		exec, err := ctx.CreateStateExecutor(machine)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		execs = append(execs, exec)
	}
	if execs[0].graph != execs[1].graph {
		t.Fatal("two executions lowered the machine twice")
	}
	if len(model.stateGraphs) != 1 || model.stateGraphs[machine] != execs[0].graph {
		t.Fatalf("model memoizes %d graphs, want the machine's one", len(model.stateGraphs))
	}
	for run, exec := range execs {
		data, err := exec.StateDataWithError()
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(data["count"]); got != fmt.Sprint(constInt(1)) {
			t.Errorf("run %d: count = %s, want 1", run, got)
		}
	}
}
