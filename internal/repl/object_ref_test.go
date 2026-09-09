package repl

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// objectRefModel has a nested part and a multi-valued part feature, so an object
// reference can walk a single object, pick one of several, and go one level
// further down.
const objectRefModel = `package Garage {
	part def Hub {
		attribute bolts = 5;
	}
	part def Wheel {
		attribute radius = 0.3;
		part hub : Hub;
	}
	part def Car {
		attribute mass = 1500.0;
		part fl : Wheel;
		part wheels : Wheel[2];
	}
	part car : Car;
	part def Spare {
		attribute count = 1;
	}
}`

func garage(t *testing.T) *Session {
	t.Helper()
	return submitted(t, objectRefModel)
}

// %instantiate prints the id the runtime gives the object, and #<id> is how a
// command reaches it afterwards.
func TestObjectAddressedByID(t *testing.T) {
	s := garage(t)
	out := run(t, s, "%instantiate car")
	inst, ok := s.instances["Garage::car"]
	if !ok {
		t.Fatal("car was not recorded")
	}
	wants(t, out, fmt.Sprintf("ID: %d", inst.ID))

	id := fmt.Sprintf("#%d", inst.ID)
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, inst.ID), "mass = 1500.0")
	wants(t, run(t, s, "%eval in "+id+" : mass * 2.0"), "3000.0", fmt.Sprintf("(on %s ID: %d)", id, inst.ID))
}

// A second %instantiate of a name re-points the name; the first object keeps its
// id and stays reachable by it, and %instances still lists it.
func TestInstantiateTwiceKeepsFirstObjectByID(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	first := s.instances["Garage::car"]
	again := run(t, s, "%instantiate car")
	second := s.instances["Garage::car"]
	if first.ID == second.ID {
		t.Fatalf("both instantiations report id %d", first.ID)
	}
	wants(t, again, fmt.Sprintf("object #%d is displaced from that name", first.ID), fmt.Sprintf("stays reachable as #%d", first.ID))

	firstID := fmt.Sprintf("#%d", first.ID)
	wants(t, run(t, s, "%features "+firstID), fmt.Sprintf("Instance: %s (ID: %d)", firstID, first.ID))
	wants(t, run(t, s, "%features car"), fmt.Sprintf("(ID: %d)", second.ID))
	wants(t, run(t, s, "%instances"),
		fmt.Sprintf("Garage::car (ID: %d)", second.ID),
		fmt.Sprintf("%s (ID: %d, displaced from Garage::car)", firstID, first.ID))

	// Each object has its own nested parts, reached through its own root.
	fl := run(t, s, "%features "+firstID+".fl")
	wants(t, fl, "Instance: "+firstID+".fl")
	rejects(t, fl, "error")
}

// A reset counts the displaced object among what it drops, as %instances listed it.
func TestResetCountsDisplacedObjects(t *testing.T) {
	t.Run("clear", func(t *testing.T) {
		s := garage(t)
		run(t, s, "%instantiate car")
		run(t, s, "%instantiate car")
		wants(t, run(t, s, "%clear"), "2 instances were dropped because the session was reset")
		wants(t, run(t, s, "%instances"), "2 instances were dropped when the session was reset")
	})
	t.Run("budgets", func(t *testing.T) {
		s := garage(t)
		run(t, s, "%instantiate car")
		run(t, s, "%instantiate car")
		budgets := s.Budgets()
		budgets.MaxSteps++
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		wants(t, run(t, s, "%instances"), "2 instances were dropped when the run bounds were changed")
	})
}

// New run bounds drop the runtime context, and with it the debuggers driving
// it: the next object gets the old id, and must not be taken for the performer.
func TestBudgetsEndDebuggers(t *testing.T) {
	rebound := func(t *testing.T, s *Session) {
		budgets := s.Budgets()
		budgets.MaxSteps++
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
	}
	t.Run("exhibited machine", func(t *testing.T) {
		s := loadFixture(t, "testdata/exhibited_machine.sysml")
		first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		wants(t, run(t, s, "%state Obj::Monitor"), "exhibited by object #"+first)
		// Carried over a declaration, the debugger keeps the replaced context's ids
		// alive; new bounds end it, so they restart.
		s.Submit("package Other { part def Unrelated; }")
		if s.replaced == nil {
			t.Fatal("the carried debugger did not keep the replaced context's ids")
		}
		rebound(t, s)
		if s.stateExec != nil {
			t.Fatal("the state machine session outlived its runtime context")
		}
		if s.replaced != nil {
			t.Fatal("the replaced context's ids outlived the debugger")
		}
		again := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		if again != first {
			t.Fatalf("the new context gave id %s, want %s reused", again, first)
		}
		wants(t, run(t, s, "%advance 10"),
			`no active debugging session: the state machine session for "Obj::Monitor" ended when the run bounds were changed`)
		wants(t, run(t, s, "%features #"+first), "count = 1")
	})
	t.Run("action performer", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Rack { attribute size = 1.0; }")
		s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		run(t, s, "%instantiate Rack")
		wants(t, run(t, s, "%action tally Rack"), "Started action executor")
		rebound(t, s)
		if s.actionExec != nil {
			t.Fatal("the action session outlived its runtime context")
		}
		run(t, s, "%instantiate Rack")
		wants(t, run(t, s, "%step"),
			`no active action session: the action session for "tally" ended when the run bounds were changed`)
	})
}

// Dotted paths rooted at a name or an id, and the `::` form, walk to the same
// nested object, which is reported under its declared root.
func TestDottedPathsReachNestedObjects(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	id := fmt.Sprintf("#%d", car.ID)

	byName := run(t, s, "%features car.fl")
	wants(t, byName, "Instance: Garage::car.fl (ID:", "radius = 0.3")
	wants(t, run(t, s, "%features Garage::car.fl"), "Instance: Garage::car.fl (ID:")
	wants(t, run(t, s, "%features Garage::car::fl"), "Instance: Garage::car.fl (ID:")
	wants(t, run(t, s, "%features car.fl.hub"), "Instance: Garage::car.fl.hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features "+id+".fl.hub"), "Instance: "+id+".fl.hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features "+id+"::fl"), "Instance: "+id+".fl (ID:")

	// The nested object's own id reaches it too, and reads the same values.
	flLine := strings.SplitN(byName, "\n", 2)[0]
	var flID int64
	if _, err := fmt.Sscanf(flLine, "Instance: Garage::car.fl (ID: %d)", &flID); err != nil {
		t.Fatalf("no id in %q: %v", flLine, err)
	}
	wants(t, run(t, s, fmt.Sprintf("%%features #%d", flID)), "radius = 0.3")
	wants(t, run(t, s, fmt.Sprintf("%%eval in #%d : radius", flID)), "0.3")
	wants(t, run(t, s, "%eval in car.fl : radius * 2.0"), "0.6", "(on Garage::car.fl ID:")
	wants(t, run(t, s, "%eval in "+id+".fl.hub : bolts"), "5", "(on "+id+".fl.hub ID:")
}

