package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// runStateMachine runs a machine to completion and returns its executor, so a
// test can assert the active configuration each region ended in.
func runStateMachine(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	ctx, sym := loadState(t, src, name)
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	return exec
}

// regionConfig returns the active state of every region, keyed by region name.
func regionConfig(exec *StateExecutor) map[string]string {
	config := make(map[string]string, len(exec.activeConfig.regionStates))
	for region, state := range exec.activeConfig.regionStates {
		config[region.Name] = state.Name
	}
	return config
}

func assertRegionConfig(t *testing.T, exec *StateExecutor, want map[string]string) {
	t.Helper()
	got := regionConfig(exec)
	if len(got) != len(want) {
		t.Fatalf("region configuration = %v, want %v", got, want)
	}
	for region, state := range want {
		if got[region] != state {
			t.Errorf("region %s is in %q, want %q", region, got[region], state)
		}
	}
}

// A choice reached from inside an orthogonal region whose branch stays in that
// region moves only that region: its siblings keep the state they were in and
// their entry behaviors do not run again.
func TestRegionLocalChoiceMovesOnlyItsRegion(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine parallel {
		attribute x : Integer = 2;
		attribute rightEntries : Integer = 0;

		state left {
			entry; then ls;
			state ls;
			state lstart;
			state lnext;
			state lother;
			succession first ls then lstart;
			transition first lstart then pick;
		}
		state right {
			entry; then rs;
			state rs;
			state rstart { entry { assign rightEntries := rightEntries + 1; } }
			succession first rs then rstart;
		}
		choice pick;

		transition first pick if x == 2 then lnext;
		transition first pick then lother;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lnext", "right": "rstart"})
	if got := intValue(t, exec.stateData, "rightEntries"); got != 1 {
		t.Errorf("rightEntries = %d, want 1 (the sibling region must not be re-entered)", got)
	}
}

// A choice's branches are taken in declaration order and an unguarded branch is
// the else branch, inside an orthogonal region as anywhere else.
func TestRegionLocalChoiceTakesElseBranch(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine parallel {
		attribute x : Integer = 7;

		state left {
			entry; then ls;
			state ls;
			state lstart;
			state lguarded;
			state lelse;
			succession first ls then lstart;
			transition first lstart then pick;
		}
		state right {
			entry; then rs;
			state rs;
			state rstart;
			succession first rs then rstart;
		}
		choice pick;

		transition first pick if x == 2 then lguarded;
		transition first pick then lelse;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lelse", "right": "rstart"})
}

// A junction reached from inside an orthogonal region routes on like a choice,
// and a chain of them is followed to the state it ends at.
func TestRegionLocalJunctionChainIsFollowed(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine parallel {
		state left {
			entry; then ls;
			state ls;
			state a;
			state b;
			succession first ls then a;
			transition first a then merge;
		}
		state right {
			entry; then rs;
			state rs;
			state c;
			succession first rs then c;
		}
		junction merge;
		junction relay;

		transition first merge then relay;
		transition first relay then b;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "b", "right": "c"})
}

// A branch that leaves the region set for a state outside it leaves every region:
// their exit behaviors run and the machine ends with a single active state.
func TestRegionPseudostateLeavingEveryRegion(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute x : Integer = 1;
		attribute leftExits : Integer = 0;
		attribute rightExits : Integer = 0;

		entry; then running;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lstart { exit { assign leftExits := leftExits + 1; } }
				succession first ls then lstart;
				transition first lstart then pick;
			}
			state right {
				entry; then rs;
				state rs;
				state rstart { exit { assign rightExits := rightExits + 1; } }
				succession first rs then rstart;
			}
		}
		choice pick;
		state outside;

		transition first pick if x == 1 then outside;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "outside" {
		t.Fatalf("current state = %v, want outside", got)
	}
	for _, name := range []string{"leftExits", "rightExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1 (every region of the set is left)", name, got)
		}
	}
}

