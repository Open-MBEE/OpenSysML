package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tankModel performs an action on a part whose constraint one schedule of the
// action breaks, so a property of the performer is what the check engine judges.
const tankModel = `package Plant {
    private import ScalarValues::*;
    part def Tank {
        attribute level : Integer = 0;
        attribute capacity : Integer = 5;
        constraint low { level < 2 }
        constraint capped { level <= capacity }
        action fill {
            first start;
            fork split;
            action a { assign level := 1; }
            action b { assign level := 2; }
            join sync;
            done;
            succession first start then split;
            succession first split then a;
            succession first split then b;
            succession first a then sync;
            succession first b then sync;
            succession first sync then done;
        }
    }
    part tank : Tank;
}
`

// checkedReport is the JSON a check engine's result carries beside the plan.
type checkedReport struct {
	Checks []struct {
		Status  string `json:"status"`
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
			Check *struct {
				Verdict    string   `json:"verdict"`
				States     int      `json:"states"`
				Moves      int      `json:"moves"`
				Depth      int      `json:"depth"`
				BoundsHit  []string `json:"boundsHit"`
				Violations []struct {
					Kind    string   `json:"kind"`
					Name    string   `json:"name"`
					Error   string   `json:"error"`
					Depth   int      `json:"depth"`
					Witness []string `json:"witness"`
					Path    string   `json:"path"`
				} `json:"violations"`
				Divergent []struct {
					Feature string `json:"feature"`
					Values  []struct {
						Value   string   `json:"value"`
						Witness []string `json:"witness"`
						Path    string   `json:"path"`
					} `json:"values"`
				} `json:"divergent"`
				Outcomes []string `json:"outcomes"`
			} `json:"check"`
		} `json:"results"`
	} `json:"checks"`
}

// -engine check over -action searches every schedule: a feature two branches write
// is divergent, each value's witness in -check-witness replayable as -schedule replay:<file>.
func TestEngineCheckWitnessesADivergence(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-witness", dir)
	wantReport(t, got, 1,
		"✗ Action Mission::race: divergent (11 states, 10 moves, depth 6)",
		"divergent: x ends as 1 or 2",
		"x = 1 (witness "+filepath.Join(dir, "Mission.race.x-1.witness")+")",
		"x = 2 (witness "+filepath.Join(dir, "Mission.race.x-2.witness")+")",
		"outcome: x = 1", "outcome: x = 2",
		"standing: sensitive (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)")

	witness := filepath.Join(dir, "Mission.race.x-2.witness")
	content, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	// The file is the schedule's choices, a blank line, then the run's trace.
	if !strings.HasPrefix(string(content), "step 3: 2@left first of 2@left, 3@right\n\n") ||
		!strings.Contains(string(content), "choice step 3: tokens 2@left, 3@right (unordered; took 2@left first)") {
		t.Errorf("witness file:\n%s", content)
	}
	replayed := check(t, binary, forkModel, "-schedule", "replay:"+witness, "-trace", "-action", "Mission::race")
	wantReport(t, replayed, 0, "took 2@left first", "x = 2", "standing: value (observed: 1 run under replay:"+witness+")")

	// A feature the schedules agree on is not divergent: the search is exhaustive, and holds.
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-diverge", "y"),
		0, "✓ Action Mission::race: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: outcomes (bounded over schedules: 11 states, 10 moves searched)")
	rejects := check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-diverge", "y")
	rejectReport(t, rejects, "divergent:")
}

// A property is evaluated on the performer at every stable state, false at one a
// violation; -check-diverge this.<feature> names the performer's features.
func TestEngineCheckJudgesAPropertyOfThePerformer(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, tankModel, "-engine", "check", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::low", "-check-diverge", "this.level")
	wantReport(t, got, 1,
		"✗ Action Plant::Tank::fill: violation (11 states, 10 moves, depth 6)",
		"violation: Plant::Tank::low is false after 3 moves",
		"divergent: this.level ends as 1 or 2",
		"standing: violated (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)")

	// A property that holds at every state, over a feature the schedules agree on, holds exhaustively.
	held := check(t, binary, tankModel, "-engine", "check", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::capped", "-check-diverge", "capacity")
	wantReport(t, held, 0, "✓ Action Plant::Tank::fill: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: holds (bounded over schedules: 11 states, 10 moves searched)")
	rejectReport(t, held, "violation:", "divergent:")
}

// The bounds are named on the verdict when reached and the check is undecided:
// exit 2, as an incomplete exploration exits, never a claim of exhaustiveness.
func TestEngineCheckNamesTheBoundsItHits(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-depth", "1"),
		2, "? Action Mission::race: no violation within bounds (2 states, 1 moves, depth 1; bounds hit: depth)",
		"standing: outcomes (bounded over schedules: 2 states, 1 move searched, depth=1 (reached))")

	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-states", "3"),
		2, "no violation within bounds (3 states, 4 moves, depth 3; bounds hit: states)", "states=3 (reached)")

	// The plan's clock ending stops the search: incomplete, naming time, not a verdict.
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-timeout", "1ns"),
		2, "? Action Mission::race: incomplete: time", "standing: not covered")
}

