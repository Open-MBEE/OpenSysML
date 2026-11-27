package modelform

import (
	"bytes"
	"errors"
	"flag"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var update = flag.Bool("update", false, "rewrite the graphs goldens in testdata")

// graphsFixture holds every shape the lowered graphs carry: a flow with a fork,
// a join, a guarded decision, a nested body performing another action, and a
// machine with orthogonal regions, entry/do/exit behaviors, a call-triggered
// guarded transition with an effect; and a flow whose
// decision weights its successions.
const graphsFixture = `package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	action run {
		attribute total : Integer = 0;
		attribute doubled : Integer = 0;
		attribute battery : Integer = 10;

		first start;
		fork split;
		action left { assign total := 1; }
		action iterate {
			for i in 1..3 {
				perform Bump;
				assign total := total + doubled;
			}
		}
		join sync;
		decide check;
		action low { assign total := 7; }
		action idle { assign total := 8; }
		done;

		succession first start then split;
		succession first split then left;
		succession first split then iterate;
		succession first left then sync;
		succession first iterate then sync;
		succession first sync then check;
		if battery < 20 then low;
		else idle;
		succession first low then done;
		succession first idle then done;
	}

	action pick {
		attribute taken : Integer = 0;

		first start;
		decide route;
		action fast { assign taken := 1; }
		action slow { assign taken := 2; }
		done;

		succession first start then route;
		first route then fast { @Probability { p = 0.7; } }
		first route then slow { @Probability { p = 0.3; } }
		succession first fast then done;
		succession first slow then done;
	}

	action def Bump {
		in i : Integer[1];
		out doubled : Integer[1];

		first start;
		action compute { assign doubled := i * 2; }
		done;
		succession first start then compute;
		succession first compute then done;
	}

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Outer parallel {
			entry { assign log := log * 10 + 1; }
			do action watch { assign log := log + 1; }
			exit { assign log := log * 10 + 2; }

			state left {
				entry; then lstart;
				state lstart;
				state lit;
				succession first lstart then lit;
				transition first lit accept wake() then lstart;
			}
			state right {
				entry; then rstart;
				state rstart;
				state chiming;
				succession first rstart then chiming;
			}
		}
		state Done;

		succession first start then Outer;
		transition first Outer accept setSpeed(value) if value > 0 do assign log := log * 10 + 9 then Done;
	}
}
`

// graphsModel parses the fixture beside the standard library into a fresh index
// and returns a runtime model over it with the document registered.
func graphsModel(t *testing.T) (*runtime.Model, *symbols.Index) {
	t.Helper()
	return graphsModelOf(t, graphsFixture)
}

// graphsModelOf is graphsModel over the given fixture text.
func graphsModelOf(t *testing.T, fixture string) (*runtime.Model, *symbols.Index) {
	t.Helper()
	const path = "graphs.sysml"
	sf := source.New(path, []byte(fixture))
	p := parser.New(sf)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	if err := libs.NewLoader(libs.DefaultSource(), nil).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.RegisterSource(sf)
	return model, idx
}

func lookupOne(t *testing.T, idx *symbols.Index, name string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(name)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", name, len(matches))
	}
	return matches[0]
}

func exportGraphs(t *testing.T, model *runtime.Model, idx *symbols.Index, name string) (*Graphs, []byte) {
	t.Helper()
	g, err := GraphsOf(model, lookupOne(t, idx, name))
	if err != nil {
		t.Fatalf("GraphsOf(%s): %v", name, err)
	}
	data, err := MarshalGraphs(g)
	if err != nil {
		t.Fatalf("MarshalGraphs: %v", err)
	}
	return g, data
}

func TestGraphsMatchTheGoldens(t *testing.T) {
	model, idx := graphsModel(t)
	for _, subject := range []string{"test::run", "test::Machine"} {
		t.Run(subject, func(t *testing.T) {
			_, got := exportGraphs(t, model, idx, subject)
			golden := filepath.Join("testdata", map[string]string{
				"test::run":     "graphs_run.golden",
				"test::Machine": "graphs_machine.golden",
			}[subject])
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write %s: %v", golden, err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read %s: %v", golden, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("graphs bytes differ from %s", golden)
			}
		})
	}
}

