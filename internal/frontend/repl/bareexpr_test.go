package repl

import (
	"strings"
	"testing"
)

func TestLoopEvaluatesBareExpressions(t *testing.T) {
	script := []string{
		"2 + 3",
		"1/0",
		"part def Wheel;",
		"%list",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()
	wants(t, got, "= 5", "division by zero", "part def Wheel")
	rejects(t, got, "expected a namespace member")
}

// TestBareExpressionCarriesMaterializationFailureIntoStatus checks a feature
// value a bare expression could not materialize reaches the session status a
// piped run exits on, as %eval's does.
func TestBareExpressionCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	wants(t, run(t, s, "%instantiate Demo::R"), "Created instance")

	var out strings.Builder
	submit(&out, s, "bad")
	wants(t, out.String(), "error:", "multiplicity violation")
	if len(s.MaterializationFailures()) == 0 {
		t.Error("a failed bare evaluation of a feature value did not reach the session status")
	}
}

// TestBareExpressionKeepsDebugSession checks evaluating an expression at the
// prompt leaves an in-progress action debugging session running.
func TestBareExpressionKeepsDebugSession(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	wants(t, run(t, s, "%action tally"), "✓ Started action executor")

	var out strings.Builder
	submit(&out, s, "1 + 1")
	wants(t, out.String(), "= 2")

	if s.actionExec == nil {
		t.Fatal("the debugging session was cleared by a bare expression")
	}
	wants(t, run(t, s, "%step"), "✓ Step complete")
}
