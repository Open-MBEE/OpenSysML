package lower

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// stateUsageIn returns the first state usage declared in the package in src.
func stateUsageIn(t *testing.T, src string) *ast.Usage {
	t.Helper()
	_, machine := parseStateUsage(t, src)
	return machine
}

// parseStateUsage parses src and returns its root and the first state usage in
// it, which a caller needing a scope tree indexes the very same root for.
func parseStateUsage(t *testing.T, src string) (*ast.RootNamespace, *ast.Usage) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var found *ast.Usage
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch n := member.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Usage:
				if n.Kind == ast.UsageState && found == nil {
					found = n
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no state usage found")
	}
	return root, found
}

// A pseudostate declared inside a composite state is part of the graph and knows
// which state owns it, which is what a history pseudostate restores from.
func TestToStateGraph_NestedPseudostateOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state outer {
					state a;
					state b;
					choice pick;
				}
				succession first start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := pseudostateNamed(graph, "pick")
	if pick == nil {
		t.Fatal("pseudostate declared inside outer was not collected")
	}
	owner := graph.PseudostateOwner[pick]
	if owner == nil || owner.Name != "outer" {
		t.Fatalf("PseudostateOwner[pick] = %v, want outer", owner)
	}
}

// A pseudostate declared directly in the machine has no owning composite state.
func TestToStateGraph_TopLevelPseudostateHasNoOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				choice pick;
				state a;
				succession first start then pick;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := pseudostateNamed(graph, "pick")
	if pick == nil {
		t.Fatal("choice pseudostate not collected")
	}
	if owner, ok := graph.PseudostateOwner[pick]; ok {
		t.Errorf("PseudostateOwner[pick] = %v, want no owner", owner)
	}
}

func TestToStateGraph_ParallelMatchesExplicitRegions(t *testing.T) {
	standard := stateDefinitionIn(t, `
		package test {
			state def Machine parallel {
				state left {
					entry; then lstart;
					state lstart;
					succession first lstart then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					succession first rstart then done;
				}
			}
		}
	`)
	explicit := stateDefinitionIn(t, `
		package test {
			state def Machine parallel {
				state left {
					entry; then lstart;
					state lstart;
					succession first lstart then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					succession first rstart then done;
				}
			}
		}
	`)

	parallelGraph, err := ToStateGraph(standard, nil)
	if err != nil {
		t.Fatalf("parallel ToStateGraph: %v", err)
	}
	regionGraph, err := ToStateGraph(explicit, nil)
	if err != nil {
		t.Fatalf("region ToStateGraph: %v", err)
	}
	if parallelGraph.Initial != nil {
		t.Fatalf("parallel graph initial = %v, want nil", parallelGraph.Initial.Name)
	}
	if len(parallelGraph.TopRegions) != 2 || len(regionGraph.TopRegions) != 2 {
		t.Fatalf("top regions: parallel=%d explicit=%d, want 2 each",
			len(parallelGraph.TopRegions), len(regionGraph.TopRegions))
	}
	for i, region := range parallelGraph.TopRegions {
		want := regionGraph.TopRegions[i]
		if region.Name != want.Name {
			t.Errorf("region %d name = %q, want %q", i, region.Name, want.Name)
		}
		if got, wantState := parallelGraph.RegionInitials[region].Name, regionGraph.RegionInitials[want].Name; got != wantState {
			t.Errorf("region %q initial = %q, want %q", region.Name, got, wantState)
		}
		var gotStates, wantStates []string
		seenGot := make(map[string]bool)
		for state, owner := range parallelGraph.RegionOf {
			if owner == region && !seenGot[state.Name] {
				gotStates = append(gotStates, state.Name)
				seenGot[state.Name] = true
			}
		}
		seenWant := make(map[string]bool)
		var gotFinal, wantFinal []string
		for state, owner := range regionGraph.RegionOf {
			if owner == want && !seenWant[state.Name] {
				wantStates = append(wantStates, state.Name)
				seenWant[state.Name] = true
			}
		}
		for state, owner := range parallelGraph.RegionOf {
			if owner == region && parallelGraph.Completes(state) {
				gotFinal = append(gotFinal, state.Name)
			}
		}
		for state, owner := range regionGraph.RegionOf {
			if owner == want && regionGraph.Completes(state) {
				wantFinal = append(wantFinal, state.Name)
			}
		}
		sort.Strings(gotStates)
		sort.Strings(wantStates)
		if !reflect.DeepEqual(gotStates, wantStates) {
			t.Errorf("region %q states = %v, want %v", region.Name, gotStates, wantStates)
		}
		sort.Strings(gotFinal)
		sort.Strings(wantFinal)
		if !reflect.DeepEqual(gotFinal, wantFinal) {
			t.Errorf("region %q final states = %v, want %v", region.Name, gotFinal, wantFinal)
		}
	}
	if got, want := transitionShape(parallelGraph), transitionShape(regionGraph); !reflect.DeepEqual(got, want) {
		t.Errorf("transitions = %v, want %v", got, want)
	}
}

