package repl

import (
	"strings"
	"testing"
)

// %features lists what an object holds for each feature of its type, nested
// expansion and object IDs included.
func TestFeaturesListsFeatureValues(t *testing.T) {
	for _, fixture := range []string{"testdata/nested_part.sysml", "testdata/collection_slots.sysml"} {
		name := "Nested::Car"
		if strings.Contains(fixture, "collection") {
			name = "Coll::Rig"
		}
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, fixture)
			run(t, s, "%instantiate "+name)
			wants(t, run(t, s, "%features "+name), "Features:")
		})
	}
}

// %slots was the pre-0.1.0 spelling and is gone, so it reads as any other
// unknown command rather than listing.
func TestSlotsIsNoLongerACommand(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Vehicle")

	got := run(t, s, "%slots Vehicle")
	wants(t, got, `unknown command "%slots"`)
	rejects(t, got, "mass = 1500.0")
	rejects(t, strings.Join(metaCommands(), " "), "%slots")
}

// %instantiate points the reader at the listing command.
func TestInstantiateSuggestsFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Vehicle"), "Use %features Vehicle to inspect")
}

// An unresolved name, a definition with no object, a feature value that cannot
// be materialized and a reset session each report rather than list.
func TestFeaturesErrorPaths(t *testing.T) {
	t.Run("no argument", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features"), "usage: %features <object>")
	})

	t.Run("unresolved name", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Nope"), "error: unresolved reference: Nope")
	})

	t.Run("no instance", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Vehicle"), "no instance of", "%instantiate")
	})

	t.Run("materialization error reaches the session status", func(t *testing.T) {
		s := submitted(t, unmaterializableModel)
		run(t, s, "%instantiate Demo::R")
		wants(t, run(t, s, "%features Demo::R"), "bad: <error:", "multiplicity violation")
		if !s.HasErrors() {
			t.Error("the listing did not carry the materialization failure into the session status")
		}
		if n := len(s.MaterializationFailures()); n != 1 {
			t.Errorf("%%features reported %d materialization failures, want 1", n)
		}
	})

	t.Run("lost objects are explained", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		run(t, s, "%instantiate Demo::Vehicle")
		run(t, s, "%clear")
		wants(t, run(t, s, "%features Demo::Vehicle"), "the session was reset")
	})
}

// Help and completion name the one spelling there is.
func TestFeaturesInHelpAndCompletion(t *testing.T) {
	help := strings.Join(helpText(), "\n")
	wants(t, help, "%features <object> [all|depth <n>] [json]")
	rejects(t, help, "%slots")

	got := NewSession().Complete("%fea", len("%fea"))
	if len(got.Candidates) != 1 || got.Candidates[0] != "%features" {
		t.Errorf("completing %%fea offered %v, want %%features", got.Candidates)
	}
	if strings.Contains(strings.Join(NewSession().Complete("%s", len("%s")).Candidates, " "), "%slots") {
		t.Error("completion still offers the removed spelling")
	}
}

// behaviorsModel is a part whose type exhibits a machine, performs an action,
// and declares an action and a state it neither performs nor exhibits.
const behaviorsModel = `private import ScalarValues::*;
state def LampModes {
    entry; then off;
    state off;
    accept after 10 [SI::s] then on;
    state on;
}
part def Lamp {
    attribute watts : Real = 60.0;
    exhibit state modes : LampModes;
    action blink { in n : Real; }
    perform action tick { first bump; action bump { assign watts := watts + 2.0; } }
    state standalone;
}
part lamp : Lamp;
part def Rig { attribute rpm : Integer = 0; part inner : Lamp; }
part rig : Rig;`

// A state or action a type declares holds no value, so %features lists it under
// its own heading with what the object is doing with it: the active state of the
// machine the object exhibits — the state %current reports, before and after the
// debugger drives it — the execution state of the action it performs, and "not
// running" for a behavior the object does not run. The behaviors follow the values,
// the library's included; an abstract library collection of behaviors
// (Part::performedActions) is a value among them, not a behavior of its own.
func TestFeaturesListsBehaviorsUnderTheirOwnHeading(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate lamp")

	got := run(t, s, "%features lamp")
	wantsInOrder(t, got, "Features:\n  watts = 62.0\n", "\n  performedActions = []\n", "\nBehaviors:\n  modes: exhibited state machine, current state off")
	wants(t, got,
		"  tick: performed action, completed",
		"  blink: action, not running",
		"  standalone: state, not running",
	)
	rejects(t, got, "<unknown>", "off = ", "on = ", "bump = ", "n = ", "modes = Instance",
		"performedActions: action", "ownedStates: state")

	wants(t, run(t, s, "%state lamp"), "Current state: off")
	wants(t, run(t, s, "%current"), "Current state: off")
	wants(t, run(t, s, "%features lamp"), "modes: exhibited state machine, current state off")

	wants(t, run(t, s, "%advance 10"), "Current state: on")
	wants(t, run(t, s, "%current"), "Current state: on")
	got = run(t, s, "%features lamp")
	wants(t, got, "modes: exhibited state machine, current state on")
	rejects(t, got, "current state off", "<unknown>")
}

// A nested object's behaviors are listed under its own row, not mixed into the
// listed object's.
func TestFeaturesListsNestedBehaviorsUnderTheNestedObject(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate rig")

	got := run(t, s, "%features rig")
	wantsInOrder(t, got, "  inner = Instance(ID: ", "    watts = 62.0\n", "\n    Behaviors:\n      modes: exhibited state machine, current state off")
	wants(t, got, "      tick: performed action, completed")
	rejects(t, got, "\nBehaviors:", "<unknown>")
}

