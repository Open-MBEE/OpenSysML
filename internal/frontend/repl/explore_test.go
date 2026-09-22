package repl

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// exploreRaceSource forks three writers of one feature, which the library leaves
// unordered: six linearizations, three values of x.
const exploreRaceSource = `
package Race {
	private import ScalarValues::*;
	action race {
		attribute x : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 3; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
	action steady {
		attribute y : Integer = 0;
		first start;
		action only { assign y := 7; }
		done;
		succession first start then only;
		succession first only then done;
	}
	part def Holder {
		attribute x : Integer = 0;
	}
}
`

// RunAction under explore tables every distinct outcome, sorted, with how many
// linearizations reached it and one witness, then reports the exploration
// complete; the run holds.
func TestRunActionExploresEveryLinearization(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"),
		"✓ explored Race::race: 3 outcomes",
		"outcome | linearizations | witness",
		"x = 1   | 2              | step 3: 3@b first of 2@a, 3@b, 4@c; step 4: 4@c first of 2@a, 4@c",
		"x = 2   | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 4@c first of 3@b, 4@c",
		"x = 3   | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 3@b first of 3@b, 4@c",
		"complete (6 runs)")
	if len(v.Outcomes) != 3 || v.Exploration == nil || !v.Exploration.Complete || v.Exploration.Runs != 6 {
		t.Errorf("verdict outcomes = %+v, exploration = %+v", v.Outcomes, v.Exploration)
	}
	if got := v.Outcomes[0]; got.Linearizations != 2 || got.Error != "" || len(got.Values) != 1 ||
		got.Values[0].Name != "x" || got.Values[0].Value != "1" || len(got.Witness) != 2 {
		t.Errorf("first outcome = %+v", got)
	}
	// Exploring is deterministic: the same model tables the same rows.
	if again := s.RunAction("Race::race"); strings.Join(again.Lines, "\n") != strings.Join(v.Lines, "\n") {
		t.Errorf("exploration rendered\n%s\nthen\n%s", strings.Join(v.Lines, "\n"), strings.Join(again.Lines, "\n"))
	}
}

// A behavior with no choice point explores in a single run.
func TestRunActionExploresNoChoiceInOneRun(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::steady")
	wants(t, strings.Join(v.Lines, "\n"), "✓ explored Race::steady: 1 outcome", "y = 7   | 1              | no choice points", "complete (1 runs)")
}

// A budget hit leaves the verdict unresolved and names the budget; the outcomes
// reached within it are still tabled.
func TestRunActionExploreReportsTheBudgetHit(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore:runs=3")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race")
	if v.Status != VerdictUnresolved {
		t.Errorf("status = %v, want unresolved", v.Status)
	}
	wants(t, strings.Join(v.Lines, "\n"), "? explored Race::race: 2 outcomes", "incomplete: runs budget 3 hit after 3 runs")
	if v.Exploration == nil || v.Exploration.Complete || strings.Join(v.Exploration.BudgetsHit, ",") != "runs" {
		t.Errorf("exploration = %+v", v.Exploration)
	}

	if err := s.SetSchedule(mustSchedule(t, "explore:depth=1,runs=100")); err != nil {
		t.Fatal(err)
	}
	v = s.RunAction("Race::race")
	wants(t, strings.Join(v.Lines, "\n"), "incomplete: depth budget 1 hit after 3 runs")
}

// With tracing on, the trace shown per outcome is its witness run's: one token
// moves per explored step, so the last write to x is the witness's last move.
// Runs vary the first run's choices earliest first: run 2 takes 3@b first at
// step 3, run 3 varies run 1's step 4, and run 5 is the second below 3@b.
func TestRunActionExploreTracesTheWitnessOfEachOutcome(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	run(t, s, "%trace on")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(s.RunAction("Race::race").Lines, "\n")
	wantsInOrder(t, out,
		"complete (6 runs)",
		"trace of outcome 1's witness (run 5):",
		"took 3@b first",
		"took 4@c first",
		"eval literal 1 -> 1",
		"trace of outcome 2's witness (run 3):",
		"took 2@a first",
		"took 4@c first",
		"eval literal 2 -> 2",
		"trace of outcome 3's witness (run 1):",
		"took 2@a first",
		"took 3@b first",
		"eval literal 3 -> 3")
	if strings.Count(out, "trace of outcome") != 3 {
		t.Errorf("want one trace per outcome:\n%s", out)
	}
}

