package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessExtentQueries covers extent and namespace-collection queries over
// types, parts and connectors that cannot be answered.
func TestRuntimeRobustnessExtentQueries(t *testing.T) {
	t.Run("extent_of_an_unresolved_or_unbounded_type", testExtentOfAnUnresolvedOrUnboundedType)
	t.Run("extent_reaching_a_namespace_collection", testExtentReachingANamespaceCollection)
	t.Run("namespace_collection_that_cannot_be_constructed", testNamespaceCollectionThatCannotBeConstructed)
	t.Run("namespace_collection_of_unfixed_count", testNamespaceCollectionOfUnfixedCount)
	t.Run("chained_write_through_a_namespace_collection", testChainedWriteThroughANamespaceCollection)
	t.Run("namespace_collection_over_budget", testNamespaceCollectionOverBudget)
	t.Run("extent_over_an_object_that_cannot_be_read", testExtentOverAnObjectThatCannotBeRead)
	t.Run("extent_reaching_a_far_usage_that_cannot_be_read", testExtentReachingAFarUsageThatCannotBeRead)
	t.Run("extent_over_far_usages_under_the_element_budget", testExtentOverFarUsagesUnderTheElementBudget)
	t.Run("extent_over_recursive_composition", testExtentOverRecursiveComposition)
	t.Run("extent_through_a_value_recursing_and_not", testExtentThroughAValueRecursingAndNot)
}

// `all T` names a type: a name resolving to nothing, an operand that is no name, and a
// data type declaring no values are each a typed error, and none disturbs the run.
func testExtentOfAnUnresolvedOrUnboundedType(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Car { part wheels : Wheel[2]; }
		part car : Car;
		package Spares;
		dependency Supply from Spares to Car;
		comment Note about Car /* wheels */
		calc missing { return : Natural = size(all MissingType); }
		calc noName { return : Natural = size(all (1 + 2)); }
		calc ofPackage { return : Natural = size(all Spares); }
		calc ofQualifiedPackage { return : Natural = size(all P::Spares); }
		calc ofRelationship { return : Natural = size(all Supply); }
		calc ofComment { return : Natural = size(all Note); }
		calc unbounded { return : Natural = size(all Integer); }
		calc unboundedString { return : Natural = size(all String); }
		attribute def Point { attribute x : Real; }
		attribute origin : Point;
		calc unboundedStructured { return : Natural = size(all Point); }
		calc counted { return : Natural = size(all Wheel); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	for _, tc := range []struct {
		calc string
		want error
		name string
	}{
		{"missing", errUnresolvedType, "MissingType"},
		{"noName", ErrTypeMismatch, "requires the name of a type"},
		{"ofPackage", ErrTypeMismatch, "Spares is a package"},
		{"ofQualifiedPackage", ErrTypeMismatch, "P::Spares is a package"},
		{"ofRelationship", ErrTypeMismatch, "Supply is a dependency"},
		{"ofComment", ErrTypeMismatch, "Note is a comment"},
		{"unbounded", errUnboundedExtent, "Integer"},
		{"unboundedString", errUnboundedExtent, "String"},
		{"unboundedStructured", errUnboundedExtent, "Point is a data type"},
	} {
		_, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, tc.calc), nil, pkg.Scope)
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", tc.calc, err, tc.want)
		}
		if !strings.Contains(err.Error(), tc.name) {
			t.Errorf("%s: error = %v, want %q named", tc.calc, err, tc.name)
		}
	}
	if _, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "car")); err != nil {
		t.Fatal(err)
	}
	got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, "counted"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "2" {
		t.Errorf("size(all Wheel) = %s, %v; want 2", FormatValue(got), err)
	}
}

