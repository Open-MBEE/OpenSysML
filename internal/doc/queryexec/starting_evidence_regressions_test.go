package queryexec

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// A projection over rows a filter already excluded must not read those rows'
// features: reading a lazy part creates the object and starts its behavior.
const excludedLazyPartFixture = `
state def Calibration { state ready; }
part def Sensor { exhibit state mode : Calibration; }
part def Device { part sensor : Sensor[0..1]; }
part device : Device;
calc def ExcludedProjection :> Query {
	in root : Element;
	Project(source = WhereName(source = root, operator = "=", value = "other"), properties = ("sensor"))
}
`

func TestQueryProjectionOfExcludedRowsKeepsLazyPartsLazy(t *testing.T) {
	fixture := loadExecutionFixture(t, excludedLazyPartFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	device, err := ctx.Instantiate(fixture.symbol(t, "device"))
	if err != nil {
		t.Fatalf("Instantiate device: %v", err)
	}
	lazy := device.FeatureValues["sensor"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("sensor before query = %+v, want an unmaterialized part", lazy)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "device", Object: device}},
	}
	result, err := Execute(fixture.program(t, "ExcludedProjection"), context,
		Bindings{"root": {ObjectValue(device, "device")}}, Options{})
	if err != nil {
		t.Fatalf("execute ExcludedProjection: %v", err)
	}
	if n := len(result.Rows()); n != 0 {
		t.Fatalf("excluded projection rows = %d, want none", n)
	}
	if lazy.Materialized {
		t.Fatal("projecting an excluded row materialized its sensor part")
	}
}

// Collecting evidence from a held row must probe its lazy features, so the
// probe's materialization cannot outlive the query in the live session.
const lazyHeldEvidenceFixture = `
private import ScalarValues::*;
metadata def Tag { attribute label : String; }
state def Calibration { state ready; }
part def Sensor { exhibit state mode : Calibration; }
part def LazyHolder { part annotations : Sensor[0..1]; }
part def NumericHolder { attribute annotations : Integer; }
part lazyHolder : LazyHolder;
part numericHolder : NumericHolder;
part annotated;
@Tag about annotated { label = "text"; }
calc def Filter :> Query {
	in lazy : Element;
	in numeric : Element;
	in text : Element;
	WhereFeature(
		source = Project(
			source = WhereName(
				source = Union(source = Union(source = lazy, other = numeric), other = text),
				operator = "=", value = "numericHolder"
			),
			properties = ("annotations")
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`

