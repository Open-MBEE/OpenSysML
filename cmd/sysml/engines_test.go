package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// engineModel states one condition that holds and one that a run refutes, so a
// verdict of each status has a standing to report.
const engineModel = `package Rover {
    constraint MassBudget { 180.0 <= 200.0 }
    constraint Overweight { 250.0 <= 200.0 }
}
`

// TestEnginesListsTheBuild checks that -engines tables every engine of the build
// with its authority, the questions it answers and its status, and exits without a model.
func TestEnginesListsTheBuild(t *testing.T) {
	binary := buildCLI(t)

	out := run(t, binary, "-engines")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "engine") || !strings.Contains(lines[0], "authority") ||
		!strings.Contains(lines[0], "answers") || !strings.Contains(lines[0], "status") {
		t.Fatalf("-engines did not table the engines:\n%s", out)
	}
	for i, want := range []string{"check", "explore", "run", "solve", "sweep"} {
		fields := strings.Fields(lines[i+1])
		if len(fields) < 4 || fields[0] != want {
			t.Errorf("line %d = %q, want engine %s", i+1, lines[i+1], want)
		}
	}
	for _, want := range []string{"bounded", "proved", "observed", "outcomes, holds", "evaluate", "satisfiable", "sweep"} {
		if !strings.Contains(out, want) {
			t.Errorf("-engines is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "ready") {
		t.Errorf("-engines reports no status:\n%s", out)
	}
}

// TestEngineRefusesAnUnknownName checks that a name no engine is registered under
// is a usage error at startup, naming the engines, before any model is read.
func TestEngineRefusesAnUnknownName(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, engineModel, "-engine", "bogus", "-constraint", "Rover::MassBudget")
	if got.status != 2 {
		t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
	}
	for _, want := range []string{`no engine named "bogus"`, "explore, run, solve, sweep", "auto", "all"} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("refusal is missing %q:\n%s", want, got.output())
		}
	}
	if strings.Contains(got.output(), "Constraint") {
		t.Errorf("a check ran under an engine that does not exist:\n%s", got.output())
	}
}

// TestVerdictsCarryTheirStanding checks that every verdict is followed by its
// standing: the claim, the strength earned and what earned it.
func TestVerdictsCarryTheirStanding(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, engineModel, "-constraint", "Rover::MassBudget", "-constraint", "Rover::Overweight"), 1,
		"✓ Constraint Rover::MassBudget passed\n  standing: holds (observed: 1 run under reverse)\n",
		"✗ Constraint Rover::Overweight failed\n  Assertion evaluated to false: 250.0 <= 200.0\n  standing: violated (witnessed: 1 run under reverse)\n")
	// The standing names the schedule the run was made under.
	wantReport(t, check(t, binary, engineModel, "-schedule", "declared", "-constraint", "Rover::MassBudget"), 0,
		"standing: holds (observed: 1 run under declared)")
}

// TestEngineNamedIsFinal checks the dispatch rule for a named engine: its refusal
// is the answer, and no other engine is tried in its place.
func TestEngineNamedIsFinal(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, engineModel, "-engine", "explore", "-constraint", "Rover::MassBudget")
	wantReport(t, got, 2, "? Constraint Rover::MassBudget could not be evaluated",
		"explore does not answer evaluate questions",
		"standing: not covered (explore refused: explore does not answer evaluate questions)")
	if strings.Contains(got.output(), "passed") {
		t.Errorf("another engine answered for the one named:\n%s", got.output())
	}

	// The engine that covers the question answers it alone, as auto would have.
	wantReport(t, check(t, binary, engineModel, "-engine", "run", "-constraint", "Rover::MassBudget"), 0,
		"✓ Constraint Rover::MassBudget passed", "standing: holds (observed: 1 run under reverse)")
	// auto is the default spelled out.
	wantReport(t, check(t, binary, engineModel, "-engine", "auto", "-constraint", "Rover::MassBudget"), 0,
		"standing: holds (observed: 1 run under reverse)")
}

// TestEngineAllListsEveryCoveringEngine checks that all puts the question to
// every engine covering it and the standing lists each engine's part.
func TestEngineAllListsEveryCoveringEngine(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, engineModel, "-engine", "all", "-constraint", "Rover::MassBudget"), 0,
		"✓ Constraint Rover::MassBudget passed",
		"standing: holds (observed: 1 run under reverse); all: run holds (observed)")
	wantReport(t, check(t, binary, engineModel, "-engine", "all", "-constraint", "Rover::Overweight"), 1,
		"standing: violated (witnessed: 1 run under reverse); all: run violated (witnessed)")
}

// TestEngineExploreIsScheduleExplore checks the synonym: -engine explore drives
// the same exploration -schedule explore does, and the two reports agree.
func TestEngineExploreIsScheduleExplore(t *testing.T) {
	binary := buildCLI(t)

	byEngine := check(t, binary, forkModel, "-engine", "explore", "-action", "Mission::race")
	bySchedule := check(t, binary, forkModel, "-schedule", "explore", "-action", "Mission::race")
	wantReport(t, byEngine, 0, "✓ explored Mission::race: 2 outcomes", "complete (2 runs)",
		"standing: outcomes (proved over schedules: 2 linearizations, inputs as written)")
	if byEngine.stdout != bySchedule.stdout || byEngine.status != bySchedule.status {
		t.Errorf("-engine explore and -schedule explore disagree:\n%s\n---\n%s", byEngine.output(), bySchedule.output())
	}
	// The budget spelled on -schedule still bounds the exploration named on -engine,
	// and reaching it lowers the standing from proved to observed, naming the bound.
	wantReport(t, check(t, binary, forkModel, "-engine", "explore", "-schedule", "explore:runs=1", "-action", "Mission::race"), 2,
		"runs budget", "standing: outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))")
}

