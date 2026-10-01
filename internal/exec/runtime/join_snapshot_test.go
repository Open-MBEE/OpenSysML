package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestCloneNilJoinArrivalsIsWritable(t *testing.T) {
	arrived := cloneJoinArrivals(nil)
	if arrived == nil {
		t.Fatal("cloneJoinArrivals(nil) = nil, want an initialized map")
	}
	arrived[&ast.PseudostateNode{}] = nil
}

func TestSnapshotRestoresJoinArrivals(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def X;
		attribute def Y;
		state Machine {
			entry; then owner;
			state owner parallel {
				state left {
					entry; then a;
					state a;
					transition first a accept X then sync;
				}
				state right {
					entry; then b;
					state b;
					transition first b accept Y then sync;
				}
				join sync;
				transition first sync then done;
			}
			state done;
		}
	}`)
	exec.SendSignal("X", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to first arrival: %v", err)
	}
	if len(exec.joinArrived) != 1 {
		t.Fatalf("join arrivals = %v after X, want one", exec.joinArrived)
	}
	canonical := (&Invocation{States: []*StateExecutor{exec}}).canonicalState(nil).text
	if !strings.Contains(canonical, "arrived{sync: a}") {
		t.Fatalf("canonical state = %s, want deterministic join arrival spelling", canonical)
	}
	snapshot, err := exec.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snapshot.Release()

	exec.SendSignal("Y", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to join completion: %v", err)
	}
	if len(exec.joinArrived) != 0 || activeStateNames(exec) != "done" || exec.State() != StateSuspended {
		t.Fatalf("after Y: arrivals = %v, state = %s, configuration = %s; want no arrivals and done", exec.joinArrived, exec.State(), activeStateNames(exec))
	}

	snapshot.Restore()
	if len(exec.joinArrived) != 1 {
		t.Fatalf("join arrivals after restore = %v, want the saved arrival", exec.joinArrived)
	}
	exec.SendSignal("Y", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after restore: %v", err)
	}
	if activeStateNames(exec) != "done" || exec.State() != StateSuspended {
		t.Fatalf("state after restored arrival completed with Y = %s, configuration = %s; want done", exec.State(), activeStateNames(exec))
	}
}
