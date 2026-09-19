package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
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
		"if not this.tcs.GS_Found and not this.tcs.'Guide Star Lost' and this.tcs.i < this.tcs.Retries",
		"action wait accept after this.tcs.ditSetup [SI::s];",
		"action attempt;",
		"attribute 'Time_Loop start' : ScalarValues::Real [0..1];",
		"attribute Time_Loop : ScalarValues::Real [0..1];",
		"if 'Time_Loop start'->SequenceFunctions::notEmpty() {",
		"assign Time_Loop := localClock.currentTime - 'Time_Loop start';",
		"if 'Time_Never start'->SequenceFunctions::notEmpty() {",
		"assign 'Time_Unfinished start' := localClock.currentTime;",
		"first start then stamp9;",
		"assign 'Time_Run start' := localClock.currentTime;",
		"first stamp9 then 'start timer';",
		"assign Time_Run := localClock.currentTime - 'Time_Run start';",
		"first stamp10 then final;",
		"first 'fork' then merge2;",
		"first stamp8 then merge2;",
		"first merge2 then stamp11;",
		"assign Time_Pass := localClock.currentTime - 'Time_Pass start';",
		"first stamp11 then done;",
		"in Retries : ScalarValues::Integer = 2;",
		"assign this.tcs.GS_Found := Retries < 1;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "start' : ScalarValues::Real default")
	wantNoLine(t, r.Notation, "this.tcs.Retries < 1")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_set", migrate.Mapped, "the JavaScript body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_seed", migrate.Mapped, "the JavaScript body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_stamp0", migrate.Mapped, "the clock variable simtime, named by 2 simulation configurations, reads the local clock")
	wantNote(t, r, "_e5", migrate.Mapped, "the JavaScript body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_e6", migrate.Mapped, "the English body is translated to v2; names resolve against the context's tcs, read as this.tcs")
	wantNote(t, r, "_lane", migrate.Mapped, "read as this.tcs")
	wantNote(t, r, "_attempt", migrate.Mapped, "a step with a duration and no further behavior")
	wantNote(t, r, "_dc", migrate.Approximated, `the duration "ditSetup s" is read as the expression this.tcs.ditSetup, in seconds`)
	wantNote(t, r, "_span", migrate.Mapped, "from the start of 'first attempt' to the end of 'guide star found' is assigned to the attribute Time_Loop, in seconds")
	wantNote(t, r, "_between", migrate.Mapped, "from the end of 'first attempt' to the start of 'guide star found' is assigned to the attribute Time_Between, in seconds")
	wantNote(t, r, "_single", migrate.Mapped, "from the start of 'attempt' to the end of 'attempt' is assigned to the attribute Time_Attempt, in seconds, and left without a value by a run that does not reach both")
	wantNote(t, r, "_never", migrate.Mapped, "from the start of 'abort' to the end of 'guide star found' is assigned to the attribute Time_Never")
	wantNote(t, r, "_unfinished", migrate.Mapped, "from the start of 'first attempt' to the end of 'abort' is assigned to the attribute Time_Unfinished")
	wantNote(t, r, "_run", migrate.Mapped, "from the start of (_init) to the start of (_final) is assigned to the attribute Time_Run")
	wantNote(t, r, "_kickoff", migrate.Unmapped, "the observation reads the clock at the end of (_init), which is no action and so has no end of its own")
	wantNote(t, r, "_pass", migrate.Mapped, "from the start of 'first attempt' to the start of (_ff) is assigned to the attribute Time_Pass")
	wantNote(t, r, "_ff", migrate.Mapped, "a flow final ends the token, as done does")
	wantNote(t, r, "_ended", migrate.Unmapped, "the observation reads the clock at the end of (_ff), which is no action and so has no end of its own")
	wantNote(t, r, "_dangle", migrate.Unmapped, "the observation's events resolve to nothing: 1 event reference(s) resolve to nothing in the document (_missing)")
	wantNote(t, r, "_astray", migrate.Unmapped, "is not a node of the activity")
	wantNote(t, r, "_blank", migrate.Unmapped, "observes no event")

	s := session(t, r)
	meta(t, s, "%instantiate Observatory")
	meta(t, s, "%seed 1")
	v := s.RunAction("Observatory::Acquire", "Observatory")
	wantVerdict(t, v)
	runs := strings.Join(s.RunRuns("Observatory::Acquire", []string{"Observatory"}, 5, seedOf(1), []string{"this.Time_Acq_Total", "Time_Loop", "Time_Between", "Time_Attempt", "Time_Run", "Time_Pass"}).Lines, "\n")
	// Four attempts of ditSetup = 2.5 s each: the loop ran until i reached Retries,
	// the observation from the initial node to the final spans the whole run, and
	// the one ending at the flow final each attempt reaches holds the last pass.
	for _, want := range []string{
		"this.Time_Acq_Total: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Run: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Pass: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Loop: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Between: 5 run(s), min 10.0, mean 10.0, max 10.0",
		"Time_Attempt: 5 run(s), min 2.5, mean 2.5, max 2.5",
	} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
	if strings.Contains(runs, "error") {
		t.Errorf("runs report an error:\n%s", runs)
	}
	// The abort branch is never taken: the observation starting there and the one
	// ending there hold no value in any run, so neither is a column nor a zero,
	// while the stamp the unfinished one did take is.
	all := strings.Join(s.RunRuns("Observatory::Acquire", []string{"Observatory"}, 2, seedOf(1), nil).Lines, "\n")
	for _, want := range []string{"| Time_Loop ", "Time_Loop: 2 run(s)", "Time_Unfinished start: 2 run(s), min 0.0"} {
		if !strings.Contains(all, want) {
			t.Errorf("runs lack %q:\n%s", want, all)
		}
	}
	for _, absent := range []string{"Time_Never", "| Time_Unfinished |", "Time_Unfinished:", "error"} {
		if strings.Contains(all, absent) {
			t.Errorf("runs mention %q:\n%s", absent, all)
		}
	}
	never := strings.Join(s.RunRuns("Observatory::Acquire", []string{"Observatory"}, 2, seedOf(1), []string{"Time_Never"}).Lines, "\n")
	if !strings.Contains(never, "error: invalid run request: no completed run of Observatory::Acquire produced a value named Time_Never") {
		t.Errorf("observing the never-stamped duration:\n%s", never)
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
// A lane representing a classifier, or a property of one, four and five
// composite parts below the context resolves through the whole chain. An
// explicit `this` in a lane is the lane's object, never the context block.
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
		"assign this.site.control.rack.controller.status := true;",
		"assign this.site.control.rack.controller.led.lit := true;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign this.tank.volume := 0;")
	wantNoLine(t, r.Notation, "assign this.pump.volume := 0;")
	wantNoLine(t, r.Notation, "if this.pump.on")
	wantNoLine(t, r.Notation, "assign this.level := 1;")
	wantNoLine(t, r.Notation, "assign this.tank.level := 1;")
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
	wantNote(t, r, "_deep", migrate.Mapped, "the partition represents the context's part site.control.rack.controller, a Controller")
	wantNote(t, r, "_ledlane", migrate.Mapped, "the partition represents Controller::led, read as this.site.control.rack.controller.led")
	wantNote(t, r, "_arm", migrate.Mapped, "names resolve against the context's part site.control.rack.controller, a Controller")
	wantNote(t, r, "_mine", migrate.Approximated, `the name "this.level" resolves to nothing readable: Tank has no feature level`)
	wantNote(t, r, "_light", migrate.Mapped, "names resolve against Controller::led, read as this.site.control.rack.controller.led")

	s := session(t, r)
	meta(t, s, "%instantiate Plant")
	wantVerdict(t, s.RunAction("Plant::Fill", "Plant"))
	for _, path := range []string{"site.control.rack.controller.status", "site.control.rack.controller.led.lit"} {
		if got := meta(t, s, "%eval in #1 : "+path); !strings.HasSuffix(got, "= true") {
			t.Errorf("%s not assigned through the deep lane:\n%s", path, got)
		}
	}
	runs := strings.Join(s.RunRuns("Plant::Fill", []string{"Plant"}, 1, seedOf(1), []string{"this.level", "this.runs"}).Lines, "\n")
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
		"assign d := t_sim * 2;",
		"attribute reading : ScalarValues::Real default = t_sim + 1;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign d := localClock.currentTime * 2;")
	wantNoLine(t, r.Notation, "d == t_sim * 2")
	wantNoLine(t, r.Notation, "RealFunctions::sqrt(x)")
	wantNoLine(t, r.Notation, "calc def Root {")
	wantNoLine(t, r.Notation, "calc def Split {")
	wantLine(t, r.Notation, "action def Root {")
	wantLine(t, r.Notation, "action def Split {")
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
		"_root":    `the body is kept as a comment: as the result expression, the types at "Math.sqrt(x)" disagree: the expression is a Real, not the Boolean wanted; as statements, the call "Math.sqrt" is not in the translated function table: a call is not a statement of the subset`,
		"_split":   `the body is kept as a comment: as the result expression, the behavior has 2 output parameters; one result expression can stand for none of them; as statements, the construct "x" is outside the translated subset: an expression that assigns nothing is not a statement of the subset`,
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