// engineReport is the part of the JSON report -engine adds: the plan and one
// result per engine.
type engineReport struct {
	Status string `json:"status"`
	Checks []struct {
		Status string   `json:"status"`
		Lines  []string `json:"lines"`
		Plan   struct {
			Engine   string `json:"engine"`
			Standing string `json:"standing"`
			Steps    []struct {
				Engine string `json:"engine"`
				Status string `json:"status"`
				Detail string `json:"detail"`
			} `json:"steps"`
		} `json:"plan"`
		Results []struct {
			Engine   string `json:"engine"`
			Claim    string `json:"claim"`
			Strength string `json:"strength"`
			Bounds   []struct {
				Name    string `json:"name"`
				Limit   int64  `json:"limit"`
				Reached bool   `json:"reached"`
			} `json:"bounds"`
			Witness *struct {
				Schedule string   `json:"schedule"`
				Choices  []string `json:"choices"`
			} `json:"witness"`
			Standing string `json:"standing"`
		} `json:"results"`
	} `json:"checks"`
}

// TestJSONReportsThePlan checks that -json carries how a check was answered: the
// selection, each engine's step, and one result per engine with its engine,
// claim, strength, bounds and witness, beside the fields it always carried.
func TestJSONReportsThePlan(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, engineModel, "-json", "-engine", "all", "-constraint", "Rover::Overweight")
	var report engineReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 1 || report.Status != "fails" || len(report.Checks) != 1 {
		t.Fatalf("status = %d %q, checks = %d\n%s", got.status, report.Status, len(report.Checks), got.output())
	}
	c := report.Checks[0]
	if c.Plan.Engine != "all" || c.Plan.Standing != "violated (witnessed: 1 run under reverse); all: run violated (witnessed)" ||
		len(c.Plan.Steps) != 1 || c.Plan.Steps[0].Engine != "run" || c.Plan.Steps[0].Status != "answered" {
		t.Errorf("report does not carry the plan:\n%s", got.stdout)
	}
	if len(c.Results) != 1 || c.Results[0].Engine != "run" || c.Results[0].Claim != "violated" ||
		c.Results[0].Strength != "witnessed" || c.Results[0].Standing != "violated (witnessed: 1 run under reverse)" {
		t.Errorf("report does not carry the run engine's result:\n%s", got.stdout)
	}
	if len(c.Results) == 1 {
		// A violation is witnessed by the run that refuted it: the schedule it ran under and its choices.
		r := c.Results[0]
		if r.Bounds == nil || r.Witness == nil || r.Witness.Schedule != "reverse" || r.Witness.Choices == nil {
			t.Errorf("a violated result's bounds and witness are not reported:\n%s", got.stdout)
		}
		for _, b := range r.Bounds {
			if b.Reached || b.Limit <= 0 || b.Name == "" {
				t.Errorf("a run within its budget reports a reached bound: %+v", b)
			}
		}
	}
	if len(c.Lines) != 3 || !strings.HasPrefix(c.Lines[2], "  standing: violated") {
		t.Errorf("the lines do not end with the standing:\n%s", got.stdout)
	}

	// A refusal is a step with its reason and an empty results[], not a missing key.
	got = check(t, binary, engineModel, "-json", "-engine", "explore", "-constraint", "Rover::MassBudget")
	report = engineReport{}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 {
		t.Fatalf("checks = %d\n%s", len(report.Checks), got.output())
	}
	c = report.Checks[0]
	if c.Plan.Engine != "explore" || len(c.Plan.Steps) != 1 || c.Plan.Steps[0].Status != "refused" ||
		!strings.Contains(c.Plan.Steps[0].Detail, "explore does not answer evaluate questions") || len(c.Results) != 0 {
		t.Errorf("report does not carry the refusal:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "\"results\": []") {
		t.Errorf("a refused check does not carry an empty results[]:\n%s", got.stdout)
	}
}

// TestJSONReportsTheExploredPlan checks that an exploration reaching a budget
// reports the bound as reached and its strength lowered.
func TestJSONReportsTheExploredPlan(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-json", "-engine", "explore", "-schedule", "explore:runs=1", "-action", "Mission::race")
	var report engineReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 || len(report.Checks[0].Results) != 1 {
		t.Fatalf("report does not carry one result:\n%s", got.stdout)
	}
	r := report.Checks[0].Results[0]
	reached := false
	for _, b := range r.Bounds {
		if b.Name == "runs" && b.Limit == 1 && b.Reached {
			reached = true
		}
	}
	// The bound reached lowers the strength from proved to observed and is named.
	if r.Engine != "explore" || r.Claim != "outcomes" || r.Strength != "observed" || !reached ||
		r.Standing != "outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))" {
		t.Errorf("the reached runs budget is not reported:\n%s", got.stdout)
	}
}

// TestEnginesExitsWithoutAModel checks that -engines needs no model and answers
// alone, as -version does.
func TestEnginesExitsWithoutAModel(t *testing.T) {
	binary := buildCLI(t)

	out, err := exec.Command(binary, "-engines", "-json").CombinedOutput()
	if err != nil {
		t.Fatalf("-engines -json: %v\n%s", err, out)
	}
	if !strings.HasPrefix(string(out), "engine") {
		t.Errorf("-engines did not list the engines:\n%s", out)
	}
}