// An action performed by an object explores on an object each run makes for
// itself; an object of the session cannot be named, since no run has it.
func TestRunActionExploreInstantiatesThePerformer(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	run(t, s, "%instantiate Race::Holder")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race", "Race::Holder")
	wants(t, strings.Join(v.Lines, "\n"), "✓ explored Race::race: 3 outcomes", "complete (6 runs)")

	v = s.RunAction("Race::race", "#1")
	var explored *ExploredObjectError
	if v.Status != VerdictUnresolved || !strings.Contains(strings.Join(v.Lines, "\n"), (&ExploredObjectError{Ref: "#1"}).Error()) {
		t.Errorf("an object of the session was explored on:\n%s", strings.Join(v.Lines, "\n"))
	}
	if !errors.As(&ExploredObjectError{}, &explored) {
		t.Fatal("ExploredObjectError is not an error")
	}
}

// exploreCommsSource is two parts talking over a connector: the craft's second
// frame and the ground's stop fall due together, so whether the frame is received is a race.
const exploreCommsSource = `
package Comms {
	private import ScalarValues::*;
	private import SI::*;
	item def Ping;
	item def Frame;
	port def Link { out item data : Frame; in item ping : Ping; }
	part def Ground {
		port p : ~Link;
		exhibit state listen {
			attribute received : Integer = 0;
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
			attribute emitted : Integer = 0;
			entry; then waiting;
			state waiting;
			state sending {
				do action tx {
					first start;
					then merge repeat;
					then action wait accept after 1 [s];
					then action emit send new Frame() via p;
					then action count assign sent := sent + 1;
					then action tally assign emitted := emitted + 1;
					then repeat;
				}
			}
			transition first waiting accept Ping via p then sending;
		}
		action ack {
			attribute seen : Integer = 0;
			first start;
			then action wait accept after 3 [s];
			then action look assign seen := sent;
			then done;
		}
	}
	part def Pair {
		part ground : Ground;
		part craft : Craft;
		connect craft.p to ground.p;
	}
	part pair : Pair;
	part def Fleet {
		part pairs : Pair[2];
		part spare : Ground[0..1];
		attribute count : Integer = 2;
	}
	analysis def Tally {
		subject c : Craft;
		return n : Integer = c.sent;
	}
}
`

// A machine on an object nested in a declared assembly is explored on that object:
// each run instantiates the path's root and walks the path, so the connector exists.
func TestRunForExploresAMachineOnANestedObject(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	listen := Behavior{Name: "Comms::Ground::listen", Performer: []string{"Comms::pair.ground"}}
	verdicts := s.RunFor(nil, []Behavior{listen}, 5)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictHolds {
		t.Fatalf("verdicts = %+v, want one that holds", verdicts)
	}
	out := strings.Join(verdicts[0].Lines, "\n")
	wantsInOrder(t, out, "✓ explored Comms::Ground::listen: 2 outcomes", "received = 1", "received = 2", "complete (")
	if len(verdicts[0].Outcomes) != 2 {
		t.Errorf("outcomes = %+v, want the frame missed and the frame received", verdicts[0].Outcomes)
	}

	// Instantiating the nested part alone loses the assembly: nothing pings it.
	modes := Behavior{Name: "Comms::Craft::modes", Performer: []string{"Comms::Pair::craft"}}
	verdicts = s.RunFor(nil, []Behavior{modes}, 5)
	wants(t, strings.Join(verdicts[0].Lines, "\n"), "✓ explored Comms::Craft::modes: 1 outcome", "finalState waiting")

	// The qualified spelling reaches the same nested object, as the prompt reads it.
	verdicts = s.RunFor(nil, []Behavior{{Name: "Comms::Ground::listen", Performer: []string{"Comms::pair::ground"}}}, 5)
	if got := strings.Join(verdicts[0].Lines, "\n"); !strings.Contains(got, "2 outcomes") {
		t.Errorf("the qualified spelling of the path explored:\n%s", got)
	}
}