// A segment after `.` is a feature of the object before it, even when the same
// spelling with `::` names a declaration the session holds an object under; a
// package before `.` is reported as one, with the `::` spelling to use.
func TestDotWalksFeaturesNotDeclarations(t *testing.T) {
	s := garage(t)
	wants(t, run(t, s, "%instantiate Garage::Car::fl"), "✓ Created instance of Garage::Car::fl", "ID: 1")
	wants(t, run(t, s, "%instantiate car"), "ID: 2")
	wants(t, run(t, s, "%features Garage::Car::fl"), "Instance: Garage::Car::fl (ID: 1)", "hub = Instance(ID: 3)")
	// Garage::car::fl resolves as the declaration Garage::Car::fl, so the `::`
	// spelling reaches the object created under it; the `.` spelling walks car.
	wants(t, run(t, s, "%features Garage::car::fl"), "Instance: Garage::Car::fl (ID: 1)")
	wants(t, run(t, s, "%features Garage::car.fl"), "Instance: Garage::car.fl (ID: 4)")
	wants(t, run(t, s, "%features car.fl"), "Instance: Garage::car.fl (ID: 4)")
	wants(t, run(t, s, "%eval in Garage::car.fl : radius"), "(on Garage::car.fl ID: 4)")
	wants(t, run(t, s, "%features #2.fl"), "Instance: #2.fl (ID: 4)")

	wants(t, run(t, s, "%features Garage.car"), `error: "Garage.car" is not an object reference: Garage is a package, not an object: its member is written Garage::car`)
	wants(t, run(t, s, "%features Garage.car.fl"), `error: "Garage.car.fl" is not an object reference: Garage is a package, not an object: its member is written Garage::car`)
	wants(t, run(t, s, "%features Car.fl"), `error: no instance of the definition "Garage::Car" itself: object #2 of "Garage::car" is typed by it — name Garage::car to address it, or use %instantiate Garage::Car to create an object of the definition`)
	wants(t, run(t, s, "%features Nope.fl"), "error: unresolved reference: Nope")
}

// A multi-valued feature is walked by a 1-based index; leaving the index out, or
// giving one that names no element, says how many there are.
func TestMultiValuedFeatureNeedsIndex(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")

	wants(t, run(t, s, "%features car.wheels"), "error:", "wheels of Garage::car holds 2 objects: pick one by index, wheels[1] to wheels[2]")
	wants(t, run(t, s, "%features car.wheels[1]"), "Instance: Garage::car.wheels[1] (ID:", "radius = 0.3")
	wants(t, run(t, s, "%features car.wheels[2].hub"), "Instance: Garage::car.wheels[2].hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features Garage::car::wheels[2]"), "Instance: Garage::car.wheels[2] (ID:")
	wants(t, run(t, s, "%features car.wheels[3]"), "error:", "wheels of Garage::car holds 2 objects, so wheels[3] names none (indexes run from 1 to 2)")
	wants(t, run(t, s, "%features car.wheels[0]"), "error:", "wheels[0] is not an index: elements are counted from 1")
	wants(t, run(t, s, "%features car.fl[1]"), "error:", "fl of Garage::car holds one value and takes no index: write fl, not fl[1]")
	wants(t, run(t, s, "%features car[1]"), "error:", "car[1] takes no index")

	one := run(t, s, "%features car.wheels[1]")
	two := run(t, s, "%features car.wheels[2]")
	if strings.SplitN(one, "\n", 2)[0] == strings.SplitN(two, "\n", 2)[0] {
		t.Errorf("wheels[1] and wheels[2] report the same object:\n%s", one)
	}
}

// Every command reports a bad reference in the same words: an unknown id lists
// the ids there are, a bad segment names the object and the segment, and a
// name with no object says so.
func TestObjectReferenceErrors(t *testing.T) {
	s := garage(t)
	wants(t, run(t, s, "%features #7"), "error: no object #7 in this session: nothing materialized has that identity (no objects have been created)")
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]

	// Nested parts are materialized when first read, so they join the listing.
	wants(t, run(t, s, "%features #999"), fmt.Sprintf("error: no object #999 in this session: nothing materialized has that identity (the objects are #%d)", car.ID))
	run(t, s, "%features car.fl")
	wants(t, run(t, s, "%features #999"), fmt.Sprintf("error: no object #999 in this session: nothing materialized has that identity (the objects are #%d, #", car.ID))
	wants(t, run(t, s, "%features car.nope"), "error: Garage::car has no feature \"nope\" (its features are fl, mass, wheels, and 13 more the library declares)")
	wants(t, run(t, s, "%features car.fl.hub.bolts"), "error: bolts of Garage::car.fl.hub holds a value (5), not an object")
	wants(t, run(t, s, "%features car.mass"), "error: mass of Garage::car holds a value (1500.0), not an object")
	wants(t, run(t, s, "%features #"), "error:", "an object id is written #<id>")
	wants(t, run(t, s, "%features #0"), "error:", "#0 is not an object id (ids count up from 1)")
	wants(t, run(t, s, "%features car."), "error:", `it ends in "." with no feature after it`)
	wants(t, run(t, s, "%features Spare"), "error: no instance of \"Garage::Spare\" (use %instantiate first)")
	wants(t, run(t, s, "%features Spare.count"), "error: no instance of \"Garage::Spare\" (use %instantiate first)")
	wants(t, run(t, s, "%features Nope.x"), "error: unresolved reference: Nope")

	// The other object-taking commands go through the same resolution.
	for _, cmd := range []string{"%eval in #999 : mass", "%invoke #999 go", "%state #999"} {
		out, _, err := s.RunMeta(cmd)
		msg := strings.Join(out, "\n")
		if err != nil {
			msg = err.Error()
		}
		wants(t, msg, "no object #999 in this session: nothing materialized has that identity (the objects are")
	}
	for _, cmd := range []string{"%eval in car.nope : mass", "%invoke car.nope go"} {
		out, _, err := s.RunMeta(cmd)
		msg := strings.Join(out, "\n")
		if err != nil {
			msg = err.Error()
		}
		wants(t, msg, `Garage::car has no feature "nope"`)
	}
}

// The debuggers take an object by id or path too: the machine an object
// exhibits, the operation it owns, and a behavior it performs.
func TestDebuggersAddressObjectsByID(t *testing.T) {
	t.Run("exhibited machine and operation", func(t *testing.T) {
		s := loadFixture(t, "testdata/exhibited_machine.sysml")
		first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		run(t, s, "%instantiate Obj::Monitor")

		wants(t, run(t, s, "%state #"+first), "exhibited by object #"+first, "Current state: idle")
		wants(t, run(t, s, "%current"), "idle")
		wants(t, run(t, s, "%invoke #"+first+" bumpBy n=4"), "✓ Invoked bumpBy on object #"+first)
		wants(t, run(t, s, "%features #"+first), "count = 5")
		// The named object is untouched.
		wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")
		wants(t, run(t, s, "%eval in #"+first+" : count"), "5")
	})

	t.Run("performing object", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Holder { attribute size = 1.0; }")
		res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		if len(res.Diagnostics) > 0 {
			t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
		}
		id := objectIDIn(t, run(t, s, "%instantiate Holder"))
		wants(t, run(t, s, "%action tally #"+id), "Started action executor")
		wants(t, run(t, s, "%action tally #99"), "error:", "no object #99 in this session: nothing materialized has that identity (the objects are #"+id+")")
		s.Submit("part def Holder { attribute size = 2.0; }")
		wants(t, run(t, s, "%step"), "no active action session", "the object #"+id+" performing it was dropped")
	})
}

