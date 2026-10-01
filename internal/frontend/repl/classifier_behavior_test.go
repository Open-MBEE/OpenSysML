package repl

import (
	"strings"
	"testing"
)

// Materializing a part starts the machine its type exhibits, so %state binds the
// debugger to that object's machine rather than a detached run of the usage.
func TestStateDebugsTheMachineAnObjectExhibits(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	got := run(t, s, "%state Obj::Monitor")
	wants(t, got, "exhibited by object #", "modes", "Current state: idle")

	// The entry action of the state the machine settled in wrote the object's
	// own feature value.
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")
}

// %current, %events, %step and %advance drive the object's machine, which the
// machine's own timer left waiting when the object was materialized.
func TestObjectMachineDebuggingCommands(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	run(t, s, "%state Obj::Monitor")

	wants(t, run(t, s, "%current"), "idle")
	wants(t, run(t, s, "%events"), "1 events")
	wants(t, run(t, s, "%step"), "Event dispatched", "Current state: awake")
	wants(t, run(t, s, "%advance 5"), "time is now 15.0")
	wants(t, run(t, s, "%current"), "awake")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 11")
}

// A second %instantiate is a second object with its own machine, and says which
// object the name now denotes.
func TestSecondInstantiateIsAnotherObject(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))

	again := run(t, s, "%instantiate Obj::Monitor")
	wants(t, again, "now denotes this object", "object #"+first+" is displaced from that name", "behavior of its own", "stays reachable as #"+first)
	if objectIDIn(t, run(t, s, "%features Obj::Monitor")) == first {
		t.Errorf("second %%instantiate reused object #%s:\n%s", first, again)
	}
}

// A machine parked on a change condition is stepped again once something else
// makes the condition true: %step polls the watched conditions rather than
// reporting the machine as suspended forever.
func TestStepDispatchesAConditionMadeTrueElsewhere(t *testing.T) {
	s := loadFixture(t, "testdata/change_condition_object.sysml")
	run(t, s, "%instantiate Watch::Sensor")
	run(t, s, "%state Watch::Sensor")

	wants(t, run(t, s, "%current"), "idle")
	wants(t, run(t, s, "%step"), "waiting on change condition")

	run(t, s, "%invoke Watch::Sensor trip")
	wants(t, run(t, s, "%step"), "Change event dispatched", "Current state: alerted")
}

// An operation of the object's type runs with the object as its performer, so
// what it writes is that object's feature value.
func TestInvokeRunsAnOperationOnTheObject(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy n=4"), "Invoked bumpBy on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 5")
}

// A positional argument list binds the operation's input parameters in declaration
// order, an expression with spaces included.
func TestInvokeBindsPositionalArguments(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy 4"), "Invoked bumpBy on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 5")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy 2 + 3"), "Invoked bumpBy on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 10")
}

// A string literal argument reaches the operation as written, its quotes and the
// spaces inside it included, positionally and by name.
func TestInvokeKeepsStringLiteralArguments(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, `%invoke Obj::Monitor setLabel "ready now"`), "Invoked setLabel on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), `label = "ready now"`)
	wants(t, run(t, s, `%invoke Obj::Monitor setLabel text="by name"`), "Invoked setLabel on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), `label = "by name"`)
	wants(t, run(t, s, `%invoke Obj::Monitor setLabel ""`), "Invoked setLabel on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), `label = ""`)
	wants(t, run(t, s, `%invoke Obj::Monitor setLabel "a" "b"`), "error:", "takes 1 input parameter(s), got 2 argument(s)")
}

// %invoke reports its usage, an operation the type does not own, an argument
// naming no parameter, a parameter left unbound, a list mixing the positional and
// the named form, a surplus positional argument and one bound twice.
func TestInvokeReportsItsFailureModes(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, "%invoke Obj::Monitor"), "usage: %invoke")
	wants(t, run(t, s, "%invoke Obj::Monitor missing"), "error:", "missing")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy"), "error:", "unbound parameter")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy other=1"), "error:", "unbound parameter")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy n"), "error:", "unresolved reference")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy 1 n=2"), "error:", "positional and named arguments mixed")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy 1 2"), "error:", "takes 1 input parameter(s), got 2 argument(s)")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy n=1 n=2"), "error:", "parameter n is given more than one argument")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")
}

