package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkedPairModel is an assembly of two parts talking through a connector: the
// ground pings the craft, which then sends a frame every second; a frame due at
// the instant the ground stops listening is a race, received or not.
const linkedPairModel = `package Comms {
    private import ScalarValues::*;
    private import SI::*;
    item def Ping;
    item def Frame;
    port def Link { out item data : Frame; in item ping : Ping; }
    part def Ground {
        port p : ~Link;
        attribute received : Integer = 0;
        exhibit state listen {
            entry; then idle;
            state idle;
            state listening {
                entry send new Ping() via p;
                do action recv {
                    first start;
                    then merge again;
                    then action got accept f : Frame via p;
                    then action count assign received := received + 1;
                    then again;
                }
            }
            transition first idle accept after 1 [s] then listening;
            transition first listening accept after 2 [s] then quiet;
            state quiet;
        }
    }
    part def Craft {
        port p : Link;
        attribute sent : Integer = 0;
        exhibit state modes {
            entry; then waiting;
            state waiting;
            state sending {
                do action tx {
                    first start;
                    then merge repeat;
                    then action wait accept after 1 [s];
                    then action emit send new Frame() via p;
                    then action count assign sent := sent + 1;
                    then repeat;
                }
            }
            transition first waiting accept Ping via p then sending;
        }
    }
    part def Pair {
        part ground : Ground;
        part craft : Craft;
        connect craft.p to ground.p;
    }
    part pair : Pair;
}
`

// TestExploreRunsAMachineOnANestedObject checks -schedule explore on a machine
// of a part nested in an assembly named by its path: each run instantiates the
// assembly and walks to the part, so the connector carries the sibling's frames
// and the race over the last one is tabled, while a model without the race
// tables one outcome; the table reads alike under one job and four.
func TestExploreRunsAMachineOnANestedObject(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, linkedPairModel, "-schedule", "explore", "-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5")
	wantReport(t, got, 0, "✓ explored Comms::Ground::listen: 2 outcomes",
		"finalState quiet; visits idle, listening, quiet; this.isSolid = true; this.received = 1 | 3              | t=3.0: state machine listen of object #2 first of state machine listen of object #2, state machine modes of object #4",
		"finalState quiet; visits idle, listening, quiet; this.isSolid = true; this.received = 2 | 1              | t=3.0: state machine modes of object #4 first of state machine listen of object #2, state machine modes of object #4; events at t=3.0: accept Frame first of time listening 1->quiet, accept Frame; at t=3.0: do listening first of do listening, dispatch time listening 1->quiet",
		"complete (4 runs)")
	for _, jobs := range []string{"1", "4"} {
		again := check(t, binary, linkedPairModel, "-jobs", jobs, "-schedule", "explore", "-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5")
		if again.output() != got.output() {
			t.Errorf("under -jobs %s:\n%s\nwant\n%s", jobs, again.output(), got.output())
		}
	}

	// The ground done listening before the last frame is due has no race to table.
	calm := strings.Replace(linkedPairModel, "accept after 2 [s] then quiet", "accept after 1.5 [s] then quiet", 1)
	wantReport(t, check(t, binary, calm, "-schedule", "explore", "-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5"), 0,
		"✓ explored Comms::Ground::listen: 1 outcome",
		"finalState quiet; visits idle, listening, quiet; this.isSolid = true; this.received = 1 | 1              | no choice points",
		"complete (1 runs)")
}

// TestExploreRunsSiblingsOnOneAssembly checks that two behaviors named on parts of
// one assembly run on one object of it per run, whether the parts are named by
// their paths or the assembly is given as -instantiate and the machines named
// alone: the craft's frames reach the ground, and the outcome spells both parts.
func TestExploreRunsSiblingsOnOneAssembly(t *testing.T) {
	binary := buildCLI(t)

	outcomes := []string{
		"Comms::pair.craft.isSolid = true; Comms::pair.craft.sent = 4; Comms::pair.ground.isSolid = true; Comms::pair.ground.received = 1 | 3              | t=3.0: state machine listen of object #2 first of state machine listen of object #2, state machine modes of object #4",
		"Comms::pair.craft.isSolid = true; Comms::pair.craft.sent = 4; Comms::pair.ground.isSolid = true; Comms::pair.ground.received = 2 | 1              | t=3.0: state machine modes of object #4 first of state machine listen of object #2, state machine modes of object #4",
		"complete (4 runs)",
	}
	byPath := check(t, binary, linkedPairModel, "-schedule", "explore",
		"-state", "Comms::Ground::listen Comms::pair.ground", "-state", "Comms::Craft::modes Comms::pair.craft", "-advance", "5")
	wantReport(t, byPath, 0, append([]string{"✓ explored Comms::Ground::listen Comms::pair.ground, Comms::Craft::modes Comms::pair.craft: 2 outcomes",
		`Comms::Craft::modes Comms::pair.craft finalState = "sending"; Comms::Craft::modes Comms::pair.craft visits = "waiting, sending"; Comms::Ground::listen Comms::pair.ground finalState = "quiet"; Comms::Ground::listen Comms::pair.ground visits = "idle, listening, quiet"; `},
		outcomes...)...)

	given := check(t, binary, linkedPairModel, "-schedule", "explore", "-instantiate", "Comms::pair",
		"-state", "Comms::Ground::listen", "-state", "Comms::Craft::modes", "-advance", "5")
	wantReport(t, given, 0, append([]string{"✓ Created instance of Comms::pair", "✓ explored Comms::Ground::listen, Comms::Craft::modes: 2 outcomes",
		`Comms::Craft::modes finalState = "sending"; Comms::Craft::modes visits = "waiting, sending"; Comms::Ground::listen finalState = "quiet"; Comms::Ground::listen visits = "idle, listening, quiet"; `},
		outcomes...)...)

	// Without the assembly given, no object of a run exhibits a machine named alone.
	wantReport(t, check(t, binary, linkedPairModel, "-schedule", "explore", "-state", "Comms::Ground::listen", "-advance", "5"), 2,
		`no object of the explored run exhibits "Comms::Ground::listen", which runs only on an object of "Comms::Ground": each run creates its own objects, so name one as Comms::Ground::listen <declaration> or Comms::Ground::listen <Assembly::part>, or -instantiate the declaration holding it for every run`)
}

