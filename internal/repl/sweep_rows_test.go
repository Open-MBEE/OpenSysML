package repl

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// sweepRowsModel declares what the parallel-row tests run: a case whose body writes a
// feature of its subject, an operation that writes a held object, a part reached only
// through another with a case nested under it, a part whose type exhibits a state whose
// transition writes a feature, one whose state runs a body that waits on the clock, a
// calc whose work grows steeply with its input, and a calc and a case taking arguments.
const sweepRowsModel = `package Rows {
	private import ScalarValues::*;
	private import SI::*;
	attribute def Lit;
	part def Ship {
		attribute cost : Real = 5.0;
		action bump {
			first add;
			action add { assign cost := cost + 2.0; }
		}
	}
	part ship : Ship;
	part def Fleet {
		part flagship : Ship {
			analysis audit : Bump { subject s = flagship; }
		}
	}
	part fleet : Fleet;
	part def Beacon :> Ship {
		exhibit state blinking {
			entry; then off;
			state off;
			transition first off accept Lit do assign cost := cost + 10.0 then on;
			state on;
			transition first on accept after 5 [s] then off;
		}
	}
	part beacon : Beacon;
	part def Watcher :> Ship {
		exhibit state w {
			entry; then watching;
			state watching {
				do action poll {
					first start;
					then action wait accept after 3 [s];
					then done;
				}
			}
		}
	}
	part watcher : Watcher;
	analysis def Bump {
		subject s : Ship;
		in tax : Real;
		action raise { assign s.cost := s.cost + tax; }
		out total : Real = s.cost;
	}
	calc def Fib {
		in n : Integer;
		return : Integer = if n < 2 ? n else Fib(n - 1) + Fib(n - 2);
	}
	calc def Price {
		in base : Real;
		in n : Real;
		return : Real = base * n;
	}
	analysis def Compare {
		subject s : Ship;
		in rival : Ship;
		in scale : Real;
		action raise { assign rival.cost := rival.cost + 1.0; }
		out total : Real = (s.cost + rival.cost) * scale;
	}
}`