// A running behavior's occurrence owns values of its own — a machine's attributes,
// an action's parameters and outputs — which the listing keeps under the behavior's
// row, distinct from the performer's same-named values, without listing the
// occurrence's states and steps as values.
func TestFeaturesListsTheValuesARunningBehaviorOwns(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`private import ScalarValues::*;
state def Counting {
    attribute count : Integer = 0;
    entry; then running;
    state running { entry action bump { assign count := count + 1; } }
}
part def Counter {
    attribute count : Integer = 10;
    exhibit state modes : Counting;
    perform action tick { out total : Integer = 7; action step; }
    action idle { in n : Real; }
}
part counter : Counter;`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate counter")

	got := run(t, s, "%features counter")
	wantsInOrder(t, got, "Features:\n  count = 10\n", "\nBehaviors:\n  modes: exhibited state machine, current state running\n    count = 1\n")
	wants(t, got,
		"  tick: performed action, completed\n    total = 7\n",
		"  idle: action, not running",
	)
	rejects(t, got, "<unknown>", "running = ", "step = ", "bump = ", "n = ", "modes = Instance", "tick = Instance")
	wants(t, run(t, s, "%eval in counter : modes.count"), "1")
	wants(t, run(t, s, "%eval in counter : tick.total"), "7")

	// The depth bound applies to an occurrence as to any nested object.
	got = run(t, s, "%features counter depth 0")
	wants(t, got,
		"  modes: exhibited state machine, current state running (not expanded: depth 0)",
		"  tick: performed action, completed (not expanded: depth 0)",
	)
	rejects(t, got, "count = 1\n", "total = 7")
}

// A redefinition renames the behavior it redefines, so the object runs one
// machine and one action under two names each: both names report that one
// execution, and neither reads "not running" while the other runs.
func TestFeaturesReportsARenamedBehaviorUnderBothNames(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel + `
part def FancyLamp :> Lamp {
    exhibit state fancyModes :>> modes;
    perform action fancyTick :>> tick { first bump; action bump { assign watts := watts + 3.0; } }
}
part fancy : FancyLamp;`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate fancy")

	got := run(t, s, "%features fancy")
	wants(t, got,
		"  fancyModes: exhibited state machine, current state off",
		"  modes: exhibited state machine, current state off",
		"  fancyTick: performed action, completed",
		"  tick: performed action, completed",
	)
	rejects(t, got, "modes: state, not running", "tick: action, not running", "<unknown>")

	wants(t, run(t, s, "%state fancy"), "Current state: off")
	wants(t, run(t, s, "%advance 10"), "Current state: on")
	got = run(t, s, "%features fancy")
	wants(t, got,
		"  fancyModes: exhibited state machine, current state on",
		"  modes: exhibited state machine, current state on",
	)
	rejects(t, got, "current state off", "modes: state, not running")
}

// An object of a state definition has states, not values: they are listed as
// behaviors the object does not run rather than as <unknown> values.
func TestFeaturesOfAMachineObjectListsItsStates(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate LampModes")

	got := run(t, s, "%features LampModes")
	wants(t, got, "Behaviors:", "  off: state, not running", "  on: state, not running")
	rejects(t, got, "<unknown>")
}

// A named transition is a feature too, and the object runs no such thing: it is
// listed as the step between states it declares, spelled as the model spells
// them, not as an idle action.
func TestFeaturesListsNamedTransitionsAsTransitions(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`private import ScalarValues::*;
state def DoorModes {
    entry; then closed;
    state closed;
    state opened;
    transition swing first closed then opened;
}
part def Door {
    attribute width : Real = 0.9;
    exhibit state modes : DoorModes;
    transition toggle first modes.closed then modes.opened;
}
part door : Door;`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}

	run(t, s, "%instantiate door")
	got := run(t, s, "%features door")
	wantsInOrder(t, got, "Features:\n  width = 0.9\n", "\nBehaviors:\n  modes: exhibited state machine, current state ")
	wants(t, got, "  toggle: transition, modes.closed → modes.opened")
	rejects(t, got, "toggle: action", "toggle = ", "<unknown>")

	run(t, s, "%instantiate DoorModes")
	got = run(t, s, "%features DoorModes")
	wants(t, got, "  closed: state, not running", "  opened: state, not running", "  swing: transition, closed → opened")
	rejects(t, got, "swing: action", "<unknown>")
}

// A transition end whose name is not a basic one — a name with spaces, a keyword —
// is rendered with its quotes, so the row reads as the model does and can be
// typed back into a command, with `.`, `::` and a `$::` root kept as written.
func TestFeaturesTransitionEndsKeepUnrestrictedNameQuotes(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`private import ScalarValues::*;
state def LampModes {
    entry; then 'switched off';
    state 'switched off';
    state 'state';
    transition 'turn on' first 'switched off' then 'state';
}
part def Lamp {
    attribute watts : Real = 40.0;
    exhibit state modes : LampModes;
    transition flip first modes.'switched off' then LampModes::'state';
    transition reset first $::LampModes::'state' then modes.'switched off';
}
part lamp : Lamp;`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}

	run(t, s, "%instantiate lamp")
	got := run(t, s, "%features lamp")
	wants(t, got,
		"  flip: transition, modes.'switched off' → LampModes::'state'",
		"  reset: transition, $::LampModes::'state' → modes.'switched off'",
	)
	rejects(t, got, "modes.switched off", "::state\n", "transition, LampModes::'state'")

	run(t, s, "%instantiate LampModes")
	got = run(t, s, "%features LampModes")
	wants(t, got, "  turn on: transition, 'switched off' → 'state'")
	rejects(t, got, "transition, switched off")
}
