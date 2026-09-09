package repl

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// lampSession loads the signal-driven lamp and instantiates the bulb that
// exhibits it, leaving no debugging session open.
func lampSession(t *testing.T) *Session {
	t.Helper()
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%instantiate bulb"), "✓ Created instance of Lamps::bulb")
	return s
}

// TestSendDrivesAnAcceptTransition is the self-contained debugging loop the
// command exists for: attach, send, step, and the machine has moved.
func TestSendDrivesAnAcceptTransition(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%state bulb"), `✓ Debugging state machine "lamp"`, "Current state: off")
	wants(t, run(t, s, "%events"), "Event queue empty")

	wants(t, run(t, s, "%send go"),
		`✓ Sent go to object #1 of "Lamps::bulb"`,
		`Accepted by state machine "Lamp" in state off`,
		"Use %step or %advance <time> to dispatch it")
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "  go")
	wants(t, run(t, s, "%step"), "Event dispatched")
	wants(t, run(t, s, "%current"), "Current state: on")
	wants(t, run(t, s, "%events"), "Event queue empty")
}

// TestSendCarriesNamedArguments binds the payload as %invoke binds parameters,
// and the accept's effect reads it.
func TestSendCarriesNamedArguments(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%state bulb")
	run(t, s, "%send go")
	run(t, s, "%advance 1")
	wants(t, run(t, s, "%current"), "Current state: on")

	wants(t, run(t, s, "%send Dim(level = 2 + 5)"), "✓ Sent Dim(level=7)", `Accepted by state machine "Lamp" in state on`)
	wants(t, run(t, s, "%advance 1"), "Current state: dimmed")
	wants(t, run(t, s, "%features bulb"), "brightness", "7")

	// A string payload keeps its escaped quotes and the parentheses after them.
	wants(t, run(t, s, `%send Label(text = "say \"hi\" (twice)")`), `✓ Sent Label(text="say \"hi\" (twice)")`, `Accepted by state machine "Lamp" in state dimmed`)
	wants(t, run(t, s, "%advance 1"), "Current state: labeled")
	wants(t, run(t, s, "%features bulb"), `label`, `"say \"hi\" (twice)"`)
}

// TestSendDecidesGuardsAsDispatchWould: a signal whose every triggered
// transition is held back by its guard is refused before it is posted, the guard
// reading the payload it carries; one a guard lets through is queued and fires.
func TestSendDecidesGuardsAsDispatchWould(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%state bulb")
	wants(t, run(t, s, "%send go"), `Accepted by state machine "Lamp" in state off: transition off_on fires on it`)
	run(t, s, "%advance 1")
	wants(t, run(t, s, "%current"), "Current state: on")

	wants(t, run(t, s, "%send Dim(level=0)"),
		`error: object #1 of "Lamps::bulb" would fire no transition on Dim(level=0) now, so it was not sent: state machine "Lamp" in state on, and the guard of every transition Dim triggers is false`)
	wants(t, run(t, s, "%events"), "Event queue empty")
	wants(t, run(t, s, "%current"), "Current state: on")
	wants(t, run(t, s, "%features bulb"), "brightness", "0")

	wants(t, run(t, s, "%send Dim(level=3)"),
		"✓ Sent Dim(level=3)",
		`Accepted by state machine "Lamp" in state on: transition on_dim fires on it`)
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "Dim(level=3)")
	out := run(t, s, "%advance 1")
	wants(t, out, "Current state: dimmed")
	if strings.Contains(out, "consumed by no transition") {
		t.Errorf("a signal a transition fired on was reported dropped:\n%s", out)
	}
	wants(t, run(t, s, "%features bulb"), "brightness", "3")
}