// A debugger started on a named object stays on that object when the name is
// given to a second one: after an unrelated declaration rebuilds the runtime it
// drives the first object's machine, and losing that object is what ends it.
func TestDebuggerFollowsDisplacedObject(t *testing.T) {
	t.Run("exhibited machine", func(t *testing.T) {
		s := loadFixture(t, "testdata/exhibited_machine.sysml")
		first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		wants(t, run(t, s, "%state Obj::Monitor"), "exhibited by object #"+first)
		second := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		if got := s.stateExec.selfFQN; got != "#"+first {
			t.Fatalf("debugger label after displacement = %q, want #%s", got, first)
		}
		res := s.Submit("package Other { part def Unrelated; }")
		if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
			t.Fatalf("declaration has errors: %v", errs)
		}
		if s.stateExec == nil {
			t.Fatal("the debugging session ended")
		}
		machine, ok := s.unnamed[0].obj.ExhibitedState()
		if !ok || machine.State != s.stateExec.executor {
			t.Fatalf("debugger drives another object's machine, not #%s's", first)
		}
		wants(t, run(t, s, "%advance 10"), "Current state: awake")
		wants(t, run(t, s, "%features #"+first), "count = 11")
		wants(t, run(t, s, "%features #"+second), "count = 1")

		// Redeclaring the definition drops the displaced object, which ends it.
		s.Submit("package Obj { part def Monitor { attribute count = 0; } }")
		wants(t, run(t, s, "%advance 1"), "the object #"+first+" performing it was dropped")
	})

	t.Run("performing object", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Holder { attribute size = 1.0; }")
		res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		if len(res.Diagnostics) > 0 {
			t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
		}
		id := objectIDIn(t, run(t, s, "%instantiate Holder"))
		wants(t, run(t, s, "%action tally Holder"), "Started action executor")
		run(t, s, "%instantiate Holder")
		if got := s.actionExec.selfFQN; got != "#"+id {
			t.Fatalf("performer label after displacement = %q, want #%s", got, id)
		}
		s.Submit("package Other { part def Unrelated; }")
		wants(t, run(t, s, "%step"), "State:")
		s.Submit("part def Holder { attribute size = 2.0; }")
		wants(t, run(t, s, "%step"), "no active action session", "the object #"+id+" performing it was dropped")
	})

	t.Run("nested performing object", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Holder { attribute size = 1.0; }\npart def Rack { part holder : Holder; }")
		s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		rack := objectIDIn(t, run(t, s, "%instantiate Rack"))
		wants(t, run(t, s, "%action tally Rack.holder"), "Started action executor")
		run(t, s, "%instantiate Rack")
		if got, want := s.actionExec.selfFQN, "#"+rack+".holder"; got != want {
			t.Fatalf("performer label after displacement = %q, want %s", got, want)
		}
		s.Submit("package Other { part def Unrelated; }")
		wants(t, run(t, s, "%step"), "State:")
	})

	// The nested object's label reads back as it, not as the object created under
	// the declaration Rack::holder, so the debugger neither starts on that one nor
	// switches to it when the rack's name is displaced.
	t.Run("nested object beside its declaration's own object", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Holder { attribute size = 1.0; }\npart def Rack { part holder : Holder; }")
		s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		run(t, s, "%instantiate Rack::holder")
		rack := objectIDIn(t, run(t, s, "%instantiate Rack"))
		nested, _, err := s.resolveObject("#" + rack + ".holder")
		if err != nil {
			t.Fatal(err)
		}
		performer := func(when string) {
			held, ok := s.heldObject(s.actionExec.selfFQN)
			if !ok || held != nested {
				t.Fatalf("%s, label %q does not reach the nested object #%d", when, s.actionExec.selfFQN, nested.ID)
			}
		}
		wants(t, run(t, s, "%action tally Rack.holder"), "Started action executor")
		if got, want := s.actionExec.selfFQN, "Rack.holder"; got != want {
			t.Fatalf("performer label = %q, want %s", got, want)
		}
		performer("at start")
		run(t, s, "%instantiate Rack")
		if got, want := s.actionExec.selfFQN, "#"+rack+".holder"; got != want {
			t.Fatalf("performer label after displacement = %q, want %s", got, want)
		}
		performer("after displacement")
		s.Submit("package Other { part def Unrelated; }")
		wants(t, run(t, s, "%step"), "State:")
		performer("after carry-over")
		s.Submit("part def Rack { part holder : Holder; attribute slots = 2; }")
		wants(t, run(t, s, "%step"), "no active action session", "the object #"+rack+".holder performing it was dropped")
	})
}

// Ids survive the carry-over an unrelated declaration triggers: the same
// objects, nested ones included, answer to the same ids and paths afterwards.
func TestObjectIDsStableAcrossCarryover(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	id := fmt.Sprintf("#%d", car.ID)
	before := run(t, s, "%features "+id+".fl")

	res := s.Submit("package Other { part def Unrelated; }")
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("declaration has errors: %v", errs)
	}
	if s.instances["Garage::car"] != car {
		t.Fatalf("car was replaced across the carry-over")
	}
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, car.ID), "mass = 1500.0")
	after := run(t, s, "%features "+id+".fl")
	if strings.SplitN(before, "\n", 2)[0] != strings.SplitN(after, "\n", 2)[0] {
		t.Errorf("the nested object changed id across the carry-over:\n%s\n---\n%s", before, after)
	}
	wants(t, run(t, s, "%instances"), fmt.Sprintf("Garage::car (ID: %d)", car.ID))

	// An object a later %instantiate displaced is carried too, still by its id.
	run(t, s, "%instantiate car")
	second := s.instances["Garage::car"]
	res = s.Submit("package Other { part def Unrelated; part def Another; }")
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("declaration has errors: %v", errs)
	}
	if s.instances["Garage::car"] != second {
		t.Fatalf("the second car was replaced across the carry-over")
	}
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, car.ID))
	wants(t, run(t, s, "%instances"),
		fmt.Sprintf("Garage::car (ID: %d)", second.ID),
		fmt.Sprintf("%s (ID: %d, displaced from Garage::car)", id, car.ID))
}

// Each object is carried over on its own, so a change can drop the one named
// while a displaced one, which never selected what changed, survives; %instances
// then lists the survivor by its id rather than reporting an empty session.
func TestInstancesListsADisplacedObjectThatAloneSurvived(t *testing.T) {
	s := submitted(t, `package Demo {
	part def Engine { attribute size default = 1.0; }
	abstract part family {
		variation part engine : Engine {
			variant part electric : Engine;
			variant part petrol : Engine;
		}
	}
	part sedan :> family { part :>> engine = engine::electric; }
}`)
	run(t, s, "%instantiate sedan")
	first := s.instances["Demo::sedan"]
	run(t, s, "%instantiate sedan")
	second := s.instances["Demo::sedan"]
	wants(t, run(t, s, "%features sedan"), "engine = electric")

	res := s.Submit(`package Demo {
	abstract part family {
		variation part engine : Engine {
			variant part electric : Engine { attribute :>> size = 3.0; }
			variant part petrol : Engine;
		}
	}
}`)
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("declaration has errors: %v", errs)
	}
	wants(t, strings.Join(res.Notices, "\n"), "1 instance was dropped")
	if _, named := s.instances["Demo::sedan"]; named {
		t.Fatalf("the object that selected the changed variant, #%d, was carried over", second.ID)
	}
	id := fmt.Sprintf("#%d", first.ID)
	listing := run(t, s, "%instances")
	wants(t, listing, "Instances:",
		fmt.Sprintf("%s (ID: %d, displaced from Demo::sedan)", id, first.ID),
		"(1 instance was also dropped when the declarations changed at submission 2 — re-run %instantiate)")
	rejects(t, listing, "no instances created", fmt.Sprintf("ID: %d)", second.ID))
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, first.ID), "engine = electric", "size = 3.0")
}

