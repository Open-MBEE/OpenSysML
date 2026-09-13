package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// objectBody is a session's worth of objects: a car holding an engine and four
// wheels, and a spare wheel held on its own, each with values a run may change.
const objectBody = `
part def Wheel {
	attribute pressure : Integer default 30;
}
part def Engine {
	attribute power : Integer default 100;
}
part def Car {
	part engine : Engine;
	part wheels : Wheel[4];
	attribute doors : Integer default 4;
}
part car : Car;
part spare : Wheel {
	attribute :>> pressure = 20;
}
`

const objectQueries = `
calc def Held :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 3),
		properties = ("qualifiedName", "name", "pressure")
	)
}
calc def Direct :> Query {
	in root : Element;
	Project(source = OwnedElements(source = root), properties = ("name", "owner", "@id"))
}
calc def Up :> Query {
	in root : Element;
	Project(source = Ancestors(source = root, maxDepth = 3), properties = ("qualifiedName", "type"))
}
calc def Wheels :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 3), type = "Wheel"),
			properties = ("qualifiedName", "pressure")
		),
		property = "pressure",
		direction = "descending",
		missing = "last",
		multiple = "first"
	)
}
calc def Soft :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = WhereType(source = Descendants(source = root, maxDepth = 3), type = "Wheel"),
			'feature' = "pressure",
			operator = "<",
			value = "30"
		),
		properties = ("qualifiedName")
	)
}
calc def Named :> Query {
	in root : Element;
	WhereName(source = Descendants(source = root, maxDepth = 3), operator = "=", value = "engine")
}
calc def Engines :> Query {
	in root : Element;
	Project(source = root, properties = ("qualifiedName", "engine", "doors"))
}
calc def EveryWheel :> Query {
	Project(source = Objects(type = "Wheel"), properties = ("qualifiedName", "pressure"))
}
calc def EveryPart :> Query {
	Objects(type = "PartUsage")
}
calc def Traced :> Query {
	in root : Element;
	RelatedElements(
		source = root,
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1
	)
}
`

type objectFixture struct {
	executionFixture
	ctx   *runtime.Context
	car   *runtime.Instance
	spare *runtime.Instance
}

func loadObjectFixture(t *testing.T) objectFixture {
	t.Helper()
	fixture := loadExecutionFixture(t, objectBody+objectQueries)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	spare, err := ctx.Instantiate(fixture.symbol(t, "spare"))
	if err != nil {
		t.Fatalf("Instantiate spare: %v", err)
	}
	return objectFixture{executionFixture: fixture, ctx: ctx, car: car, spare: spare}
}

func (f objectFixture) context() Context {
	return Context{
		Index: f.index, Resolver: f.resolver, Model: f.model, Runtime: f.ctx,
		Roots: []Root{{Label: "car", Object: f.car}, {Label: "spare", Object: f.spare}},
	}
}

func (f objectFixture) run(t *testing.T, name string, bindings Bindings) (*RowSet, error) {
	t.Helper()
	return Execute(f.program(t, name), f.context(), bindings, Options{})
}

func (f objectFixture) rows(t *testing.T, name string, bindings Bindings) *RowSet {
	t.Helper()
	result, err := f.run(t, name, bindings)
	if err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return result
}

func (f objectFixture) write(t *testing.T, inst *runtime.Instance, feature string, value int64) {
	t.Helper()
	err := inst.SetFeatureValue(f.ctx, feature, runtime.Value{
		Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: value},
	})
	if err != nil {
		t.Fatalf("write %s: %v", feature, err)
	}
}

// objectLabels renders the objects a result selected, in order.
// integerOf returns the single integer a cell holds.
func integerOf(values []Value) (int64, bool) {
	if len(values) != 1 {
		return 0, false
	}
	return values[0].Integer()
}

func objectLabels(t *testing.T, result *RowSet) []string {
	t.Helper()
	var out []string
	for _, row := range result.Rows() {
		_, label, ok := row.Element().Object()
		if !ok {
			t.Fatalf("row %v is not an object", row.Element().Kind())
		}
		out = append(out, label)
	}
	return out
}