// The check flags need -engine check and an action, -advance has no place in a
// search of every schedule, a bound is a positive count: each misuse is refused.
func TestEngineCheckRefusesMisuse(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, forkModel, "-check-depth", "3", "-action", "Mission::race"),
		2, "-check-diverge, -check-property, -check-witness, -check-depth, -check-states and -check-timeout are the check engine's; select it, as -engine check")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-depth", "3"),
		2, "-engine check searches an action's schedules; name one, as -action <name>")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-advance", "5", "-action", "Mission::race"),
		2, "-advance runs behaviors on one clock, which -engine check, searching every schedule of an action, does not; drop one of them")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-depth", "x", "-action", "Mission::race"),
		2, `-check-depth takes a bound of at least one, not "x"`)
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-states", "0", "-action", "Mission::race"),
		2, `-check-states takes a bound of at least one, not "0"`)

	// A state machine is not an action's schedules: the named engine's refusal is the answer.
	wantReport(t, check(t, binary, forkModel+lampModel, "-engine", "check", "-action", "Mission::race", "-state", "Shine::Lamp"),
		2, "check does not answer evaluate questions", "✗ Action Mission::race: divergent")
}

// lampModel is a state machine beside the fork model.
const lampModel = `package Shine {
    state def Lamp { entry; then off; state off; state on; }
}
`

// -json carries the search on its result: verdict, counts, bounds hit, each
// violation and divergent value with its witness path, and the outcomes.
func TestJSONReportsTheCheckedPlan(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := check(t, binary, forkModel, "-json", "-engine", "check", "-action", "Mission::race", "-check-witness", dir)
	var report checkedReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 1 || len(report.Checks) != 1 || len(report.Checks[0].Results) != 1 {
		t.Fatalf("status %d, checks %d\n%s", got.status, len(report.Checks), got.output())
	}
	r := report.Checks[0].Results[0]
	if r.Engine != "check" || r.Claim != "sensitive" || r.Strength != "witnessed" || r.Check == nil {
		t.Fatalf("result does not carry the check engine's search:\n%s", got.stdout)
	}
	if r.Witness == nil || r.Witness.Schedule != "replay" || len(r.Witness.Choices) != 1 {
		t.Errorf("the result's witness is not the replayed schedule:\n%s", got.stdout)
	}
	bounds := map[string]int64{}
	for _, b := range r.Bounds {
		if b.Reached {
			t.Errorf("a search within its bounds reports %s reached", b.Name)
		}
		bounds[b.Name] = b.Limit
	}
	if bounds["depth"] != 10000 || bounds["states"] != 1000000 || bounds["actionSteps"] == 0 {
		t.Errorf("the bounds do not name the engine's defaults and the executor's budgets: %v", bounds)
	}
	c := r.Check
	if c.Verdict != "divergent" || c.States != 11 || c.Moves != 10 || c.Depth != 6 || len(c.BoundsHit) != 0 ||
		len(c.Violations) != 0 || len(c.Divergent) != 1 || strings.Join(c.Outcomes, ";") != "x = 1;x = 2" {
		t.Errorf("the search is misreported:\n%s", got.stdout)
	}
	if d := c.Divergent[0]; d.Feature != "x" || len(d.Values) != 2 ||
		d.Values[0].Value != "1" || d.Values[0].Path != filepath.Join(dir, "Mission.race.x-1.witness") ||
		d.Values[1].Value != "2" || d.Values[1].Path != filepath.Join(dir, "Mission.race.x-2.witness") ||
		strings.Join(d.Values[1].Witness, ";") != "step 3: 2@left first of 2@left, 3@right" {
		t.Errorf("the divergence is misreported:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, `"boundsHit": []`) || !strings.Contains(got.stdout, `"violations": []`) {
		t.Errorf("empty lists are not carried as []:\n%s", got.stdout)
	}

	// A bound reached is named in boundsHit and on the result's bounds; a violation carries its kind and error.
	got = check(t, binary, forkModel, "-json", "-engine", "check", "-action", "Mission::race", "-check-depth", "1")
	report = checkedReport{}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	r = report.Checks[0].Results[0]
	if got.status != 2 || r.Claim != "outcomes" || r.Strength != "bounded" || r.Check.Verdict != "no violation within bounds" ||
		strings.Join(r.Check.BoundsHit, ";") != "depth" {
		t.Errorf("the reached depth bound is misreported:\n%s", got.stdout)
	}
	got = check(t, binary, tankModel, "-json", "-engine", "check", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::low")
	report = checkedReport{}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	r = report.Checks[len(report.Checks)-1].Results[0]
	if r.Claim != "violated" || len(r.Check.Violations) != 1 || r.Check.Violations[0].Kind != "property" ||
		r.Check.Violations[0].Name != "Plant::Tank::low" || r.Check.Violations[0].Depth != 3 || len(r.Check.Violations[0].Witness) != 1 {
		t.Errorf("the property violation is misreported:\n%s", got.stdout)
	}
}
