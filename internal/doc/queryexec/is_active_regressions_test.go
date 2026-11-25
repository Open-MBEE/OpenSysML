package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

const parallelActivityFixture = `
private import ScalarValues::*;
item def Go;
state def Regions {
	entry; then both;
	state both parallel {
		state regionL {
			entry; then left;
			state left;
		}
		state regionR {
			entry; then right;
			state right;
		}
	}
}
part def Controller { exhibit state modes : Regions; }
part controller : Controller;
calc def Q :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, relationships = true),
		properties = ("name", "isActive"), tolerate = true
	)
}
`

// A running exhibit and every declaration-backed owner of an active parallel
// region answer true, including graph-only owners and the machine usage.
func TestQueryActivityOfExhibitAndParallelRegionOwners(t *testing.T) {
	fixture := loadExecutionFixture(t, parallelActivityFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}
	got := map[string]bool{}
	for _, root := range []string{"Controller", "Regions"} {
		result, err := Execute(fixture.program(t, "Q"), context,
			Bindings{"root": {ElementValue(fixture.symbol(t, root))}}, Options{})
		if err != nil {
			t.Fatalf("execute Q over %s: %v", root, err)
		}
		names := cellStrings(t, result, "name")
		active := cellBooleans(t, result, "isActive")
		for i, name := range names {
			if len(name) != 1 || name[0] == "" || len(active[i]) != 1 {
				continue
			}
			got[name[0]] = active[i][0]
		}
	}
	for _, name := range []string{"modes", "both", "regionL", "left", "regionR", "right"} {
		if !got[name] {
			t.Errorf("isActive of %s = %v, want true; all = %v", name, got[name], got)
		}
	}
}

const copiedActivityFixture = `
private import ScalarValues::*;
item def Go;
state def Counter {
	entry; then counting;
	state counting;
}
state def Twice {
	entry; then one;
	state one : Counter;
	state two : Counter;
	transition first one.counting accept Go then two;
}
part def Controller { exhibit state modes : Twice; }
part controller : Controller;
calc def Q :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, relationships = true),
		properties = ("name", "isActive"), tolerate = true
	)
}
`

// Two usages of one state definition are two graph copies of one declaration.
// Once an event enters the second, the declaration is active in either copy.
func TestQueryActivityAggregatesCopiesOfOneDeclaration(t *testing.T) {
	fixture := loadExecutionFixture(t, copiedActivityFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	behavior, ok := controller.ExhibitedState()
	if !ok || behavior.State == nil {
		t.Fatal("controller exhibits no machine")
	}
	if err := behavior.State.Enqueue(runtime.QueuedEvent{Signal: "Go"}); err != nil {
		t.Fatalf("Enqueue Go: %v", err)
	}
	if err := behavior.State.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	result, err := Execute(fixture.program(t, "Q"), Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}, Bindings{"root": {ElementValue(fixture.symbol(t, "Counter"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	names := cellStrings(t, result, "name")
	active := cellBooleans(t, result, "isActive")
	found := false
	for i, name := range names {
		if len(name) != 1 || name[0] != "counting" {
			continue
		}
		found = true
		if len(active[i]) != 1 || !active[i][0] {
			t.Fatalf("isActive of the second counting copy = %v, want [true]; all = %v", active[i], active)
		}
	}
	if !found {
		t.Fatalf("no counting row in %v", names)
	}
}