func TestObjectValueCarriesInstanceLabelAndDeclaration(t *testing.T) {
	fixture := loadObjectFixture(t)
	value := ObjectValue(fixture.car, "Demo::car")
	if value.Kind() != ValueObject {
		t.Fatalf("kind = %s, want %s", value.Kind(), ValueObject)
	}
	inst, label, ok := value.Object()
	if !ok || inst != fixture.car || label != "Demo::car" {
		t.Fatalf("Object() = %v %q %v", inst, label, ok)
	}
	if _, ok := value.Element(); ok {
		t.Fatal("an object value is not an element")
	}
	if _, ok := value.String(); ok {
		t.Fatal("an object value is not a string")
	}
	if value.Origin() != provenance.Symbol(fixture.car.Type) {
		t.Fatalf("origin = %v, want the declaration car", value.Origin())
	}
	if _, _, ok := ObjectValue(nil, "gone").Object(); ok {
		t.Fatal("a nil instance is no object value")
	}
	if _, _, ok := ElementValue(fixture.symbol(t, "car")).Object(); ok {
		t.Fatal("an element value is no object value")
	}
}

func TestExecuteTraversesObjectsUnderTheirPaths(t *testing.T) {
	fixture := loadObjectFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}

	held := fixture.rows(t, "Held", root)
	wantPaths := []string{"car.engine", "car.wheels[1]", "car.wheels[2]", "car.wheels[3]", "car.wheels[4]"}
	if got := objectLabels(t, held); strings.Join(got, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("descendants = %v, want %v", got, wantPaths)
	}
	if got := cellTexts(t, held, 0); strings.Join(got, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("qualifiedName = %v, want %v", got, wantPaths)
	}
	if got := cellTexts(t, held, 1); strings.Join(got, ",") != "engine,wheels[1],wheels[2],wheels[3],wheels[4]" {
		t.Fatalf("name = %v", got)
	}
	// The engine has no pressure: absent, not an error.
	if values := held.Rows()[0].Cells()[2].Values(); len(values) != 0 {
		t.Fatalf("engine pressure = %v, want absent", values)
	}
	for i, row := range held.Rows()[1:] {
		values := row.Cells()[2].Values()
		if n, ok := integerOf(values); !ok || n != 30 {
			t.Fatalf("wheels[%d] pressure = %v, want 30", i+1, values)
		}
	}

	direct := fixture.rows(t, "Direct", root)
	if got := cellTexts(t, direct, 0); strings.Join(got, ",") != "engine,wheels[1],wheels[2],wheels[3],wheels[4]" {
		t.Fatalf("owned names = %v", got)
	}
	if got := cellTexts(t, direct, 1); got[0] != "car" || got[4] != "car" {
		t.Fatalf("owners = %v, want car", got)
	}
	if got := cellTexts(t, direct, 2); !strings.HasPrefix(got[0], "#") || got[0] == got[1] {
		t.Fatalf("ids = %v, want distinct session identities", got)
	}

	// Ancestors of a wheel: the car, reached under the path it was bound by.
	wheels, err := fixture.ctx.HeldObjects(fixture.car)
	if err != nil {
		t.Fatalf("held objects: %v", err)
	}
	up := fixture.rows(t, "Up", Bindings{"root": {ObjectValue(wheels[2].Instance, "car.wheels[2]")}})
	if got := cellTexts(t, up, 0); strings.Join(got, ",") != "car" {
		t.Fatalf("ancestors = %v, want car", got)
	}
	if got := cellTexts(t, up, 1); strings.Join(got, ",") != "Observatory::Car" {
		t.Fatalf("ancestor elementType = %v, want Observatory::Car", got)
	}
}

