package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Repeated -action flags run one performance each in one process; a dynamics
// action ends at the same integrated values every time, after itself or after another.
func TestRunActionRepeatedPerformsAfresh(t *testing.T) {
	binary := buildCLI(t)
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "exec", "runtime", "testdata", "conformance", "state_space_performed_after_another.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	model := string(src)
	const fell = "stateSpace = ⟨55.855000000000004, -29.429999999999996⟩"

	got := check(t, binary, model, "-action", "test::fall", "-action", "test::fall")
	wantReport(t, got, 0, fell, "time = 3.0 [s]")
	if n := strings.Count(got.output(), fell); n != 2 {
		t.Errorf("fall ended at its integrated state %d times, want 2:\n%s", n, got.output())
	}

	got = check(t, binary, model, "-action", "test::drop", "-action", "test::fall")
	wantReport(t, got, 0, "time = 2.0 [s]", fell, "time = 3.0 [s]")
	if strings.Contains(got.output(), "stateSpace = ⟨100.0, 0.0⟩") {
		t.Errorf("fall after drop ended at its bound initial state:\n%s", got.output())
	}
}