func TestToStateGraph_ParallelStateUsage(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine parallel {
				state left {
					entry; then idle;
					state idle;
				}
				state right {
					entry; then ready;
					state ready;
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("parallel state usage: %v", err)
	}
	if graph.Initial != nil {
		t.Fatalf("parallel state usage initial = %v, want nil", graph.Initial.Name)
	}
	if len(graph.TopRegions) != 2 {
		t.Fatalf("parallel state usage regions = %d, want 2", len(graph.TopRegions))
	}
}

func TestToStateGraph_ParallelStateBehaviorsAndDeferredEvents(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine parallel {
				entry action begin { }
				do action work { }
				exit action finish { }
				state left {
					defer Ping;
					entry; then idle;
					state idle;
				}
				state right {
					entry; then ready;
					state ready;
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("parallel state behaviors: %v", err)
	}
	if graph.Machine == nil {
		t.Fatal("parallel machine root was not lowered")
	}
	if got := len(graph.Behaviors[graph.Machine].Entry); got != 1 {
		t.Fatalf("parallel machine entry behaviors = %d, want 1", got)
	}
	if got := len(graph.Behaviors[graph.Machine].Do); got != 1 {
		t.Fatalf("parallel machine do behaviors = %d, want 1", got)
	}
	if got := len(graph.Behaviors[graph.Machine].Exit); got != 1 {
		t.Fatalf("parallel machine exit behaviors = %d, want 1", got)
	}
	idle := stateNamed(graph, "idle")
	if idle == nil {
		t.Fatal("left region's idle state was not lowered")
	}
	left := graph.ParentState[idle]
	if left == nil || left.Name != "left" {
		t.Fatalf("idle parent = %v, want graph-only left region state", left)
	}
	if got := len(graph.Deferred[left]); got != 1 {
		t.Fatalf("left deferred triggers = %d, want 1", got)
	}
}

func TestToStateGraph_ParallelBodyNonRegionMembers(t *testing.T) {
	t.Run("port is not a region", func(t *testing.T) {
		graph, err := ToStateGraph(stateUsageIn(t, `
			package test {
				state Machine parallel {
					port p;
					state left {
						entry; then idle;
						state idle;
					}
				}
			}
		`), nil)
		if err != nil {
			t.Fatalf("parallel state with port: %v", err)
		}
		if len(graph.TopRegions) != 1 || graph.TopRegions[0].Name != "left" {
			t.Fatalf("parallel regions = %v, want only left (not port)", graph.TopRegions)
		}
	})

	t.Run("metadata is not a region", func(t *testing.T) {
		graph, err := ToStateGraph(stateUsageIn(t, `
			package test {
				metadata def Reviewed;
				state Machine parallel {
					state left {
						entry; then idle;
						state idle;
					}
					metadata Reviewed about left;
				}
			}
		`), nil)
		if err != nil {
			t.Fatalf("parallel state with metadata: %v", err)
		}
		if len(graph.TopRegions) != 1 || graph.TopRegions[0].Name != "left" {
			t.Fatalf("parallel regions = %v, want only left (not metadata)", graph.TopRegions)
		}
	})

	t.Run("perform is rejected", func(t *testing.T) {
		_, err := ToStateGraph(stateUsageIn(t, `
			package test {
				state Machine parallel {
					perform action work;
				}
			}
		`), nil)
		if err == nil {
			t.Fatal("parallel state with perform succeeded")
		}
		if !strings.Contains(err.Error(), "parallel state body contains unsupported member") {
			t.Fatalf("error = %v, want unsupported parallel-body member", err)
		}
	})
}

// A parallel state owns what its regions branch through: the pseudostate, the
// edges leaving it, and the deferred events of the state that declares them.
func TestToStateGraph_ParallelStateOwnsItsPseudostatesAndEdges(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, `
		package test {
			state def Machine {
				entry; then start;
				state start;
				state working parallel {
					state left {
						entry; then lidle;
						state lidle;
						state lfast;
						defer done;
						transition first lidle then pick;
					}
					state right {
						entry; then ridle;
						state ridle;
					}
					choice pick;
					transition first pick then lfast;
				}
				succession first start then working;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	working := stateNamed(graph, "working")
	pick := pseudostateNamed(graph, "pick")
	if working == nil || pick == nil {
		t.Fatalf("working = %v, pick = %v, want both collected", working, pick)
	}
	if owner := graph.PseudostateOwner[pick]; owner != working {
		t.Fatalf("PseudostateOwner[pick] = %v, want working", owner)
	}
	lidle, lfast := stateNamed(graph, "lidle"), stateNamed(graph, "lfast")
	if !leadsTo(graph, lidle, pick) {
		t.Fatal("transition lidle -> pick missing")
	}
	if !leadsTo(graph, pick, lfast) {
		t.Fatal("transition pick -> lfast missing")
	}
	left := graph.CompositeStates[working][0]
	if got := graph.RegionOf[lidle]; got != left {
		t.Fatalf("RegionOf[lidle] = %v, want the left region", got)
	}
	var leftOwner *ast.StateNode
	for state, region := range graph.HiddenRegionOf {
		if region == left {
			leftOwner = state
		}
	}
	if leftOwner == nil {
		t.Fatal("no graph state stands for the left region")
	}
	if len(graph.Deferred[leftOwner]) == 0 {
		t.Fatal("defer declared by a region substate was dropped")
	}
}

func TestToStateGraph_ParallelWithoutInitialFailsClearly(t *testing.T) {
	_, err := ToStateGraph(stateDefinitionIn(t, `
		package test {
			state def Machine parallel {
				state left {
					state idle;
				}
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("parallel state without a region initial succeeded")
	}
	if !strings.Contains(err.Error(), "region left has no initial state") {
		t.Fatalf("error = %q, want missing region initial", err)
	}
	if !strings.Contains(err.Error(), "entry; then <state>;") {
		t.Fatalf("error = %q, want initial-state notation guidance", err)
	}
}

func TestToStateGraph_NestedParallelRegions(t *testing.T) {
	graph, err := ToStateGraph(stateDefinitionIn(t, `
		package test {
			state def Machine {
				entry; then start;
				state start;
				state outer parallel {
					state a {
						do action work { }
						entry; then a1;
						state a1;
					}
					state b {
						entry; then b1;
						state b1;
					}
				}
				succession first start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	outer := stateNamed(graph, "outer")
	if outer == nil {
		t.Fatal("outer state not collected")
	}
	regions := graph.CompositeStates[outer]
	if len(regions) != 2 {
		t.Fatalf("outer regions = %d, want 2", len(regions))
	}
	for _, region := range regions {
		if graph.RegionOwner[region] != outer {
			t.Errorf("region %q owner = %v, want outer", region.Name, graph.RegionOwner[region])
		}
		if graph.RegionInitials[region] == nil {
			t.Errorf("region %q has no initial", region.Name)
		}
	}
	for _, name := range []string{"a1", "b1"} {
		var matches []*ast.StateNode
		for _, state := range graph.States {
			if state.Name == name {
				matches = append(matches, state)
			}
		}
		if len(matches) != 1 {
			t.Fatalf("%s states = %d, want one", name, len(matches))
		}
		parent := graph.ParentState[matches[0]]
		if parent == nil || !graph.HiddenStates[parent] {
			t.Fatalf("%s parent = %v, want hidden region owner", name, parent)
		}
		if graph.RegionOf[matches[0]] == nil {
			t.Fatalf("%s has no owning region", name)
		}
	}
}

// stateDefinitionIn returns the first state definition in src.
func stateDefinitionIn(t *testing.T, src string) *ast.Definition {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	var found *ast.Definition
	var walk func([]ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			switch n := unwrapMembership(member).(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Definition:
				if n.Kind == ast.DefState && found == nil {
					found = n
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no state definition found")
	}
	return found
}

func transitionShape(graph *StateGraph) map[string][]string {
	shape := make(map[string][]string)
	for source, transitions := range graph.Transitions {
		for _, transition := range transitions {
			sourceState, sourceOK := source.(*ast.StateNode)
			targetState, targetOK := transition.Target.(*ast.StateNode)
			if !sourceOK || !targetOK {
				continue
			}
			shape[sourceState.Name] = append(shape[sourceState.Name], targetState.Name)
		}
	}
	for source := range shape {
		sort.Strings(shape[source])
	}
	return shape
}

// An endpoint naming no vertex leaves its edge out of the graph rather than
// failing the lowering: the name-resolution tier reports the name, and a machine
// missing one edge still runs: leniency about `TransitionUsage::source`/`::target
// : ActionUsage[1..1]` (stdlib `Systems Library/SysML.sysml`).
func TestToStateGraph_EndpointNamingNoVertexLeavesTheEdgeOut(t *testing.T) {
	machine := stateUsageIn(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy;
				succession first start then busy;
				transition first busy then nowhere;
			}
		}
	`)
	graph, err := ToStateGraph(machine, nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	for from, transitions := range graph.Transitions {
		for _, transition := range transitions {
			if transition.Target == nil {
				t.Errorf("transition from %v was lowered with no target", from)
			}
		}
	}
	var busy *ast.StateNode
	for _, state := range graph.States {
		if state.Name == "busy" {
			busy = state
		}
	}
	if busy == nil {
		t.Fatal("busy was not collected")
	}
	if got := graph.Transitions[busy]; len(got) != 0 {
		t.Errorf("expected no transition out of busy, got %d", len(got))
	}
}

// An unqualified endpoint is resolved from where it was written: a transition
// inside beta naming work means beta's work, not the earlier alpha's.
func TestToStateGraph_UnqualifiedEndpointResolvesFromWhereItIsWritten(t *testing.T) {
	src := `
		package test {
			state Machine {
				entry; then start;
				state start;
				state alpha {
					entry; then astart;
					state astart;
					state work;
					succession first astart then work;
				}
				state beta {
					entry; then bstart;
					state bstart;
					state work;
					succession first bstart then work;
					transition first work then done;
				}
				state done;
				succession first start then beta;
			}
		}
	`
	root, machine := parseStateUsage(t, src)
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := scopeOfNode(idx.DocumentRoot("test.sysml"), machine)
	if scope == nil {
		t.Fatal("the index has no scope for the machine")
	}
	graph, err := ToStateGraph(machine, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	var from *ast.StateNode
	for node := range graph.Transitions {
		state, ok := node.(*ast.StateNode)
		if !ok || state.Name != "work" {
			continue
		}
		if graph.ParentState[state] != nil && graph.ParentState[state].Name == "beta" {
			from = state
		}
	}
	if from == nil {
		t.Fatal("expected the transition to leave beta's work, it leaves another state of that name")
	}
}

// scopeOfNode returns the scope node declares its members in.
func scopeOfNode(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if scope == nil {
		return nil
	}
	if scope.Node() == node {
		return scope
	}
	for _, child := range scope.Children() {
		if found := scopeOfNode(child, node); found != nil {
			return found
		}
	}
	return nil
}

// A succession names its endpoints the way a transition does, so one whose
// target is a pseudostate reaches it rather than being left out of the graph.
func TestSuccessionReachesAPseudostate(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy;
				junction route;
				state done;

				succession first start then busy;
				succession first busy then route;
				succession first route then done;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	route := pseudostateNamed(graph, "route")
	if route == nil {
		t.Fatal("junction route was not collected")
	}
	if len(graph.Transitions[route]) != 1 {
		t.Fatalf("transitions out of route = %d, want the succession `route then done`", len(graph.Transitions[route]))
	}
	if target := graph.Transitions[route][0].Target; target != ast.Node(stateNamed(graph, "done")) {
		t.Fatalf("route leads to %v, want done", target)
	}
	if !leadsTo(graph, stateNamed(graph, "busy"), route) {
		t.Fatal("the succession `busy then route` is not in the graph")
	}
}

// A succession qualifying its target reaches the vertex it names, not the first
// one of that simple name: two composite states may each declare a `work`.
func TestSuccessionQualifiedTargetNamesTheVertexItQualifies(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state alpha {
					state work;
				}
				state beta {
					state work;
				}

				succession first start then alpha;
				succession first alpha then beta::work;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	alpha := stateNamed(graph, "alpha")
	if len(graph.Transitions[alpha]) != 1 {
		t.Fatalf("transitions out of alpha = %d, want the succession `alpha then beta::work`", len(graph.Transitions[alpha]))
	}
	target, ok := graph.Transitions[alpha][0].Target.(*ast.StateNode)
	if !ok {
		t.Fatalf("alpha leads to %T, want a state", graph.Transitions[alpha][0].Target)
	}
	if owner := graph.ParentState[target]; owner == nil || owner.Name != "beta" {
		t.Fatalf("alpha leads to the work of %v, want beta's", owner)
	}
}

// Two regions may declare same-named pseudostates, and each is its own vertex.
func TestSameNamedPseudostatesInSiblingRegionsAreBothCollected(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				entry; then start;
				state start;
				state running parallel {
					state left {
						entry; then lstart;
						state lstart;
						state lidle;
						junction pick;
						succession first lstart then lidle;
						succession first lidle then pick;
						succession first pick then lidle;
					}
					state right {
						entry; then rstart;
						state rstart;
						state ridle;
						junction pick;
						succession first rstart then ridle;
						succession first ridle then pick;
						succession first pick then ridle;
					}
				}
				succession first start then running;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	var picks []*ast.PseudostateNode
	for _, ps := range graph.Pseudostates {
		if ps.Name == "pick" {
			picks = append(picks, ps)
		}
	}
	if len(picks) != 2 {
		t.Fatalf("pseudostates named pick = %d, want one per region", len(picks))
	}
	if picks[0] == picks[1] {
		t.Fatal("the two regions share one pseudostate node")
	}
	for _, pick := range picks {
		if len(graph.Transitions[pick]) != 1 {
			t.Fatalf("transitions out of a pick = %d, want its own region's", len(graph.Transitions[pick]))
		}
		target, ok := graph.Transitions[pick][0].Target.(*ast.StateNode)
		if !ok {
			t.Fatalf("a pick leads to %T, want a state", graph.Transitions[pick][0].Target)
		}
		if graph.RegionOf[target] != graph.RegionOf[graph.PseudostateOwner[pick]] &&
			graph.RegionOf[target] == nil {
			t.Fatalf("a pick leads to %s, which no region declares", target.Name)
		}
	}
}

// A state body's two-ended `first a then b;` is the succession `succession first a
// then b;` spelt without its keyword: the same completion transitions, resolving
// nested, qualified, pseudostate and region-local endpoints alike.
func TestKeywordlessSuccessionLowersLikeTheSuccessionKeyword(t *testing.T) {
	const body = `
		package test {
			state Machine {
				entry; then start;
				state start;
				state busy {
					state work;
				}
				junction route;
				state running parallel {
					state left {
						entry; then li;
						state li;
						state idle;
						%[1]s first li then idle;
					}
					state right {
						entry; then ri;
						state ri;
						state idle;
						%[1]s first ri then idle;
					}
				}
				%[1]s first start then busy::work;
				%[1]s first busy then route;
				%[1]s first route then running;
				%[1]s first running then done;
			}
		}
	`
	describe := func(graph *StateGraph) []string {
		var edges []string
		for source, transitions := range graph.Transitions {
			for _, tr := range transitions {
				if tr.Trigger != nil || tr.Guard != nil || tr.Effect != nil {
					t.Errorf("%v -> %v carries a trigger, guard or effect, want a bare completion", source, tr.Target)
				}
				edges = append(edges, vertexName(source)+"->"+vertexName(tr.Target)+"@"+regionName(graph, tr.Target))
			}
		}
		sort.Strings(edges)
		return edges
	}

	root, machine := parseStateUsage(t, fmt.Sprintf(body, "succession"))
	want := describe(graphOf(t, root, machine))
	root, machine = parseStateUsage(t, fmt.Sprintf(body, ""))
	graph := graphOf(t, root, machine)
	got := describe(graph)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keyword-less successions lower to\n%v\nwant the `succession` spelling's\n%v", got, want)
	}
	if graph.Initial != stateNamed(graph, "start") {
		t.Fatalf("the machine starts in %v, want the entry succession's start", graph.Initial)
	}
	if len(want) < 6 {
		t.Fatalf("the `succession` spelling lowered only %v", want)
	}
}

// A succession end written as a feature chain (`c.c1`) names the nested vertex
// the same qualified name (`c::c1`) does, as source and as target, in both the
// `succession` and the keyword-less spelling.
func TestSuccessionChainedEndpointsNameNestedVertices(t *testing.T) {
	const body = `
		package test {
			state Machine {
				entry; then a;
				state a;
				state b {
					state b1;
					state b2;
				}
				state c {
					state c1 {
						state deep;
					}
				}
				%[1]s first a then b%[2]sb1;
				%[1]s first b%[2]sb1 then b%[2]sb2;
				%[1]s first b%[2]sb2 then c%[2]sc1%[2]sdeep;
				%[1]s first c%[2]sc1%[2]sdeep then done;
			}
		}
	`
	describe := func(graph *StateGraph) []string {
		var edges []string
		for source, transitions := range graph.Transitions {
			for _, tr := range transitions {
				if tr.Trigger != nil || tr.Guard != nil || tr.Effect != nil {
					t.Errorf("%v -> %v carries a trigger, guard or effect, want a bare completion", source, tr.Target)
				}
				edges = append(edges, vertexName(source)+"->"+vertexName(tr.Target))
			}
		}
		sort.Strings(edges)
		return edges
	}
	want := []string{"a->b1", "b1->b2", "b2->deep", "deep->done"}
	for _, keyword := range []string{"succession", ""} {
		for _, separator := range []string{"::", "."} {
			name := keyword + "/" + separator
			root, machine := parseStateUsage(t, fmt.Sprintf(body, keyword, separator))
			graph := graphOf(t, root, machine)
			if got := describe(graph); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: lowered %v, want %v", name, got, want)
			}
			if graph.Initial != stateNamed(graph, "a") {
				t.Errorf("%s: the machine starts in %v, want a", name, graph.Initial)
			}
		}
	}
}

// regionName is the name of the region a vertex belongs to, "" at the top level.
func regionName(graph *StateGraph, vertex ast.Node) string {
	if state, ok := vertex.(*ast.StateNode); ok && graph.RegionOf[state] != nil {
		return graph.RegionOf[state].Name
	}
	return ""
}

// graphOf lowers the machine src declares with the scope tree of its document.
func graphOf(t *testing.T, root *ast.RootNamespace, machine *ast.Usage) *StateGraph {
	t.Helper()
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := scopeOfNode(idx.DocumentRoot("test.sysml"), machine)
	if scope == nil {
		t.Fatal("the index has no scope for the machine")
	}
	graph, err := ToStateGraph(machine, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	return graph
}

// stateNamed is the graph's state with that simple name, or nil.
func stateNamed(graph *StateGraph, name string) *ast.StateNode {
	for _, state := range graph.States {
		if state.Name == name {
			return state
		}
	}
	return nil
}

// leadsTo reports whether a transition of the graph goes from source to target.
func leadsTo(graph *StateGraph, source, target ast.Node) bool {
	for _, trans := range graph.Transitions[source] {
		if trans.Target == target {
			return true
		}
	}
	return false
}

// A machine lowered without a scope tree is indexed on its own, and that index
// is descended per region: a succession names its own region's same-named state.
func TestScopelessLoweringNamesTheRegionLocalState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				state both parallel {
					state left {
						entry; then li;
						state li;
						state idle;
						state done;
						succession first li then idle;
						succession first idle then done;
					}
					state right {
						entry; then ri;
						state ri;
						state idle;
						state ready;
						succession first ri then idle;
						succession first idle then ready;
					}
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	for _, region := range graph.CompositeStates[stateNamed(graph, "both")] {
		for state, owner := range graph.RegionOf {
			if owner != region || state.Name != "idle" {
				continue
			}
			transitions := graph.Transitions[state]
			if len(transitions) != 1 {
				t.Fatalf("transitions out of %s's idle = %d, want its own", region.Name, len(transitions))
			}
			target, ok := transitions[0].Target.(*ast.StateNode)
			if !ok || graph.RegionOf[target] != region {
				t.Fatalf("%s's idle leads outside its region, to %v", region.Name, transitions[0].Target)
			}
		}
	}
}
