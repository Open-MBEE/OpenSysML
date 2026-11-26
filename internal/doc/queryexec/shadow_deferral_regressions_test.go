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

// A direct projection carries its declared feature's source: one numeric match
// cannot hide an ordering comparison against a text value of the same feature.
const projectedDeclaredFeatureFixture = `
private import ScalarValues::*;
part def NumericLevel { attribute level : Integer = 5; }
part def TextLevel { attribute level : String = "five"; }
part numericLevel : NumericLevel;
part textLevel : TextLevel;
calc def BeforeProjection :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Named(qualifiedName = "Observatory::numericLevel"),
			other = Named(qualifiedName = "Observatory::textLevel")
		),
		'feature' = "level", operator = ">", value = "3"
	)
}
calc def AfterProjection :> Query {
	in root : Element;
	WhereFeature(
		source = Project(
			source = Union(
				source = Named(qualifiedName = "Observatory::numericLevel"),
				other = Named(qualifiedName = "Observatory::textLevel")
			),
			properties = ("level")
		),
		'feature' = "level", operator = ">", value = "3"
	)
}
`

func TestQueryProjectedDeclaredFeatureKeepsStrictOperandValidation(t *testing.T) {
	fixture := loadExecutionFixture(t, projectedDeclaredFeatureFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "numericLevel"))}}
	for _, name := range []string{"BeforeProjection", "AfterProjection"} {
		_, err := fixture.execute(t, name, bindings, Options{})
		failure := executionError(t, err, ErrorInvalidOperator)
		if failure.Actual != ">" {
			t.Errorf("%s operator error = %+v, want >", name, failure)
		}
	}
}

// A comparable cell that does not match is still a valid comparison: the
// incompatible text cell must not turn an empty result into an operator error.
const nonmatchingComputedColumnFixture = `
part numeric;
part textual;
calc def Nonmatching :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Project(
				source = Named(qualifiedName = "Observatory::numeric"),
				columns = (Column(name = "isActive", expression = 2))
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

func TestQueryValidNonmatchingComputedCellIsNotAnOperatorError(t *testing.T) {
	fixture := loadExecutionFixture(t, nonmatchingComputedColumnFixture)
	result, err := fixture.execute(t, "Nonmatching", Bindings{
		"root": {ElementValue(fixture.symbol(t, "numeric"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Nonmatching: %v", err)
	}
	if got := len(result.Rows()); got != 0 {
		t.Fatalf("nonmatching rows = %d, want none", got)
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

// A delegated dotted read keeps the terminal property's source: the annotated
// owner's built-in text is not mislabelled as a declared feature of the owner.
const delegatedBuiltinOnStateFixture = `
metadata def Tag { attribute label : String; }
state def Regions {
	entry; then both;
	state both parallel {
		state regionL {
			entry; then left;
			state left;
		}
		@Tag about regionL { label = "text"; }
		state regionR {
			attribute annotations : String = "feature";
			entry; then right;
			state right;
		}
	}
}
part def Controller { exhibit state modes : Regions; }
part controller : Controller;
calc def ScopedOwners :> Query {
	in root : Element;
	WhereFeature(
		source = WhereName(
			source = States(source = root), operator = "=", value = "left"
		),
		'feature' = "owner.annotations", operator = ">", value = "3"
	)
}
`

func TestQueryStateDelegatedDottedReadKeepsBuiltinProvenance(t *testing.T) {
	fixture := loadExecutionFixture(t, delegatedBuiltinOnStateFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}
	result, err := Execute(fixture.program(t, "ScopedOwners"), context,
		Bindings{"root": {ObjectValue(controller, "controller")}}, Options{})
	if err != nil {
		t.Fatalf("execute ScopedOwners: %v", err)
	}
	if got := len(result.Rows()); got != 0 {
		t.Fatalf("scoped owner rows = %d, want the built-in text skipped", got)
	}
}

// A union arm that emits no row is outside the union's scope: its declared
// feature cannot make a built-in operand of a surviving state row defer.
const unionEmptyArmFixture = `
private import ScalarValues::*;
state def Machine { entry; then ready; state ready; }
part def TextActivity { attribute isActive : String = "ready"; }
part def Controller { exhibit state modes : Machine; }
part controller : Controller;
part textActivity : TextActivity;
calc def StateOnly :> Query {
	in root : Element;
	WhereFeature(
		source = States(source = root),
		'feature' = "isActive", operator = "=", value = "ready"
	)
}
calc def WithEmptyArm :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = States(source = root),
			other = WhereName(
				source = Named(qualifiedName = "Observatory::textActivity"),
				operator = "=", value = "nomatch"
			)
		),
		'feature' = "isActive", operator = "=", value = "ready"
	)
}
`

func TestQueryUnionEmptyArmDoesNotSupplyOperandEvidence(t *testing.T) {
	fixture := loadExecutionFixture(t, unionEmptyArmFixture)
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
	for _, name := range []string{"StateOnly", "WithEmptyArm"} {
		_, err := Execute(fixture.program(t, name), context, bindings, Options{})
		failure := executionError(t, err, ErrorInvalidArgument)
		if failure.Parameter != "value" || failure.Actual != "ready" {
			t.Errorf("%s operand error = %+v, want value ready", name, failure)
		}
	}
}

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
