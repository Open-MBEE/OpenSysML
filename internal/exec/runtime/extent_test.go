package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestNamespaceBindingDenotesOneObject requires a namespace-level object usage given a
// value to denote one binding for the run (KerML 1.0 §7.4.11): every read of it reads the
// same object, an untyped usage included, and `all T` reaches those objects before any
// read of the usage. A `default` value, binding nothing, is not a namespace root.
func TestNamespaceBindingDenotesOneObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			part def Car;
			part def Truck :> Car;
			part def Boat;
			ref part car : Car = new Car();
			part fleet = new Truck();
			part dinghy = new Boat();
			part spare : Car;
			ref part alias : Car = spare;
			part loaner : Car default new Car();
			part rental : Car := new Car();
			part def Depot {
				part parked : Car default new Car();
			}
			part depot : Depot;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	size := func(expr string) int {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return len(elementsOf(val))
	}
	// car and fleet are bound before any read of them; alias denotes spare, once.
	if got := size("all Car"); got != 6 {
		t.Fatalf("size(all Car) before reading the usages = %d, want 6 (car, fleet, spare, loaner, rental, depot.parked)", got)
	}
	if got := size("all Truck"); got != 1 {
		t.Fatalf("size(all Truck) = %d, want 1", got)
	}
	// An untyped usage is reached through its value's static type: the dinghy, a Boat, is
	// not read for the cars.
	for _, inst := range ctx.instances {
		if inst.Type != nil && inst.Type.Name == "Boat" {
			t.Errorf("all Car read dinghy, whose value is a Boat")
		}
	}
	// A default or initial value at namespace level is realized for the run, the one individual
	// there is, since no other value can be given for it.
	for _, name := range []string{"car", "fleet", "alias", "loaner", "rental"} {
		first, err := evalIn(t, ctx, pkg.Scope, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		second, err := evalIn(t, ctx, pkg.Scope, name)
		if err != nil {
			t.Fatalf("%s again: %v", name, err)
		}
		a, aok := first.Object()
		b, bok := second.Object()
		if !aok || !bok || a != b {
			t.Errorf("%s read twice = %v then %v, want one object", name, first, second)
		}
	}
	if got := size("all Car"); got != 6 {
		t.Errorf("size(all Car) after reading the usages = %d, want 6", got)
	}
	same, err := evalIn(t, ctx, pkg.Scope, "alias === spare")
	if err != nil || !(same.isBool() && same.Const.Bool) {
		t.Errorf("alias === spare = %v, %v; want true", same, err)
	}
	// A default is read when the depot is built, once, and the object it made is held.
	same, err = evalIn(t, ctx, pkg.Scope, "depot.parked === depot.parked")
	if err != nil || !(same.isBool() && same.Const.Bool) {
		t.Errorf("depot.parked === depot.parked = %v, %v; want true", same, err)
	}
}

// TestNamespaceBindingProbeIsUndone requires a binding a probe made to be undone with the
// probe, so the run reads the usage afresh rather than an object the probe abandoned.
func TestNamespaceBindingProbeIsUndone(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			part def Car;
			ref part car : Car = new Car();
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	end := ctx.beginProbe()
	probed, err := evalIn(t, ctx, pkg.Scope, "car")
	if err != nil {
		t.Fatalf("car in a probe: %v", err)
	}
	end()
	if len(ctx.namespaceBindings) != 0 {
		t.Fatalf("bindings after the probe = %d, want none", len(ctx.namespaceBindings))
	}
	if probedID, _ := probed.Object(); ctx.instances[probedID] != nil {
		t.Fatalf("the probe's object %d outlived the probe", probedID)
	}
	read, err := evalIn(t, ctx, pkg.Scope, "car")
	if err != nil {
		t.Fatalf("car: %v", err)
	}
	id, _ := read.Object()
	if _, live := ctx.instances[id]; !live {
		t.Fatalf("car after the probe denotes %v, which the run does not hold", read)
	}
	again, err := evalIn(t, ctx, pkg.Scope, "car")
	if err != nil {
		t.Fatalf("car again: %v", err)
	}
	if againID, _ := again.Object(); againID != id {
		t.Errorf("car read twice = %d then %d, want one object", id, againID)
	}
}