// Where a command takes an object, completion offers the ids the session holds
// and walks a typed reference into the object-holding features of the object
// it names, an element of a multi-valued one picked by index; elsewhere names
// complete as before.
func TestCompleteObjectReferences(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	id := fmt.Sprintf("#%d", s.instances["Garage::car"].ID)

	complete := func(line string) Completion {
		return s.Complete(line, len(line))
	}
	tests := []struct {
		line    string
		prefix  string
		wants   []string
		rejects []string
	}{
		{line: "%features ", prefix: "", wants: []string{id, "car", "Garage"}},
		{line: "%features #", prefix: "#", wants: []string{id}, rejects: []string{"car"}},
		{line: "%features ca", prefix: "ca", wants: []string{"car"}, rejects: []string{id}},
		{line: "%features car.", prefix: "car.", wants: []string{"car.fl", "car.wheels[1]", "car.wheels[2]"}, rejects: []string{"car.mass"}},
		{line: "%features car.f", prefix: "car.f", wants: []string{"car.fl"}, rejects: []string{"car.wheels[1]"}},
		{line: "%features car.wheels[", prefix: "car.wheels[", wants: []string{"car.wheels[1]", "car.wheels[2]"}, rejects: []string{"car.fl"}},
		{line: "%features car.fl.", prefix: "car.fl.", wants: []string{"car.fl.hub"}},
		{line: "%features car::", prefix: "car::", wants: []string{"car::fl"}},
		{line: "%features " + id + ".", prefix: id + ".", wants: []string{id + ".fl", id + ".wheels[1]"}},
		{line: "%features Garage::", prefix: "Garage::", wants: []string{"Garage::car"}},
		{line: "%eval in ", prefix: "", wants: []string{id, "car"}},
		{line: "%eval in car.", prefix: "car.", wants: []string{"car.fl"}},
		{line: "%invoke ", prefix: "", wants: []string{id}},
		{line: "%state Sm ", prefix: "", wants: []string{id}},
		{line: "%action Act ", prefix: "", wants: []string{id}},
		{line: "%action ", prefix: "", rejects: []string{id}},
		{line: "%show ", prefix: "", wants: []string{"car"}, rejects: []string{id}},
		{line: "%eval in car : ", prefix: "", rejects: []string{id}},
		{line: "%features nobody.", prefix: "nobody.", rejects: []string{"car.fl"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := complete(tt.line)
			if got.Prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", got.Prefix, tt.prefix)
			}
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("want %q in %v", want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("did not want %q in %v", bad, got.Candidates)
				}
			}
		})
	}
}

// A quoted segment still being typed keeps the root and separator before it,
// after `.` and `::` alike, an escaped quote inside it does not end it, and a
// qualified root is offered as the notation spells it, quoted where needed.
func TestCompleteQuotedSegments(t *testing.T) {
	s := submitted(t, `package Q {
	part def Gauge;
	part def Rack {
		part 'main gauge' : Gauge;
		part 'main valve' : Gauge;
		part 'rack\'s spare' : Gauge;
	}
	part 'the rack' : Rack;
}
package 'Two Words' {
	part def Car;
	part car : Car;
}`)
	run(t, s, "%instantiate 'the rack'")
	before := s.rtCtx.InstanceIDs()

	tests := []struct {
		line    string
		prefix  string
		wants   []string
		rejects []string
	}{
		{line: "%features 'the ra", prefix: "'the ra", wants: []string{"'the rack'"}},
		{line: "%features Q::'the ra", prefix: "Q::'the ra", wants: []string{"Q::'the rack'"}},
		{line: "%features Q::", prefix: "Q::", wants: []string{"Q::'the rack'", "Q::Rack"}},
		{line: "%features 'Two Words'::ca", prefix: "'Two Words'::ca", wants: []string{"'Two Words'::car"}},
		{line: "%features 'Two W", prefix: "'Two W", wants: []string{"'Two Words'"}},
		{line: "%print 'the ra", prefix: "'the ra", wants: []string{"'the rack'"}},
		{line: "%print 'Two Words'::c", prefix: "'Two Words'::c", wants: []string{"'Two Words'::car"}},
		{line: "%features 'the rack'.", prefix: "'the rack'.", wants: []string{"'the rack'.'main gauge'", "'the rack'.'main valve'", `'the rack'.'rack\'s spare'`}},
		{line: "%features 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'", "'the rack'.'main valve'"}, rejects: []string{`'the rack'.'rack\'s spare'`}},
		{line: "%features 'the rack'::'main g", prefix: "'the rack'::'main g", wants: []string{"'the rack'::'main gauge'"}, rejects: []string{"'the rack'::'main valve'"}},
		{line: `%features 'the rack'.'rack\'s`, prefix: `'the rack'.'rack\'s`, wants: []string{`'the rack'.'rack\'s spare'`}},
		{line: "%features Q::'the rack'.'main v", prefix: "Q::'the rack'.'main v", wants: []string{"Q::'the rack'.'main valve'"}},
		{line: "%eval in 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'"}},
		{line: "%state Sm 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'"}},
		{line: "%features 'the rack'.'nothing", prefix: "'the rack'.'nothing", rejects: []string{"'the rack'.'main gauge'"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := s.Complete(tt.line, len(tt.line))
			if got.Prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", got.Prefix, tt.prefix)
			}
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("want %q in %v", want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("did not want %q in %v", bad, got.Candidates)
				}
			}
		})
	}
	if after := s.rtCtx.InstanceIDs(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("completion materialized objects: ids %v, were %v", after, before)
	}
	if out := run(t, s, "%features 'the rack'.'main gauge'"); strings.Contains(out, "error") {
		t.Errorf("completed reference does not resolve:\n%s", out)
	}
}

// A part is completed to the elements it holds, not to what its multiplicity
// admits: a ranged part materializes its lower bound, an optional one only what
// subsets it — and every reference offered is one that then resolves.
func TestCompleteIndexesOnlyHeldElements(t *testing.T) {
	s := submitted(t, `package Fleet {
	part def Wheel;
	part def Truck {
		part axles : Wheel[2..4];
		part spare : Wheel[0..1];
		part crew : Wheel[0..*];
		part backup : Wheel[0..1];
		part kept : Wheel subsets backup;
	}
	part truck : Truck;
}`)
	run(t, s, "%instantiate truck")
	got := s.Complete("%features truck.", len("%features truck."))
	for _, want := range []string{"truck.axles[1]", "truck.axles[2]", "truck.backup", "truck.kept"} {
		if !contains(got.Candidates, want) {
			t.Errorf("want %q in %v", want, got.Candidates)
		}
	}
	for _, bad := range []string{"truck.axles[3]", "truck.axles[4]", "truck.spare", "truck.spare[1]", "truck.crew[1]"} {
		if contains(got.Candidates, bad) {
			t.Errorf("did not want %q in %v", bad, got.Candidates)
		}
	}
	for _, c := range got.Candidates {
		if _, _, err := s.resolveObject(c); err != nil {
			t.Errorf("completion %q does not resolve: %v", c, err)
		}
	}
}

