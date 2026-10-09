package runtime

import "testing"

func TestRuntimeRobustnessInactiveRegion(t *testing.T) {
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
	for _, visit := range visits {
		switch visit {
		case "idle", "waiting":
			t.Errorf("no-entry region state %s was activated", visit)
		case "finished":
			t.Fatal("working completed while both regions were inactive")
		}
	}
}
