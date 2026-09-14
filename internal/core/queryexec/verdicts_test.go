package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// verdictBody declares a car whose own assertions hold, decide nothing (an
// unvalued capacity) and fail on the objects it holds, a satisfaction about its
// nested engine, and verification cases about the engine's requirement.
const verdictBody = `
part def Injector {
	attribute rate : Real = 2.0;
	assert constraint ratePositive { rate > 0.0 }
}
part def Engine {
	attribute power : Real = 300.0;
	part injector : Injector;
	assert constraint powerLow { power < 200.0 }
}
part def Wheel {
	attribute pressure : Real default = 32.0;
	assert constraint pressureOk { pressure >= 30.0 }
}
part def Car {
	attribute mass : Real = 1500.0;
	attribute capacity : Real;
	part engine : Engine;
	part wheels : Wheel[3] {
		attribute :>> pressure = 20.0;
	}
	assert constraint massOk { mass < 2000.0 }
	assert constraint fits { mass <= capacity }
	requirement lightEnough {
		attribute m : Real = mass;
		require constraint { m < 1600.0 }
	}
}
requirement def PowerReq {
	subject e : Engine;
	require constraint { e.power > 100.0 }
}
part car : Car;
requirement strongEngine : PowerReq;
satisfy strongEngine by car.engine;
verification def PowerCheck {
	subject e : Engine;
	objective { verify strongEngine; }
	VerificationCases::PassIf(e.power > 100.0)
}
verification checkEngine : PowerCheck { subject e = car.engine; }
verification def Silent {
	subject e : Engine;
	objective { verify strongEngine; }
	action measure;
}
verification silentCheck : Silent { subject e = car.engine; }
part def Bare;
part bare : Bare;
`

const verdictQueries = `
calc def All :> Query {
	in root : Element;
	Project(
		source = Verdicts(source = root),
		properties = ("kind", "assertion", "path", "verdict", "condition", "reason", "verification")
	)
}
calc def Kinds :> Query {
	in root : Element;
	in kind : String;
	Project(source = Verdicts(source = root, kind = kind), properties = ("assertion", "path", "verdict"))
}
calc def Violated :> Query {
	in root : Element;
	Project(
		source = WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "=", value = "violated"),
		properties = ("path", "assertion", "reason")
	)
}
calc def ByPath :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = Verdicts(source = root, kind = "constraint"),
			property = "path",
			direction = "descending",
			missing = "last",
			multiple = "first"
		),
		properties = ("path", "name")
	)
}
calc def Requirements :> Query {
	in root : Element;
	Project(source = WhereType(source = Verdicts(source = root), type = "RequirementUsage"), properties = ("path", "verdict"))
}
calc def Carriers :> Query {
	in root : Element;
	Project(source = Verdicts(source = root, kind = "constraint"), properties = ("carrier", "qualifiedName"))
}
calc def NamedOk :> Query {
	in root : Element;
	Project(source = WhereName(source = Verdicts(source = root), operator = "ends-with", value = "Ok"), properties = ("path", "verdict"))
}
calc def Summary :> Query {
	in root : Element;
	Project(
		source = Verdicts(source = root, kind = "constraint"),
		columns = (Column(name = "summary", expression = Verdict::path + ": " + Verdict::verdict))
	)
}
calc def Verified :> Query {
	in root : Element;
	Project(
		source = Verdicts(source = root, kind = "verification"),
		columns = (Column(name = "verified", expression = Verdict::path + ": " + Verdict::'verification'))
	)
}
calc def Rows :> Query {
	in root : Element;
	Verdicts(source = root)
}
calc def Twice :> Query {
	in root : Element;
	Verdicts(source = Verdicts(source = root))
}
calc def Related :> Query {
	in root : Element;
	RelatedElements(source = Verdicts(source = root), relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1)
}
calc def Owned :> Query {
	in root : Element;
	OwnedElements(source = Verdicts(source = root))
}
calc def Wrong :> Query {
	in root : Element;
	Verdicts(source = root, kind = "maybe")
}
`

type verdictFixture struct {
	executionFixture
	ctx *runtime.Context
	car *runtime.Instance
}

