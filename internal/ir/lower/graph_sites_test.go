package lower

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const sitesSource = `
	action run {
		action def Work { }
		attribute n = 1;
		first start;
		then fork f;
		then action grind : Work;
		then action wait accept after n;
		then join j;
		then action choose { if n > 0 { perform Work; } else { assign n := 2; } }
		then done;
		first f then j;
		bind grind.out1 = choose.in1;
	}
`

// TestSiteOfNamesTheNodesOfAFlow pins what an export calls each node of a flow
// and where, by id.
func TestSiteOfNamesTheNodesOfAFlow(t *testing.T) {
	graph := identityActionGraph(t, sitesSource)
	want := []Site{
		{Kind: "start", Name: "start"},
		{Kind: "fork", Name: "f"},
		{Kind: "action usage", Name: "grind"},
		{Kind: "action usage", Name: "wait"},
		{Kind: "join", Name: "j"},
		{Kind: "action usage", Name: "choose"},
		{Kind: "end", Name: "done"},
	}
	ids := graph.NodeIDs()
	if len(ids) != len(want) {
		t.Fatalf("the flow has %d nodes, want %d", len(ids), len(want))
	}
	for i, id := range ids {
		site, ok := graph.SiteOf(id)
		want[i].Span = graph.Origins[id].Span
		if !ok || site != want[i] || site.Span.Len == 0 {
			t.Errorf("SiteOf(%d) = %+v, %v; want %+v", id, site, ok, want[i])
		}
	}
	if site, ok := graph.SiteOf(0); ok || site != (Site{}) {
		t.Errorf("SiteOf(0) = %+v, %v; want no site", site, ok)
	}
}

// TestFlowRecordsSayWhereTheyWereWritten pins the extents of the successions,
// bindings and statements of a flow, and that what was not written has none.
func TestFlowRecordsSayWhereTheyWereWritten(t *testing.T) {
	graph := identityActionGraph(t, sitesSource)
	named := map[string]program.NodeID{}
	for _, id := range graph.NodeIDs() {
		named[graph.Origins[id].Name] = id
	}
	if edges := graph.EdgesOf(named["f"]); len(edges) != 2 {
		t.Fatalf("the fork has %d successions, want 2", len(edges))
	} else {
		for _, edge := range edges {
			if at := edge.At(); !at.Written || at.Span.Len == 0 {
				t.Errorf("succession out of f: At = %+v, want its declaration", at)
			}
			if gate := edge.GateAt(); gate != (Extent{}) {
				t.Errorf("succession out of f: GateAt = %+v, want none", gate)
			}
		}
	}
	if len(graph.Bindings) != 2 {
		t.Fatalf("the flow has %d bindings, want 2", len(graph.Bindings))
	}
	for _, b := range graph.Bindings {
		at, other := b.At(), b.OtherAt()
		if !at.Written || !other.Written || other.Span.Offset < at.Span.Offset || other.Span.Offset+other.Span.Len > at.Span.Offset+at.Span.Len {
			t.Errorf("binding: At = %+v, OtherAt = %+v; want the other end within the declaration", at, other)
		}
		if b.PathNames() != nil || b.OtherPathNames() != nil {
			t.Errorf("binding: paths %v, %v; want none", b.PathNames(), b.OtherPathNames())
		}
	}
	body := graph.BodyOf(named["choose"])
	if len(body) != 1 {
		t.Fatalf("choose has %d statements, want 1", len(body))
	}
	branch, ok := body[0].(If)
	if !ok {
		t.Fatalf("choose states %T, want an If", body[0])
	}
	whole := StatementAt(branch)
	if !whole.Written || whole.Span.Len == 0 {
		t.Errorf("StatementAt(if) = %+v, want its place", whole)
	}
	for _, s := range append(append([]Statement{}, branch.Then.Statements...), branch.Else.Statements...) {
		at := StatementAt(s)
		if !at.Written || at.Span.Offset <= whole.Span.Offset || at.Span.Offset+at.Span.Len > whole.Span.Offset+whole.Span.Len {
			t.Errorf("StatementAt(%T) = %+v, want a place within the if at %+v", s, at, whole)
		}
	}
	if at := StatementAt(Assign{}); at != (Extent{}) {
		t.Errorf("StatementAt of a statement lowering supplied = %+v, want none", at)
	}
}

// TestPerformedByResolvesWhatANodePerforms pins the behaviors a typed node and a
// `perform` of a block name, and that a node naming none has none.
func TestPerformedByResolvesWhatANodePerforms(t *testing.T) {
	graph := identityActionGraph(t, sitesSource)
	for _, id := range graph.NodeIDs() {
		var got []string
		for _, sym := range graph.PerformedBy(id, graph.Resolver(), graph.Scope) {
			got = append(got, symbols.FQNOf(sym))
		}
		var want []string
		if graph.Origins[id].Name == "grind" {
			want = []string{"run::Work"}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s performs %v, want %v", graph.Origins[id].Name, got, want)
		}
	}
	var members []FlowNode
	for _, id := range graph.NodeIDs() {
		members = append(members, graph.BlockNodesOf(id)...)
	}
	if len(members) != 1 {
		t.Fatalf("the blocks of the flow declare %d nodes, want the perform alone", len(members))
	}
	block, id := members[0].Graph, members[0].ID
	if site, ok := block.SiteOf(id); !ok || site.Kind != "action usage" || site.Name != "Work" {
		t.Errorf("the block's node: SiteOf = %+v, %v; want the usage performing Work", site, ok)
	}
	for what, got := range map[string][]*symbols.Symbol{
		"node":   block.PerformedBy(id, graph.Resolver(), graph.Scope),
		"effect": Effect{Kind: EffectPerform, Node: block.Provenance.Node(id)}.Performed(graph.Resolver(), graph.Scope),
	} {
		if len(got) != 1 || symbols.FQNOf(got[0]) != "run::Work" {
			t.Errorf("the perform as a %s names %v, want run::Work", what, got)
		}
	}
}

