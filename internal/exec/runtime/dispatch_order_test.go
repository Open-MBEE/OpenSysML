package runtime

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// dispatchOrderRun advances one machine, whose two regions each arm a timer
// falling due at t=2, under policy; it returns the value the effects left in
// `last` and the choices of the machine's entry and of the advance, in order.
func dispatchOrderRun(t *testing.T, policy SchedulePolicy) (int64, []ChoicePoint) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			state twin {
				attribute last : Integer = 0;
				entry; then work;
				state work parallel {
					state a {
						entry; then a1;
						state a1;
						state a2;
						transition a1 then a2 accept after 2 [s] do assign last := 1;
					}
					state b {
						entry; then b1;
						state b1;
						state b2;
						transition b1 then b2 accept after 2 [s] do assign last := 2;
					}
				}
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "twin", ast.DefState)
	if sym == nil {
		t.Fatal("state twin not found")
	}
	ctx.SetSchedule(policy)
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	choices := ctx.Choices()
	if _, err := ctx.Advance(3); err != nil {
		t.Fatalf("%s: Advance: %v", policy, err)
	}
	last := exec.StateData()["last"]
	if last.Kind != ValConst {
		t.Fatalf("%s: last = %v, want an integer", policy, last)
	}
	return last.Const.Int, append(choices, ctx.Choices()...)
}

// Two time triggers falling due at one instant are a dispatch-order choice: the
// earlier armed first by default and under `declared`, a draw under a seed, and
// the choice is spelt so a witness reads back to it.
func TestDispatchOrderChoice(t *testing.T) {
	for _, tc := range []struct {
		policy string
		last   int64
	}{{"reverse", 2}, {"declared", 2}} {
		last, choices := dispatchOrderRun(t, mustPolicy(t, tc.policy))
		if last != tc.last {
			t.Errorf("%s: last = %d, want %d (arrival order kept)", tc.policy, last, tc.last)
		}
		if len(choices) != 2 || choices[0].Kind != ChoiceEntryOrder {
			t.Fatalf("%s: choices = %v, want the regions' entry order and the one dispatch-order choice", tc.policy, choices)
		}
		c := choices[1]
		if c.Kind != ChoiceDispatchOrder || c.Where != "events at t=2.0" || c.Taken != 0 {
			t.Errorf("%s: choice = %+v, want a dispatch order at t=2.0 taking the first", tc.policy, c)
		}
		if len(c.Alternatives) != 2 || c.Alternatives[0] != "time a1 1->a2" || c.Alternatives[1] != "time b1 1->b2" {
			t.Errorf("%s: alternatives = %v, want the two timers by state and transition", tc.policy, c.Alternatives)
		}
		want := "events at t=2.0: time a1 1->a2, time b1 1->b2 (unordered; dispatched time a1 1->a2 first)"
		if got := c.Describe(); got != want {
			t.Errorf("%s: Describe() = %q, want %q", tc.policy, got, want)
		}
		taken := c.Choice()
		line := taken.String()
		parsed, err := ParseChoice(line)
		if err != nil {
			t.Fatalf("ParseChoice(%q): %v", line, err)
		}
		if parsed.Kind != ChoiceDispatchOrder || parsed.Where != taken.Where || parsed.Took != taken.Took || parsed.Taken != 0 {
			t.Errorf("ParseChoice(%q) = %+v, want %+v", line, parsed, taken)
		}
	}

	seen := map[int64]bool{}
	for seed := 0; seed < 16; seed++ {
		last, choices := dispatchOrderRun(t, mustPolicy(t, fmt.Sprintf("seed:%d", seed)))
		if len(choices) != 2 || choices[1].Kind != ChoiceDispatchOrder {
			t.Fatalf("seed:%d: choices = %v, want the entry order and one dispatch-order choice", seed, choices)
		}
		seen[last] = true
	}
	if !seen[1] || !seen[2] {
		t.Errorf("sixteen seeds left last in only %v; want both orders drawn", seen)
	}
}

// A replay follows a dispatch order to the outcome it names and refuses one
// naming an event not due.
func TestDispatchOrderReplay(t *testing.T) {
	_, choices := dispatchOrderRun(t, mustPolicy(t, "seed:1"))
	entry, witness := choices[0].Choice(), choices[1].Choice()
	witness.Taken, witness.Took = 1, witness.Among[1]
	last, replayed := dispatchOrderRun(t, ReplayPolicy([]ChoiceTaken{entry, witness}))
	if last != 1 {
		t.Errorf("replaying the second-first order left last = %d, want 1", last)
	}
	if len(replayed) != 2 || replayed[1].Taken != 1 {
		t.Errorf("replay choices = %v, want the entry order and the dispatch order taking the second", replayed)
	}
}
