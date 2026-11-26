package queryexec

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// A union or tree of projected arms hands its own cells to later filters: a
// removed comparable cell still validates the operand in either term order.
const projectedUnionEvidenceFixture = `
private import ScalarValues::*;
metadata def Tag { attribute label : String; }
part def NumericAnnotations {
	attribute annotations : Integer = 5;
	attribute detail : Real = 1.0;
}
part def TextAnnotations {
	attribute annotations : String = "text";
	attribute detail : Real = 1.0;
}
part numericAnnotations : NumericAnnotations;
part textAnnotations : TextAnnotations;
@Tag about textAnnotations { label = "txt"; }
calc def NameThenFilter :> Query {
	in root : Element;
	WhereFeature(
		source = WhereName(
			source = Union(
				source = Project(source = Named(qualifiedName = "Observatory::numericAnnotations"), properties = ("annotations", "detail")),
				other = Project(source = Named(qualifiedName = "Observatory::textAnnotations"), properties = ("annotations", "detail"))
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
			source = Union(
				source = Project(source = Named(qualifiedName = "Observatory::numericAnnotations"), properties = ("annotations", "detail")),
				other = Project(source = Named(qualifiedName = "Observatory::textAnnotations"), properties = ("annotations", "detail"))
			),
			'feature' = "annotations", operator = ">", value = "3"
		),
		operator = "=", value = "textAnnotations"
	)
}
calc def TreeNameThenFilter :> Query {
	in root : Element;
	WhereFeature(
		source = WhereName(
			source = Tree(
				source = Union(
					source = Project(source = Named(qualifiedName = "Observatory::numericAnnotations"), properties = ("annotations", "detail")),
					other = Project(source = Named(qualifiedName = "Observatory::textAnnotations"), properties = ("annotations", "detail"))
				)
			),
			operator = "=", value = "textAnnotations"
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
calc def TreeFilterThenName :> Query {
	in root : Element;
	WhereName(
		source = WhereFeature(
			source = Tree(
				source = Union(
					source = Project(source = Named(qualifiedName = "Observatory::numericAnnotations"), properties = ("annotations", "detail")),
					other = Project(source = Named(qualifiedName = "Observatory::textAnnotations"), properties = ("annotations", "detail"))
				)
			),
			'feature' = "annotations", operator = ">", value = "3"
		),
		operator = "=", value = "textAnnotations"
	)
}
`

func TestQueryProjectedSetOpsKeepOperandEvidenceAcrossFilters(t *testing.T) {
	fixture := loadExecutionFixture(t, projectedUnionEvidenceFixture)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "numericAnnotations"))}}
	for _, name := range []string{
		"NameThenFilter", "FilterThenName", "TreeNameThenFilter", "TreeFilterThenName",
	} {
		t.Run(name, func(t *testing.T) {
			result, err := fixture.execute(t, name, bindings, Options{})
			if err != nil {
				t.Fatalf("execute %s: %v", name, err)
			}
			if got := rowNames(result); len(got) != 0 {
				t.Fatalf("rows = %v, want none", got)
			}
		})
	}
}

// Evidence of an object-valued feature must capture the value and its label
// inside the probe: resolving the object after rollback drops that evidence.
const lazyShadowIdentityFixture = `
private import ScalarValues::*;
item def Go;
state def Calibration { entry; then ready; state ready; }
item def Payload;
part def LazyHolder {
	item isActive : Payload[4];
	item head : Payload[1] :> isActive;
	item spare : Payload[0..3] :> isActive;
}
part def Controller { exhibit state modes : Calibration; }
part lazyHolder : LazyHolder;
part controller : Controller;
calc def Filter :> Query {
	in root : Element;
	WhereFeature(
		source = Union(
			source = States(source = root),
			other = WhereName(source = Objects(type = "PartUsage"), operator = "=", value = "lazyHolder")
		),
		'feature' = "isActive", operator = "=", value = "lazyHolder.isActive[1]"
	)
}
`