// TestNamespaceBindingFailureLeavesNoBinding requires a value that fails to evaluate to
// bind nothing, so the extent refuses with that failure and a later read fails the same way.
func TestNamespaceBindingFailureLeavesNoBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			part def Car;
			part def Boat;
			ref part car : Car = new Boat();
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, expr := range []string{"all Car", "car", "all Car"} {
		_, err := evalIn(t, ctx, pkg.Scope, expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
	if len(ctx.namespaceBindings) != 0 {
		t.Errorf("bindings after failures = %d, want none", len(ctx.namespaceBindings))
	}
}

// TestNamespaceBindingFailureLeavesNoObject requires a refused binding to leave behind none of
// the objects its value constructed nor the behaviors they started, however often it is read;
// an extent taken elsewhere in the model reaches the usage, fails on the same refusal naming it,
// and leaves nothing behind either — it never counts what the value constructed.
func TestNamespaceBindingFailureLeavesNoObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			part def Car;
			part def Boat {
				exhibit state life { entry; then afloat; state afloat; }
			}
			ref part car : Car = new Boat();
		}
		package elsewhere {
			private import SequenceFunctions::*;
			private import test::Boat;
			part def Buoy;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	other, ok := idx.DocumentRoot("<test>").LookupLocal("elsewhere")
	if !ok || other.Scope == nil {
		t.Fatal("elsewhere package not indexed")
	}
	for i := range 3 {
		_, err := evalIn(t, ctx, pkg.Scope, "car")
		if !errors.Is(err, ErrTypeMismatch) {
			t.Fatalf("read %d: error = %v, want ErrTypeMismatch", i, err)
		}
		if n := len(ctx.instances); n != 0 {
			t.Errorf("read %d: objects held = %d, want none", i, n)
		}
		if n := len(ctx.created); n != 0 {
			t.Errorf("read %d: objects registered = %d, want none", i, n)
		}
		if n := len(ctx.objectBehaviors); n != 0 {
			t.Errorf("read %d: behaviors attached = %d, want none", i, n)
		}
	}
	_, err := evalIn(t, ctx, other.Scope, "size(all Boat)")
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "usage car") {
		t.Fatalf("size(all Boat) after failed bindings: error = %v, want ErrTypeMismatch naming the usage", err)
	}
	if n := len(ctx.instances) + len(ctx.created) + len(ctx.objectBehaviors); n != 0 {
		t.Errorf("objects and behaviors left behind by the extent = %d, want none", n)
	}
	val, err := evalIn(t, ctx, other.Scope, "size(all Buoy)")
	if err != nil {
		t.Fatalf("size(all Buoy): %v", err)
	}
	if val.Kind != ValConst || val.Const.Int != 0 {
		t.Errorf("size(all Buoy) after failed bindings = %v, want 0", val)
	}
}

// TestNamespaceBindingToAnExtent requires a namespace usage bound to an extent of its own type
// to stand for no object while it is being bound, and a usage whose value depends on it to stand
// for none yet, so `all Car` binds it to the Cars there are; a value reaching back to its own
// usage is refused as a cycle.
func TestNamespaceBindingToAnExtent(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SequenceFunctions::*;
			part def Car;
			part def Truck :> Car;
			part car : Car;
			ref part spare : Car = new Car();
			ref part cars : Car[*] = all Car;
			ref part lead : Car = cars#(1);
			ref part convoy : Car[*] = (new Truck(), cars);
		}
		package broken {
			part def Car;
			ref part loop : Car = loop;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	broken, ok := idx.DocumentRoot("<test>").LookupLocal("broken")
	if !ok || broken.Scope == nil {
		t.Fatal("broken package not indexed")
	}
	if _, err := evalIn(t, ctx, pkg.Scope, "cars"); err != nil {
		t.Fatalf("cars: %v", err)
	}
	if got := len(ctx.created); got != 2 {
		t.Errorf("objects after binding cars = %d, want 2 (car, spare): a Truck convoy made before cycling stays", got)
	}
	for expr, want := range map[string]int{"cars": 2, "all Car": 3, "convoy": 3, "all Truck": 1} {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := len(elementsOf(val)); got != want {
			t.Errorf("size(%s) = %d, want %d", expr, got, want)
		}
	}
	same, err := evalIn(t, ctx, pkg.Scope, "lead === car")
	if err != nil || !same.isBool() || !same.Const.Bool {
		t.Errorf("lead === car = %v, %v; want true", same, err)
	}
	for _, expr := range []string{"loop", "all Car"} {
		if _, err := evalIn(t, ctx, broken.Scope, expr); !errors.Is(err, ErrCyclicFeatureValue) {
			t.Errorf("%s in broken: error = %v, want ErrCyclicFeatureValue", expr, err)
		}
	}
	if len(ctx.bindingStack) != 0 {
		t.Errorf("usages still being bound = %d, want none", len(ctx.bindingStack))
	}
}

// The extent materializes a usage's object under the registered scope tree's symbol, so
// reading the usage afterwards — or before — reads the same object.
func TestExtentRootsAreTheSymbolsTheCallerResolvesIn(t *testing.T) {
	const src = `package Demo {
		private import SequenceFunctions::*;
		part def Car { attribute n : Integer = 3; }
		part car : Car;
	}`
	for name, order := range map[string][]string{
		"extent first": {"size(all Car)", "car.n", "size(all Car)"},
		"usage first":  {"car.n", "size(all Car)"},
	} {
		ctx, scope := documentContextOver(t, src)
		demo := resolveSymbol(t, scope, "Demo").Scope
		for _, expr := range order {
			if _, err := evalIn(t, ctx, demo, expr); err != nil {
				t.Fatalf("%s, %s: %v", name, expr, err)
			}
		}
		if n := len(ctx.instances); n != 1 {
			t.Errorf("%s: the context holds %d objects, want the one car", name, n)
		}
		val, err := evalIn(t, ctx, demo, "size(all Car)")
		if err != nil || FormatValue(val) != "1" {
			t.Errorf("%s: size(all Car) = %s, %v; want 1", name, FormatValue(val), err)
		}
	}
}
