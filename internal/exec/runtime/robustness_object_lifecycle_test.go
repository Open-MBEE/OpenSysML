package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessObjectLifecycle exercises objects created and destroyed while a
// behavior runs: constructor expressions yield distinct live occurrences, in a loop too,
// held and classified by the feature written; destruction ends the object wherever it is
// referred to, releases it from the extent, ends the machine it exhibits, refuses a second
// destruction and later reads with typed errors, and stays deterministic under exploration.
func TestRuntimeRobustnessObjectLifecycle(t *testing.T) {
	t.Run("two_objects_of_one_usage_are_distinct_and_held", testObjectLifecycleTwoOfOneUsage)
	t.Run("creation_in_a_loop", testObjectLifecycleCreationInLoop)
	t.Run("destroy_through_a_reference_ends_the_held_object", testObjectLifecycleDestroyThroughReference)
	t.Run("destroy_ends_the_created_objects_machine", testObjectLifecycleDestroyEndsMachine)
	t.Run("destroy_twice_is_refused", testObjectLifecycleDestroyTwice)
	t.Run("destroy_of_the_whole_ends_the_created_parts", testObjectLifecycleDestroyWholeEndsCreatedParts)
	t.Run("a_part_moved_between_wholes_ends_with_the_new_whole", testObjectLifecycleMovedPart)
	t.Run("send_new_starts_the_message_objects_behaviors", testObjectLifecycleSendNew)
	t.Run("a_failed_constructor_leaves_no_argument_object", testObjectLifecycleFailedConstructor)
	t.Run("destroying_a_holder_leaves_what_it_referred_to_in_the_extent", testObjectLifecycleDestroyedHolderExtent)
	t.Run("a_destroyed_object_is_no_subject_of_a_check", testObjectLifecycleDestroyedNotSubject)
	t.Run("a_destroyed_part_a_live_whole_retains_is_no_subject_of_a_check", testObjectLifecycleDestroyedNestedNotSubject)
	t.Run("a_failed_constructor_rolls_back_what_its_behaviors_wrote", testObjectLifecycleFailedConstructorWrites)
	t.Run("a_refused_write_leaves_no_adoption_for_an_outer_rollback", testObjectLifecycleRefusedWriteJournal)
	t.Run("a_rolled_back_store_or_constructor_leaves_no_trace_of_what_it_undid", testObjectLifecycleRolledBackTrace)
	t.Run("a_failed_constructor_revives_what_its_behaviors_destroyed_whole", testObjectLifecycleFailedConstructorDestroy)
	t.Run("a_part_declared_or_bound_to_a_new_object_ends_with_the_whole", testObjectLifecycleDeclaredPart)
	t.Run("an_object_two_wholes_hold_composite_ends_with_either", testObjectLifecycleSharedPortion)
	t.Run("an_object_its_home_drops_is_rehomed_to_the_whole_still_holding_it", testObjectLifecycleSharedPortionRehomed)
	t.Run("a_composite_write_making_the_holder_its_own_portion_is_refused", testObjectLifecycleCompositeCycle)
	t.Run("a_composite_write_giving_an_ended_whole_a_live_portion_is_refused", testObjectLifecycleEndedWholeAdoptsNothing)
	t.Run("destroying_an_object_forgets_the_messages_addressed_to_it", testObjectLifecycleForgetsMessages)
	t.Run("derivations_that_read_lifetimes_derive_again_when_they_change", testObjectLifecycleDerivationsFollowLives)
	t.Run("an_imaged_derivation_that_read_lifetimes_follows_the_lives_where_materialized", testObjectLifecycleImagedDerivationsFollowLives)
	t.Run("behaviors_a_write_starts_run_once_the_feature_holds_the_object", testObjectLifecycleStartsAfterStore)
	t.Run("behaviors_a_constructor_argument_starts_run_once_every_argument_is_stored", testObjectLifecycleConstructorStartsAfterStores)
	t.Run("behaviors_a_bound_default_starts_run_once_the_feature_holds_the_object", testObjectLifecycleBoundDefaultStartsAfterStore)
	t.Run("explore_creating_objects_is_deterministic", testObjectLifecycleExploreCreation)
	t.Run("explore_destroy_race_reaches_both_outcomes", testObjectLifecycleExploreDestroyRace)
}

const objectLifecycleModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		private import SequenceFunctions::*;
		part def Car {
			attribute n : Integer;
			exhibit state running { entry; then idle; state idle; }
		}
		part def Fleet {
			part cars : Car[0..*];
			ref part spare : Car[0..1];
			attribute made : Integer;
			perform action build {
				first start;
				then action make {
					assign cars := addNew(cars, new Car(1));
					assign cars := addNew(cars, new Car(2));
					assign spare := cars#(2);
				}
				then done;
			}
		}
		part def Lot {
			part cars : Car[0..*];
			attribute made : Integer;
			perform action fill {
				attribute i : Integer = 0;
				first start;
				then action loop {
					while i < 5 {
						assign i := i + 1;
						assign cars := addNew(cars, new Car(i));
					}
				}
				then action count { assign made := size(all Car); }
				then done;
			}
		}
		calc def Destroy { in c : Car[0..1]; return : Car[0..1] = destroy(c); }
		calc def Extent { in f : Fleet; return : Integer = size(all Car); }
		calc def SpareN { in f : Fleet; return : Integer = f.spare.n; }
	}`

// heldCars reads the cars a fleet or lot holds, failing the test when they cannot be read.
func heldCars(t *testing.T, ctx *Context, inst *Instance) []*Instance {
	t.Helper()
	return heldNamed(t, ctx, inst, "cars")
}

// heldNamed reads the objects inst's feature holds, failing the test when they cannot be read.
func heldNamed(t *testing.T, ctx *Context, inst *Instance, name string) []*Instance {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []*Instance
	for _, el := range elementsOf(fv.HeldValue()) {
		id, ok := el.Object()
		if !ok {
			t.Fatalf("%s holds %v, want objects", name, fv.HeldValue())
		}
		out = append(out, ctx.instances[id])
	}
	return out
}

// testObjectLifecycleTwoOfOneUsage: the two constructed cars are distinct live objects,
// each performing its machine, classified by and owned through the feature written.
func testObjectLifecycleTwoOfOneUsage(t *testing.T) {
	ctx, fleet, err := instantiateWithLibraries(t, objectLifecycleModel, "test::Fleet")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	cars := heldCars(t, ctx, fleet)
	if len(cars) != 2 || cars[0].ID == cars[1].ID {
		t.Fatalf("cars = %v, want two distinct objects", cars)
	}
	carsFeature := fleet.FeatureValues["cars"].Feature.heldBy()
	for _, car := range cars {
		if l, ok := ctx.OccurrenceLife(car.ID); !ok || !l.Alive() {
			t.Errorf("OccurrenceLife(#%d) = %v, %v; want alive", car.ID, l, ok)
		}
		if b, ok := car.Behavior("running"); !ok || b.State == nil || b.State.State().Ended() {
			t.Errorf("#%d running = %v, %v; want the machine under way", car.ID, b, ok)
		}
		if car.owner != fleet || car.ownerFeature != "cars" {
			t.Errorf("#%d owned by %v.%s; want the fleet's cars", car.ID, car.owner, car.ownerFeature)
		}
		if !ctx.isDirectTypeOf(car, carsFeature) {
			t.Errorf("#%d classifiers = %v; want the cars feature among them", car.ID, car.classifiers)
		}
	}
	held, err := ctx.HeldObjects(fleet)
	if err != nil {
		t.Fatalf("HeldObjects: %v", err)
	}
	var segments []string
	for _, h := range held {
		if h.Feature == "cars" || h.Feature == "spare" {
			segments = append(segments, h.Segment)
		}
	}
	if strings.Join(segments, " ") != "cars[1] spare" {
		t.Errorf("held segments = %v; want the two cars, the second once under the scalar spare", segments)
	}
}

// testObjectLifecycleCreationInLoop: a `new` evaluated once per iteration yields one object
// per iteration, each in the extent of its definition.
func testObjectLifecycleCreationInLoop(t *testing.T) {
	ctx, lot, err := instantiateWithLibraries(t, objectLifecycleModel, "test::Lot")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	cars := heldCars(t, ctx, lot)
	seen := make(map[int64]bool)
	for i, car := range cars {
		seen[car.ID] = true
		n, err := car.GetFeatureValue(ctx, "n")
		if err != nil || FormatValue(n.Value) != []string{"1", "2", "3", "4", "5"}[i] {
			t.Errorf("cars[%d].n = %v, %v", i+1, n, err)
		}
	}
	if len(cars) != 5 || len(seen) != 5 {
		t.Fatalf("cars = %v, want five distinct objects", cars)
	}
	made, err := lot.GetFeatureValue(ctx, "made")
	if err != nil || FormatValue(made.Value) != "5" {
		t.Errorf("made = %v, %v; want all Car to count the five", made, err)
	}
}

// testObjectLifecycleDestroyThroughReference: destroying the car through `spare` ends the
// object `cars` still holds, drops it from `all Car`, keeps the stale references in place,
// and refuses a later feature read through either as a read of a destroyed occurrence.
func testObjectLifecycleDestroyThroughReference(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel)
	fleet := instantiate("Fleet")
	cars := heldCars(t, ctx, fleet)
	spare, err := fleet.GetFeatureValue(ctx, "spare")
	if err != nil {
		t.Fatalf("spare: %v", err)
	}
	if id, _ := spare.Value.Object(); id != cars[1].ID {
		t.Fatalf("spare = %v, want the second car #%d", spare.Value, cars[1].ID)
	}
	if got, err := invoke("Extent", objectValue(fleet)); err != nil || FormatValue(got) != "2" {
		t.Fatalf("all Car = %v, %v; want the two cars", got, err)
	}
	if _, err := invoke("Destroy", spare.Value); err != nil {
		t.Fatalf("destroy(spare) = %v", err)
	}
	if l, _ := ctx.OccurrenceLife(cars[1].ID); !l.Destroyed || l.Alive() {
		t.Errorf("OccurrenceLife(spare) = %v; want destroyed", l)
	}
	if l, _ := ctx.OccurrenceLife(cars[0].ID); !l.Alive() {
		t.Errorf("OccurrenceLife(first) = %v; want alive", l)
	}
	if got, err := invoke("Extent", objectValue(fleet)); err != nil || FormatValue(got) != "1" {
		t.Errorf("all Car = %v, %v after destroy; want the first car alone", got, err)
	}
	if after := heldCars(t, ctx, fleet); len(after) != 2 || after[1] != cars[1] {
		t.Errorf("cars = %v after destroy; want the stale reference kept", after)
	}
	_, err = invoke("SpareN", objectValue(fleet))
	if !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("spare.n = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := cars[1].GetFeatureValue(ctx, "n"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("cars[2].n = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := ctx.HeldObjects(cars[1]); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("HeldObjects(destroyed) = %v; want %v", err, ErrOccurrenceDestroyed)
	}
}

// testObjectLifecycleDestroyEndsMachine: the machine a constructed car exhibits ends,
// terminated, with the car.
func testObjectLifecycleDestroyEndsMachine(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel)
	fleet := instantiate("Fleet")
	car := heldCars(t, ctx, fleet)[1]
	running, ok := car.Behavior("running")
	if !ok || running.State == nil || running.State.State().Ended() {
		t.Fatalf("running = %v, %v; want the machine under way", running, ok)
	}
	if _, err := invoke("Destroy", objectValue(car)); err != nil {
		t.Fatalf("destroy(car) = %v", err)
	}
	if running.State.State() != StateTerminated {
		t.Errorf("running = %v after destroy; want terminated", running.State.State())
	}
	if l, ok := ctx.OccurrenceLife(running.State.occurrence.ID); !ok || l.Alive() {
		t.Errorf("OccurrenceLife(running) = %v, %v; want ended", l, ok)
	}
	if err := ctx.drainObjectBehaviors(); err != nil {
		t.Errorf("drain after destroy = %v; want the ended machine left alone", err)
	}
}

// testObjectLifecycleDestroyTwice: a second destroy is refused, naming when the first ended it.
func testObjectLifecycleDestroyTwice(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel)
	fleet := instantiate("Fleet")
	car := heldCars(t, ctx, fleet)[0]
	if _, err := invoke("Destroy", objectValue(car)); err != nil {
		t.Fatalf("destroy(car) = %v", err)
	}
	_, err := invoke("Destroy", objectValue(car))
	if !errors.Is(err, ErrOccurrenceDestroyed) || !strings.Contains(err.Error(), "was destroyed at") {
		t.Errorf("second destroy = %v; want %v naming the first", err, ErrOccurrenceDestroyed)
	}
}

// testObjectLifecycleDestroyWholeEndsCreatedParts: the cars a fleet adopted as its parts
// end with the fleet, each machine terminated, and none may end a second time.
func testObjectLifecycleDestroyWholeEndsCreatedParts(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel+`
		package more { private import OccurrenceFunctions::*; private import test::*;
			calc def DestroyFleet { in f : Fleet; return : Fleet = destroy(f); }
		}`)
	fleet := instantiate("Fleet")
	cars := heldCars(t, ctx, fleet)
	sym, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "more", "DestroyFleet")
	if _, err := ctx.InvokeCalc(sym, []Value{objectValue(fleet)}, scope); err != nil {
		t.Fatalf("destroy(fleet) = %v", err)
	}
	for _, car := range cars {
		if l, _ := ctx.OccurrenceLife(car.ID); !l.Destroyed {
			t.Errorf("OccurrenceLife(#%d) = %v; want destroyed with the fleet", car.ID, l)
		}
		if b, ok := car.Behavior("running"); !ok || b.State.State() != StateTerminated {
			t.Errorf("#%d running = %v, %v; want terminated with the fleet", car.ID, b, ok)
		}
		if _, err := invoke("Destroy", objectValue(car)); !errors.Is(err, ErrOccurrenceDestroyed) {
			t.Errorf("destroy(#%d) after the fleet = %v; want %v", car.ID, err, ErrOccurrenceDestroyed)
		}
	}
}

// testObjectLifecycleMovedPart: a car dropped from one composite feature and written into
// another is owned by the second holder, so destroying the first spares it and the second ends it.
func testObjectLifecycleMovedPart(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, objectLifecycleModel+`
		package more { private import OccurrenceFunctions::*; private import SequenceFunctions::*; private import test::*;
			part def Garage { part cars : Car[0..*]; }
			calc def DestroyFleet { in f : Fleet; return : Fleet = destroy(f); }
			calc def DestroyGarage { in g : Garage; return : Garage = destroy(g); }
		}`)
	fleet := instantiate("Fleet")
	idx := ctx.model.resolver.Index()
	garageSyms := idx.LookupQualified("more::Garage")
	if len(garageSyms) != 1 {
		t.Fatalf("more::Garage: %d matching symbols, want 1", len(garageSyms))
	}
	garage, err := ctx.Instantiate(garageSyms[0])
	if err != nil {
		t.Fatalf("Instantiate(Garage): %v", err)
	}
	root := idx.DocumentRoot("<test>")
	destroyWith := func(name string, inst *Instance) {
		sym, scope := calcByName(t, root, "more", name)
		if _, err := ctx.InvokeCalc(sym, []Value{objectValue(inst)}, scope); err != nil {
			t.Fatalf("%s = %v", name, err)
		}
	}
	cars := heldCars(t, ctx, fleet)
	moved := cars[0]
	if err := fleet.SetFeatureValue(ctx, "cars", sequenceOf([]Value{objectValue(cars[1])})); err != nil {
		t.Fatalf("fleet.cars := (second) = %v", err)
	}
	if moved.owner != nil {
		t.Fatalf("#%d owned by %v.%s after being dropped; want no owner", moved.ID, moved.owner, moved.ownerFeature)
	}
	if err := garage.SetFeatureValue(ctx, "cars", sequenceOf([]Value{objectValue(moved)})); err != nil {
		t.Fatalf("garage.cars := (moved) = %v", err)
	}
	if moved.owner != garage || moved.ownerFeature != "cars" {
		t.Fatalf("#%d owned by %v.%s; want the garage's cars", moved.ID, moved.owner, moved.ownerFeature)
	}
	destroyWith("DestroyFleet", fleet)
	if l, _ := ctx.OccurrenceLife(moved.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(moved) = %v after destroying the fleet; want alive", l)
	}
	if l, _ := ctx.OccurrenceLife(cars[1].ID); l.Alive() {
		t.Errorf("OccurrenceLife(kept) = %v after destroying the fleet; want ended with it", l)
	}
	destroyWith("DestroyGarage", garage)
	if l, _ := ctx.OccurrenceLife(moved.ID); l.Alive() {
		t.Errorf("OccurrenceLife(moved) = %v after destroying the garage; want ended with it", l)
	}
}

// testObjectLifecycleSendNew: the object `send new Car(9)` constructs performs Car's machine
// like one any other expression constructs.
func testObjectLifecycleSendNew(t *testing.T) {
	ctx, _, err := instantiateWithLibraries(t, objectLifecycleModel+`
		package more { private import ScalarValues::*; private import test::*;
			part def Sender {
				perform action ship { first start; then send new Car(9); then done; }
			}
		}`, "more::Sender")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	var sent *Instance
	for _, inst := range ctx.instances {
		if inst.Type != nil && inst.Type.Name == "Car" {
			sent = inst
		}
	}
	if sent == nil {
		t.Fatal("no Car constructed by the send")
	}
	if l, ok := ctx.OccurrenceLife(sent.ID); !ok || !l.Alive() {
		t.Errorf("OccurrenceLife(sent) = %v, %v; want alive", l, ok)
	}
	if b, ok := sent.Behavior("running"); !ok || b.State == nil || b.State.State().Ended() {
		t.Errorf("sent running = %v, %v; want the machine under way", b, ok)
	}
}

// testObjectLifecycleFailedConstructor: a `new Pair(new Car(1), 1/0)` whose later argument
// fails constructs nothing, the car its earlier argument made included.
func testObjectLifecycleFailedConstructor(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel+`
		package more { private import ScalarValues::*; private import test::*;
			part def Pair { part c : Car[0..1]; attribute k : Integer; }
		}`)
	fleet := instantiate("Fleet")
	before := len(ctx.created)
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Extent")
	if _, err := evalIn(t, ctx, scope, "new more::Pair(new test::Car(1), 1/0)"); err == nil {
		t.Fatal("new Pair(new Car(1), 1/0) succeeded; want the failing argument reported")
	}
	after := 0
	for _, id := range ctx.created {
		if inst := ctx.instances[id]; inst != nil && inst.Type.Name == "Car" {
			after++
		}
	}
	if after != 2 || len(ctx.created) != before {
		t.Errorf("%d cars, %d objects after the failed constructor; want the fleet's two cars and %d objects", after, len(ctx.created), before)
	}
	if got, err := invoke("Extent", objectValue(fleet)); err != nil || FormatValue(got) != "2" {
		t.Errorf("all Car = %v, %v after the failed constructor; want the fleet's two", got, err)
	}
}

// testObjectLifecycleDestroyedHolderExtent: destroying a fleet ends the cars it owns but not
// the car its `ref part spare` refers to, which `all Car` still reaches.
func testObjectLifecycleDestroyedHolderExtent(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, objectLifecycleModel+`
		package more { private import OccurrenceFunctions::*; private import test::*;
			calc def DestroyFleet { in f : Fleet; return : Fleet = destroy(f); }
		}`)
	fleet := instantiate("Fleet")
	cars := heldCars(t, ctx, fleet)
	loose := cars[1]
	if err := fleet.SetFeatureValue(ctx, "cars", sequenceOf([]Value{objectValue(cars[0])})); err != nil {
		t.Fatalf("fleet.cars := (first) = %v", err)
	}
	if loose.owner != nil {
		t.Fatalf("#%d owned by %v; want released", loose.ID, loose.owner)
	}
	sym, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "more", "DestroyFleet")
	if _, err := ctx.InvokeCalc(sym, []Value{objectValue(fleet)}, scope); err != nil {
		t.Fatalf("destroy(fleet) = %v", err)
	}
	if l, _ := ctx.OccurrenceLife(loose.ID); !l.Alive() {
		t.Fatalf("OccurrenceLife(spare) = %v after destroying the fleet; want alive", l)
	}
	if got, err := invoke("Extent", objectValue(fleet)); err != nil || FormatValue(got) != "1" {
		t.Errorf("all Car = %v, %v after destroying the fleet; want the spare alone", got, err)
	}
}

// testObjectLifecycleDestroyedNotSubject: of two standalone cars, the latest destroyed, the
// destroyed one is no carrier of Car's constraint, so the check is about the live one.
func testObjectLifecycleDestroyedNotSubject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Car { attribute n : Integer = 1; constraint small { n < 10 } }
			calc def Destroy { in c : Car; return : Car[0..1] = destroy(c); }
		}`)
	live, doomed := instantiate("Car"), instantiate("Car")
	small := memberPath(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Car", "small")
	if _, err := invoke("Destroy", objectValue(doomed)); err != nil {
		t.Fatalf("destroy = %v", err)
	}
	if l, _ := ctx.OccurrenceLife(live.ID); !l.Alive() {
		t.Fatalf("the other car ended too")
	}
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if err != nil || !satisfied {
		t.Errorf("EvaluateConstraint after destroying one car = %t, %v; want the live car's verdict", satisfied, err)
	}
}