func kinds(nodes []NodeForm) map[string]int {
	out := map[string]int{}
	for _, n := range nodes {
		out[n.Kind]++
	}
	return out
}

func TestGraphsActionCarriesTheLoweredGraph(t *testing.T) {
	model, idx := graphsModel(t)
	g, data := exportGraphs(t, model, idx, "test::run")
	if g.Version != GraphsVersion || g.Subject != "test::run" {
		t.Fatalf("version %d subject %q, want %d test::run", g.Version, g.Subject, GraphsVersion)
	}
	if !bytes.HasPrefix(data, []byte(`{"version":1,`)) {
		t.Fatalf("form does not start with its version: %.60s", data)
	}
	if len(g.Actions) != 2 || g.Actions[0].Name != "test::run" || g.Actions[1].Name != "test::Bump" {
		names := make([]string, 0, len(g.Actions))
		for _, a := range g.Actions {
			names = append(names, a.Name)
		}
		t.Fatalf("actions %v, want the subject then the action it performs", names)
	}
	if len(g.States) != 0 {
		t.Fatalf("an action exported %d state graphs", len(g.States))
	}
	run := g.Actions[0]
	if len(run.Attributes) != 3 || run.Attributes[0].Name != "total" || run.Attributes[0].Value == nil {
		t.Fatalf("attributes %+v, want total, doubled and battery with their defaults", run.Attributes)
	}
	if run.Initial == nil || len(run.Finals) != 1 {
		t.Fatalf("initial %v finals %v, want a start and one done", run.Initial, run.Finals)
	}
	byKind := kinds(run.Nodes)
	for _, k := range []string{"fork", "join", "decision", "start", "end"} {
		if byKind[k] == 0 {
			t.Errorf("no %s vertex among %v", k, byKind)
		}
	}
	guarded, elses := 0, 0
	for _, e := range run.Edges {
		if e.Guard != nil {
			guarded++
			if e.Guard.Text != "battery < 20" {
				t.Errorf("guard %q on edge %d->%d, want the decision's", e.Guard.Text, e.Source, e.Target)
			}
		}
		if e.Else {
			elses++
		}
	}
	if guarded != 1 || elses != 1 {
		t.Errorf("%d guarded and %d else edges, want the decision's one of each", guarded, elses)
	}
	var iterate *NodeForm
	for i := range run.Nodes {
		if run.Nodes[i].Name == "iterate" {
			iterate = &run.Nodes[i]
		}
	}
	if iterate == nil || len(iterate.Body) != 1 || iterate.Body[0].Kind != "loop" || iterate.Body[0].Body == nil {
		t.Fatalf("iterate node %+v, want a body of one loop statement", iterate)
	}
	loop := iterate.Body[0].Body
	if loop.Graph == nil || len(loop.Statements) != 0 {
		t.Fatalf("loop body %+v, want the flow a body performing an action states", loop)
	}
	performs, assigns := 0, 0
	for _, n := range loop.Graph.Nodes {
		if len(n.Performs) == 1 && n.Performs[0] == "test::Bump" {
			performs++
		}
		if n.Kind == "assignment" {
			assigns++
		}
	}
	if performs != 1 || assigns != 1 {
		t.Fatalf("loop flow %+v, want a perform of test::Bump and an assignment", loop.Graph.Nodes)
	}
	bump := g.Actions[1]
	if len(bump.Nodes) == 0 {
		t.Fatalf("Bump exported no vertices")
	}
	var params []string
	for _, p := range bump.Parameters {
		params = append(params, p.Direction+" "+p.Name+" : "+strings.Join(p.Types, ","))
	}
	want := []string{"in i : ScalarValues::Integer", "out doubled : ScalarValues::Integer"}
	if !reflect.DeepEqual(params, want) {
		t.Errorf("Bump's parameters %v, want %v", params, want)
	}
	if !strings.Contains(string(data), `"parameters":[{"name":"i","direction":"in"`) {
		t.Errorf("parameters are not written in the form: %s", data)
	}
	assertFootprints(t, run)
}

