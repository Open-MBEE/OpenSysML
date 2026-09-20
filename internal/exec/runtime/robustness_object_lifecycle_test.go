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