func loadVerdictFixture(t *testing.T) verdictFixture {
	t.Helper()
	fixture := loadExecutionFixture(t, verdictBody+verdictQueries)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	return verdictFixture{executionFixture: fixture, ctx: ctx, car: car}
}

func (f verdictFixture) session() Context {
	return Context{
		Index: f.index, Resolver: f.resolver, Model: f.model, Runtime: f.ctx,
		Roots: []Root{{Label: "car", Object: f.car}},
	}
}

func (f verdictFixture) modelOnly() Context {
	return Context{Index: f.index, Resolver: f.resolver, Model: f.model}
}

func (f verdictFixture) rows(t *testing.T, context Context, name string, bindings Bindings) *RowSet {
	t.Helper()
	result, err := Execute(f.program(t, name), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return result
}

func (f verdictFixture) write(t *testing.T, inst *runtime.Instance, feature string, value float64) {
	t.Helper()
	err := inst.SetFeatureValue(f.ctx, feature, runtime.Value{
		Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: value},
	})
	if err != nil {
		t.Fatalf("write %s: %v", feature, err)
	}
}

// optionalTexts renders a column whose cells hold one string or nothing.
func optionalTexts(t *testing.T, result *RowSet, column int) []string {
	t.Helper()
	var out []string
	for _, row := range result.Rows() {
		values := row.Cells()[column].Values()
		switch len(values) {
		case 0:
			out = append(out, "")
		case 1:
			text, ok := values[0].String()
			if !ok {
				t.Fatalf("cell %d holds an unexpected %s", column, values[0].Kind())
			}
			out = append(out, text)
		default:
			t.Fatalf("cell %d has %d values", column, len(values))
		}
	}
	return out
}

// listTexts renders a column whose cells hold any number of strings, joined.
func listTexts(t *testing.T, result *RowSet, column int) []string {
	t.Helper()
	var out []string
	for _, row := range result.Rows() {
		var parts []string
		for _, value := range row.Cells()[column].Values() {
			text, ok := value.String()
			if !ok {
				t.Fatalf("cell %d holds an unexpected %s", column, value.Kind())
			}
			parts = append(parts, text)
		}
		out = append(out, strings.Join(parts, "+"))
	}
	return out
}

// The verdicts about a session object walk the object and every object it holds
// in the validation's order: the car's own assertions, then each held object's,
// the satisfaction about the engine grouped with the engine, and the
// verification cases verifying a requirement following its first verdict.
func TestExecuteVerdictsAboutASessionObject(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	all := fixture.rows(t, fixture.session(), "All", root)

	wantKinds := "constraint,constraint,requirement,constraint,satisfaction,verification,verification,constraint,constraint,constraint,constraint"
	if got := cellTexts(t, all, 0); strings.Join(got, ",") != wantKinds {
		t.Fatalf("kinds = %v\nwant %s", got, wantKinds)
	}
	wantPaths := "car,car,car,car.engine,car.engine,car.engine,car.engine,car.engine.injector,car.wheels[1],car.wheels[2],car.wheels[3]"
	if got := cellTexts(t, all, 2); strings.Join(got, ",") != wantPaths {
		t.Fatalf("paths = %v\nwant %s", got, wantPaths)
	}
	wantStatus := "holds,undecided,holds,violated,holds,holds,undecided,holds,violated,violated,violated"
	if got := cellTexts(t, all, 3); strings.Join(got, ",") != wantStatus {
		t.Fatalf("verdicts = %v\nwant %s", got, wantStatus)
	}
	assertions := all.Rows()
	if sym, ok := assertions[0].Cells()[1].Values()[0].Element(); !ok || sym.Name != "massOk" {
		t.Fatalf("first assertion = %v, want massOk", assertions[0].Cells()[1].Values())
	}
	if sym, ok := assertions[4].Cells()[1].Values()[0].Element(); !ok || sym.Name != "" {
		t.Fatalf("satisfaction assertion = %v, want the anonymous satisfy usage", assertions[4].Cells()[1].Values())
	}
	if sym, ok := assertions[5].Cells()[1].Values()[0].Element(); !ok || sym.Name != "checkEngine" {
		t.Fatalf("verification assertion = %v, want checkEngine", assertions[5].Cells()[1].Values())
	}

	conditions := optionalTexts(t, all, 4)
	if conditions[3] != "power < 200.0" || conditions[8] != "pressure >= 30.0" || conditions[0] != "" || conditions[1] != "" {
		t.Fatalf("conditions = %q", conditions)
	}
	reasons := optionalTexts(t, all, 5)
	if reasons[0] != "" || reasons[4] != "" {
		t.Fatalf("holding verdicts carry reasons: %q", reasons)
	}
	if !strings.Contains(reasons[1], "capacity") {
		t.Fatalf("undecided reason = %q, want the unvalued capacity named", reasons[1])
	}
	if !strings.Contains(reasons[3], "power < 200.0") {
		t.Fatalf("violated reason = %q, want the condition named", reasons[3])
	}
	if reasons[6] == "" {
		t.Fatalf("an inconclusive verification carries no reason: %q", reasons)
	}
	// Every verdict about strongEngine reports the kinds its verification cases answered.
	verifications := listTexts(t, all, 6)
	if verifications[4] != "pass+inconclusive" || verifications[5] != "pass" || verifications[6] != "inconclusive" {
		t.Fatalf("verifications = %q", verifications)
	}
	if verifications[0] != "" || verifications[2] != "" {
		t.Fatalf("assertions no case verifies report verifications: %q", verifications)
	}
}

