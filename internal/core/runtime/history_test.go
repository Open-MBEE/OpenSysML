package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// transitionBetween returns the lowered transition from the named source to the
// named target, so a test can drive an exact sequence of transitions instead of
// relying on the order completion events happen to be queued in.
func transitionBetween(t *testing.T, exec *StateExecutor, source, target string) *lower.Transition {
	t.Helper()
	for node, transitions := range exec.graph.Transitions {
		if getNodeName(node) != source {
			continue
		}
		for _, trans := range transitions {
			if getNodeName(trans.Target) == target {
				return trans
			}
		}
	}
	t.Fatalf("no transition %s -> %s in the lowered graph", source, target)
	return nil
}

func fire(t *testing.T, exec *StateExecutor, source, target string) {
	t.Helper()
	fired, err := exec.resolveAndFire(nil, transitionBetween(t, exec, source, target))
	if err != nil {
		t.Fatalf("fire %s -> %s: %v", source, target, err)
	}
	if !fired {
		t.Fatalf("fire %s -> %s: not taken", source, target)
	}
}

// visitsAfter returns the states visited since mark, so a test can assert what
// one transition entered rather than the whole run.
func visitsAfter(exec *StateExecutor, mark int) []string {
	return exec.stateVisits[mark:]
}

// shallowHistoryMachine is `outer` with two flat substates and a shallow history
// that a state outside `outer` transitions into. The machines here are built on the
// AST directly to vary one detail at a time; the `history` notation is covered by
// the conformance cases.
func shallowHistoryMachine() *ast.Usage {
	first := &ast.StateNode{Name: "first"}
	second := &ast.StateNode{Name: "second"}
	history := &ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"}
	outer := &ast.StateNode{Name: "outer", Substates: []ast.Node{first, second, history}}

	return &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "first"),
			transitionMember("first", "second"),
			transitionMember("second", "away"),
			transitionMember("away", "H"),
			transitionMember("H", "first"), // default history transition
		},
	}
}

// A shallow history re-enters the substate that was active when the composite
// state was last exited, not the composite's default one.
func TestShallowHistoryRestoresLastSubstate(t *testing.T) {
	exec := stateExecutorFor(t, shallowHistoryMachine())
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "first")
	fire(t, exec, "first", "second")
	fire(t, exec, "second", "away")

	mark := len(exec.stateVisits)
	fire(t, exec, "away", "H")

	if got := exec.getCurrentState(); got == nil || got.Name != "second" {
		t.Fatalf("current state = %v, want second", got)
	}
	if got := visitsAfter(exec, mark); len(got) != 2 || got[0] != "outer" || got[1] != "second" {
		t.Errorf("history entered %v, want [outer second]", got)
	}
}

// Before the composite state has ever been exited there is nothing to restore,
// so the history's own outgoing transition supplies the target.
func TestHistoryTakesDefaultTransitionWhenUnvisited(t *testing.T) {
	machine := shallowHistoryMachine()
	// Reach `away` without ever entering `outer`, so no configuration was recorded.
	machine.Members = append(machine.Members, transitionMember("init", "away"))

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "away")
	fire(t, exec, "away", "H")

	if got := exec.getCurrentState(); got == nil || got.Name != "first" {
		t.Errorf("current state = %v, want first (default history transition)", got)
	}
}

// A deep history restores the innermost substate that was active, while a
// shallow history stops at the composite state's own child.
func TestDeepHistoryRestoresInnermostSubstate(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind ast.PseudostateKind
		want string
	}{
		{"shallow", ast.PseudostateShallowHistory, "mid"},
		{"deep", ast.PseudostateDeepHistory, "inner2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner1 := &ast.StateNode{Name: "inner1"}
			inner2 := &ast.StateNode{Name: "inner2"}
			mid := &ast.StateNode{Name: "mid", Substates: []ast.Node{inner1, inner2}}
			history := &ast.PseudostateNode{Kind: tc.kind, Name: "H"}
			outer := &ast.StateNode{Name: "outer", Substates: []ast.Node{mid, history}}

			exec := stateExecutorFor(t, &ast.Usage{
				Kind:  ast.UsageState,
				Ident: ast.Identification{Name: "Machine"},
				Members: []ast.Node{
					entryStart("init"),
					&ast.StateNode{Name: "init"},
					outer,
					&ast.StateNode{Name: "away"},
					transitionMember("init", "inner1"),
					transitionMember("inner1", "inner2"),
					transitionMember("inner2", "away"),
					transitionMember("away", "H"),
				},
			})
			if err := exec.initialize(); err != nil {
				t.Fatalf("initialize: %v", err)
			}

			fire(t, exec, "init", "inner1")
			fire(t, exec, "inner1", "inner2")
			fire(t, exec, "inner2", "away")
			fire(t, exec, "away", "H")

			if got := exec.getCurrentState(); got == nil || got.Name != tc.want {
				t.Errorf("current state = %v, want %s", got, tc.want)
			}
		})
	}
}