// assertFootprints pins that the flow's moves carry the lowering's footprint: the
// vertex writing `total` says so, and the vertices before the join converge on it.
func assertFootprints(t *testing.T, run *ActionForm) {
	t.Helper()
	join := -1
	for _, n := range run.Nodes {
		if n.Kind == "join" {
			join = n.ID
		}
	}
	writes := func(name string) *NodeForm {
		for i := range run.Nodes {
			if run.Nodes[i].Name == name {
				return &run.Nodes[i]
			}
		}
		t.Fatalf("no vertex named %s", name)
		return nil
	}
	left := writes("left")
	if left.Footprint == nil {
		t.Fatalf("left carries no footprint")
	}
	if len(left.Footprint.Writes) != 1 || left.Footprint.Writes[0].Name != "total" || left.Footprint.Writes[0].Symbol != "test::run::total" {
		t.Errorf("left writes %+v, want total resolved to test::run::total", left.Footprint.Writes)
	}
	converging := 0
	for _, n := range run.Nodes {
		if n.Footprint == nil {
			continue
		}
		for _, c := range n.Footprint.Control {
			if c == join {
				converging++
			}
		}
	}
	if converging == 0 {
		t.Errorf("no vertex converges on the join %d", join)
	}
}

func TestGraphsStateCarriesTheLoweredGraph(t *testing.T) {
	model, idx := graphsModel(t)
	g, data := exportGraphs(t, model, idx, "test::Machine")
	if len(g.States) != 1 || g.States[0].Name != "test::Machine" || len(g.Actions) != 0 {
		t.Fatalf("exported %d state and %d action graphs, want the machine alone", len(g.States), len(g.Actions))
	}
	m := g.States[0]
	if m.Machine == nil || len(m.Attributes) != 1 || m.Attributes[0].Name != "log" {
		t.Fatalf("machine %v attributes %+v", m.Machine, m.Attributes)
	}
	var outer *StateVertexForm
	hidden := 0
	for i := range m.Vertices {
		v := &m.Vertices[i]
		if v.Name == "Outer" {
			outer = v
		}
		if v.Hidden {
			hidden++
			if v.HiddenRegion == nil {
				t.Errorf("hidden vertex %d names no region", v.ID)
			}
		}
	}
	if outer == nil {
		t.Fatal("no Outer vertex")
	}
	if len(outer.Regions) != 2 {
		t.Errorf("Outer has %d regions, want its two orthogonal ones", len(outer.Regions))
	}
	if hidden != 2 {
		t.Errorf("%d hidden region states, want one per region of Outer", hidden)
	}
	if len(outer.Entry) != 1 || len(outer.Do) != 1 || len(outer.Exit) != 1 {
		t.Errorf("Outer entry %d do %d exit %d behaviors, want one each", len(outer.Entry), len(outer.Do), len(outer.Exit))
	}
	if len(outer.Do) == 1 && (outer.Do[0].Name != "watch" || len(outer.Do[0].Body) != 1) {
		t.Errorf("do behavior %+v, want watch with one statement", outer.Do[0])
	}
	for _, r := range m.Regions {
		if r.Owner == nil || *r.Owner != outer.ID {
			t.Errorf("region %d owned by %v, want Outer", r.ID, r.Owner)
		}
		if r.State == nil || r.Initial == nil {
			t.Errorf("region %d lacks its state %v or initial %v", r.ID, r.State, r.Initial)
		}
	}
	var setSpeed *TransitionForm
	triggered := 0
	for i := range m.Transitions {
		tr := &m.Transitions[i]
		if tr.Trigger != nil {
			triggered++
		}
		if tr.Source == outer.ID {
			setSpeed = tr
		}
	}
	if triggered != 2 {
		t.Errorf("%d triggered transitions, want wake and setSpeed", triggered)
	}
	if setSpeed == nil || setSpeed.Trigger == nil || setSpeed.Guard == nil || len(setSpeed.Effect) != 1 {
		t.Fatalf("Outer's transition %+v, want trigger, guard and effect", setSpeed)
	}
	if setSpeed.Trigger.Kind != "call" || setSpeed.Trigger.Operation != "setSpeed" || len(setSpeed.Trigger.Parameters) != 1 {
		t.Errorf("trigger %+v, want the call setSpeed(value)", setSpeed.Trigger)
	}
	if setSpeed.Guard.Text != "value > 0" || setSpeed.BodyScope == "" || setSpeed.BodyScope == setSpeed.Scope {
		t.Errorf("guard %+v in body scope %q, want `value > 0` resolving in the trigger's scope", setSpeed.Guard, setSpeed.BodyScope)
	}
	if len(setSpeed.Effect[0].Body) != 1 || setSpeed.Effect[0].Body[0].Kind != "assign" {
		t.Errorf("effect %+v, want one assignment", setSpeed.Effect[0])
	}
	if len(m.EntryTransitions) < 3 {
		t.Errorf("%d entry transitions, want the machine's and each region's", len(m.EntryTransitions))
	}
	if !strings.Contains(string(data), `"kind":"call"`) {
		t.Errorf("call trigger missing from the JSON form")
	}
}