func TestExecuteVerdictsSelectsAKind(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	for kind, want := range map[string]string{
		"constraint":   "car,car,car.engine,car.engine.injector,car.wheels[1],car.wheels[2],car.wheels[3]",
		"requirement":  "car",
		"satisfaction": "car.engine",
		"verification": "car.engine,car.engine",
		"all":          "car,car,car,car.engine,car.engine,car.engine,car.engine,car.engine.injector,car.wheels[1],car.wheels[2],car.wheels[3]",
	} {
		rows := fixture.rows(t, fixture.session(), "Kinds", Bindings{"root": root["root"], "kind": {StringValue(kind)}})
		if got := cellTexts(t, rows, 1); strings.Join(got, ",") != want {
			t.Fatalf("Verdicts(kind = %s) paths = %v, want %s", kind, got, want)
		}
	}
	_, err := Execute(fixture.program(t, "Wrong"), fixture.session(), root, Options{})
	if invalid := executionError(t, err, ErrorInvalidArgument); invalid.Parameter != "kind" {
		t.Fatalf("invalid kind error = %v", invalid)
	}
}

// Verdict rows go through the row operations by their own properties: filtered
// and ordered by verdict and path, typed and named by their assertion, and
// their carrier projected as the object checked.
func TestExecuteVerdictRowsThroughRowOperations(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	session := fixture.session()

	violated := fixture.rows(t, session, "Violated", root)
	if got := cellTexts(t, violated, 0); strings.Join(got, ",") != "car.engine,car.wheels[1],car.wheels[2],car.wheels[3]" {
		t.Fatalf("violated paths = %v", got)
	}
	byPath := fixture.rows(t, session, "ByPath", root)
	if got := cellTexts(t, byPath, 0); strings.Join(got, ",") != "car.wheels[3],car.wheels[2],car.wheels[1],car.engine.injector,car.engine,car,car" {
		t.Fatalf("constraints by path descending = %v", got)
	}
	if got := optionalTexts(t, byPath, 1); got[0] != "pressureOk" || got[3] != "ratePositive" {
		t.Fatalf("assertion names = %q", got)
	}
	// A satisfy usage is a requirement usage in the metamodel, so it is typed as one.
	requirements := fixture.rows(t, session, "Requirements", root)
	if got := cellTexts(t, requirements, 0); strings.Join(got, ",") != "car,car.engine" {
		t.Fatalf("WhereType(RequirementUsage) paths = %v, want lightEnough and the satisfaction", got)
	}
	named := fixture.rows(t, session, "NamedOk", root)
	if got := cellTexts(t, named, 0); strings.Join(got, ",") != "car,car.wheels[1],car.wheels[2],car.wheels[3]" {
		t.Fatalf("WhereName(ends-with Ok) paths = %v", got)
	}
	carriers := fixture.rows(t, session, "Carriers", root)
	cells := carriers.Rows()[2].Cells()
	if inst, label, ok := cells[0].Values()[0].Object(); !ok || label != "car.engine" || inst.Type != fixture.symbol(t, "Engine") {
		t.Fatalf("engine carrier = %v %q %v", inst, label, ok)
	}
	if got := cellTexts(t, carriers, 1); got[0] != "Observatory::Car::massOk" || got[3] != "Observatory::Injector::ratePositive" {
		t.Fatalf("assertion qualified names = %v", got)
	}
	summary := fixture.rows(t, session, "Summary", root)
	if got := cellTexts(t, summary, 0); got[3] != "car.engine.injector: holds" || got[4] != "car.wheels[1]: violated" {
		t.Fatalf("computed summaries = %v", got)
	}
	verified := fixture.rows(t, session, "Verified", root)
	if got := cellTexts(t, verified, 0); strings.Join(got, ",") != "car.engine: pass,car.engine: inconclusive" {
		t.Fatalf("computed verification column = %v", got)
	}

	// The result rows are verdict values declared by their assertion.
	rows := fixture.rows(t, session, "Rows", root)
	first := rows.Rows()[0].Element()
	verdict, ok := first.Verdict()
	if !ok || first.Kind() != ValueVerdict {
		t.Fatalf("row = %s, want a verdict", first.Kind())
	}
	if verdict.Assertion() != first.Declaration() || verdict.Assertion().Name != "massOk" {
		t.Fatalf("declaration = %v, want massOk", first.Declaration())
	}
	if carrier, held := verdict.Carrier(); carrier != fixture.car || !held {
		t.Fatalf("carrier = %v %v, want the held car", carrier, held)
	}
	if verdict.Summary() != "assert constraint massOk on car: holds" {
		t.Fatalf("summary = %q", verdict.Summary())
	}
	if _, ok := first.Element(); ok {
		t.Fatal("a verdict is not an element value")
	}
	if _, _, ok := first.Object(); ok {
		t.Fatal("a verdict is not an object value")
	}

	// A verdict is about an object; it is neither checked again, traced nor walked.
	_, err := Execute(fixture.program(t, "Twice"), session, root, Options{})
	if refused := executionError(t, err, ErrorVerdictRow); !strings.Contains(refused.Error(), "not to verdict assert constraint massOk on car") {
		t.Fatalf("verdict-row error = %v", refused)
	}
	for _, name := range []string{"Related", "Owned"} {
		_, err = Execute(fixture.program(t, name), session, root, Options{})
		executionError(t, err, ErrorVerdictRow)
	}
}

