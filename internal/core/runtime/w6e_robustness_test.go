package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A completion self-transition on a simple state re-runs its exit and entry
// actions every round, so the run must stay bounded and report rather than hang.
func TestSimpleSelfTransitionThatNeverSettlesIsBounded(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			entry; then start;
			state start;
			state s {
				entry { assign log := log + 1; }
				exit { assign log := log + 1; }
			}

			succession first start then s;
			transition first s do assign log := log + 1 then s;
		}
	}`)

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()

	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run to completion hangs on a simple state transitioning to itself")
	}
	if err == nil {
		t.Fatal("expected a budget error for a self-transition that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("err = %v; want a budget error", err)
	}
}

// The `start` shot a state inherits is where `first start then off;` starts the
// machine, not a vertex: a triggered transition leaving it is refused when the
// machine is built.
func TestFirstMarkerNamedAsATransitionSourceIsRefused(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		attribute def StartSignal;
		state Machine {
			first start then off;
			state off;
			transition t1 first start accept StartSignal then off;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}

	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected building the machine to report the marker source")
	}
	if !strings.Contains(err.Error(), "start") || !strings.Contains(err.Error(), "not a vertex") {
		t.Errorf("err = %v; want it to name the endpoint as no vertex", err)
	}
}

// Only the library's own `start` is the shot `first start then off;` leaves from:
// a state usage merely named `start`, whether declared beside the machine or
// inherited from a definition of the model, designates no starting state.
func TestFirstStartNamingAModelStateUsageDesignatesNoStart(t *testing.T) {
	cases := map[string]struct {
		src       string
		libraries bool
	}{
		"state beside the machine, no library": {src: `package test {
			state start;
			state Machine {
				first start then off;
				state off;
			}
		}`},
		"state inherited from a definition of the model": {libraries: true, src: `package test {
			state def Base { state start; }
			state def Machine :> Base {
				first start then off;
				state off;
			}
		}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			build := buildRuntime
			if tc.libraries {
				build = buildRuntimeWithLibraries
			}
			idx, _, ctx := build(t, "<test>", parseAndBuild(t, tc.src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
			if sym == nil {
				t.Fatal("state machine Machine not found")
			}
			exec, err := newStateExecutor(ctx, sym, nil)
			if err != nil {
				t.Fatalf("newStateExecutor: %v", err)
			}
			err = exec.initialize()
			if !errors.Is(err, ErrNoInitialState) {
				t.Fatalf("initialize = %v; want ErrNoInitialState, `start` naming no shot of the library", err)
			}
		})
	}
}

// The effect of a self-transition is executed between the exit and the entry, so
// an effect reading a feature the machine does not declare reports there.
func TestSimpleSelfTransitionEffectReadingAnUnknownFeatureIsReported(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			entry; then start;
			state start;
			state s {
				entry { assign log := log + 1; }
				exit { assign log := log + 1; }
			}

			succession first start then s;
			transition first s accept again do assign log := missingName + 1 then s;
		}
	}`)

	exec.SendSignal("again", nil)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
	// The exit ran before the effect failed, so the entry did not: log is 1+1.
	if got := exec.StateData()["log"]; got.Const.Int != 2 {
		t.Errorf("log = %v; want 2 (entry, exit), the entry not re-run after a failed effect", got.Const.Int)
	}
}
