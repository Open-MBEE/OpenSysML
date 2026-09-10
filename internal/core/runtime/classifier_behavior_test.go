package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// intArgument is an integer argument of an invocation.
func intArgument(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

// An object of a type exhibiting a machine runs that machine on materialization.
func TestInstantiateStartsExhibitedStateMachine(t *testing.T) {
	src := `
		part def Controller {
			attribute log: String;
			exhibit state modes {
				entry; then off;
				state off {
					entry action mark { assign log := "off"; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Controller"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	behavior, ok := inst.ExhibitedState()
	if !ok {
		t.Fatalf("object exhibits no state machine, behaviors: %v", inst.Behaviors())
	}
	if behavior.Name != "modes" {
		t.Errorf("behavior name = %q, want modes", behavior.Name)
	}
	if got := activeStateNames(behavior.State); got != "off" {
		t.Errorf("current state = %q, want off", got)
	}
}

// Two objects of one type exhibit two machines, with their own current states.
func TestExhibitedMachinesOfTwoObjectsAreIndependent(t *testing.T) {
	src := `
		part def Light {
			exhibit state modes {
				entry; then dark;
				state dark;
				state lit;
				transition dark_to_lit first dark accept Toggle then lit;
			}
		}
		attribute def Toggle;
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	sym := resolveSymbol(t, root, "Light")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate first: %v", err)
	}
	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate second: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("two objects share identity %d", first.ID)
	}

	firstMachine, _ := first.ExhibitedState()
	secondMachine, _ := second.ExhibitedState()
	if firstMachine.State == secondMachine.State {
		t.Fatal("two objects share one machine execution")
	}

	firstMachine.State.SendSignal("Toggle", nil)
	if err := firstMachine.State.RunToCompletion(); err != nil {
		t.Fatalf("run first machine: %v", err)
	}
	if got := activeStateNames(firstMachine.State); got != "lit" {
		t.Errorf("first current state = %q, want lit", got)
	}
	if got := activeStateNames(secondMachine.State); got != "dark" {
		t.Errorf("second current state = %q, want dark", got)
	}
}

// A machine's entry action writes the feature values of the object exhibiting it.
func TestExhibitedMachineWritesItsObjectsFeatureValues(t *testing.T) {
	src := `
		part def Counter {
			attribute count: Integer = 0;
			exhibit state modes {
				entry; then running;
				state running {
					entry action bump { assign count := count + 1; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if got := fv.HeldValue(); got.Const.Int != 1 {
		t.Errorf("count = %v, want 1", got.Const)
	}
}

// A machine-declared feature is written on the exhibited performance occurrence,
// while a like-named feature of the performer remains distinct.
func TestExhibitedMachineWritesItsOwnOccurrence(t *testing.T) {
	src := `
		state def Counting {
			attribute count: Integer = 0;
			entry; then running;
			state running {
				entry action bump { assign count := count + 1; }
			}
		}
		part def Counter {
			attribute count: Integer = 10;
			exhibit state modes : Counting;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := inst.Behavior("modes")
	if !ok {
		t.Fatal("object runs no modes behavior")
	}
	occurrence := instanceAtPath(t, ctx, inst, "modes")
	if behavior.State.occurrence != occurrence {
		t.Fatal("machine does not target the occurrence held by modes")
	}

	performerCount, err := inst.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("performer count: %v", err)
	}
	if got := performerCount.HeldValue().Const.Int; got != 10 {
		t.Errorf("performer count = %d, want 10", got)
	}
	occurrenceCount, err := occurrence.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("occurrence count: %v", err)
	}
	if got := occurrenceCount.HeldValue().Const.Int; got != 1 {
		t.Errorf("occurrence count = %d, want 1", got)
	}
	if got := behavior.State.stateData["count"].Const.Int; got != 1 {
		t.Errorf("state data count = %d, want 1", got)
	}
}

