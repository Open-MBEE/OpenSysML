package smt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// conformanceDir is the interpreter's conformance corpus, the referee's cases.
const conformanceDir = "../runtime/testdata/conformance"

// fixture indexes one document over the standard library and returns a context
// over it, the way the solver tests do.
func fixture(t *testing.T, path, src string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	sf := source.New(path, []byte(src))
	idx.AddDocument(path, parser.New(sf).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	ctx := runtime.NewContext(model, 10000)
	ctx.Model().RegisterSource(sf)
	return ctx, idx
}

// conformanceAction lowers the named action of a conformance case.
func conformanceAction(t *testing.T, file, fqn string) *lower.ActionGraph {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(conformanceDir, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	_, idx := fixture(t, file, string(src))
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	graph, err := lower.ToActionGraph(matches[0].Decl, matches[0].Scope)
	if err != nil {
		t.Fatalf("lower %s: %v", fqn, err)
	}
	lower.StartFlow(graph)
	return graph
}

// TestAnalyzeNumbersForkJoinFlow: a fork of two branches into a join numbers
// every node and succession once, labels them as a trace does, and sizes two
// token slots for the two branches in flight.
func TestAnalyzeNumbersForkJoinFlow(t *testing.T) {
	graph := conformanceAction(t, "action_fork_branches_write_one_feature.sysml", "test::clash")
	f, err := Analyze(graph, 10)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got, want := len(f.Nodes), 6; got != want {
		t.Fatalf("nodes: got %d, want %d: %v", got, want, f.Labels)
	}
	if got, want := len(f.Edges), 6; got != want {
		t.Fatalf("edges: got %d, want %d", got, want)
	}
	if f.Slots != 2 || f.Cyclic {
		t.Errorf("slots: got %d cyclic=%v, want 2 acyclic", f.Slots, f.Cyclic)
	}
	for i, edge := range f.Edges {
		if f.EdgeIndex[edge] != i {
			t.Errorf("edge %d indexed as %d", i, f.EdgeIndex[edge])
		}
	}
	labels := map[string]bool{}
	for _, label := range f.Labels {
		if labels[label] {
			t.Errorf("label %q repeated", label)
		}
		labels[label] = true
	}
	for _, want := range []string{"start", "split", "left", "right", "sync", "done"} {
		if !labels[want] {
			t.Errorf("no node labelled %q among %v", want, f.Labels)
		}
	}
	if len(f.Loops) != 0 {
		t.Errorf("body loops: got %d, want none", len(f.Loops))
	}
}

// TestAnalyzeRefusesMessages: a flow sending and accepting a message is refused
// before any encoding at the first such node in graph order, as ErrNotEncoded.
func TestAnalyzeRefusesMessages(t *testing.T) {
	graph := conformanceAction(t, "action_accept_message.sysml", "test::communicator")
	_, err := Analyze(graph, 10)
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) || !errors.Is(err, ErrNotEncoded) {
		t.Fatalf("Analyze: got %v, want an UnsupportedError", err)
	}
	if unsupported.Node != "sender" || unsupported.Construct != "send" {
		t.Errorf("refusal names %q/%q, want node sender, construct send", unsupported.Node, unsupported.Construct)
	}
	delete(graph.Bodies, graph.Nodes[1])
	_, err = Analyze(graph, 10)
	if !errors.As(err, &unsupported) || unsupported.Node != "receiver" || unsupported.Construct != "accept" {
		t.Errorf("with the send gone: got %v, want node receiver, construct accept", err)
	}
}

// TestAnalyzeRecordsBodyLoops: a body `while` is recorded as a loop to unroll,
// under the node whose body it is in.
func TestAnalyzeRecordsBodyLoops(t *testing.T) {
	_, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::*;
			action counter {
				attribute n : Integer = 0;
				first start;
				action count { while n < 3 { assign n := n + 1; } }
				done;
				succession first start then count;
				succession first count then done;
			}
		}`)
	sym := idx.LookupQualified("test::counter")
	if len(sym) != 1 {
		t.Fatalf("test::counter matched %d symbols", len(sym))
	}
	graph, err := lower.ToActionGraph(sym[0].Decl, sym[0].Scope)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	f, err := Analyze(graph, 5)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(f.Loops) != 1 || f.Loops[0].Label != "count" {
		t.Fatalf("loops: got %+v, want one under count", f.Loops)
	}
	if f.Slots != 1 {
		t.Errorf("slots: got %d, want 1", f.Slots)
	}
}

// TestAnalyzeSizesSlotsPerArrival: a fork that several merge arrivals reach
// adds its branches once per arrival; a fork past a join, once per firing; a
// fork on a cycle, once per move; and no flow gets more than a fork per move
// could make.
func TestAnalyzeSizesSlotsPerArrival(t *testing.T) {
	_, idx := fixture(t, "<test>", `
		package test {
			action twice {
				first start;
				fork f1; action a; action b; merge m; fork f2;
				action c; action d; merge m2; done;
				succession first start then f1;
				succession first f1 then a;
				succession first f1 then b;
				succession first a then m;
				succession first b then m;
				succession first m then f2;
				succession first f2 then c;
				succession first f2 then d;
				succession first c then m2;
				succession first d then m2;
				succession first m2 then done;
			}
			action synced {
				first start;
				fork f1; action a; action b; join j; fork f2;
				action c; action d; join j2; done;
				succession first start then f1;
				succession first f1 then a;
				succession first f1 then b;
				succession first a then j;
				succession first b then j;
				succession first j then f2;
				succession first f2 then c;
				succession first f2 then d;
				succession first c then j2;
				succession first d then j2;
				succession first j2 then done;
			}
			action cycling {
				first start;
				merge m; fork f; action a; action b; done;
				succession first start then m;
				succession first m then f;
				succession first f then a;
				succession first f then b;
				succession first a then m;
				succession first b then done;
			}
		}`)
	for _, c := range []struct {
		action string
		k      int
		slots  int
		cyclic bool
	}{
		{"test::twice", 16, 4, false},
		{"test::twice", 2, 3, false},
		{"test::synced", 16, 3, false},
		{"test::cycling", 10, 11, true},
	} {
		sym := idx.LookupQualified(c.action)
		if len(sym) != 1 {
			t.Fatalf("%s matched %d symbols", c.action, len(sym))
		}
		graph, err := lower.ToActionGraph(sym[0].Decl, sym[0].Scope)
		if err != nil {
			t.Fatalf("lower %s: %v", c.action, err)
		}
		lower.StartFlow(graph)
		f, err := Analyze(graph, c.k)
		if err != nil {
			t.Fatalf("Analyze %s: %v", c.action, err)
		}
		if f.Slots != c.slots || f.Cyclic != c.cyclic {
			t.Errorf("%s within %d moves: %d slots cyclic=%v, want %d cyclic=%v", c.action, c.k, f.Slots, f.Cyclic, c.slots, c.cyclic)
		}
	}
}

// TestAnalyzeRefusesNoInitial: a flow with no initial node is what the
// interpreter refuses at initialize, and the encoding refuses it as malformed.
func TestAnalyzeRefusesNoInitial(t *testing.T) {
	if _, err := Analyze(&lower.ActionGraph{}, 3); !errors.Is(err, ErrMalformedFlow) {
		t.Fatalf("Analyze: got %v, want ErrMalformedFlow", err)
	}
	if _, err := Analyze(nil, 3); !errors.Is(err, ErrMalformedFlow) {
		t.Fatalf("Analyze(nil): got %v, want ErrMalformedFlow", err)
	}
}

// TestSortsNameEveryNodeEdgeAndSlot: the finite sorts carry one constructor per
// node plus Absent, per succession plus none, per slot plus stutter.
func TestSortsNameEveryNodeEdgeAndSlot(t *testing.T) {
	graph := conformanceAction(t, "action_fork_branches_write_one_feature.sysml", "test::clash")
	f, err := Analyze(graph, 10)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	sorts := newSorts("clash", f)
	if got, want := len(sorts.Node.Values), len(f.Nodes)+1; got != want {
		t.Errorf("node sort: %d constructors, want %d", got, want)
	}
	if got, want := len(sorts.Edge.Values), len(f.Edges)+1; got != want {
		t.Errorf("edge sort: %d constructors, want %d", got, want)
	}
	if got, want := len(sorts.Choice.Values), f.Slots+1; got != want {
		t.Errorf("choice sort: %d constructors, want %d", got, want)
	}
	s := newState(sorts, f, 3)
	if len(s.Slots) != f.Slots || s.Slots[1].At.Name != "at[1]@3" || s.Slots[1].At.Sort.Name != sorts.Node.Name {
		t.Errorf("state slots: %+v", s.Slots)
	}
	if s.Overflow != nil {
		t.Errorf("an acyclic flow declares an overflow flag")
	}
	if s.Now != nil || len(s.Bus) != 0 || s.BusOverflow != nil || s.Slots[0].Parked != nil || s.Slots[0].Due != nil {
		t.Errorf("a flow without accepts or sends declares clock or bus variables: %+v", s)
	}
	if len(f.Frames) != 1 || f.Frames[0].Graph != graph || f.Frames[0].Slots != f.Slots || f.Nested() {
		t.Errorf("a flat flow numbers %d frames", len(f.Frames))
	}
}

// TestStateVectorNamesEveryVariableAcrossMoves: two states of one encoding
// list the same names in the same order, each naming that state's own copy,
// the flag of a feature that may hold no value included.
func TestStateVectorNamesEveryVariableAcrossMoves(t *testing.T) {
	graph := conformanceAction(t, "action_fork_branches_write_one_feature.sysml", "test::clash")
	f, err := Analyze(graph, 10)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	sorts := newSorts("clash", f)
	features := []*solve.Var{intVar("x"), intVar("y")}
	flagged := map[string]bool{"y": true}
	before, after := newState(sorts, f, 0).Vector(features, flagged), newState(sorts, f, 1).Vector(features, flagged)
	if len(before) != len(after) || len(before) != 4*f.Slots+5 {
		t.Fatalf("vectors of %d and %d variables, want %d", len(before), len(after), 4*f.Slots+5)
	}
	for i := range before {
		if before[i].Name != after[i].Name {
			t.Errorf("entry %d: %q at move 0, %q at move 1", i, before[i].Name, after[i].Name)
		}
		if before[i].Var.Name != before[i].Name+"@0" || after[i].Var.Name != after[i].Name+"@1" {
			t.Errorf("entry %d: %q holds %q and %q", i, before[i].Name, before[i].Var.Name, after[i].Var.Name)
		}
	}
	n := len(before)
	if before[0].Name != "at[0]" || before[n-3].Name != "x" || before[n-2].Name != "y" || before[n-1].Name != "has(y)" {
		t.Errorf("vector starts %q, ends %q %q %q", before[0].Name, before[n-3].Name, before[n-2].Name, before[n-1].Name)
	}
}

// TestAnalyzeRefusesANestedFlowBeforeLookingInside: a node stating a flow of
// its own is refused as a nested flow whether that flow is well formed or not.
func TestAnalyzeRefusesANestedFlowBeforeLookingInside(t *testing.T) {
	for _, tc := range []struct {
		name, leg string
		noGraph   bool
	}{
		{name: "well formed", leg: "first a; action a; action b; succession first a then b;"},
		{name: "missing step", leg: "first a; action a; succession first a then missing;"},
		{name: "no start", leg: "action a; action b; succession first a then b; succession first b then a;"},
		{name: "no graph", leg: "first a; action a;", noGraph: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, idx := fixture(t, "<test>", `
				package test {
					action outer {
						first start;
						action leg { `+tc.leg+` }
						done;
						succession first start then leg;
						succession first leg then done;
					}
				}`)
			sym := idx.LookupQualified("test::outer")
			if len(sym) != 1 {
				t.Fatalf("test::outer matched %d symbols", len(sym))
			}
			graph, err := lower.ToActionGraph(sym[0].Decl, sym[0].Scope)
			if err != nil {
				t.Fatalf("lower: %v", err)
			}
			if graph.Subflows[graph.Nodes[1]] == nil {
				t.Fatalf("leg states no flow of its own: %+v", graph.Subflows)
			}
			if tc.noGraph {
				graph.Subflows[graph.Nodes[1]] = &lower.Subflow{}
			}
			_, err = Analyze(graph, 10)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) || !errors.Is(err, ErrNotEncoded) {
				t.Fatalf("Analyze: got %v, want an UnsupportedError", err)
			}
			if unsupported.Node != "leg" || unsupported.Construct != "nested flow" {
				t.Errorf("refusal names %q/%q, want node leg, construct nested flow", unsupported.Node, unsupported.Construct)
			}
		})
	}
}