// testObjectLifecycleDestroyedNestedNotSubject: a live whole retains the destroyed part it
// held, but the destroyed part is no carrier of Part's constraint, so the check is about the live one.
func testObjectLifecycleDestroyedNestedNotSubject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Wheel { attribute n : Integer = 1; constraint small { n < 10 } }
			part def Car { part front : Wheel; part rear : Wheel; }
			part car : Car;
			calc def Destroy { in w : Wheel; return : Wheel[0..1] = destroy(w); }
		}`)
	car := instantiate("car")
	fv, err := car.GetFeatureValue(ctx, "rear")
	if err != nil {
		t.Fatalf("car.rear: %v", err)
	}
	rear := fv.HeldValue()
	small := memberPath(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Wheel", "small")
	if _, err := invoke("Destroy", rear); err != nil {
		t.Fatalf("destroy = %v", err)
	}
	if _, ok := car.FeatureValues["rear"].HeldValue().Object(); !ok {
		t.Fatalf("car.rear after destroy = %s; want the stale object kept", FormatValue(car.FeatureValues["rear"].HeldValue()))
	}
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if err != nil || !satisfied {
		t.Errorf("EvaluateConstraint after destroying the rear wheel = %t, %v; want the front wheel's verdict", satisfied, err)
	}
}

// testObjectLifecycleFailedConstructorWrites: the action a `new Worker(plant)` performs writes
// the plant's count before failing, and the failed construction leaves the count as it was.
func testObjectLifecycleFailedConstructorWrites(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			part def Plant { attribute count : Integer = 0; }
			part def Worker {
				ref part p : Plant;
				perform action go {
					first start;
					then action write { assign p.count := 1; }
					then action fail { assign p.count := 1/0; }
					then done;
				}
			}
			part plant : Plant;
		}`)
	plant := instantiate("plant")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Plant")
	before := len(ctx.created)
	if _, err := evalIn(t, ctx, scope, "new Worker(plant)"); err == nil {
		t.Fatal("new Worker(plant) succeeded; want its failing action reported")
	}
	fv, err := plant.GetFeatureValue(ctx, "count")
	if err != nil || FormatValue(fv.HeldValue()) != "0" {
		t.Errorf("plant.count = %v, %v after the failed constructor; want 0, the write rolled back", fv, err)
	}
	if len(ctx.created) != before {
		t.Errorf("%d objects after the failed constructor; want %d", len(ctx.created), before)
	}
}