// A collection holds the objects of the features subsetting it, then anonymous
// objects up to its lower bound; completion offers exactly those indexes before
// anything is materialized, for an object it holds and for one known by type.
func TestCompleteIndexesSubsettingContributions(t *testing.T) {
	s := submitted(t, `package Fleet {
	part def Wheel { attribute radius = 0.3; }
	part def Car {
		part wheels : Wheel[0..4];
		part frontLeft : Wheel subsets wheels;
		part frontRight : Wheel subsets wheels;
		part axles : Wheel[3];
		part rear : Wheel subsets axles;
		part slots : Wheel[1..*];
		part filled : Wheel[2] subsets slots;
	}
	part def Garage { part parked : Car; }
	part car : Car;
	part garage : Garage;
}`)
	run(t, s, "%instantiate car")
	run(t, s, "%instantiate garage")
	ids := fmt.Sprint(s.rtCtx.InstanceIDs())
	for _, root := range []string{"car", "garage.parked"} {
		got := s.Complete("%features "+root+".", len("%features "+root+".")).Candidates
		want := []string{root + ".axles[1]", root + ".axles[2]", root + ".axles[3]",
			root + ".filled[1]", root + ".filled[2]", root + ".frontLeft", root + ".frontRight", root + ".rear",
			root + ".slots[1]", root + ".slots[2]", root + ".wheels[1]", root + ".wheels[2]"}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: completion offered %v, want %v", root, got, want)
		}
	}
	if now := fmt.Sprint(s.rtCtx.InstanceIDs()); now != ids {
		t.Fatalf("completion materialized objects: ids %s, were %s", now, ids)
	}
	for _, root := range []string{"car", "garage.parked"} {
		for _, c := range s.Complete("%features "+root+".", len("%features "+root+".")).Candidates {
			if _, _, err := s.resolveObject(c); err != nil {
				t.Errorf("completion %q does not resolve: %v", c, err)
			}
		}
		for _, bad := range []string{".wheels[3]", ".axles[4]", ".slots[3]"} {
			if _, _, err := s.resolveObject(root + bad); err == nil {
				t.Errorf("%s%s resolves, though completion did not offer it", root, bad)
			}
		}
	}
}

// A redefinition renames a collection — the redefined name is no longer a member
// of the type, though the object still holds the value under both — and a feature
// subsetting the new name contributes to it; completion counts that contribution
// for a nested object known only by type, and offers the same indexes once it exists.
func TestCompleteIndexesThroughRedefinedCollection(t *testing.T) {
	s := submitted(t, `package Fleet {
	part def Wheel { attribute radius = 0.3; }
	part def Axle { part wheels : Wheel[0..4]; }
	part def FrontAxle :> Axle {
		part front :>> wheels;
		part spare : Wheel subsets front;
	}
	part def Car { part axle : FrontAxle; }
	part car : Car;
}`)
	run(t, s, "%instantiate car")
	ids := fmt.Sprint(s.rtCtx.InstanceIDs())
	want := []string{"car.axle.front[1]", "car.axle.spare", "car.axle.wheels[1]"}
	got := s.Complete("%features car.axle.", len("%features car.axle.")).Candidates
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("completion before materialization offered %v, want %v", got, want)
	}
	if now := fmt.Sprint(s.rtCtx.InstanceIDs()); now != ids {
		t.Fatalf("completion materialized objects: ids %s, were %s", now, ids)
	}
	for _, c := range want {
		if _, _, err := s.resolveObject(c); err != nil {
			t.Errorf("completion %q does not resolve: %v", c, err)
		}
	}
	got = s.Complete("%features car.axle.", len("%features car.axle.")).Candidates
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("completion after materialization offered %v, want %v", got, want)
	}
	wants(t, run(t, s, "%features car.axle.wheels[1]"), "Instance: Fleet::car.axle.wheels[1] (ID: 3)")
	wants(t, run(t, s, "%features car.axle.front[1]"), "Instance: Fleet::car.axle.front[1] (ID: 3)")
	wants(t, run(t, s, "%features car.axle.spare"), "Instance: Fleet::car.axle.spare (ID: 3)")
}

// A path follows every feature the runtime holds an object for — a structured
// attribute (an `attribute def` with attributes of its own) as much as a part —
// and completion offers exactly those, before and after they are materialized;
// an attribute holding a plain value is offered by neither.
func TestStructuredAttributesAreObjects(t *testing.T) {
	s := submitted(t, `package Geo {
	attribute def Point {
		attribute x : ScalarValues::Real = 1.0;
		attribute y : ScalarValues::Real = 2.0;
	}
	part def Hub { attribute bolts : ScalarValues::Integer = 5; }
	part def Wheel {
		attribute center : Point;
		attribute radius : ScalarValues::Real = 0.3;
		part hub : Hub;
	}
	part wheel : Wheel;
}`)
	run(t, s, "%instantiate wheel")
	check := func(when string) {
		got := s.Complete("%features wheel.", len("%features wheel.")).Candidates
		if want := []string{"wheel.center", "wheel.hub"}; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: completion offered %v, want %v", when, got, want)
		}
		for _, c := range got {
			if _, _, err := s.resolveObject(c); err != nil {
				t.Errorf("%s: completion %q does not resolve: %v", when, c, err)
			}
		}
	}
	check("before materialization")
	wants(t, run(t, s, "%features wheel.center"), "Instance: Geo::wheel.center", "x = 1.0", "y = 2.0")
	wants(t, run(t, s, "%eval in wheel.center : x"), "= 1.0")
	wants(t, run(t, s, "%eval in wheel::center : y"), "= 2.0")
	wants(t, run(t, s, "%features wheel.radius"), "error:", "radius of Geo::wheel holds a value (0.3), not an object")
	wants(t, run(t, s, "%features wheel.center.x"), "error:", "x of Geo::wheel.center holds a value (1.0), not an object")
	check("after materialization")
	id := objectIDIn(t, run(t, s, "%features wheel.center"))
	wants(t, run(t, s, "%features #"+id), "Instance: #"+id+" (ID: "+id+")", "x = 1.0")
}

// A variation's object is of whichever variant is selected, so completion offers
// the path to it once the selection is materialized, and the object-holding
// features of the variant beyond it, still reading and materializing nothing.
func TestCompleteSelectedVariation(t *testing.T) {
	s := submitted(t, `package Demo {
	part def Block { attribute mass = 2.0; }
	part def Engine { attribute size = 1.0; part block : Block; }
	abstract part family {
		variation part engine : Engine {
			variant part electric : Engine;
			variant part petrol : Engine;
		}
	}
	part sedan :> family { part :>> engine = engine::electric; }
}`)
	run(t, s, "%instantiate sedan")
	if got := s.Complete("%features sedan.", len("%features sedan.")).Candidates; contains(got, "sedan.engine") {
		t.Errorf("an unread variation, whose variant is not known yet, was offered: %v", got)
	}
	wants(t, run(t, s, "%features sedan"), "engine = electric (Instance ID: 2)")
	before := s.rtCtx.InstanceIDs()
	if got := s.Complete("%features sedan.", len("%features sedan.")).Candidates; !contains(got, "sedan.engine") {
		t.Errorf("the selected variation is not offered: %v", got)
	}
	if got := s.Complete("%features sedan.engine.", len("%features sedan.engine.")).Candidates; fmt.Sprint(got) != "[sedan.engine.block]" {
		t.Errorf("the variant's parts are not offered: %v", got)
	}
	if after := s.rtCtx.InstanceIDs(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("completion materialized objects: ids %v, were %v", after, before)
	}
	wants(t, run(t, s, "%features sedan.engine.block"), "mass = 2.0")
}