// testExtentReachingANamespaceCollection: a namespace-level usage of several occurrences
// (`part wheels : Wheel[2]` in a package) denotes its lower bound of objects for the run,
// created once, so an extent counts them, each once, along with the objects nested in them,
// wherever in the model the extent is taken and the usage declared; a `[0..*]` usage denotes
// none. Read directly, a usage of exact count is the sequence of its objects on every surface,
// one of open count undetermined of that count, as a nested collection reads. A namespace-level
// port denotes no object the run reaches, so an extent it may contribute to is refused rather
// than answered short, while one a port nested in a part contributes to is answered.
func testExtentReachingANamespaceCollection(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Hub { part wheel : Wheel; }
		part def Seat;
		port def Link;
		part wheels : Wheel[2];
		part hubs : Hub[1..*];
		part spares : Wheel[0..*];
		part seat : Seat;
		port link : Link;
		calc wheelCount { return : Natural = size(all Wheel); }
		calc hubCount { return : Natural = size(all Hub); }
		calc linkCount { return : Natural = size(all Link); }
		calc seatCount { return : Natural = size(all Seat); }
		package Q {
			part seat2 : Seat;
			part spares : Wheel[0..*];
			calc seatCount { return : Natural = size(all Seat); }
			calc wheelCount { return : Natural = size(all Wheel); }
		}
	}
	package R {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		calc linkCount { return : Natural = size(all P::Link); }
		package S {
			port links : P::Link[2];
			calc linkCount { return : Natural = size(all P::Link); }
		}
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	for calc, want := range map[string]string{"wheelCount": "3", "hubCount": "1", "seatCount": "2"} {
		for attempt := 1; attempt <= 2; attempt++ {
			got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
			if err != nil || FormatValue(got) != want {
				t.Errorf("attempt %d: %s = %s, %v; want %s: the two wheels, the hub and its wheel, both packages' seats", attempt, calc, FormatValue(got), err, want)
			}
		}
	}
	wheels, ok := ctx.liveOccurrences(resolveSymbol(t, pkg.Scope, "wheels"))
	if !ok || len(wheels) != 2 {
		t.Fatalf("wheels denotes %d objects, want the two created once", len(wheels))
	}
	want := fmt.Sprintf("[instance(%d), instance(%d)]", wheels[0].ID, wheels[1].ID)
	val, err := ctx.EvalDeclaredValue(resolveSymbol(t, pkg.Scope, "wheels"))
	wantFormatted(t, "declared wheels", val, err, want)
	val, err = evalIn(t, ctx, pkg.Scope, "wheels")
	wantFormatted(t, "wheels", val, err, want)
	val, err = ctx.EvalDeclaredValue(resolveSymbol(t, pkg.Scope, "hubs"))
	wantUndetermined(t, "declared hubs", val, err, "[1..*]")
	for src, count := range map[string]string{"hubs": "[1..*]", "size(hubs)": "[1]", "hubs#(1)": "[1]", "hubs.wheel": "[1..*]"} {
		val, err = evalIn(t, ctx, pkg.Scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	if _, ok := ctx.occurrences[resolveSymbol(t, pkg.Scope, "spares")]; ok {
		t.Error("spares, a [0..*] usage, denotes objects; want none")
	}
	_, err = ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "linkCount"), nil, pkg.Scope)
	if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), "link is a port") {
		t.Errorf("size(all Link) = %v, want %v naming the port link", err, errExtentUnavailable)
	}
	q := resolveSymbol(t, pkg.Scope, "Q")
	got, err := invokeValue(ctx, resolveSymbol(t, q.Scope, "seatCount"), nil, q.Scope)
	if err != nil || FormatValue(got) != "2" {
		t.Errorf("Q: size(all Seat) = %s, %v; want 2: both packages' seats, and the wheel collections hold no Seat", FormatValue(got), err)
	}
	got, err = invokeValue(ctx, resolveSymbol(t, q.Scope, "wheelCount"), nil, q.Scope)
	if err != nil || FormatValue(got) != "3" {
		t.Errorf("Q: size(all Wheel) = %s, %v; want 3: the enclosing package's wheels, once", FormatValue(got), err)
	}
	if got := len(ctx.instances); got != 6 {
		t.Errorf("%d objects stand, want 6: two wheels, a hub and its wheel, two seats", got)
	}
	r := resolveSymbol(t, root, "R")
	_, err = ctx.InvokeCalc(resolveSymbol(t, r.Scope, "linkCount"), nil, r.Scope)
	if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), "link is a port") {
		t.Errorf("R: size(all Link) = %v, want the other package's port refused", err)
	}
	s := resolveSymbol(t, r.Scope, "S")
	_, err = ctx.InvokeCalc(resolveSymbol(t, s.Scope, "linkCount"), nil, s.Scope)
	if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), "link is a port") {
		t.Errorf("S: size(all Link) = %v, want the first unreadable usage of the model refused", err)
	}

	model, resolver, root = parseAndBuildLibraryModel(t, `package R {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		port def Link;
		part def Rig { port p : Link; }
		part rig : Rig;
		port spare : Link[0..1];
		calc linkCount { return : Natural = size(all Link); }
	}`)
	r = resolveSymbol(t, root, "R")
	ctx = NewContext(typedModel(model, resolver), 1000)
	got, err = invokeValue(ctx, resolveSymbol(t, r.Scope, "linkCount"), nil, r.Scope)
	if err != nil || FormatValue(got) != "1" {
		t.Errorf("R: size(all Link) = %s, %v; want 1: the rig's port, an optional one holding nothing", FormatValue(got), err)
	}

	model, resolver, root = parseAndBuildLibraryModel(t, `package S {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		port def Link;
		part def Rig { port p : Link; }
		part rig : Rig;
		calc linkCount { return : Natural = size(all Link); }
		package T {
			port links : Link[2];
		}
	}`)
	s = resolveSymbol(t, root, "S")
	ctx = NewContext(typedModel(model, resolver), 1000)
	_, err = ctx.InvokeCalc(resolveSymbol(t, s.Scope, "linkCount"), nil, s.Scope)
	if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), "links") || !strings.Contains(err.Error(), "port") {
		t.Errorf("S: size(all Link) = %v, want the nested package's two-port usage refused", err)
	}
}

