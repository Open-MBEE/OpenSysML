package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
)

const declaredSource = `
	action run {
		attribute n = 1;
		attribute go = true;
		item def Ping { attribute level; }
		first start;
		then action produce { out value; assign value := n + 1; }
		then action consume { in value; }
		then action notify { send Ping(level = n) to produce; }
		then action repeat { while n < 3 { assign n := n + 1; } until n > 5; }
		then action choose { if n > 0 { assign n := 2; } }
		then action work { in amount = n; in fixed = 3; in open; }
		first produce if go then f;
		succession flow f from produce.value to consume.value;
		bind produce.value = consume.value;
	}
`

// textAt is the text an extent covers in the source, "" for one not written.
func textAt(src string, at Extent) string {
	if !at.Written {
		return ""
	}
	return src[at.Span.Offset : at.Span.Offset+at.Span.Len]
}

// TestActionRecordsAnswerForTheirDeclarations pins what a reader that draws a
// flow asks of its successions, bindings and statements without their tree fields.
func TestActionRecordsAnswerForTheirDeclarations(t *testing.T) {
	graph := identityActionGraph(t, declaredSource)
	named := map[string]program.NodeID{}
	for _, id := range graph.NodeIDs() {
		named[graph.Origins[id].Name] = id
	}

	gated, plain := 0, 0
	for _, id := range graph.NodeIDs() {
		for _, edge := range graph.EdgesOf(id) {
			if edge.Succession() != edge.Decl {
				t.Error("Succession is not the succession's own declaration")
			}
			if !edge.Gated() {
				plain++
				if edge.Declared() != edge.Decl || edge.GuardAt() != (Extent{}) {
					t.Errorf("ungated succession: Declared is not its own, or GuardAt = %+v", edge.GuardAt())
				}
				continue
			}
			gated++
			if edge.Declared() != edge.Gate || edge.Declared() == edge.Succession() {
				t.Error("gated succession: Declared is not the gating succession")
			}
			if got := textAt(declaredSource, edge.GuardAt()); got != "go" {
				t.Errorf("gated succession: guard written %q, want go", got)
			}
			if at, ok := edge.WeightAt(); ok || at != (Extent{}) {
				t.Errorf("unweighted succession: WeightAt = %+v, %v", at, ok)
			}
		}
	}
	if gated != 1 || plain != 6 {
		t.Errorf("the flow has %d gated and %d plain successions, want 1 and 6", gated, plain)
	}

	flows := graph.FlowsOf(named["produce"])
	if len(flows) != 1 || flows[0].Declared() != flows[0].Decl || flows[0].Declared() == nil {
		t.Fatalf("FlowsOf(produce) = %d flows, want one answering for its declaration", len(flows))
	}
	if len(graph.Bindings) == 0 {
		t.Fatal("the flow has no bindings")
	}
	for _, b := range graph.Bindings {
		if b.Declared() != b.Decl || !b.AtNode() || !b.OtherAtNode() || b.NodeID == 0 || b.OtherNodeID == 0 {
			t.Errorf("binding: Declared own %v, AtNode %v, OtherAtNode %v, ids %d, %d",
				b.Declared() == b.Decl, b.AtNode(), b.OtherAtNode(), b.NodeID, b.OtherNodeID)
		}
		if len(b.PathDecls()) != 0 || len(b.OtherPathDecls()) != 0 {
			t.Errorf("binding between nodes of the flow steps through %d and %d declarations", len(b.PathDecls()), len(b.OtherPathDecls()))
		}
	}

	statement := func(node string) Statement {
		t.Helper()
		body := graph.BodyOf(named[node])
		if len(body) != 1 {
			t.Fatalf("%s has %d statements, want 1", node, len(body))
		}
		return body[0]
	}
	assign, ok := statement("produce").(Assign)
	if !ok {
		t.Fatalf("produce states %T, want an Assign", statement("produce"))
	}
	target, value, ok := assign.Sides()
	if !ok || textAt(declaredSource, target) != "value" || textAt(declaredSource, value) != "n + 1" {
		t.Errorf("Sides = %q, %q, %v", textAt(declaredSource, target), textAt(declaredSource, value), ok)
	}
	if _, _, ok := (Assign{}).Sides(); ok {
		t.Error("an assignment lowering supplied has sides")
	}

	send, ok := statement("notify").(Send)
	if !ok {
		t.Fatalf("notify states %T, want a Send", statement("notify"))
	}
	if name, ok := send.Constructs(); !ok || name != "Ping" {
		t.Errorf("Constructs = %q, %v; want Ping", name, ok)
	}
	if got := textAt(declaredSource, send.MessageAt()); got != "Ping(level = n)" {
		t.Errorf("message written %q", got)
	}
	if got := textAt(declaredSource, send.TargetAt()); got != "produce" || send.ReceiverAt() != (Extent{}) {
		t.Errorf("a send to produce: target written %q, ReceiverAt = %+v", got, send.ReceiverAt())
	}
	if (Send{}).TargetAt() != (Extent{}) || (Send{}).MessageAt() != (Extent{}) {
		t.Error("a send lowering supplied says where it was written")
	}
	if name, ok := (Send{}).Constructs(); ok || name != "" {
		t.Errorf("a send of no message constructs %q", name)
	}

	loop, ok := statement("repeat").(Loop)
	if !ok {
		t.Fatalf("repeat states %T, want a Loop", statement("repeat"))
	}
	if cond, until := textAt(declaredSource, loop.ConditionAt()), textAt(declaredSource, loop.UntilAt()); cond != "n < 3" || until != "n > 5" {
		t.Errorf("loop condition %q until %q", cond, until)
	}
	if loop.CollectionAt() != (Extent{}) {
		t.Errorf("a while loop iterates %+v", loop.CollectionAt())
	}
	if loop.Body.Flows() || loop.Body.Declared() == nil {
		t.Errorf("loop body: Flows %v, Declared %v; want statements of a declared block", loop.Body.Flows(), loop.Body.Declared())
	}

	branch, ok := statement("choose").(If)
	if !ok {
		t.Fatalf("choose states %T, want an If", statement("choose"))
	}
	if got := textAt(declaredSource, branch.ConditionAt()); got != "n > 0" {
		t.Errorf("if condition written %q", got)
	}

	if (Return{}).ValueAt() != (Extent{}) || (Return{}).Syntax() != nil || (Effect{}).TargetAt() != (Extent{}) || (Assert{}).At() != (Extent{}) {
		t.Error("a statement lowering supplied says where it was written")
	}
}

