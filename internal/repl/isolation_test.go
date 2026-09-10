package repl

import (
	"errors"
	"slices"
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

// nestedCaseSource declares a case under a part nested in a part def, so the
// object owning it is reached through a held object's feature values.
const nestedCaseSource = `package Nested {
	private import ScalarValues::*;
	part def Vehicle {
		part subsystem : Subsystem {
			analysis check : Rated { subject s = sensor; }
			part sensor : Sensor;
		}
	}
	part def Subsystem;
	part def Sensor { attribute rating : Real = 3.0; }
	analysis def Rated { subject s : Sensor; out r : Real = s.rating; }
}`

const nestedCase = "Nested::Vehicle::subsystem::check"

// An explored case nested in a type finds the object owning it while the
// session's state is held; a run then instantiates the owner in its own context,
// reading nothing of the session, so the held objects stay as the plan left them.
func TestExploredNestedCaseOwnerIsPlannedBeforeRelease(t *testing.T) {
	s := loadSource(t, nestedCaseSource)
	run(t, s, "%instantiate Nested::Vehicle")
	defer s.enter()()
	inv, err := splitAnalysisArgs(nestedCase)
	if err != nil {
		t.Fatal(err)
	}
	sym, fqn, err := s.analysisSymbol(inv)
	if err != nil {
		t.Fatal(err)
	}
	plan := s.planFresh()
	s.planOwner(plan, sym, fqn)
	if _, ok := plan.owners[fqn]; !ok {
		t.Fatalf("the plan did not find the held owner of %s", fqn)
	}
	held := s.heldIDs()

	sem, resolver, err := s.semanticModel()
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := s.newRuntimeOver(sem, resolver)
	if err != nil {
		t.Fatal(err)
	}
	objects := plan.bind(ctx)
	self, name := objects.owner(fqn)
	if self == nil || name != "Nested::Vehicle::subsystem" {
		t.Fatalf("owner = %v %q, want an object of Nested::Vehicle::subsystem", self, name)
	}
	if inSession, _ := s.rtCtx.Instance(self.ID); inSession == self {
		t.Error("the owner was instantiated in the session's runtime, not the run's")
	}
	var unplanned *UnplannedObjectError
	if _, _, err := objects.object("Nested::Vehicle"); !errors.As(err, &unplanned) {
		t.Errorf("an object the plan did not resolve = %v, want UnplannedObjectError", err)
	}
	if got := s.heldIDs(); !slices.Equal(got, held) {
		t.Errorf("the run changed the session's held objects: %v, was %v", got, held)
	}
}

// Explorations of a case nested in a held object's part, beside completion of
// that object's features on another goroutine, answer as they do alone.
func TestNestedCaseExplorationsAnswerAlikeBesideCompletion(t *testing.T) {
	s := loadSource(t, nestedCaseSource)
	run(t, s, "%instantiate Nested::Vehicle")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	want := strings.Join(s.RunAnalysis(nestedCase).Lines, "\n")
	if !strings.Contains(want, "r = 3.0") || !strings.Contains(want, "complete (1 runs)") {
		t.Fatalf("exploration alone:\n%s", want)
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 4; i++ {
			if got := strings.Join(s.RunAnalysis(nestedCase).Lines, "\n"); got != want {
				t.Errorf("exploration %d beside completion:\n%s\nwant\n%s", i, got, want)
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
			if c := s.Complete("%features #1.", len("%features #1.")); !slices.Contains(c.Candidates, "#1.subsystem") {
				t.Errorf("Complete beside an exploration = %v", c.Candidates)
			}
		}
	}()
	wg.Wait()
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