// TestExploreRefusesPathsItCannotPlan checks the paths an exploration refuses
// before any run: an object of the session by id, whose objects the runs never
// see, and a path through a feature the declaration does not hold.
func TestExploreRefusesPathsItCannotPlan(t *testing.T) {
	binary := buildCLI(t)

	byID := check(t, binary, linkedPairModel, "-schedule", "explore", "-instantiate", "Comms::pair", "-state", "Comms::Ground::listen #1.ground", "-advance", "5")
	wantReport(t, byID, 2, `"#1.ground" names an object of this session, which an exploration does not run on: each explored run creates its own objects, so name a declaration to instantiate, or a path from one to an object it holds (Assembly::part.nested)`)
	rejectReport(t, byID, "explored Comms")

	wantReport(t, check(t, binary, linkedPairModel, "-schedule", "explore", "-state", "Comms::Ground::listen Comms::pair.tower", "-advance", "5"), 2,
		`Comms::pair has no feature "tower" (its features are craft, ground, and`)
	wantReport(t, check(t, binary, linkedPairModel, "-schedule", "explore", "-state", "Comms::Ground::listen Comms::pair.ground[2]", "-advance", "5"), 2,
		"ground of Comms::pair holds one value and takes no index: write ground, not ground[2]")
}

// TestEngineCheckWitnessesANestedObjectsDivergence checks -engine check and
// -engine all on a machine of a nested part: the race is a divergence of the
// part's attribute, each value's witness written for the part's path and
// replayed on the assembly's object to the value it claims.
func TestEngineCheckWitnessesANestedObjectsDivergence(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := check(t, binary, linkedPairModel, "-engine", "check", "-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5", "-check-witness", dir)
	one := filepath.Join(dir, "Comms.Ground.listen@Comms.pair%2Eground-this.received-1.witness")
	two := filepath.Join(dir, "Comms.Ground.listen@Comms.pair%2Eground-this.received-2.witness")
	wantReport(t, got, 1,
		"✗ State machine Comms::Ground::listen: divergent up to t=5.0",
		"divergent: this.received ends as 1 or 2",
		"this.received = 1 (witness "+one+")",
		"this.received = 2 (witness "+two+")",
		"standing: sensitive (witnessed:")

	content, err := os.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "object #2 = Comms::pair#1.ground\nobject #4 = Comms::pair#1.craft\n") ||
		!strings.Contains(string(content), "\nt=3.0: state machine modes of object #4 first of state machine listen of object #2, state machine modes of object #4\n") {
		t.Errorf("witness file:\n%s", content)
	}
	for witness, received := range map[string]string{one: "listen of object #2 first", two: "modes of object #4 first"} {
		replayed := check(t, binary, linkedPairModel, "-schedule", "replay:"+witness, "-trace", "-instantiate", "Comms::pair",
			"-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5")
		wantReport(t, replayed, 0, "(unordered; ran state machine "+received+")", "Current state: quiet",
			"standing: value (observed: 1 run under replay:"+witness+")")
		rejectReport(t, replayed, "replay refused", "names no move to follow")
	}
	replayed := check(t, binary, linkedPairModel, "-schedule", "replay:"+two, "-trace", "-instantiate", "Comms::pair",
		"-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5")
	if n := strings.Count(replayed.output(), "stmt assign received"); n != 2 {
		t.Errorf("the replay of received = 2 assigned received %d times:\n%s", n, replayed.output())
	}

	all := check(t, binary, linkedPairModel, "-engine", "all", "-state", "Comms::Ground::listen Comms::pair.ground", "-advance", "5", "-check-diverge", "this.received")
	wantReport(t, all, 1, "divergent: this.received ends as 1 or 2",
		"all: check sensitive (witnessed), smt refused")
}