// testObjectLifecycleFailedConstructorDestroy: a constructor whose behavior destroys a live object and
// then fails is rolled back whole: the object lives on and its machine stands where it stood.
func testObjectLifecycleFailedConstructorDestroy(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Plant {
				attribute count : Integer = 0;
				exhibit state running { entry; then idle; state idle; }
			}
			part def Worker {
				ref part p : Plant;
				perform action go {
					first start;
					then action kill { assign p := destroy(p); }
					then action fail { assign p.count := 1/0; }
					then done;
				}
			}
			part plant : Plant;
		}`)
	plant := instantiate("plant")
	b, ok := plant.Behavior("running")
	if !ok || b.State == nil || b.State.State().Ended() {
		t.Fatalf("running = %v, %v; want the machine under way", b, ok)
	}
	was := b.State.State()
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Plant")
	if _, err := evalIn(t, ctx, scope, "new Worker(plant)"); err == nil {
		t.Fatal("new Worker(plant) succeeded; want its failing action reported")
	}
	if l, _ := ctx.OccurrenceLife(plant.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(plant) = %v after the failed constructor; want alive again", l)
	}
	if got := b.State.State(); got != was {
		t.Errorf("running = %v after the failed constructor; want %v, as before", got, was)
	}
}

// testObjectLifecycleSharedPortion: an object two wholes hold in composite features (as a binding
// makes them) is a portion of both, ended with either, whichever is its home.
func testObjectLifecycleSharedPortion(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			part def Car;
			part def Garage { part slot : Car[0..1]; }
			part a : Garage;
			part b : Garage;
			calc def Scrap { in g : Garage; return : Garage = destroy(g); }
		}`)
	a, b := instantiate("a"), instantiate("b")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Garage")
	car, err := evalIn(t, ctx, scope, "new Car()")
	if err != nil {
		t.Fatalf("new Car(): %v", err)
	}
	for _, g := range []*Instance{a, b} {
		if err := g.SetFeatureValue(ctx, "slot", car); err != nil {
			t.Fatalf("#%d.slot := car: %v", g.ID, err)
		}
	}
	id, _ := car.Object()
	if owner := ctx.instances[id].owner; owner != a {
		t.Fatalf("car's home is %v; want a, the first to hold it", owner)
	}
	if _, err := evalIn(t, ctx, scope, "Scrap(b)"); err != nil {
		t.Fatalf("Scrap(b): %v", err)
	}
	if l, _ := ctx.OccurrenceLife(id); !l.Destroyed {
		t.Errorf("OccurrenceLife(car) = %v after destroying b; want destroyed with the whole holding it", l)
	}
	if l, _ := ctx.OccurrenceLife(a.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(a) = %v; want alive, it was not destroyed", l)
	}
}