// Migrated Math.floor/ceil/round are exact through the least Integer, the ceiling
// of which is a value, and a typed overflow, never a wrapped Integer, beyond.
func TestTranslatedRoundingsStopAtTheIntegerRange(t *testing.T) {
	r := migrateXMI(t, "reactor")
	for _, line := range []string{
		"calc def Floor {",
		"    RealFunctions::floor(x)\n",
		"    OpenSysMLMathFunctions::ceiling(x)\n",
		"    RealFunctions::floor(x + 0.5)\n",
	} {
		wantLine(t, r.Notation, line)
	}
	s := session(t, r)
	for _, tc := range []struct{ call, want string }{
		{"Floor(2.7)", "= 2"},
		{"Floor(-2.1)", "= -3"},
		{"Floor(9223372036854774784.0)", "= 9223372036854774784"},
		{"Floor(-9223372036854775808.0)", "= -9223372036854775808"},
		{"Ceil(-9223372036854774784.0)", "= -9223372036854774784"},
		{"Ceil(-9223372036854775808.0)", "= -9223372036854775808"},
		{"Ceil(2.1)", "= 3"},
		{"Ceil(-2.9)", "= -2"},
		{"Round(9007199254740993.0)", "= 9007199254740992"},
		{"Round(-2.5)", "= -2"},
		{"Floor(9223372036854775808.0)", "arithmetic overflow: 9.223372036854776e+18 exceeds the Integer range"},
		{"Floor(1.0e20)", "arithmetic overflow: 1e+20 exceeds the Integer range"},
		{"Ceil(9223372036854775808.0)", "arithmetic overflow: 9.223372036854776e+18 exceeds the Integer range"},
		{"Ceil(1.0e20)", "arithmetic overflow: 1e+20 exceeds the Integer range"},
		{"Ceil(-1.0e20)", "arithmetic overflow: -1e+20 exceeds the Integer range"},
		{"Round(1.0e20)", "arithmetic overflow: 1e+20 exceeds the Integer range"},
		{"Round(-1.0e20)", "arithmetic overflow: -1e+20 exceeds the Integer range"},
	} {
		v := s.RunCalc(tc.call)
		if lines := strings.Join(v.Lines, "\n"); !strings.Contains(lines, tc.want) {
			t.Errorf("%s: want %q:\n%s", tc.call, tc.want, lines)
		}
	}
}

