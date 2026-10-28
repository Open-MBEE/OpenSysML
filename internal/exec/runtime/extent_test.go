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

// TestNamespaceConnectorUsageDenotesOneObject requires a namespace-level connection,
// interface or allocation usage to denote one connector for the run, its ends attached to
// the features the `connect` clause names (SysML v2.1 Beta 2 §8.4.9.2: a ConnectionUsage is
// a PartUsage), reached by `all T` before any read of the usage, while a usage stating
// `[0..*]` denotes none, as a package-level part does.
func TestNamespaceConnectorUsageDenotesOneObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			connection def Link { end a; end b; }
			port def Pt;
			interface def Ifc { end port x : Pt; end port y : Pt; }
			allocation def Aloc { end part x; end part y; }
			part sys {
				part x; part y;
				port px : Pt; port py : Pt;
				connection inner : Link connect x to y;
			}
			connection outer : Link connect sys.x to sys.y;
			connection none : Link[0..*] connect sys.x to sys.y;
			interface wire : Ifc connect sys.px to sys.py;
			allocation alloc : Aloc allocate sys.x to sys.y;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	eval := func(expr string) Value {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return val
	}
	for expr, want := range map[string]int{"all Link": 2, "all Ifc": 1, "all Aloc": 1, "none": 0} {
		if got := len(elementsOf(eval(expr))); got != want {
			t.Errorf("size(%s) = %d, want %d", expr, got, want)
		}
	}
	outer := eval("outer")
	if outer.Kind != ValInstance {
		t.Fatalf("outer = %s, want one object", outer.Kind)
	}
	if again := eval("outer"); again.Instance != outer.Instance {
		t.Errorf("outer read twice denotes objects %d and %d, want the same one", outer.Instance, again.Instance)
	}
	if ids := ctx.occurrences[lookupOne(t, idx, "test::outer")]; len(ids) != 1 || ids[0] != outer.Instance {
		t.Errorf("occurrences[outer] = %v, want [%d]", ids, outer.Instance)
	}
	for _, expr := range []string{"outer.a === sys.x", "outer.b === sys.y", "wire.x === sys.px", "alloc.y === sys.y", "outer != sys.inner"} {
		if val := eval(expr); val.Kind != ValConst || !val.Const.Bool {
			t.Errorf("%s = %v, want true", expr, val)
		}
	}
	if ends := ctx.instances[outer.Instance].Ends; len(ends) != 2 {
		t.Errorf("outer holds %d ends, want 2", len(ends))
	}
}

// TestNamespacePerformanceUsageDenotesOneObject requires a valueless package-level action or
// state usage to denote one performance for the run, as a nested one inside a part does: reached
// by `all T` before any read of it, read twice as the same object, its type's exhibited behavior
// started and its own body not run; `[0..*]` denotes none and `[2]` two.
func TestNamespacePerformanceUsageDenotesOneObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			state def Inner;
			action def Act {
				attribute hits : Integer := 0;
				exhibit state m : Inner;
				first start;
				then assign hits := hits + 1;
				then done;
			}
			state def Sta { attribute n : Integer := 5; entry; then s0; state s0; }
			action a : Act;
			state s : Sta;
			action none : Act[0..*];
			action pair : Act[2];
			part sys { part x; action na : Act; state ns : Sta; }
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	eval := func(expr string) Value {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return val
	}
	for expr, want := range map[string]int{"all Act": 4, "all Sta": 2, "all Inner": 4, "none": 0, "pair": 2} {
		if got := len(elementsOf(eval(expr))); got != want {
			t.Errorf("size(%s) = %d, want %d", expr, got, want)
		}
	}
	a := eval("a")
	if a.Kind != ValInstance {
		t.Fatalf("a = %s, want one object", a.Kind)
	}
	if again := eval("a"); again.Instance != a.Instance {
		t.Errorf("a read twice denotes objects %d and %d, want the same one", a.Instance, again.Instance)
	}
	if ids := ctx.occurrences[lookupOne(t, idx, "test::a")]; len(ids) != 1 || ids[0] != a.Instance {
		t.Errorf("occurrences[a] = %v, want [%d]", ids, a.Instance)
	}
	for _, expr := range []string{"(all Act)#(1) === a", "(all Act)#(4) === sys.na", "a != sys.na", "a.hits == 0", "sys.na.hits == 0", "s.n == 5", "pair#(1) != pair#(2)"} {
		if val := eval(expr); val.Kind != ValConst || !val.Const.Bool {
			t.Errorf("%s = %v, want true", expr, val)
		}
	}
}