// testObjectLifecycleSharedPortionRehomed: when the home of an object two wholes hold drops it,
// the other whole still holding it becomes its home, so the object keeps its way to its whole.
func testObjectLifecycleSharedPortionRehomed(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			part def Car;
			part def Garage { part slot : Car[0..1]; }
			part a : Garage;
			part b : Garage;
		}`)
	a, b := instantiate("a"), instantiate("b")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Garage")
	car, err := evalIn(t, ctx, scope, "new Car()")
	if err != nil {
		t.Fatalf("new Car(): %v", err)
	}
	for _, g := range []*Instance{a, b} {
		if err := g.SetFeatureValue(ctx, "slot", car); err != nil {
			t.Fatalf("#%d.slot := car: %v", g.ID, err)
		}
	}
	id, _ := car.Object()
	inst := ctx.instances[id]
	_, rollback := ctx.beginJournal()
	if err := a.SetFeatureValue(ctx, "slot", nullValue()); err != nil {
		t.Fatalf("a.slot := null: %v", err)
	}
	if inst.owner != b || inst.ownerFeature != "slot" {
		t.Errorf("car's home after a drops it = %v.%s; want b.slot, still holding it", inst.owner, inst.ownerFeature)
	}
	rollback()
	if inst.owner != a || inst.ownerFeature != "slot" {
		t.Errorf("car's home after rolling the drop back = %v.%s; want a.slot", inst.owner, inst.ownerFeature)
	}
	if err := a.SetFeatureValue(ctx, "slot", nullValue()); err != nil {
		t.Fatalf("a.slot := null: %v", err)
	}
	if err := b.SetFeatureValue(ctx, "slot", nullValue()); err != nil {
		t.Fatalf("b.slot := null: %v", err)
	}
	if inst.owner != nil {
		t.Errorf("car's home after both drop it = %v; want none", inst.owner)
	}
}

// testObjectLifecycleEndedWholeAdoptsNothing: a composite write giving an ended whole a portion live or
// ended after it is refused with ErrOccurrenceLifetime and holds nothing; a reference to the live
// object, and a portion ended no later than the whole, are held.
func testObjectLifecycleEndedWholeAdoptsNothing(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			part def Car;
			part def Garage { part slot : Car[0..1]; ref part seen : Car[0..1]; }
			part garage : Garage;
			part shed : Garage;
			part barn : Garage;
		}`)
	garage, shed, barn := instantiate("garage"), instantiate("shed"), instantiate("barn")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Garage")
	car, err := evalIn(t, ctx, scope, "new Car()")
	if err != nil {
		t.Fatalf("new Car(): %v", err)
	}
	if _, err := ctx.endOccurrence(garage); err != nil {
		t.Fatalf("endOccurrence(garage): %v", err)
	}
	if err := garage.SetFeatureValue(ctx, "slot", car); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Fatalf("ended garage.slot := live car = %v; want ErrOccurrenceLifetime", err)
	}
	id, _ := car.Object()
	if owner := ctx.instances[id].owner; owner != nil {
		t.Errorf("the car's home after the refused write = %v; want none", owner)
	}
	if fv, err := garage.GetFeatureValue(ctx, "slot"); err != nil || len(heldObjects(fv.HeldValue())) != 0 {
		t.Errorf("ended garage.slot = %s, %v; want no object, the write refused", FormatValue(fv.HeldValue()), err)
	}
	if l, _ := ctx.OccurrenceLife(id); !l.Alive() {
		t.Errorf("OccurrenceLife(car) = %v; want alive, the refused write ended nothing", l)
	}
	if err := garage.SetFeatureValue(ctx, "seen", car); err != nil {
		t.Errorf("ended garage.seen := live car = %v; want held, a reference makes no portion", err)
	}
	if err := shed.SetFeatureValue(ctx, "slot", car); err != nil {
		t.Fatalf("shed.slot := car: %v", err)
	}
	if _, err := ctx.endOccurrence(shed); err != nil {
		t.Fatalf("endOccurrence(shed): %v", err)
	}
	if err := garage.SetFeatureValue(ctx, "slot", car); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Errorf("ended garage.slot := car ended after it = %v; want ErrOccurrenceLifetime", err)
	}
	if _, err := ctx.endOccurrence(barn); err != nil {
		t.Fatalf("endOccurrence(barn): %v", err)
	}
	if err := barn.SetFeatureValue(ctx, "slot", car); err != nil {
		t.Errorf("ended barn.slot := car ended before it = %v; want held, its life within the whole's", err)
	}
}

// testObjectLifecycleCompositeCycle: a composite write that would make the holder a portion of
// itself — holding itself, or a whole it is a portion of — is refused with ErrOccurrenceLifetime and
// holds nothing, so destroying the would-be portion cannot end its whole.
func testObjectLifecycleCompositeCycle(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			part def Node { part child : Node[0..1]; }
			part root : Node;
			calc def Drop { in n : Node; return : Node = destroy(n); }
		}`)
	root := instantiate("root")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Node")
	leaf, err := evalIn(t, ctx, scope, "new Node()")
	if err != nil {
		t.Fatalf("new Node(): %v", err)
	}
	if err := root.SetFeatureValue(ctx, "child", leaf); err != nil {
		t.Fatalf("root.child := leaf: %v", err)
	}
	id, _ := leaf.Object()
	leafInst := ctx.instances[id]
	for _, whole := range []*Instance{root, leafInst} {
		err := leafInst.SetFeatureValue(ctx, "child", objectValue(whole))
		if !errors.Is(err, ErrOccurrenceLifetime) {
			t.Errorf("leaf.child := #%d = %v; want ErrOccurrenceLifetime, it would make the leaf a portion of itself", whole.ID, err)
		}
	}
	if fv := leafInst.FeatureValues["child"]; fv != nil && fv.Written {
		t.Errorf("leaf.child holds %v after the refused writes; want nothing written", fv.HeldValue())
	}
	if _, err := evalIn(t, ctx, scope, "Drop(root.child)"); err != nil {
		t.Fatalf("Drop(root.child): %v", err)
	}
	if l, _ := ctx.OccurrenceLife(root.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(root) = %v after destroying its leaf; want alive", l)
	}
}

// testObjectLifecycleForgetsMessages: the messages addressed to a destroyed object, or routed to
// a destroyed port of a live one, which no consumer can take, leave the bus with it, and come
// back when the destruction is rolled back.
func testObjectLifecycleForgetsMessages(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			attribute def Ping;
			port def Ear;
			part def Device { port ear : Ear; }
			part def Fleet { part units : Device[0..*]; }
			part fleet : Fleet;
			part other : Device;
			calc def Drop { in f : Fleet; return : Fleet = destroy(f); }
		}`)
	fleet, other := instantiate("fleet"), instantiate("other")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Fleet")
	unit, err := evalIn(t, ctx, scope, "new Device()")
	if err != nil {
		t.Fatalf("new Device(): %v", err)
	}
	if err := fleet.SetFeatureValue(ctx, "units", unit); err != nil {
		t.Fatalf("fleet.units := unit: %v", err)
	}
	id, _ := unit.Object()
	for _, to := range []int64{id, other.ID} {
		ctx.PostMessage(Message{SignalType: "Ping", Object: to})
	}
	_, rollback := ctx.beginJournal()
	if _, err := evalIn(t, ctx, scope, "Drop(fleet)"); err != nil {
		t.Fatalf("Drop(fleet): %v", err)
	}
	if got := ctx.PendingMessages(); len(got) != 1 || got[0].Object != other.ID {
		t.Errorf("pending after destroying the fleet: %v; want the one Ping to other alone", got)
	}
	rollback()
	if got := ctx.PendingMessages(); len(got) != 2 {
		t.Errorf("pending after rolling the destruction back: %v; want both Pings", got)
	}

	ear, err := ctx.portInstanceID(other, "ear")
	if err != nil {
		t.Fatalf("portInstanceID(other.ear): %v", err)
	}
	ctx.PostMessage(Message{SignalType: "Ping", Object: other.ID, Port: "ear", PortID: ear, Delivery: DeliverPort})
	if err := ctx.destroy(ctx.instances[ear]); err != nil {
		t.Fatalf("destroy(other.ear): %v", err)
	}
	if l, _ := ctx.OccurrenceLife(other.ID); !l.Alive() {
		t.Fatalf("OccurrenceLife(other) = %v after destroying its port; want alive", l)
	}
	for _, msg := range ctx.PendingMessages() {
		if msg.PortID == ear {
			t.Errorf("pending after destroying other.ear still routes to it: %v; want the Ping to the port gone", msg)
		}
	}
	if got := ctx.PendingMessages(); len(got) != 2 {
		t.Errorf("pending after destroying other.ear: %v; want the two Pings to the objects themselves", got)
	}
}