// A redefinition renames the behavior it redefines, so the object runs one
// behavior that answers to both names; a name of no behavior answers to none.
func TestBehaviorNamedFollowsRedefinition(t *testing.T) {
	src := `
		state def Modes {
			entry; then off;
			state off;
		}
		part def Lamp {
			exhibit state modes : Modes;
			perform action tick { action step; }
			action blink;
		}
		part def FancyLamp :> Lamp {
			exhibit state fancyModes :>> modes;
			perform action fancyTick :>> tick { action step; }
			action fancyBlink :>> blink;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "FancyLamp"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, names := range [][2]string{{"fancyModes", "modes"}, {"fancyTick", "tick"}} {
		renamed, ok := inst.Behavior(names[0])
		if !ok {
			t.Fatalf("object runs no %s behavior: %v", names[0], inst.Behaviors())
		}
		if _, ok := inst.Behavior(names[1]); ok {
			t.Errorf("Behavior(%q) matched by name alone", names[1])
		}
		for _, name := range names {
			got, ok := ctx.BehaviorNamed(inst, name)
			if !ok || got != renamed {
				t.Errorf("BehaviorNamed(%q) = %v, %v; want the %s behavior", name, got, ok, names[0])
			}
		}
	}
	for _, name := range []string{"blink", "fancyBlink", "nothing", ""} {
		if got, ok := ctx.BehaviorNamed(inst, name); ok {
			t.Errorf("BehaviorNamed(%q) = %v; want none", name, got)
		}
	}
}

// An object classified by a type renaming a behavior it already runs, or by one it
// renames a behavior of, keeps running that one behavior, and answers to both names
// whichever of its types declared the name it is asked by.
func TestBehaviorNamedFollowsRedefinitionByAClassifier(t *testing.T) {
	cases := map[string]struct {
		room  string
		held  string
		types []string
	}{
		"narrowed": {
			room:  `part lamp : Lamp [1]; part fancy : FancyLamp [1] = lamp;`,
			held:  "fancy",
			types: []string{"FancyLamp"},
		},
		"widened": {
			room:  `part lamp : FancyLamp [1]; part plain : Lamp [1] = lamp;`,
			held:  "plain",
			types: []string{"FancyLamp", "Lamp"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				state def Modes {
					entry; then off;
					state off;
				}
				part def Lamp {
					exhibit state modes : Modes;
					perform action tick { action step; }
				}
				part def FancyLamp :> Lamp {
					exhibit state fancyModes :>> modes;
					perform action fancyTick :>> tick { action step; }
				}
				part def Room { ` + tc.room + ` }
				part room : Room;
			`
			model, resolver, root := parseAndBuildModel(t, src)
			ctx := NewContext(model, resolver, 10000)

			room, err := ctx.Instantiate(resolveSymbol(t, root, "room"))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			lamp := readInstance(t, ctx, room, "lamp")
			if held := readInstance(t, ctx, room, tc.held); held != lamp {
				t.Fatalf("%s holds object %d, want lamp (%d)", tc.held, held.ID, lamp.ID)
			}
			for _, typ := range tc.types {
				if !ctx.instanceConforms(lamp, resolveSymbol(t, root, typ)) {
					t.Fatalf("lamp is of %v (classified by %v), want a %s", lamp.Type, lamp.classifiers, typ)
				}
			}
			if got := len(lamp.Behaviors()); got != 2 {
				t.Fatalf("lamp runs %d behaviors %v, want its one machine and one action", got, lamp.Behaviors())
			}
			for _, names := range [][2]string{{"modes", "fancyModes"}, {"tick", "fancyTick"}} {
				var found []*ObjectBehavior
				for _, name := range names {
					got, ok := ctx.BehaviorNamed(lamp, name)
					if !ok {
						t.Errorf("BehaviorNamed(%q) finds no behavior of the lamp", name)
						continue
					}
					found = append(found, got)
				}
				if len(found) == 2 && found[0] != found[1] {
					t.Errorf("BehaviorNamed(%q) and BehaviorNamed(%q) are two executions, want one", names[0], names[1])
				}
			}
			for _, name := range []string{"nothing", ""} {
				if got, ok := ctx.BehaviorNamed(lamp, name); ok {
					t.Errorf("BehaviorNamed(%q) = %v; want none", name, got)
				}
			}
		})
	}
}

