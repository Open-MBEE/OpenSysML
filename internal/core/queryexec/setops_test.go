package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

const setQueries = `
calc def Difference :> Query {
	in source : Element[0..*] ordered;
	in exclude : Element[0..*] ordered;
	Except(source = source, exclude = exclude)
}
calc def Combined :> Query {
	in source : Element[0..*] ordered;
	in other : Element[0..*] ordered;
	Union(source = source, other = other)
}
calc def Uncovered :> Query {
	in root : Element;
	Union(
		source = Unsatisfied(root = root),
		other = Unverified(root = root)
	)
}
calc def UnsatisfiedByDifference :> Query {
	in root : Element;
	Except(
		source = RequirementUsages(root = root),
		exclude = Satisfied(root = root)
	)
}
calc def NamedDifference :> Query {
	in root : Element;
	Except(
		source = Project(source = RequirementUsages(root = root), properties = ("name")),
		exclude = Satisfied(root = root)
	)
}
calc def NamedUnion :> Query {
	in root : Element;
	Union(
		source = Project(source = Unsatisfied(root = root), properties = ("name")),
		other = Project(source = Unverified(root = root), properties = ("name"))
	)
}
calc def MismatchedUnion :> Query {
	in root : Element;
	Union(
		source = Project(source = Unsatisfied(root = root), properties = ("name")),
		other = Unverified(root = root)
	)
}
calc def WheelsAndParts :> Query {
	Union(source = Objects(type = "Wheel"), other = Objects(type = "PartUsage"))
}
calc def PartsButWheels :> Query {
	Except(source = Objects(type = "PartUsage"), exclude = Objects(type = "Wheel"))
}
calc def DeclaredAndHeld :> Query {
	in root : Element;
	Union(source = WhereType(source = Descendants(source = root, maxDepth = 3), type = "Wheel"), other = Objects(type = "Wheel"))
}
`

func TestExecuteSetOperationsOverElements(t *testing.T) {
	fixture := loadExecutionFixture(t, coverageBody+coverageQueries+setQueries)
	subsystem := ElementValue(fixture.symbol(t, "Subsystem"))
	optical := ElementValue(fixture.symbol(t, "OpticalSubsystem"))
	mirror := ElementValue(fixture.symbol(t, "MirrorAssembly"))
	telescope := ElementValue(fixture.symbol(t, "telescope"))

	// Except keeps source order, drops every occurrence of an excluded identity
	// and emits a repeated source identity once.
	difference, err := fixture.execute(t, "Difference", Bindings{
		"source":  {mirror, subsystem, optical, mirror, subsystem, telescope},
		"exclude": {telescope, mirror},
	}, Options{})
	if err != nil {
		t.Fatalf("Difference: %v", err)
	}
	if got := strings.Join(rowNames(difference), ","); got != "Observatory::Subsystem,Observatory::OpticalSubsystem" {
		t.Fatalf("Except = %v", got)
	}
	// Union emits the source rows first, then the unseen rows of other, once each.
	combined, err := fixture.execute(t, "Combined", Bindings{
		"source": {mirror, subsystem, mirror},
		"other":  {telescope, subsystem, optical, telescope},
	}, Options{})
	if err != nil {
		t.Fatalf("Combined: %v", err)
	}
	want := "Observatory::MirrorAssembly,Observatory::Subsystem,Observatory::telescope,Observatory::OpticalSubsystem"
	if got := strings.Join(rowNames(combined), ","); got != want {
		t.Fatalf("Union = %v", got)
	}
	empty, err := fixture.execute(t, "Combined", Bindings{"source": {}, "other": {}}, Options{})
	if err != nil || len(empty.Rows()) != 0 {
		t.Fatalf("Union of nothing = %v, %v", rowNames(empty), err)
	}
	for _, row := range combined.Rows() {
		if !row.Origin().Located() {
			t.Fatalf("row %v without provenance", row.Element())
		}
	}
}