// testObjectLifecycleDerivationsFollowLives: a `=` value that read a car's feature, whether the
// car is alive, or the extent derives again once a car is created — by a `=` value deriving
// too — or destroyed, rather than answering from what it derived before.
func testObjectLifecycleDerivationsFollowLives(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			private import SequenceFunctions::*;
			part def Car { attribute n : Integer = 2; }
			part def Garage {
				part slot : Car[0..1];
				attribute slotN : Integer = slot.n;
				attribute slotAlive : Boolean = isDuring(slot);
				attribute cars : Natural = (all Car)->size();
				attribute spareN : Integer = (new Car()).n;
			}
			part garage : Garage;
			calc def Scrap { in c : Car; return : Car[0..1] = destroy(c); }
		}`)
	garage := instantiate("garage")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Garage")
	read := func(name string) (string, error) {
		fv, err := garage.GetFeatureValue(ctx, name)
		if err != nil {
			return "", err
		}
		return FormatValue(fv.HeldValue()), nil
	}
	if got, err := read("cars"); err != nil || got != "0" {
		t.Fatalf("garage.cars before any car = %s, %v; want 0", got, err)
	}
	car, err := evalIn(t, ctx, scope, "new Car()")
	if err != nil {
		t.Fatalf("new Car(): %v", err)
	}
	if got, err := read("cars"); err != nil || got != "1" {
		t.Errorf("garage.cars after new Car() = %s, %v; want 1", got, err)
	}
	if _, err := read("spareN"); err != nil {
		t.Fatalf("garage.spareN: %v", err)
	}
	if got, err := read("cars"); err != nil || got != "2" {
		t.Errorf("garage.cars after the derived spareN's new Car() = %s, %v; want 2", got, err)
	}
	if err := garage.SetFeatureValue(ctx, "slot", car); err != nil {
		t.Fatalf("garage.slot := car: %v", err)
	}
	if got, err := read("slotN"); err != nil || got != "2" {
		t.Fatalf("garage.slotN = %s, %v; want 2", got, err)
	}
	if got, err := read("slotAlive"); err != nil || got != "true" {
		t.Fatalf("garage.slotAlive = %s, %v; want true", got, err)
	}
	_, rollback := ctx.beginJournal()
	if _, err := evalIn(t, ctx, scope, "Scrap(garage.slot)"); err != nil {
		t.Fatalf("Scrap(garage.slot): %v", err)
	}
	if got, err := read("slotAlive"); err != nil || got != "false" {
		t.Errorf("garage.slotAlive after scrapping the car = %s, %v; want false", got, err)
	}
	rollback()
	if got, err := read("slotAlive"); err != nil || got != "true" {
		t.Errorf("garage.slotAlive after rolling the scrapping back = %s, %v; want true", got, err)
	}
	if _, err := evalIn(t, ctx, scope, "Scrap(garage.slot)"); err != nil {
		t.Fatalf("Scrap(garage.slot): %v", err)
	}
	if _, err := read("slotN"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("garage.slotN after scrapping the car = %v; want ErrOccurrenceDestroyed, derived again", err)
	}
	if got, err := read("slotAlive"); err != nil || got != "false" {
		t.Errorf("garage.slotAlive after scrapping the car = %s, %v; want false", got, err)
	}
	if got, err := read("cars"); err != nil || got != "1" {
		t.Errorf("garage.cars after scrapping the car = %s, %v; want 1, the spare", got, err)
	}
}

// testObjectLifecycleImagedDerivationsFollowLives: an image carries that a `=` value read the
// lives, so where it is materialized the value derives again once a car is created or destroyed.
func testObjectLifecycleImagedDerivationsFollowLives(t *testing.T) {
	_, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			private import SequenceFunctions::*;
			part def Car;
			part def Garage {
				part slot : Car[0..1];
				attribute cars : Natural = (all Car)->size();
				attribute slotAlive : Boolean = isDuring(slot);
			}
			part def Lot { attribute cars : Natural = (all Car)->size(); }
		}`)
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Garage")
	made, err := evalIn(t, ctx, scope, "new Garage()")
	if err != nil {
		t.Fatalf("new Garage(): %v", err)
	}
	garageID, _ := made.Object()
	garage := ctx.instances[garageID]
	car, err := evalIn(t, ctx, scope, "new Car()")
	if err != nil {
		t.Fatalf("new Car(): %v", err)
	}
	if err := garage.SetFeatureValue(ctx, "slot", car); err != nil {
		t.Fatalf("garage.slot := car: %v", err)
	}
	read := func(ctx *Context, garage *Instance, name string) (string, error) {
		fv, err := garage.GetFeatureValue(ctx, name)
		if err != nil {
			return "", err
		}
		return FormatValue(fv.HeldValue()), nil
	}
	if got, err := read(ctx, garage, "cars"); err != nil || got != "1" {
		t.Fatalf("garage.cars = %s, %v; want 1", got, err)
	}
	if got, err := read(ctx, garage, "slotAlive"); err != nil || got != "true" {
		t.Fatalf("garage.slotAlive = %s, %v; want true", got, err)
	}

	img, err := ctx.Image(garage)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	dst := NewContext(ctx.Model(), 10000)
	dst.claimID(99)
	own, err := evalIn(t, dst, scope, "new Lot()")
	if err != nil {
		t.Fatalf("new Lot() in the destination: %v", err)
	}
	ownID, _ := own.Object()
	local, _ := dst.Instance(ownID)
	if got, err := read(dst, local, "cars"); err != nil || got != "0" {
		t.Fatalf("local.cars before the image = %s, %v; want 0", got, err)
	}
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, err := read(dst, local, "cars"); err != nil || got != "1" {
		t.Errorf("local.cars after the image = %s, %v; want 1, derived again over the imaged car", got, err)
	}
	copied, _ := dst.Instance(garage.ID)
	if got, err := read(dst, copied, "cars"); err != nil || got != "1" {
		t.Fatalf("copy.cars as imaged = %s, %v; want 1", got, err)
	}
	if _, err := evalIn(t, dst, scope, "new Car()"); err != nil {
		t.Fatalf("new Car() in the destination: %v", err)
	}
	if got, err := read(dst, copied, "cars"); err != nil || got != "2" {
		t.Errorf("copy.cars after new Car() in the destination = %s, %v; want 2, derived again", got, err)
	}
	carID, _ := car.Object()
	if err := dst.destroy(dst.instances[carID]); err != nil {
		t.Fatalf("destroy(the copy's car) in the destination: %v", err)
	}
	if got, err := read(dst, copied, "slotAlive"); err != nil || got != "false" {
		t.Errorf("copy.slotAlive after scrapping its car = %s, %v; want false, derived again", got, err)
	}
	if got, err := read(dst, copied, "cars"); err != nil || got != "1" {
		t.Errorf("copy.cars after scrapping its car = %s, %v; want 1", got, err)
	}
	if got, err := read(ctx, garage, "cars"); err != nil || got != "1" {
		t.Errorf("garage.cars in the source after the destination's moves = %s, %v; want 1, untouched", got, err)
	}
}

