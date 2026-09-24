package queryexec

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func (f executionFixture) related(
	t *testing.T,
	source *symbols.Symbol,
	kind, direction string,
	maxDepth int64,
	options Options,
) (*RowSet, error) {
	t.Helper()
	return f.execute(t, "Related", Bindings{
		"source":    {ElementValue(source)},
		"kind":      {StringValue(kind)},
		"direction": {StringValue(direction)},
		"maxDepth":  {IntegerValue(maxDepth)},
	}, options)
}

func rowNames(result *RowSet) []string {
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, symbols.FQNOf(sym))
	}
	return names
}

func assertRelated(
	t *testing.T,
	fixture executionFixture,
	source, kind, direction string,
	maxDepth int64,
	want []string,
) {
	t.Helper()
	result, err := fixture.related(t, fixture.symbol(t, source), kind, direction, maxDepth, Options{})
	if err != nil {
		t.Fatalf("%s %s from %s: %v", kind, direction, source, err)
	}
	names := rowNames(result)
	if len(names) != len(want) {
		t.Fatalf("%s %s from %s = %v, want %v", kind, direction, source, names, want)
	}
	for i, name := range want {
		if names[i] != "Observatory::"+name {
			t.Fatalf("%s %s from %s = %v, want %v", kind, direction, source, names, want)
		}
	}
	for _, row := range result.Rows() {
		if !row.Origin().Located() {
			t.Fatalf("%s %s from %s: row without provenance", kind, direction, source)
		}
	}
}

func TestExecuteRelatedLineageBothDirectionsAndDepth(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")

	assertRelated(t, fixture, "MirrorAssembly", "specialization", "outgoing", 1,
		[]string{"OpticalSubsystem"})
	assertRelated(t, fixture, "MirrorAssembly", "specialization", "outgoing", 3,
		[]string{"OpticalSubsystem", "Subsystem"})
	assertRelated(t, fixture, "Subsystem", "specialization", "incoming", 1,
		[]string{"OpticalSubsystem"})
	assertRelated(t, fixture, "Subsystem", "specialization", "incoming", 2,
		[]string{"OpticalSubsystem", "MirrorAssembly"})
	// A definition with no generalizations traverses to a valid empty result.
	assertRelated(t, fixture, "Subsystem", "specialization", "outgoing", 3, nil)

	assertRelated(t, fixture, "iris", "subsetting", "outgoing", 1, []string{"instruments"})
	assertRelated(t, fixture, "instruments", "subsetting", "incoming", 1,
		[]string{"iris", "modhis"})

	assertRelated(t, fixture, "MirrorAssembly::mass", "redefinition", "outgoing", 1,
		[]string{"Subsystem::mass"})
	assertRelated(t, fixture, "Subsystem::mass", "redefinition", "incoming", 1,
		[]string{"MirrorAssembly::mass"})

	assertRelated(t, fixture, "telescope::primaryMirror", "typing", "outgoing", 1,
		[]string{"MirrorAssembly"})
	assertRelated(t, fixture, "Subsystem", "typing", "incoming", 1,
		[]string{"telescope::instrumentCluster", "telescope::mountControl"})
}

