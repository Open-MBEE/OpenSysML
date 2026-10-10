package runtime

import (
	"slices"
	"testing"
)

func TestRuntimeRobustnessCompositeWithoutEntry(t *testing.T) {
	_, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then Work;
			state Work {
				state W1;
			}
			state X;
			transition first Work then X;
		}
	}`)
	if err != nil {
		t.Fatalf("execute composite without an initial substate: %v", err)
	}
	if !slices.Contains(visits, "Work") {
		t.Errorf("Work was not entered: %v", visits)
	}
	if slices.Contains(visits, "W1") {
		t.Errorf("unstarted substate W1 was entered: %v", visits)
	}
	if slices.Contains(visits, "X") {
		t.Errorf("Work completed without an active substate: %v", visits)
	}
}
