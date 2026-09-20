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
	t.Run("a_failed_constructor_rolls_back_what_its_behaviors_wrote", testObjectLifecycleFailedConstructorWrites)
	t.Run("a_refused_write_leaves_no_adoption_for_an_outer_rollback", testObjectLifecycleRefusedWriteJournal)
	t.Run("a_failed_constructor_revives_what_its_behaviors_destroyed_whole", testObjectLifecycleFailedConstructorDestroy)
	t.Run("a_part_declared_or_bound_to_a_new_object_ends_with_the_whole", testObjectLifecycleDeclaredPart)
	t.Run("an_object_two_wholes_hold_composite_ends_with_either", testObjectLifecycleSharedPortion)
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
	fv, err := inst.GetFeatureValue(ctx, "cars")
	if err != nil {
		t.Fatalf("cars: %v", err)
	}
	var out []*Instance
	for _, el := range elementsOf(fv.HeldValue()) {
		id, ok := el.Object()
		if !ok {
			t.Fatalf("cars holds %v, want objects", fv.HeldValue())
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