// An action-declared feature is written on the action performance occurrence,
// while a like-named feature of the performer remains distinct. The results the
// run reports mirror the occurrence, which is authoritative.
func TestPerformedActionWritesItsOwnOccurrence(t *testing.T) {
	src := `
		action def Bump {
			attribute count: Integer = 0;
			out attribute doubled: Integer;
			action step {
				assign count := count + 1;
				assign doubled := count * 2;
			}
			first step;
		}
		part def Counter {
			attribute count: Integer = 10;
			perform action work : Bump;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := inst.Behavior("work")
	if !ok {
		t.Fatal("object runs no work behavior")
	}
	occurrence := instanceAtPath(t, ctx, inst, "work")
	if behavior.Action.occurrence != occurrence {
		t.Fatal("action does not target the occurrence held by work")
	}

	performerCount, err := inst.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("performer count: %v", err)
	}
	if got := performerCount.HeldValue().Const.Int; got != 10 {
		t.Errorf("performer count = %d, want 10", got)
	}
	for name, want := range map[string]int64{"count": 1, "doubled": 2} {
		fv, err := occurrence.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("occurrence %s: %v", name, err)
		}
		if got := fv.HeldValue().Const.Int; got != want {
			t.Errorf("occurrence %s = %d, want %d", name, got, want)
		}
		if got := behavior.Action.Results()[name].Const.Int; got != want {
			t.Errorf("reported result %s = %d, want %d", name, got, want)
		}
	}
}

// A performed action with no occurrence keeps its features in executor-local
// data, which is what a directly executed action has.
func TestDirectlyExecutedActionKeepsItsFeaturesLocal(t *testing.T) {
	src := `
		action def Bump {
			attribute count: Integer = 0;
			action step { assign count := count + 1; }
			first step;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	exec, err := newActionExecutor(ctx, resolveSymbol(t, root, "Bump"), nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	if exec.occurrence != nil {
		t.Error("a directly executed action materialized a performance occurrence")
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.Results()["count"].Const.Int; got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

// A machine's state data has the collection shape its occurrence gives a
// many-valued attribute written with one element.
func TestExhibitedMachineNormalizesItsManyValuedAttribute(t *testing.T) {
	src := `
		state def Logging {
			attribute entries: String[*];
			entry; then running;
			state running {
				entry action note { assign entries := "first"; }
			}
		}
		part def Log {
			exhibit state modes : Logging;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Log"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := inst.Behavior("modes")
	if !ok {
		t.Fatal("object runs no modes behavior")
	}
	occurrence := instanceAtPath(t, ctx, inst, "modes")
	fv, err := occurrence.GetFeatureValue(ctx, "entries")
	if err != nil {
		t.Fatalf("occurrence entries: %v", err)
	}
	assertSingleStringCollection(t, "occurrence entries", fv.HeldValue(), "first")
	assertSingleStringCollection(t, "state data entries", behavior.State.stateData["entries"], "first")
}

func assertSingleStringCollection(t *testing.T, name string, value Value, want string) {
	t.Helper()
	if value.Kind != ValSequence && value.Kind != ValSet {
		t.Fatalf("%s holds %v, want a collection", name, value.Kind)
	}
	elements := elementsOf(value)
	if len(elements) != 1 || elements[0].Str() != want {
		t.Errorf("%s = %v, want one element %q", name, elements, want)
	}
}

// invokeFixture owns an operation, a machine, calcs, constraints and an
// attribute, so one object exercises each classifier behavior path.
const invokeFixture = `
	part def Tank {
		attribute level: Integer = 2;
		action fillBy { in n; out filled; first apply; action apply { assign level := level + n; assign filled := level; } }
		exhibit state modes { entry; then holding; state holding; }
		calc capacity { in bonus : Integer; return total : Integer = level + bonus; }
		calc rawCapacity { return : Integer = level + 1; }
		constraint acceptable { in minimum : Integer; level >= minimum }
		constraint rejected { level > 10 }
	}
`

// An operation of the object's type runs with that object as performer: it reads
// and writes that object's feature values and answers its declared outputs.
func TestInvokeOperationPerformedByTheObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, invokeFixture)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Tank"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	results, err := ctx.InvokeOperation(inst, "fillBy", map[string]Value{"n": intArgument(3)})
	if err != nil {
		t.Fatalf("InvokeOperation: %v", err)
	}
	if got, ok := results["filled"]; !ok || got.Const.Int != 5 {
		t.Errorf("filled = %v, want 5", results)
	}
	fv, err := inst.GetFeatureValue(ctx, "level")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if got := fv.HeldValue(); got.Const.Int != 5 {
		t.Errorf("level = %v, want 5", got.Const)
	}

	results, err = ctx.InvokeOperation(inst, "capacity", map[string]Value{"bonus": intArgument(3)})
	if err != nil {
		t.Fatalf("InvokeOperation(calc): %v", err)
	}
	if got, ok := results["total"]; !ok || got.Const.Int != 8 {
		t.Errorf("calc result = %v, want total = 8", results)
	}
	results, err = ctx.InvokeOperation(inst, "rawCapacity", nil)
	if err != nil {
		t.Fatalf("InvokeOperation(anonymous calc): %v", err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 6 {
		t.Errorf("anonymous calc result = %v, want result = 6", results)
	}

	for _, tc := range []struct {
		name string
		args map[string]Value
		want bool
	}{
		{"acceptable", map[string]Value{"minimum": intArgument(3)}, true},
		{"acceptable", map[string]Value{"minimum": intArgument(6)}, false},
		{"rejected", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, err := ctx.InvokeOperation(inst, tc.name, tc.args)
			if err != nil {
				t.Fatalf("InvokeOperation(constraint): %v", err)
			}
			got, ok := results["result"]
			if !ok || got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
				t.Fatalf("constraint result = %v, want Boolean result", results)
			}
			if got.Const.Bool != tc.want {
				t.Errorf("constraint result = %v, want %v", got.Const.Bool, tc.want)
			}
		})
	}
}

// An operation invocation counts its own calc or constraint cost against the
// step budget, while separate invocations receive separate budgets.
func TestInvokeOperationCountsItsOwnCostAgainstBudget(t *testing.T) {
	newInvocation := func(maxSteps int64) (*Context, *Instance) {
		model, resolver, root := parseAndBuildModel(t, invokeFixture)
		ctx := NewContext(model, resolver, maxSteps)
		inst, err := ctx.Instantiate(resolveSymbol(t, root, "Tank"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		return ctx, inst
	}

	t.Run("calc", func(t *testing.T) {
		ctx, inst := newInvocation(2)
		results, err := ctx.InvokeOperation(inst, "rawCapacity", nil)
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("InvokeOperation(rawCapacity) with budget 2: %v, want ErrStepLimitExceeded", err)
		}
		if results != nil {
			t.Fatalf("InvokeOperation(rawCapacity) with budget 2 = %v, want no results", results)
		}

		ctx, inst = newInvocation(3)
		for i := 0; i < 2; i++ {
			results, err := ctx.InvokeOperation(inst, "rawCapacity", nil)
			if err != nil {
				t.Fatalf("InvokeOperation(rawCapacity), call %d with budget 3: %v", i, err)
			}
			if got, ok := results["result"]; !ok || got.Const.Int != 3 {
				t.Fatalf("rawCapacity result on call %d = %v, want 3", i, results)
			}
		}
	})

	t.Run("constraint", func(t *testing.T) {
		ctx, inst := newInvocation(2)
		results, err := ctx.InvokeOperation(inst, "acceptable", map[string]Value{
			"minimum": intArgument(1),
		})
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("InvokeOperation(acceptable) with budget 2: %v, want ErrStepLimitExceeded", err)
		}
		if results != nil {
			t.Fatalf("InvokeOperation(acceptable) with budget 2 = %v, want no results", results)
		}

		ctx, inst = newInvocation(3)
		for i := 0; i < 2; i++ {
			results, err := ctx.InvokeOperation(inst, "acceptable", map[string]Value{
				"minimum": intArgument(1),
			})
			if err != nil {
				t.Fatalf("InvokeOperation(acceptable), call %d with budget 3: %v", i, err)
			}
			got, ok := results["result"]
			if !ok || got.Kind != ValConst || got.Const.Kind != semantics.ValBool || !got.Const.Bool {
				t.Fatalf("acceptable result on call %d = %v, want true", i, results)
			}
		}
	})
}

const nestedCalcOperationFixture = `
	package test {
		private import ScalarValues::*;
		part def Tank {
			attribute level : Integer = 2;
			calc def ReadsLevel {
				attribute observed : Integer = 0;
				assign observed := level;
				return value : Integer = observed;
			}
			calc reading : ReadsLevel;
			calc capacityViaUsage {
				return result : Integer = reading.value;
			}
		}
	}
`

// A calc operation preserves its performing object through a nested calc usage.
func TestCalcOperationNestedUsageSeesPerformingObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedCalcOperationFixture))
	root := idx.DocumentRoot("<test>")
	tank := findSymbolByName(root, "Tank", ast.DefPart)
	if tank == nil {
		t.Fatal("Tank not found")
	}
	inst, err := ctx.Instantiate(tank)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := inst.SetFeatureValue(ctx, "level", intArgument(7)); err != nil {
		t.Fatalf("SetFeatureValue: %v", err)
	}

	results, err := ctx.InvokeOperation(inst, "capacityViaUsage", nil)
	if err != nil {
		t.Fatalf("InvokeOperation: %v", err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 7 {
		t.Errorf("nested calc result = %v, want result = 7", results)
	}
}

// An object-scoped calc usage keeps the materialized object's feature context.
func TestObjectScopedCalcUsageSeesPerformingObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedCalcOperationFixture))
	root := idx.DocumentRoot("<test>")
	tank := findSymbolByName(root, "Tank", ast.DefPart)
	if tank == nil {
		t.Fatal("Tank not found")
	}
	inst, err := ctx.Instantiate(tank)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := inst.SetFeatureValue(ctx, "level", intArgument(7)); err != nil {
		t.Fatalf("SetFeatureValue: %v", err)
	}
	matches := idx.LookupQualified("test::Tank::reading")
	if len(matches) != 1 {
		t.Fatalf("test::Tank::reading: %d matching symbols, want 1", len(matches))
	}

	outputs, err := ctx.CalcUsageOutputs(matches[0], matches[0].OwnerScope, inst)
	if err != nil {
		t.Fatalf("CalcUsageOutputs: %v", err)
	}
	wantInt(t, outputs, "value", 7)
}

const nestedCalcInvocationFixture = `
	package test {
		private import ScalarValues::*;
		part def Robot {
			attribute charge : Integer = 10;
			calc direct { return : Integer = charge + 100; }
			calc anon { return : Integer = charge * 2; }
			calc nested { return : Integer = anon() + 1; }
			calc usesDirect { return : Integer = direct() + 1000; }
			action drain {
				first start;
				action cut { assign charge := 3; }
				done;
				succession first start then cut;
				succession first cut then done;
			}
		}
	}
`

// A calc invocation expression preserves its enclosing calc's performing object.
func TestCalcInvocationExpressionSeesPerformingObject(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedCalcInvocationFixture))
	root := idx.DocumentRoot("<test>")
	robot := findSymbolByName(root, "Robot", ast.DefPart)
	if robot == nil {
		t.Fatal("Robot not found")
	}
	inst, err := ctx.Instantiate(robot)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := ctx.InvokeOperation(inst, "drain", nil); err != nil {
		t.Fatalf("InvokeOperation(drain): %v", err)
	}
	charge, err := inst.GetFeatureValue(ctx, "charge")
	if err != nil {
		t.Fatalf("GetFeatureValue(charge): %v", err)
	}
	if got := charge.HeldValue().Const.Int; got != 3 {
		t.Fatalf("charge = %d, want 3", got)
	}

	for _, tc := range []struct {
		name string
		want int64
	}{
		{"direct", 103},
		{"anon", 6},
		{"nested", 7},
		{"usesDirect", 1103},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, err := ctx.InvokeOperation(inst, tc.name, nil)
			if err != nil {
				t.Fatalf("InvokeOperation(%s): %v", tc.name, err)
			}
			got, ok := results["result"]
			if !ok || got.Const.Int != tc.want {
				t.Errorf("%s result = %v, want result = %d", tc.name, results, tc.want)
			}
		})
	}
}

// Every path %invoke cannot run reports a typed error naming what it was asked.
func TestInvokeOperationFailureModes(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, invokeFixture)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Tank"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	cases := []struct {
		name      string
		object    *Instance
		operation string
		args      map[string]Value
		want      error
	}{
		{"no object", nil, "fillBy", nil, ErrNoSuchBehavior},
		{"unknown operation", inst, "drain", nil, ErrNoSuchBehavior},
		{"state machine", inst, "modes", nil, ErrUnsupportedClassifierBehavior},
		{"attribute", inst, "level", nil, ErrNotABehavior},
		{"unbound parameter", inst, "fillBy", nil, ErrUnboundParameter},
		{"argument naming no parameter", inst, "fillBy", map[string]Value{"n": intArgument(1), "m": intArgument(2)}, ErrUnboundParameter},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ctx.InvokeOperation(tc.object, tc.operation, tc.args)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// An action stating no flow performs no step, so an object of a type performing
// one is still created, with that behavior of its own and nothing to run.
func TestPerformedActionWithoutAFlowStillMaterializes(t *testing.T) {
	src := `
		action def Report;
		part def Camera {
			perform action report : Report;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Camera"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := inst.Behavior("report")
	if !ok {
		t.Fatalf("object performs no report action, behaviors: %v", inst.Behaviors())
	}
	if behavior.Action == nil {
		t.Error("performed action has no execution")
	}
}

// The value an `out` member of the binding declares is the answer's default,
// not an input, so it neither fails the start nor is lost, with or without a flow.
func TestPerformedActionDeclaringAnOutputDefaultStarts(t *testing.T) {
	for name, body := range map[string]string{
		"flowed":  "out total : Integer = 7; first start; then done;",
		"no_flow": "out total : Integer = 7; action step;",
	} {
		src := `
			private import ScalarValues::*;
			part def Counter { perform action tick { ` + body + ` } }
		`
		model, resolver, root := parseAndBuildLibraryModel(t, src)
		ctx := NewContext(model, resolver, 10000)
		inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
		if err != nil {
			t.Errorf("%s: Instantiate: %v", name, err)
			continue
		}
		if got := performedFeature(t, ctx, inst, "tick", "total"); !valueIdentical(got, constInt(7)) {
			t.Errorf("%s: tick.total = %s; want 7", name, FormatValue(got))
		}
	}
}

// An `out` member on a typed binding redefines the answer's default; it does
// not replace the body, so the referenced action's flow still runs.
func TestPerformedActionOutputDefaultKeepsTheReferencedFlow(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `
		private import ScalarValues::*;
		action def Report {
			out total : Integer;
			attribute steps : Integer = 0;
			action step { assign steps := steps + 1; }
			first step;
		}
		part def Counter { perform action report : Report { out total = 7; } }
	`)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := performedFeature(t, ctx, inst, "report", "total"); !valueIdentical(got, constInt(7)) {
		t.Errorf("report.total = %s; want the binding's default 7", FormatValue(got))
	}
	if got := performedFeature(t, ctx, inst, "report", "steps"); !valueIdentical(got, constInt(1)) {
		t.Errorf("report.steps = %s; want 1, the referenced flow's one step", FormatValue(got))
	}
}

// performedFeature reads a feature of the occurrence of the action an object performs.
func performedFeature(t *testing.T, ctx *Context, inst *Instance, action, feature string) Value {
	t.Helper()
	behavior, ok := inst.Behavior(action)
	if !ok || behavior.Action == nil {
		t.Fatalf("object performs no %s action, behaviors: %v", action, inst.Behaviors())
	}
	fv, err := behavior.Action.occurrence.GetFeatureValue(ctx, feature)
	if err != nil {
		t.Fatalf("%s.%s: %v", action, feature, err)
	}
	return fv.HeldValue()
}

// A single value written to a many-valued feature is that collection's one
// element, the shape materialization gives such a feature.
func TestWritingOneValueToAManyValuedFeatureHoldsACollection(t *testing.T) {
	src := `
		part def Log {
			attribute entries: String[*];
			exhibit state modes {
				entry; then open;
				state open {
					entry action note { assign entries := "first"; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Log"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "entries")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	held := fv.HeldValue()
	if held.Kind != ValSequence && held.Kind != ValSet {
		t.Fatalf("entries holds %v, want a collection", held.Kind)
	}
	if got := elementsOf(held); len(got) != 1 || got[0].Str() != "first" {
		t.Errorf("entries = %v, want one element \"first\"", got)
	}
}

// A type that exhibits a machine no element states is reported, not ignored.
func TestExhibitedMachineNamingNoBodyIsReported(t *testing.T) {
	src := `
		part def Controller {
			exhibit state modes;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "Controller"))
	if err == nil {
		t.Fatal("expected an error for a machine naming no body")
	}
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
	if !strings.Contains(err.Error(), "modes") {
		t.Errorf("error %q does not name the behavior", err)
	}
}

// A type exhibits a machine through the declaration stating it inline, one typed
// by a definition, or one naming a state usage declared beside it, and every
// binding on the way to the body addresses it; a machine it merely performs, and
// a definition no declaration names, it does not exhibit.
func TestExhibitsStateResolvesTheBodyABindingRuns(t *testing.T) {
	src := `
		state def Blink { entry; then dark; state dark; }
		state def Check { entry; then checking; state checking; }
		part def Lamp {
			exhibit state front : Blink;
			exhibit state own { entry; then idle; state idle; }
			state spare : Blink;
			exhibit spare;
			state standby : Check;
			exhibit state night ::> standby;
			perform action watch { first start; then done; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	lamp := resolveSymbol(t, root, "Lamp")
	member := func(name string) *symbols.Symbol {
		sym, ok := lamp.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("Lamp declares no %s", name)
		}
		return sym
	}
	exhibitSpare := lamp.Scope.LookupLocalAll("spare")[1]

	for _, tc := range []struct {
		member, machine string
		memberSym       *symbols.Symbol
		machineSym      *symbols.Symbol
		want            bool
	}{
		{"front", "Blink", member("front"), resolveSymbol(t, root, "Blink"), true},
		{"front", "front", member("front"), member("front"), true},
		{"own", "own", member("own"), member("own"), true},
		{"exhibit spare", "spare", exhibitSpare, member("spare"), true},
		{"exhibit spare", "Blink", exhibitSpare, resolveSymbol(t, root, "Blink"), true},
		{"night", "standby", member("night"), member("standby"), true},
		{"night", "Check", member("night"), resolveSymbol(t, root, "Check"), true},
		{"night", "Blink", member("night"), resolveSymbol(t, root, "Blink"), false},
		{"front", "Check", member("front"), resolveSymbol(t, root, "Check"), false},
		{"watch", "watch", member("watch"), member("watch"), false},
		{"spare", "spare", member("spare"), member("spare"), false},
	} {
		if got := ctx.ExhibitsState(tc.memberSym, tc.machineSym); got != tc.want {
			t.Errorf("ExhibitsState(%s, %s) = %v, want %v", tc.member, tc.machine, got, tc.want)
		}
	}
	if ctx.ExhibitsState(nil, lamp) || ctx.ExhibitsState(lamp, nil) {
		t.Error("ExhibitsState over a nil symbol reported an exhibit")
	}

	// An object of the type addresses its machines the same way.
	inst, err := ctx.Instantiate(lamp)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := inst.ExhibitedStatesOf(member("spare")); len(got) != 1 || got[0].Member() != exhibitSpare {
		t.Errorf("ExhibitedStatesOf(spare) = %v, want the machine `exhibit spare` binds", got)
	}
	if got := inst.ExhibitedStatesOf(resolveSymbol(t, root, "Blink")); len(got) != 2 {
		t.Errorf("ExhibitedStatesOf(Blink) = %v, want front and spare", got)
	}
	if got := inst.ExhibitedStatesOf(member("standby")); len(got) != 1 || got[0].Member() != member("night") {
		t.Errorf("ExhibitedStatesOf(standby) = %v, want night", got)
	}
	if got := inst.ExhibitedStatesOf(member("watch")); len(got) != 0 {
		t.Errorf("ExhibitedStatesOf(watch) = %v, want none", got)
	}
}

// An exhibited usage stating its own body under a definition's type is still a
// machine of that definition, and of the definitions it specializes, so the
// definition addresses it — while an unrelated definition does not.
func TestExhibitsStateThroughTheTypeOfABodyStatingUsage(t *testing.T) {
	src := `
		state def Base { entry; then idle; state idle; }
		state def Blink :> Base { entry; then dark; state dark; }
		state def Check { entry; then checking; state checking; }
		part def Lamp {
			exhibit state tuned : Blink { state bright; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	lamp := resolveSymbol(t, root, "Lamp")
	tuned, ok := lamp.Scope.LookupLocal("tuned")
	if !ok {
		t.Fatal("Lamp declares no tuned")
	}
	def := func(name string) *symbols.Symbol { return resolveSymbol(t, root, name) }

	for _, tc := range []struct {
		machine string
		want    bool
	}{{"Blink", true}, {"Base", true}, {"Check", false}} {
		if got := ctx.ExhibitsState(tuned, def(tc.machine)); got != tc.want {
			t.Errorf("ExhibitsState(tuned, %s) = %v, want %v", tc.machine, got, tc.want)
		}
	}

	inst, err := ctx.Instantiate(lamp)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, machine := range []string{"Blink", "Base"} {
		if got := inst.ExhibitedStatesOf(def(machine)); len(got) != 1 || got[0].Member() != tuned {
			t.Errorf("ExhibitedStatesOf(%s) = %v, want tuned", machine, got)
		}
	}
	if got := inst.ExhibitedStatesOf(def("Check")); len(got) != 0 {
		t.Errorf("ExhibitedStatesOf(Check) = %v, want none", got)
	}
}

// featureInt is the integer an object's feature value holds.
func featureInt(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue %s: %v", name, err)
	}
	return fv.HeldValue().Const.Int
}

// An operation output the object's type also declares a feature for answers the
// caller: an action's own parameter is not the performing object's feature.
func TestOperationOutputNamedLikeAFeatureAnswersTheCaller(t *testing.T) {
	src := `
		part def Gauge {
			attribute level: Integer = 2;
			action read { out level; first apply; action apply { assign level := 7; } }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Gauge"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	results, err := ctx.InvokeOperation(inst, "read", nil)
	if err != nil {
		t.Fatalf("InvokeOperation: %v", err)
	}
	if got, ok := results["level"]; !ok || got.Const.Int != 7 {
		t.Errorf("output level = %v, want 7", results)
	}
	fv, err := inst.GetFeatureValue(ctx, "level")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if got := fv.HeldValue(); got.Const.Int != 2 {
		t.Errorf("object level = %v, want the feature untouched at 2", got.Const)
	}
}

// An object performing an action that awaits a message is materialized waiting
// rather than deadlocked, and the message a sibling sends wakes it.
func TestPerformedActionAwaitingAMessageIsWokenByASibling(t *testing.T) {
	src := `
		package test {
			part def Waiter {
				attribute woken: Integer = 0;
				perform action await {
					first start;
					action heard accept g : Integer;
					action mark { assign woken := 1; }
					done;
					succession first start then heard;
					succession first heard then mark;
					succession first mark then done;
				}
			}

			part def Sender {
				exhibit state sending {
					entry; then sent;
					state sent { entry send 5 to w; }
				}
			}

			part def Pair {
				part w : Waiter;
				part s : Sender;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")

	// Materialized alone, the waiter parks at the accept: nothing has sent it a
	// message yet, which is quiescence for an object rather than a deadlock.
	alone := NewContext(model, resolver, 10000)
	waiter, err := alone.Instantiate(resolveSymbol(t, pkg.Scope, "Waiter"))
	if err != nil {
		t.Fatalf("Instantiate Waiter: %v", err)
	}
	behavior, ok := waiter.Behavior("await")
	if !ok || behavior.Action == nil {
		t.Fatalf("object performs no await action, behaviors: %v", waiter.Behaviors())
	}
	if behavior.Action.state != StateWaiting {
		t.Errorf("await is %v, want waiting at its accept", behavior.Action.state)
	}
	if got := featureInt(t, alone, waiter, "woken"); got != 0 {
		t.Errorf("woken = %d before any message, want 0", got)
	}

	// Materialized beside a sender, the message wakes the parked action, which
	// then writes its own object's feature value.
	ctx := NewContext(model, resolver, 10000)
	pair, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Pair"))
	if err != nil {
		t.Fatalf("Instantiate Pair: %v", err)
	}
	// Each nested object is materialized when its feature value is reached, so the
	// waiter parks before the sender it wakes is materialized at all.
	nested := instanceAtPath(t, ctx, pair, "w")
	instanceAtPath(t, ctx, pair, "s")
	if got := featureInt(t, ctx, nested, "woken"); got != 1 {
		t.Errorf("woken = %d, want 1 once the sibling's message arrived", got)
	}
}

// A failed materialization leaves no behavior of the object behind, so the next
// object materialized runs its own behaviors and nothing else.
func TestFailedMaterializationLeavesNoBehaviorBehind(t *testing.T) {
	src := `
		part def Broken {
			attribute n: Integer = 0;
			exhibit state modes { entry; then on; state on { entry action bump { assign n := 1; } } }
			exhibit state missing;
		}
		part def Fine {
			exhibit state modes { entry; then idle; state idle; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	broken, err := ctx.Instantiate(resolveSymbol(t, root, "Broken"))
	if err == nil {
		t.Fatal("expected the unresolved machine to fail materialization")
	}
	if broken != nil {
		t.Errorf("failed materialization answered object %v", broken)
	}
	if got := len(ctx.objectBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left attached after a failed materialization", got)
	}
	if got := len(ctx.pendingBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left queued after a failed materialization", got)
	}

	fine, err := ctx.Instantiate(resolveSymbol(t, root, "Fine"))
	if err != nil {
		t.Fatalf("Instantiate after a failed one: %v", err)
	}
	if got := len(ctx.objectBehaviors); got != 1 {
		t.Errorf("%d behaviors attached, want only the new object's", got)
	}
	if _, ok := fine.ExhibitedState(); !ok {
		t.Error("the new object exhibits no machine")
	}
}

// An object exhibiting a machine typed by the library's StateAction, with its own
// body, materializes, reads its attributes and runs that body, however named.
func TestObjectExhibitsAMachineTypedByTheLibraryStateAction(t *testing.T) {
	for _, tc := range []struct{ name, typing string }{
		{"imported", "StateAction"},
		{"qualified", "States::StateAction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `
			package test {
				private import States::StateAction;
				part def Mission {
					attribute mass: Integer = 7;
					attribute launched: Integer = 0;
					exhibit state phases : ` + tc.typing + ` {
						entry; then launch;
						state launch { entry action mark { assign launched := 1; } }
					}
				}
			}`
			model, resolver, root := parseAndBuildLibraryModel(t, src)
			pkg := resolveSymbol(t, root, "test")
			ctx := NewContext(model, resolver, 10000)

			inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Mission"))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			if got := featureInt(t, ctx, inst, "mass"); got != 7 {
				t.Errorf("mass = %d, want 7", got)
			}
			if got := featureInt(t, ctx, inst, "launched"); got != 1 {
				t.Errorf("launched = %d, want 1 once the machine entered launch", got)
			}
			machine, ok := inst.ExhibitedState()
			if !ok || machine.State == nil {
				t.Fatal("the object exhibits no machine")
			}
			if machine.Name != "phases" {
				t.Errorf("machine name = %q, want phases", machine.Name)
			}
			if got := machine.State.FinalStateName(); got != "launch" {
				t.Errorf("final state = %q, want launch", got)
			}
		})
	}
}

// A creation that fails leaves none of the objects it reached in the session: a
// sibling materialized on the way would otherwise survive running nothing.
func TestFailedMaterializationLeavesNoNeighbourBehind(t *testing.T) {
	src := `
		package test {
			item def Ping;

			part def Listener {
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then heard;
					state heard;
				}
			}

			part def Broken {
				exhibit state sending {
					entry; then sent;
					state sent { entry send Ping() to good; }
				}
				exhibit state empty;
			}

			part good : Listener;
			part bad : Broken;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)

	if _, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "bad")); err == nil {
		t.Fatal("expected the machine with no initial state to fail materialization")
	}
	if got := len(ctx.instances); got != 0 {
		t.Errorf("%d object(s) left alive after a failed creation, want none", got)
	}
	if got := len(ctx.objectBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left attached after a failed creation", got)
	}
}

// Two nested objects addressing each other are materialized once each: an object
// is held by the feature that materializes it before its behaviors start, so a
// reply addressed back reaches it instead of materializing a second copy. The
// only other objects are the two messages the sends construct.
func TestMutuallyAddressedObjectsAreMaterializedOnce(t *testing.T) {
	src := `
		package test {
			item def Ping;
			item def Pong;

			part def Node;

			part def Pair {
				part a : Node {
					exhibit state pinging {
						entry; then sending;
						state sending {
							entry send new Ping() to b;
						}
						accept Pong then answered;
						state answered;
					}
				}
				part b : Node {
					exhibit state replying {
						entry; then waiting;
						state waiting;
						accept Ping then replied;
						state replied { entry send new Pong() to a; }
					}
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)

	pair, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Pair"))
	if err != nil {
		t.Fatalf("Instantiate Pair: %v", err)
	}
	a := instanceAtPath(t, ctx, pair, "a")
	b := instanceAtPath(t, ctx, pair, "b")

	if got := len(ctx.instances); got != 7 {
		t.Errorf("%d objects materialized, want the pair, its parts, their state performances, and the two messages", got)
	}
	assertCurrentState(t, machineOf(t, a, "pinging").State, "answered")
	assertCurrentState(t, machineOf(t, b, "replying").State, "replied")
}

// machineOf is the object's execution of the named state machine.
func machineOf(t *testing.T, inst *Instance, name string) *ObjectBehavior {
	t.Helper()
	behavior, ok := inst.Behavior(name)
	if !ok || behavior.State == nil {
		t.Fatalf("object #%d runs no state machine %q", inst.ID, name)
	}
	return behavior
}

// A holder's creation fails with the creation of a part it runs, and leaves
// nothing behind: neither the holder, nor the neighbour the failed part reached,
// nor any behavior of theirs.
func TestFailedNestedStartFailsTheHolder(t *testing.T) {
	src := `
		package test {
			item def Ping;

			part def Listener {
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then heard;
					state heard;
				}
			}

			part def Broken {
				exhibit state sending {
					entry; then sent;
					state sent { entry send Ping() to good; }
				}
				exhibit state empty;
			}

			part def Group {
				part good : Listener;
				part bad : Broken;
			}
			part group : Group;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)

	sym := resolveSymbol(t, pkg.Scope, "group")
	if _, err := ctx.Instantiate(sym); err == nil {
		t.Fatal("expected the part whose machine has no initial state to fail its holder's creation")
	}
	if got := len(ctx.instances); got != 0 {
		t.Errorf("%d object(s) left alive after a failed creation, want none", got)
	}
	if got := len(ctx.objectBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left attached after a failed creation", got)
	}
	if _, named := ctx.occurrences[sym]; named {
		t.Error("the failed creation is still what the usage denotes")
	}
}

// An optional part whose type runs a machine is not created with its holder:
// nothing is required to hold it, so nothing runs until something reads it.
func TestOptionalBehavingPartIsNotCreatedWithItsHolder(t *testing.T) {
	src := `
		package test {
			part def Ticker {
				attribute n : ScalarValues::Integer = 0;
				exhibit state ticking {
					entry; then on;
					state on { entry assign n := n + 1; }
				}
			}
			part def Bay { part maybe : Ticker [0..1]; }
			part def Holder {
				part maybe : Ticker [0..1];
				part bay : Bay;
				part must : Ticker;
			}
			part holder : Holder;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "holder"))
	if err != nil {
		t.Fatal(err)
	}
	if fv := inst.FeatureValues["maybe"]; fv.Materialized {
		t.Errorf("the optional part was created with its holder: %+v", fv)
	}
	// A required part that runs nothing itself, and whose only behaving part is
	// optional, is as lazy as any other part without behaviors.
	if fv := inst.FeatureValues["bay"]; fv.Materialized {
		t.Errorf("the part holding only an optional behaving part was created with its holder: %+v", fv)
	}
	if fv := inst.FeatureValues["must"]; !fv.Materialized {
		t.Errorf("the required part was not created with its holder: %+v", fv)
	}
	// The holder, the required part, and the required part's state performance.
	if got := len(ctx.instances); got != 3 {
		t.Errorf("%d object(s) after creating the holder, want 3", got)
	}

	// A part required in unbounded number is not optional: it fails its holder
	// as reading it would.
	src = `
		package test {
			part def Ticker {
				exhibit state ticking { entry; then on; state on; }
			}
			part def Holder { part all : Ticker [*..*]; }
			part holder : Holder;
		}
	`
	model, resolver, root = parseAndBuildModel(t, src)
	pkg = resolveSymbol(t, root, "test")
	ctx = NewContext(model, resolver, 10000)
	if _, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "holder")); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("creating a holder of an unbounded required part: got %v, want ErrMultiplicityViolation", err)
	}

	// A required part whose upper bound cannot be determined is not optional
	// either: it fails its holder as reading it would.
	src = `
		package test {
			part def Ticker {
				exhibit state ticking { entry; then on; state on; }
			}
			part def Holder { part some : Ticker [1..n]; }
			part holder : Holder;
		}
	`
	model, resolver, root = parseAndBuildModel(t, src)
	pkg = resolveSymbol(t, root, "test")
	ctx = NewContext(model, resolver, 10000)
	if _, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "holder")); err == nil || !strings.Contains(err.Error(), "unknown multiplicity") {
		t.Errorf("creating a holder of a required part of unknown upper bound: got %v, want an unknown-multiplicity error", err)
	}
}

// A behavior started by a creation that names its own usage reaches the object
// being created, and a second creation of the usage is what it denotes from then
// on; a failed second creation leaves the first as what the usage denotes.
func TestStartupNamingItsOwnUsageReachesTheObjectCreated(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Counter {
				attribute n : Integer = 0;
				attribute seen : Integer = 0;
				exhibit state sm {
					entry; then s1;
					state s1 { entry assign seen := counter.n + 1; }
				}
			}
			part counter : Counter;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	sym := resolveSymbol(t, pkg.Scope, "counter")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate counter: %v", err)
	}
	if got := len(ctx.instances); got != 2 {
		t.Errorf("%d objects materialized, want the counter and its state performance", got)
	}
	if id := ctx.occurrences[sym]; id != first.ID {
		t.Errorf("counter denotes object %d, want the one created, %d", id, first.ID)
	}

	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate counter again: %v", err)
	}
	if id := ctx.occurrences[sym]; id != second.ID {
		t.Errorf("counter denotes object %d after a second creation, want %d", id, second.ID)
	}
	if _, held := ctx.Instance(first.ID); !held {
		t.Error("the first object is gone after a second creation")
	}
}

// A machine that reached its final state takes no step, so a message still
// addressed to it does not make a later materialization spin to its budget.
func TestMessageLeftForACompletedMachineDoesNotBlockANewObject(t *testing.T) {
	src := `
		part def Chirp {
			exhibit state modes {
				entry; then working;
				state working {
					entry; then inner;
					state inner;
					accept Ping then working;
					succession first inner then done;
				}
			}
		}
		attribute def Ping;
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	sym := resolveSymbol(t, root, "Chirp")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate first: %v", err)
	}
	behavior, ok := first.ExhibitedState()
	if !ok {
		t.Fatal("the object exhibits no machine")
	}
	if got := behavior.State.State(); got != StateCompleted {
		t.Fatalf("machine state = %v, want it completed at its final state", got)
	}
	// The completed machine's enclosing state still accepts Ping, so the message
	// stays in flight with no consumer able to take it.
	ctx.PostMessage(Message{SignalType: "Ping", Target: "modes", Object: first.ID})

	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate second: %v", err)
	}
	if second.ID == first.ID {
		t.Errorf("the second object took identity %d", second.ID)
	}
}

// A decision of an action an object performs reads the object's feature values,
// so a branch is chosen on what the action itself has written.
func TestPerformedActionDecidesOnItsOwnWrite(t *testing.T) {
	src := `
		part def Watchdog {
			attribute level: Integer = 0;
			attribute alerted: Integer = 0;

			perform action watch {
				first start;

				action raise {
					assign level := 5;
				}

				action alert {
					assign alerted := 1;
				}

				action quiet {
					assign alerted := 2;
				}

				done;

				succession first start then raise;
				succession first raise then check;
				succession first alert then done;
				succession first quiet then done;

				decide check;
				if level > 0 then alert;
				else quiet;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Watchdog"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := featureInt(t, ctx, inst, "level"); got != 5 {
		t.Fatalf("level = %d, want 5 written by the action", got)
	}
	if got := featureInt(t, ctx, inst, "alerted"); got != 1 {
		t.Errorf("alerted = %d, want 1: the decision read the level the action wrote", got)
	}
}