func TestGraphsRefusesSubjectsWithoutAGraph(t *testing.T) {
	model, idx := graphsModel(t)
	sym := lookupOne(t, idx, "test")
	for name, want := range map[string]string{
		"test":               "no lowered graph for the package test: the subject must be an action or a state machine",
		"test::Machine::log": "no lowered graph for the attribute usage test::Machine::log: the subject must be an action or a state machine",
	} {
		_, err := GraphsOf(model, lookupOne(t, idx, name))
		if !errors.Is(err, ErrGraphsSubject) || err.Error() != want {
			t.Errorf("GraphsOf(%s) = %v, want ErrGraphsSubject reading %q", name, err, want)
		}
	}
	if _, err := GraphsOf(nil, sym); !errors.Is(err, ErrGraphsSubject) {
		t.Fatalf("GraphsOf(nil model) = %v, want ErrGraphsSubject", err)
	}
}

// A state without a body, a transition and a control node are parts of their
// owner's graph: the refusal says so in words, not by the type of the syntax node.
func TestGraphsRefusesAVertexOrAnEdgeOfAMachine(t *testing.T) {
	model, idx := graphsModelOf(t, `package P {
		state def Machine { state off; state on; transition off_on first off then on; }
		action def Split { fork f; action a; action b; succession guarded first a if true then b; }
	}`)
	for name, want := range map[string]string{
		"P::Machine::off":    "no lowered graph for the state usage P::Machine::off: a state declared without a body is a vertex of its owner's graph",
		"P::Machine::off_on": "no lowered graph for the action usage P::Machine::off_on: a transition is an edge of its owner's graph",
		"P::Split::f":        "no lowered graph for the action usage P::Split::f: a control node is a node of its owner's graph",
		"P::Split::guarded":  "no lowered graph for the action usage P::Split::guarded: a transition is an edge of its owner's graph",
	} {
		_, err := GraphsOf(model, lookupOne(t, idx, name))
		if !errors.Is(err, ErrGraphsSubject) || err.Error() != want {
			t.Errorf("GraphsOf(%s) = %v, want ErrGraphsSubject reading %q", name, err, want)
		}
	}
}

