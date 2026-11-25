package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

const stateActivityFixture = `
private import SI::*;
attribute def Go;
state def Machine {
	entry; then ready;
	state ready;
	state idle;
	transition first ready accept Go then done;
}
part def Controller { exhibit state modes : Machine; }
part controller : Controller;
calc def StateActivityQuery :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "StateUsage", exact = true),
		properties = ("name", "isActive")
	)
}
`

// isActive answers every state usage of a running machine in one query: the
// state the machine started in is active and the one it never entered is not.
func TestQueryAnswersStateActivity(t *testing.T) {
	fixture := loadExecutionFixture(t, stateActivityFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}
	result, err := Execute(fixture.program(t, "StateActivityQuery"), context, Bindings{
		"root": {ElementValue(fixture.symbol(t, "Machine"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute StateActivityQuery: %v", err)
	}
	names := cellStrings(t, result, "name")
	active := cellBooleans(t, result, "isActive")
	if len(names) != len(active) {
		t.Fatalf("names = %v, isActive = %v", names, active)
	}
	got := map[string]bool{}
	for i, name := range names {
		if len(name) != 1 || len(active[i]) != 1 {
			t.Fatalf("row %d: name = %v, isActive = %v, want one value each", i, name, active[i])
		}
		got[name[0]] = active[i][0]
	}
	want := map[string]bool{"ready": true, "idle": false}
	for name, value := range want {
		if found, ok := got[name]; !ok || found != value {
			t.Errorf("isActive of %s = %v (present %v), want %v; all = %v", name, found, ok, value, got)
		}
	}
}

// Without a runtime session no state is running, so every state answers false
// and the property stays a Boolean one rather than an absent one.
func TestQueryStateActivityIsFalseWithoutASession(t *testing.T) {
	fixture := loadExecutionFixture(t, stateActivityFixture)
	result, err := Execute(fixture.program(t, "StateActivityQuery"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, Bindings{
			"root": {ElementValue(fixture.symbol(t, "Machine"))},
		}, Options{})
	if err != nil {
		t.Fatalf("execute StateActivityQuery: %v", err)
	}
	active := cellBooleans(t, result, "isActive")
	if len(active) != 2 {
		t.Fatalf("isActive rows = %v, want the two state usages", active)
	}
	for i, values := range active {
		if len(values) != 1 || values[0] {
			t.Fatalf("row %d isActive = %v, want [false]", i, values)
		}
	}
}
