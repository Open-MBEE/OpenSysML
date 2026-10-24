package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every performance of a dynamics action in one session ends at the same
// integrated values, whichever performances the session ran before it.
func TestActionPerformedAgainAnswersAlike(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "exec", "runtime", "testdata", "conformance", "state_space_performed_after_another.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	s := loadSource(t, string(src))
	first := strings.Join(s.RunAction("test::fall").Lines, "\n")
	if !strings.Contains(first, "stateSpace = ⟨55.855000000000004, -29.429999999999996⟩") || !strings.Contains(first, "time = 3.0 [s]") {
		t.Fatalf("first performance:\n%s", first)
	}
	if again := strings.Join(s.RunAction("test::fall").Lines, "\n"); again != first {
		t.Errorf("second performance answered differently:\n%s\nfirst:\n%s", again, first)
	}
	if drop := strings.Join(s.RunAction("test::drop").Lines, "\n"); !strings.Contains(drop, "time = 2.0 [s]") {
		t.Fatalf("drop:\n%s", drop)
	}
	if after := strings.Join(s.RunAction("test::fall").Lines, "\n"); after != first {
		t.Errorf("performance after drop answered differently:\n%s\nfirst:\n%s", after, first)
	}
}
