package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// checkTankSource is a fork race on a performer's attribute, with a property
// one branch breaks and one every schedule keeps.
const checkTankSource = `
package Plant {
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

// The checker settings show their defaults, take values, and clear with off.
func TestCheckSettingsShowSetAndClear(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	wants(t, run(t, s, "%check-diverge"), "check-diverge: off (every attribute of the action and of its performing object)")
	wants(t, run(t, s, "%check-diverge leftCount rightCount"), "check-diverge: leftCount rightCount")
	if got := strings.Join(s.CheckDiverge(), ","); got != "leftCount,rightCount" {
		t.Errorf("CheckDiverge() = %q", got)
	}
	wants(t, run(t, s, "%check-diverge off"), "check-diverge: off (every attribute of the action and of its performing object)")

	wants(t, run(t, s, "%check-property"), "check-property: off (none)")
	wants(t, run(t, s, "%check-property Plant::Tank::low"), "check-property: Plant::Tank::low")
	wants(t, run(t, s, "%check-property off"), "check-property: off (none)")

	wants(t, run(t, s, "%check-witness"), "check-witness: off")
	wants(t, run(t, s, "%check-witness "+t.TempDir()), "check-witness: "+s.CheckWitnessDir())
	wants(t, run(t, s, "%check-witness off"), "check-witness: off")

	wants(t, run(t, s, "%check-bounds"), "check-bounds: depth=10000 (default), states=1000000 (default), timeout=off")
	wants(t, run(t, s, "%check-bounds depth=3 timeout=2s"), "check-bounds: depth=3, states=1000000 (default), timeout=2s")
	if depth, states, timeout := s.CheckBounds(); depth != 3 || states != 0 || timeout != 2*time.Second {
		t.Errorf("CheckBounds() = %d, %d, %s", depth, states, timeout)
	}
	for _, bad := range []string{"depth", "depth=0", "states=x", "timeout=-1s", "width=4"} {
		out := run(t, s, "%check-bounds "+bad)
		rejects(t, out, "check-bounds: ")
		if !strings.HasPrefix(out, "error: ") && !strings.HasPrefix(out, "usage: ") {
			t.Errorf("%%check-bounds %s: %q", bad, out)
		}
	}
	wants(t, run(t, s, "%check-bounds"), "check-bounds: depth=3, states=1000000 (default), timeout=2s")
	wants(t, run(t, s, "%check-bounds off"), "check-bounds: depth=10000 (default), states=1000000 (default), timeout=off")
}

// Under %engine check, %action searches the schedules instead of stepping one:
// the race diverges, witnesses land in %check-witness, and a steady feature clears.
func TestEngineCheckSearchesTheActionsSchedules(t *testing.T) {
	s := loadSource(t, checkTankSource)
	dir := t.TempDir()
	wants(t, run(t, s, "%engine check"), "engine: check")
	run(t, s, "%check-witness "+dir)
	run(t, s, "%instantiate Plant::tank")

	v := s.RunAction("Plant::Tank::fill", "Plant::tank")
	wantVerdict(t, v, VerdictFails,
		"Action Plant::Tank::fill: divergent (11 states, 10 moves, depth 6)",
		"divergent: this.level ends as 1 or 2",
		"this.level = 1 (witness "+filepath.Join(dir, "Plant.Tank.fill@Plant.tank-this.level-1.witness")+")",
		"standing: sensitive (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)")
	rejects(t, run(t, s, "%step"), "Step complete")
	witness, err := os.ReadFile(filepath.Join(dir, "Plant.Tank.fill@Plant.tank-this.level-2.witness"))
	if err != nil {
		t.Fatal(err)
	}
	header, body, found := strings.Cut(string(witness), "\n\n")
	if !found || !strings.HasPrefix(header, "step 3: ") || !strings.Contains(body, "choice step 3: tokens") {
		t.Errorf("witness is not choices, a blank line, then the trace:\n%s", witness)
	}

	run(t, s, "%check-diverge this.capacity")
	wantVerdict(t, s.RunAction("Plant::Tank::fill", "Plant::tank"), VerdictHolds,
		"Action Plant::Tank::fill: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: outcomes (bounded over schedules: 11 states, 10 moves searched)")
}

// A property the check evaluates at every state is judged about the performer:
// low fails on the branch that writes 2, capped holds on every schedule.
func TestEngineCheckJudgesPropertiesOfThePerformer(t *testing.T) {
	s := loadSource(t, checkTankSource)
	run(t, s, "%engine check")
	run(t, s, "%instantiate Plant::tank")

	run(t, s, "%check-property Plant::Tank::low")
	wantVerdict(t, s.RunAction("Plant::Tank::fill", "Plant::tank"), VerdictFails,
		"Action Plant::Tank::fill: violation (11 states, 10 moves, depth 6)",
		"violation: Plant::Tank::low is false after 3 moves",
		"standing: violated (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)")

	run(t, s, "%check-property Plant::Tank::capped")
	run(t, s, "%check-diverge this.capacity")
	wantVerdict(t, s.RunAction("Plant::Tank::fill", "Plant::tank"), VerdictHolds,
		"Action Plant::Tank::fill: no violation, exhaustive (11 states, 10 moves, depth 6)",
		"standing: holds (bounded over schedules")

	run(t, s, "%check-property Plant::Tank::level")
	wantVerdict(t, s.RunAction("Plant::Tank::fill", "Plant::tank"), VerdictUnresolved,
		`"Plant::Tank::level" is not a constraint or requirement`)
}

// Under %engine all a check setting puts %action to check and explore together,
// the one states figure bounding each in its unit; without one, %action steps.
func TestEngineAllChecksOnceASettingIsMade(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	wants(t, run(t, s, "%engine all"), "engine: all")
	run(t, s, "%check-bounds states=100")
	v := s.RunAction("Debug::tally")
	wantVerdict(t, v, VerdictHolds, "explored Debug::tally: 1 outcome",
		"all: check outcomes (bounded), explore outcomes (proved)")
	limits := map[string]int64{}
	for _, step := range v.Plan.Steps {
		if step.Result == nil {
			t.Fatalf("%s did not answer: %v", step.Engine, step)
		}
		for _, b := range step.Result.Bounds {
			limits[step.Engine+"."+b.Name] = b.Limit
		}
	}
	if limits["check.states"] != 100 || limits["explore.runs"] != 100 {
		t.Errorf("the one figure does not bound each engine in its unit: %v", limits)
	}
	rejects(t, run(t, s, "%step"), "Step complete")

	run(t, s, "%check-bounds off")
	wants(t, run(t, s, "%action Debug::tally"), "Started action executor")
	wants(t, run(t, s, "%step"), "Step complete")
}

// The bounds set by %check-bounds cut the search short and are named in the
// verdict, which then claims no exhaustiveness.
func TestEngineCheckNamesTheBoundsItHits(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%engine check")
	run(t, s, "%check-bounds depth=2")
	v := s.RunAction("Debug::tally")
	wantVerdict(t, v, VerdictUnresolved, "no violation within bounds", "bounds hit: depth")
	rejectVerdict(t, v, "exhaustive")

	run(t, s, "%check-bounds off")
	run(t, s, "%check-bounds states=2")
	wantVerdict(t, s.RunAction("Debug::tally"), VerdictUnresolved, "no violation within bounds", "bounds hit: states")

	run(t, s, "%check-bounds off")
	run(t, s, "%check-bounds timeout=1ns")
	wantVerdict(t, s.RunAction("Debug::tally"), VerdictUnresolved, "incomplete", "time")
}

// %replay installs a witness's schedule and the next %action steps the run it
// records, ending at the value the witness claims.
func TestReplayStepsTheRunAWitnessRecords(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	dir := t.TempDir()
	run(t, s, "%engine check")
	run(t, s, "%check-witness "+dir)
	run(t, s, "%check-diverge leftCount")
	wantVerdict(t, s.RunAction("Debug::tally"), VerdictHolds, "no violation, exhaustive")
	run(t, s, "%engine auto")

	witness := filepath.Join(dir, "Debug.tally.trace.witness")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("a clean check wrote witnesses: %v %v", entries, err)
	}
	if err := os.WriteFile(witness, []byte("step 3: 3@right first of 2@left, 3@right\n\ntrailing trace\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wants(t, run(t, s, "%replay"), "usage: %replay <witness>")
	out := run(t, s, "%replay "+filepath.Join(dir, "missing"))
	wants(t, out, "error: invalid scheduling policy")
	wants(t, run(t, s, "%schedule"), "schedule: reverse")

	run(t, s, "%engine check")
	wants(t, run(t, s, "%replay "+witness), "schedule: replay:"+witness,
		"Under %engine check, %action searches every schedule; select %engine auto to step the one the witness records")
	run(t, s, "%engine auto")
	out = run(t, s, "%replay "+witness)
	wants(t, out, "schedule: replay:"+witness,
		"Use %action or %state to start the run the witness records, then %step or %continue")
	if strings.Contains(out, "Under %engine check") {
		t.Fatalf("the check-engine hint shown under auto: %v", out)
	}
	run(t, s, "%trace on")
	run(t, s, "%action Debug::tally")
	run(t, s, "%step")
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)")
	wants(t, run(t, s, "%continue"), "Action completed", "leftCount = 1", "rightCount = 10")
}

// checkStraightSource breaks a property with no choice point on the way to it.
const checkStraightSource = `
package Plant {
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

// A violation the check reaches before any choice point is witnessed as `no choice
// points`, and %replay of that witness steps the one run there is to the state it claims.
func TestReplayStepsAWitnessOfNoChoice(t *testing.T) {
	s := loadSource(t, checkStraightSource)
	dir := t.TempDir()
	run(t, s, "%engine check")
	run(t, s, "%check-witness "+dir)
	run(t, s, "%check-property Plant::Tank::low")
	run(t, s, "%instantiate Plant::tank")
	witness := filepath.Join(dir, "Plant.Tank.overfill@Plant.tank.violation-1.witness")
	wantVerdict(t, s.RunAction("Plant::Tank::overfill", "Plant::tank"), VerdictFails,
		"violation: Plant::Tank::low is false after 2 moves (witness "+witness+")")
	content, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "no choice points\n\n") {
		t.Errorf("witness is not `no choice points`, a blank line, then the trace:\n%s", content)
	}

	run(t, s, "%engine auto")
	wants(t, run(t, s, "%replay "+witness), "schedule: replay:"+witness,
		"Use %action or %state to start the run the witness records, then %step or %continue")
	run(t, s, "%action Plant::Tank::overfill Plant::tank")
	wants(t, run(t, s, "%step"), "Step complete")
	wants(t, run(t, s, "%continue"), "Action completed")
	wants(t, run(t, s, "%features Plant::tank"), "level = 2")
}