// testNamespaceCollectionOfUnfixedCount: a namespace-level usage whose multiplicity bound the
// model does not evaluate (`[2..n]`, `[n]`) fixes no count, so it is never materialized — not
// as one object, not as its lower bound: an extent that may reach it is refused naming the usage
// and its bounds, and a model-level read is undetermined of the bounds the declaration does fix.
func testNamespaceCollectionOfUnfixedCount(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		part def Wheel;
		part def Hub;
		part def Seat;
		attribute n : Natural;
		part wheels : Wheel[2..n];
		part hubs : Hub[n];
		part seat : Seat;
		calc wheelCount { return : Natural = size(all Wheel); }
		calc hubCount { return : Natural = size(all Hub); }
		calc seatCount { return : Natural = size(all Seat); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	for calc, want := range map[string]string{"wheelCount": "wheels declares [2..?]", "hubCount": "hubs declares [?..?]"} {
		_, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
		if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s = %v, want %v naming %q", calc, err, errExtentUnavailable, want)
		}
	}
	got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, "seatCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "1" {
		t.Errorf("seatCount = %s, %v; want 1: the usages of unfixed count hold no Seat", FormatValue(got), err)
	}
	for src, count := range map[string]string{"wheels": "[2..?]", "size(wheels)": "[1]", "hubs": "[?..?]", "hubs#(1)": "[1]"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	val, err := ctx.EvalDeclaredValue(resolveSymbol(t, pkg.Scope, "wheels"))
	wantUndetermined(t, "declared wheels", val, err, "[2..?]")
	val, err = evalIn(t, ctx, pkg.Scope, "notEmpty(wheels)")
	wantFormatted(t, "notEmpty(wheels)", val, err, "true")
	for _, name := range []string{"wheels", "hubs"} {
		if _, ok := ctx.occurrences[resolveSymbol(t, pkg.Scope, name)]; ok {
			t.Errorf("%s, a usage of unfixed count, denotes objects; want none", name)
		}
	}
	if got := len(ctx.instances); got != 1 {
		t.Errorf("%d objects stand, want 1: the seat alone", got)
	}
}

