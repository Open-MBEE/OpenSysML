package queryexec

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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

// A shadow whose only value cannot take the operand does not justify deferring
// the built-in when an earlier conjunction term removes that shadow's row.
const filteredTextShadowFixture = `
metadata def Tag { attribute label : String; }
state def Machine {
	entry; then ready;
	state ready;
	@Tag about ready { label = "txt"; }
}
part def TextShadow { attribute annotations : String = "x"; }
part def Controller { exhibit state modes : Machine; }
part controller : Controller;
part textShadow : TextShadow;
calc def BareShadow :> Query {
	in root : Element;
	WhereFeature(
		source = States(source = root),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def UnionShadow :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = States(source = root),
			other = Named(qualifiedName = "Observatory::textShadow")
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def FilteredShadow :> Query {
	in root : Element;
	WhereFeature(
		source = WhereName(
			source = Union(
				source = States(source = root),
				other = Named(qualifiedName = "Observatory::textShadow")
			),
			operator = "=", value = "ready"
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`

func TestQueryFilteredTextShadowStillRejectsOrderingOperand(t *testing.T) {
	fixture := loadExecutionFixture(t, filteredTextShadowFixture)
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
	for _, name := range []string{"BareShadow", "UnionShadow", "FilteredShadow"} {
		_, err := Execute(fixture.program(t, name), context, bindings, Options{})
		failure := executionError(t, err, ErrorInvalidOperator)
		if failure.Actual != ">" {
			t.Errorf("%s operator error = %+v, want >", name, failure)
		}
	}
}

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
			attribute annotations : Integer = 5;
			entry; then right;
			state right;
		}
	}
}
part def Controller { exhibit state modes : Regions; }
part controller : Controller;
calc def DelegatedValues :> Query {
	in root : Element;
	Project(
		source = States(source = root),
		properties = ("statePath", "owner.annotations"), tolerate = true
	)
}
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
	bindings := Bindings{"root": {ObjectValue(controller, "controller")}}
	values, err := Execute(fixture.program(t, "DelegatedValues"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute DelegatedValues: %v", err)
	}
	for _, row := range values.Rows() {
		cells := row.Cells()
		pathValues := cells[0].Values()
		if len(pathValues) != 1 {
			t.Fatalf("state path = %v, want one value", pathValues)
		}
		path, _ := pathValues[0].String()
		want := sourceProjectedBuiltin
		if path == "both.right" {
			want = sourceProjectedShadow
		}
		sources := cells[1].sources
		if len(sources) != 1 || sources[0] != want {
			t.Errorf("owner.annotations of %s has sources %v, want %v", path, sources, want)
		}
	}
	result, err := Execute(fixture.program(t, "ScopedOwners"), context, bindings, Options{})
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

// A live value written into an otherwise uninitialized feature is the evidence
// a filter needs before it can skip a built-in annotation's incompatible text.
const liveHeldAnnotationFixture = `
private import ScalarValues::*;
metadata def Tag { attribute label : String; }
part def Holder { attribute annotations : Integer; }
part holder : Holder;
part annotated;
@Tag about annotated { label = "text"; }
calc def Matches :> Query {
	WhereFeature(
		source = Objects(type = "PartUsage"),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def NoRows :> Query {
	WhereFeature(
		source = Objects(type = "PartUsage"),
		'feature' = "annotations", operator = ">", value = "9"
	)
}
`

func TestQueryLiveHeldShadowSuppliesOperandEvidence(t *testing.T) {
	fixture := loadExecutionFixture(t, liveHeldAnnotationFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	holder, err := ctx.Instantiate(fixture.symbol(t, "holder"))
	if err != nil {
		t.Fatalf("Instantiate holder: %v", err)
	}
	if err := holder.SetFeatureValue(ctx, "annotations", runtime.Value{
		Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5},
	}); err != nil {
		t.Fatalf("SetFeatureValue annotations: %v", err)
	}
	annotated, err := ctx.Instantiate(fixture.symbol(t, "annotated"))
	if err != nil {
		t.Fatalf("Instantiate annotated: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "holder", Object: holder}, {Label: "annotated", Object: annotated}},
	}
	result, err := Execute(fixture.program(t, "Matches"), context, Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute Matches: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("live annotation rows = %d, want the holder alone", len(rows))
	}
	_, label, isObject := rows[0].Element().Object()
	if !isObject || label != "holder" {
		t.Fatalf("live annotation row = %q, want holder", label)
	}
	// A live value that compares but does not match is still a comparable
	// shadow: it may yield no rows, but must not become an operand error.
	result, err = Execute(fixture.program(t, "NoRows"), context, Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute NoRows: %v", err)
	}
	if n := len(result.Rows()); n != 0 {
		t.Fatalf("nonmatching rows = %d, want none", n)
	}
}

// A direct projection of a shadowed name must defer to the same comparable
// feature its unprojected read defers to, without making ordinary features lax.
const projectedShadowFixture = `
private import ScalarValues::*;
part def NumericAnnotations { attribute annotations : Integer = 5; }
part def TextAnnotations { attribute annotations : String = "text"; }
part numericAnnotations : NumericAnnotations;
part textAnnotations : TextAnnotations;
calc def BeforeProjection :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Named(qualifiedName = "Observatory::numericAnnotations"),
			other = Named(qualifiedName = "Observatory::textAnnotations")
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def AfterProjection :> Query {
	in root : Element;
	WhereFeature(
		source = Project(
			source = Union(
				source = Named(qualifiedName = "Observatory::numericAnnotations"),
				other = Named(qualifiedName = "Observatory::textAnnotations")
			),
			properties = ("annotations")
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`

func TestQueryProjectedShadowKeepsDeferralEligibility(t *testing.T) {
	fixture := loadExecutionFixture(t, projectedShadowFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "numericAnnotations"))}}
	for _, name := range []string{"BeforeProjection", "AfterProjection"} {
		result, err := fixture.execute(t, name, bindings, Options{})
		if err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
		if got, want := rowNames(result), []string{"Observatory::numericAnnotations"}; !slices.Equal(got, want) {
			t.Errorf("%s rows = %v, want %v", name, got, want)
		}
	}
}

