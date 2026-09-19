package repl

import (
	"strings"
	"testing"
)

// Tracing reports the execution behind a command's answer, and reports each
// command's own steps only.
func TestTracingReportsExecutionPerCommand(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	wants(t, run(t, s, "%trace on"), "trace: on")
	run(t, s, "%instantiate Derived::Vehicle")

	got := run(t, s, "%features Derived::Vehicle")
	wants(t, got, "[trace] ", "eval feature mass", "doubled = 3000.0")

	if again := run(t, s, "%instances"); strings.Contains(again, "[trace] ") {
		t.Errorf("a later command replayed an earlier trace:\n%s", again)
	}
}

func TestTracingIsOffByDefaultAndSwitchable(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	wants(t, run(t, s, "%trace"), "trace: off")
	rejects(t, run(t, s, "%instantiate Derived::Vehicle"), "[trace] ")

	run(t, s, "%trace on")
	run(t, s, "%trace off")
	if s.Tracing() {
		t.Error("tracing stayed on")
	}
	rejects(t, run(t, s, "%features Derived::Vehicle"), "[trace] ")
	wants(t, run(t, s, "%trace loud"), `error: unknown trace setting "loud"`)
}

// Turning tracing on mid-session reaches the runtime context and the executor
// that already exist.
func TestTracingAppliesToAnExistingSession(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	run(t, s, "%trace on")

	wants(t, run(t, s, "%step"), "[trace] step 1:")
}

// A declaration typed during a debugging session drops the session's runtime
// context, but the executor evaluates in the one it was created with — turning
// tracing on has to reach that context too, or the values go unreported.
func TestTracingReachesADebuggerThatOutlivedItsContext(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	s.Submit("package Unrelated { part def Widget { attribute size = 1.0; } }")
	run(t, s, "%trace on")

	got := run(t, s, "%step") + run(t, s, "%step")
	wants(t, got, "[trace] step 1:", "eval ")
}
