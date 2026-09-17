package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestTerminatedMachineHoldsNoPendingWork: a transition to a terminate action
// nested in a composite state leaves the other region's timer, the event it still
// defers and the change condition it watched with nothing to act on, so the ended
// machine holds none of them.
func TestTerminatedMachineHoldsNoPendingWork(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import SI::*;
		private import Time::*;
		attribute def Abort;
		attribute def Later;
		state def Machine {
			attribute level : Integer = 0;
			entry; then busy;
			state busy parallel {
				state r1 {
					entry; then a;
					state a { defer Later; }
					transition first a accept Abort then stop;
				}
				state r2 {
					entry; then b;
					state b { defer Later; }
					transition first b accept after 10 [s] then c;
					transition first b accept when level > 0 then c;
					state c;
				}
				action stop terminate;
			}
		}
	}`))
	exec, err := ctx.CreateStateExecutor(findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState))
	if err != nil {
		t.Fatal(err)
	}
	exec.SendSignal("Later", nil)
	exec.SendSignal("Abort", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch Later: %v", err)
	}
	if got := exec.DeferredEvents(); len(got) != 1 {
		t.Fatalf("deferred = %v; want the Later event held by a", got)
	}
	if fired, err := exec.PollChangeEvents(); err != nil || fired {
		t.Fatalf("poll = %v, %v; want nothing fired with level at 0", fired, err)
	}
	if got := exec.ChangeWaits(); len(got) != 1 {
		t.Fatalf("ChangeWaits() = %v; want the level condition b waits on", got)
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch Abort: %v", err)
	}
	if exec.State() != StateTerminated {
		t.Fatalf("state = %v; want Terminated", exec.State())
	}
	if exec.HasPendingWork() {
		t.Errorf("HasPendingWork() = true on a terminated machine")
	}
	if got := exec.EventQueue().Events(); len(got) != 0 {
		t.Errorf("queue = %v after termination; want empty", got)
	}
	if got := exec.DeferredEvents(); len(got) != 0 {
		t.Errorf("deferred = %v after termination; want none", got)
	}
	if _, waiting := exec.NextWait(); waiting {
		t.Errorf("NextWait() reports a timer on a terminated machine")
	}
	if got := exec.ChangeWaits(); len(got) != 0 {
		t.Errorf("ChangeWaits() = %v after termination; want none", got)
	}
}

// TestChangeTriggeredTerminationHoldsNoWaits: a change condition rising in one
// region routes to the terminate action while the other region's condition stays
// false; the poll that ended the machine publishes no wait for it.
func TestChangeTriggeredTerminationHoldsNoWaits(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute abort : Boolean = false;
			attribute level : Integer = 0;
			entry; then busy;
			state busy parallel {
				state r1 {
					entry; then a;
					state a;
					transition first a accept when abort then stop;
				}
				state r2 {
					entry; then b;
					state b;
					transition first b accept when level > 0 then c;
					state c;
				}
				action stop terminate;
			}
		}
	}`))
	exec, err := ctx.CreateStateExecutor(findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState))
	if err != nil {
		t.Fatal(err)
	}
	if fired, err := exec.PollChangeEvents(); err != nil || fired {
		t.Fatalf("poll = %v, %v; want nothing fired while both conditions are false", fired, err)
	}
	if got := exec.ChangeWaits(); len(got) != 2 {
		t.Fatalf("ChangeWaits() = %v; want both conditions", got)
	}
	exec.stateData["abort"] = boolValue(true)
	fired, err := exec.PollChangeEvents()
	if err != nil || !fired {
		t.Fatalf("poll = %v, %v; want the abort condition to fire", fired, err)
	}
	if exec.State() != StateTerminated {
		t.Fatalf("state = %v; want Terminated", exec.State())
	}
	if got := exec.ChangeWaits(); len(got) != 0 {
		t.Errorf("ChangeWaits() = %v after termination; want none", got)
	}
	if exec.HasPendingWork() {
		t.Errorf("HasPendingWork() = true on a terminated machine")
	}
}
