package runtime

import "testing"

func TestRuntimeRobustnessInactiveRegion(t *testing.T) {
	t.Run("empty regions complete without activating inner states", func(t *testing.T) {
		_, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then working;
			state working parallel {
				state left { state idle; }
				state right { state waiting; }
			}
			state finished;
			transition first working then finished;
		}
	}`)
		if err != nil {
			t.Fatalf("execute state machine: %v", err)
		}
		finished := false
		for _, visit := range visits {
			switch visit {
			case "idle", "waiting":
				t.Errorf("no-entry region state %s was activated", visit)
			case "finished":
				finished = true
			}
		}
		if !finished {
			t.Fatal("working did not complete after both regions were left inactive")
		}
	})

	t.Run("stand-in do action waits for a signal before owner completion", func(t *testing.T) {
		_, visits, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				entry; then working;
				state working parallel {
					state standIn {
						do action wait {
							first start;
							then action receive accept Never;
							then done;
						}
						state inner;
					}
				}
				state finished;
				transition first working then finished;
			}
			attribute def Never;
		}`)
		if err != nil {
			t.Fatalf("execute state machine: %v", err)
		}
		for _, visit := range visits {
			switch visit {
			case "inner":
				t.Error("unstarted inner state was activated")
			case "finished":
				t.Error("owner completed while its stand-in do action awaited a signal")
			}
		}
	})
}
