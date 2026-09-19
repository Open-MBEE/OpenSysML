package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// executeStateSource executes the named state machine declared in src and
// returns its outputs together with the ordered list of visited states.
func executeStateSource(t *testing.T, name, src string) (map[string]Value, []string, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	return ctx.ExecuteStateWithEvents(sym, nil)
}

// A fork enters its branch targets directly, so the initial state of a targeted
// region is bypassed and its entry behavior does not run. A region the fork does
// not target still starts at its own initial state.
const forkPartialRegions = `package P {
	state Machine {
		attribute leftInit : Integer = 0;
		attribute leftWork : Integer = 0;
		attribute rightInit : Integer = 0;
		attribute rightWork : Integer = 0;

		entry; then init;
		state init;
		state ready;
		state working parallel {
			state left {
				entry; then ls;
				state ls;
				state lstart { entry { assign leftInit := 1; } }
				state lwork { entry { assign leftWork := 1; } }
				succession first ls then lstart;
			}
			state right {
				entry; then rs;
				state rs;
				state rstart { entry { assign rightInit := 1; } }
				state rwork { entry { assign rightWork := 1; } }
				succession first rs then rstart;
			}
			state aux {
				entry; then as;
				state as;
				state astart;
				succession first as then astart;
			}
		}
		fork split;
		join sync;

		succession first init then ready;
		transition first ready then split;
		transition first split then lwork;
		transition first split then rwork;
		transition first lwork then sync;
		transition first rwork then sync;
		transition first sync then done;
	}
}`

func TestForkBypassesTargetedRegionInitials(t *testing.T) {
	outputs, visits, err := executeStateSource(t, "Machine", forkPartialRegions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	want := map[string]int64{
		"leftInit":  0, // bypassed by the fork
		"rightInit": 0, // bypassed by the fork
		"leftWork":  1,
		"rightWork": 1,
	}
	for name, expected := range want {
		val, ok := outputs[name]
		if !ok {
			t.Errorf("missing output %s", name)
			continue
		}
		if val.Const.Int != expected {
			t.Errorf("%s = %d, want %d", name, val.Const.Int, expected)
		}
	}

	visited := strings.Join(visits, ",")
	for _, bypassed := range []string{"lstart", "rstart"} {
		if strings.Contains(visited, bypassed) {
			t.Errorf("fork entered %s, which its branch bypasses; visits: %v", bypassed, visits)
		}
	}
	// The region the fork does not target still starts at its own initial.
	if !strings.Contains(visited, "as") {
		t.Errorf("untargeted region aux was not initialized; visits: %v", visits)
	}
}

// Branch entry and exit order is observable through stateVisits and the trace,
// so it must not depend on map iteration order.
func TestForkJoinVisitOrderIsDeterministic(t *testing.T) {
	var first []string
	for i := 0; i < 25; i++ {
		_, visits, err := executeStateSource(t, "Machine", forkPartialRegions)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if i == 0 {
			first = visits
			continue
		}
		if strings.Join(visits, ",") != strings.Join(first, ",") {
			t.Fatalf("run %d visited %v, run 0 visited %v", i, visits, first)
		}
	}
}