// The form is the same bytes on every export: repeated over one model, over
// fresh models of the same text, and from eight goroutines at once.
// A weighted succession's probability expression is exported with its edge, so an
// engine reading the graph sees the weights the run draws by.
func TestGraphsActionCarriesTheEdgeProbabilities(t *testing.T) {
	model, idx := graphsModel(t)
	g, _ := exportGraphs(t, model, idx, "test::pick")
	if len(g.Actions) != 1 || g.Actions[0].Error != "" {
		t.Fatalf("actions %+v, want the subject lowered", g.Actions)
	}
	pick := g.Actions[0]
	weights := map[string]string{}
	for _, e := range pick.Edges {
		if e.Probability == nil {
			continue
		}
		if e.Guard != nil || e.Else {
			t.Errorf("edge %d->%d is weighted and guarded: %+v", e.Source, e.Target, e)
		}
		weights[pick.Nodes[e.Target].Name] = e.Probability.Text
	}
	if want := map[string]string{"fast": "0.7", "slow": "0.3"}; !maps.Equal(weights, want) {
		t.Errorf("weighted edges %v, want %v", weights, want)
	}
	run, _ := exportGraphs(t, model, idx, "test::run")
	for _, a := range run.Actions {
		for _, e := range a.Edges {
			if e.Probability != nil {
				t.Errorf("%s: edge %d->%d carries a probability the model does not state", a.Name, e.Source, e.Target)
			}
		}
	}
}

