package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// migrateXMI migrates one of the fixtures under testdata/xmi, checking its
// notation and report against the goldens beside it (rewritten with -update).
func migrateXMI(t *testing.T, name string) *migrate.Result {
	t.Helper()
	path := "testdata/xmi/" + name
	data, err := os.ReadFile(path + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate(name+".xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	checkGolden(t, path+".golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, path+".golden.report.txt", report.Bytes())
	return r
}

// The acquisition fixture is a workflow in the shape of a telescope acquisition:
// the observatory block owns the activity, whose swimlane represents its
// telescope control part. Inside the lane JavaScript actions count the attempts
// against the part's Retries and flag success; the decision's guards are
// JavaScript and English; the observatory's own timer is stamped from the clock
// variable at each end; a duration observation spans two nodes and a duration
// constraint names a property of the lane's part as its length. The lane's
// names are read through the part, the clock variable through the local clock,
// and the loop runs until i reaches Retries.
func TestSwimlaneBodiesAndGuardsRunAgainstTheRepresentedPart(t *testing.T) {
	r := migrateXMI(t, "acquisition")
	for _, line := range []string{
		"assign this.Time_Acq_Total := localClock.currentTime;",
		"assign this.tcs.i := 1;",
		"assign this.tcs.GS_Found := false;",
		"assign this.tcs.GS_Found := true;",
		"assign this.tcs.i := this.tcs.i + 1;",
		"assign this.Time_Acq_Total := localClock.currentTime - this.Time_Acq_Total;",
		"if this.tcs.i >= this.tcs.Retries",
		"if not this.tcs.GS_Found and this.tcs.i < this.tcs.Retries",
		"action wait accept after this.tcs.ditSetup [SI::s];",
		"action attempt;",
		"attribute Time_Loop : ScalarValues::Real default = 0.0;",
		":= localClock.currentTime - 'Time_Loop start';",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_set", migrate.Mapped, "the JavaScript body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_stamp0", migrate.Mapped, "the clock variable reads the local clock")
	wantNote(t, r, "_e5", migrate.Mapped, "the JavaScript body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_e6", migrate.Mapped, "the English body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_lane", migrate.Mapped, "read as this.tcs")
	wantNote(t, r, "_attempt", migrate.Mapped, "a step with a duration and no further behavior")
	wantNote(t, r, "_dc", migrate.Approximated, `the duration "ditSetup s" is read as the expression this.tcs.ditSetup, in seconds`)
	wantNote(t, r, "_span", migrate.Mapped, "is assigned to the attribute Time_Loop, in seconds")
	wantNote(t, r, "_single", migrate.Mapped, "from the start of 'attempt' to the end of 'attempt' is assigned to the attribute Time_Attempt, in seconds")
	wantNote(t, r, "_astray", migrate.Unmapped, "is not a node of the activity")
	wantNote(t, r, "_blank", migrate.Unmapped, "observes no event")

	s := session(t, r)
	meta(t, s, "%instantiate Observatory")
	meta(t, s, "%seed 1")
	v := s.RunAction("Observatory::Acquire", "Observatory")
	wantVerdict(t, v)
	runs := strings.Join(s.RunRuns("Observatory::Acquire", []string{"Observatory"}, 5, 1, []string{"this.Time_Acq_Total", "Time_Loop", "Time_Attempt"}).Lines, "\n")
	// Four attempts of ditSetup = 2.5 s each: the loop ran until i reached Retries.
	for _, want := range []string{
		"this.Time_Acq_Total: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Loop: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Attempt: 5 run(s), min 2.5, mean 2.5, max 2.5",
	} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
	if strings.Contains(runs, "error") {
		t.Errorf("runs report an error:\n%s", runs)
	}
}

// The plant fixture is an activity with partitions of every shape: an outer lane
// representing a part and, nested in it, one representing a part of that part;
// a lane representing the context classifier; one representing nothing; one
// whose represents refers outside the document; a node in no lane. The nodes
// are listed on the lanes in both encodings, the lane's node list and the
// node's inPartition.
func TestPartitionsOfEveryShapeResolveNames(t *testing.T) {
	r := migrateXMI(t, "plant")
	for _, line := range []string{
		"assign this.tank.valve.open := true;",
		"assign this.tank.volume := this.tank.volume * 2 + (if this.tank.valve.open ? 1 else 0);",
		"assign this.runs := this.runs + 1;",
		"assign this.level := this.tank.volume;",
		"assign this.level := this.level + 1;",
		"assign this.level := this.level + this.tank.volume;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_inner", migrate.Mapped, "the partition represents valve of the enclosing partition's object, read as this.tank.valve")
	wantNote(t, r, "_outer", migrate.Mapped, "the partition represents the context's tank, read as this.tank")
	wantNote(t, r, "_self", migrate.Mapped, "the partition represents the context object itself, a Plant")
	wantNote(t, r, "_unset", migrate.Approximated, "the partition represents nothing")
	wantNote(t, r, "_gone", migrate.Approximated, "the partition represents an element outside the document")
	wantNote(t, r, "_idle", migrate.Approximated, "read as this.tank")
	wantNote(t, r, "_openv", migrate.Mapped, "names resolve against valve of the enclosing partition's object, read as this.tank.valve")
	wantNote(t, r, "_note", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_free", migrate.Mapped, "the JavaScript body is translated to v2")

	s := session(t, r)
	meta(t, s, "%instantiate Plant")
	wantVerdict(t, s.RunAction("Plant::Fill", "Plant"))
	runs := strings.Join(s.RunRuns("Plant::Fill", []string{"Plant"}, 1, 1, []string{"this.level", "this.runs"}).Lines, "\n")
	for _, want := range []string{"this.level: 1 run(s), min 7.0", "this.runs: 1 run(s), min 1"} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
}

// The reactor fixture has a configured clock variable named other than the
// default, a JavaScript function behavior and defaults the translator writes,
// and bodies of each kind the translator refuses, kept as comments with a
// typed reason.
func TestTranslatorRefusalsAndConfiguredClockName(t *testing.T) {
	r := migrateXMI(t, "reactor")
	for _, line := range []string{
		"calc def Energy {",
		"RealFunctions::max(E_0 - P * dt, 0)",
		"attribute limit : ScalarValues::Real default = RealFunctions::min(power * 2, 5);",
		"assign this.started := localClock.currentTime;",
		"attribute k : ScalarValues::Integer;",
		"assign n := n * k;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign this.power := this.label + 1;")
	wantNoLine(t, r.Notation, "assign x := x * 2;")
	wantNoLine(t, r.Notation, "assign a := a + step;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_energy", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_limit", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_mark", migrate.Mapped, "the clock variable reads the local clock")
	wantNote(t, r, "_double", migrate.Mapped, "the JavaScript body is translated to v2")
	for id, want := range map[string]string{
		"_scale":   `the construct "x" is outside the translated subset: an in parameter is not assigned`,
		"_shift":   `the construct "a" is outside the translated subset: an in parameter is not assigned`,
		"_loop":    `the construct "for" is outside the translated subset`,
		"_alloc":   `the construct "new" is outside the translated subset`,
		"_trim":    `the call "label.trim" is not in the translated function table`,
		"_self":    `the name "this.owner.power" resolves to nothing readable: Reactor has no feature owner`,
		"_stray":   `the name "nowhere" resolves to nothing readable`,
		"_mixed":   `the construct "+" is outside the translated subset: string concatenation has no v2 form in the subset`,
		"_python":  `the Python body is written as v2 assignments`,
		"_partial": `the call "label.trim" is not in the translated function table`,
	} {
		wantNote(t, r, id, migrate.Approximated, want)
	}
}