// TestFeatureAnswersForItsValue pins what a feature says of the value it states.
func TestFeatureAnswersForItsValue(t *testing.T) {
	graph := identityActionGraph(t, declaredSource)
	var work program.NodeID
	for _, id := range graph.NodeIDs() {
		if graph.Origins[id].Name == "work" {
			work = id
		}
	}
	features := map[string]Feature{}
	for _, f := range graph.FeaturesOf(work) {
		features[f.Name] = f
	}
	amount, fixed, open := features["amount"], features["fixed"], features["open"]
	if amount.Declared() == nil || fixed.Declared() == nil || open.Declared() == nil {
		t.Fatalf("FeaturesOf(work) = %v, want amount, fixed and open", features)
	}
	if name, ok := amount.ValueName(); !amount.Bound() || !ok || name != "n" || amount.ValueLiteral() != "" {
		t.Errorf("amount: Bound %v, ValueName %q, %v, ValueLiteral %q", amount.Bound(), name, ok, amount.ValueLiteral())
	}
	if got := textAt(declaredSource, amount.ValueAt()); got != "n" {
		t.Errorf("amount: value written %q", got)
	}
	if name, ok := fixed.ValueName(); !fixed.Bound() || ok || name != "" || fixed.ValueLiteral() != "3" {
		t.Errorf("fixed: Bound %v, ValueName %q, %v, ValueLiteral %q", fixed.Bound(), name, ok, fixed.ValueLiteral())
	}
	if _, ok := open.ValueName(); open.Bound() || ok || open.ValueLiteral() != "" || open.ValueAt() != (Extent{}) {
		t.Errorf("open: Bound %v, ValueAt %+v; want no value", open.Bound(), open.ValueAt())
	}
}

// TestStateGraphAnswersForItsDeclarationsByID pins the declaration, document
// and start of each state and region by id, and that no id answers for none.
func TestStateGraphAnswersForItsDeclarationsByID(t *testing.T) {
	graph := identityStateGraph(t)
	named := map[string]program.StateID{}
	for _, id := range graph.StateIDs() {
		named[graph.Origins[id].Name] = id
		if graph.StateDecl(id) != graph.DeclOf(graph.Provenance.State(id)) || graph.StateDecl(id) == nil {
			t.Errorf("StateDecl(%s) is not the state's declaration", graph.Origins[id].Name)
		}
		if graph.Completing(id) {
			t.Errorf("Completing(%s) in a machine with no done vertex", graph.Origins[id].Name)
		}
		if got, want := graph.StartsBody(id), graph.Origins[id].Name == "idle"; got != want {
			t.Errorf("StartsBody(%s) = %v, want %v", graph.Origins[id].Name, got, want)
		}
	}
	unknown := program.StateID(len(graph.StateIDs()) + 100)
	if graph.StateDecl(unknown) != nil || graph.Completing(unknown) || graph.StartsBody(unknown) || graph.StartOfState(unknown) != nil {
		t.Error("an id the graph does not hold answers for a state")
	}
	own := graph.DocOf(nil)
	if graph.RegionDecl(0) != nil || graph.RegionDoc(0) != own || graph.VertexDoc(program.VertexRef{}) != own {
		t.Error("no region or vertex answers for a declaration, or for a document other than the machine's")
	}
	for _, id := range graph.RegionsOf(named["work"]) {
		if graph.RegionDecl(id) != graph.RegionDeclOf(graph.Provenance.Region(id)) || graph.RegionDecl(id) == nil {
			t.Errorf("RegionDecl(%d) is not the region's declaration", id)
		}
	}

	start := graph.StartOfState(0)
	if len(start) != 1 {
		t.Fatalf("the machine starts by %d entry transitions, want 1", len(start))
	}
	ref, name := start[0].TargetVertex()
	if ref != program.StateVertex(named["idle"]) || name != "idle" {
		t.Errorf("TargetVertex = %+v, %q; want idle", ref, name)
	}
	if start[0].Declared() != start[0].Decl || start[0].GuardAt() != (Extent{}) || graph.EntryDoc(start[0]) != graph.DocOf(start[0].Decl) {
		t.Error("the machine's entry transition does not answer for its declaration")
	}

	transitions := graph.TransitionsOf(program.StateVertex(named["idle"]))
	if len(transitions) != 1 {
		t.Fatalf("idle has %d transitions, want 1", len(transitions))
	}
	tr := transitions[0]
	if source, target := tr.EndNames(); source != "idle" || target != "split" {
		t.Errorf("EndNames = %q, %q; want idle, split", source, target)
	}
	if tr.Declared() != tr.Decl || tr.GuardAt() != (Extent{}) || graph.TransitionDoc(tr) != graph.DocOf(tr.Decl) {
		t.Error("the transition does not answer for its declaration")
	}
	if signal, ok := (Accept{}).Signal(); ok || signal != "" {
		t.Errorf("an accept naming no signal names %q", signal)
	}
}