// testNamespaceCollectionThatCannotBeConstructed: a namespace-level usage of several
// occurrences whose type cannot be constructed — the behavior its definition exhibits fails on
// entry — is refused with that failure naming the usage, whether read or counted, and leaves no
// object behind: not a partial extent, not an occurrence for a later read to find.
func testNamespaceCollectionThatCannotBeConstructed(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Bad {
			attribute hits : Rational = 0;
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := 1 / hits; } }
			}
		}
		part wheels : Wheel[2];
		part bads : Bad[3];
		calc badCount { return : Natural = size(all Bad); }
		calc wheelCount { return : Natural = size(all Wheel); }
		calc firstBad { return : Bad = bads#(1); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	for attempt := 1; attempt <= 2; attempt++ {
		for _, calc := range []string{"badCount", "firstBad"} {
			got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
			if !errors.Is(err, ErrDivisionByZero) || !strings.Contains(err.Error(), "bads") {
				t.Fatalf("attempt %d: %s = %s, %v; want %v naming bads", attempt, calc, FormatValue(got), err, ErrDivisionByZero)
			}
		}
		if _, ok := ctx.occurrences[resolveSymbol(t, pkg.Scope, "bads")]; ok {
			t.Fatalf("attempt %d: bads denotes objects after a refused construction", attempt)
		}
		if got := len(ctx.instances); got != 0 {
			t.Fatalf("attempt %d: a refused construction left %d objects standing", attempt, got)
		}
		if len(ctx.created) != 0 || len(ctx.objectBehaviors) != 0 {
			t.Fatalf("attempt %d: a refused construction left %d creations and %d behaviors", attempt, len(ctx.created), len(ctx.objectBehaviors))
		}
	}
	got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "2" {
		t.Errorf("size(all Wheel) = %s, %v; want 2: the bads hold no Wheel and are not read", FormatValue(got), err)
	}
}

// testChainedWriteThroughANamespaceCollection: a write chained from a namespace-level
// collection reaches several objects, so it is refused as a write through a nested
// collection is, and the objects the collection denotes are left as they were —
// no object is made or replaced for the write; a scalar usage beside it is written.
func testChainedWriteThroughANamespaceCollection(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		part def Sensor { attribute reading : Real = 0.0; }
		part sensors : Sensor[2];
		part probe : Sensor;
		action def Calibrate { action step { assign sensors.reading := 4.5; } first step; }
		action def Tune { action step { assign probe.reading := 4.5; } first step; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	sensors := resolveSymbol(t, pkg.Scope, "sensors")
	before, err := ctx.occurrencesOf(sensors)
	if err != nil || len(before) != 2 {
		t.Fatalf("sensors denote %d objects, %v; want 2", len(before), err)
	}
	_, err = ctx.ExecuteAction(resolveSymbol(t, pkg.Scope, "Calibrate"))
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "sensors") {
		t.Fatalf("assign sensors.reading: %v; want %v naming sensors", err, ErrTypeMismatch)
	}
	after, err := ctx.occurrencesOf(sensors)
	if err != nil || len(after) != 2 || after[0] != before[0] || after[1] != before[1] {
		t.Fatalf("sensors denote %v, %v after a refused write; want the same two objects", after, err)
	}
	if got := len(ctx.instances); got != 2 {
		t.Fatalf("a refused write left %d objects standing; want the 2 sensors", got)
	}
	for _, inst := range after {
		if val, err := inst.FeatureValues["reading"].ReadValue("reading"); err != nil || FormatValue(val) != "0.0" {
			t.Errorf("sensor #%d reading = %s, %v after a refused write; want 0.0", inst.ID, FormatValue(val), err)
		}
	}
	if _, err := ctx.ExecuteAction(resolveSymbol(t, pkg.Scope, "Tune")); err != nil {
		t.Fatalf("assign probe.reading: %v", err)
	}
	probe, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "probe"))
	if err != nil {
		t.Fatal(err)
	}
	if val, err := probe.FeatureValues["reading"].ReadValue("reading"); err != nil || FormatValue(val) != "4.5" {
		t.Errorf("probe reading = %s, %v; want 4.5", FormatValue(val), err)
	}
}