// TestNamespaceConnectorUsagesBoundAndCounted requires every namespace path that makes a connector
// usage's objects — a `bind`, a `[2]` collection, a lazily held `[1001]` one — to attach their ends.
func TestNamespaceConnectorUsagesBoundAndCounted(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			connection def Link { end a; end b; }
			part sys { part x; part y; }
			connection c : Link connect sys.x to sys.y;
			connection d : Link connect sys.x to sys.y;
			bind c = d;
			connection pair : Link[2] connect sys.x to sys.y;
			connection many : Link[1001] connect sys.x to sys.y;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	eval := func(expr string) Value {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return val
	}
	c := eval("c")
	if c.Kind != ValInstance {
		t.Fatalf("c = %s, want one object", c.Kind)
	}
	if ends := ctx.instances[c.Instance].Ends; len(ends) != 2 {
		t.Fatalf("c bound before it was read holds %d ends, want 2", len(ends))
	}
	for _, expr := range []string{"c === d", "c.a === sys.x", "d.b === sys.y"} {
		if val := eval(expr); val.Kind != ValConst || !val.Const.Bool {
			t.Errorf("%s = %v, want true", expr, val)
		}
	}
	pair := elementsOf(eval("pair"))
	if len(pair) != 2 {
		t.Fatalf("pair denotes %d objects, want 2", len(pair))
	}
	for i, obj := range pair {
		id, ok := obj.Object()
		if !ok {
			t.Fatalf("pair#%d = %v, want an object", i+1, obj)
		}
		ends := ctx.instances[id].Ends
		if len(ends) != 2 || !valueIdentical(ends[0].Value, eval("sys.x")) || !valueIdentical(ends[1].Value, eval("sys.y")) {
			t.Errorf("pair#%d holds ends %v, want sys.x and sys.y", i+1, ends)
		}
	}
	for _, expr := range []string{"many#(1).a === sys.x", "many#(1001).b === sys.y", "many#(1) != many#(2)"} {
		if val := eval(expr); val.Kind != ValConst || !val.Const.Bool {
			t.Errorf("%s = %v, want true", expr, val)
		}
	}
	if got := len(elementsOf(eval("all Link"))); got != 1004 {
		t.Errorf("size(all Link) = %d, want 1004 (the shared c/d, the two of pair, the 1001 of many)", got)
	}
	for id, inst := range ctx.instances {
		if inst.Type != nil && inst.Type.Name == "Link" && len(inst.Ends) != 2 {
			t.Fatalf("Link %d holds %d ends, want 2", id, len(inst.Ends))
		}
	}
}

// TestNamespaceConnectorBehaviorReadsItsOwnUsage requires a package-level connector to be what
// its usage denotes before its classifier behaviors start, so one of them reading the usage reaches it.
func TestNamespaceConnectorBehaviorReadsItsOwnUsage(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			connection def Link {
				end a; end b;
				attribute seen : Integer = 0;
				exhibit state sm {
					entry; then s1;
					state s1 { entry assign seen := size(c) + size(pair); }
				}
			}
			part sys { part x; part y; }
			connection c : Link connect sys.x to sys.y;
			connection pair : Link[2] connect sys.x to sys.y;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	eval := func(expr string) Value {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return val
	}
	c := eval("c")
	if c.Kind != ValInstance {
		t.Fatalf("c = %s, want one object", c.Kind)
	}
	if ids := ctx.occurrences[lookupOne(t, idx, "test::c")]; len(ids) != 1 || ids[0] != c.Instance {
		t.Errorf("occurrences[c] = %v, want [%d]", ids, c.Instance)
	}
	for expr, want := range map[string]int64{"c.seen": 3, "pair#(1).seen": 3, "pair#(2).seen": 3} {
		if val := eval(expr); val.Kind != ValConst || val.Const.Int != want {
			t.Errorf("%s = %v, want %d: the behavior read the usages as the objects made", expr, val, want)
		}
	}
	if got := len(elementsOf(eval("all Link"))); got != 3 {
		t.Errorf("size(all Link) = %d, want 3", got)
	}
}

// TestNamespaceConnectorWhoseStartFailsLeavesNoTrace requires a package-level connector whose own
// behavior writes to an older object before failing to be undone whole, as a nested one is.
func TestNamespaceConnectorWhoseStartFailsLeavesNoTrace(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + 1; } }
				}
			}
			part good : Listener;
			part def A { port p : P; }
			part a : A;
			connection def Faulty {
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
					transition first ping then fail;
					state fail { entry action bad { assign good.nosuch := 1; } }
				}
			}
			connection def Fine {
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
				}
			}
			connection faulty : Faulty connect a.p to a.p;
			connection fine : Fine connect a.p to a.p;
			connection pair : Faulty[2] connect a.p to a.p;
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(typedModel(model, resolver), 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	for _, name := range []string{"faulty", "pair"} {
		if _, err := evalIn(t, ctx, pkg.Scope, name); !errors.Is(err, ErrNoSuchFeature) {
			t.Fatalf("%s: got %v, want ErrNoSuchFeature from the connector's behavior", name, err)
		}
		if n := countInstancesOfType(ctx, "Faulty"); n != 0 {
			t.Errorf("%d Faulty left behind after %s's start failed, want none", n, name)
		}
		assertCurrentState(t, machineOf(t, good, "listening").State, "waiting")
		if got := featureInt(t, ctx, good, "heard"); got != 0 {
			t.Errorf("heard = %d after %s's start failed, want 0: its write was undone", got, name)
		}
		if got := len(ctx.messages); got != 0 {
			t.Errorf("%d message(s) left on the bus by %s", got, name)
		}
	}
	if _, err := evalIn(t, ctx, pkg.Scope, "fine"); err != nil {
		t.Fatalf("fine: %v", err)
	}
	assertCurrentState(t, machineOf(t, good, "listening").State, "noted")
	if got := featureInt(t, ctx, good, "heard"); got != 11 {
		t.Errorf("heard = %d once a connector's start succeeded, want 11: its write kept and its signal answered", got)
	}
}