// testObjectLifecycleStartsAfterStore: the behaviors a write starts on the object written run once
// the feature holds it, so one reading the holder through the extent sees the object, not what was there.
func testObjectLifecycleStartsAfterStore(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*; private import SequenceFunctions::*;
			part def Device { attribute slotted : Integer; }
			part def Rack {
				part slot : Device[0..1] {
					perform action look { first start; then action count { assign slotted := (all Rack).slot->size(); } then done; }
				}
			}
			part rack : Rack;
		}`)
	rack := instantiate("rack")
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Rack")
	device, err := evalIn(t, ctx, scope, "new Device()")
	if err != nil {
		t.Fatalf("new Device(): %v", err)
	}
	if err := rack.SetFeatureValue(ctx, "slot", device); err != nil {
		t.Fatalf("rack.slot := device: %v", err)
	}
	id, _ := device.Object()
	fv, err := ctx.instances[id].GetFeatureValue(ctx, "slotted")
	if err != nil {
		t.Fatalf("device.slotted: %v", err)
	}
	if got := FormatValue(fv.HeldValue()); got != "1" {
		t.Errorf("device.slotted = %s after the write started look; want 1, the slot holding it", got)
	}
}

// testObjectLifecycleConstructorStartsAfterStores: the behavior a constructor's first argument starts
// through the feature holding it runs once the later arguments are stored too, so it reads them.
func testObjectLifecycleConstructorStartsAfterStores(t *testing.T) {
	_, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*; private import SequenceFunctions::*;
			part def Device { attribute seen : Integer; }
			part def Pair {
				part lead : Device[0..1] {
					perform action look { first start; then action count { assign seen := (all Pair).trail->size(); } then done; }
				}
				part trail : Device[0..1];
			}
		}`)
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Pair")
	pair, err := evalIn(t, ctx, scope, "new Pair(new Device(), new Device())")
	if err != nil {
		t.Fatalf("new Pair(...): %v", err)
	}
	id, _ := pair.Object()
	lead := heldNamed(t, ctx, ctx.instances[id], "lead")
	if len(lead) != 1 {
		t.Fatalf("pair.lead holds %d objects; want 1", len(lead))
	}
	fv, err := lead[0].GetFeatureValue(ctx, "seen")
	if err != nil {
		t.Fatalf("lead.seen: %v", err)
	}
	if got := FormatValue(fv.HeldValue()); got != "1" {
		t.Errorf("lead.seen = %s after the construction started look; want 1, the trail stored before it ran", got)
	}
}

// testObjectLifecycleBoundDefaultStartsAfterStore: a default object reached through a binding's
// endpoint starts its behavior once the endpoint holds it, so the behavior reads it there.
func testObjectLifecycleBoundDefaultStartsAfterStore(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*; private import SequenceFunctions::*;
			part def Device { attribute seen : Integer; }
			part def Rack {
				part source : Device[0..1] = new Device() {
					perform action look { first start; then action count { assign seen := (all Rack).source->size(); } then done; }
				}
				part slot : Device[0..1];
				bind slot = source;
			}
			part rack : Rack;
		}`)
	slot := heldNamed(t, ctx, instantiate("rack"), "slot")
	if len(slot) != 1 {
		t.Fatalf("rack.slot holds %d objects; want the bound default", len(slot))
	}
	fv, err := slot[0].GetFeatureValue(ctx, "seen")
	if err != nil {
		t.Fatalf("slot.seen: %v", err)
	}
	if got := FormatValue(fv.HeldValue()); got != "1" {
		t.Errorf("slot.seen = %s after the default started look; want 1, the source holding it", got)
	}
}

// testObjectLifecycleRefusedWriteJournal: a write refused when the behavior its feature adds fails
// restores ownership and leaves an enclosing journal nothing to undo, so a later move stands.
func testObjectLifecycleRefusedWriteJournal(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			part def Device;
			part def Rack {
				part slot : Device[0..1] {
					attribute bad : Integer;
					perform action boom { first start; then action b { assign bad := 1/0; } then done; }
				}
			}
			part def Shelf { part slot : Device[0..1]; }
		}`)
	rack, shelf, device := instantiate("Rack"), instantiate("Shelf"), instantiate("Device")
	commit, _ := ctx.beginJournal()
	defer commit()
	undos := len(ctx.journalUndos)
	if err := rack.SetFeatureValue(ctx, "slot", objectValue(device)); err == nil {
		t.Fatal("rack.slot := device succeeded; want the slot's failing action reported")
	}
	if device.owner != nil || len(ctx.journalUndos) != undos {
		t.Errorf("after the refused write: owner %v, %d journal entries added; want none of either", device.owner, len(ctx.journalUndos)-undos)
	}
	if err := shelf.SetFeatureValue(ctx, "slot", objectValue(device)); err != nil || device.owner != shelf {
		t.Errorf("shelf.slot := device = %v, owner %v; want the shelf to own it", err, device.owner)
	}
}