// regionHistoryMachine is `outer` with two orthogonal regions, each of which can
// advance independently, and a history that restores both at once.
func regionHistoryMachine(kind ast.PseudostateKind, nestLeft bool) *ast.Usage {
	lstart := &ast.StateNode{Name: "lstart"}
	lwork := &ast.StateNode{Name: "lwork"}
	var extra []ast.Node
	if nestLeft {
		// A composite region state: its own configuration sits below the region.
		lwork.Substates = []ast.Node{&ast.StateNode{Name: "ldeep"}}
		extra = append(extra, transitionMember("lwork", "ldeep"))
	}
	rstart := &ast.StateNode{Name: "rstart"}
	rwork := &ast.StateNode{Name: "rwork"}

	left := &ast.StateRegion{Name: "left", States: []ast.Node{entryStart("lstart"), lstart, lwork}}
	right := &ast.StateRegion{Name: "right", States: []ast.Node{entryStart("rstart"), rstart, rwork}}
	history := &ast.PseudostateNode{Kind: kind, Name: "H"}
	outer := &ast.StateNode{
		Name:      "outer",
		Regions:   []*ast.StateRegion{left, right},
		Substates: []ast.Node{history},
	}

	return &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: append([]ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "outer"),
			transitionMember("lstart", "lwork"),
			transitionMember("rstart", "rwork"),
			transitionMember("outer", "away"),
			transitionMember("away", "H"),
		}, extra...),
	}
}

// advanceRegion fires a region-local transition, the way the event loop does for
// a composite state with orthogonal regions.
func advanceRegion(t *testing.T, exec *StateExecutor, regionName, source, target string) {
	t.Helper()
	for region := range exec.activeConfig.regionStates {
		if region.Name != regionName {
			continue
		}
		trans := transitionBetween(t, exec, source, target)
		route, err := exec.resolveRoute(trans)
		if err != nil {
			t.Fatalf("advance region %s: %v", regionName, err)
		}
		fired, err := exec.fireTransitionInRegion(region, trans, route)
		if err != nil {
			t.Fatalf("advance region %s: %v", regionName, err)
		}
		if !fired {
			t.Fatalf("advance region %s: %s -> %s not taken", regionName, source, target)
		}
		return
	}
	t.Fatalf("region %s is not active", regionName)
}

// A history restores every orthogonal region of its composite state to the state
// that region was last in, not to the regions' initial states.
func TestHistoryRestoresOrthogonalRegions(t *testing.T) {
	exec := stateExecutorFor(t, regionHistoryMachine(ast.PseudostateShallowHistory, false))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "outer")
	advanceRegion(t, exec, "left", "lstart", "lwork")
	advanceRegion(t, exec, "right", "rstart", "rwork")
	fire(t, exec, "outer", "away")

	mark := len(exec.stateVisits)
	fire(t, exec, "away", "H")

	restored := map[string]bool{}
	for _, state := range exec.activeConfig.regionStates {
		restored[state.Name] = true
	}
	if !restored["lwork"] || !restored["rwork"] {
		t.Errorf("restored regions = %v, want lwork and rwork", restored)
	}
	for _, entered := range visitsAfter(exec, mark) {
		if entered == "lstart" || entered == "rstart" {
			t.Errorf("history re-entered the region's initial state %s, visits: %v", entered, visitsAfter(exec, mark))
		}
	}
}