// A computed cell's incompatible text does not hide a successful comparison
// against a directly projected number in the same column.
const projectedComputedMixedColumnFixture = `
private import ScalarValues::*;
part def Level { attribute level : Integer = 5; }
part levelPart : Level;
part plain;
calc def Filter :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Project(
				source = Named(qualifiedName = "Observatory::levelPart"),
				properties = ("level")
			),
			other = Project(
				source = Named(qualifiedName = "Observatory::plain"),
				columns = (Column(name = "level", expression = "five"))
			)
		),
		'feature' = "level", operator = ">", value = "3"
	)
}
`

func TestQueryComparableProjectedCellClearsComputedColumnError(t *testing.T) {
	fixture := loadExecutionFixture(t, projectedComputedMixedColumnFixture)
	result, err := fixture.execute(t, "Filter", Bindings{
		"root": {ElementValue(fixture.symbol(t, "levelPart"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Filter: %v", err)
	}
	if got, want := rowNames(result), []string{"Observatory::levelPart"}; !slices.Equal(got, want) {
		t.Fatalf("mixed column rows = %v, want %v", got, want)
	}
}

// A feature whose name is literally the dotted spelling is read as that
// feature; the owner's separate annotations feature is not its shadow.
const literalDottedFeatureFixture = `
private import ScalarValues::*;
part def Owner {
	attribute annotations : Integer = 5;
	part literal { attribute 'owner.annotations' : String = "text"; }
}
calc def Read :> Query {
	in root : Element;
	Project(
		source = Named(qualifiedName = "Observatory::Owner::literal"),
		properties = ("owner.annotations")
	)
}
calc def Filter :> Query {
	in root : Element;
	WhereFeature(
		source = Named(qualifiedName = "Observatory::Owner::literal"),
		'feature' = "owner.annotations", operator = ">", value = "3"
	)
}
`

func TestQueryLiteralDottedFeatureDoesNotBorrowAPathShadow(t *testing.T) {
	fixture := loadExecutionFixture(t, literalDottedFeatureFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Owner"))}}
	result, err := fixture.execute(t, "Read", bindings, Options{})
	if err != nil {
		t.Fatalf("execute Read: %v", err)
	}
	if got := cellStrings(t, result, "owner.annotations"); !slices.EqualFunc(got, [][]string{{"text"}}, slices.Equal) {
		t.Fatalf("literal dotted feature = %v, want text", got)
	}
	_, err = fixture.execute(t, "Filter", bindings, Options{})
	failure := executionError(t, err, ErrorInvalidOperator)
	if failure.Actual != ">" {
		t.Fatalf("operator error = %+v, want >", failure)
	}
}

// A computed column replaces the declared feature under its name, so that
// hidden declaration cannot excuse an invalid operand on a built-in cell.
const computedColumnHidesShadowFixture = `
private import ScalarValues::*;
state def Machine { entry; then ready; state ready; }
part def TextActivity { attribute isActive : String = "ready"; }
part def Controller { exhibit state modes : Machine; }
part controller : Controller;
part textActivity : TextActivity;
calc def Filter :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = Project(
				source = States(source = root),
				properties = ("isActive")
			),
			other = Project(
				source = Named(qualifiedName = "Observatory::textActivity"),
				columns = (Column(name = "isActive", expression = "other"))
			)
		),
		'feature' = "isActive", operator = "=", value = "ready"
	)
}
`

func TestQueryComputedColumnDoesNotInheritAHiddenShadow(t *testing.T) {
	fixture := loadExecutionFixture(t, computedColumnHidesShadowFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}},
	}
	_, err = Execute(fixture.program(t, "Filter"), context,
		Bindings{"root": {ObjectValue(controller, "controller")}}, Options{})
	failure := executionError(t, err, ErrorInvalidArgument)
	if failure.Parameter != "value" || failure.Actual != "ready" {
		t.Fatalf("operand error = %+v, want value ready", failure)
	}
}

// A projected shadow removed by an earlier conjunction term still excuses an
// incompatible cell, so reversing the terms cannot turn no rows into an error.
const projectedShadowConjunctionOrderFixture = `
private import ScalarValues::*;
part def NumericAnnotations { attribute annotations : Integer = 5; }
part def TextAnnotations { attribute annotations : String = "text"; }
part numericAnnotations : NumericAnnotations;
part textAnnotations : TextAnnotations;
calc def NameThenFilter :> Query {
	in root : Element;
	WhereFeature(
		source = WhereName(
			source = Project(
				source = Union(
					source = Named(qualifiedName = "Observatory::numericAnnotations"),
					other = Named(qualifiedName = "Observatory::textAnnotations")
				),
				properties = ("annotations")
			),
			operator = "=", value = "textAnnotations"
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def FilterThenName :> Query {
	in root : Element;
	WhereName(
		source = WhereFeature(
			source = Project(
				source = Union(
					source = Named(qualifiedName = "Observatory::numericAnnotations"),
					other = Named(qualifiedName = "Observatory::textAnnotations")
				),
				properties = ("annotations")
			),
			'feature' = "annotations", operator = ">", value = "3"
		),
		operator = "=", value = "textAnnotations"
	)
}
`

func TestQueryProjectedShadowOperandIsTermOrderIndependent(t *testing.T) {
	fixture := loadExecutionFixture(t, projectedShadowConjunctionOrderFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "numericAnnotations"))}}
	for _, name := range []string{"NameThenFilter", "FilterThenName"} {
		result, err := fixture.execute(t, name, bindings, Options{})
		if err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
		if n := len(result.Rows()); n != 0 {
			t.Errorf("%s rows = %d, want none in either order", name, n)
		}
	}
}