// TestNamespaceConnectorAnsweredByAFailingBehaviorIsKept requires a package-level connector created
// whole to be kept when an older object's behavior fails answering it, the failure reported as its own.
func TestNamespaceConnectorAnsweredByAFailingBehaviorIsKept(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + nosuch; } }
				}
			}
			part good : Listener;
			part def A { port p : P; }
			part a : A;
			connection def Fine {
				attribute pokes : Integer = 0;
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; assign pokes := pokes + 1; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
				}
			}
			connection fine : Fine connect a.p to a.p;
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(typedModel(model, resolver), 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	sym := resolveSymbol(t, pkg.Scope, "fine")
	if _, err := evalIn(t, ctx, pkg.Scope, "fine"); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected the listener's behavior to fail on its unresolved reference, got: %v", err)
	}
	ids := ctx.occurrences[sym]
	if len(ids) != 1 {
		t.Fatalf("occurrences[fine] = %v, want the kept connector", ids)
	}
	fine, live := ctx.Instance(ids[0])
	if !live || len(fine.Ends) != 2 {
		t.Fatalf("the connector the listener failed answering is not held whole: live %v, %d end(s)", live, len(fine.Ends))
	}
	assertCurrentState(t, machineOf(t, fine, "life").State, "ping")
	if got := featureInt(t, ctx, good, "heard"); got != 10 {
		t.Errorf("heard = %d, want 10: the kept connector's write stays with the connector", got)
	}
	if got := featureInt(t, ctx, fine, "pokes"); got != 1 {
		t.Errorf("pokes = %d, want 1: the connector's own write is kept", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus: the listener took the signal before failing", got)
	}
	again, err := evalIn(t, ctx, pkg.Scope, "fine")
	if err != nil {
		t.Fatalf("reading the kept connector again: %v", err)
	}
	if again.Kind != ValInstance || again.Instance != fine.ID {
		t.Errorf("reading again denotes %v, want connector %d: the kept one", again, fine.ID)
	}
	if got := featureInt(t, ctx, fine, "pokes"); got != 1 {
		t.Errorf("pokes = %d after reading again, want 1: the kept connector is not rerun", got)
	}
}

// TestNamespaceSubsettedConnectorIsReadableByTheBehaviorsItWakes requires the older behaviors a
// connector filling a subsetted usage woke to answer only once the usage is recorded, so reading it is no cycle.
func TestNamespaceSubsettedConnectorIsReadableByTheBehaviorsItWakes(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			item def Ping;
			port def P;
			part def Listener {
				attribute seen : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign seen := size(links); } }
				}
			}
			part good : Listener;
			part def A { port p : P; }
			part a : A;
			connection def Link {
				end a; end b;
				attribute loud : Boolean = true;
				exhibit state life {
					entry; then check;
					state check;
					transition first check if loud then ping;
					state ping { entry send Ping() to good; }
				}
			}
			connection links : Link[2] connect a.p to a.p;
			connection c : Link :> links connect a.p to a.p { attribute :>> loud = false; }
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	good, err := ctx.occurrenceOf(lookupOne(t, idx, "test::good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	links, err := evalIn(t, ctx, pkg.Scope, "links")
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if got := len(elementsOf(links)); got != 2 {
		t.Fatalf("links denotes %d objects, want 2: c and the fill", got)
	}
	assertCurrentState(t, machineOf(t, good, "listening").State, "noted")
	if got := featureInt(t, ctx, good, "seen"); got != 2 {
		t.Errorf("seen = %d, want 2: the listener read links once it was recorded", got)
	}
	if again, err := evalIn(t, ctx, pkg.Scope, "links"); err != nil || len(elementsOf(again)) != 2 || !valueIdentical(again, links) {
		t.Errorf("links read again = %v, %v; want the same two objects", again, err)
	}
	for i, obj := range elementsOf(links) {
		id, _ := obj.Object()
		if inst, live := ctx.Instance(id); !live || len(inst.Ends) != 2 {
			t.Errorf("links#%d: live %v with %d end(s), want a live connector with 2", i+1, live, len(inst.Ends))
		}
	}
}
