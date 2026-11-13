package lower

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const identityActionSrc = `
	action brew {
		first start;
		action grind;
		action pour;
		fork f;
		join j;
		if x > 1 { action a1; }
		if x > 2 { action a2; }
		assign y := 1;
		assign y := 2;
		first start then grind;
		first grind then pour;
		first pour then done;
	}
`

// identityActionGraph lowers the first action usage of src within its document,
// so that origins carry the document name and qualified paths.
func identityActionGraph(t *testing.T, src string) *ActionGraph {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	var usage *ast.Usage
	for _, member := range root.Members {
		if m, ok := member.(*ast.Membership); ok {
			if u, ok := m.Member.(*ast.Usage); ok && u.Kind == ast.UsageAction {
				usage = u
				break
			}
		}
	}
	if usage == nil {
		t.Fatal("no action usage found")
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := idx.DocumentRoot("test.sysml").ChildFor(usage)
	if scope == nil {
		t.Fatal("scope for action usage missing")
	}
	graph, err := ToActionGraphWith(usage, scope, resolve.New(idx))
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	return graph
}

func TestAssignIDs_ActionDenseInDeclarationOrder(t *testing.T) {
	graph := identityActionGraph(t, identityActionSrc)
	if len(graph.Nodes) < 6 {
		t.Fatalf("fixture lowers %d nodes, want several", len(graph.Nodes))
	}
	if got := len(graph.Origins); got != len(graph.Nodes)+1 {
		t.Fatalf("len(Origins) = %d, want %d", got, len(graph.Nodes)+1)
	}
	if len(graph.Kinds) != len(graph.Origins) || len(graph.ID) != len(graph.Nodes) {
		t.Fatalf("Kinds %d, ID %d, Nodes %d", len(graph.Kinds), len(graph.ID), len(graph.Nodes))
	}
	for i, node := range graph.Nodes {
		id := graph.ID[node]
		if id != program.NodeID(i+1) {
			t.Errorf("node %d has id %d, want %d", i, id, i+1)
		}
		if graph.Provenance.Node(id) != node {
			t.Errorf("Provenance.Node(%d) is not the node", id)
		}
		if graph.Kinds[id] == program.NodeNone {
			t.Errorf("node %d has no kind", id)
		}
		if origin := graph.Origins[id]; origin.Doc != "test.sysml" || origin.Span != node.Span() {
			t.Errorf("node %d origin = %#v", id, origin)
		}
	}
	if graph.Provenance.Node(0) != nil || graph.Provenance.Node(program.NodeID(len(graph.Nodes)+1)) != nil {
		t.Error("ids outside the table must map to nil")
	}
}

func TestAssignIDs_ActionOriginsNameWhatMessagesName(t *testing.T) {
	graph := identityActionGraph(t, identityActionSrc)
	byName := make(map[string]program.NodeID)
	for _, node := range graph.Nodes {
		byName[graph.Origins[graph.ID[node]].Name] = graph.ID[node]
	}
	grind := graph.Origins[byName["grind"]]
	if grind.String() != "node grind" || grind.Kind != program.NodeExecution.Word() || grind.Path != "brew::grind" || grind.Doc != "test.sysml" {
		t.Errorf("grind origin = %#v", grind)
	}
	if graph.Kinds[1] != program.NodeInitial || graph.Kinds[byName["f"]] != program.NodeFork || graph.Kinds[byName["grind"]] != program.NodeExecution {
		t.Errorf("kinds: initial %v, fork %v, grind %v", graph.Kinds[1], graph.Kinds[byName["f"]], graph.Kinds[byName["grind"]])
	}
	done := graph.Origins[byName["done"]]
	if done.String() != "node done" || done.Kind != program.NodeFinal.Word() || graph.Kinds[byName["done"]] != program.NodeFinal {
		t.Errorf("done origin = %#v", done)
	}
	for id := 1; id < len(graph.Origins); id++ {
		if origin := graph.Origins[id]; origin.Name == "" && origin.String() != "the "+origin.Kind {
			t.Errorf("unnamed node %d renders %q", id, origin.String())
		}
	}
}

func TestAssignIDs_ActionPathUniquePerGraph(t *testing.T) {
	graph := identityActionGraph(t, identityActionSrc)
	paths := make(map[string]program.NodeID)
	unnamedKinds := make(map[string]int)
	for id := 1; id < len(graph.Origins); id++ {
		origin := graph.Origins[id]
		if origin.Path == "" {
			t.Errorf("node %d has an empty path", id)
		}
		if prev, dup := paths[origin.Path]; dup {
			t.Errorf("nodes %d and %d share path %q", prev, id, origin.Path)
		}
		paths[origin.Path] = program.NodeID(id)
		if origin.Name == "" {
			unnamedKinds[origin.Kind]++
		}
	}
	// The fixture must make uniqueness do work: two unnamed nodes of one kind.
	exercised := false
	for _, n := range unnamedKinds {
		exercised = exercised || n > 1
	}
	if !exercised {
		t.Fatalf("fixture has no two unnamed nodes of one kind: %v", unnamedKinds)
	}
}

func TestAssignIDs_ActionDeterministicAcrossLowerings(t *testing.T) {
	first := identityActionGraph(t, identityActionSrc)
	second := identityActionGraph(t, identityActionSrc)
	if !reflect.DeepEqual(first.Origins, second.Origins) {
		t.Errorf("origins differ:\n%+v\n%+v", first.Origins, second.Origins)
	}
	if !reflect.DeepEqual(first.Kinds, second.Kinds) {
		t.Errorf("kinds differ: %v vs %v", first.Kinds, second.Kinds)
	}
	for i := range first.Nodes {
		if first.ID[first.Nodes[i]] != second.ID[second.Nodes[i]] {
			t.Errorf("node %d: ids %d and %d", i, first.ID[first.Nodes[i]], second.ID[second.Nodes[i]])
		}
	}
}

func identityStateGraph(t *testing.T) *StateGraph {
	t.Helper()
	graph, err := ToStateGraph(stateDefinitionIn(t, forkMachine(
		`state a; state b;`,
		`state c;`,
		`transition first split then a;
		 transition first split then c;
		 transition maintain first a then b;`,
	)), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	return graph
}

func TestAssignIDs_StateTablesDenseInDeclarationOrder(t *testing.T) {
	graph := identityStateGraph(t)
	if len(graph.States) < 4 || len(graph.Pseudostates) < 1 || len(graph.RegionID) < 2 || len(graph.TransID) < 3 {
		t.Fatalf("fixture lowers %d states, %d pseudostates, %d regions, %d transitions",
			len(graph.States), len(graph.Pseudostates), len(graph.RegionID), len(graph.TransID))
	}
	if len(graph.Origins) != len(graph.ID)+1 || len(graph.PseudoOrigins) != len(graph.Pseudostates)+1 ||
		len(graph.RegionOrigins) != len(graph.RegionID)+1 || len(graph.TransOrigins) != len(graph.TransID)+1 {
		t.Fatal("every origin table is one longer than its records")
	}
	for i, state := range graph.States {
		id := graph.ID[state]
		if id != program.StateID(i+1) || graph.Provenance.State(id) != state {
			t.Errorf("state %s: id %d at %d", state.Name, id, i)
		}
		if origin := graph.Origins[id]; origin.Name != state.Name || origin.Kind != "state" || origin.Span != state.Span() {
			t.Errorf("state %s origin = %+v", state.Name, origin)
		}
	}
	for i, ps := range graph.Pseudostates {
		id := graph.PseudoID[ps]
		if id != program.PseudoID(i+1) || graph.Provenance.Pseudostate(id) != ps {
			t.Errorf("pseudostate %s: id %d at %d", ps.Name, id, i)
		}
	}
	for region, id := range graph.RegionID {
		if graph.Provenance.Region(id) != region || graph.RegionOrigins[id].Kind != "region" {
			t.Errorf("region %s: id %d", region.Name, id)
		}
	}
	for i, region := range graph.TopRegions {
		if graph.RegionID[region] != program.RegionID(i+1) {
			t.Errorf("top region %d has id %d", i, graph.RegionID[region])
		}
	}
	seen := 0
	for _, state := range graph.States {
		for _, tr := range graph.Transitions[state] {
			seen++
			id := graph.TransID[tr]
			if id != program.TransID(seen) || graph.Provenance.Transition(id) != tr {
				t.Errorf("transition %d from %s has id %d", seen, state.Name, id)
			}
		}
	}
	if origin := graph.TransOrigins[graph.TransID[graph.Transitions[stateNamed(graph, "a")][0]]]; origin.Name != "maintain" || origin.Kind != "transition" {
		t.Errorf("named transition origin = %+v", origin)
	}
}

func TestAssignIDs_StatePathUniquePerGraph(t *testing.T) {
	graph := identityStateGraph(t)
	paths := make(map[string]bool)
	check := func(table string, origins []program.Origin) {
		for id := 1; id < len(origins); id++ {
			path := origins[id].Path
			if path == "" || paths[path] {
				t.Errorf("%s %d: path %q empty or taken", table, id, path)
			}
			paths[path] = true
		}
	}
	check("state", graph.Origins)
	check("region", graph.RegionOrigins)
	check("pseudostate", graph.PseudoOrigins)
	check("transition", graph.TransOrigins)
}

func TestAssignIDs_StateDeterministicAcrossLowerings(t *testing.T) {
	first, second := identityStateGraph(t), identityStateGraph(t)
	for name, pair := range map[string][2][]program.Origin{
		"states":       {first.Origins, second.Origins},
		"regions":      {first.RegionOrigins, second.RegionOrigins},
		"pseudostates": {first.PseudoOrigins, second.PseudoOrigins},
		"transitions":  {first.TransOrigins, second.TransOrigins},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s differ:\n%+v\n%+v", name, pair[0], pair[1])
		}
	}
}

// The states a parallel body's usages stand for are graph-only (States leaves
// them out) but head parent chains and region tables, so they are numbered too.
func TestAssignIDs_StateHiddenRegionOwnersNumbered(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, `
		package test {
			state def Machine {
				entry; then outer;
				state outer parallel {
					state a {
						entry; then a1;
						state a1;
					}
					state b {
						entry; then b1;
						state b1;
					}
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if len(graph.Origins) != len(graph.ID)+1 || len(graph.ID) != len(graph.States)+2 {
		t.Fatalf("%d ids for %d states and two hidden region owners, %d origins", len(graph.ID), len(graph.States), len(graph.Origins))
	}
	for i, state := range graph.States {
		if graph.ID[state] != program.StateID(i+1) {
			t.Errorf("state %s: id %d at %d; want States numbered first", state.Name, graph.ID[state], i)
		}
	}
	for _, name := range []string{"a1", "b1"} {
		parent := graph.ParentState[stateNamed(graph, name)]
		if parent == nil || !graph.HiddenStates[parent] {
			t.Fatalf("%s parent = %v, want hidden region owner", name, parent)
		}
		id := graph.ID[parent]
		if id == 0 || graph.Provenance.State(id) != parent || graph.Origins[id].Name != parent.Name {
			t.Errorf("hidden %s: id %d, provenance %v, origin %+v", parent.Name, id, graph.Provenance.State(id), graph.Origins[id])
		}
	}
	for region, owner := range graph.RegionState {
		if graph.ID[owner] == 0 {
			t.Errorf("region %s owner %s has no id", region.Name, owner.Name)
		}
	}
}

func TestAssignIDs_CaseFlowCoversNodesAddedAfterLowering(t *testing.T) {
	body := caseBodyOf(t, `
		analysis def Among {
			subject s : S;
			if s.ready {
				assign s.x := 1;
				out r : Integer = s.x;
			}
			perform action p : P;
			return r : Integer = s.x;
		}
	`)
	flow, ok := body[0].(Block)
	if !ok || flow.Graph == nil || len(flow.Graph.Nodes) != 2 {
		t.Fatalf("body[0] = %#v, want the case flow of two nodes", body[0])
	}
	graph := flow.Graph
	if len(graph.ID) != len(graph.Nodes) || len(graph.Origins) != len(graph.Nodes)+1 {
		t.Fatalf("%d ids and %d origins for %d nodes", len(graph.ID), len(graph.Origins), len(graph.Nodes))
	}
	for i, node := range graph.Nodes {
		if graph.ID[node] != program.NodeID(i+1) {
			t.Errorf("node %d has id %d", i, graph.ID[node])
		}
	}
	if graph.Kinds[1] != program.NodeIf || graph.Kinds[2] != program.NodeExecution {
		t.Errorf("kinds = %v, want the branch then the performed action", graph.Kinds[1:])
	}
}

func TestAssignIDs_RegionOriginsCarryDocumentAndOwner(t *testing.T) {
	src := forkMachine(`state a;`, `state c;`, `transition first split then a; transition first split then c;`)
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	machine := idx.LookupQualified("test::Machine")
	if len(machine) != 1 || machine[0].Scope == nil {
		t.Fatalf("test::Machine resolves to %d symbols", len(machine))
	}
	graph, err := ToStateGraph(machine[0].Decl, machine[0].Scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if len(graph.RegionOrigins) < 3 {
		t.Fatalf("%d region origins, want the two regions of work", len(graph.RegionOrigins)-1)
	}
	for id := 1; id < len(graph.RegionOrigins); id++ {
		origin := graph.RegionOrigins[id]
		if origin.Doc != "test.sysml" || !strings.HasPrefix(origin.Path, "test::Machine") {
			t.Errorf("region %d origin = %#v", id, origin)
		}
	}
}