// Verdicts read the objects as they stand: a value written since changes them.
func TestExecuteVerdictsReadCurrentValues(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	held, err := fixture.ctx.HeldObjects(fixture.car)
	if err != nil {
		t.Fatalf("held objects: %v", err)
	}
	fixture.write(t, held[0].Instance, "power", 150.0)
	fixture.write(t, fixture.car, "capacity", 1400.0)
	all := fixture.rows(t, fixture.session(), "All", root)
	got := cellTexts(t, all, 3)
	if got[1] != "violated" || got[3] != "holds" || got[4] != "holds" {
		t.Fatalf("verdicts after writes = %v", got)
	}
	if got := optionalTexts(t, all, 5); !strings.Contains(got[1], "mass <= capacity") {
		t.Fatalf("fits reason = %q", got[1])
	}
}

// Asking again answers the same rows, and a validation after agrees with them:
// checking the object leaves no assertion restated about it.
func TestExecuteVerdictsAnswerTheSameRowsAgain(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ObjectValue(fixture.car, "car")}}
	first := fixture.rows(t, fixture.session(), "All", root)
	again := fixture.rows(t, fixture.session(), "All", root)
	for _, column := range []int{0, 2, 3} {
		before, after := cellTexts(t, first, column), cellTexts(t, again, column)
		if strings.Join(before, ",") != strings.Join(after, ",") {
			t.Fatalf("column %d changed between queries:\n%v\n%v", column, before, after)
		}
	}
	var scopes []*symbols.Scope
	for _, name := range fixture.index.WorkspaceDocuments() {
		scopes = append(scopes, fixture.index.DocumentRoot(name))
	}
	report, err := fixture.ctx.ValidateObject(fixture.car, scopes)
	if err != nil {
		t.Fatalf("validate after querying: %v", err)
	}
	if want := len(first.Rows()) - 2; len(report.Verdicts) != want {
		t.Fatalf("validation after querying has %d verdicts, want %d", len(report.Verdicts), want)
	}
}