// testNamespaceCollectionOverBudget: a namespace-level usage whose extent takes more work than
// a run may do — past the element budget, or past the step budget — is refused with the typed
// limit naming the usage and leaves no partial extent, while its size and an extent it cannot
// contribute to are answered without making its occurrences.
func testNamespaceCollectionOverBudget(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Seat;
		part many : Wheel[10000];
		part seats : Seat[2];
		calc wheelCount { return : Natural = size(all Wheel); }
		calc seatCount { return : Natural = size(all Seat); }
		calc manyCount { return : Natural = size(many); }
	}
	package R {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part some : Wheel[5];
		calc wheelCount { return : Natural = size(all Wheel); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if !errors.Is(err, ErrStepLimitExceeded) || !strings.Contains(err.Error(), "many") {
		t.Fatalf("wheelCount = %s, %v; want %v naming many", FormatValue(got), err, ErrStepLimitExceeded)
	}
	if got := len(ctx.instances); got != 0 {
		t.Fatalf("a refused extent over 10000 left %d objects standing", got)
	}
	got, err = invokeValue(ctx, resolveSymbol(t, pkg.Scope, "manyCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "10000" {
		t.Fatalf("size(many) = %s, %v; want 10000", FormatValue(got), err)
	}
	if got := len(ctx.instances); got != 0 {
		t.Fatalf("the size of a collection of 10000 made %d of its objects", got)
	}
	got, err = ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "seatCount"), nil, pkg.Scope)
	got = soleElement(got)
	if err != nil || FormatValue(got) != "2" {
		t.Errorf("size(all Seat) = %s, %v; want 2: the wheels hold no Seat and are not read", FormatValue(got), err)
	}
	r := resolveSymbol(t, root, "R")
	ctx = NewContext(typedModel(model, resolver), 1000)
	ctx.maxElements = 4
	got, err = ctx.InvokeCalc(resolveSymbol(t, r.Scope, "wheelCount"), nil, r.Scope)
	if !errors.Is(err, ErrElementLimitExceeded) || !strings.Contains(err.Error(), "some") {
		t.Fatalf("R: size(all Wheel) = %s, %v; want %v naming some", FormatValue(got), err, ErrElementLimitExceeded)
	}
	if got := len(ctx.instances); got != 0 {
		t.Errorf("R: a collection past the element budget left %d objects standing", got)
	}
}

