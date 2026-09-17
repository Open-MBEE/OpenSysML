package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestTerminatedMachineHoldsNoPendingWork: a transition to a terminate action
// nested in a composite state leaves the other region's timer and the event it
// still defers with nothing to dispatch on, so the ended machine holds neither.
func TestTerminatedMachineHoldsNoPendingWork(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import SI::*;
		private import Time::*;
		attribute def Abort;
		attribute def Later;
		state def Machine {
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
}