// A flow is exported with its kind, so an engine tells a streaming `flow` from a
// `succession flow` the way the runtime does.
func TestGraphsActionCarriesTheFlowKind(t *testing.T) {
	model, idx := graphsModelOf(t, `package test {
	private import ScalarValues::*;

	action stream {
		action producer { out value : Integer; assign value := 1; }
		action consumer { in value : Integer[1]; }
		action last { in value : Integer[1]; }

		succession first start then producer;
		succession first producer then consumer;
		succession first consumer then last;
		succession first last then done;

		flow producer.value to consumer.value;
		succession flow producer.value to last.value;
	}
}
`)
	g, data := exportGraphs(t, model, idx, "test::stream")
	if len(g.Actions) != 1 || g.Actions[0].Error != "" {
		t.Fatalf("actions %+v, want the subject lowered", g.Actions)
	}
	stream := g.Actions[0]
	kinds := map[string]string{}
	for _, f := range stream.Flows {
		kinds[stream.Nodes[f.Target].Name] = f.Kind
	}
	if want := map[string]string{"consumer": "streaming", "last": "succession"}; !maps.Equal(kinds, want) {
		t.Errorf("flow kinds %v, want %v", kinds, want)
	}
	for _, want := range []string{`"kind":"streaming"`, `"kind":"succession"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("marshalled form lacks %s", want)
		}
	}
}

// A guarded succession leading to a succession flow keeps its own span on the edge
// it orders and on the flow it gates, so a consumer finds both declarations.
func TestGraphsActionCarriesTheGateOfAFlow(t *testing.T) {
	const fixture = `package test {
	private import ScalarValues::*;

	action pass {
		attribute go : Boolean = true;
		action producer { out value : Integer; assign value := 1; }
		action consumer { in value : Integer; }

		succession first start then producer;
		first producer if go then f;
		succession flow f of Integer from producer.value to consumer.value;
	}
}
`
	model, idx := graphsModelOf(t, fixture)
	g, data := exportGraphs(t, model, idx, "test::pass")
	if len(g.Actions) != 1 || g.Actions[0].Error != "" {
		t.Fatalf("actions %+v, want the subject lowered", g.Actions)
	}
	pass := g.Actions[0]
	text := func(s *SpanForm) string {
		if s == nil {
			return ""
		}
		return fixture[s.Offset : s.Offset+s.Len]
	}
	const gate = "first producer if go then f;"
	var edges int
	for _, e := range pass.Edges {
		if pass.Nodes[e.Target].Name != "consumer" {
			continue
		}
		edges++
		if got := text(e.Gate); got != gate || e.Guard == nil || e.Guard.Text != "go" {
			t.Errorf("edge into consumer: gate %q guard %+v, want gate %q guarded by go", got, e.Guard, gate)
		}
	}
	if edges != 1 || len(pass.Flows) != 1 {
		t.Fatalf("%d edges into consumer, %d flows; want one of each", edges, len(pass.Flows))
	}
	if got := text(pass.Flows[0].Gate); got != gate {
		t.Errorf("flow gate %q, want %q", got, gate)
	}
	if !strings.Contains(string(data), `"gate":`) {
		t.Error("marshalled form lacks the gate")
	}
}

// A body index has no expression-printer spelling, so its selector must come
// from where it was written, with an ordered selector even when text is absent.
func TestGraphsCarriesIndexedEndSelectorsFromTheirExtent(t *testing.T) {
	const fixture = `package indexed {
	action run {
		port p[3];
		port q;
		connect p#(1) to q;
		connect p#({ in i : Integer; i + 1 }) to q;
		connect q to p#(2);
	}
}
`
	model, idx := graphsModelOf(t, fixture)
	g, data := exportGraphs(t, model, idx, "indexed::run")
	if len(g.Actions) != 1 || g.Actions[0].Error != "" {
		t.Fatalf("actions %+v, want the subject lowered", g.Actions)
	}
	conns := g.Actions[0].Connections
	if len(conns) != 3 {
		t.Fatalf("connections = %+v, want the three of the body", conns)
	}
	const bodyIndex = "{ in i : Integer; i + 1 }"
	if got := conns[1].Ends[0]; got != "p#("+bodyIndex+")" {
		t.Errorf("body-indexed end = %q, want %q", got, "p#("+bodyIndex+")")
	}
	if got := conns[0].Ends[0]; got != "p#(1)" {
		t.Errorf("constant-indexed end = %q, want p#(1)", got)
	}
	if got := conns[2].Ends[1]; got != "p#(2)" {
		t.Errorf("second indexed end = %q, want p#(2)", got)
	}
	for _, tc := range []struct {
		conn int
		at   int
		text string
	}{
		{0, 0, "1"},
		{1, 0, bodyIndex},
		{2, 1, "2"},
	} {
		conn := conns[tc.conn]
		if len(conn.Selectors) != 1 {
			t.Fatalf("connection %d selectors = %+v, want one", tc.conn, conn.Selectors)
		}
		sel := conn.Selectors[0]
		if sel.At != tc.at || sel.Index == nil || sel.Index.Text != tc.text {
			t.Errorf("connection %d selector = %+v, want at %d text %q", tc.conn, sel, tc.at, tc.text)
		}
		if sel.Index.Span.Document != "graphs.sysml" || sel.Index.Span.Len <= 0 {
			t.Errorf("connection %d selector span = %+v, want a written extent", tc.conn, sel.Index.Span)
		}
		if start := sel.Index.Span.Offset; fixture[start:start+sel.Index.Span.Len] != tc.text {
			t.Errorf("connection %d selector span names %q, want %q", tc.conn, fixture[start:start+sel.Index.Span.Len], tc.text)
		}
	}
	if !strings.Contains(string(data), `"selectors":[{"at":0,"index":{"text":"{ in i : Integer; i + 1 }"`) {
		t.Errorf("the marshalled form should state the body selector\n%s", data)
	}
}

// A quoted port named `a.b` and the nested port `a.b` flatten to the same
// routing path, but the analysis form keeps their written names apart.
func TestGraphsDistinguishesAQuotedIndexedPortFromANestedOne(t *testing.T) {
	const fixture = `package quoted {
	action run {
		part a { port b[3]; }
		port 'a.b'[3];
		port q;
		connect 'a.b'#(1) to q;
		connect a.b#(1) to q;
	}
}
`
	model, idx := graphsModelOf(t, fixture)
	g, _ := exportGraphs(t, model, idx, "quoted::run")
	conns := g.Actions[0].Connections
	if len(conns) != 2 {
		t.Fatalf("connections = %+v, want the quoted and the nested one", conns)
	}
	if got := conns[0].Ends[0]; got != "'a.b'#(1)" {
		t.Errorf("quoted end = %q, want 'a.b'#(1)", got)
	}
	if got := conns[1].Ends[0]; got != "a.b#(1)" {
		t.Errorf("nested end = %q, want a.b#(1)", got)
	}
	if conns[0].Ends[0] == conns[1].Ends[0] {
		t.Error("the two ends should not share a written spelling")
	}
	for _, conn := range conns {
		if len(conn.Selectors) != 1 || conn.Selectors[0].At != 0 || conn.Selectors[0].Index == nil || conn.Selectors[0].Index.Text != "1" {
			t.Errorf("connection selectors = %+v, want the index at end 0", conn.Selectors)
		}
	}
}

func TestGraphsAreByteStable(t *testing.T) {
	model, idx := graphsModel(t)
	for _, subject := range []string{"test::run", "test::Machine"} {
		_, want := exportGraphs(t, model, idx, subject)
		for i := 0; i < 3; i++ {
			if _, got := exportGraphs(t, model, idx, subject); !bytes.Equal(got, want) {
				t.Fatalf("%s: export %d differs from the first", subject, i)
			}
		}
		fresh, freshIdx := graphsModel(t)
		if _, got := exportGraphs(t, fresh, freshIdx, subject); !bytes.Equal(got, want) {
			t.Fatalf("%s: a fresh model exports different bytes", subject)
		}
		const jobs = 8
		results := make([][]byte, jobs)
		errs := make([]error, jobs)
		var wg sync.WaitGroup
		for j := 0; j < jobs; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				resolver := resolve.New(idx)
				m := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
				for _, sf := range model.Sources() {
					m.RegisterSource(sf)
				}
				g, err := GraphsOf(m, lookupOne(t, idx, subject))
				if err != nil {
					errs[j] = err
					return
				}
				results[j], errs[j] = MarshalGraphs(g)
			}(j)
		}
		wg.Wait()
		for j := 0; j < jobs; j++ {
			if errs[j] != nil {
				t.Fatalf("%s: job %d: %v", subject, j, errs[j])
			}
			if !bytes.Equal(results[j], want) {
				t.Fatalf("%s: job %d exported different bytes", subject, j)
			}
		}
	}
}

func TestSourcesOfListsEveryDocumentInOrder(t *testing.T) {
	model, _ := graphsModel(t)
	model.RegisterSource(source.New("aux.sysml", []byte("package aux;")))
	s, err := SourcesOf(model)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Documents) != 2 || s.Documents[0].Path != "aux.sysml" || s.Documents[1].Path != "graphs.sysml" {
		t.Fatalf("documents %+v, want aux.sysml then graphs.sysml", s.Documents)
	}
	if s.Documents[1].Text != graphsFixture || s.Documents[0].Text != "package aux;" {
		t.Error("document text is not the registered text")
	}
	if !strings.HasPrefix(s.Library, "sha256:") || s.Library != libs.Version() {
		t.Errorf("library %q, want the sha256 set digest libs.Version reports", s.Library)
	}
	again, err := SourcesOf(model)
	if err != nil || again.Library != s.Library {
		t.Errorf("a second export reports library %q (%v), want %q", again.Library, err, s.Library)
	}
	if _, err := SourcesOf(nil); !errors.Is(err, ErrNoSources) {
		t.Errorf("SourcesOf(nil) = %v, want ErrNoSources", err)
	}
	if _, err := SourcesOf(runtime.NewModel(passes.NewTypedModel(nil), nil)); !errors.Is(err, ErrNoSources) {
		t.Errorf("SourcesOf(empty) = %v, want ErrNoSources", err)
	}
}

func TestRefuseRDFFormIsTyped(t *testing.T) {
	err := RefuseRDFForm()
	var typed *FormUnsupportedError
	if !errors.Is(err, ErrFormUnsupported) || !errors.As(err, &typed) || typed.Form != "rdf" {
		t.Fatalf("RefuseRDFForm() = %v, want a FormUnsupportedError for rdf", err)
	}
	if !strings.Contains(err.Error(), "graphs:1") {
		t.Errorf("refusal %q does not name the forms that are served", err)
	}
}
