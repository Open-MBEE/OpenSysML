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
	wantNote(t, r, "_stamp0", migrate.Mapped, "the clock variable simtime, named by 2 simulation configurations, reads the local clock")
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
// node's inPartition. Two lanes are dimensions: a node in both, whose objects
// differ, resolves through neither, and so does the guard of an edge leaving
// it, though the edge's target resolves; a node in a dimension and a lane
// representing nothing, or two lanes representing the same object, resolves.
func TestPartitionsOfEveryShapeResolveNames(t *testing.T) {
	r := migrateXMI(t, "plant")
	for _, line := range []string{
		"assign this.tank.valve.open := true;",
		"assign this.tank.volume := this.tank.volume * 2 + (if this.tank.valve.open ? 1 else 0);",
		"assign this.runs := this.runs + 1;",
		"assign this.level := this.tank.volume;",
		"assign this.level := this.level + 1;",
		"assign this.level := this.level + this.tank.volume;",
		"assign this.pump.on := true;",
		"assign this.tank.volume := this.tank.volume + 1;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign this.tank.volume := 0;")
	wantNoLine(t, r.Notation, "assign this.pump.volume := 0;")
	wantNoLine(t, r.Notation, "if this.pump.on")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_f3a", migrate.Approximated, `the guard [{JavaScript} on] is kept as a comment and the edge written unguarded: the name "on" resolves to nothing readable: nothing visible from Plant::Fill::<ControlFlow> is called on; its source 'torn' is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition`)
	wantNote(t, r, "_inner", migrate.Mapped, "the partition represents valve of the enclosing partition's object, read as this.tank.valve")
	wantNote(t, r, "_outer", migrate.Mapped, "the partition represents the context's tank, read as this.tank")
	wantNote(t, r, "_self", migrate.Mapped, "the partition represents the context object itself, a Plant")
	wantNote(t, r, "_unset", migrate.Approximated, "the partition represents nothing")
	wantNote(t, r, "_gone", migrate.Approximated, "the partition represents an element outside the document")
	wantNote(t, r, "_idle", migrate.Mapped, "the partition represents the context's tank, read as this.tank")
	wantNote(t, r, "_spare", migrate.Approximated, "read as this.tank, but nothing in it names the object's features")
	wantNote(t, r, "_pumps", migrate.Mapped, "read as this.pump; 'torn' it holds are also in a partition representing another object, so names in them resolve through no partition")
	wantNote(t, r, "_torn", migrate.Approximated, `the name "volume" resolves to nothing readable: nothing visible from Plant::Fill::torn is called volume; it is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition`)
	wantNote(t, r, "_prime", migrate.Mapped, "names resolve against the context's pump, read as this.pump")
	wantNote(t, r, "_twice", migrate.Mapped, "names resolve against the context's tank, read as this.tank")
	wantNote(t, r, "_openv", migrate.Mapped, "names resolve against valve of the enclosing partition's object, read as this.tank.valve")
	wantNote(t, r, "_note", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_free", migrate.Mapped, "the JavaScript body is translated to v2")

	s := session(t, r)
	meta(t, s, "%instantiate Plant")
	wantVerdict(t, s.RunAction("Plant::Fill", "Plant"))
	runs := strings.Join(s.RunRuns("Plant::Fill", []string{"Plant"}, 1, 1, []string{"this.level", "this.runs"}).Lines, "\n")
	for _, want := range []string{"this.level: 1 run(s), min 9.0", "this.runs: 1 run(s), min 1"} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
}

// The reactor fixture has a configured clock variable named other than the
// default, which a parameter or property of the same name shadows, a
// JavaScript function behavior and defaults the translator writes, and bodies
// of each kind the translator refuses, kept as comments with a typed reason.
func TestTranslatorRefusalsAndConfiguredClockName(t *testing.T) {
	r := migrateXMI(t, "reactor")
	for _, line := range []string{
		"calc def Energy {",
		"RealFunctions::max(E_0 - P * dt, 0)",
		"attribute limit : ScalarValues::Real default = RealFunctions::min(power * 2, 5);",
		"assign this.started := localClock.currentTime;",
		"attribute k : ScalarValues::Integer;",
		"assign n := n * k;",
		"assign y := x * this.power;",
		"d == t_sim * 2",
		"attribute reading : ScalarValues::Real default = t_sim + 1;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "d == localClock.currentTime * 2")
	wantNoLine(t, r.Notation, "attribute reading : ScalarValues::Real default = localClock.currentTime + 1;")
	wantNoLine(t, r.Notation, "attribute x : ScalarValues::Integer;")
	wantNoLine(t, r.Notation, "attribute power : ScalarValues::Integer;")
	wantNoLine(t, r.Notation, "assign y := x * 2;")
	wantNoLine(t, r.Notation, "assign this.power := this.label + 1;")
	wantNoLine(t, r.Notation, "assign x := x * 2;")
	wantNoLine(t, r.Notation, "assign a := a + step;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_energy", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_limit", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_mark", migrate.Mapped, "the clock variable t_sim, named by the configuration Run, reads the local clock")
	wantNote(t, r, "_delay", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_reading", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_double", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_gain", migrate.Mapped, "the JavaScript body is translated to v2")
	for id, want := range map[string]string{
		"_shadow":  `the construct "var power" is outside the translated subset: power is already a feature here, which a declaration would shadow`,
		"_pinned":  `the construct "var x" is outside the translated subset: x is already a feature here, which a declaration would shadow`,
		"_relay":   `the construct "x" is outside the translated subset: an input pin is not assigned`,
		"_bare":    `the name "raw.level" resolves to nothing readable: Reactor::Cycle::bare::raw has no type, so no feature level`,
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

	s := session(t, r)
	meta(t, s, "%instantiate Reactor")
	v := s.RunAction("Reactor::Cycle", "Reactor")
	wantVerdict(t, v)
	// gain reads its value pin x = 1.5 and the context's power = 4 into its out pin.
	if lines := strings.Join(v.Lines, "\n"); !strings.Contains(lines, "gain.y = 6.0") {
		t.Errorf("run lacks gain.y = 6.0:\n%s", lines)
	}
}

// The meter fixture chains opaque actions through their pins: a translated body
// assigns its output pin, and the object flow carries the value to the next
// action's input, which its body reads; a Java body divides two integers and two
// reals. A translated body that never assigns its output pin, like a body that
// is not translated, leaves the flow from that pin unwritten, with the reason.
func TestTranslatedOutputPinsFeedTheirFlows(t *testing.T) {
	r := migrateXMI(t, "meter")
	for _, line := range []string{
		"assign y := x * 2;",
		"flow sense.y to record.v;",
		"assign this.total := v + 1;",
		"assign this.half := RealFunctions::floor((this.ticks - this.ticks % 2) / 2);",
		"assign this.ratio := this.total / 2;",
		"/* flow idle.z to sink.w not written: the body of 'idle' never assigns idle.z */",
		"/* flow dark.q to drain.w not written: 'dark' is not migrated and produces no value */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "flow idle.z to sink.w;")
	wantNoLine(t, r.Notation, "flow dark.q to drain.w;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_sense", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_o1", migrate.Mapped, "")
	wantNote(t, r, "_split", migrate.Mapped, "the Java body is translated to v2")
	wantNote(t, r, "_o2", migrate.Approximated, "the flow is kept as a comment: the body of 'idle' never assigns 'z', so no value leaves it")
	wantNote(t, r, "_sink", migrate.Approximated, "its input sink.w receives no value, since the body of 'idle' never assigns idle.z; the action cannot be performed until one is bound")
	wantNote(t, r, "_o3", migrate.Approximated, "the flow is kept as a comment: its source 'dark' is not migrated, so no value reaches 'q'")

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	wantVerdict(t, s.RunAction("Meter::Measure", "Meter"))
	runs := strings.Join(s.RunRuns("Meter::Measure", []string{"Meter"}, 1, 1, []string{"this.total", "this.half", "this.ratio"}).Lines, "\n")
	for _, want := range []string{"this.total: 1 run(s), min 4.0", "this.half: 1 run(s), min 3", "this.ratio: 1 run(s), min 2.0"} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
}

// A feature typed by an enumeration or a block is known to hold no scalar: a
// body that counts, compares or assigns it against a number, or against another
// non-scalar type, is refused with both types named, while a value of a type
// specializing the target's is assigned. A JavaScript whole number
// beyond what its Number holds exactly is refused; one within it, and a string
// spelling a character as a UTF-16 surrogate pair, are translated.
func TestNonScalarFeaturesAndScriptLiterals(t *testing.T) {
	r := migrateXMI(t, "meter")
	for _, line := range []string{
		`assign this.label := "` + "\U0001F600" + `";`,
		"assign this.count := 9007199254740991;",
		"assign this.dial := this.hand;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign this.hand := this.dial;")
	wantNoLine(t, r.Notation, "assign this.count := this.mode + 1;")
	wantNoLine(t, r.Notation, "assign this.count := this.mode;")
	wantNoLine(t, r.Notation, "assign this.mode := this.dial;")
	wantNoLine(t, r.Notation, "assign this.count := 9007199254740993;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_smile", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_widen", migrate.Mapped, "the JavaScript body is translated to v2")
	for id, want := range map[string]string{
		"_bump":   `the types at "+" disagree: an operand is a Mode, not a number`,
		"_pick":   `the types at "count =" disagree: a Mode is assigned to the Integer count holds`,
		"_match":  `the types at "mode =" disagree: a Gauge is assigned to the Mode mode holds`,
		"_narrow": `the types at "hand =" disagree: a Gauge is assigned to the Needle hand holds`,
		"_huge":   `the construct "9007199254740993" is outside the translated subset: a script rounds a whole number beyond 9007199254740991 to the nearest floating-point value`,
	} {
		wantNote(t, r, id, migrate.Approximated, want)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	wantVerdict(t, s.RunAction("Meter::Flip", "Meter"))
	runs := strings.Join(s.RunRuns("Meter::Flip", []string{"Meter"}, 1, 1, []string{"this.count"}).Lines, "\n")
	if want := "this.count: 1 run(s), min 9007199254740991"; !strings.Contains(runs, want) {
		t.Errorf("runs lack %q:\n%s", want, runs)
	}
}