func TestExecuteRelatedSeedsAreDeduplicatedBySemanticIdentity(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")
	result, err := fixture.execute(t, "Related", Bindings{
		"source": {
			ElementValue(fixture.symbol(t, "MirrorAssembly")),
			ElementValue(fixture.symbol(t, "OpticalSubsystem")),
		},
		"kind":      {StringValue("specialization")},
		"direction": {StringValue("outgoing")},
		"maxDepth":  {IntegerValue(3)},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// OpticalSubsystem is a seed, so only Subsystem remains once.
	names := rowNames(result)
	if len(names) != 1 || names[0] != "Observatory::Subsystem" {
		t.Fatalf("rows = %v", names)
	}
}

func TestExecuteRelatedConnectionsAllocationsAndAssertions(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")

	assertRelated(t, fixture, "telescope::primaryMirror::opticalOut", "connection", "outgoing", 1,
		[]string{"telescope::instrumentCluster::opticalIn"})
	assertRelated(t, fixture, "telescope::instrumentCluster::opticalIn", "connection", "incoming", 1,
		[]string{"telescope::primaryMirror::opticalOut"})
	// An untyped `connect` clause carries connection edges too.
	assertRelated(t, fixture, "telescope::instrumentCluster::dataOut", "connection", "outgoing", 1,
		[]string{"telescope::mountControl::dataIn"})
	// Connection traversal never follows allocation edges.
	assertRelated(t, fixture, "telescope::instrumentCluster", "connection", "outgoing", 1, nil)

	assertRelated(t, fixture, "telescope::instrumentCluster", "allocation", "outgoing", 1,
		[]string{"scienceComputer"})
	assertRelated(t, fixture, "scienceComputer", "allocation", "incoming", 1,
		[]string{"telescope::instrumentCluster"})

	assertRelated(t, fixture, "telescope", "satisfaction", "outgoing", 1,
		[]string{"massRequirement"})
	assertRelated(t, fixture, "massRequirement", "satisfaction", "incoming", 1,
		[]string{"telescope"})
	// A satisfy without `by` relates the element stating it.
	assertRelated(t, fixture, "groundStation", "satisfaction", "outgoing", 1,
		[]string{"pointingRequirement"})

	assertRelated(t, fixture, "massVerification", "verification", "outgoing", 1,
		[]string{"massRequirement"})
	assertRelated(t, fixture, "massRequirement", "verification", "incoming", 1,
		[]string{"massVerification"})
	// Verification traversal never follows satisfaction edges.
	assertRelated(t, fixture, "massRequirement", "verification", "incoming", 1,
		[]string{"massVerification"})
}

func TestExecuteRelatedDeclaredRequirements(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")

	// A satisfy or verify assertion that declares its requirement relates the
	// subject to the declared requirement usage itself and, when the
	// declaration is typed, to its requirement definition.
	assertRelated(t, fixture, "scienceComputer", "satisfaction", "outgoing", 1,
		[]string{"dataArchive::archiveRequirement", "DataRequirement"})
	assertRelated(t, fixture, "dataArchive::archiveRequirement", "satisfaction", "incoming", 1,
		[]string{"scienceComputer"})
	assertRelated(t, fixture, "DataRequirement", "satisfaction", "incoming", 1,
		[]string{"scienceComputer"})
	assertRelated(t, fixture, "scienceComputer", "verification", "outgoing", 1,
		[]string{"archiveVerification::archiveObjective::archiveCheck"})
	assertRelated(t, fixture, "archiveVerification::archiveObjective::archiveCheck", "verification", "incoming", 1,
		[]string{"scienceComputer"})

	// A declared requirement that subsets another is still the edge target.
	assertRelated(t, fixture, "relayHub", "satisfaction", "outgoing", 1,
		[]string{"relayControl::relayRequirement"})
	assertRelated(t, fixture, "relayControl::relayRequirement", "satisfaction", "incoming", 1,
		[]string{"relayHub"})

	// An anonymous declaration form traverses in both directions too.
	for _, kind := range []string{"satisfaction", "verification"} {
		subject := fixture.symbol(t, "mountControlStation")
		outgoing, err := fixture.related(t, subject, kind, "outgoing", 1, Options{})
		if err != nil {
			t.Fatalf("%s outgoing: %v", kind, err)
		}
		if len(outgoing.Rows()) != 1 {
			t.Fatalf("%s outgoing rows = %v", kind, rowNames(outgoing))
		}
		anonymous, _ := outgoing.Rows()[0].Element().Element()
		incoming, err := fixture.related(t, anonymous, kind, "incoming", 1, Options{})
		if err != nil {
			t.Fatalf("%s incoming: %v", kind, err)
		}
		names := rowNames(incoming)
		if len(names) != 1 || names[0] != "Observatory::mountControlStation" {
			t.Fatalf("%s incoming rows = %v", kind, names)
		}
	}
}

func TestExecuteRelatedConsumesTheVisitBudget(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")
	_, err := fixture.related(t, fixture.symbol(t, "instruments"), "subsetting", "incoming", 1,
		Options{VisitBudget: 1})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorVisitBudget {
		t.Fatalf("visit budget error = %v", err)
	}
}

func TestExecuteRelatedLeavesTableConstructionUncharged(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")
	// Building the edge table scans every declaration in the workspace, yet a
	// source with no matching edges reaches nothing and so pays nothing.
	none, err := fixture.related(t, fixture.symbol(t, "Subsystem"), "connection", "incoming", 1,
		Options{VisitBudget: 1})
	if err != nil {
		t.Fatalf("no edges: %v", err)
	}
	if len(none.Rows()) != 0 {
		t.Fatalf("rows = %v, want none", rowNames(none))
	}
	// Only the elements reached are charged: the two subsetters fit a budget of
	// exactly two, however many declarations the scan examined.
	both, err := fixture.related(t, fixture.symbol(t, "instruments"), "subsetting", "incoming", 1,
		Options{VisitBudget: 2})
	if err != nil {
		t.Fatalf("exact budget: %v", err)
	}
	if names := rowNames(both); len(names) != 2 ||
		names[0] != "Observatory::iris" || names[1] != "Observatory::modhis" {
		t.Fatalf("rows = %v", names)
	}
}

func TestExecuteSharesRelationshipTablesThroughTheContext(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")
	tables := NewRelationshipTables()
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Related: tables}
	bindings := func(source string) Bindings {
		return Bindings{
			"source":    {ElementValue(fixture.symbol(t, source))},
			"kind":      {StringValue("subsetting")},
			"direction": {StringValue("incoming")},
			"maxDepth":  {IntegerValue(1)},
		}
	}
	program := fixture.program(t, "Related")
	if _, err := Execute(program, context, bindings("instruments"), Options{}); err != nil {
		t.Fatalf("first execution: %v", err)
	}
	built, ok := tables.entries["subsetting"]
	if !ok {
		t.Fatal("the first execution must leave its subsetting table in the context")
	}
	if _, err := Execute(program, context, bindings("Subsystem"), Options{}); err != nil {
		t.Fatalf("second execution: %v", err)
	}
	if tables.entries["subsetting"] != built {
		t.Fatal("a second execution under the same context must reuse the built table")
	}
	// Tables built against another model are discarded rather than trusted.
	other := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")
	_, err := Execute(other.program(t, "Related"),
		Context{Index: other.index, Resolver: other.resolver, Model: other.model, Related: tables},
		Bindings{
			"source":    {ElementValue(other.symbol(t, "instruments"))},
			"kind":      {StringValue("subsetting")},
			"direction": {StringValue("incoming")},
			"maxDepth":  {IntegerValue(1)},
		}, Options{})
	if err != nil {
		t.Fatalf("other model: %v", err)
	}
	if tables.entries["subsetting"] == built || tables.index != other.index {
		t.Fatal("tables built against another index must be rebuilt")
	}
}

func TestExecuteRelatedComposesWithFiltersProjectionAndInvocation(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_relationships.sysml")

	report, err := fixture.execute(t, "SpecializerReport", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Subsystem"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute report: %v", err)
	}
	var columns []string
	for _, column := range report.Columns() {
		columns = append(columns, column.Name())
	}
	if len(columns) != 2 || columns[0] != "name" || columns[1] != "qualifiedName" {
		t.Fatalf("columns = %v", columns)
	}
	names := rowNames(report)
	if len(names) != 2 ||
		names[0] != "Observatory::MirrorAssembly" ||
		names[1] != "Observatory::OpticalSubsystem" {
		t.Fatalf("rows = %v", names)
	}

	invoked, err := fixture.execute(t, "InvokedSatisfiers", Bindings{
		"req": {ElementValue(fixture.symbol(t, "massRequirement"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute invoked: %v", err)
	}
	names = rowNames(invoked)
	if len(names) != 1 || names[0] != "Observatory::telescope" {
		t.Fatalf("rows = %v", names)
	}

	// An empty traversal keeps the projected schema of a downstream Project.
	empty, err := fixture.execute(t, "SpecializerReport", Bindings{
		"root": {ElementValue(fixture.symbol(t, "MirrorAssembly"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute empty report: %v", err)
	}
	if len(empty.Rows()) != 0 || len(empty.Columns()) != 2 {
		t.Fatalf("empty rows = %d columns = %d", len(empty.Rows()), len(empty.Columns()))
	}
}
