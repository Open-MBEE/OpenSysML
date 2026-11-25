package queryexec

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// A projected column is read from its cell, so a numeric feature on the unused
// model path cannot excuse an ordering operator on text.
const computedColumnShadowFixture = `
part def Shadowed {
	attribute isActive : Integer = 5;
}
part shadowed : Shadowed;
calc def Filtered :> Query {
	in root : Element;
	WhereFeature(
		source = Project(
			source = Named(qualifiedName = "Observatory::shadowed"),
			columns = (Column(name = "isActive", expression = "text"))
		),
		'feature' = "isActive", operator = ">", value = "3"
	)
}
`

func TestQueryComputedColumnDoesNotInheritAModelShadow(t *testing.T) {
	fixture := loadExecutionFixture(t, computedColumnShadowFixture)
	_, err := fixture.execute(t, "Filtered", Bindings{
		"root": {ElementValue(fixture.symbol(t, "shadowed"))},
	}, Options{})
	failure := executionError(t, err, ErrorInvalidOperator)
	if failure.Actual != ">" {
		t.Fatalf("operator error = %+v, want >", failure)
	}
}

// A heterogeneous column keeps the cells that can be compared: the numeric
// cell matches, the text cell is incompatible and does not abort the filter.
const mixedComputedColumnFixture = `
part numeric;
part textual;
calc def Mixed :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Project(
				source = Named(qualifiedName = "Observatory::numeric"),
				columns = (Column(name = "isActive", expression = 5))
			),
			other = Project(
				source = Named(qualifiedName = "Observatory::textual"),
				columns = (Column(name = "isActive", expression = "five"))
			)
		),
		'feature' = "isActive", operator = ">", value = "3"
	)
}
`

func TestQueryFilterSkipsIncompatibleComputedCells(t *testing.T) {
	fixture := loadExecutionFixture(t, mixedComputedColumnFixture)
	result, err := fixture.execute(t, "Mixed", Bindings{
		"root": {ElementValue(fixture.symbol(t, "numeric"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Mixed: %v", err)
	}
	if got, want := rowNames(result), []string{"Observatory::numeric"}; !slices.Equal(got, want) {
		t.Fatalf("mixed column rows = %v, want %v", got, want)
	}
}

// A state row's text isActive shadows activity while a sibling leaf's Boolean
// row is skipped, not rejected by the filter.
const stateDeclarationShadowFixture = `
private import ScalarValues::*;
item def Go;
state def Regions {
	entry; then both;
	state both parallel {
		state regionL {
			entry; then left;
			state left { attribute isActive : String = "ready"; }
		}
		state regionR {
			entry; then right;
			state right;
		}
	}
}
part def Controller { exhibit state modes : Regions; }
part controller : Controller;
calc def Active :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = States(source = root),
			'feature' = "isActive", operator = "=", value = "ready"
		),
		properties = ("name", "statePath")
	)
}
calc def Strict :> Query {
	in root : Element;
	WhereFeature(
		source = States(source = root),
		'feature' = "@isActive", operator = "=", value = "ready"
	)
}
`

func TestQueryStateDeclarationShadowsActivityInAFilter(t *testing.T) {
	fixture := loadExecutionFixture(t, stateDeclarationShadowFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}
	bindings := Bindings{"root": {ObjectValue(controller, "controller")}}
	result, err := Execute(fixture.program(t, "Active"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute Active: %v", err)
	}
	assertColumn(t, cellsByColumn(t, result), "statePath", [][]string{{"both.left"}})
	// The unambiguous spelling still reaches the Boolean built-in and rejects
	// the text operand even though the declaration shadows the bare name.
	_, err = Execute(fixture.program(t, "Strict"), context, bindings, Options{})
	executionError(t, err, ErrorInvalidArgument)
}
