package export

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// graphsFixture holds every shape the lowered graphs carry: a flow with a fork,
// a join, a guarded decision, a nested body performing another action, and a
// machine with orthogonal regions, entry/do/exit behaviors, a call-triggered
// guarded transition with an effect and a deferred trigger.
const graphsFixture = `package test {
	private import ScalarValues::*;

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

	action def Bump {
		in i : Integer;
		out doubled : Integer;

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
	const path = "graphs.sysml"
	sf := source.New(path, []byte(graphsFixture))
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
	model := runtime.NewModel(semantics.NewModel(resolver), resolver)
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
	if _, err := GraphsOf(model, sym); !errors.Is(err, ErrGraphsSubject) {
		t.Fatalf("GraphsOf(package) = %v, want ErrGraphsSubject", err)
	}
	if _, err := GraphsOf(nil, sym); !errors.Is(err, ErrGraphsSubject) {
		t.Fatalf("GraphsOf(nil model) = %v, want ErrGraphsSubject", err)
	}
}

// The form is the same bytes on every export: repeated over one model, over
// fresh models of the same text, and from eight goroutines at once.
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
				m := runtime.NewModel(semantics.NewModel(resolver), resolver)
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
	if _, err := SourcesOf(runtime.NewModel(semantics.NewModel(nil), nil)); !errors.Is(err, ErrNoSources) {
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