// A debugger performed by a nested object under quoted names keeps running over
// an unrelated declaration: the label the session holds the performer under is
// read back as the reference it spells.
func TestQuotedNestedPerformerSurvivesUnrelatedDeclaration(t *testing.T) {
	const model = `package 'Two Words' {
	part def Gauge {
		attribute count = 0;
	}
	part def Rack {
		part 'main gauge' : Gauge;
	}
	state def Check {
		entry; then checking;
		state checking;
		accept after 5 [SI::s] then checked;
		state checked;
	}
	part 'the rack' : Rack;
}`
	t.Run("state", func(t *testing.T) {
		s := submitted(t, model)
		run(t, s, "%instantiate 'the rack'")
		wants(t, run(t, s, "%state 'Two Words'::Check 'the rack'.'main gauge'"), "Started state machine executor")
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		wants(t, run(t, s, "%current"), "checking")
		wants(t, run(t, s, "%advance 5"), "Current state: checked")
	})
	t.Run("action", func(t *testing.T) {
		s := submitted(t, model)
		res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		if len(res.Diagnostics) > 0 {
			t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
		}
		run(t, s, "%instantiate 'the rack'")
		wants(t, run(t, s, "%action tally 'the rack'.'main gauge'"), "Started action executor")
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		rejects(t, run(t, s, "%step"), "no active action session")
	})
}

// Completion reads and never materializes: it follows a part not yet reached by
// a command by type, and pressing Tab leaves the runtime as it found it.
func TestCompletionMaterializesNothing(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	before := s.rtCtx.InstanceIDs()

	for _, line := range []string{"%features car.", "%features car.fl.", "%features car.wheels[2].", "%eval in car.wheels[1].", "%features #1.fl.hu"} {
		got := s.Complete(line, len(line))
		if len(got.Candidates) == 0 {
			t.Errorf("%q offered nothing", line)
		}
	}
	if after := s.rtCtx.InstanceIDs(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("completion changed the objects held: %v, was %v", after, before)
	}
	for _, name := range []string{"fl", "wheels"} {
		if car.FeatureValues[name].Materialized {
			t.Errorf("completion materialized car.%s", name)
		}
	}
	wants(t, run(t, s, "%features car.wheels[2].hub"), "bolts = 5")
	rejects(t, run(t, s, "%features car.wheels[3]"), "bolts")
}