// TestTriggerWrittenDescribesATimer pins the trigger of a timed accept and the
// nodes a footprint converges on, by id.
func TestTriggerWrittenDescribesATimer(t *testing.T) {
	graph := identityActionGraph(t, sitesSource)
	named := map[string]program.NodeID{}
	for _, id := range graph.NodeIDs() {
		named[graph.Origins[id].Name] = id
	}
	accept, ok := graph.AcceptOf(named["wait"])
	if !ok {
		t.Fatal("wait is not an accept")
	}
	trigger, ok := accept.TriggerWritten()
	if !ok || trigger.Kind != "time" || trigger.Absolute || !trigger.Expr.Written || trigger.Expr.Span.Len != 1 {
		t.Errorf("TriggerWritten = %+v, %v; want a relative timer over n", trigger, ok)
	}
	if trigger.Expr.Span.Offset <= trigger.Span.Offset || accept.SignalName() != "" || accept.SubsetsAt() != (Extent{}) {
		t.Errorf("trigger %+v, signal %q, subsets %+v", trigger, accept.SignalName(), accept.SubsetsAt())
	}
	if _, ok := (Accept{}).TriggerWritten(); ok {
		t.Error("an accept stating no trigger has one")
	}
	fp, ok := graph.FootprintOf(named["f"])
	if !ok || !reflect.DeepEqual(graph.ControlIDs(fp), []program.NodeID{named["j"]}) {
		t.Errorf("the fork converges on %v, %v; want the join %d", graph.ControlIDs(fp), ok, named["j"])
	}
	if _, ok := graph.FootprintOf(0); ok {
		t.Error("no node has a footprint")
	}
}

// TestVertexSiteNamesTheVerticesOfAMachine pins what an export calls the states,
// pseudostates and regions of a machine, by id.
func TestVertexSiteNamesTheVerticesOfAMachine(t *testing.T) {
	graph := identityStateGraph(t)
	for _, id := range graph.StateIDs() {
		site, ok := graph.VertexSite(program.StateVertex(id))
		origin := graph.Origins[id]
		if !ok || site != (Site{Kind: "state", Name: origin.Name, Span: origin.Span}) {
			t.Errorf("state %d: VertexSite = %+v, %v; want %s", id, site, ok, origin.Name)
		}
		if graph.StateScope(id) == nil {
			t.Errorf("state %s owns no scope", origin.Name)
		}
	}
	machine, ok := graph.VertexSite(program.StateVertex(graph.MachineID()))
	if !ok || machine.Kind != "state" || machine.Name != "Machine" {
		t.Errorf("machine: VertexSite = %+v, %v", machine, ok)
	}
	pseudo := graph.PseudoIDs()
	if len(pseudo) != 1 {
		t.Fatalf("the machine has %d pseudostates, want 1", len(pseudo))
	}
	fork := program.PseudoVertex(pseudo[0])
	if site, ok := graph.VertexSite(fork); !ok || site.Kind != "pseudostate fork" || site.Name != "split" || site.Span != graph.PseudoOrigins[pseudo[0]].Span {
		t.Errorf("fork: VertexSite = %+v, %v", site, ok)
	}
	for _, trans := range graph.TransitionsOf(fork) {
		if at := trans.At(); !at.Written || at.Span.Len == 0 {
			t.Errorf("transition out of the fork: At = %+v", at)
		}
		if _, ok := trans.TriggerWritten(); ok {
			t.Error("a completion transition has a trigger")
		}
	}
	composite := graph.CompositeIDs()
	if len(composite) != 1 || graph.Origins[composite[0]].Name != "work" {
		t.Fatalf("CompositeIDs = %v, want work alone", composite)
	}
	for i, id := range graph.RegionsOf(composite[0]) {
		site, ok := graph.RegionSite(id)
		if !ok || site.Kind != "region" || site.Name != []string{"left", "right"}[i] || site.Span != graph.RegionOrigins[id].Span {
			t.Errorf("region %d: RegionSite = %+v, %v", id, site, ok)
		}
	}
	if _, ok := graph.VertexSite(program.VertexRef{}); ok {
		t.Error("no vertex has a site")
	}
	if _, ok := graph.RegionSite(0); ok {
		t.Error("no region has a site")
	}
}

// TestValuedIsTheExpressionADeclarationIsValuedBy pins the value of a valued
// declaration and that an unvalued one, or no declaration, has none.
func TestValuedIsTheExpressionADeclarationIsValuedBy(t *testing.T) {
	graph := identityActionGraph(t, sitesSource)
	n, ok := graph.Scope.LookupLocal("n")
	if !ok {
		t.Fatal("the flow declares no n")
	}
	value := Valued(n)
	if at := value.Extent(); value == nil || !at.Written || at.Span.Len != 1 || at.Span != value.Origin.Span {
		t.Fatalf("Valued(n) = %+v at %+v, want the literal 1", value, at)
	}
	if decl := Declared(n).Extent(); !decl.Written || decl.Span.Offset >= value.Origin.Span.Offset {
		t.Errorf("Declared(n) at %+v, want the declaration around its value", decl)
	}
	work, ok := graph.Scope.LookupLocal("Work")
	if !ok {
		t.Fatal("the flow declares no Work")
	}
	if Valued(work) != nil || Valued(nil) != nil {
		t.Error("an unvalued declaration has a value")
	}
	if at := (*ExprEntry)(nil).Extent(); at != (Extent{}) {
		t.Errorf("no expression is written at %+v", at)
	}
}