// An element row is checked as the model declares it, in the session and
// without one; an element declaring no object is a typed error.
func TestExecuteVerdictsAboutADeclaredElement(t *testing.T) {
	fixture := loadVerdictFixture(t)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "car"))}}
	for name, context := range map[string]Context{"session": fixture.session(), "model": fixture.modelOnly()} {
		all := fixture.rows(t, context, "All", root)
		wantPaths := "Observatory::car,Observatory::car,Observatory::car,Observatory::car.engine,Observatory::car.engine," +
			"Observatory::car.engine,Observatory::car.engine,Observatory::car.engine.injector," +
			"Observatory::car.wheels[1],Observatory::car.wheels[2],Observatory::car.wheels[3]"
		if got := cellTexts(t, all, 2); strings.Join(got, ",") != wantPaths {
			t.Fatalf("%s: declared paths = %v", name, got)
		}
		if got := cellTexts(t, all, 3); strings.Join(got, ",") != "holds,undecided,holds,violated,holds,holds,undecided,holds,violated,violated,violated" {
			t.Fatalf("%s: declared verdicts = %v", name, got)
		}
		// The carrier of a declared check is the element checked, not a session object.
		carriers := fixture.rows(t, context, "Carriers", root)
		if sym, ok := carriers.Rows()[2].Cells()[0].Values()[0].Element(); !ok || sym != fixture.symbol(t, "Car::engine") {
			t.Fatalf("%s: engine carrier = %v, want the declared part", name, carriers.Rows()[2].Cells()[0].Values())
		}
		rows := fixture.rows(t, context, "Rows", root)
		verdict, _ := rows.Rows()[0].Element().Verdict()
		if carrier, held := verdict.Carrier(); carrier == nil || held {
			t.Fatalf("%s: declared carrier = %v %v, want an unheld object", name, carrier, held)
		}

		// A definition is checked as declared too; a package declares no object.
		definition := fixture.rows(t, context, "Kinds",
			Bindings{"root": {ElementValue(fixture.symbol(t, "Wheel"))}, "kind": {StringValue("constraint")}})
		if got := cellTexts(t, definition, 2); strings.Join(got, ",") != "holds" {
			t.Fatalf("%s: Wheel as declared = %v, want its default pressure to hold", name, got)
		}
		_, err := Execute(fixture.program(t, "Rows"), context,
			Bindings{"root": {ElementValue(fixture.symbol(t, "Car::capacity"))}}, Options{})
		if notObject := executionError(t, err, ErrorNotAnObject); notObject.Target != "Observatory::Car::capacity" {
			t.Fatalf("%s: not-an-object error = %v", name, notObject)
		}
		// An object stating no assertion has no verdicts: an empty result, not an error.
		if bare := fixture.rows(t, context, "Rows", Bindings{"root": {ElementValue(fixture.symbol(t, "bare"))}}); len(bare.Rows()) != 0 {
			t.Fatalf("%s: bare verdicts = %d rows, want none", name, len(bare.Rows()))
		}
	}
}

// A walk that cannot reach every held object — a part holding another of its own
// type without end — is a typed error rather than a table missing its rows.
func TestExecuteVerdictsRefuseAnIncompleteWalk(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Link {
	attribute load : Real = 1.0;
	part next : Link;
	assert constraint loaded { load > 0.0 }
}
part chain : Link;
calc def Rows :> Query {
	in root : Element;
	Verdicts(source = root)
}
`)
	_, err := Execute(fixture.program(t, "Rows"), Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model},
		Bindings{"root": {ElementValue(fixture.symbol(t, "chain"))}}, Options{})
	incomplete := executionError(t, err, ErrorIncompleteValidation)
	if incomplete.Target != "Observatory::chain" || !strings.Contains(incomplete.Error(), "not checked") {
		t.Fatalf("incomplete-validation error = %v", incomplete)
	}
}
