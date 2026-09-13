package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
		Subject string `json:"subject"`
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
		"x = 1 (witness "+filepath.Join(dir, "Mission.race-x-1.witness")+")",
		"x = 2 (witness "+filepath.Join(dir, "Mission.race-x-2.witness")+")",
		"outcome: x = 1", "outcome: x = 2",
		"standing: sensitive (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)")

	witness := filepath.Join(dir, "Mission.race-x-2.witness")
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
	agreed := strings.Replace(forkModel, "attribute x : Integer = 0;", "attribute x : Integer = 0;\n        attribute y : Integer = 0;", 1)
	wantReport(t, check(t, binary, agreed, "-engine", "check", "-action", "Mission::race", "-check-diverge", "y"),
		0, "✓ Action Mission::race: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: outcomes (bounded over schedules: 11 states, 10 moves searched)")
	rejects := check(t, binary, agreed, "-engine", "check", "-action", "Mission::race", "-check-diverge", "y")
	rejectReport(t, rejects, "divergent:")

	// A name nothing holds is refused, not silently a clean search.
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-action", "Mission::race", "-check-diverge", "y"), 2,
		"no such feature to check divergence of: y: the action holds no such feature and performs no such node")

	// The action's own attribute is divergent whatever its name is spelt with, by default and by name.
	dotted := strings.ReplaceAll(forkModel, " x ", " 'a.b' ")
	for _, args := range [][]string{nil, {"-check-diverge", "a.b"}} {
		got := check(t, binary, dotted, append([]string{"-engine", "check", "-action", "Mission::race", "-check-witness", dir}, args...)...)
		wantReport(t, got, 1, "divergent: a.b ends as 1 or 2",
			"a.b = 2 (witness "+filepath.Join(dir, "Mission.race-a.b-2.witness")+")")
	}
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
		"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::capped", "-check-diverge", "this.capacity")
	wantReport(t, held, 0, "✓ Action Plant::Tank::fill: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: holds (bounded over schedules: 11 states, 10 moves searched)")
	rejectReport(t, held, "violation:", "divergent:")
}

// straightTankModel breaks a property with no choice point on the way to it.
const straightTankModel = `package Plant {
    private import ScalarValues::*;
    part def Tank {
        attribute level : Integer = 0;
        constraint low { level < 2 }
        action overfill {
            first start;
            action a { assign level := 2; }
            done;
            succession first start then a;
            succession first a then done;
        }
    }
    part tank : Tank;
}
`

// A violation reached before any choice point is witnessed as `no choice points`
// over its trace, and that witness replays as -schedule replay:<file> to the state it claims.
func TestEngineCheckWitnessOfNoChoiceReplays(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := check(t, binary, straightTankModel, "-engine", "check", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::overfill Plant::tank", "-check-property", "Plant::Tank::low", "-check-witness", dir)
	witness := filepath.Join(dir, "Plant.Tank.overfill@Plant.tank.violation-1.witness")
	wantReport(t, got, 1, "✗ Action Plant::Tank::overfill: violation",
		"violation: Plant::Tank::low is false after 2 moves (witness "+witness+")",
		"standing: violated (witnessed:")
	content, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "no choice points\n\n") || !strings.Contains(string(content), "\nproperty: Plant::Tank::low") {
		t.Errorf("witness file:\n%s", content)
	}
	replayed := check(t, binary, straightTankModel, "-schedule", "replay:"+witness, "-trace", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::overfill Plant::tank")
	wantReport(t, replayed, 0, "stmt assign level", "Action completed",
		"standing: value (observed: 1 run under replay:"+witness+")")
	rejectReport(t, replayed, "replay refused", "names no move to follow")
}

// One action checked on two objects writes two sets of witnesses, each named for
// its performer, so the second check does not overwrite the first's files.
func TestEngineCheckNamesWitnessesForThePerformer(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := strings.Replace(tankModel, "part tank : Tank;", "part tank : Tank;\n    part spare : Tank;", 1)

	got := check(t, binary, model, "-engine", "check", "-instantiate", "Plant::tank", "-instantiate", "Plant::spare",
		"-action", "Plant::Tank::fill Plant::tank", "-action", "Plant::Tank::fill Plant::spare",
		"-check-diverge", "this.level", "-check-witness", dir)
	tank := filepath.Join(dir, "Plant.Tank.fill@Plant.tank-this.level-1.witness")
	spare := filepath.Join(dir, "Plant.Tank.fill@Plant.spare-this.level-1.witness")
	wantReport(t, got, 1,
		"this.level = 1 (witness "+tank+")",
		"this.level = 2 (witness "+filepath.Join(dir, "Plant.Tank.fill@Plant.tank-this.level-2.witness")+")",
		"this.level = 1 (witness "+spare+")",
		"this.level = 2 (witness "+filepath.Join(dir, "Plant.Tank.fill@Plant.spare-this.level-2.witness")+")")
	for _, path := range []string{tank, spare} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), "step 3: ") {
			t.Errorf("%s:\n%s", path, content)
		}
	}
	replayed := check(t, binary, model, "-schedule", "replay:"+spare, "-trace", "-instantiate", "Plant::spare",
		"-action", "Plant::Tank::fill Plant::spare")
	wantReport(t, replayed, 0, "took 3@b first", "standing: value (observed: 1 run under replay:"+spare+")")
}

