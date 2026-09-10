package repl

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// isolationSource is exploreRaceSource with a calculation, whose invocation the
// session parses through its argument memo.
const isolationSource = exploreRaceSource + `
package Calcs {
	private import ScalarValues::*;
	calc def Twice { in x : Integer; return : Integer = x * 2; }
}
`

// An exploration runs on a worker and contexts of its own, so the session's
// readers answer while it runs, and a second command still waits for it.
func TestExploreReleasesTheSessionToReadersOnly(t *testing.T) {
	s := loadSource(t, isolationSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	const wait = 5 * time.Second
	var (
		readers   = make(chan Completion, 1)
		command   = make(chan []string, 1)
		commanded time.Time
	)
	release := s.enter()
	steady := s.browseIndex().LookupQualified("Race::steady")
	if len(steady) != 1 {
		t.Fatalf("Race::steady resolved to %d symbols", len(steady))
	}
	v := s.exploreVerdict("Race::steady", func(ctx *runtime.Context) (runtime.Outcome, error) {
		go func() { readers <- s.Complete("Race::", len("Race::")) }()
		go func() { command <- s.List() }()
		select {
		case c := <-readers:
			if !strings.Contains(strings.Join(c.Candidates, " "), "Race::race") {
				t.Errorf("Complete beside the plan offered %v, want Race::race", c.Candidates)
			}
		case <-time.After(wait):
			t.Fatal("Complete waited on the exploration: the session's state was not released")
		}
		if s.Budgets() != s.budgets || s.Schedule() != s.schedule || s.Tracing() {
			t.Error("the getters beside the plan do not read the session")
		}
		select {
		case <-command:
			t.Fatal("List ran beside the exploration: a second command must wait for the first")
		case <-time.After(100 * time.Millisecond):
		}
		commanded = time.Now()
		outputs, err := ctx.ExecuteAction(steady[0])
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	})
	release()
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	select {
	case lines := <-command:
		if len(lines) == 0 || commanded.IsZero() {
			t.Errorf("List after the exploration = %v", lines)
		}
	case <-time.After(wait):
		t.Fatal("List did not run once the exploration was over")
	}
}

// Explorations and completion requests interleaved on two goroutines answer as
// they do alone; the run is under -race.
func TestExplorationsAnswerAlikeBesideCompletion(t *testing.T) {
	s := loadSource(t, isolationSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	wantRace := strings.Join(s.RunAction("Race::race").Lines, "\n")
	wantCalc := strings.Join(s.RunCalc("Calcs::Twice(21)").Lines, "\n")
	if !strings.Contains(wantRace, "complete (6 runs)") || !strings.Contains(wantCalc, "42") {
		t.Fatalf("explorations alone:\n%s\n%s", wantRace, wantCalc)
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 4; i++ {
			if got := strings.Join(s.RunAction("Race::race").Lines, "\n"); got != wantRace {
				t.Errorf("exploration %d beside completion:\n%s\nwant\n%s", i, got, wantRace)
			}
			if got := strings.Join(s.RunCalc("Calcs::Twice(21)").Lines, "\n"); got != wantCalc {
				t.Errorf("calculation %d beside completion:\n%s\nwant\n%s", i, got, wantCalc)
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			if c := s.Complete("Race::r", len("Race::r")); len(c.Candidates) != 1 || c.Candidates[0] != "Race::race" {
				t.Errorf("Complete beside an exploration = %v", c.Candidates)
			}
			if c := s.Complete("Calcs::Tw", len("Calcs::Tw")); len(c.Candidates) != 1 {
				t.Errorf("Complete beside an exploration = %v", c.Candidates)
			}
			s.Budgets()
			s.Schedule()
			s.Verbosity()
			s.ConformanceMode()
		}
	}()
	wg.Wait()
}