// A branch into a sibling region of the same composite state exits its source
// only — KerML StateTransitionPerformance orders `guard then
// transitionLinkSource.exit` — so the state owning the regions is not re-entered.
func TestRegionPseudostateIntoSiblingRegionExitsSourceOnly(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute x : Integer = 1;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lstart;
				succession first ls then lstart;
				transition first lstart if x == 1 then cross;
			}
			state right {
				entry; then rs;
				state rs;
				state rstart;
				state rtarget { entry { assign x := 2; } }
				succession first rs then rstart;
			}
		}
		choice cross;

		succession first init then running;
		transition first cross if x == 1 then rtarget;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"right": "rtarget"})
	if got := countVisits(exec.stateVisits, "running"); got != 1 {
		t.Errorf("running entered %d times, want 1 (the composite state is not re-entered)", got)
	}
	if got := countVisits(exec.stateVisits, "lstart"); got != 1 {
		t.Errorf("lstart entered %d times, want 1 (the source region does not restart)", got)
	}
}

// Leaving a composite state through a pseudostate reached from inside one of its
// regions records the configuration of every region, so a history entered later
// restores what the regions were doing rather than their initial states.
func TestRegionPseudostateExitRecordsHistory(t *testing.T) {
	history := &ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"}
	lwork := &ast.StateNode{Name: "lwork"}
	rwork := &ast.StateNode{Name: "rwork"}
	outer := &ast.StateNode{
		Name: "outer",
		Regions: []*ast.StateRegion{
			{Name: "left", States: []ast.Node{entryStart("lstart"), &ast.StateNode{Name: "lstart"}, lwork}},
			{Name: "right", States: []ast.Node{entryStart("rstart"), &ast.StateNode{Name: "rstart"}, rwork}},
		},
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
			&ast.PseudostateNode{Kind: ast.PseudostateJunction, Name: "out"},
			transitionMember("init", "outer"),
			transitionMember("lstart", "lwork"),
			transitionMember("rstart", "rwork"),
			transitionMember("lwork", "out"),
			transitionMember("out", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	fire(t, exec, "init", "outer")
	advanceRegion(t, exec, "left", "lstart", "lwork")
	advanceRegion(t, exec, "right", "rstart", "rwork")

	// A junction reached from inside the left region routes out of the whole state.
	advanceRegion(t, exec, "left", "lwork", "out")
	if got := exec.getCurrentState(); got == nil || got.Name != "away" {
		t.Fatalf("current state = %v, want away (the junction routes out of outer)", got)
	}
	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active after leaving outer: %v", regionConfig(exec))
	}

	fire(t, exec, "away", "H")
	assertRegionConfig(t, exec, map[string]string{"left": "lwork", "right": "rwork"})
}

// The regions of the machine itself are left in declaration order, so the states
// visited when a pseudostate routes out of them do not depend on map iteration
// order.
func TestRegionPseudostateExitOrderIsDeterministic(t *testing.T) {
	const machine = `package P {
	state Machine {
		attribute x : Integer = 1;

		entry; then running;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lstart;
				succession first ls then lstart;
				transition first lstart then pick;
			}
			state middle {
				entry; then ms;
				state ms;
				state mstart;
				succession first ms then mstart;
			}
			state right {
				entry; then rs;
				state rs;
				state rstart;
				succession first rs then rstart;
			}
		}
		choice pick;
		state outside;

		transition first pick if x == 1 then outside;
	}
}`

	var first string
	for run := 0; run < 25; run++ {
		exec := runStateMachine(t, "Machine", machine)
		visits := strings.Join(exec.stateVisits, ",")
		if run == 0 {
			first = visits
			continue
		}
		if visits != first {
			t.Fatalf("run %d visited %q, run 0 visited %q", run, visits, first)
		}
	}
}

func countVisits(visits []string, name string) int {
	count := 0
	for _, visit := range visits {
		if visit == name {
			count++
		}
	}
	return count
}
