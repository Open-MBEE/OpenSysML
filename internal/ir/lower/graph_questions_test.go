package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
)

// TestGraphQuestionsAnswerByID pins what the id-addressed questions answer for each
// way a node is written: its form, its trace label and whether it performs another action.
func TestGraphQuestionsAnswerByID(t *testing.T) {
	graph := actionGraphOf(t, `action outer {
		first start;
		action work;
		action call : Other;
		fork split;
		join gather;
		merge again;
		decide pick;
		assign x := 1;
		done;
		succession first start then work;
		succession first work then call;
	}`)
	type answer struct {
		form    NodeForm
		invokes bool
	}
	want := map[string]answer{
		"start":  {FormInitial, false},
		"work":   {FormUsage, false},
		"call":   {FormUsage, true},
		"split":  {FormFork, false},
		"gather": {FormJoin, false},
		"again":  {FormMerge, false},
		"pick":   {FormDecision, false},
		"done":   {FormFinal, false},
	}
	seen := map[string]bool{}
	assigns := 0
	for _, id := range graph.NodeIDs() {
		if id == 0 {
			t.Fatalf("a node has no id")
		}
		label := graph.TraceLabel(id)
		if graph.FormOf(id) == FormAssign {
			assigns++
			continue
		}
		w, ok := want[label]
		if !ok {
			t.Errorf("unexpected node %q (%s)", label, graph.FormName(id))
			continue
		}
		seen[label] = true
		if got := (answer{graph.FormOf(id), graph.Invokes(id)}); got != w {
			t.Errorf("%s: %+v, want %+v", label, got, w)
		}
	}
	if len(seen) != len(want) || assigns != 1 {
		t.Errorf("saw %v and %d assignments, want every one of %d and 1", seen, assigns, len(want))
	}
	if got := graph.TraceLabel(graph.InitialID()); got != "start" {
		t.Errorf("initial node is labelled %q", got)
	}
	var none program.NodeID
	if graph.FormOf(none) != FormOther || graph.Invokes(none) || graph.TraceLabel(none) != "nil" || graph.ExpressionOf(none) != 0 {
		t.Errorf("the zero id answers for a node")
	}
}

// TestRecordQuestions pins the questions the lowered records answer about how they were written.
func TestRecordQuestions(t *testing.T) {
	graph := actionGraphOf(t, `action outer {
		first start;
		action work {
			while true { assign x := 1; }
			loop { assign y.z := 2; } until false;
			for i in (1, 2) { send S() to p; }
		}
		succession flow start.a to work.b;
		succession first start then work;
	}`)
	var loops []Loop
	var assigns []Assign
	var sends []Send
	var walk func(stmts []Statement)
	walk = func(stmts []Statement) {
		for _, st := range stmts {
			switch s := st.(type) {
			case Loop:
				loops = append(loops, s)
				walk(s.Body.Statements)
			case Assign:
				assigns = append(assigns, s)
			case Send:
				sends = append(sends, s)
			}
		}
	}
	for _, id := range graph.NodeIDs() {
		walk(graph.BodyOf(id))
	}
	if len(loops) != 3 || len(assigns) != 2 || len(sends) != 1 {
		t.Fatalf("lowered %d loops, %d assignments, %d sends", len(loops), len(assigns), len(sends))
	}
	for i, want := range []struct {
		word      string
		forLoop   bool
		postTests bool
	}{{"while", false, false}, {"loop", false, true}, {"for", true, false}} {
		l := loops[i]
		if l.Word() != want.word || l.For() != want.forLoop || l.PostTested() != want.postTests {
			t.Errorf("loop %d: %s for=%v post=%v, want %+v", i, l.Word(), l.For(), l.PostTested(), want)
		}
		if !l.Same(loops[i]) || l.Same(loops[(i+1)%3]) {
			t.Errorf("loop %d is not told from the next", i)
		}
	}
	if assigns[0].Chained() || !assigns[1].Chained() {
		t.Errorf("chained: %v %v, want only the second", assigns[0].Chained(), assigns[1].Chained())
	}
	if got := sends[0].Signal(); got != "S" {
		t.Errorf("send signals %q, want S", got)
	}
	var flows []ObjectFlow
	for _, id := range graph.NodeIDs() {
		flows = append(flows, graph.FlowsOf(id)...)
	}
	if len(flows) != 1 {
		t.Fatalf("lowered %d flows, want 1", len(flows))
	}
	flow := flows[0]
	if span, ok := flow.Written(); !ok || span != flow.Decl.Span() || flow.Gated() {
		t.Errorf("flow: written %v %v gated %v", span, ok, flow.Gated())
	}
	if _, ok := (ObjectFlow{}).Written(); ok || (ObjectFlow{}).Gated() {
		t.Errorf("an undeclared flow answers as written or gated")
	}
	carrying := 0
	for _, id := range graph.NodeIDs() {
		for _, edge := range graph.EdgesOf(id) {
			if edge.Carrying(flow) {
				carrying++
			}
			if edge.CountsTarget() {
				t.Errorf("an edge stating no multiplicity counts its target")
			}
		}
	}
	if carrying != 1 {
		t.Errorf("%d edges carry the flow, want 1", carrying)
	}
}