func TestExecuteReadsCurrentObjectValues(t *testing.T) {
	fixture := loadObjectFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	wheels, err := fixture.ctx.HeldObjects(fixture.car)
	if err != nil {
		t.Fatalf("held objects: %v", err)
	}
	fixture.write(t, wheels[1].Instance, "pressure", 12)
	fixture.write(t, wheels[3].Instance, "pressure", 45)

	ordered := fixture.rows(t, "Wheels", root)
	if got := cellTexts(t, ordered, 0); strings.Join(got, ",") != "car.wheels[3],car.wheels[2],car.wheels[4],car.wheels[1]" {
		t.Fatalf("wheels by pressure = %v", got)
	}
	if got := integerTexts(t, ordered, 1); got[0] != 45 || got[3] != 12 {
		t.Fatalf("pressures = %v, want 45..12", got)
	}
	soft := fixture.rows(t, "Soft", root)
	if got := cellTexts(t, soft, 0); strings.Join(got, ",") != "car.wheels[1]" {
		t.Fatalf("soft wheels = %v, want car.wheels[1]", got)
	}

	// A second execution reads the value written since, not a snapshot.
	fixture.write(t, wheels[1].Instance, "pressure", 60)
	if got := integerTexts(t, fixture.rows(t, "Wheels", root), 1); got[0] != 60 {
		t.Fatalf("pressures after write = %v, want 60 first", got)
	}

	named := fixture.rows(t, "Named", root)
	if got := objectLabels(t, named); strings.Join(got, ",") != "car.engine" {
		t.Fatalf("named engine = %v", got)
	}

	// Projecting a part feature keeps the object it holds as an object cell.
	engines := fixture.rows(t, "Engines", root)
	cell := engines.Rows()[0].Cells()[1].Values()
	if len(cell) != 1 {
		t.Fatalf("engine cell = %v, want one object", cell)
	}
	if inst, label, ok := cell[0].Object(); !ok || label != "car.engine" || inst.HeldUnder() != fixture.symbol(t, "Car::engine") {
		t.Fatalf("engine cell = %v %q %v", inst, label, ok)
	}
	if got := integerTexts(t, engines, 2); got[0] != 4 {
		t.Fatalf("doors = %v, want 4", got)
	}
}

func TestExecuteObjectsEnumeratesTheSession(t *testing.T) {
	fixture := loadObjectFixture(t)
	every := fixture.rows(t, "EveryWheel", nil)
	// Roots in the session's order, then the objects they hold.
	want := "spare,car.wheels[1],car.wheels[2],car.wheels[3],car.wheels[4]"
	if got := cellTexts(t, every, 0); strings.Join(got, ",") != want {
		t.Fatalf("Objects(Wheel) = %v, want %s", got, want)
	}
	if got := integerTexts(t, every, 1); got[0] != 20 || got[4] != 30 {
		t.Fatalf("pressures = %v, want 20 then 30s", got)
	}
	parts := fixture.rows(t, "EveryPart", nil)
	if got := objectLabels(t, parts); strings.Join(got, ",") != "car,spare,car.engine,car.wheels[1],car.wheels[2],car.wheels[3],car.wheels[4]" {
		t.Fatalf("Objects(PartUsage) = %v", got)
	}

	// Without a session there is nothing to enumerate: a typed error, not an empty result.
	_, err := Execute(fixture.program(t, "EveryWheel"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, nil, Options{})
	executionError(t, err, ErrorNoRuntime)

	_, err = Execute(fixture.program(t, "EveryWheel"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model,
			Roots: []Root{{Label: "car", Object: fixture.car}}}, nil, Options{})
	executionError(t, err, ErrorInvalidContext)
}

func TestExecuteRefusesObjectRowsWhereTheModelIsRead(t *testing.T) {
	fixture := loadObjectFixture(t)
	_, err := fixture.run(t, "Traced", Bindings{"root": {ObjectValue(fixture.car, "car")}})
	related := executionError(t, err, ErrorObjectRow)
	if related.Target != "car" || !strings.Contains(related.Error(), "not to object car") {
		t.Fatalf("object-row error = %v", related)
	}

	// An object bound to a query executed over the model alone is not a row.
	_, err = Execute(fixture.program(t, "Held"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model},
		Bindings{"root": {ObjectValue(fixture.car, "car")}}, Options{})
	executionError(t, err, ErrorInvalidArgument)

	// Element rows keep reading the model in the same execution.
	declared := fixture.rows(t, "Held", Bindings{"root": {ElementValue(fixture.symbol(t, "Car"))}})
	if got := cellTexts(t, declared, 0); strings.Join(got, ",") != "Observatory::Car::engine,Observatory::Car::wheels,Observatory::Car::doors" {
		t.Fatalf("declared descendants = %v", got)
	}
}