// testObjectLifecycleRolledBackTrace: the behaviors a refused write or a failed constructor
// started are rolled back trace and all, so the trace reports no step the run does not show;
// the trace of a store that is kept stays.
func testObjectLifecycleRolledBackTrace(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			part def Device;
			part def Rack {
				part slot : Device[0..1] {
					attribute bad : Integer;
					perform action boom { first start; then action b { assign bad := 1/0; } then done; }
				}
			}
			part def Shelf {
				part slot : Device[0..1] {
					attribute n : Integer;
					perform action fill { first start; then action f { assign n := 1; } then done; }
				}
			}
			part def Plant { attribute count : Integer = 0; }
			part def Worker {
				ref part p : Plant;
				perform action go {
					first start;
					then action write { assign p.count := 1; }
					then action fail { assign p.count := 1/0; }
					then done;
				}
			}
			part plant : Plant;
		}`)
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)
	rack, shelf, device, plant := instantiate("Rack"), instantiate("Shelf"), instantiate("Device"), instantiate("plant")
	// stepsIn is what the records say of the behaviors: everything but the failing evaluation's own line.
	stepsIn := func(records []TraceRecord) (steps []string) {
		for _, r := range records {
			if r.Kind == TraceLine && !strings.HasPrefix(r.text, "eval construct ") {
				steps = append(steps, r.text)
			}
		}
		return steps
	}
	before := len(tr.records)
	if err := rack.SetFeatureValue(ctx, "slot", objectValue(device)); err == nil {
		t.Fatal("rack.slot := device succeeded; want the slot's failing action reported")
	}
	if steps := stepsIn(tr.records[before:]); len(steps) != 0 {
		t.Errorf("the refused write left %d step records in the trace: %q; want none", len(steps), steps)
	}
	_, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "Plant")
	before = len(tr.records)
	if _, err := evalIn(t, ctx, scope, "new Worker(plant)"); err == nil {
		t.Fatal("new Worker(plant) succeeded; want its failing action reported")
	}
	if steps := stepsIn(tr.records[before:]); len(steps) != 0 {
		t.Errorf("the failed constructor left %d step records in the trace: %q; want none", len(steps), steps)
	}
	if fv, err := plant.GetFeatureValue(ctx, "count"); err != nil || FormatValue(fv.HeldValue()) != "0" {
		t.Errorf("plant.count = %v, %v after the failed constructor; want 0", fv, err)
	}
	before = len(tr.records)
	if err := shelf.SetFeatureValue(ctx, "slot", objectValue(device)); err != nil {
		t.Fatalf("shelf.slot := device: %v", err)
	}
	if steps := stepsIn(tr.records[before:]); len(steps) == 0 {
		t.Error("the kept write left no step records in the trace; want the fill action's steps")
	}
}

// testObjectLifecycleDeclaredPart: an object a composite feature's default or a binding makes
// is the whole's portion just like a written one, so destroying the whole ends it.
// (A binding end names a feature, so the bound object is made by a reference's default.)
func testObjectLifecycleDeclaredPart(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			part def Child;
			part def Whole {
				part byDefault : Child = new Child();
				ref part made : Child = new Child();
				part byBinding : Child;
				bind byBinding = made;
			}
			calc def DestroyWhole { in w : Whole; return : Whole = destroy(w); }
		}`)
	whole := instantiate("Whole")
	children := map[string]*Instance{}
	for _, name := range []string{"byDefault", "byBinding"} {
		fv, err := whole.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		id, ok := fv.HeldValue().Object()
		if !ok {
			t.Fatalf("%s holds %v; want an object", name, fv.HeldValue())
		}
		children[name] = ctx.instances[id]
		if child := children[name]; child.owner != whole || child.ownerFeature != name {
			t.Errorf("%s #%d owned by %v.%s; want the whole", name, child.ID, child.owner, child.ownerFeature)
		}
	}
	sym, scope := calcByName(t, ctx.model.resolver.Index().DocumentRoot("<test>"), "test", "DestroyWhole")
	if _, err := ctx.InvokeCalc(sym, []Value{objectValue(whole)}, scope); err != nil {
		t.Fatalf("destroy(whole) = %v", err)
	}
	for name, child := range children {
		if l, _ := ctx.OccurrenceLife(child.ID); !l.Destroyed {
			t.Errorf("%s #%d = %v; want destroyed with the whole", name, child.ID, l)
		}
	}
}

const objectLifecycleExploreModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		private import SequenceFunctions::*;
		part def Car { attribute n : Integer; }
		part def Garage {
			part left : Car[0..1];
			part right : Car[0..1];
			attribute count : Integer;
			attribute distinct : Boolean;
			perform action race {
				first start;
				fork split;
				action a { assign left := new Car(1); }
				action b { assign right := new Car(2); }
				join sync;
				action read {
					assign count := size(all Car);
					assign distinct := not (left === right);
				}
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then read;
				succession first read then done;
			}
		}
		part def Yard {
			part car : Car[0..1] = new Car(3);
			attribute alive : Boolean;
			perform action scrap {
				first start;
				fork split;
				action a { assign car := destroy(car); }
				action b { assign alive := isDuring(car); }
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then done;
			}
		}
	}`

// exploreInstance explores the runs of instantiating the named definition; the
// outcome of a run is what the listed attributes of the object hold.
func (m *exploreModel) exploreInstance(t *testing.T, name string, attributes ...string) *Exploration {
	t.Helper()
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatalf("policy explore: %v", err)
	}
	sym := lookupOne(t, m.idx, "test::"+name)
	result, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
		inst, err := ctx.Instantiate(sym)
		if err != nil {
			return Outcome{}, err
		}
		outputs := make(map[string]Value, len(attributes))
		for _, attribute := range attributes {
			fv, err := inst.GetFeatureValue(ctx, attribute)
			if err != nil {
				return Outcome{}, err
			}
			outputs[attribute] = fv.HeldValue()
		}
		return ctx.ActionOutcome(outputs), nil
	})
	if err != nil {
		t.Fatalf("explore %s: %v", name, err)
	}
	return result
}

// testObjectLifecycleExploreCreation: every linearization of two branches each constructing
// a car reaches one outcome, and two explorations agree move for move.
func testObjectLifecycleExploreCreation(t *testing.T) {
	m := parseLibraryModel(t, objectLifecycleExploreModel)
	first := m.exploreInstance(t, "Garage", "count", "distinct")
	if !first.Complete() || first.Runs != 2 {
		t.Fatalf("status %q after %d runs, want complete after the 2 orders of two tokens", first.Status(), first.Runs)
	}
	if got := outcomeTexts(first); len(got) != 1 || got[0] != "count = 2; distinct = true" {
		t.Fatalf("outcomes %v, want one: two distinct cars", got)
	}
	second := m.exploreInstance(t, "Garage", "count", "distinct")
	if first.Status() != second.Status() || outcomeTexts(first)[0] != outcomeTexts(second)[0] ||
		FormatChoices(first.Outcomes[0].Witness) != FormatChoices(second.Outcomes[0].Witness) {
		t.Errorf("explorations differ: %+v then %+v", first.Outcomes, second.Outcomes)
	}
}

// testObjectLifecycleExploreDestroyRace: whether the car is alive when read depends on the
// order of the branches alone, so exploration reaches exactly the two outcomes.
func testObjectLifecycleExploreDestroyRace(t *testing.T) {
	m := parseLibraryModel(t, objectLifecycleExploreModel)
	x := m.exploreInstance(t, "Yard", "alive")
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q after %d runs, want complete after the 2 orders of two tokens", x.Status(), x.Runs)
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != "alive = false|alive = true" {
		t.Fatalf("outcomes %v, want the read after and before the destroy", got)
	}
	for _, o := range x.Outcomes {
		if o.Linearizations != 1 {
			t.Errorf("%s reached by %d linearizations, want 1", o.Outcome, o.Linearizations)
		}
	}
}