// A region left by a transition that started inside its own composite state's
// region still records the state it was left in, so a deep history restores that
// composite state and its inner configuration instead of the region's initial state.
func TestDeepHistoryRestoresARegionLeftFromInsideItsCompositeState(t *testing.T) {
	istart := &ast.StateNode{Name: "istart"}
	ideep := &ast.StateNode{Name: "ideep"}
	inner := &ast.StateRegion{Name: "inner", States: []ast.Node{entryStart("istart"), istart, ideep}}
	wrapper := &ast.StateNode{Name: "wrapper", Regions: []*ast.StateRegion{inner}}

	lstart := &ast.StateNode{Name: "lstart"}
	lwork := &ast.StateNode{Name: "lwork"}
	rstart := &ast.StateNode{Name: "rstart"}
	left := &ast.StateRegion{Name: "left", States: []ast.Node{entryStart("lstart"), lstart, lwork}}
	right := &ast.StateRegion{Name: "right", States: []ast.Node{entryStart("rstart"), rstart, wrapper}}
	history := &ast.PseudostateNode{Kind: ast.PseudostateDeepHistory, Name: "H"}
	outer := &ast.StateNode{
		Name:      "outer",
		Regions:   []*ast.StateRegion{left, right},
		Substates: []ast.Node{history},
	}

	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "outer"),
			transitionMember("lstart", "lwork"),
			transitionMember("rstart", "wrapper"),
			transitionMember("istart", "ideep"),
			transitionMember("ideep", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "outer")
	advanceRegion(t, exec, "left", "lstart", "lwork")
	advanceRegion(t, exec, "right", "rstart", "wrapper")
	advanceRegion(t, exec, "inner", "istart", "ideep")
	// Leaving the composite state from inside its own region records both regions.
	advanceRegion(t, exec, "inner", "ideep", "away")

	mark := len(exec.stateVisits)
	fire(t, exec, "away", "H")

	restored := map[string]bool{}
	for _, state := range exec.activeConfig.regionStates {
		restored[state.Name] = true
	}
	if !restored["lwork"] || !restored["wrapper"] || !restored["ideep"] {
		t.Errorf("restored regions = %v, want lwork, wrapper and ideep", restored)
	}
	for _, entered := range visitsAfter(exec, mark) {
		if entered == "rstart" || entered == "istart" {
			t.Errorf("history re-entered the initial state %s, visits: %v", entered, visitsAfter(exec, mark))
		}
	}
}

// A deep history restores a configuration nested below an orthogonal region:
// entering the region at the innermost recorded state still runs the entry
// behavior of the states above it inside that region.
func TestDeepHistoryRestoresBelowRegion(t *testing.T) {
	exec := stateExecutorFor(t, regionHistoryMachine(ast.PseudostateDeepHistory, true))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "outer")
	advanceRegion(t, exec, "left", "lstart", "lwork")
	advanceRegion(t, exec, "left", "lwork", "ldeep")
	advanceRegion(t, exec, "right", "rstart", "rwork")
	fire(t, exec, "outer", "away")

	mark := len(exec.stateVisits)
	fire(t, exec, "away", "H")

	restored := map[string]bool{}
	for _, state := range exec.activeConfig.regionStates {
		restored[state.Name] = true
	}
	if !restored["ldeep"] || !restored["rwork"] {
		t.Errorf("restored regions = %v, want ldeep and rwork", restored)
	}
	entered := strings.Join(visitsAfter(exec, mark), " ")
	for _, want := range []string{"outer", "lwork", "ldeep", "rwork"} {
		if !strings.Contains(entered, want) {
			t.Errorf("history entered %q, want it to include %s", entered, want)
		}
	}
}

// Leaving a composite state that sits inside an orthogonal region leaves the
// sibling regions of the enclosing state running: the active configuration of
// every composite state shares one map, so a teardown must only touch the
// regions of the state being left.
func TestExitingNestedRegionsKeepsSiblingRegions(t *testing.T) {
	innerA := &ast.StateNode{Name: "innerA"}
	innerB := &ast.StateNode{Name: "innerB"}
	nested := &ast.StateNode{
		Name: "nested",
		Regions: []*ast.StateRegion{
			{Name: "a", States: []ast.Node{entryStart("innerA"), innerA}},
			{Name: "b", States: []ast.Node{entryStart("innerB"), innerB}},
		},
	}
	leftDone := &ast.StateNode{Name: "leftDone"}
	rstart := &ast.StateNode{Name: "rstart"}

	outer := &ast.StateNode{
		Name: "outer",
		Regions: []*ast.StateRegion{
			{Name: "left", States: []ast.Node{entryStart("nested"), nested, leftDone}},
			{Name: "right", States: []ast.Node{entryStart("rstart"), rstart}},
		},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			transitionMember("init", "nested"),
			transitionMember("nested", "leftDone"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "nested")
	advanceRegion(t, exec, "left", "nested", "leftDone")

	active := map[string]bool{}
	for _, state := range exec.activeConfig.regionStates {
		active[state.Name] = true
	}
	if !active["rstart"] {
		t.Errorf("active configuration = %v, want the sibling region to still hold rstart", active)
	}
	if !active["leftDone"] {
		t.Errorf("active configuration = %v, want the left region to hold leftDone", active)
	}
	if active["innerA"] || active["innerB"] {
		t.Errorf("active configuration = %v, want the left nested regions to be gone", active)
	}
}
