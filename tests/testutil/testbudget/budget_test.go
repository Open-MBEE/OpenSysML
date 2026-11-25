package testbudget

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// recordingFataler records Fatalf without the testing package, so the helper's
// error path can be exercised directly.
type recordingFataler struct{ message string }

func (f *recordingFataler) Fatalf(format string, args ...any) {
	f.message = fmt.Sprintf(format, args...)
}

func TestDefaultLeavesTheDeadlineUnchanged(t *testing.T) {
	t.Setenv(EnvName, "")
	scale, err := Scale()
	if err != nil || scale != 1 {
		t.Fatalf("Scale() = %v, %v; want 1, nil", scale, err)
	}
	got, err := Apply(10 * time.Second)
	if err != nil || got != 10*time.Second {
		t.Fatalf("Apply(10s) = %v, %v; want 10s, nil", got, err)
	}
	if got := Deadline(t, 3*time.Second); got != 3*time.Second {
		t.Errorf("Deadline(3s) = %v, want 3s", got)
	}
	if got, err := Parse("  "); err != nil || got != 1 {
		t.Errorf("Parse(whitespace) = %v, %v; want 1, nil", got, err)
	}
}

func TestScaleIsAppliedToTheDeadline(t *testing.T) {
	t.Setenv(EnvName, "4")
	deadline := Deadline(t, 25*time.Millisecond)
	if want := 100 * time.Millisecond; deadline != want {
		t.Fatalf("Deadline(25ms) = %v, want %v", deadline, want)
	}
	// A real deadline, not only the arithmetic: work taking 60ms misses the
	// declared 25ms but fits the 100ms the scale buys.
	done := make(chan struct{})
	go func() {
		time.Sleep(60 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(deadline):
		t.Fatal("scaled deadline rejected work that fits it")
	}
}

func TestInvalidScaleFailsAtThePointOfUse(t *testing.T) {
	for _, value := range []string{"abc", "0", "-2", "NaN", "+Inf", "1001"} {
		t.Run(value, func(t *testing.T) {
			if _, err := Parse(value); err == nil {
				t.Fatalf("Parse(%q) accepted an invalid scale", value)
			}
			t.Setenv(EnvName, value)
			if _, err := Apply(time.Second); err == nil {
				t.Fatal("Apply accepted an invalid scale")
			}
			f := &recordingFataler{}
			Deadline(f, time.Second)
			if !strings.Contains(f.message, EnvName) {
				t.Errorf("Deadline reported %q, want the variable named", f.message)
			}
		})
	}
}

func TestOverflowIsRejected(t *testing.T) {
	t.Setenv(EnvName, "1000")
	if _, err := Apply(time.Duration(1) << 62); err == nil {
		t.Fatal("Apply accepted an overflowing scale")
	}
}

// TestBudgetOverrunProbe is run as a child process by
// TestOverBudgetStillFailsAtDefaultScale. It uses the same deadline pattern as
// the tests whose budgets the scale stretches, and must fail when the work
// exceeds the deadline at the default scale.
func TestBudgetOverrunProbe(t *testing.T) {
	if os.Getenv("OPENSYSML_TEST_BUDGET_PROBE") == "" {
		t.Skip("only run as the overrun probe's child process")
	}
	deadline := Deadline(t, time.Millisecond)
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Second)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(deadline):
		t.Fatalf("over-budget probe did not finish in %v", deadline)
	}
}

func TestOverBudgetStillFailsAtDefaultScale(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestBudgetOverrunProbe$", "-test.v")
	cmd.Env = append(os.Environ(), "OPENSYSML_TEST_BUDGET_PROBE=1", EnvName+"=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the over-budget probe passed; its output:\n%s", out)
	}
	if !strings.Contains(string(out), "over-budget probe did not finish") {
		t.Fatalf("the probe failed for the wrong reason:\n%s", out)
	}
}