// An action performed by a nested object runs beside the machines the assembly's
// objects exhibit, and reads the features they write.
func TestRunForExploresAnActionOnANestedObject(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Comms::Craft::ack", "Comms::pair.craft")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Comms::Craft::ack: 2 outcomes", "seen = 1", "seen = 2", "complete (")

	// The craft instantiated on its own is never pinged, so it sends nothing.
	v = s.RunAction("Comms::Craft::ack", "Comms::Pair::craft")
	wants(t, strings.Join(v.Lines, "\n"), "✓ explored Comms::Craft::ack: 1 outcome", "seen = 0", "complete (1 runs)")
}

// A case's subject is named by a path as a performer is: each run makes the
// object of the path's root and runs the case on the one the path reaches.
func TestRunAnalysisExploresOnANestedSubject(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAnalysis("Comms::Tally Comms::pair.craft")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Comms::Tally: 1 outcome", "n = 0", "complete (1 runs)")
	v = s.RunAnalysis("Comms::Tally Comms::pair.tower")
	wants(t, strings.Join(v.Lines, "\n"), `Comms::pair has no feature "tower"`)
	v = s.RunAnalysis("Comms::Tally #1")
	wants(t, strings.Join(v.Lines, "\n"), (&ExploredObjectError{Ref: "#1"}).Error())
}

// Two behaviors on sibling parts of one root share the root's object in every
// run, so their outcomes agree about the frames the connector carried.
func TestRunForExploresSiblingsOnOneRoot(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	listen := Behavior{Name: "Comms::Ground::listen", Performer: []string{"Comms::pair.ground"}}
	modes := Behavior{Name: "Comms::Craft::modes", Performer: []string{"Comms::pair.craft"}}
	verdicts := s.RunFor(nil, []Behavior{listen, modes}, 5)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictHolds {
		t.Fatalf("verdicts = %+v, want one that holds", verdicts)
	}
	out := strings.Join(verdicts[0].Lines, "\n")
	wantsInOrder(t, out, "✓ explored Comms::Ground::listen Comms::pair.ground, Comms::Craft::modes Comms::pair.craft: 2 outcomes",
		"Comms::Craft::modes Comms::pair.craft.emitted = 4", "Comms::Ground::listen Comms::pair.ground.received = 1",
		"Comms::Craft::modes Comms::pair.craft.emitted = 4", "Comms::Ground::listen Comms::pair.ground.received = 2",
		"complete (4 runs)")
	if len(verdicts[0].Outcomes) != 2 {
		t.Errorf("outcomes = %+v, want two", verdicts[0].Outcomes)
	}
	// One witness names both objects on one clock: the root was instantiated once.
	if w := strings.Join(verdicts[0].Outcomes[0].Witness, "; "); !strings.Contains(w, "state machine listen of object #") || !strings.Contains(w, "state machine modes of object #") {
		t.Errorf("witness = %s, want both machines' choices", w)
	}

	// The same table under one job and under four.
	for _, jobs := range []int{1, 4} {
		if err := s.SetJobs(jobs); err != nil {
			t.Fatal(err)
		}
		again := s.RunFor(nil, []Behavior{listen, modes}, 5)
		if got := strings.Join(again[0].Lines, "\n"); got != out {
			t.Errorf("under %d jobs:\n%s\nwant\n%s", jobs, got, out)
		}
	}
}