// submits adds a declaration to a session that already holds one.
func submits(t *testing.T, s *Session, src string) {
	t.Helper()
	if errs := errorDiagnostics(s.Submit(src).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
}

// cellPadding matches the padding before a column rule, which in the time column and
// its header follows the width of the longest time run.
var cellPadding = regexp.MustCompile(` +\|`)

// onOneJobAndOnEight runs a command on one job and on eight and returns both tables,
// run times and the padding they set masked.
func onOneJobAndOnEight(t *testing.T, s *Session, command string) (string, string) {
	t.Helper()
	run(t, s, "%jobs 1")
	one := tableOf(run(t, s, command))
	run(t, s, "%jobs 8")
	eight := tableOf(run(t, s, command))
	return one, eight
}

// Every sweep the tests run — calcs, cases on a subject, a failing row, a trade study, a
// sample, a descending range whose rows arrive out of plan order — reports the same
// table on eight jobs as on one: rows in plan order, the same values, verdicts,
// evaluations and errors.
func TestSweepRowsReadAlikeOnOneJobAndOnEight(t *testing.T) {
	s := sweepSession(t)
	submits(t, s, tradeStudyModel)
	submits(t, s, sweepRowsModel)
	run(t, s, "%instantiate Sw::ship")
	for _, command := range []string{
		"%sweep Sw::Twice n=1..4",
		"%sweep Sw::Plus a=1..2 b=10..12:2",
		"%sweep Sw::Priced Sw::ship tax=0.0..1.0:0.5",
		"%sweep Sw::Ratio(a = 4.0) b=-1..1",
		"%sweep Sw::Reach(t = 2.0) v=0.0..10.0:5.0",
		"%samples 5 7 Sw::Twice n=1..100",
		"%sweep Trade::weighted powerWeight=0.0..0.2:0.1",
		"%sweep Trade::failing divisor=1.0..2.0:1.0",
		"%sweep Rows::Fib n=20..8:-1",
	} {
		one, eight := onOneJobAndOnEight(t, s, command)
		if one != eight {
			t.Errorf("%s reads differently on eight jobs than on one:\n%s\nagainst\n%s", command, eight, one)
		}
		if !strings.Contains(one, "run(s)") || strings.Contains(one, "unresolved") {
			t.Errorf("%s did not run:\n%s", command, one)
		}
	}
	one, _ := onOneJobAndOnEight(t, s, "%sweep Rows::Fib n=20..8:-1")
	wantsInOrder(t, one, "20 | 6765", "8 | 21")
}

// A case whose body writes a feature of its subject sees the declaration's value on
// every row: no row observes another's write, and the held object the sweep was named
// on is as it was before, on one job and on eight.
func TestSweepRowsWritingTheSubjectKeepTheWritesToThemselves(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::ship")
	wants(t, run(t, s, "%features Rows::ship"), "cost = 5.0")
	want := strings.Join([]string{
		"sweep Rows::Bump — 3 run(s)",
		"tax | total | time",
		"-+-+-",
		"1.0 | 6.0 | <time>",
		"2.0 | 7.0 | <time>",
		"3.0 | 8.0 | <time>",
		"  standing: table (observed: 3 rows)",
	}, "\n")
	one, eight := onOneJobAndOnEight(t, s, "%sweep Rows::Bump Rows::ship tax=1.0..3.0:1.0")
	if one != want || eight != want {
		t.Errorf("tables are\n%s\nand\n%s\nwant\n%s", one, eight, want)
	}
	wants(t, run(t, s, "%features Rows::ship"), "ID: 1", "cost = 5.0")
}

// tableOf is a sweep's table with its run times and the padding they set masked.
func tableOf(out string) string { return cellPadding.ReplaceAllString(sweepTable(out), " |") }

// sweepsAlike runs a sweep on one job and on eight and checks both tables read the same
// and hold the fragments.
func sweepsAlike(t *testing.T, s *Session, command string, fragments ...string) string {
	t.Helper()
	one, eight := onOneJobAndOnEight(t, s, command)
	if one != eight {
		t.Errorf("%s reads differently on eight jobs than on one:\n%s\nagainst\n%s", command, eight, one)
	}
	wants(t, one, fragments...)
	return one
}

// A sweep over a held object no longer as its declaration made it — named by its
// identity, written by a run, or reached through an object written by a run — runs each
// row on an object made from an image of the held graph as the sweep found it, so the
// rows read as the prompt's run on the held object would, on one job and on eight, and
// the held objects stay as they were. One whose state machine stands where its start
// left it is as its declaration made it, and sweeps from the declaration.
func TestSweepOverAnObjectNotAsItsDeclarationMadeItRunsOnItsImage(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::ship")
	run(t, s, "%instantiate Rows::Fleet")
	run(t, s, "%instantiate Rows::beacon")
	fresh := []string{"3 run(s)", "1.0 | 6.0 |", "2.0 | 7.0 |", "3.0 | 8.0 |"}
	sweepsAlike(t, s, "%sweep Rows::Bump #1 tax=1.0..3.0:1.0", fresh...)
	wants(t, run(t, s, "%features Rows::ship"), "ID: 1", "cost = 5.0")
	sweepsAlike(t, s, "%sweep Rows::Bump Rows::beacon tax=1.0..3.0:1.0", fresh...)
	wants(t, run(t, s, "%features Rows::beacon"), "cost = 5.0", "current state off")

	wants(t, run(t, s, "%invoke Rows::ship bump"), "Invoked bump on object #1")
	before := run(t, s, "%features Rows::ship all json")
	wants(t, before, `"realValue": 7`)
	bumped := []string{"3 run(s)", "1.0 | 8.0 |", "2.0 | 9.0 |", "3.0 | 10.0 |"}
	for _, object := range []string{"Rows::ship", "#1", "ship"} {
		sweepsAlike(t, s, "%sweep Rows::Bump "+object+" tax=1.0..3.0:1.0", bumped...)
	}
	if after := run(t, s, "%features Rows::ship all json"); after != before {
		t.Errorf("the sweeps left the held ship as\n%s\nwas\n%s", after, before)
	}

	wants(t, run(t, s, "%invoke Rows::Fleet.flagship bump"), "Invoked bump on object #")
	before = run(t, s, "%features Rows::Fleet all json")
	held := s.heldIDs()
	for _, command := range []string{
		"%sweep Rows::Bump Rows::Fleet.flagship tax=1.0..3.0:1.0",
		"%sweep Rows::Fleet::flagship::audit tax=1.0..3.0:1.0",
	} {
		sweepsAlike(t, s, command, bumped...)
	}
	if after := run(t, s, "%features Rows::Fleet all json"); after != before {
		t.Errorf("the sweeps left the held fleet as\n%s\nwas\n%s", after, before)
	}
	if ids := s.heldIDs(); !slices.Equal(ids, held) {
		t.Errorf("the sweeps changed the session's held objects: %v, was %v", ids, held)
	}

	// The fleet is no Ship: its rows fail as the prompt's run on it does.
	mismatch := `argument for parameter "s": type mismatch`
	table := sweepsAlike(t, s, "%sweep Rows::Bump Rows::Fleet tax=1.0..3.0:1.0", "3 run(s)", "error 1: analysis Rows::Bump: "+mismatch, "error 3:")
	rejects(t, table, "| 6.0 |")
	wants(t, run(t, s, "%analysis Rows::Bump(tax=1.0) Rows::Fleet"), mismatch)

	// The prompt's own run writes the held object, as a sweep never does.
	wants(t, run(t, s, "%analysis Rows::Bump(tax=1.0) Rows::ship"), "total = 8.0")
	wants(t, run(t, s, "%features Rows::ship"), "cost = 8.0")
}

// A held object whose state machine has moved — a transition fired, writing a feature,
// and a timer set by the state entered — is swept from an image of that state: each row
// starts where the machine stands, on one job and on eight, as the prompt's run on the
// held object does, and the held object is byte for byte as it was before the sweep,
// with a debugger attached to its machine.
func TestSweepOverAMovedStateMachineRunsOnItsImage(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::beacon")
	wants(t, run(t, s, "%send Lit to beacon"), "✓ Sent Lit to object #1", "transition off -> on fires on it")
	wants(t, run(t, s, "%state beacon"), "Current state: off")
	wants(t, run(t, s, "%advance 1"), "Current state: on", "t=5.0: state machine blinking of object #1, time -> off")
	before := run(t, s, "%features Rows::beacon all json")
	wants(t, before, `"realValue": 15`)
	wants(t, run(t, s, "%features Rows::beacon"), "cost = 15.0", "current state on")
	held := s.heldIDs()

	lit := []string{"3 run(s)", "1.0 | 16.0 |", "2.0 | 17.0 |", "3.0 | 18.0 |"}
	for _, object := range []string{"Rows::beacon", "#1"} {
		sweepsAlike(t, s, "%sweep Rows::Bump "+object+" tax=1.0..3.0:1.0", lit...)
	}
	if after := run(t, s, "%features Rows::beacon all json"); after != before {
		t.Errorf("the sweeps left the held beacon as\n%s\nwas\n%s", after, before)
	}
	wants(t, run(t, s, "%current"), "Current state: on")
	wants(t, run(t, s, "%events"), "Event queue: 1 events")
	if ids := s.heldIDs(); !slices.Equal(ids, held) {
		t.Errorf("the sweeps changed the session's held objects: %v, was %v", ids, held)
	}
	wants(t, run(t, s, "%analysis Rows::Bump(tax=1.0) Rows::beacon"), "total = 16.0")
	wants(t, run(t, s, "%advance 5"), "Current state: off")
}

// A held object whose state cannot be imaged — its machine runs a body paused at a wait
// inside a statement — refuses the sweep with the typed reason, names the object, runs no
// row and leaves the object as it is. Fresh from its declaration the same object sweeps,
// since the rows then make their own from the declaration.
func TestSweepOverAnObjectNoImageCarriesIsRefused(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::watcher")
	sweepsAlike(t, s, "%sweep Rows::Bump Rows::watcher tax=1.0..3.0:1.0", "3 run(s)", "1.0 | 6.0 |", "3.0 | 8.0 |")
	wants(t, run(t, s, "%invoke Rows::watcher bump"), "Invoked bump on object #1")
	before := run(t, s, "%features Rows::watcher all json")
	for _, object := range []string{"Rows::watcher", "#1"} {
		out := run(t, s, "%sweep Rows::Bump "+object+" tax=1.0..3.0:1.0")
		wants(t, out, "error: "+object+": image of object #1 (watcher): exhibited state machine w: snapshot of a body paused mid-statement: do behavior of state watching of w; each row of a sweep runs on an object of its own", "this one cannot be imaged")
		rejects(t, out, "run(s)")
	}
	if after := run(t, s, "%features Rows::watcher all json"); after != before {
		t.Errorf("the refused sweeps left the held watcher as\n%s\nwas\n%s", after, before)
	}
	wants(t, run(t, s, "%features Rows::watcher"), "cost = 7.0", "current state watching")
}

// A sweep over an object reached through another's feature — as its subject, or as the
// owner of a case nested under it — runs each row on an object of the root's
// declaration walked along the same features in the row's context, so the rows read
// as the prompt's run on the held object would, and the held objects stay as they were.
func TestSweepOverAnObjectReachedThroughAnotherRunsOnItsLike(t *testing.T) {
	for _, tc := range []struct{ name, command string }{
		{"as subject", "%sweep Rows::Bump Rows::Fleet.flagship tax=1.0..3.0:1.0"},
		{"as the owner of a nested case", "%sweep Rows::Fleet::flagship::audit tax=1.0..3.0:1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := loadSource(t, sweepRowsModel)
			run(t, s, "%instantiate Rows::Fleet")
			wants(t, run(t, s, "%features Rows::Fleet.flagship"), "ID: 2", "cost = 5.0")
			one, eight := onOneJobAndOnEight(t, s, tc.command)
			for _, table := range []string{one, eight} {
				wants(t, table, "3 run(s)", "1.0 | 6.0 |", "2.0 | 7.0 |", "3.0 | 8.0 |")
			}
			if one != eight {
				t.Errorf("tables differ on one job\n%s\nand on eight\n%s", one, eight)
			}
			wants(t, run(t, s, "%features Rows::Fleet.flagship"), "ID: 2", "cost = 5.0")
			if ids := s.heldIDs(); len(ids) != 2 {
				t.Errorf("the session holds %v, want the fleet and its flagship alone", ids)
			}
		})
	}
}

