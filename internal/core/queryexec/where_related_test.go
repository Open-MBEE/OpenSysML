package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// coverageBody declares requirements with every coverage state: satisfied and
// verified, verified only, satisfied only, nested and covered by nothing.
const coverageBody = `
part def Subsystem;
part def OpticalSubsystem :> Subsystem;
part def MirrorAssembly :> OpticalSubsystem;
part telescope : MirrorAssembly;
part def Wheel;
part def Car {
	part wheels : Wheel[2];
}
part car : Car;

requirement def Traced {
	requirement nested;
}
package Specification {
	requirement massRequirement;
	requirement pointingRequirement;
	requirement dataRequirement : Traced;
	requirement thermalRequirement {
		requirement coolantRequirement;
	}
}
part observatory {
	satisfy Specification::massRequirement by telescope;
	satisfy Specification::dataRequirement by car;
}
verification def MassTest;
verification massVerification : MassTest {
	objective { verify Specification::massRequirement; }
}
verification pointingVerification : MassTest {
	objective { verify Specification::pointingRequirement; }
}
`

const coverageQueries = `
calc def RequirementUsages :> Query {
	in root : Element;
	WhereType(source = Descendants(source = root, maxDepth = 3), type = "RequirementUsage")
}
calc def Unsatisfied :> Query {
	in root : Element;
	WhereRelated(
		source = RequirementUsages(root = root),
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1,
		exists = false
	)
}
calc def Satisfied :> Query {
	in root : Element;
	WhereRelated(
		source = RequirementUsages(root = root),
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1
	)
}
calc def Unverified :> Query {
	in root : Element;
	WhereRelated(RequirementUsages(root = root), "verification", "incoming", 1, false)
}
calc def Filtered :> Query {
	in source : Element[0..*] ordered;
	in kind : String;
	in direction : String;
	in maxDepth : Integer;
	in exists : Boolean;
	WhereRelated(
		source = source,
		relationshipKind = kind,
		direction = direction,
		maxDepth = maxDepth,
		exists = exists
	)
}
calc def UnsatisfiedTable :> Query {
	in root : Element;
	WhereRelated(
		source = Project(source = RequirementUsages(root = root), properties = ("name", "qualifiedName")),
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1,
		exists = false
	)
}
`

func loadCoverageFixture(t *testing.T) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, coverageBody+coverageQueries)
}

func (f executionFixture) filtered(
	t *testing.T,
	source []Value,
	kind, direction string,
	maxDepth int64,
	exists bool,
	options Options,
) (*RowSet, error) {
	t.Helper()
	return f.execute(t, "Filtered", Bindings{
		"source":    source,
		"kind":      {StringValue(kind)},
		"direction": {StringValue(direction)},
		"maxDepth":  {IntegerValue(maxDepth)},
		"exists":    {BooleanValue(exists)},
	}, options)
}

func TestExecuteWhereRelatedKeepsRowsByRelationshipExistence(t *testing.T) {
	fixture := loadCoverageFixture(t)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Specification"))}}

	all, err := fixture.execute(t, "RequirementUsages", root, Options{})
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}
	// Nested requirement usages are requirement usages too.
	wantAll := "Observatory::Specification::massRequirement,Observatory::Specification::pointingRequirement," +
		"Observatory::Specification::dataRequirement,Observatory::Specification::thermalRequirement," +
		"Observatory::Specification::thermalRequirement::coolantRequirement"
	if got := strings.Join(rowNames(all), ","); got != wantAll {
		t.Fatalf("requirement usages = %v", got)
	}

	unsatisfied, err := fixture.execute(t, "Unsatisfied", root, Options{})
	if err != nil {
		t.Fatalf("Unsatisfied: %v", err)
	}
	wantUnsatisfied := "Observatory::Specification::pointingRequirement,Observatory::Specification::thermalRequirement," +
		"Observatory::Specification::thermalRequirement::coolantRequirement"
	if got := strings.Join(rowNames(unsatisfied), ","); got != wantUnsatisfied {
		t.Fatalf("unsatisfied = %v", got)
	}
	// exists defaults to true and keeps the complement, in source order.
	satisfied, err := fixture.execute(t, "Satisfied", root, Options{})
	if err != nil {
		t.Fatalf("Satisfied: %v", err)
	}
	if got := strings.Join(rowNames(satisfied), ","); got != "Observatory::Specification::massRequirement,Observatory::Specification::dataRequirement" {
		t.Fatalf("satisfied = %v", got)
	}
	unverified, err := fixture.execute(t, "Unverified", root, Options{})
	if err != nil {
		t.Fatalf("Unverified: %v", err)
	}
	wantUnverified := "Observatory::Specification::dataRequirement,Observatory::Specification::thermalRequirement," +
		"Observatory::Specification::thermalRequirement::coolantRequirement"
	if got := strings.Join(rowNames(unverified), ","); got != wantUnverified {
		t.Fatalf("unverified = %v", got)
	}
	for _, row := range unsatisfied.Rows() {
		if !row.Origin().Located() {
			t.Fatalf("row %v without provenance", row.Element())
		}
	}
}