// An object -instantiate created is given to the explored runs: each creates one of
// its declaration before the behaviors start, so a machine named alone attaches to
// the object exhibiting it and a path into the declaration reuses the object. The
// prompt's %instantiate creates the session's object alone, which no run sees.
func TestExploredRunsAreGivenTheObjectsInstantiated(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	listen := Behavior{Name: "Comms::Ground::listen"}
	none := (&ExhibitorsError{Machine: "Comms::Ground::listen", Types: []string{"Comms::Ground"}, Fresh: true}).Error()
	wants(t, strings.Join(s.RunFor(nil, []Behavior{listen}, 5)[0].Lines, "\n"), none)

	run(t, s, "%instantiate Comms::pair")
	wants(t, strings.Join(s.RunFor(nil, []Behavior{listen}, 5)[0].Lines, "\n"), none)

	if _, err := s.InstantiateReport("Comms::pair"); err != nil {
		t.Fatal(err)
	}
	verdicts := s.RunFor(nil, []Behavior{listen}, 5)
	if v := verdicts[0]; v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(verdicts[0].Lines, "\n"), "✓ explored Comms::Ground::listen: 2 outcomes", "received = 1", "received = 2")

	// The given root is the one a path into it walks: one pair per run, its craft
	// pinged by its ground, the same runs as with the ground named by its path,
	// which the machine's object is reported under.
	modes := Behavior{Name: "Comms::Craft::modes", Performer: []string{"Comms::pair.craft"}}
	given := s.RunFor(nil, []Behavior{listen, modes}, 5)[0]
	wantsInOrder(t, strings.Join(given.Lines, "\n"), "✓ explored Comms::Ground::listen, Comms::Craft::modes: 2 outcomes",
		"Comms::Ground::listen.received = 1; Comms::pair.craft.isSolid = true; Comms::pair.craft.sent = 4; Comms::pair.ground.isSolid = true",
		"Comms::Ground::listen.received = 2; Comms::pair.craft.isSolid = true; Comms::pair.craft.sent = 4; Comms::pair.ground.isSolid = true")
	alone := loadSource(t, exploreCommsSource)
	if err := alone.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	pathed := Behavior{Name: "Comms::Ground::listen", Performer: []string{"Comms::pair.ground"}}
	named := alone.RunFor(nil, []Behavior{pathed, modes}, 5)[0]
	for i, o := range given.Outcomes {
		if want := named.Outcomes[i]; strings.Join(o.Witness, "; ") != strings.Join(want.Witness, "; ") || o.Linearizations != want.Linearizations {
			t.Errorf("outcome %d given the pair: %d × %s\nwant, as with the ground named: %d × %s", i, o.Linearizations, o.Witness, want.Linearizations, want.Witness)
		}
	}

	// A second given object exhibiting the machine makes naming it alone
	// ambiguous, refused naming the run's objects as the prompt names the session's.
	if _, err := s.InstantiateReport("Comms::Fleet"); err != nil {
		t.Fatal(err)
	}
	wants(t, strings.Join(s.RunFor(nil, []Behavior{listen}, 5)[0].Lines, "\n"),
		`3 objects of the explored run exhibit "Comms::Ground::listen"`, `of "Comms::pair.ground"`, `of "Comms::Fleet.pairs[1].ground"`)

	// A given object lasts as long as the session holds it: a submission leaving its
	// declaration as it was keeps it, one changing the declaration drops it with the object.
	changed := strings.Replace(exploreCommsSource, "part def Fleet {", "part def Fleet {\n\t\tattribute name : String;", 1)
	if res := s.Submit(changed); len(res.Diagnostics) > 0 {
		t.Fatalf("resubmission has diagnostics: %v", res.Diagnostics)
	}
	if got := s.given; !slices.Equal(got, []string{"Comms::pair"}) {
		t.Errorf("given after the Fleet changed = %q, want the pair alone", got)
	}
	wantsInOrder(t, strings.Join(s.RunFor(nil, []Behavior{listen}, 5)[0].Lines, "\n"), "✓ explored Comms::Ground::listen: 2 outcomes", "received = 1", "received = 2")
}

// Each -instantiate gives the runs an object of its own, as it creates one of the
// session's: a declaration given twice is two objects per run, the name denoting
// the later and the earlier reached by id alone, so a machine named alone is
// ambiguous between them as the prompt's is.
func TestExploredRunsAreGivenOneObjectPerInstantiate(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := s.InstantiateReport("Comms::pair"); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.given; !slices.Equal(got, []string{"Comms::pair", "Comms::pair"}) {
		t.Fatalf("given after two -instantiate = %q, want the pair twice", got)
	}
	prompt := strings.Join(s.RunFor(nil, []Behavior{{Name: "Comms::Ground::listen"}}, 5)[0].Lines, "\n")
	wants(t, prompt, `2 objects of the explored run exhibit "Comms::Ground::listen"`, `of "Comms::pair.ground"`, `.ground"`)
	if strings.Count(prompt, ".ground\"") != 2 {
		t.Errorf("want the ground of each pair named, the displaced one's by id:\n%s", prompt)
	}

	// The path walks into the pair the name denotes: the run creates two, both
	// running, and the exploration tables the named one's machine over the runs
	// the second pair's choices multiply.
	pathed := Behavior{Name: "Comms::Ground::listen", Performer: []string{"Comms::pair.ground"}}
	wantsInOrder(t, strings.Join(s.RunFor(nil, []Behavior{pathed}, 5)[0].Lines, "\n"), "explored Comms::Ground::listen: 2 outcomes", "received = 1", "received = 2")
}

