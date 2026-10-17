package runtime

import "testing"

// KerML StateTransitionPerformance orders `guard then transitionLinkSource.exit`
// for every state transition, so a transition back to its own simple state exits
// and re-enters it: exit action, effect, entry action, in that order.
func TestSimpleSelfTransitionExitsAndReEntersItsState(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			entry; then start;
			state start;
			state s {
				entry { assign log := log * 10 + 1; }
				exit { assign log := log * 10 + 2; }
			}

			succession first start then s;
			transition first s accept again do assign log := log * 10 + 9 then s;
		}
	}`)

	exec.SendSignal("again", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "s")
	// 1 (entry), 2 (exit), 9 (effect), 1 (entry again).
	if got := exec.StateData()["log"]; got.Const.Int != 1291 {
		t.Errorf("self-transition log is %v, want 1291 (entry, exit, effect, entry)", got.Const.Int)
	}
	assertVisits(t, exec.stateVisits, "start", "s", "s")
}

// The same rule inside an orthogonal region: the self-transitioning state is
// exited and re-entered while its sibling region keeps its own active state.
func TestSimpleSelfTransitionInARegionLeavesSiblingRegionsAlone(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm parallel {
			attribute log : Integer = 0;
			attribute other : Integer = 0;

			state left {
				entry; then lstart;
				state lstart;
				state l1 {
					entry { assign log := log * 10 + 1; }
					exit { assign log := log * 10 + 2; }
				}
				succession first lstart then l1;
				transition first l1 accept again do assign log := log * 10 + 9 then l1;
			}

			state right {
				entry; then rstart;
				state rstart;
				state r1 {
					entry { assign other := other * 10 + 1; }
					exit { assign other := other * 10 + 2; }
				}
				succession first rstart then r1;
			}
		}
	}`)

	exec.SendSignal("again", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := exec.StateData()["log"]; got.Const.Int != 1291 {
		t.Errorf("self-transition log is %v, want 1291 (entry, exit, effect, entry)", got.Const.Int)
	}
	if got := exec.StateData()["other"]; got.Const.Int != 1 {
		t.Errorf("the sibling region was disturbed, its log is %v, want 1", got.Const.Int)
	}
}