func TestQueryHeldShadowEvidenceDoesNotMaterializeLazyParts(t *testing.T) {
	fixture := loadExecutionFixture(t, lazyHeldEvidenceFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	lazyHolder, err := ctx.Instantiate(fixture.symbol(t, "lazyHolder"))
	if err != nil {
		t.Fatalf("Instantiate lazyHolder: %v", err)
	}
	lazy := lazyHolder.FeatureValues["annotations"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("annotations before query = %+v, want an unmaterialized part", lazy)
	}
	numericHolder, err := ctx.Instantiate(fixture.symbol(t, "numericHolder"))
	if err != nil {
		t.Fatalf("Instantiate numericHolder: %v", err)
	}
	if err := numericHolder.SetFeatureValue(ctx, "annotations", runtime.Value{
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
		Roots: []Root{{Label: "lazyHolder", Object: lazyHolder}, {Label: "numericHolder", Object: numericHolder}, {Label: "annotated", Object: annotated}},
	}
	bindings := Bindings{
		"lazy":    {ObjectValue(lazyHolder, "lazyHolder")},
		"numeric": {ObjectValue(numericHolder, "numericHolder")},
		"text":    {ObjectValue(annotated, "annotated")},
	}
	result, err := Execute(fixture.program(t, "Filter"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute Filter: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want the numeric holder alone", len(rows))
	}
	if _, label, _ := rows[0].Element().Object(); label != "numericHolder" {
		t.Fatalf("filtered row = %q, want numericHolder", label)
	}
	if lazy.Materialized {
		t.Fatal("collecting shadow evidence materialized a lazy part in the live session")
	}
}

// A failed activity scan must surface as a typed error: the partial index must
// never answer isActive for a state the walk did not reach.
const failedActivityScanFixture = `
state def Machine { entry; then ready; state ready; }
part def Controller { exhibit state modes : Machine; }
part controller : Controller;
part other;
calc def Inactive :> Query {
	in root : Element;
	WhereFeature(
		source = Named(qualifiedName = "Observatory::Machine::ready"),
		'feature' = "isActive", operator = "=", value = "false"
	)
}
`

func TestQueryFailedActivityScanIsNotTreatedAsInactive(t *testing.T) {
	fixture := loadExecutionFixture(t, failedActivityScanFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	other, err := ctx.Instantiate(fixture.symbol(t, "other"))
	if err != nil {
		t.Fatalf("Instantiate other: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "controller", Object: controller}, {Label: "other", Object: other}},
	}
	// Enough budget for the query's own reads, too little to walk the session
	// and build the activity index.
	_, err = Execute(fixture.program(t, "Inactive"), context,
		Bindings{"root": {ElementValue(fixture.symbol(t, "Machine"))}}, Options{VisitBudget: 1})
	failure := executionError(t, err, ErrorUnevaluableFeature)
	var budget *Error
	if !errors.As(failure.Cause, &budget) || budget.Kind != ErrorVisitBudget {
		t.Fatalf("failed activity scan cause = %v, want a visit-budget error", failure.Cause)
	}
}

// Evidence for a state row must not run the isActive built-in: doing so builds
// the activity index and can swallow a failed session walk during evidence.
func TestShadowEvidenceOnAStateRowDoesNotBuildTheActivityIndex(t *testing.T) {
	fixture := loadExecutionFixture(t, failedActivityScanFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	machines := stateMachinesOf(controller)
	if len(machines) == 0 || len(machines[0].State.ActiveStates()) == 0 {
		t.Fatal("controller has no active state to read")
	}
	row := StateValue(activeState(controller, "controller", machines[0], machines[0].State.ActiveStates()[0]))
	e := &executor{
		context: Context{
			Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
			Roots: []Root{{Label: "controller", Object: controller}},
		},
		budget: &visitBudget{remaining: 100},
	}
	_, _, sources, err := e.shadowEvidenceReach(row, "isActive")
	if err != nil {
		t.Fatalf("shadow evidence for isActive: %v", err)
	}
	for _, source := range sources {
		if source == sourceBuiltin {
			t.Fatalf("evidence returned a built-in value from %v", sources)
		}
	}
	if e.stateActivity != nil {
		t.Fatal("evidence read built the state activity index")
	}
}

// A failed session walk must not publish a partial activity index: a later
// read must see the failure, not treat unvisited states as inactive.
func TestStateActivityIndexDoesNotPublishAFailedScan(t *testing.T) {
	fixture := loadExecutionFixture(t, failedActivityScanFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	other, err := ctx.Instantiate(fixture.symbol(t, "other"))
	if err != nil {
		t.Fatalf("Instantiate other: %v", err)
	}
	e := &executor{
		context: Context{
			Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
			Roots: []Root{{Label: "controller", Object: controller}, {Label: "other", Object: other}},
		},
		budget: &visitBudget{remaining: 1},
	}
	idx := &stateActivityIndex{}
	if err := idx.build(e, queryplan.Expression{}); err == nil {
		t.Fatal("building the activity index over an exhausted budget succeeded")
	}
	if err := idx.build(e, queryplan.Expression{}); err == nil {
		t.Fatal("a failed scan was published as a complete activity index")
	}
	if idx.built || len(idx.active) != 0 {
		t.Fatalf("failed scan left built=%t active=%v", idx.built, idx.active)
	}
}

// Evidence for a projected column must survive an unevaluable sibling column
// on an excluded row: dropping the whole row cannot hide a comparable shadow.
const siblingColumnEvidenceFixture = `
private import ScalarValues::*;
part def NumericAnnotations {
	attribute annotations : Integer = 5;
	calc def Detail { in factor : Real; return : Real = factor * 2.0; }
	attribute detail : Real = Detail();
}
part def TextAnnotations {
	attribute annotations : String = "text";
	attribute detail : Real = 1.0;
}
part numericAnnotations : NumericAnnotations;
part textAnnotations : TextAnnotations;
calc def NameThenFilter :> Query {
	in root : Element;
	WhereFeature(
		source = Project(
			source = WhereName(
				source = Union(
					source = Named(qualifiedName = "Observatory::numericAnnotations"),
					other = Named(qualifiedName = "Observatory::textAnnotations")
				),
				operator = "=", value = "textAnnotations"
			),
			properties = ("annotations", "detail"), tolerate = true
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`

func TestQueryExcludedRowKeepsEvidenceFromAnotherColumn(t *testing.T) {
	fixture := loadExecutionFixture(t, siblingColumnEvidenceFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "numericAnnotations"))}}
	result, err := fixture.execute(t, "NameThenFilter", bindings, Options{})
	if err != nil {
		t.Fatalf("execute NameThenFilter: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
}