// A sweep's arguments are evaluated once, where the prompt evaluates them, and their
// values carried into every row: a feature a run wrote on a held object reads as
// written, not as its declaration would make it afresh.
func TestSweepArgumentsReadAsThePromptReadsThem(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::ship")
	wants(t, run(t, s, "%invoke Rows::ship bump"), "Invoked bump on object #1")
	wants(t, run(t, s, "%features Rows::ship"), "cost = 7.0")
	one, eight := onOneJobAndOnEight(t, s, "%sweep Rows::Price(base = ship.cost) n=1.0..2.0:1.0")
	for _, table := range []string{one, eight} {
		wants(t, table, "2 run(s)", "1.0 | 7.0 |", "2.0 | 14.0 |")
	}
	if one != eight {
		t.Errorf("tables differ on one job\n%s\nand on eight\n%s", one, eight)
	}
	wants(t, run(t, s, "%features Rows::ship"), "cost = 7.0")
}

// An argument naming a held object binds, in each row, the object the row makes for
// it — the same one the row's subject or owner is when they coincide — so a row's
// writes through it stay in the row; an object no row can make refuses the sweep.
func TestSweepArgumentsNamingAnObjectBindTheRowsOwn(t *testing.T) {
	s := loadSource(t, sweepRowsModel)
	run(t, s, "%instantiate Rows::ship")
	run(t, s, "%instantiate Rows::fleet")
	wants(t, run(t, s, "%features Rows::fleet.flagship"), "ID: 3", "cost = 5.0")
	one, eight := onOneJobAndOnEight(t, s, "%sweep Rows::Compare(rival = fleet.flagship) Rows::ship scale=1.0..2.0:1.0")
	for _, table := range []string{one, eight} {
		wants(t, table, "2 run(s)", "1.0 | 11.0 |", "2.0 | 22.0 |")
	}
	if one != eight {
		t.Errorf("tables differ on one job\n%s\nand on eight\n%s", one, eight)
	}
	wants(t, run(t, s, "%features Rows::fleet.flagship"), "ID: 3", "cost = 5.0")
	if ids := s.heldIDs(); len(ids) != 3 {
		t.Errorf("the session holds %v, want the ship, the fleet and its flagship alone", ids)
	}

	out := run(t, s, "%sweep Rows::Compare(rival = ship) Rows::ship scale=1.0..2.0:1.0")
	wants(t, out, "2 run(s)", "1.0   | 12.0  |", "2.0   | 24.0  |")
	wants(t, run(t, s, "%features Rows::ship"), "cost = 5.0")

	// An argument naming an object a run wrote binds the row's copy of it, imaged as
	// the sweep found it; the write of the row's case stays with the copy.
	wants(t, run(t, s, "%invoke Rows::fleet.flagship bump"), "Invoked bump on object #3")
	one, eight = onOneJobAndOnEight(t, s, "%sweep Rows::Compare(rival = fleet.flagship) Rows::ship scale=1.0..2.0:1.0")
	for _, table := range []string{one, eight} {
		wants(t, table, "2 run(s)", "1.0 | 13.0 |", "2.0 | 26.0 |")
	}
	if one != eight {
		t.Errorf("tables differ on one job\n%s\nand on eight\n%s", one, eight)
	}
	wants(t, run(t, s, "%features Rows::fleet.flagship"), "ID: 3", "cost = 7.0")
	wants(t, run(t, s, "%features Rows::ship"), "cost = 5.0")
	out = run(t, s, "%sweep Rows::Price(base = fleet.flagship.cost) n=1.0..2.0:1.0")
	wants(t, out, "2 run(s)", "1.0 | 7.0    |", "2.0 | 14.0   |")
}

// A sweep leaves a debugging session under way where it was: its rows run in contexts
// of their own, not in the one the debugger steps.
func TestSweepLeavesADebuggerStepping(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	submits(t, s, sweepRowsModel)
	run(t, s, "%action tally")
	wants(t, run(t, s, "%step"), "Step complete")
	if _, eight := onOneJobAndOnEight(t, s, "%sweep Rows::Fib n=1..6"); !strings.Contains(eight, "6 run(s)") {
		t.Fatalf("sweep did not run:\n%s", eight)
	}
	if s.actionExec == nil {
		t.Fatal("the sweep ended the action debugging session")
	}
	wants(t, run(t, s, "%step"), "Step complete")
}