// One action on two objects run on one clock is one invocation: a feature to check
// is named on its object, the witnesses bind each object to its path, and a fresh
// run, numbering its objects otherwise, replays them to the value they record.
func TestEngineCheckBindsWitnessObjectsAcrossRuns(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := strings.Replace(tankModel, "part tank : Tank;", "part tank : Tank;\n    part spare : Tank;", 1)
	invocation := []string{"-instantiate", "Plant::tank", "-instantiate", "Plant::spare",
		"-action", "Plant::Tank::fill Plant::tank", "-action", "Plant::Tank::fill Plant::spare", "-advance", "1"}
	name := func(n int) string {
		return filepath.Join(dir, fmt.Sprintf("Plant.Tank.fill@Plant.tank+Plant.Tank.fill@Plant.spare-Plant.spare.level-%d.witness", n))
	}

	wantReport(t, check(t, binary, model, slices.Concat([]string{"-engine", "check"}, invocation, []string{"-check-diverge", "this.level"})...), 2,
		"no such feature to check divergence of: this.level: the behaviors perform on different objects, Plant::tank and Plant::spare; name the object's feature, as Plant::tank.level")

	got := check(t, binary, model, slices.Concat([]string{"-engine", "check"}, invocation, []string{"-check-diverge", "Plant::spare.level", "-check-witness", dir})...)
	wantReport(t, got, 1, "divergent: Plant::spare.level ends as 1 or 2",
		"Plant::spare.level = 1 (witness "+name(1)+")", "Plant::spare.level = 2 (witness "+name(2)+")", "witness of 3 choices replayed)")
	// The spare's fill runs first, so its step 3 is the first: `b` first leaves level 1, `a` first 2.
	for _, c := range []struct {
		n     int
		took  string
		level string
	}{{1, "3@b", "1"}, {2, "2@a", "2"}} {
		content, err := os.ReadFile(name(c.n))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), "object #1 = Plant::tank#1\nobject #2 = Plant::spare#1\n"+
			"t=0.0: action fill of object #2 first of action fill of object #1, action fill of object #2\nstep 3: "+c.took+" first of 2@a, 3@b\n") {
			t.Errorf("witness %d, spare.level = %s:\n%s", c.n, c.level, content)
		}
		// The run makes the spare's performance after the tank's: the spare is object #3 here.
		replayed := check(t, binary, model, slices.Concat([]string{"-schedule", "replay:" + name(c.n), "-trace"}, invocation)...)
		wantReport(t, replayed, 0, "materialize: spare #3", "ran action fill of object #3 first)",
			"standing: value (observed: 1 run under replay:"+name(c.n)+")")
		_, spareRan, _ := strings.Cut(replayed.output(), "ran action fill of object #3 first)")
		if !strings.Contains(spareRan, "choice step 3: tokens 2@a, 3@b (unordered; took "+c.took+" first)") ||
			strings.Count(replayed.output(), "Action completed") != 2 {
			t.Errorf("witness %d does not replay to spare.level = %s:\n%s", c.n, c.level, replayed.output())
		}
	}
}