// exploreBayEquipmentSource is a ticker held required deep in a rack, and optionally
// in a bay, neither named by any behavior.
const exploreBayEquipmentSource = `
package Bay {
	private import ScalarValues::*;
	part def Ticker {
		attribute n : Integer = 0;
		exhibit state ticking { entry; then on; state on { entry assign n := n + 1; } }
	}
	part def Shelf { part ticker : Ticker; }
	part def Rack { part shelves : Shelf[2]; }
	part def Bay { part maybe : Ticker[0..1]; part rack : Rack; }
	part bay : Bay;
}
`

// A machine named alone finds the run's objects exhibiting it however deep a given
// object holds them, as every required part of a behaving type is created with its
// holder. An optional part is left absent, so the run has no object of it to find,
// as the prompt has none: the machine is named on a path to attach to one.
func TestExploredMachineNamedAloneFindsTheGivenObjectsRequiredParts(t *testing.T) {
	s := loadSource(t, exploreBayEquipmentSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstantiateReport("Bay::bay"); err != nil {
		t.Fatal(err)
	}
	ticking := Behavior{Name: "Bay::Ticker::ticking"}
	wants(t, strings.Join(s.RunFor(nil, []Behavior{ticking}, 1)[0].Lines, "\n"),
		`2 objects of the explored run exhibit "Bay::Ticker::ticking"`,
		`of "Bay::bay.rack.shelves[1].ticker"`, `of "Bay::bay.rack.shelves[2].ticker"`)
	prompt := loadSource(t, exploreBayEquipmentSource)
	run(t, prompt, "%instantiate Bay::bay")
	wants(t, run(t, prompt, "%state Bay::Ticker::ticking"),
		`2 objects of this session exhibit "Bay::Ticker::ticking"`, `of "Bay::bay.rack.shelves[1].ticker"`)

	v := s.RunFor(nil, []Behavior{{Name: "Bay::Ticker::ticking", Performer: []string{"Bay::bay.rack.shelves[2].ticker"}}}, 1)[0]
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Bay::Ticker::ticking: 1 outcome", "finalState on", "this.n = 1")

	wants(t, strings.Join(s.RunFor(nil, []Behavior{{Name: "Bay::Ticker::ticking", Performer: []string{"Bay::bay.maybe"}}}, 1)[0].Lines, "\n"),
		"error: maybe of Bay::bay holds no object")
}

// exploreTankSource is a part performing an action of its own that writes the
// part's attribute, held alone and twice over in a farm.
const exploreTankSource = `
package Tank {
	private import ScalarValues::*;
	part def Tank {
		attribute level : Integer = 0;
		perform action fill {
			first start;
			then action pour assign level := level + 1;
			then done;
		}
	}
	part tank : Tank;
	part def Farm {
		part tanks : Tank[2];
	}
}
`

// An action named alone attaches to the performance the run's one given object
// already runs of it, so the outcome is that object's: its attribute written once,
// not a second detached performance's. Several given objects performing it are refused
// naming them, as machines are.
func TestExploredActionNamedAloneAttachesToTheGivenObjectPerformingIt(t *testing.T) {
	s := loadSource(t, exploreTankSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	fill := Behavior{Name: "Tank::Tank::fill"}
	if _, err := s.InstantiateReport("Tank::tank"); err != nil {
		t.Fatal(err)
	}
	v := s.RunFor([]Behavior{fill}, nil, 1)[0]
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Tank::Tank::fill: 1 outcome", "this.level = 1", "complete (1 runs)")

	// Named on the object performing it, the action attaches to that performance too.
	v = s.RunFor([]Behavior{{Name: "Tank::Tank::fill", Performer: []string{"Tank::tank"}}}, nil, 1)[0]
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Tank::Tank::fill: 1 outcome", "this.level = 1", "complete (1 runs)")
	v = s.RunFor([]Behavior{{Name: "Tank::Tank::fill", Performer: []string{"Tank::Farm.tanks[2]"}}}, nil, 1)[0]
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "✓ explored Tank::Tank::fill: 1 outcome", "this.level = 1", "complete (1 runs)")

	if _, err := s.InstantiateReport("Tank::Farm"); err != nil {
		t.Fatal(err)
	}
	v = s.RunFor([]Behavior{fill}, nil, 1)[0]
	if v.Status != VerdictUnresolved {
		t.Errorf("status = %v, want unresolved:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wants(t, strings.Join(v.Lines, "\n"), `3 objects of the explored run perform "Tank::Tank::fill"`,
		`of "Tank::tank"`, `of "Tank::Farm.tanks[1]"`, `of "Tank::Farm.tanks[2]"`, "name one as Tank::Tank::fill <Assembly::part>")
	var refused *PerformersError
	if !errors.As(&PerformersError{}, &refused) {
		t.Fatal("PerformersError is not an error")
	}
}

// A path an exploration cannot follow is refused while the session is held: an
// object named by id, an unknown usage, an index on a scalar or off a fixed multiplicity.
func TestExploredPathsAreCheckedAgainstTheDeclarations(t *testing.T) {
	s := loadSource(t, exploreCommsSource)
	run(t, s, "%instantiate Comms::pair")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	listen := "Comms::Ground::listen"
	for performer, want := range map[string]string{
		"#1":                      (&ExploredObjectError{Ref: "#1"}).Error(),
		"#1.ground":               (&ExploredObjectError{Ref: "#1.ground"}).Error(),
		"Comms::pair.tower":       `Comms::pair has no feature "tower" (its features are craft, ground`,
		"Comms::pair.ground.p.x":  `Comms::pair.ground.p has no feature "x"`,
		"Comms::pair.ground[1]":   "ground of Comms::pair holds one value and takes no index: write ground, not ground[1]",
		"Comms::Fleet.pairs":      "pairs of Comms::Fleet holds 2 objects: pick one by index, pairs[1] to pairs[2]",
		"Comms::Fleet.pairs[3]":   "pairs of Comms::Fleet holds 2 objects, so pairs[3] names none (indexes run from 1 to 2)",
		"Comms::Fleet.count.x":    "count of Comms::Fleet holds a value, not an object",
		"Comms::nowhere.ground":   "unresolved reference: Comms::nowhere",
		"Comms::Ground::listen.x": "",
	} {
		verdicts := s.RunFor(nil, []Behavior{{Name: listen, Performer: []string{performer}}}, 5)
		if len(verdicts) != 1 || verdicts[0].Status != VerdictUnresolved {
			t.Errorf("%s: verdicts = %+v, want one unresolved", performer, verdicts)
			continue
		}
		out := strings.Join(verdicts[0].Lines, "\n")
		if !strings.Contains(out, want) {
			t.Errorf("%s:\n%s\nwant %q", performer, out, want)
		}
		if strings.Contains(out, "explored "+listen) {
			t.Errorf("%s was explored on:\n%s", performer, out)
		}
	}
	// The refusal of a session object does not claim every dotted name is refused.
	if msg := (&ExploredObjectError{Ref: "#1"}).Error(); !strings.Contains(msg, "a path from one to an object it holds") {
		t.Errorf("ExploredObjectError = %q", msg)
	}

	// An element of a multi-valued usage is an object of its own, reached by index.
	verdicts := s.RunFor(nil, []Behavior{{Name: listen, Performer: []string{"Comms::Fleet.pairs[2].ground"}}}, 5)
	wantsInOrder(t, strings.Join(verdicts[0].Lines, "\n"), "explored Comms::Ground::listen: 2 outcomes", "received = 1", "received = 2")

	// A usage the declarations allow to hold nothing is planned, and a run finding it
	// empty fails as a run does: an outcome of the table, not a refusal of the plan.
	verdicts = s.RunFor(nil, []Behavior{{Name: listen, Performer: []string{"Comms::Fleet.spare"}}}, 5)
	wantsInOrder(t, strings.Join(verdicts[0].Lines, "\n"), "explored Comms::Ground::listen: 1 outcome", "error: spare of Comms::Fleet holds no object", "complete (1 runs)")

	// A path is planned from the declarations, never from the objects the session holds.
	plan := s.planFresh("Comms::pair.ground", "Comms::pair.craft", "Comms::pair")
	for _, text := range []string{"Comms::pair.ground", "Comms::pair.craft", "Comms::pair"} {
		ref := plan.refs[text]
		if ref.err != nil || ref.fqn != "Comms::pair" || ref.label != text {
			t.Errorf("plan of %s = %+v", text, ref)
		}
	}
	if got := len(plan.refs["Comms::pair.ground"].path); got != 1 {
		t.Errorf("path of Comms::pair.ground has %d segments", got)
	}
	var pathErr *ObjectPathError
	if err := s.freshRef("Comms::pair.tower").err; err == nil || !errors.As(err, &pathErr) {
		t.Errorf("Comms::pair.tower = %v, want *ObjectPathError", err)
	}
}

// exploreLampSource has a machine and an action of one part due at one instant of
// the clock they share, so which runs first is a choice point.
const exploreLampSource = `
package Shared {
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

// RunFor under explore runs the behaviors named on one clock in every run, the
// executors due together drawn in every order, and tables the joint outcome; both
// behaviors performed by one declared part are performed by one object of it.
func TestRunForExploresEveryDueOrder(t *testing.T) {
	s := loadSource(t, exploreLampSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	peek := Behavior{Name: "Shared::Lamp::peek", Performer: []string{"Shared::Lamp"}}
	glow := Behavior{Name: "Shared::Lamp::glow", Performer: []string{"Shared::Lamp"}}
	verdicts := s.RunFor([]Behavior{peek}, []Behavior{glow}, 3)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictHolds {
		t.Fatalf("verdicts = %+v, want one that holds", verdicts)
	}
	wantsInOrder(t, strings.Join(verdicts[0].Lines, "\n"),
		"✓ explored Shared::Lamp::peek, Shared::Lamp::glow: 2 outcomes",
		`Shared::Lamp::glow finalState = "on"; Shared::Lamp::glow visits = "off, on"; Shared::Lamp::peek.saw = false; this.isSolid = true; this.lit = true | 1              | t=3.0: action peek of object #1 first of state machine glow of object #1, action peek of object #1`,
		`Shared::Lamp::glow finalState = "on"; Shared::Lamp::glow visits = "off, on"; Shared::Lamp::peek.saw = true; this.isSolid = true; this.lit = true  | 1              | t=3.0: state machine glow of object #1 first of state machine glow of object #1, action peek of object #1`,
		"complete (2 runs)")

	// One behavior explored with a duration is its own outcome, as RunStateMachine tables it.
	verdicts = s.RunFor(nil, []Behavior{glow}, 3)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictHolds {
		t.Fatalf("verdicts = %+v, want one that holds", verdicts)
	}
	wants(t, strings.Join(verdicts[0].Lines, "\n"), "✓ explored Shared::Lamp::glow: 1 outcome",
		"finalState on; visits off, on; this.isSolid = true; this.lit = true | 1              | no choice points", "complete (1 runs)")

	// A behavior that does not resolve is reported and nothing is explored.
	verdicts = s.RunFor([]Behavior{{Name: "Shared::Lamp::nothing"}}, []Behavior{glow}, 3)
	if len(verdicts) != 1 || verdicts[0].Status != VerdictUnresolved {
		t.Fatalf("verdicts = %+v, want one unresolved", verdicts)
	}
}