// testExtentOverAnObjectThatCannotBeRead: an extent materializes the nested usages it walks that
// may hold an object of its type, and only those, so a usage that cannot be materialized — five
// wheels under a budget of four elements — ends the extent with that usage's typed error, never
// an extent short of what stands behind it, and leaves the extent of another type alone.
func testExtentOverAnObjectThatCannotBeRead(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Seat;
		part def Driver;
		part def Car {
			part seats : Seat[2];
			part wheels : Wheel[5];
			part driver : Driver;
		}
		part car : Car;
		calc wheelCount { return : Natural = size(all Wheel); }
		calc seatCount { return : Natural = size(all Seat); }
		calc driverCount { return : Natural = size(all Driver); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	ctx.maxElements = 4
	got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	got = soleElement(got)
	if !errors.Is(err, ErrElementLimitExceeded) || !strings.Contains(err.Error(), "wheels") {
		t.Fatalf("size(all Wheel) = %s, %v; want %v naming wheels", FormatValue(got), err, ErrElementLimitExceeded)
	}
	for calc, want := range map[string]string{"seatCount": "2", "driverCount": "1"} {
		got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
		if err != nil || FormatValue(got) != want {
			t.Fatalf("%s = %s, %v; want %s: the wheels hold no such object and are not read", calc, FormatValue(got), err, want)
		}
	}
}

// testExtentReachingAFarUsageThatCannotBeRead: a namespace usage of another document that cannot
// be read ends the extent with a typed error naming it; a type it cannot hold is still answered,
// as is one a far document's collection holds.
func testExtentReachingAFarUsageThatCannotBeRead(t *testing.T) {
	ctx := contextOverDocs(t, [][2]string{
		{"model.sysml", `package P {
			private import ScalarValues::*;
			private import SequenceFunctions::size;
			part def Wheel;
			part def Seat;
			part def Driver;
			part def Car { part wheel : Wheel; }
			part local : Car;
			part driver : Driver;
			calc carCount { return : Natural = size(all Car); }
			calc wheelCount { return : Natural = size(all Wheel); }
			calc seatCount { return : Natural = size(all Seat); }
			calc driverCount { return : Natural = size(all Driver); }
		}`},
		{"far.sysml", `package Far {
			part def Boat;
			ref part car : P::Car = new Boat();
		}`},
		{"spares.sysml", `package Spares {
			part seats : P::Seat[2];
		}`},
	})
	pkg := lookupOne(t, ctx.Resolver().Index(), "P")
	for _, tc := range []struct {
		calc string
		want error
		name []string
	}{
		{"carCount", ErrTypeMismatch, []string{"usage car", "Far::car"}},
		{"wheelCount", ErrTypeMismatch, []string{"usage car", "Far::car"}},
	} {
		got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, tc.calc), nil, pkg.Scope)
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s = %s, %v; want %v", tc.calc, FormatValue(got), err, tc.want)
		}
		for _, want := range tc.name {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error = %v, want %q named", tc.calc, err, want)
			}
		}
	}
	for calc, want := range map[string]string{"driverCount": "1", "seatCount": "2"} {
		got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s = %s, %v; want %s: the far car is not read, the far seats collection is counted", calc, FormatValue(got), err, want)
		}
	}
}