// A witness explore writes for a step the clock retries replays under -schedule
// replay:<file>: the token order is drawn at the retry, where both branches are due.
func TestEngineReplaysAnOrderDrawnAfterTheClockRetriesAStep(t *testing.T) {
	binary := buildCLI(t)
	model, err := os.ReadFile(filepath.Join("..", "..", "internal", "core", "runtime", "testdata", "conformance",
		"action_explore_performed_and_accept_due_together.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(t.TempDir(), "wake.witness")
	const choices = "step 3: 2@performed first of 2@performed, 3@direct\nstep 4: 2@writeOne first of 2@writeOne, 3@direct\n"
	if err := os.WriteFile(witness, []byte(choices), 0o644); err != nil {
		t.Fatal(err)
	}
	replayed := check(t, binary, string(model), "-schedule", "replay:"+witness, "-trace", "-action", "test::wake")
	wantReport(t, replayed, 0, "took 2@performed first", "took 2@writeOne first", "x = 2",
		"standing: value (observed: 1 run under replay:"+witness+")")
	rejectReport(t, replayed, "replay refused")
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

// A check flag under -engine all puts the action to check and explore together:
// the one -check-states figure bounds the search's states and the exploration's
// runs, and the exploration referees the search's outcomes.
func TestEngineAllChecksBesideExploring(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-json", "-engine", "all", "-action", "Mission::race", "-check-states", "100")
	var report checkedReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 0 || len(report.Checks) != 1 || len(report.Checks[0].Results) != 2 {
		t.Fatalf("status %d, checks %d\n%s", got.status, len(report.Checks), got.output())
	}
	limits := map[string]int64{}
	for _, r := range report.Checks[0].Results {
		for _, b := range r.Bounds {
			limits[r.Engine+"."+b.Name] = b.Limit
		}
		switch r.Engine {
		case "check":
			if r.Claim != "sensitive" || r.Strength != "witnessed" || r.Check == nil ||
				strings.Join(r.Check.Outcomes, ";") != "x = 1;x = 2" {
				t.Errorf("the check engine's search is misreported:\n%s", got.stdout)
			}
		case "explore":
			if r.Claim != "outcomes" || r.Strength != "proved" {
				t.Errorf("the exploration is misreported:\n%s", got.stdout)
			}
		default:
			t.Errorf("engine %s answered an action under -engine all", r.Engine)
		}
	}
	if limits["check.states"] != 100 || limits["explore.runs"] != 100 || limits["check.depth"] != 64 {
		t.Errorf("the one figure does not bound each engine in its unit: %v", limits)
	}

	// Without a check flag, -engine all leaves an action to the run and explore engines.
	got = check(t, binary, forkModel, "-json", "-engine", "all", "-action", "Mission::race")
	report = checkedReport{}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	for _, r := range report.Checks[0].Results {
		if r.Engine == "check" {
			t.Errorf("the check engine searched an action no check flag asked it to:\n%s", got.stdout)
		}
	}
}

// The check flags need -engine check or all and a behavior, a bound is a positive
// count: each misuse is refused.
func TestEngineCheckRefusesMisuse(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, forkModel, "-check-depth", "3", "-action", "Mission::race"),
		2, "-check-diverge, -check-property, -check-input, -check-assume, -check-witness, -check-depth, -check-states, -check-unroll and -check-timeout are the check and smt engines'; select one, as -engine check or -engine smt, or every engine, as -engine all")
	wantReport(t, check(t, binary, forkModel, "-engine", "explore", "-check-depth", "3", "-action", "Mission::race"),
		2, "select one, as -engine check or -engine smt, or every engine, as -engine all")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-depth", "3"),
		2, "the -check-* flags search a behavior's schedules; name one, as -action <name> or -state <name>")
	wantReport(t, check(t, binary, forkModel, "-engine", "all", "-check-depth", "3"),
		2, "the -check-* flags search a behavior's schedules; name one, as -action <name> or -state <name>")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-depth", "x", "-action", "Mission::race"),
		2, `-check-depth takes a bound of at least one, not "x"`)
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-states", "0", "-action", "Mission::race"),
		2, `-check-states takes a bound of at least one, not "0"`)

	// A flag one engine alone reads is refused under the other engine alone, not dropped.
	wantReport(t, check(t, binary, forkModel, "-engine", "smt", "-check-diverge", "x", "-action", "Mission::race"),
		2, "-check-diverge is the check engine's, which -engine smt leaves out; select it, as -engine check, or every engine, as -engine all")
	wantReport(t, check(t, binary, forkModel, "-engine", "smt", "-check-diverge", "x", "-check-states", "3", "-action", "Mission::race"),
		2, "-check-diverge and -check-states are the check engine's, which -engine smt leaves out")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-input", "x", "-action", "Mission::race"),
		2, "-check-input is the smt engine's, which -engine check leaves out; select it, as -engine smt, or every engine, as -engine all")
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-check-input", "x", "-check-assume", "Mission::x", "-check-unroll", "2", "-action", "Mission::race"),
		2, "-check-input, -check-assume and -check-unroll are the smt engine's, which -engine check leaves out")

	// The smt engine searches an action's schedules alone, with no clock to advance.
	wantReport(t, check(t, binary, lampModel, "-engine", "smt", "-check-unroll", "2", "-state", "Shine::Lamp::glow Shine::Lamp"),
		2, "the -check-* flags search an action's schedules under -engine smt; name one, as -action <name>")
	wantReport(t, check(t, binary, forkModel, "-engine", "smt", "-action", "Mission::race", "-advance", "1"),
		2, "-advance runs behaviors on one clock, which -engine smt's search of an action's schedules does not; drop one of them")
}