// An unrelated declaration submitted while an object's machine is being debugged
// keeps the object and its identity, and restarts its machine from the initial
// state with a reported reason: an execution belongs to the analysis it started
// in, so it is never resumed on the values the discarded run left behind.
func TestObjectMachineRestartsOverAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	started := run(t, s, "%state Obj::Monitor")
	id := objectIDIn(t, started)
	// Drive the machine on, so a resumed execution would be visible as `awake`
	// and `count = 11` rather than the initial state below.
	run(t, s, "%step")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 11")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	wants(t, strings.Join(res.Notices, "\n"), "restarted from its initial state")

	wants(t, run(t, s, "%current"), "idle")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1", "ID: "+id)
}

// A restarted machine runs in the context the submission built, so the debugger
// drives its queue there rather than the discarded one.
func TestRestartedMachineRunsInTheNewContext(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	run(t, s, "%state Obj::Monitor")

	if res := s.Submit("package Other { part def Unrelated; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	// The restarted machine's own timer is queued in the new context, so the
	// debugger drives it there instead of reporting nothing to do.
	wants(t, run(t, s, "%events"), "1 events")
	wants(t, run(t, s, "%step"), "Event dispatched", "Current state: awake")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 11")
}

// Re-declaring the machine an object exhibits drops that object and says so: a
// rewritten body never resumes on values the old one wrote.
func TestRewritingTheExhibitedMachineDropsTheObject(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")

	rewritten := `
		package Obj {
			part def Monitor {
				attribute count = 0;
				exhibit state modes {
					entry; then idle;
					state idle { entry action bump { assign count := count + 5; } }
				}
			}
		}
	`
	res := s.Submit(rewritten)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("rewritten declaration has diagnostics: %v", res.Diagnostics)
	}
	wants(t, strings.Join(res.Notices, "\n"), "dropped")
	wants(t, run(t, s, "%features Obj::Monitor"), "no instance")

	run(t, s, "%instantiate Obj::Monitor")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 5")
}

// A machine is not something an object exhibits, so materializing the machine
// itself leaves it debuggable as a machine another object performs.
func TestStateDebugsAMachineMaterializedByName(t *testing.T) {
	s := loadFixture(t, "../../exec/runtime/testdata/conformance/variant_connection_per_owner.sysml")
	run(t, s, "%instantiate VariantRouting::alpha")
	run(t, s, "%instantiate VariantRouting::Router::Route")

	wants(t, run(t, s, "%state VariantRouting::Router::Route VariantRouting::alpha"), "Started state machine executor")
	wants(t, run(t, s, "%advance 1"), "Current state: arrived")
}

// A session over a machine an object merely performs stays on that machine: only
// a session over the object's own exhibited machine follows a restart.
func TestStateOverAPerformedMachineStaysOnIt(t *testing.T) {
	s := loadFixture(t, "testdata/performed_machine.sysml")
	run(t, s, "%instantiate Two::g")
	wants(t, run(t, s, "%state Two::Check Two::g"), "Started state machine executor")

	if res := s.Submit("package Other { part def Unrelated; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}

	wants(t, run(t, s, "%current"), "checking")
	wants(t, run(t, s, "%advance 5"), "Current state: checked")
}

// A part exhibiting no machine is reported as such rather than debugged.
func TestStateReportsAnObjectExhibitingNoMachine(t *testing.T) {
	s := loadFixture(t, "../../exec/runtime/testdata/conformance/variant_connection_per_owner.sysml")
	run(t, s, "%instantiate VariantRouting::alpha")

	wants(t, run(t, s, "%state VariantRouting::alpha"), "exhibits no state machine")
}

// objectIDIn reports the object identity a command's output names, written either
// as `#<n>` or as `ID: <n>`.
func objectIDIn(t *testing.T, out string) string {
	t.Helper()
	at, width := strings.Index(out, "#"), 1
	if named := strings.Index(out, "ID: "); named >= 0 && (at < 0 || named < at) {
		at, width = named, len("ID: ")
	}
	if at < 0 {
		t.Fatalf("output names no object identity:\n%s", out)
	}
	id := ""
	for _, r := range out[at+width:] {
		if r < '0' || r > '9' {
			break
		}
		id += string(r)
	}
	if id == "" {
		t.Fatalf("output names no object identity:\n%s", out)
	}
	return id
}