// exploreRegionsSource has a machine starting in a parallel state whose two
// regions each log their own entry and their start state's.
const exploreRegionsSource = `
package Regions {
	private import ScalarValues::*;
	state def Pair {
		attribute log : String = "";
		entry; then work;
		state work parallel {
			state left {
				entry { assign log := log + "left "; } then l;
				state l { entry { assign log := log + "l "; } }
			}
			state right {
				entry { assign log := log + "right "; } then r;
				state r { entry { assign log := log + "r "; } }
			}
		}
	}
}
`

// RunStateMachine under explore draws the order a parallel state's regions are
// entered in one unit at a time: the six linearizations of two chains of two
// are tabled, each witnessed by its entry-order lines, and the table is the same
// on every run.
func TestRunStateMachineExploresEveryRegionEntryOrder(t *testing.T) {
	s := loadSource(t, exploreRegionsSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunStateMachine("Regions::Pair")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	out := strings.Join(v.Lines, "\n")
	wantsInOrder(t, out,
		"✓ explored Regions::Pair: 6 outcomes",
		"outcome                                                    | linearizations | witness",
		`finalState l+r; visits work, l, r; log = "left l right r " | 1              | entering work: left(entry) first of left(entry), right(entry); entering work: l(entry) first of l(entry), right(entry)`,
		`finalState l+r; visits work, l, r; log = "left right l r " | 1              | entering work: left(entry) first of left(entry), right(entry); entering work: right(entry) first of l(entry), right(entry); entering work: l(entry) first of l(entry), r(entry)`,
		`finalState l+r; visits work, l, r; log = "right left l r " | 1              | entering work: right(entry) first of left(entry), right(entry); entering work: left(entry) first of left(entry), r(entry); entering work: l(entry) first of l(entry), r(entry)`,
		`finalState l+r; visits work, r, l; log = "left right r l " | 1              | entering work: left(entry) first of left(entry), right(entry); entering work: right(entry) first of l(entry), right(entry); entering work: r(entry) first of l(entry), r(entry)`,
		`finalState l+r; visits work, r, l; log = "right left r l " | 1              | entering work: right(entry) first of left(entry), right(entry); entering work: left(entry) first of left(entry), r(entry); entering work: r(entry) first of l(entry), r(entry)`,
		`finalState l+r; visits work, r, l; log = "right r left l " | 1              | entering work: right(entry) first of left(entry), right(entry); entering work: r(entry) first of left(entry), r(entry)`,
		"complete (6 runs)")
	if again := s.RunStateMachine("Regions::Pair"); strings.Join(again.Lines, "\n") != out {
		t.Errorf("exploration rendered\n%s\nthen\n%s", out, strings.Join(again.Lines, "\n"))
	}
}

// %schedule explore is a typed error at the prompt, leaving the policy in force,
// and a session set to explore programmatically refuses to start a debugger
// while still answering %schedule.
func TestSchedulingExploreAtThePromptIsRefused(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%schedule declared")
	out := run(t, s, "%schedule explore:runs=8")
	wants(t, out, "error: explore:runs=8 replays a behavior from the start once per linearization",
		"run `sysml -schedule explore:runs=8 -action <name>`")
	rejects(t, out, "schedule: ")
	wants(t, run(t, s, "%schedule"), "schedule: declared")

	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	wants(t, run(t, s, "%schedule"), "schedule: explore")
	wants(t, run(t, s, "%action tally"), "error: explore replays a behavior from the start")
	wants(t, run(t, s, "%state tally"), "error: explore replays a behavior from the start")
	if s.actionExec != nil || s.stateExec != nil {
		t.Error("a debugger started under explore")
	}
	var typed *ExploreAtPromptError
	if _, err := s.startAction("tally", nil); !errors.As(err, &typed) {
		t.Errorf("startAction under explore = %v, want *ExploreAtPromptError", err)
	}
}

// A session set to explore keeps its own context on the default policy, and a
// debugger running when explore is set is left alone.
func TestSetScheduleExploreLeavesTheSessionContextDriven(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%trace on")
	run(t, s, "%action tally")
	run(t, s, "%step")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	if got := s.rtCtx.Schedule(); got != runtime.DefaultSchedulePolicy {
		t.Errorf("session context schedule = %s, want the default", got)
	}
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "took 3@right first")
	wants(t, run(t, s, "%continue"), "✓ Action completed")
}

