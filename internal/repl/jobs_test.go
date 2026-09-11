package repl

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
)

// %jobs shows the jobs, one per CPU until set, and rejects anything but a positive
// integer without changing the count in force.
func TestJobsShowsAndSetsTheCount(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	wants(t, run(t, s, "%jobs"), fmt.Sprintf("jobs: %d", analysis.DefaultJobs()))
	wants(t, run(t, s, "%jobs 3"), "jobs: 3")
	if got := s.Jobs(); got != 3 {
		t.Errorf("Jobs() = %d, want 3", got)
	}
	for _, bad := range []string{"0", "-1", "many", "2.5"} {
		out := run(t, s, "%jobs "+bad)
		wants(t, out, "error: %jobs=\""+bad+"\" is not a positive integer")
		rejects(t, out, "jobs: ")
	}
	wants(t, run(t, s, "%jobs"), "jobs: 3")
}

// SetJobs reaches the budget every question is asked under and, unlike a bound on a
// run, leaves the held context and a debugger under way alone.
func TestSetJobsReachesTheBudgetAndKeepsTheSession(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%action tally")
	run(t, s, "%step")
	if err := s.SetJobs(5); err != nil {
		t.Fatal(err)
	}
	if got := s.budgetFor(s.schedule, analysis.Holds).Jobs; got != 5 {
		t.Errorf("budgetFor Jobs = %d, want 5", got)
	}
	if s.actionExec == nil {
		t.Fatal("SetJobs ended the action debugging session")
	}
	wants(t, run(t, s, "%step"), "Step complete")
	err := s.SetJobs(0)
	var typed *analysis.JobsError
	if !errors.As(err, &typed) || !errors.Is(err, analysis.ErrJobs) {
		t.Fatalf("SetJobs(0) = %v, want a JobsError", err)
	}
	if got := s.Jobs(); got != 5 {
		t.Errorf("a rejected SetJobs left Jobs() = %d, want 5", got)
	}
}