func TestExecuteWhereRelatedHonoursDepthAndEveryKind(t *testing.T) {
	fixture := loadCoverageFixture(t)
	lineage := []Value{
		ElementValue(fixture.symbol(t, "Subsystem")),
		ElementValue(fixture.symbol(t, "OpticalSubsystem")),
		ElementValue(fixture.symbol(t, "MirrorAssembly")),
	}
	roots, err := fixture.filtered(t, lineage, "specialization", "outgoing", 1, false, Options{})
	if err != nil {
		t.Fatalf("specialization exists=false: %v", err)
	}
	if got := rowNames(roots); len(got) != 1 || got[0] != "Observatory::Subsystem" {
		t.Fatalf("definitions specializing nothing = %v", got)
	}
	// Incoming typing within one edge: only Subsystem's direct type users count,
	// so MirrorAssembly is typed by telescope and the others by nothing.
	typed, err := fixture.filtered(t, lineage, "typing", "incoming", 1, true, Options{})
	if err != nil {
		t.Fatalf("typing exists=true: %v", err)
	}
	if got := rowNames(typed); len(got) != 1 || got[0] != "Observatory::MirrorAssembly" {
		t.Fatalf("definitions typing a usage = %v", got)
	}
	// Nothing is reachable within depth 0, so every row lacks a related element.
	general := []Value{ElementValue(fixture.symbol(t, "OpticalSubsystem"))}
	none, err := fixture.filtered(t, general, "specialization", "outgoing", 0, true, Options{})
	if err != nil {
		t.Fatalf("depth 0 exists=true: %v", err)
	}
	if len(none.Rows()) != 0 {
		t.Fatalf("depth 0 kept %v", rowNames(none))
	}
	unreached, err := fixture.filtered(t, general, "specialization", "outgoing", 0, false, Options{})
	if err != nil {
		t.Fatalf("depth 0 exists=false: %v", err)
	}
	if got := rowNames(unreached); len(got) != 1 || got[0] != "Observatory::OpticalSubsystem" {
		t.Fatalf("depth 0 exists=false = %v", got)
	}
	for _, kind := range []string{"subsetting", "redefinition", "connection", "allocation"} {
		none, err := fixture.filtered(t, general, kind, "outgoing", 1, false, Options{})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if got := rowNames(none); len(got) != 1 || got[0] != "Observatory::OpticalSubsystem" {
			t.Fatalf("%s exists=false = %v", kind, got)
		}
	}
}

func TestExecuteWhereRelatedKeepsProjectedColumns(t *testing.T) {
	fixture := loadCoverageFixture(t)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Specification"))}}
	table, err := fixture.execute(t, "UnsatisfiedTable", root, Options{})
	if err != nil {
		t.Fatalf("UnsatisfiedTable: %v", err)
	}
	var columns []string
	for _, column := range table.Columns() {
		columns = append(columns, column.Name())
	}
	if got := strings.Join(columns, ","); got != "name,qualifiedName" {
		t.Fatalf("columns = %v", got)
	}
	if got := cellTexts(t, table, 0); strings.Join(got, ",") != "pointingRequirement,thermalRequirement,coolantRequirement" {
		t.Fatalf("names = %v", got)
	}
}

func TestExecuteWhereRelatedReportsTheErrorsOfRelatedElements(t *testing.T) {
	fixture := loadCoverageFixture(t)
	source := []Value{ElementValue(fixture.symbol(t, "Subsystem"))}
	_, err := fixture.filtered(t, source, "ownership", "outgoing", 1, true, Options{})
	if unknown := executionError(t, err, ErrorUnknownRelationship); unknown.Actual != "ownership" {
		t.Fatalf("unknown relationship = %v", unknown)
	}
	_, err = fixture.filtered(t, source, "specialization", "sideways", 1, true, Options{})
	executionError(t, err, ErrorInvalidOperator)

	// An object row cannot be traced through the model's relationships.
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	_, err = Execute(fixture.program(t, "Filtered"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
			Roots: []Root{{Label: "car", Object: car}}},
		Bindings{
			"source":    {ObjectValue(car, "car")},
			"kind":      {StringValue("satisfaction")},
			"direction": {StringValue("incoming")},
			"maxDepth":  {IntegerValue(1)},
			"exists":    {BooleanValue(true)},
		}, Options{})
	if refused := executionError(t, err, ErrorObjectRow); refused.Target != "car" {
		t.Fatalf("object-row error = %v", refused)
	}
}

func TestExecuteWhereRelatedChargesTheVisitBudget(t *testing.T) {
	fixture := loadCoverageFixture(t)
	leaf := []Value{ElementValue(fixture.symbol(t, "MirrorAssembly"))}
	// An existence check stops at the first element reached, so one visit suffices.
	kept, err := fixture.filtered(t, leaf, "specialization", "outgoing", 3, true, Options{VisitBudget: 1})
	if err != nil {
		t.Fatalf("exists=true within budget: %v", err)
	}
	if got := rowNames(kept); len(got) != 1 || got[0] != "Observatory::MirrorAssembly" {
		t.Fatalf("kept = %v", got)
	}
	// Each row's check charges the shared budget, so a second row exhausts it.
	pair := append(leaf, ElementValue(fixture.symbol(t, "OpticalSubsystem")))
	_, err = fixture.filtered(t, pair, "specialization", "outgoing", 3, true, Options{VisitBudget: 1})
	executionError(t, err, ErrorVisitBudget)
	// Building an edge table charges each declaration scanned, as RelatedElements does.
	_, err = fixture.filtered(t, leaf, "satisfaction", "incoming", 1, false, Options{VisitBudget: 3})
	executionError(t, err, ErrorVisitBudget)
}
