package repl

import (
	"errors"
	"slices"
	"testing"
)

func TestRunCommandFailsAUsageAnswer(t *testing.T) {
	s := NewSession()
	for _, line := range []string{"%save", "%validate a b", "%render-run nope"} {
		printed, _, err := s.RunMeta(line)
		if err != nil {
			t.Fatalf("RunMeta(%q) = %v", line, err)
		}
		out, _, err := s.RunCommand(line)
		var usage *UsageError
		if !errors.As(err, &usage) || len(out) != 0 || !slices.Equal(usage.Lines, printed) {
			t.Errorf("RunCommand(%q) = %q, %v; want a UsageError holding %q", line, out, err, printed)
		}
	}
	if out, _, err := s.RunCommand("%verbosity"); err != nil || len(out) != 1 {
		t.Errorf("RunCommand(%%verbosity) = %q, %v", out, err)
	}
}