// A body paused mid-statement — a performed action waiting at an accept while a
// sibling accept falls due with it — is a state the search holds and resumes.
func TestEngineCheckSearchesAPausedBody(t *testing.T) {
	binary := buildCLI(t)
	paused, err := os.ReadFile(filepath.Join("..", "..", "internal", "core", "runtime", "testdata", "conformance",
		"action_explore_performed_and_accept_due_together.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	wantReport(t, check(t, binary, string(paused), "-engine", "check", "-action", "test::wake"),
		1, "✗ Action test::wake: divergent", "divergent: x ends as 1 or 2")
}

// lampModel has a machine and an action of one part due at one instant of the
// clock they share, so which runs first is a choice point the search draws.
const lampModel = `package Shine {
    private import SI::*;
    private import ScalarValues::*;
    part def Lamp {
        attribute lit : Boolean = false;
        exhibit state glow {
            entry; then off;
            state off;
            accept after 3 [s] then on;
            state on { entry assign lit := true; }
        }
        action peek {
            attribute saw : Boolean = false;
            first start;
            then action wait accept after 3 [s];
            then action look assign saw := lit;
            then done;
        }
    }
}
`

// -engine check searches a state machine's schedules as it does an action's, its
// clock advanced until nothing is due or, with -advance, up to that instant; the
// behaviors named together are one invocation on one clock, answered by one verdict.
func TestEngineCheckSearchesBehaviorsOnOneClock(t *testing.T) {
	binary := buildCLI(t)
	glow, peek := "Shine::Lamp::glow Shine::Lamp", "Shine::Lamp::peek Shine::Lamp"

	wantReport(t, check(t, binary, lampModel, "-engine", "check", "-state", glow),
		0, "✓ State machine Shine::Lamp::glow: no violation, exhaustive (2 states, 1 moves, depth 1)",
		"outcome: finalState on; visits off, on; this.isSolid = true; this.lit = true")
	wantReport(t, check(t, binary, lampModel, "-engine", "check", "-state", glow, "-advance", "1"),
		0, "✓ State machine Shine::Lamp::glow: no violation, exhaustive up to t=1.0 (1 states, 0 moves, depth 0)",
		"outcome: finalState off; visits off; this.isSolid = true; this.lit = false")

	// Named apart, each behavior is its own search; the action's clock runs to its end.
	wantReport(t, check(t, binary, lampModel, "-engine", "check", "-action", peek, "-state", glow),
		1, "✗ Action Shine::Lamp::peek: divergent (10 states, 9 moves, depth 5)", "divergent: saw ends as false or true",
		"✓ State machine Shine::Lamp::glow: no violation, exhaustive (2 states, 1 moves, depth 1)")

	// With -advance they are one invocation: the machine's timer and the action's
	// wait are due together, and the order the search draws decides what peek saw.
	got := check(t, binary, lampModel, "-engine", "check", "-action", peek, "-state", glow, "-advance", "3")
	wantReport(t, got, 1, "✗ Behaviors Shine::Lamp::peek, Shine::Lamp::glow: divergent up to t=3.0 (10 states, 9 moves, depth 5)",
		"divergent: Shine::Lamp::peek.saw ends as false or true",
		`outcome: Shine::Lamp::glow finalState = "on"; Shine::Lamp::glow visits = "off, on"; Shine::Lamp::peek.saw = false; this.isSolid = true; this.lit = true`,
		`outcome: Shine::Lamp::glow finalState = "on"; Shine::Lamp::glow visits = "off, on"; Shine::Lamp::peek.saw = true; this.isSolid = true; this.lit = true`)
	if strings.Count(got.stdout, "✗") != 1 {
		t.Errorf("one invocation is answered by one verdict:\n%s", got.output())
	}

	// A horizon before the tie leaves both waiting: no divergence up to it.
	wantReport(t, check(t, binary, lampModel, "-engine", "check", "-action", peek, "-state", glow, "-advance", "2"),
		0, "✓ Behaviors Shine::Lamp::peek, Shine::Lamp::glow: no violation, exhaustive up to t=2.0")

	// -advance bounds an action's search as it does a run: the fork is drawn within it.
	wantReport(t, check(t, binary, forkModel, "-engine", "check", "-advance", "5", "-action", "Mission::race"),
		1, "✗ Action Mission::race: divergent up to t=5.0 (11 states, 10 moves, depth 6)")

	// -json tables the joint outcome, each behavior's observables under its name.
	got = check(t, binary, lampModel, "-json", "-engine", "check", "-action", peek, "-state", glow, "-advance", "3")
	var report checkedReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 || len(report.Checks[0].Results) != 1 || report.Checks[0].Results[0].Check == nil {
		t.Fatalf("one invocation is one check with the engine's result:\n%s", got.stdout)
	}
	c := report.Checks[0].Results[0].Check
	if report.Checks[0].Subject != "Shine::Lamp::peek, Shine::Lamp::glow" || c.Verdict != "divergent" || len(c.Outcomes) != 2 ||
		!strings.Contains(c.Outcomes[0], `Shine::Lamp::glow finalState = "on"`) || !strings.Contains(c.Outcomes[1], "Shine::Lamp::peek.saw = true") {
		t.Errorf("the joint outcome is misreported:\n%s", got.stdout)
	}
}

// A witness of behaviors on one clock names the tie's due order among them, and
// -schedule replay:<file> runs the same behaviors under it to the value it records.
func TestEngineCheckWitnessOfBehaviorsOnOneClockReplays(t *testing.T) {
	binary := buildCLI(t)
	glow, peek := "Shine::Lamp::glow Shine::Lamp", "Shine::Lamp::peek Shine::Lamp"
	dir := t.TempDir()
	name := func(n int) string {
		return filepath.Join(dir, fmt.Sprintf("Shine.Lamp.peek+Shine.Lamp.glow@Shine.Lamp-Shine.Lamp.peek.saw-%d.witness", n))
	}

	got := check(t, binary, lampModel, "-engine", "check", "-action", peek, "-state", glow, "-advance", "3", "-check-witness", dir)
	wantReport(t, got, 1, "divergent: Shine::Lamp::peek.saw ends as false or true",
		"Shine::Lamp::peek.saw = false (witness "+name(1)+")", "Shine::Lamp::peek.saw = true (witness "+name(2)+")",
		"standing: sensitive (witnessed: 10 states, 9 moves searched, witness of 1 choice replayed)")

	for _, c := range []struct {
		n     int
		first string
		saw   string
	}{
		{1, "action peek of object #1", "false"},
		{2, "state machine glow of object #1", "true"},
	} {
		content, err := os.ReadFile(name(c.n))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), "object #1 = Shine::Lamp#1\nt=3.0: "+c.first+" first of action peek of object #1, state machine glow of object #1\n\n") ||
			!strings.Contains(string(content), "choice at t=3.0: due action peek of object #1, state machine glow of object #1 (unordered; ran "+c.first+" first)") {
			t.Errorf("witness %d:\n%s", c.n, content)
		}
		replayed := check(t, binary, lampModel, "-schedule", "replay:"+name(c.n), "-trace", "-instantiate", "Shine::Lamp", "-action", peek, "-state", glow, "-advance", "3")
		wantReport(t, replayed, 0, "ran "+c.first+" first)", "saw = "+c.saw, "Current state: on",
			"standing: value (observed: 1 run under replay:"+name(c.n)+")")
	}
}

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
		d.Values[0].Value != "1" || d.Values[0].Path != filepath.Join(dir, "Mission.race-x-1.witness") ||
		d.Values[1].Value != "2" || d.Values[1].Path != filepath.Join(dir, "Mission.race-x-2.witness") ||
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