// testExtentOverFarUsagesUnderTheElementBudget: far usages materialize in document order against
// the element budget, whose exhaustion names the usage it stopped at; a type they cannot hold reads none.
func testExtentOverFarUsagesUnderTheElementBudget(t *testing.T) {
	docs := [][2]string{
		{"fleet_a.sysml", `package FleetA { part first : P::Car; }`},
		{"fleet_b.sysml", `package FleetB { part second : P::Car; }`},
		{"model.sysml", `package P {
			private import ScalarValues::*;
			private import SequenceFunctions::size;
			part def Wheel;
			part def Driver;
			part def Car { part wheels : Wheel[2]; }
			part driver : Driver;
			calc wheelCount { return : Natural = size(all Wheel); }
			calc driverCount { return : Natural = size(all Driver); }
		}`},
	}
	ctx := contextOverDocs(t, docs)
	pkg := lookupOne(t, ctx.Resolver().Index(), "P")
	ctx.maxElements = 3
	got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if !errors.Is(err, ErrElementLimitExceeded) || !strings.Contains(err.Error(), "object of second: feature wheels") {
		t.Fatalf("size(all Wheel) = %s, %v; want %v naming the second document's usage and its wheels", FormatValue(got), err, ErrElementLimitExceeded)
	}
	cars := 0
	for _, id := range ctx.InstanceIDs() {
		if inst, _ := ctx.Instance(id); ctx.isOf(inst, lookupOne(t, ctx.Resolver().Index(), "P::Car")) {
			cars++
		}
	}
	if cars != 2 {
		t.Errorf("cars materialized before the budget ran out = %d, want both far usages read in document order", cars)
	}
	got, err = invokeValue(ctx, resolveSymbol(t, pkg.Scope, "driverCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "1" {
		t.Errorf("size(all Driver) = %s, %v; want 1: the cars hold no Driver and are not read", FormatValue(got), err)
	}

	ctx = contextOverDocs(t, docs)
	pkg = lookupOne(t, ctx.Resolver().Index(), "P")
	ctx.maxElements = 8
	got, err = invokeValue(ctx, resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "4" {
		t.Errorf("size(all Wheel) under a budget of nine = %s, %v; want 4: both cars' wheels, the extent holding them and the one-element result", FormatValue(got), err)
	}
}

// testExtentOverRecursiveComposition: an extent walked into a composition recursing through one
// declaration — a part of its own type, a constructor of it, two types holding each other, a value
// choosing one at run time — ends, leaving unread only the feature that would create another object
// of a declaration on the path; every object it does create has its own wheel read, so none is
// answered short of it.
func testExtentOverRecursiveComposition(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Tree { part left : Tree; part leaf : Wheel; }
		part def Chain { ref part tail : Chain = new Chain(); part leaf : Wheel; }
		part def Pair { part a : Half; part b : Half; }
		part def Half { part back : Pair; part hub : Wheel; }
		part def Fork {
			part tine = if false ? new Fork() else new Wheel();
			part stem = if true ? new Fork() else new Wheel();
		}
		part tree : Tree;
		part chain : Chain;
		part pair : Pair;
		part fork : Fork;
		calc wheelCount { return : Natural = size(all Wheel); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 100000)
	done := make(chan struct{})
	var got Value
	var err error
	go func() {
		defer close(done)
		got, err = ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	}()
	select {
	case <-done:
	case <-watchdog(20 * time.Second):
		t.Fatal("the extent did not terminate on recursive composition")
	}
	if err != nil {
		t.Fatalf("size(all Wheel): %v", err)
	}
	wheels := 0
	for _, inst := range ctx.instances {
		if inst.Type == nil {
			continue
		}
		switch inst.Type.Name {
		case "Wheel":
			wheels++
		case "tree", "Tree", "chain", "Chain", "Half", "fork", "Fork":
			for _, name := range []string{"leaf", "hub", "tine"} {
				if fv, has := inst.FeatureValues[name]; has && !fv.Materialized {
					t.Errorf("object %d of %s: %s left unread, the extent is short of it", inst.ID, inst.Type.Name, name)
				}
			}
			if fv, has := inst.FeatureValues["stem"]; has && fv.Materialized {
				t.Errorf("object %d of %s: stem read, though what it makes is another Fork", inst.ID, inst.Type.Name)
			}
		}
	}
	if FormatValue(soleElement(got)) != fmt.Sprint(wheels) || wheels < 5 {
		t.Errorf("size(all Wheel) = %s with %d wheels materialized; want every wheel of the objects there are, the tree's and chain's leaves, the pair's two hubs and the fork's tine at least", FormatValue(got), wheels)
	}
	if len(ctx.instances) > 20 {
		t.Errorf("%d objects materialized: the walk is not bounded by the declarations on its path", len(ctx.instances))
	}
}

// testExtentThroughAValueRecursingAndNot: a value that makes an object of a declaration on the path
// together with one the extent would reach can be kept neither whole (the walk would not end) nor
// in part, so the extent is refused with a typed error naming the usage, and the read is undone.
func testExtentThroughAValueRecursingAndNot(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		part def Wheel;
		part def Seat;
		part def Fork {
			part mixed[*] = (new Wheel(), new Fork());
			part leaf : Wheel;
			part seat : Seat;
		}
		part fork : Fork;
		calc wheelCount { return : Natural = size(all Wheel); }
		calc seatCount { return : Natural = size(all Seat); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 100000)
	_, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if !errors.Is(err, errExtentUnavailable) || !strings.Contains(err.Error(), "mixed") || !strings.Contains(err.Error(), "Fork") {
		t.Fatalf("size(all Wheel) = %v, want errExtentUnavailable naming mixed and Fork", err)
	}
	if fv := ctx.instances[1].FeatureValues["mixed"]; fv != nil && fv.Materialized {
		t.Error("mixed kept materialized after the refusal")
	}
	if n := len(ctx.instances); n != 1 {
		t.Errorf("%d objects after the refusal, want the fork alone: what mixed made is undone", n)
	}
	got, err := invokeValue(ctx, resolveSymbol(t, pkg.Scope, "seatCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "1" {
		t.Errorf("size(all Seat) = %s, %v; want 1: mixed holds no Seat and is left unread", FormatValue(got), err)
	}
}
