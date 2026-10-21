package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// executionAllocs is what running the action named run in src costs in
// allocations once the model is warm.
func executionAllocs(t *testing.T, src string) float64 {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "run", ast.DefAction)
	if sym == nil {
		t.Fatal("action run not found")
	}
	return testing.AllocsPerRun(10, func() {
		if _, err := ctx.ExecuteAction(sym); err != nil {
			t.Fatal(err)
		}
	})
}

// actionLoopPassAllocs is what one more pass of an action's `for` loop costs in
// allocations: the difference between runs of 2n and n passes, per pass. The
// total assigned is the loop's own action's where own is set, else its parent's.
func actionLoopPassAllocs(t *testing.T, n int, own bool) float64 {
	t.Helper()
	run := func(passes int) float64 {
		outer, inner := "attribute total : Integer = 0;", ""
		if own {
			outer, inner = inner, outer
		}
		return executionAllocs(t, fmt.Sprintf(`package T {
			private import ScalarValues::*;
			action run {
				%s
				first start;
				action iterate {
					%s
					for i in 1..%d {
						assign total := total + i;
					}
				}
				done;
				succession first start then iterate;
				succession first iterate then done;
			}
		}`, outer, inner, passes))
	}
	return (run(2*n) - run(n)) / float64(n)
}

// A pass of an action's loop assigns through a footprint stamped for the
// performance once, not re-stamped (two slices cloned) per pass: bound below.
func TestActionLoopPassDoesNotRestampStatementFootprint(t *testing.T) {
	const bound = 4.5
	for name, own := range map[string]bool{"parent's attribute": false, "own attribute": true} {
		t.Run(name, func(t *testing.T) {
			got := actionLoopPassAllocs(t, 200, own)
			t.Logf("one more pass allocates %.2f objects", got)
			if got > bound {
				t.Errorf("one more pass of the loop allocates %.2f objects, want at most %v", got, bound)
			}
		})
	}
}

// actionChainStepAllocs is what one more action in a chain of assigning actions
// costs in allocations: the difference between chains of 2n and n, per action.
func actionChainStepAllocs(t *testing.T, n int) float64 {
	t.Helper()
	run := func(length int) float64 {
		var actions, successions strings.Builder
		for i := 0; i < length; i++ {
			fmt.Fprintf(&actions, "action a%d { assign total := total + 1; }\n", i)
			if i > 0 {
				fmt.Fprintf(&successions, "succession first a%d then a%d;\n", i-1, i)
			}
		}
		return executionAllocs(t, fmt.Sprintf(`package T {
			private import ScalarValues::*;
			action run {
				attribute total : Integer = 0;
				first start;
				%s
				done;
				succession first start then a0;
				%s
				succession first a%d then done;
			}
		}`, actions.String(), successions.String(), length-1))
	}
	return (run(2*n) - run(n)) / float64(n)
}

// An action's body has its statement order and stamped footprint computed once
// per model, not once per executor: a chained action costs under the bound below.
func TestActionBodyOrderIsComputedOncePerModel(t *testing.T) {
	const bound = 74.0
	got := actionChainStepAllocs(t, 50)
	t.Logf("one more chained action allocates %.2f objects", got)
	if got > bound {
		t.Errorf("one more chained action allocates %.2f objects, want at most %v", got, bound)
	}
}

// stateEventAllocs is what one more event of a counting state machine costs in
// allocations: the difference between runs counting to 2n and to n, per event.
func stateEventAllocs(t *testing.T, n int) float64 {
	t.Helper()
	run := func(count int) float64 {
		src := fmt.Sprintf(`package T {
			private import ScalarValues::*;
			state def SM {
				attribute count : Integer = 0;
				entry; then s0;
				state s0;
				accept after 1 [SI::s] if count < %d then s1;
				accept after 1 [SI::s] if count >= %d then done;
				state s1 {
					entry assign count := count + 1;
				}
				accept after 1 [SI::s] then s0;
			}
		}`, count, count)
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "SM", ast.DefState)
		if sym == nil {
			t.Fatal("state def SM not found")
		}
		return testing.AllocsPerRun(10, func() {
			exec, err := ctx.CreateStateExecutor(sym)
			if err != nil {
				t.Fatal(err)
			}
			for exec.State() == StateRunning && exec.HasPendingWork() {
				if err := exec.ProcessNextEvent(); err != nil {
					t.Fatal(err)
				}
			}
			if exec.State() != StateCompleted {
				t.Fatalf("state %v", exec.State())
			}
		})
	}
	return (run(2*n) - run(n)) / float64(2*n)
}

// A state's entry behavior has its statement order computed once per model, not
// on every entry: an event entering the state costs under the bound below.
func TestStateEntryOrderIsComputedOncePerModel(t *testing.T) {
	const bound = 128.0
	got := stateEventAllocs(t, 100)
	t.Logf("one more event allocates %.2f objects", got)
	if got > bound {
		t.Errorf("one more event allocates %.2f objects, want at most %v", got, bound)
	}
}
