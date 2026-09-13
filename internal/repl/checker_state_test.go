package repl

import "testing"

// Under %engine check, %state searches every schedule of a machine as %action does
// an action's, the clock advanced until nothing is due; %advance searches the last
// checked invocation again up to that instant, and RunFor searches the behaviors
// named as one invocation on one clock.
func TestEngineCheckSearchesStateMachines(t *testing.T) {
	s := loadSource(t, exploreLampSource)
	run(t, s, "%engine check")

	out := run(t, s, "%state Shared::Lamp::glow Shared::Lamp")
	wants(t, out, "✓ State machine Shared::Lamp::glow: no violation, exhaustive (2 states, 1 moves, depth 1)",
		"outcome: finalState on; visits off, on")
	rejects(t, out, "Started state machine")
	wants(t, run(t, s, "%current"), "no active state machine session")

	wants(t, run(t, s, "%advance 1"), "✓ State machine Shared::Lamp::glow: no violation, exhaustive up to t=1.0 (1 states, 0 moves, depth 0)",
		"outcome: finalState off; visits off")
	wants(t, run(t, s, "%advance 3"), "✓ State machine Shared::Lamp::glow: no violation, exhaustive up to t=3.0 (2 states, 1 moves, depth 1)",
		"outcome: finalState on; visits off, on")

	peek := Behavior{Name: "Shared::Lamp::peek", Performer: []string{"Shared::Lamp"}}
	glow := Behavior{Name: "Shared::Lamp::glow", Performer: []string{"Shared::Lamp"}}
	verdicts := s.RunFor([]Behavior{peek}, []Behavior{glow}, 3)
	if len(verdicts) != 1 {
		t.Fatalf("verdicts = %+v, want one for the invocation", verdicts)
	}
	wantVerdict(t, verdicts[0], VerdictFails, "✗ Behaviors Shared::Lamp::peek, Shared::Lamp::glow: divergent up to t=3.0 (9 states, 8 moves, depth 5)",
		"divergent: Shared::Lamp::peek.saw ends as false or true",
		`Shared::Lamp::glow finalState = "on"; Shared::Lamp::glow visits = "off, on"; Shared::Lamp::peek.saw = false`,
		`Shared::Lamp::glow finalState = "on"; Shared::Lamp::glow visits = "off, on"; Shared::Lamp::peek.saw = true`)
	if verdicts[0].Subject != "Shared::Lamp::peek, Shared::Lamp::glow" {
		t.Errorf("subject = %q, want the behaviors in start order", verdicts[0].Subject)
	}
	wantVerdict(t, s.RunFor([]Behavior{peek}, []Behavior{glow}, 2)[0], VerdictHolds,
		"✓ Behaviors Shared::Lamp::peek, Shared::Lamp::glow: no violation, exhaustive up to t=2.0")

	// A behavior that does not resolve is reported by name and nothing is searched.
	verdicts = s.RunFor([]Behavior{{Name: "Shared::Lamp::nothing"}}, []Behavior{glow}, 3)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictUnresolved || verdicts[0].Subject != "Shared::Lamp::nothing" {
		t.Fatalf("verdicts = %+v, want one unresolved", verdicts)
	}

	// RunStateMachineFor is %state then %advance under the check engine.
	wantVerdict(t, s.RunStateMachineFor("Shared::Lamp::glow", 1, "Shared::Lamp"), VerdictHolds, "exhaustive up to t=1.0")
	wantVerdict(t, s.RunStateMachine("Shared::Lamp::glow", "Shared::Lamp"), VerdictHolds, "exhaustive (2 states, 1 moves, depth 1)")
}

// %advance under the check engine with nothing checked yet says what it would search.
func TestEngineCheckAdvanceNeedsACheckedInvocation(t *testing.T) {
	s := loadSource(t, exploreLampSource)
	run(t, s, "%engine check")
	wants(t, run(t, s, "%advance 3"), "error: no behavior checked yet; under %engine check, %action or %state searches every schedule of a behavior, and %advance <time> then searches it again up to that instant")
}