// Malformed explore spellings are refused with a typed error naming the option.
func TestSetScheduleRejectsMalformedExplore(t *testing.T) {
	for spelling, reason := range map[string]string{
		"explore:":                "explore: needs runs=<n> and/or depth=<d> after the colon, or no colon",
		"explore:bogus":           `explore option "bogus" is not runs=<n> or depth=<d>`,
		"explore:runs=0":          `explore runs "0" is not a decimal integer of at least 1`,
		"explore:depth=-1":        `explore depth "-1" is not a decimal integer of at least 0`,
		"explore:runs=2,runs=3":   "explore option runs is given twice",
		"explore:runs=x":          `explore runs "x" is not a decimal integer of at least 1`,
		"explores":                "want one of declared, reverse, seed:<n>, explore[:runs=<n>,depth=<d>]",
		"explore:runs=1,depth=1,": `explore option "" is not runs=<n> or depth=<d>`,
	} {
		_, err := runtime.ParseSchedulePolicy(spelling)
		var typed *runtime.SchedulePolicyError
		if !errors.As(err, &typed) {
			t.Errorf("%s: %v, want *SchedulePolicyError", spelling, err)
			continue
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: %v, want %q", spelling, err, reason)
		}
		s := loadSource(t, choiceForkSource)
		wants(t, run(t, s, "%schedule "+spelling), "error: invalid scheduling policy \""+spelling+"\"")
	}
}