// TestSendReportsAGuardThatCannotBeEvaluated: a guard that fails on the payload
// is a %send error, and the signal is not queued.
func TestSendReportsAGuardThatCannotBeEvaluated(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package GuardErr {
		private import ScalarValues::*;
		attribute def Dim { attribute level : Integer; }
		state def Gate {
			entry; then shut;
			state shut;
			transition shut_through first shut accept d : Dim if 10 / d.level > 1 then through;
			state through;
		}
		part def Keeper { exhibit state gate : Gate; }
		part keeper : Keeper;
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate keeper")
	run(t, s, "%state keeper")
	wants(t, run(t, s, "%send Dim(level=0)"),
		`error: state machine "Gate" cannot decide Dim(level=0): state shut: eval guard of transition shut_through: division by zero`)
	wants(t, run(t, s, "%events"), "Event queue empty")
	wants(t, run(t, s, "%send Dim(level=2)"), "✓ Sent Dim(level=2)", "transition shut_through fires on it")
	wants(t, run(t, s, "%advance 1"), "Current state: through")
}

// TestSendDecidesOnWhatAnObjectsStartupMakesOfIt: a guard reaching a part not
// yet built starts that part's machine, under %send as under the step — the
// sensor's entry sets level to 10 — so %send decides on the level the step reads.
func TestSendDecidesOnWhatAnObjectsStartupMakesOfIt(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package Startup {
		private import ScalarValues::*;
		attribute def Go;
		attribute def Halt;
		part def Sensor {
			attribute level : Integer = 1;
			exhibit state calibrate {
				entry; then ready;
				state ready { entry assign level := 10; }
			}
		}
		part def Controller {
			part sensor : Sensor;
			exhibit state life {
				entry; then off;
				state off;
				transition off_on first off accept Go if sensor.level > 5 then on;
				transition off_halt first off accept Halt if sensor.level < 5 then halted;
				state on;
				state halted;
			}
		}
		part ctl : Controller;
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate ctl")
	run(t, s, "%state ctl")
	wants(t, run(t, s, "%send Halt"),
		`would fire no transition on Halt now, so it was not sent: state machine "life" in state off, and the guard of every transition Halt triggers is false`)
	wants(t, run(t, s, "%send Go"), "✓ Sent Go", `Accepted by state machine "life" in state off: transition off_on fires on it`)
	out := run(t, s, "%advance 1")
	wants(t, out, "Current state: on")
	if strings.Contains(out, "consumed by no transition") {
		t.Errorf("a signal decided to fire was reported dropped:\n%s", out)
	}
	wants(t, run(t, s, "%features ctl"), "sensor")
}

// TestStepReportsASignalDispatchedToNothing: a guard true when the signal was
// sent may be false when it is dispatched — here a Lock dispatched first shuts
// the gate — and the step that drops the signal says so.
func TestStepReportsASignalDispatchedToNothing(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package Guarded {
		private import ScalarValues::*;
		attribute def Poke;
		attribute def Lock;
		state def Gate {
			attribute open : Boolean = true;
			entry; then shut;
			state shut;
			transition lock first shut accept Lock do assign open := false then shut;
			transition shut_through first shut accept Poke if open then through;
			state through;
		}
		part def Keeper { exhibit state gate : Gate; }
		part keeper : Keeper;
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%instantiate keeper"), "✓ Created instance")
	run(t, s, "%state keeper")
	wants(t, run(t, s, "%send Lock"), `Accepted by state machine "Gate" in state shut: transition lock fires on it`)
	wants(t, run(t, s, "%send Poke"), `Accepted by state machine "Gate" in state shut: transition shut_through fires on it`)
	wants(t, run(t, s, "%events"), "Signals in flight: 2", "Lock", "Poke")
	wants(t, run(t, s, "%step"), "✓ Event dispatched\n", "Current state: shut")
	wants(t, run(t, s, "%features keeper"), "open = false")
	wants(t, run(t, s, "%step"),
		"✓ Event dispatched, but Poke was consumed by no transition: since it was sent, the state or the data its guards read had changed",
		"Current state: shut")
	wants(t, run(t, s, "%events"), "Event queue empty")
}

// TestParseSendLineScansQuotesLikeTheLexer keeps a string's escaped quotes, and
// the parentheses and commas after them, inside the argument they belong to.
func TestParseSendLineScansQuotesLikeTheLexer(t *testing.T) {
	cases := []struct {
		line   string
		signal string
		args   []string
		target string
	}{
		{`Label(text="a\"b") to bulb`, "Label", []string{`text="a\"b"`}, "bulb"},
		{`Label(text="(\") , x", n=1) to bulb`, "Label", []string{`text="(\") , x"`, "n=1"}, "bulb"},
		{`Label(text="\\") to bulb`, "Label", []string{`text="\\"`}, "bulb"},
		{`Label(text='a (\') b') to bulb`, "Label", []string{`text='a (\') b'`}, "bulb"},
		{`'my signal'(text=")") to 'the bulb'`, "'my signal'", []string{`text=")"`}, "'the bulb'"},
	}
	for _, tc := range cases {
		req, err := parseSendLine(tc.line)
		if err != nil {
			t.Errorf("parseSendLine(%s): %v", tc.line, err)
			continue
		}
		if req.signal != tc.signal || req.target != tc.target || strings.Join(req.args, "|") != strings.Join(tc.args, "|") {
			t.Errorf("parseSendLine(%s) = %+v, want signal %s args %q target %s", tc.line, req, tc.signal, tc.args, tc.target)
		}
	}
	if _, err := parseSendLine(`Label(text="a\") to bulb`); err == nil || !strings.Contains(err.Error(), "is not closed") {
		t.Errorf("an escaped quote that leaves the string open gave %v, want the list reported unclosed", err)
	}
}

// TestSendToNamedObjectNeedsNoSession delivers to an object named after `to`,
// whose exhibited machine takes the signal on its own step.
func TestSendToNamedObjectNeedsNoSession(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%send go to bulb"), `✓ Sent go to object #1 of "Lamps::bulb"`, `Accepted by state machine "Lamp" in state off`)
	rejects(t, run(t, s, "%send Dim(level=1) to Lamps::bulb"), "✓ Sent")

	wants(t, run(t, s, "%state bulb"), "Current state: off")
	wants(t, run(t, s, "%events"), "Signals in flight: 1")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendAddressesObjectsLikeEveryCommand names the destination the way every
// object-taking command does: by id, by path, with the shared errors.
func TestSendAddressesObjectsLikeEveryCommand(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%send go to #1"), "✓ Sent go to object #1", `Accepted by state machine "Lamp" in state off`)
	wants(t, run(t, s, "%send go to #9"), "error: no object #9 in this session", "the objects are #1")
	wants(t, run(t, s, "%send go to bulb.nosuch"), "error:", `"nosuch"`)
	wants(t, run(t, s, "%send go to #1."), "error:", "is not an object reference")

	// The object a second %instantiate displaces is still reached by id: its
	// machine, driven to on by a step of its own, takes no second go.
	m := regexp.MustCompile(`ID: (\d+)`).FindStringSubmatch(run(t, s, "%instantiate bulb"))
	if m == nil {
		t.Fatal("%instantiate bulb printed no id")
	}
	newID := "#" + m[1]
	wants(t, run(t, s, "%state #1"), "Current state: off")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
	wants(t, run(t, s, "%send go to #1"), "error: object #1 accepts no signal go now", `state machine "Lamp" in state on`)
	got := run(t, s, "%send go to bulb")
	wants(t, got, "✓ Sent go to object "+newID+` of "Lamps::bulb"`, `Accepted by state machine "Lamp" in state off`)

	for _, tc := range []struct {
		line    string
		wants   []string
		rejects []string
	}{
		{line: "%send go to ", wants: []string{"#1", newID, "bulb"}},
		{line: "%send go to #", wants: []string{"#1", newID}, rejects: []string{"bulb"}},
		{line: "%send Dim(level=1) to bu", wants: []string{"bulb"}},
		{line: "%send go ", rejects: []string{"#1"}},
		{line: "%send ", rejects: []string{"#1"}},
	} {
		got := s.Complete(tc.line, len(tc.line)).Candidates
		for _, want := range tc.wants {
			if !contains(got, want) {
				t.Errorf("%q: want %q in %v", tc.line, want, got)
			}
		}
		for _, bad := range tc.rejects {
			if contains(got, bad) {
				t.Errorf("%q: did not want %q in %v", tc.line, bad, got)
			}
		}
	}
}

// TestSendResolvesQualifiedSignals accepts the signal's qualified name.
func TestSendResolvesQualifiedSignals(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%state bulb")
	wants(t, run(t, s, "%send Lamps::go"), "✓ Sent go to object #1")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendRefusesWhatCannotBeDelivered covers every way a send goes wrong, each
// with a message that says what to do instead of a silently queued message.
func TestSendRefusesWhatCannotBeDelivered(t *testing.T) {
	s := lampSession(t)

	// No session and no `to`: nothing to guess a target from.
	wants(t, run(t, s, "%send go"), "error: no active state machine session", "name one with `to <object>`")
	wants(t, run(t, s, "%send"), "usage: %send <signal>")
	wants(t, run(t, s, "%send go bulb"), `unexpected "bulb" after the signal`, "usage: %send")
	wants(t, run(t, s, "%send go to"), "usage: %send")
	wants(t, run(t, s, "%send Dim(level=1 to bulb"), "not closed")

	// An object nothing runs a machine on, and one nothing instantiated.
	wants(t, run(t, s, "%send go to plain"), `error: no instance of "Lamps::plain"`, "%instantiate first")
	run(t, s, "%instantiate plain")
	wants(t, run(t, s, "%send go to plain"), `runs no state machine`, "%state <machine> <object>")
	wants(t, run(t, s, "%send go to nobody"), "error: unresolved reference: nobody")

	run(t, s, "%state bulb")
	// Unknown signals, in the wording every unresolved name gets, with a suggestion.
	wants(t, run(t, s, "%send gone"), "error: unresolved reference: gone", "did you mean")
	wants(t, run(t, s, "%send Lamps::gone"), "error: unresolved reference: Lamps::gone")
	// Not a signal at all.
	wants(t, run(t, s, "%send Lamp"), "error: Lamp is a state def, not a signal definition")

	// Arguments the signal has no feature for, malformed and doubled ones.
	wants(t, run(t, s, "%send go(level=1)"), "error: signal argument: go carries no feature", `go carries no feature "level"`, "(it carries none)")
	wants(t, run(t, s, "%send Dim(brightness=1)"), `Dim carries no feature "brightness"`, "(it carries level)")
	wants(t, run(t, s, "%send Dim(7)"), `argument "7" is not written as <parameter>=<expression>`)
	wants(t, run(t, s, "%send Dim(level=1, level=2)"), "error: argument level is given twice")
	wants(t, run(t, s, "%send Dim(level=nosuch)"), "error: argument level:")
	// A value the feature does not admit is refused here, not when the accept binds it.
	wants(t, run(t, s, `%send Dim(level="x")`), "error: signal argument: Dim.level: type mismatch", `cannot write "x" (string) to a feature typed by Integer`)
	wants(t, run(t, s, "%send Batch(levels=1)"), "error: signal argument: Batch.levels: multiplicity violation")

	// A signal the machine does not take in its current state is refused, not
	// left in flight. The queue stays empty.
	wants(t, run(t, s, "%send Dim(level=1)"), `error: object #1 of "Lamps::bulb" accepts no signal Dim now`, `state machine "Lamp" in state off`)
	wants(t, run(t, s, "%send Reset"), "accepts no signal Reset now")
	wants(t, run(t, s, "%events"), "Event queue empty")
	wants(t, run(t, s, "%current"), "Current state: off")
}

// TestSendToAMachineNoObjectPerforms targets the executor a %state started for
// a definition no type exhibits; one a type exhibits is refused without an object.
func TestSendToAMachineNoObjectPerforms(t *testing.T) {
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%state Lamp"), `error: no object of this session exhibits "Lamp"`, "%state Lamp <object>")
	wants(t, run(t, s, "%state Loose"), `✓ Started state machine executor for "Loose"`, "Current state: off")
	rejects(t, run(t, s, "%state Loose"), "Performed by")
	wants(t, run(t, s, "%send go"), `✓ Sent go to state machine "Loose"`, `Accepted by state machine "Loose" in state off`)
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendMatchesAnUndeclaredSignalByName: an accept naming a signal no
// declaration types is matched by name, as the runtime matches a model's send.
func TestSendMatchesAnUndeclaredSignalByName(t *testing.T) {
	s := loadFixture(t, "testdata/state_typed_usage.sysml")
	wants(t, run(t, s, "%state Machine"), "Current state: i1")
	wants(t, run(t, s, "%send Swop"), "error: unresolved reference: Swop")
	wants(t, run(t, s, "%send Swap(x=1)"), "error: unresolved reference: Swap")
	wants(t, run(t, s, "%send Swap"), `✓ Sent Swap to state machine "Machine"`, "No declaration types Swap, so the signal is matched by name alone")
	// Entering `two` starts it in the `i1` its definition's entry transition names.
	wants(t, run(t, s, "%advance 1"), "Current state: i1")
	wants(t, run(t, s, "%current"), "0. two", "1. i1")
}

// TestSendIsInHelpAndCompletion keeps the command discoverable.
func TestSendIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%send") {
		t.Error("the send command is dispatched but not in help")
	}
	if !slices.Contains(metaCommands(), "%send") {
		t.Error("the send command is not in the command table")
	}
	s := lampSession(t)
	if got := s.Complete("%sen", len("%sen")); !slices.Contains(got.Candidates, "%send") {
		t.Errorf("completing %%sen offered %v, want %%send", got.Candidates)
	}
	if got := s.Complete("%send Di", len("%send Di")); !slices.Contains(got.Candidates, "Dim") {
		t.Errorf("completing a signal name offered %v, want Dim", got.Candidates)
	}
	if got := s.Complete("%send go to bu", len("%send go to bu")); !slices.Contains(got.Candidates, "bulb") {
		t.Errorf("completing the object offered %v, want bulb", got.Candidates)
	}
	wants(t, run(t, s, "%snd go"), "%send")
}

// TestStateOnAnObjectAttachesToItsRunningMachine: naming the definition and an
// object that already exhibits a machine of it debugs that machine, so the
// object never runs two.
func TestStateOnAnObjectAttachesToItsRunningMachine(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%state Lamp bulb"),
		`✓ Debugging state machine "lamp" exhibited by object #1 of "Lamps::bulb"`,
		"attaches to that running machine rather than starting a second performance of it",
		"Current state: off")
	rejects(t, run(t, s, "%state Lamp bulb"), "Started state machine executor")

	// It is the exhibited machine: a send moves the object's own state.
	run(t, s, "%send go")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
	wants(t, run(t, s, "%stop"), "Stopped")
	wants(t, run(t, s, "%state bulb"), `✓ Debugging state machine "lamp"`, "Current state: on")

	// The usage form keeps attaching too.
	run(t, s, "%stop")
	wants(t, run(t, s, "%state Lamps::Bulb::lamp bulb"), `✓ Debugging state machine "lamp" exhibited by object #1`, "attaches to that running machine")
}

// TestSendDefersWhatTheActiveStateDefers: a signal the active state defers
// rather than accepts is sent, held by the machine once dispatched, and recalled
// to fire once a state accepting it is reached; one it neither accepts nor
// defers is still refused.
func TestSendDefersWhatTheActiveStateDefers(t *testing.T) {
	s := loadFixture(t, "testdata/deferring.sysml")
	run(t, s, "%instantiate Deferring::server")
	wants(t, run(t, s, "%state Deferring::server"), `✓ Debugging state machine "worker"`, "Current state: busy")

	wants(t, run(t, s, "%send Noise"), `accepts no signal Noise now: state machine "Worker" in state busy`)
	wants(t, run(t, s, "%send Ping"), "✓ Sent Ping", `Deferred by state machine "Worker" in state busy, to be dispatched once it leaves`)
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "  Ping")
	wants(t, run(t, s, "%step"), "Current state: busy", "Ping was deferred by the active state, to be dispatched again once it leaves")
	wants(t, run(t, s, "%events"), "Deferred by the active state, held until it leaves: 1", "  Ping")
	rejects(t, run(t, s, "%events"), "Signals in flight")

	wants(t, run(t, s, "%send Go"), `Accepted by state machine "Worker" in state busy: transition busy_ready fires on it`)
	wants(t, run(t, s, "%step"), "Current state: ready")
	wants(t, run(t, s, "%events"), "Event queue: 1 events")
	wants(t, run(t, s, "%step"), "Current state: finished")
	wants(t, run(t, s, "%events"), "Event queue empty")
}

// TestStateOnASecondMachineFollowsItOverARestart: a session attached to the
// second of an object's machines stays on that machine when an unrelated
// declaration restarts the object's machines, rather than falling back to the
// first.
func TestStateOnASecondMachineFollowsItOverARestart(t *testing.T) {
	s := loadFixture(t, "testdata/twin_machines.sysml")
	run(t, s, "%instantiate Twins::unit")
	wants(t, run(t, s, "%state Twins::Fan Twins::unit"),
		`✓ Debugging state machine "fan" exhibited by object #1 of "Twins::unit"`,
		"attaches to that running machine", "Current state: still")
	run(t, s, "%send spin")
	wants(t, run(t, s, "%advance 1"), "Current state: spinning")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	wants(t, strings.Join(res.Notices, "\n"), "2 behaviors of carried-over objects were restarted")

	// The restarted fan, not the restarted heater: it is still, and takes spin.
	wants(t, run(t, s, "%current"), "Current state: still")
	wants(t, run(t, s, "%send spin"), `Accepted by state machine "Fan" in state still`)
	wants(t, run(t, s, "%advance 1"), "Current state: spinning")
	// A signal only the object's other machine accepts goes to it, and a step of
	// the fan is not what dispatches it.
	out := run(t, s, "%send heat")
	wants(t, out, `Accepted by state machine "Heater" in state cold`)
	rejects(t, out, "Use %step")

	// The heater was restarted beside it, and is found by its definition.
	run(t, s, "%stop")
	wants(t, run(t, s, "%state Twins::Heater Twins::unit"), `Debugging state machine "heater"`, "attaches to that running machine", "Current state: cold")
}

// TestSendReachesTheMachineWhoseGuardLetsItThrough: of two machines on one
// object accepting the same signal, the first — whose guard refuses it — leaves
// it in flight for the second, whether it is stepped first or the signal is
// sent to the object while the second is being debugged.
func TestSendReachesTheMachineWhoseGuardLetsItThrough(t *testing.T) {
	s := loadFixture(t, "testdata/shared_signal.sysml")
	run(t, s, "%instantiate Shared::unit")
	wants(t, run(t, s, "%state Shared::Unit::heater Shared::unit"), `✓ Debugging state machine "heater"`, "Current state: cold")

	out := run(t, s, "%send tap")
	wants(t, out,
		`✓ Sent tap to object #1 of "Shared::unit"`,
		`Accepted by state machine "fan" in state still: transition`,
		`state machine "heater" in state cold would fire nothing on it, so a step of it leaves it to the machine above`)
	rejects(t, out, "Use %step")
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "  tap")

	// A step of the heater neither takes nor drops it.
	wants(t, run(t, s, "%step"), "State machine Suspended (cold)")
	wants(t, run(t, s, "%current"), "Current state: cold")
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "  tap")

	run(t, s, "%stop")
	wants(t, run(t, s, "%state Shared::Unit::fan Shared::unit"), "attaches to that running machine", "Current state: still")
	wants(t, run(t, s, "%step"), "Event dispatched")
	wants(t, run(t, s, "%current"), "Current state: spinning")
	wants(t, run(t, s, "%events"), "Event queue empty")

	// With the fan gone from still, nothing lets tap through, so it is refused.
	wants(t, run(t, s, "%send tap to Shared::unit"),
		`would fire no transition on tap now, so it was not sent: state machine "heater" in state cold, and the guard of every transition tap triggers is false`)
}

// TestStateOnAnObjectStartsWhatItDoesNotRun: an object exhibiting no machine of
// the definition gets a fresh executor performed on its behalf, and says so.
func TestStateOnAnObjectStartsWhatItDoesNotRun(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%instantiate plain")
	wants(t, run(t, s, "%state Lamp plain"),
		`✓ Started state machine executor for "Lamp"`,
		`of "Lamps::plain", which exhibits no running machine of this kind`,
		"Current state: off")
	wants(t, run(t, s, "%send go"), "✓ Sent go to object #", `of "Lamps::plain"`, `Accepted by state machine "Lamp" in state off`)
	wants(t, run(t, s, "%advance 1"), "Current state: on")
	// The bulb's own machine was not touched.
	run(t, s, "%stop")
	wants(t, run(t, s, "%state bulb"), "Current state: off")
}