// A materialized state machine exhibits none, so a reference to it — by id or by
// path — debugs a run of its machine, as its declared name always has.
func TestStateMachineObjectsByReference(t *testing.T) {
	s := submitted(t, `package Plant {
	state def Modes {
		entry; then off;
		state off;
		accept after 2 [SI::s] then on;
		state on;
	}
	part def Monitor {
		state modes : Modes;
	}
	part monitor : Monitor;
}`)
	id := objectIDIn(t, run(t, s, "%instantiate Plant::Modes"))
	wants(t, run(t, s, "%state Plant::Modes"), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%state #"+id), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%advance 2"), "Current state: on")
	run(t, s, "%instantiate monitor")
	wants(t, run(t, s, "%state monitor.modes"), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%advance 2"), "Current state: on")
	wants(t, run(t, s, "%state monitor.nope"), "error:", `Plant::monitor has no feature "nope"`)
}

// %state on the name of an object that does not exist yet says so with the
// resolver's guidance, whether or not the object would exhibit a machine, while
// a machine's own name still runs the machine.
func TestStateNamesAnObjectNotYetInstantiated(t *testing.T) {
	s := submitted(t, `package Fleet {
	part def Plain { attribute n = 1; }
	part def Rover {
		exhibit state modes { entry; then idle; state idle; }
	}
	part plain : Plain;
	part rover : Rover;
	part spare : Rover;
}`)

	wants(t, run(t, s, "%state rover"), "error:", `"rover" is not a state machine, and no instance of "Fleet::rover" (use %instantiate first)`)
	wants(t, run(t, s, "%state plain"), "error:", `"plain" is not a state machine, and no instance of "Fleet::plain" (use %instantiate first)`)

	id := objectIDIn(t, run(t, s, "%instantiate spare"))
	wants(t, run(t, s, "%state rover"), "error:",
		`no instance of the usage "Fleet::rover": object #`+id+` of "Fleet::spare" is of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::rover to create the usage's object, or name Fleet::spare to address it")
	wants(t, run(t, s, "%state Fleet::Rover::modes"), `Debugging state machine "modes" exhibited by object #`+id+` of "Fleet::spare"`, "Current state: idle")

	rover := objectIDIn(t, run(t, s, "%instantiate rover"))
	wants(t, run(t, s, "%state rover"), `Debugging state machine "modes" exhibited by object #`+rover+` of "Fleet::rover"`, "Current state: idle")
	run(t, s, "%instantiate plain")
	got := run(t, s, "%state plain")
	wants(t, got, "error:", `object "Fleet::plain" exhibits no state machine`)
	rejects(t, got, "%instantiate")
}

// An object displaced from its name still carries its type's conditions, so an
// unpinned check or evaluation names it as one of the objects it could be about
// instead of quietly answering about the currently named one.
func TestDisplacedObjectsCarryImplicitSubjects(t *testing.T) {
	const lab = `package Lab {
	part def Sensor {
		attribute reading : ScalarValues::Real = 1.0;
		constraint inRange { reading < 5.0 }
	}
	part def Rack { part gauge : Sensor; }
	part sensor : Sensor;
	part rack : Rack;
}`
	t.Run("displaced object", func(t *testing.T) {
		s := submitted(t, lab)
		first := objectIDIn(t, run(t, s, "%instantiate sensor"))
		wants(t, run(t, s, "%constraint Lab::Sensor::inRange"), "passed (on Lab::sensor ID: "+first+")")
		second := objectIDIn(t, run(t, s, "%instantiate sensor"))
		if first == second {
			t.Fatalf("a second %%instantiate reused id #%s", first)
		}
		both := "carried by more than one object of this session (Lab::sensor, #" + first + ")"
		wants(t, run(t, s, "%constraint Lab::Sensor::inRange"), "error:", both)
		wants(t, run(t, s, "%eval Lab::Sensor::reading"), "error:", both)
		// Naming the object settles the question.
		wants(t, run(t, s, "%eval in #"+first+" : reading"), "1.0")
	})

	t.Run("part of a displaced object", func(t *testing.T) {
		s := submitted(t, lab)
		rack := objectIDIn(t, run(t, s, "%instantiate rack"))
		run(t, s, "%instantiate rack")
		wants(t, run(t, s, "%eval Lab::Sensor::reading"),
			"carried by more than one object of this session (Lab::rack.gauge, #"+rack+".gauge)")
	})
}

// The elements of a multi-valued part carry their type's conditions like any
// nested object, each under its index, so an unpinned check names them rather
// than answering about declared defaults.
func TestCollectionElementsCarryImplicitSubjects(t *testing.T) {
	const garage = `package Garage {
	part def Wheel {
		attribute psi : ScalarValues::Real = 32.0;
		constraint inflated { psi > 20.0 }
	}
	part def Car { part wheels : Wheel[2]; }
	part def Cart { part wheel : Wheel[1..3]; }
	part car : Car;
	part cart : Cart;
}`
	t.Run("one element", func(t *testing.T) {
		s := submitted(t, garage)
		cart := objectIDIn(t, run(t, s, "%instantiate cart"))
		out := run(t, s, "%constraint Garage::Wheel::inflated")
		wants(t, out, "passed (on Garage::cart.wheel[1] ID: ")
		rejects(t, out, "ID: "+cart+")")
		wants(t, run(t, s, "%eval Garage::Wheel::psi"), "32.0")
	})

	t.Run("several elements", func(t *testing.T) {
		s := submitted(t, garage)
		run(t, s, "%instantiate car")
		both := "carried by more than one object of this session (Garage::car.wheels[1], Garage::car.wheels[2])"
		wants(t, run(t, s, "%constraint Garage::Wheel::inflated"), "error:", both)
		wants(t, run(t, s, "%eval Garage::Wheel::psi"), "error:", both)
		wants(t, run(t, s, "%eval in car.wheels[2] : psi"), "32.0")
	})

	t.Run("elements of a displaced object", func(t *testing.T) {
		s := submitted(t, garage)
		car := objectIDIn(t, run(t, s, "%instantiate car"))
		run(t, s, "%instantiate car")
		wants(t, run(t, s, "%eval Garage::Wheel::psi"),
			"carried by more than one object of this session (Garage::car.wheels[1], Garage::car.wheels[2], #"+car+".wheels[1], #"+car+".wheels[2])")
	})
}

// A declared name spelled like an id ('#3') or an indexed element ('hub[2]') is
// a name: the REPL reports it quoted, so every label it prints reads back to
// the object it printed it for, and the generated `#<id>` and `[<n>]` stay
// distinguishable from names.
func TestReservedLookingNamesStayNames(t *testing.T) {
	const odd = `package Odd {
	part def Hub {
		attribute bolts : ScalarValues::Integer = 5;
		constraint tight { bolts > 4 }
	}
	part def Wheel {
		part '#7' : Hub;
		part 'hub[2]' : Hub;
	}
	part def Cart { part 'wheel[2]' : Wheel[1..3]; }
	state def Check {
		entry; then checking;
		state checking;
		accept after 5 [SI::s] then checked;
		state checked;
	}
	part '#3' : Wheel;
	part 'car[2]' : Wheel;
	part cart : Cart;
}`
	// carriersIn reads the labels an ambiguity lists back out of its message.
	carriersIn := func(t *testing.T, out string) []string {
		t.Helper()
		_, listed, ok := strings.Cut(out, "of this session (")
		listed, _, _ = strings.Cut(listed, ")")
		if !ok || listed == "" {
			t.Fatalf("no carriers listed in:\n%s", out)
		}
		return strings.Split(listed, ", ")
	}
	// eachResolves asserts every label names an object of its own.
	eachResolves := func(t *testing.T, s *Session, labels []string) {
		t.Helper()
		seen := make(map[int64]string, len(labels))
		for _, label := range labels {
			inst, reported, err := s.resolveObject(label)
			if err != nil {
				t.Errorf("label %s does not read back: %v", label, err)
				continue
			}
			if reported != label {
				t.Errorf("label %s reads back as %s", label, reported)
			}
			if other, dup := seen[inst.ID]; dup {
				t.Errorf("labels %s and %s name one object, #%d", other, label, inst.ID)
			}
			seen[inst.ID] = label
		}
	}

	t.Run("roots and features", func(t *testing.T) {
		s := submitted(t, odd)
		wants(t, run(t, s, "%instantiate '#3'"), "Created instance of Odd::'#3'", "ID: 1")
		wants(t, run(t, s, "%features '#3'"), "Instance: Odd::'#3' (ID: 1)")
		wants(t, run(t, s, "%features #1"), "Instance: #1 (ID: 1)")
		wants(t, run(t, s, "%features #3"), "Instance: #3 (ID: 3)")
		wants(t, run(t, s, "%features '#3'.'#7'"), "Instance: Odd::'#3'.'#7' (ID: ", "bolts = 5")
		wants(t, run(t, s, "%features '#3'::'hub[2]'"), "Instance: Odd::'#3'.'hub[2]' (ID: ", "bolts = 5")
		wants(t, run(t, s, "%features '#3'.hub[2]"), "error:", `Odd::'#3' has no feature "hub"`)
		if got := s.Complete("%features '#3'.", len("%features '#3'.")).Candidates; fmt.Sprint(got) != "['#3'.'#7' '#3'.'hub[2]']" {
			t.Errorf("completion offered %v", got)
		}
		wants(t, run(t, s, "%instantiate 'car[2]'"), "Created instance of Odd::'car[2]'")
		wants(t, run(t, s, "%features 'car[2]'"), "Instance: Odd::'car[2]' (ID: ")
		wants(t, run(t, s, "%features car[2]"), "error:", "car[2] takes no index")
		wants(t, run(t, s, "%eval in 'car[2]'.'hub[2]' : bolts"), "= 5", "(on Odd::'car[2]'.'hub[2]' ID: ")

		out := run(t, s, "%eval Odd::Hub::bolts")
		wants(t, out, "error:", "carried by more than one object of this session (Odd::'#3'.'#7', Odd::'#3'.'hub[2]', Odd::'car[2]'.'#7', Odd::'car[2]'.'hub[2]')")
		eachResolves(t, s, carriersIn(t, out))
	})

	t.Run("displaced root and indexed element", func(t *testing.T) {
		s := submitted(t, odd)
		run(t, s, "%instantiate '#3'")
		run(t, s, "%instantiate '#3'")
		wants(t, run(t, s, "%features #1"), "Instance: #1 (ID: 1)")
		rejects(t, run(t, s, "%features '#3'"), "(ID: 1)")
		wants(t, run(t, s, "%instantiate cart"), "Created instance of Odd::cart")
		wants(t, run(t, s, "%features cart.'wheel[2]'[1]"), "Instance: Odd::cart.'wheel[2]'[1] (ID: ")
		wants(t, run(t, s, "%features cart.'wheel[2]'"), "error:", "'wheel[2]' of Odd::cart holds 1 object: pick one by index, 'wheel[2]'[1] to 'wheel[2]'[1]")
		wants(t, run(t, s, "%features cart.'wheel[2]'[1].'#7'"), "Instance: Odd::cart.'wheel[2]'[1].'#7' (ID: ")

		out := run(t, s, "%constraint Odd::Hub::tight")
		wants(t, out, "error:", "(Odd::'#3'.'#7', Odd::'#3'.'hub[2]', Odd::cart.'wheel[2]'[1].'#7', Odd::cart.'wheel[2]'[1].'hub[2]', #1.'#7', #1.'hub[2]')")
		eachResolves(t, s, carriersIn(t, out))
	})

	t.Run("debugger performer", func(t *testing.T) {
		s := submitted(t, odd)
		run(t, s, "%instantiate '#3'")
		wants(t, run(t, s, "%state Odd::Check '#3'.'hub[2]'"), "Started state machine executor")
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		wants(t, run(t, s, "%current"), "checking")
		wants(t, run(t, s, "%advance 5"), "Current state: checked")
	})
}

// A name holding `::` inside its quotes is one segment, and stays one in every
// label the REPL prints: a flat spelling (Demo::left::right) would read back as
// two names and reach a different declaration, or none.
func TestQuotedNamesHoldingSeparatorsStayOneSegment(t *testing.T) {
	const model = `package Demo {
	part def Hub {
		attribute bolts : ScalarValues::Integer = 5;
		constraint tight { bolts > 4 }
	}
	part def Wheel { part 'in::ner' : Hub; }
	state def Check {
		entry; then checking;
		state checking;
		accept after 5 [SI::s] then checked;
		state checked;
	}
	part 'left::right' : Wheel;
	part left : Wheel;
}`
	// readsBack asserts a printed label resolves to the object with id and is
	// reported under the same spelling.
	readsBack := func(t *testing.T, s *Session, label, id string) {
		t.Helper()
		inst, reported, err := s.resolveObject(label)
		if err != nil {
			t.Fatalf("label %s does not read back: %v", label, err)
		}
		if fmt.Sprint(inst.ID) != id || reported != label {
			t.Fatalf("label %s reads back as %s (#%d), want #%s", label, reported, inst.ID, id)
		}
	}

	t.Run("root, nested feature and carriers", func(t *testing.T) {
		s := submitted(t, model)
		out := run(t, s, "%instantiate Demo::'left::right'")
		wants(t, out, "Created instance of Demo::'left::right'")
		rejects(t, out, "Demo::left::right")
		root := objectIDIn(t, out)
		wants(t, run(t, s, "%instances"), "Demo::'left::right' (ID: "+root+")")
		readsBack(t, s, "Demo::'left::right'", root)
		wants(t, run(t, s, "%features 'left::right'"), "Instance: Demo::'left::right' (ID: "+root+")")
		wants(t, run(t, s, "%eval in 'left::right' : 'in::ner'.bolts"), "= 5", "(on Demo::'left::right' ID: "+root+")")

		nested, label, err := s.resolveObject("'left::right'.'in::ner'")
		if err != nil {
			t.Fatalf("'left::right'.'in::ner': %v", err)
		}
		if label != "Demo::'left::right'.'in::ner'" {
			t.Fatalf("nested label = %s", label)
		}
		nestedID := fmt.Sprint(nested.ID)
		readsBack(t, s, label, nestedID)
		wants(t, run(t, s, "%features Demo::'left::right'::'in::ner'"), "Instance: "+label+" (ID: "+nestedID+")")
		wants(t, run(t, s, "%constraint Demo::Hub::tight"), "passed (on "+label+" ID: "+nestedID+")")

		// The declaration the flat spelling's first segment names is untouched.
		wants(t, run(t, s, "%features Demo::left"), `error: no instance of the usage "Demo::left"`, "name Demo::'left::right' to address it")

		for _, c := range []struct{ line, want string }{
			{"%features Demo::'le", "Demo::'left::right'"},
			{"%features 'left::right'.", "'left::right'.'in::ner'"},
			{"%features Demo::'left::right'::", "Demo::'left::right'::'in::ner'"},
		} {
			got := s.Complete(c.line, len(c.line)).Candidates
			if !slices.Contains(got, c.want) {
				t.Errorf("completing %q offered %v, want %s", c.line, got, c.want)
			}
		}
	})

	t.Run("debugger across carry-over and displacement", func(t *testing.T) {
		s := submitted(t, model)
		first := objectIDIn(t, run(t, s, "%instantiate Demo::'left::right'"))
		wants(t, run(t, s, "%state Demo::Check 'left::right'.'in::ner'"), "Started state machine executor", `of "Demo::'left::right'.'in::ner'"`)
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		if got, want := s.stateExec.selfFQN, "Demo::'left::right'.'in::ner'"; got != want {
			t.Fatalf("debugger label after carry-over = %q, want %s", got, want)
		}
		wants(t, run(t, s, "%current"), "checking")

		again := run(t, s, "%instantiate Demo::'left::right'")
		wants(t, again, "note: Demo::'left::right' now denotes this object", "stays reachable as #"+first,
			"keeps running over the object Demo::'left::right'.'in::ner' named, now #"+first+".'in::ner'")
		if got, want := s.stateExec.selfFQN, "#"+first+".'in::ner'"; got != want {
			t.Fatalf("debugger label after displacement = %q, want %s", got, want)
		}
		wants(t, run(t, s, "%instances"), "#"+first+" (ID: "+first+", displaced from Demo::'left::right')")
		wants(t, run(t, s, "%advance 5"), "Current state: checked")
		inner, _ := s.unnamed[0].obj.FeatureValues["in::ner"].Value.Object()
		readsBack(t, s, "#"+first+".'in::ner'", fmt.Sprint(inner))
	})

	// The index registers declarations by their flat name, so two spelled the
	// same flat are told apart only in the report of their clash.
	t.Run("clashing flat spellings", func(t *testing.T) {
		s := submitted(t, "package Demo { part def Wheel; part 'a::b' : Wheel; part a : Wheel { part b : Wheel; } }")
		for _, ref := range []string{"Demo::'a::b'", "Demo::a::b"} {
			wants(t, run(t, s, "%instantiate "+ref), "error:", "is ambiguous: Demo::'a::b', Demo::a::b")
		}
	})
}

// A verdict about a nested object spells each walked feature as one segment,
// so a name spelled with `::` inside its quotes stays one feature — and the
// label a constraint, a requirement and a satisfaction print reads back to the
// object they were about.
func TestVerdictLabelsKeepQuotedFeatureNamesWhole(t *testing.T) {
	s := submitted(t, `package Lab {
	part def Sensor {
		attribute reading : ScalarValues::Real = 1.0;
		constraint inRange { reading < 5.0 }
		requirement lim {
			require constraint { reading < 5.0 }
		}
		assert satisfy lim;
	}
	part def Rack { part 'in::out' : Sensor; }
	part rack : Rack;
}`)
	rack := objectIDIn(t, run(t, s, "%instantiate rack"))
	nested, _, err := s.resolveObject("rack.'in::out'")
	if err != nil {
		t.Fatalf("rack.'in::out': %v", err)
	}
	if fmt.Sprint(nested.ID) == rack {
		t.Fatalf("rack.'in::out' resolved to the rack itself, #%s", rack)
	}
	on := fmt.Sprintf("(on Lab::rack.'in::out' ID: %d)", nested.ID)
	for _, check := range []struct{ line, verdict string }{
		{"%constraint Lab::Sensor::inRange", "✓ Constraint Lab::Sensor::inRange passed "},
		{"%requirement Lab::Sensor::lim", "✓ Requirement Lab::Sensor::lim satisfied "},
		{"%satisfy Lab::Sensor", "✓ satisfy lim holds "},
	} {
		out := run(t, s, check.line)
		wants(t, out, check.verdict+on)
		rejects(t, out, "rack.in.out", "rack.'in'.'out'")
		_, label, ok := strings.Cut(out, "(on ")
		label, _, _ = strings.Cut(label, " ID:")
		if !ok {
			t.Fatalf("%s reported no object:\n%s", check.line, out)
		}
		inst, reported, err := s.resolveObject(label)
		if err != nil {
			t.Errorf("%s label %s does not read back: %v", check.line, label, err)
			continue
		}
		if inst.ID != nested.ID || reported != label {
			t.Errorf("%s label %s reads back as %s (#%d), want #%d", check.line, label, reported, inst.ID, nested.ID)
		}
	}
}
