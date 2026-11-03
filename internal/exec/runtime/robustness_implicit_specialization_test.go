package runtime

import "testing"

// Every performance the run creates — a loop node, each iteration body — carries the
// implicit specializations of its kind (KerML §7.3.4), so the Kernel collections the
// library derives for it read from what it holds and create nothing.
const implicitSpecializationModel = `package test {
	private import ScalarValues::Integer;
	part def K {
		attribute n : Integer = 0;
		perform action loopy {
			first start;
			then loop action body {
				first start;
				then action inc assign n := n + 1;
				then done;
			} until n >= 3;
			then done;
		}
		state sm {
			entry action en;
			do action d;
		}
	}
	part k : K;
}`

func TestRuntimeRobustnessImplicitSpecialization(t *testing.T) {
	t.Run("a_loop_node_derives_its_enclosed_performances_without_new_objects", testLoopNodeDerivedCollectionsCreateNothing)
	t.Run("a_state_entry_action_and_its_entryAction_are_one_performance", testStateEntryActionIsEntryAction)
}

func testLoopNodeDerivedCollectionsCreateNothing(t *testing.T) {
	ctx, idx := libraryModelContext(t, implicitSpecializationModel)
	k, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if err != nil {
		t.Fatalf("Instantiate(k): %v", err)
	}
	loop := heldInstance(t, ctx, heldInstance(t, ctx, k, "loopy"), "loops")
	before := ctx.InstanceCount()
	ctx.SetMaxInstances(before)
	enclosed := readElements(t, ctx, loop, "enclosedPerformances")
	if len(enclosed) != 3 {
		t.Fatalf("loop.enclosedPerformances holds %d objects, want the three iteration bodies", len(enclosed))
	}
	bodies := readElements(t, ctx, loop, "body")
	for i, body := range bodies {
		if mustObject(body) != mustObject(enclosed[i]) {
			t.Errorf("enclosedPerformances#(%d) = #%d, want body#(%d) = #%d", i+1, mustObject(enclosed[i]), i+1, mustObject(body))
		}
	}
	for _, name := range []string{"subperformances", "suboccurrences"} {
		if got := readElements(t, ctx, loop, name); len(got) != 0 {
			t.Errorf("loop.%s holds %d objects, want none: LoopAction::body is not composite", name, len(got))
		}
	}
	if got := ctx.InstanceCount(); got != before {
		t.Errorf("reading the derived collections made %d objects, want none", got-before)
	}
}

func testStateEntryActionIsEntryAction(t *testing.T) {
	ctx, idx := libraryModelContext(t, implicitSpecializationModel)
	k, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if err != nil {
		t.Fatalf("Instantiate(k): %v", err)
	}
	sm := heldInstance(t, ctx, k, "sm")
	for _, pair := range [][2]string{{"en", "entryAction"}, {"d", "doAction"}} {
		declared, library := heldInstance(t, ctx, sm, pair[0]), heldInstance(t, ctx, sm, pair[1])
		if declared.ID != library.ID {
			t.Errorf("sm.%s = #%d, sm.%s = #%d: want one performance, the state action redefines the StateAction one", pair[0], declared.ID, pair[1], library.ID)
		}
	}
}

func heldInstance(t *testing.T, ctx *Context, owner *Instance, name string) *Instance {
	t.Helper()
	held := readElements(t, ctx, owner, name)
	if len(held) == 0 {
		t.Fatalf("%s holds nothing", name)
	}
	inst, ok := ctx.Instance(mustObject(held[0]))
	if !ok {
		t.Fatalf("%s holds #%d, which is no object of the run", name, mustObject(held[0]))
	}
	return inst
}

func readElements(t *testing.T, ctx *Context, owner *Instance, name string) []Value {
	t.Helper()
	fv, err := owner.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	val, err := fv.ReadValue(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return elementsOf(val)
}