func TestExecuteSetOperationsComposeCoverage(t *testing.T) {
	fixture := loadExecutionFixture(t, coverageBody+coverageQueries+setQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Specification"))}}

	// The requirements nothing satisfies, whether filtered or subtracted.
	unsatisfied, err := fixture.execute(t, "Unsatisfied", root, Options{})
	if err != nil {
		t.Fatalf("Unsatisfied: %v", err)
	}
	subtracted, err := fixture.execute(t, "UnsatisfiedByDifference", root, Options{})
	if err != nil {
		t.Fatalf("UnsatisfiedByDifference: %v", err)
	}
	if got, want := strings.Join(rowNames(subtracted), ","), strings.Join(rowNames(unsatisfied), ","); got != want {
		t.Fatalf("Except(all, satisfied) = %v, want %v", got, want)
	}
	// Either gap, each requirement once, unsatisfied first.
	uncovered, err := fixture.execute(t, "Uncovered", root, Options{})
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	want := "Observatory::Specification::pointingRequirement,Observatory::Specification::thermalRequirement," +
		"Observatory::Specification::thermalRequirement::coolantRequirement,Observatory::Specification::dataRequirement"
	if got := strings.Join(rowNames(uncovered), ","); got != want {
		t.Fatalf("Union(unsatisfied, unverified) = %v", got)
	}
}

func TestExecuteSetOperationsKeepProjectedColumns(t *testing.T) {
	fixture := loadExecutionFixture(t, coverageBody+coverageQueries+setQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Specification"))}}

	// Except compares identities beneath the projection and keeps the source's columns.
	named, err := fixture.execute(t, "NamedDifference", root, Options{})
	if err != nil {
		t.Fatalf("NamedDifference: %v", err)
	}
	if got := projectedNames(t, named); strings.Join(got, ",") != "pointingRequirement,thermalRequirement,coolantRequirement" {
		t.Fatalf("projected Except = %v", got)
	}
	union, err := fixture.execute(t, "NamedUnion", root, Options{})
	if err != nil {
		t.Fatalf("NamedUnion: %v", err)
	}
	if got := projectedNames(t, union); strings.Join(got, ",") != "pointingRequirement,thermalRequirement,coolantRequirement,dataRequirement" {
		t.Fatalf("projected Union = %v", got)
	}
	// Rows of different shapes cannot share one table.
	_, err = fixture.execute(t, "MismatchedUnion", root, Options{})
	if mismatch := executionError(t, err, ErrorInvalidArgument); mismatch.Parameter != "other" || mismatch.Actual != "unprojected row set" {
		t.Fatalf("mismatched Union = %v", mismatch)
	}
}

func TestExecuteSetOperationsOverSessionObjects(t *testing.T) {
	fixture := loadExecutionFixture(t, objectBody+setQueries)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	spare, err := ctx.Instantiate(fixture.symbol(t, "spare"))
	if err != nil {
		t.Fatalf("Instantiate spare: %v", err)
	}
	session := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "car", Object: car}, {Label: "spare", Object: spare}},
	}
	run := func(name string, bindings Bindings) *RowSet {
		t.Helper()
		result, err := Execute(fixture.program(t, name), session, bindings, Options{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return result
	}

	// Objects are identified by instance, so every wheel appears once.
	union := run("WheelsAndParts", nil)
	wantUnion := "spare,car.wheels[1],car.wheels[2],car.wheels[3],car.wheels[4],car,car.engine"
	if got := strings.Join(objectLabels(t, union), ","); got != wantUnion {
		t.Fatalf("Union over objects = %v", got)
	}
	except := run("PartsButWheels", nil)
	if got := strings.Join(objectLabels(t, except), ","); got != "car,car.engine" {
		t.Fatalf("Except over objects = %v", got)
	}
	// A declared wheel and the objects instantiated from it are distinct rows.
	mixed := run("DeclaredAndHeld", Bindings{"root": {ElementValue(fixture.symbol(t, "Car"))}})
	rows := mixed.Rows()
	if len(rows) != 6 {
		t.Fatalf("mixed Union has %d rows", len(rows))
	}
	if sym, ok := rows[0].Element().Element(); !ok || sym != fixture.symbol(t, "Car::wheels") {
		t.Fatalf("first mixed row = %v", rows[0].Element())
	}
	if got := strings.Join(objectLabels(t, &RowSet{rows: rows[1:]}), ","); got != "spare,car.wheels[1],car.wheels[2],car.wheels[3],car.wheels[4]" {
		t.Fatalf("mixed Union objects = %v", got)
	}
	wheels := Bindings{"source": {ObjectValue(car, "car"), ObjectValue(spare, "spare")}, "exclude": {ElementValue(fixture.symbol(t, "spare"))}}
	if got := strings.Join(objectLabels(t, run("Difference", wheels)), ","); got != "car,spare" {
		t.Fatalf("Except of objects by their declaration = %v", got)
	}
}

// Verdict rows take part by assertion and carrier: the same assertion checked
// on three wheels stays three rows, and asking twice adds none.
func TestExecuteSetOperationsOverVerdicts(t *testing.T) {
	fixture := loadExecutionFixture(t, verdictBody+verdictQueries+setQueries+`
calc def AllTwice :> Query {
	in root : Element;
	Union(source = Verdicts(source = root), other = Verdicts(source = root))
}
calc def NotConstraints :> Query {
	in root : Element;
	Project(
		source = Except(source = Verdicts(source = root), exclude = Verdicts(source = root, kind = "constraint")),
		properties = ("kind", "path")
	)
}
`)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	session := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "car", Object: car}},
	}
	modelOnly := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	cases := map[string]struct {
		context Context
		root    Value
	}{
		"session": {session, ObjectValue(car, "car")},
		"model":   {modelOnly, ElementValue(fixture.symbol(t, "car"))},
	}
	for name, tc := range cases {
		root := Bindings{"root": {tc.root}}
		twice, err := Execute(fixture.program(t, "AllTwice"), tc.context, root, Options{})
		if err != nil {
			t.Fatalf("%s AllTwice: %v", name, err)
		}
		if got := len(twice.Rows()); got != 11 {
			t.Fatalf("%s: Union of the same verdicts has %d rows, want 11", name, got)
		}
		rest, err := Execute(fixture.program(t, "NotConstraints"), tc.context, root, Options{})
		if err != nil {
			t.Fatalf("%s NotConstraints: %v", name, err)
		}
		if got := cellTexts(t, rest, 0); strings.Join(got, ",") != "requirement,satisfaction,verification,verification" {
			t.Fatalf("%s: verdicts but constraints = %v", name, got)
		}
	}
}