// A witness that does not fit the run it is replayed against is refused where
// the run departs from it, not followed silently.
func TestReplayRefusesAWitnessOfAnotherRun(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	witness := filepath.Join(t.TempDir(), "other.witness")
	if err := os.WriteFile(witness, []byte("step 3: 9@nowhere first of 9@nowhere, 3@right\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, s, "%replay "+witness)
	run(t, s, "%action Debug::tally")
	run(t, s, "%step")
	run(t, s, "%step")
	out := run(t, s, "%step")
	wants(t, out, "error: step failed: replay refused: move 1 (step 3: 9@nowhere first of 9@nowhere, 3@right): step 3: 9@nowhere is not able to act (able to act: 2@left, 3@right)")
	rejects(t, out, "Step complete")
}

// A witness with moves left when the run completes is refused as the run ends,
// whether %continue or %step brings the run there.
func TestReplayRefusesMovesLeftWhenTheRunCompletes(t *testing.T) {
	witness := filepath.Join(t.TempDir(), "long.witness")
	if err := os.WriteFile(witness, []byte("step 3: 3@right first of 2@left, 3@right\nstep 9: 2@left first of 2@left, 3@right\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refused := "replay refused: move 2 (step 9: 2@left first of 2@left, 3@right): the run ended"

	s := loadSource(t, choiceForkSource)
	run(t, s, "%replay "+witness)
	run(t, s, "%action Debug::tally")
	out := run(t, s, "%continue")
	wants(t, out, refused)
	rejects(t, out, "Action completed")

	s = loadSource(t, choiceForkSource)
	run(t, s, "%replay "+witness)
	run(t, s, "%action Debug::tally")
	for i := 0; i < 20; i++ {
		out = run(t, s, "%step")
		if !strings.Contains(out, "Step complete") {
			break
		}
	}
	wants(t, out, refused)
	rejects(t, out, "Action completed")
}