func TestQueryLazyShadowEvidenceKeepsItsCapturedValue(t *testing.T) {
	fixture := loadExecutionFixture(t, lazyShadowIdentityFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	controller, err := ctx.Instantiate(fixture.symbol(t, "controller"))
	if err != nil {
		t.Fatalf("Instantiate controller: %v", err)
	}
	lazyHolder, err := ctx.Instantiate(fixture.symbol(t, "lazyHolder"))
	if err != nil {
		t.Fatalf("Instantiate lazyHolder: %v", err)
	}
	lazy := lazyHolder.FeatureValues["isActive"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("isActive before query = %+v, want an unmaterialized part", lazy)
	}
	e := &executor{
		context: Context{
			Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
			Roots: []Root{{Label: "controller", Object: controller}, {Label: "lazyHolder", Object: lazyHolder}},
		},
		budget: &visitBudget{remaining: 100},
	}
	values, present, err := e.objectFeatureValuesEvidence(ObjectValue(lazyHolder, "lazyHolder"), "isActive")
	if err != nil {
		t.Fatalf("probe isActive evidence: %v", err)
	}
	if !present {
		t.Fatal("isActive evidence absent")
	}
	labels := make([]string, 0, len(values))
	for _, value := range values {
		labels = append(labels, e.valueText(value))
	}
	want := []string{"lazyHolder.isActive[1]", "lazyHolder.isActive[2]", "lazyHolder.isActive[3]", "lazyHolder.isActive[4]"}
	if !slices.Equal(labels, want) {
		t.Fatalf("evidence labels = %v, want %v", labels, want)
	}
	if lazy.Materialized {
		t.Fatal("probing evidence materialized the lazy part in the live session")
	}
	bindings := Bindings{"root": {ObjectValue(controller, "controller")}}
	result, err := Execute(fixture.program(t, "Filter"), e.context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute Filter: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want the lazy holder alone", len(rows))
	}
	if _, label, _ := rows[0].Element().Object(); label != "lazyHolder" {
		t.Fatalf("filtered row = %q, want lazyHolder", label)
	}
}

// A dotted evidence read must probe the whole path: resolving an ordinary
// segment outside the probe creates the excluded part and starts its behavior.
const lazyDottedEvidenceFixture = `
private import ScalarValues::*;
metadata def Tag { attribute label : String; }
state def Calibration { entry; then ready; state ready; }
part def PlainSensor { exhibit state mode : Calibration; }
part def NumericSensor {
	exhibit state mode : Calibration;
	attribute annotations : Integer = 5;
}
@Tag about PlainSensor { label = "txt"; }
part def LazyHolder {
	part sensor : NumericSensor[0..1];
	attribute detail : Real = 1.0;
}
part def NumericHolder {
	part sensor : NumericSensor;
	attribute detail : Real = 1.0;
}
part def TextHolder {
	part sensor : PlainSensor;
	attribute detail : Real = 1.0;
}
part lazyHolder : LazyHolder;
part numericHolder : NumericHolder;
part textHolder : TextHolder;
calc def NameThenFilter :> Query {
	in lazy : Element;
	in numeric : Element;
	in text : Element;
	WhereFeature(
		source = Project(
			source = WhereName(
				source = Union(
					source = Union(source = lazy, other = numeric),
					other = text
				),
				operator = "=", value = "textHolder"
			),
			properties = ("sensor.annotations", "detail"), tolerate = true
		),
		'feature' = "sensor.annotations", operator = ">", value = "3"
	)
}
`

func TestQueryDottedEvidenceLeavesExcludedLazyPartLazy(t *testing.T) {
	fixture := loadExecutionFixture(t, lazyDottedEvidenceFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	lazyHolder, err := ctx.Instantiate(fixture.symbol(t, "lazyHolder"))
	if err != nil {
		t.Fatalf("Instantiate lazyHolder: %v", err)
	}
	lazy := lazyHolder.FeatureValues["sensor"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("sensor before query = %+v, want an unmaterialized part", lazy)
	}
	numericHolder, err := ctx.Instantiate(fixture.symbol(t, "numericHolder"))
	if err != nil {
		t.Fatalf("Instantiate numericHolder: %v", err)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "lazyHolder", Object: lazyHolder}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy":    {ObjectValue(lazyHolder, "lazyHolder")},
		"numeric": {ObjectValue(numericHolder, "numericHolder")},
		"text":    {ObjectValue(textHolder, "textHolder")},
	}
	result, err := Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute NameThenFilter: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
	if lazy.Materialized {
		t.Fatal("reading dotted evidence materialized the excluded lazy part")
	}
}