// State rows are one identity per object, machine and state path; event rows
// one per trace record. Union keeps every distinct row, Except drops only the
// rows named.
func TestExecuteSetOperationsOverStatesAndEvents(t *testing.T) {
	fixture := loadLampFixture(t, `
calc def StatesTwice :> Query {
	in root : Element;
	Project(source = Union(source = States(source = root), other = States(source = root)), properties = ("path", "statePath"))
}
calc def StatesBut :> Query {
	in root : Element;
	in exclude : Element;
	Project(source = Except(source = States(source = root), exclude = States(source = exclude)), properties = ("path", "statePath"))
}
calc def AcceptsTwice :> Query {
	in root : Element;
	Project(source = Union(source = Events(source = root, kind = "accept"), other = Events(source = root, kind = "accept")), properties = ("time", "path", "event"))
}
calc def LaterAccepts :> Query {
	in root : Element;
	Project(
		source = Except(source = Events(source = root, kind = "accept"), exclude = Events(source = root, kind = "accept", before = 2 [s])),
		properties = ("time", "path", "event")
	)
}
`)
	lamps := Bindings{"root": {ElementValue(fixture.symbol(t, "Lamp"))}}
	got := rowTexts(t, fixture.rows(t, "StatesTwice", lamps))
	want := []string{
		"path=lamp1 statePath=on.dim",
		"path=lamp1 statePath=on.fast",
		"path=lamp2 statePath=off",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("Union of the same states:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "StatesBut", Bindings{
		"root":    {ElementValue(fixture.symbol(t, "Lamp"))},
		"exclude": {ObjectValue(fixture.lamp1, "lamp1")},
	}))
	if joinLines(got) != "path=lamp2 statePath=off" {
		t.Fatalf("lamp states but lamp1's:\n%s", joinLines(got))
	}

	got = rowTexts(t, fixture.rows(t, "AcceptsTwice", lamps))
	want = []string{
		"time=0.0 [s] path=lamp1 event=Toggle",
		"time=1.0 [s] path=lamp1 event=Dim",
		"time=2.0 [s] path=lamp2 event=Toggle",
		"time=2.5 [s] path=lamp2 event=Toggle",
		"time=2.5 [s] path=lamp1 event=Boost",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("Union of the same accepts:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "LaterAccepts", lamps))
	if joinLines(got) != joinLines(want[2:]) {
		t.Fatalf("accepts from 2 s:\n%s\nwant:\n%s", joinLines(got), joinLines(want[2:]))
	}
}
