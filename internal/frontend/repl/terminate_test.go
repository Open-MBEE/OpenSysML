package repl

import "testing"

// A transition to a terminate action ends the machine short of a final state: the
// debugger says so rather than "completed", no state is active, `%current` keeps the
// values the run had, and a signal sent afterwards finds no machine to take it.
func TestStateDebuggerReportsATerminatedMachine(t *testing.T) {
	s := loadFixture(t, "../../exec/runtime/testdata/conformance/state_terminate_transition_ends_machine.sysml")
	wants(t, run(t, s, "%state test::Machine"), "Current state: busy")

	wants(t, run(t, s, "%send Abort"), `Accepted by state machine "Machine" in state busy`)
	wants(t, run(t, s, "%advance 1"), "Current state: <none>", stateTerminatedText)
	wants(t, run(t, s, "%current"), "Current state: <none>", "Execution state: Terminated", "count = 1", "exits = 1", "effects = 7")
	wants(t, run(t, s, "%step"), stateTerminatedText)
	wants(t, run(t, s, "%send Abort"), `accepts no signal Abort now: state machine "Machine" terminated`)
}

// terminatingPart is a part whose machine ends the part itself on Kill, while an
// action it performs waits for a signal that never comes.
const terminatingPart = `package test {
	private import ScalarValues::*;
	attribute def Kill;
	attribute def Go;
	part def Probe {
		attribute seen : Integer = 0;
		exhibit state life {
			entry; then alive;
			state alive;
			transition first alive accept Kill do action { terminate this; } then alive;
		}
		perform action work {
			first start;
			then action wait accept Go;
			then action mark assign seen := 1;
			then done;
		}
	}
	part p : Probe;
}`

// `terminate this` in a part's behavior ends the part: every behavior it performs is
// over, the action debugger stepping one of them reports it terminated rather than
// waiting on, and the ended part performs nothing further.
func TestTerminatingThisEndsThePartsBehaviors(t *testing.T) {
	s := NewSession()
	submits(t, s, terminatingPart)
	wants(t, run(t, s, "%instantiate test::p"), "Created instance")
	wants(t, run(t, s, "%action work test::p"), `Started action executor for "work"`)
	wants(t, run(t, s, "%step"), "State: Running")
	wants(t, run(t, s, "%step"), "State: Waiting")

	wants(t, run(t, s, "%send Kill to test::p"), `Accepted by state machine "life" in state alive`)
	wants(t, run(t, s, "%advance 1"), "1 event(s) processed")
	wants(t, run(t, s, "%step"), "State: Terminated", "Tokens: 0", actionTerminatedText)
	wants(t, run(t, s, "%continue"), "Action already terminated")
	wants(t, run(t, s, "%features test::p"), "seen = 0",
		"life: exhibited state machine, terminated", "work: performed action, terminated")
	wants(t, run(t, s, "%instances"), "test::p (ID: 1, ended)")
	wants(t, run(t, s, "%state life test::p"), "Current state: <none>")
	wants(t, run(t, s, "%current"), "Execution state: Terminated")
	wants(t, run(t, s, "%send Kill to test::p"), `accepts no signal Kill now: state machine "life" terminated`)
	wants(t, run(t, s, "%action work test::p"), "performer of the behavior: occurrence lifetime does not admit the change", "ended at")
}
