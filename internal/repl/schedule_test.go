package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// %schedule shows the policy, reverse until one is set, and rejects a spelling
// that names no policy without changing the one in force.
func TestScheduleShowsAndSetsThePolicy(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	wants(t, run(t, s, "%schedule"), "schedule: reverse")
	wants(t, run(t, s, "%schedule declared"), "schedule: declared")
	wants(t, run(t, s, "%schedule seed:7"), "schedule: seed:7")
	if got := s.Schedule().String(); got != "seed:7" {
		t.Errorf("Schedule() = %s, want seed:7", got)
	}

	for _, bad := range []string{"random", "seed:", "seed:-1", "seed:abc"} {
		out := run(t, s, "%schedule "+bad)
		wants(t, out, "error: invalid scheduling policy \""+bad+"\"")
		rejects(t, out, "schedule: ")
	}
	wants(t, run(t, s, "%schedule"), "schedule: seed:7")
}

// A policy set before a debugging session starts is the one its steps take:
// under declared the fork's left branch steps first, where reverse steps right.
func TestScheduleAppliesToTheNextDebuggingSession(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%trace on")
	run(t, s, "%schedule declared")
	run(t, s, "%action tally")
	run(t, s, "%step")
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "choice step 3: tokens 2@left, 3@right (unordered; took 2@left first)")
}

// A debugging session under way keeps the policy it started with; the one set
// meanwhile reaches the session started after it.
func TestScheduleLeavesARunningDebuggerOnItsPolicy(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%trace on")
	run(t, s, "%action tally")
	run(t, s, "%step")
	run(t, s, "%schedule declared")
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "took 3@right first")
	run(t, s, "%continue")

	run(t, s, "%action tally")
	run(t, s, "%step")
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "took 2@left first")
}

// The policy set on the session reaches a runtime context created later and
// the one already in place.
func TestSetScheduleReachesTheRuntimeContext(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	s.SetSchedule(mustSchedule(t, "seed:3"))
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if got := ctx.Schedule().String(); got != "seed:3" {
		t.Errorf("new context: schedule = %s, want seed:3", got)
	}
	s.SetSchedule(mustSchedule(t, "declared"))
	if got := ctx.Schedule().String(); got != "declared" {
		t.Errorf("existing context: schedule = %s, want declared", got)
	}
}

func mustSchedule(t *testing.T, spelling string) runtime.SchedulePolicy {
	t.Helper()
	policy, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}
