package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func shapeTestModel(t *testing.T) (*Model, *symbols.Symbol) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		private import ScalarValues::*;
		action def Proc {
			in a : Integer; in b : Integer; out c : Integer;
			attribute d : Integer = 1; attribute e : Integer = 2;
			assign c := a + b + d + e;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	syms := idx.LookupQualified("test::Proc")
	if len(syms) != 1 {
		t.Fatalf("test::Proc resolves to %d symbols", len(syms))
	}
	return typedModel(semantics.NewModel(resolver), resolver), syms[0]
}

// TestPerformanceShapeIsSharedAcrossRuns pins that two runs on one worker's Model
// perform the same lowered graph and read one shape, while each holds its own values.
func TestPerformanceShapeIsSharedAcrossRuns(t *testing.T) {
	model, proc := shapeTestModel(t)
	var roots []*actionFrame
	var graphs []*lower.ActionGraph
	var results []map[string]Value
	for run := 0; run < 2; run++ {
		ctx := NewContext(model, 1_000_000)
		exec, err := ctx.CreateActionExecutorWithInputs(proc, nil, map[string]Value{"a": constInt(int64(run)), "b": constInt(10)})
		if err != nil {
			t.Fatal(err)
		}
		for exec.State() != StateCompleted {
			if err := exec.Step(); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
		}
		out, err := exec.ResultsWithError()
		if err != nil {
			t.Fatal(err)
		}
		roots = append(roots, exec.root)
		graphs = append(graphs, exec.graph)
		results = append(results, out)
	}
	if graphs[0] != graphs[1] {
		t.Fatal("two runs lowered the action twice")
	}
	if len(model.actionGraphs) != 1 {
		t.Fatalf("model memoizes %d graphs, want one", len(model.actionGraphs))
	}
	shape := &model.actionGraphs[proc].shape
	if shape.features == nil || model.actionGraphs[proc].graph != graphs[0] {
		t.Fatal("the lowered graph's shape is not memoized for the action")
	}
	for i, root := range roots {
		if !root.sharedShape {
			t.Errorf("run %d's root frame does not share the shape", i)
		}
		if len(root.declared) != len(shape.declared) || len(root.features) != len(shape.features) {
			t.Errorf("run %d's root holds %d features and %d declarations, the shape %d and %d",
				i, len(root.features), len(root.declared), len(shape.features), len(shape.declared))
		}
	}
	for i, want := range []int64{13, 14} {
		if got := soleElement(results[i]["c"]); got.Kind != ValConst || got.Const.Int != want {
			t.Errorf("run %d: c = %s, want %d", i, DescribeValue(got), want)
		}
	}
}

// TestPerformanceShapeIsCopiedBeforeAWrite pins that a frame declaring a feature of
// its own, as a flow's accept payload is, writes a copy and not the shared shape.
func TestPerformanceShapeIsCopiedBeforeAWrite(t *testing.T) {
	model, proc := shapeTestModel(t)
	ctx := NewContext(model, 1_000_000)
	exec, err := newActionExecutor(ctx, proc, nil)
	if err != nil {
		t.Fatal(err)
	}
	shape := &model.actionGraphs[proc].shape
	held := len(shape.features)
	root := exec.root
	root.ownShape()
	root.features["payload"] = ast.DirNone
	if root.sharedShape {
		t.Error("the root frame still marks its maps shared after taking copies")
	}
	if _, leaked := shape.features["payload"]; leaked || len(shape.features) != held {
		t.Fatalf("the shared shape holds the frame's own feature")
	}
	if _, own := root.features["payload"]; !own || len(root.features) != held+1 {
		t.Fatalf("the frame does not hold its own feature")
	}
	other, err := newActionExecutor(NewContext(model, 1_000_000), proc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := other.root.features["payload"]; leaked {
		t.Fatal("a later run sees the earlier run's own feature")
	}
}

// TestPerformanceShapeOfAGraphTheModelDidNotLowerIsNotMemoized pins that a graph built
// outside the Model (a state's do-flow block) gets its shape without entering the memo.
func TestPerformanceShapeOfAGraphTheModelDidNotLowerIsNotMemoized(t *testing.T) {
	model, proc := shapeTestModel(t)
	ctx := NewContext(model, 1_000_000)
	exec, err := newActionExecutor(ctx, proc, nil)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := lower.ToActionGraph(proc.Decl, DeclScope(proc))
	if err != nil {
		t.Fatal(err)
	}
	exec.graph = graph
	shape := exec.performanceShape()
	if shape == nil || len(shape.features) == 0 {
		t.Fatal("no shape built for a graph the Model did not lower")
	}
	if lowered := model.actionGraphs[proc]; &lowered.shape == shape || lowered.graph == graph {
		t.Fatal("a graph the Model did not lower entered the memo")
	}
	if len(model.actionGraphs) != 1 || len(model.actionInterfaces) != 0 {
		t.Fatalf("model memoizes %d graphs and %d interfaces, want the lowered graph alone", len(model.actionGraphs), len(model.actionInterfaces))
	}
}