// The meter fixture chains opaque actions through their pins: a translated body
// assigns its output pin, and the object flow carries the value to the next
// action's input, which its body reads; a Java body divides two integers and two
// reals (its Math.floor a double, its Math.round a long), a versioned Java label
// reads the same, and a JavaCC label is another language, whose body is written
// only as the v2 assignments it already is. A
// translated body that never assigns its output pin, like a body that is not
// translated, leaves the flow from that pin unwritten, with the reason.
func TestTranslatedOutputPinsFeedTheirFlows(t *testing.T) {
	r := migrateXMI(t, "meter")
	for _, line := range []string{
		"assign y := x * 2;",
		"flow sense.y to record.v;",
		"assign this.total := v + 1;",
		"assign this.half := OpenSysMLMathFunctions::quotient(this.ticks, 2);",
		"assign this.ratio := this.total / 2;",
		"assign this.floored := RealFunctions::floor(this.total) / 8;",
		"assign this.rounded := OpenSysMLMathFunctions::quotient(RealFunctions::floor(this.total + 0.5), 8);",
		"assign this.quarter := OpenSysMLMathFunctions::quotient(this.ticks, 4);",
		"assign this.eighth := this.ticks / 8;",
		"/* flow idle.z to sink.w not written: the body of 'idle' never assigns idle.z */",
		"/* flow dark.q to drain.w not written: 'dark' is not migrated and produces no value */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "flow idle.z to sink.w;")
	wantNoLine(t, r.Notation, "flow dark.q to drain.w;")
	wantNoLine(t, r.Notation, "OpenSysMLMathFunctions::quotient(this.ticks, 8)")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_judgeok", migrate.Approximated, `the types at "Math.sqrt(total)" disagree: the expression is a Real, not the Boolean wanted`)
	wantNote(t, r, "_judgen", migrate.Mapped, "")
	wantNote(t, r, "_judgem", migrate.Mapped, "")
	for _, line := range []string{
		"in ok : ScalarValues::Boolean;",
		"in n : ScalarValues::Integer = RealFunctions::floor(this.total);",
		"in m : ScalarValues::Integer = 2;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "in ok : ScalarValues::Boolean = RealFunctions::sqrt(this.total);")
	wantNote(t, r, "_sense", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_o1", migrate.Mapped, "")
	wantNote(t, r, "_split", migrate.Mapped, "the Java body is translated to v2")
	wantNote(t, r, "_quarterly", migrate.Mapped, "the Java 1.8.0_202 body is translated to v2")
	wantNote(t, r, "_cc", migrate.Approximated, "the JavaCC body is written as v2 assignments")
	wantNote(t, r, "_o2", migrate.Approximated, "the flow is kept as a comment: the body of 'idle' never assigns 'z', so no value leaves it")
	wantNote(t, r, "_sink", migrate.Approximated, "its input sink.w receives no value, since the body of 'idle' never assigns idle.z; the action cannot be performed until one is bound")
	wantNote(t, r, "_o3", migrate.Approximated, "the flow is kept as a comment: its source 'dark' is not migrated, so no value reaches 'q'")

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	wantVerdict(t, s.RunAction("Meter::Measure", "Meter"))
	runs := strings.Join(s.RunRuns("Meter::Measure", []string{"Meter"}, 1, seedOf(1), []string{"this.total", "this.half", "this.ratio", "this.floored", "this.rounded", "this.quarter", "this.eighth"}).Lines, "\n")
	for _, want := range []string{"this.total: 1 run(s), min 4.0", "this.half: 1 run(s), min 3", "this.ratio: 1 run(s), min 2.0", "this.floored: 1 run(s), min 0.5", "this.rounded: 1 run(s), min 0", "this.quarter: 1 run(s), min 1", "this.eighth: 1 run(s), min 0.875"} {
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
// spelling a character as a UTF-16 surrogate pair, are translated. A default
// in a language the translator reads is refused with it, never read as v2
// even when its text is v2 syntax over visible names: a call outside the
// table, string concatenation, the `-x ** y` JavaScript itself rejects, and
// text that is not JavaScript at all (`ready and enabled`) as a default, a
// guard or a statement. A parameter's opaque default is checked against the
// parameter's type like a property's.
func TestNonScalarFeaturesAndScriptLiterals(t *testing.T) {
	r := migrateXMI(t, "meter")
	for _, line := range []string{
		`assign this.label := "` + "\U0001F600" + `";`,
		`assign this.lit := this.label == "` + "\U0001F600" + `";`,
		"assign this.count := 9007199254740991;",
		"assign this.dial := this.hand;",
		"attribute cube : ScalarValues::Real default = -(total ** 3);",
		"ref part shown : Gauge default = if (count > 0) ? dial else hand;",
		`/* default value not migrated: {JavaScript} count > 0 ? dial : hand — the types at "count > 0 ? dial : hand" disagree: the expression is a Gauge, not the Needle wanted */`,
		`/* default value not migrated: {JavaScript} cells.reading — the types at "cells.reading" disagree: the expression is a collection, not the one value wanted */`,
		`/* default value not migrated: {JavaScript} total(count) — the call "total" is not in the translated function table */`,
		`/* default value not migrated: {JavaScript} label + "!" — the construct "+" is outside the translated subset: string concatenation has no v2 form in the subset */`,
		"/* default value not migrated: {JavaScript} -total ** 2 — the construct \"-total **\" is outside the translated subset: JavaScript parenthesizes a unary operand of `**` */",
		`/* default value not migrated: {JavaScript} ready or enabled — the text "or" is not expression syntax: text follows the expression */`,
		`/* default value not migrated: {JavaScript} Math.sqrt(total) — the types at "Math.sqrt(total)" disagree: the expression is a Real, not the Boolean wanted */`,
		"in limit : ScalarValues::Integer default = this.count + 1;",
		"/* guard not migrated: [{JavaScript} ready and enabled] — the text \"and\" is not expression syntax: text follows the expression */",
		"/* body not migrated (the text \"and\" is not expression syntax: a statement ends at `;` or a newline) {JavaScript}:",
		"assign this.flag := this.ready and this.enabled;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "attribute lit : ScalarValues::Boolean default = ready or enabled;")
	wantNoLine(t, r.Notation, "in armed : ScalarValues::Boolean default = RealFunctions::sqrt(this.total);")
	wantNoLine(t, r.Notation, "if ready and enabled")
	wantNoLine(t, r.Notation, "assign this.flag := ready and enabled;")
	wantNoLine(t, r.Notation, "default = total(count);")
	wantNoLine(t, r.Notation, `default = label + "!";`)
	wantNoLine(t, r.Notation, "default = -total ** 2;")
	wantNoLine(t, r.Notation, "assign this.hand := this.dial;")
	wantNoLine(t, r.Notation, "assign this.count := this.mode + 1;")
	wantNoLine(t, r.Notation, "assign this.count := this.mode;")
	wantNoLine(t, r.Notation, "assign this.mode := this.dial;")
	wantNoLine(t, r.Notation, "assign this.count := 9007199254740993;")
	if strings.Count(string(r.Notation), `assign this.lit := this.label == "`+"\U0001F600"+`";`) != 1 {
		t.Errorf("the Java == on strings is written beside its equals:\n%s", r.Notation)
	}
	wantNoLine(t, r.Notation, "ref part pointer : Needle default = if (count > 0) ? dial else hand;")
	wantNoLine(t, r.Notation, "attribute top : ScalarValues::Real default = cells.reading;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_smile", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_same", migrate.Mapped, "the Java body is translated to v2")
	wantNote(t, r, "_widen", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_shown", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_latch", migrate.Mapped, "the JavaScript body is translated to v2")
	wantNote(t, r, "_armed", migrate.Approximated, `default value not migrated: the types at "Math.sqrt(total)" disagree: the expression is a Real, not the Boolean wanted`)
	wantNote(t, r, "_lit", migrate.Approximated, `default value not migrated: the text "or" is not expression syntax: text follows the expression`)
	wantNote(t, r, "_a2", migrate.Approximated, `the guard [{JavaScript} ready and enabled] is kept as a comment and the edge written unguarded: the text "and" is not expression syntax: text follows the expression`)
	for id, want := range map[string]string{
		"_bump":    `the types at "+" disagree: an operand is a Mode, not a number`,
		"_pick":    `the types at "count =" disagree: a Mode is assigned to the Integer count holds`,
		"_match":   `the types at "mode =" disagree: a Gauge is assigned to the Mode mode holds`,
		"_narrow":  `the types at "hand =" disagree: a Gauge is assigned to the Needle hand holds`,
		"_huge":    `the construct "9007199254740993" is outside the translated subset: a script rounds a whole number beyond 9007199254740991 to the nearest floating-point value`,
		"_alias":   "the construct \"==\" is outside the translated subset: Java compares strings by identity with `==`, which a comparison of their values does not reproduce; `equals` compares their content",
		"_pointer": `the types at "count > 0 ? dial : hand" disagree: the expression is a Gauge, not the Needle wanted`,
		"_top":     `the types at "cells.reading" disagree: the expression is a collection, not the one value wanted`,
	} {
		wantNote(t, r, id, migrate.Approximated, want)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	wantVerdict(t, s.RunAction("Meter::Flip", "Meter"))
	runs := strings.Join(s.RunRuns("Meter::Flip", []string{"Meter"}, 1, seedOf(1), []string{"this.count", "this.lit"}).Lines, "\n")
	for _, want := range []string{"this.count: 1 run(s), min 9007199254740991", "this.lit: true ×1"} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs lack %q:\n%s", want, runs)
		}
	}
}

// A name read through a collection — a plural part on the path, or a swimlane
// representing a plural part — is a collection: arithmetic on it and a scalar
// assignment of it are refused, an assignment through it is refused, and a
// collection function over it translates and runs.
func TestPluralPathsStayCollections(t *testing.T) {
	r := migrateXMI(t, "meter")
	wantLine(t, r.Notation, "assign this.total := this.cells.reading->ControlFunctions::reduce { in x; in y; RealFunctions::max(x, y) };")
	wantLine(t, r.Notation, "action ticking : Gauge::Tick;")
	wantNoLine(t, r.Notation, "assign this.total := this.cells.reading + 1;")
	wantNoLine(t, r.Notation, "assign this.cells.reading := 1;")
	wantNoLine(t, r.Notation, "assign this.cells.reading := 2;")
	wantNoLine(t, r.Notation, "assign this.total := this.cells.reading;")
	wantNoLine(t, r.Notation, "perform action ticking ::> cells.tick;")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_peak", migrate.Mapped, "the JavaScript body is translated to v2")
	for id, want := range map[string]string{
		"_spread":  `the types at "+" disagree: an operand is a collection, not a number`,
		"_reset":   `the construct "cells.reading" is outside the translated subset: this.cells is a collection, so the assignment would write through several objects`,
		"_calib":   `the construct "reading" is outside the translated subset: this.cells is a collection, so the assignment would write through several objects`,
		"_span":    `the types at "total =" disagree: one side is a collection and the other a single value`,
		"_ticking": "its swimlane represents this.cells, a collection of objects, so none of them performs the call, which runs in the caller's context",
	} {
		wantNote(t, r, id, migrate.Approximated, want)
	}
	wantNote(t, r, "_cells_lane", migrate.Mapped, "read as this.cells; it is a collection, so names read through it are collections and are not assigned")

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	wantVerdict(t, s.RunAction("Meter::Sweep", "Meter"))
	runs := strings.Join(s.RunRuns("Meter::Sweep", []string{"Meter"}, 1, seedOf(1), []string{"this.total"}).Lines, "\n")
	if want := "this.total: 1 run(s), min 3"; !strings.Contains(runs, want) {
		t.Errorf("runs lack %q:\n%s", want, runs)
	}
}
