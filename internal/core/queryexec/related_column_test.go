package queryexec

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const traceMatrixFixture = "testdata/trace_matrix.sysml"

// observatory is the fixture's root package, the source every requirement descends from.
func (f executionFixture) observatory(t *testing.T) Value {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Observatory"))
	if len(matches) != 1 {
		t.Fatalf("lookup Observatory: got %d symbols", len(matches))
	}
	return ElementValue(matches[0])
}

// cellNames renders a cell's element values by qualified name, its scalars by text.
func cellNames(t *testing.T, cell Cell) []string {
	t.Helper()
	var out []string
	for _, value := range cell.Values() {
		if sym, ok := value.Element(); ok {
			out = append(out, strings.TrimPrefix(symbols.FQNOf(sym), "Observatory::"))
			continue
		}
		switch value.Kind() {
		case ValueInteger:
			n, _ := value.Integer()
			out = append(out, strconv.FormatInt(n, 10))
		case ValueBoolean:
			b, _ := value.Boolean()
			out = append(out, strconv.FormatBool(b))
		default:
			text, _ := value.String()
			out = append(out, text)
		}
	}
	return out
}

func cellsByColumn(t *testing.T, result *RowSet) map[string][][]string {
	t.Helper()
	out := make(map[string][][]string)
	for _, column := range result.Columns() {
		out[column.Name()] = nil
	}
	for _, row := range result.Rows() {
		cells := row.Cells()
		if len(cells) != len(result.Columns()) {
			t.Fatalf("row has %d cells for %d columns", len(cells), len(result.Columns()))
		}
		for i, column := range result.Columns() {
			out[column.Name()] = append(out[column.Name()], cellNames(t, cells[i]))
		}
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func assertColumn(t *testing.T, cells map[string][][]string, column string, want [][]string) {
	t.Helper()
	got := cells[column]
	if len(got) != len(want) {
		t.Fatalf("column %s = %v, want %v", column, got, want)
	}
	for i := range want {
		if !equalStrings(got[i], want[i]) {
			t.Fatalf("column %s row %d = %v, want %v", column, i, got[i], want[i])
		}
	}
}

func TestExecuteRelatedColumnsBuildATraceabilityMatrix(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	result, err := fixture.execute(t, "Matrix", Bindings{
		"root": {fixture.observatory(t)},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	names := rowNames(result)
	if !equalStrings(names, []string{
		"Observatory::massRequirement",
		"Observatory::pointingRequirement",
		"Observatory::dataRequirement",
	}) {
		t.Fatalf("rows = %v", names)
	}
	var columns []string
	for _, column := range result.Columns() {
		columns = append(columns, column.Name())
		if !column.Origin().Located() {
			t.Fatalf("column %s must carry the RelatedColumn's provenance", column.Name())
		}
	}
	if !equalStrings(columns, []string{"name", "satisfiedBy", "verifiedBy", "verifications", "verified"}) {
		t.Fatalf("columns = %v", columns)
	}
	cells := cellsByColumn(t, result)
	// Lists follow declaration order; a requirement nothing reaches is an empty cell.
	assertColumn(t, cells, "satisfiedBy", [][]string{
		{"telescope", "groundStation"},
		{"mount"},
		nil,
	})
	assertColumn(t, cells, "verifiedBy", [][]string{
		{"massVerification", "pointingVerification"},
		{"pointingVerification"},
		nil,
	})
	assertColumn(t, cells, "verifications", [][]string{{"2"}, {"1"}, {"0"}})
	assertColumn(t, cells, "verified", [][]string{{"true"}, {"true"}, {"false"}})
	for _, row := range result.Rows() {
		for _, value := range row.Cells()[1].Values() {
			if !value.Origin().Located() {
				t.Fatal("related elements must carry declaration provenance")
			}
		}
		if _, ok := row.Cells()[3].Values()[0].Integer(); !ok {
			t.Fatalf("count cell = %v, want an integer", row.Cells()[3].Values()[0].Kind())
		}
		if _, ok := row.Cells()[4].Values()[0].Boolean(); !ok {
			t.Fatalf("any cell = %v, want a Boolean", row.Cells()[4].Values()[0].Kind())
		}
	}
}

func (f executionFixture) traced(
	t *testing.T,
	root Value,
	kind, direction string,
	maxDepth int64,
	aggregate string,
	options Options,
) (*RowSet, error) {
	t.Helper()
	return f.execute(t, "Traced", Bindings{
		"root":      {root},
		"kind":      {StringValue(kind)},
		"direction": {StringValue(direction)},
		"maxDepth":  {IntegerValue(maxDepth)},
		"aggregate": {StringValue(aggregate)},
	}, options)
}

func TestExecuteRelatedColumnFollowsDepthAndDirection(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	mirror := ElementValue(fixture.symbol(t, "MirrorAssembly"))

	shallow, err := fixture.traced(t, mirror, "specialization", "outgoing", 1, "list", Options{})
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	assertColumn(t, cellsByColumn(t, shallow), "related", [][]string{{"OpticalSubsystem"}})

	deep, err := fixture.traced(t, mirror, "specialization", "outgoing", 3, "list", Options{})
	if err != nil {
		t.Fatalf("depth 3: %v", err)
	}
	// Breadth-first: the direct general first, then what it generalizes to.
	assertColumn(t, cellsByColumn(t, deep), "related", [][]string{{"OpticalSubsystem", "Subsystem"}})

	count, err := fixture.traced(t, mirror, "specialization", "outgoing", 3, "count", Options{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	assertColumn(t, cellsByColumn(t, count), "related", [][]string{{"2"}})

	incoming, err := fixture.traced(t, ElementValue(fixture.symbol(t, "Subsystem")), "specialization", "incoming", 2, "list", Options{})
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	assertColumn(t, cellsByColumn(t, incoming), "related", [][]string{{"OpticalSubsystem", "MirrorAssembly"}})

	// A zero depth reaches nothing: an empty list, a zero count, a false any.
	none, err := fixture.traced(t, mirror, "specialization", "outgoing", 0, "list", Options{})
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	assertColumn(t, cellsByColumn(t, none), "related", [][]string{nil})
	any, err := fixture.traced(t, mirror, "specialization", "outgoing", 0, "any", Options{})
	if err != nil {
		t.Fatalf("depth 0 any: %v", err)
	}
	assertColumn(t, cellsByColumn(t, any), "related", [][]string{{"false"}})
}

func TestExecuteRelatedColumnReportsItsColumnInErrors(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	mirror := ElementValue(fixture.symbol(t, "MirrorAssembly"))
	cases := []struct {
		name      string
		kind      string
		direction string
		aggregate string
		options   Options
		want      ErrorKind
		message   string
	}{
		{
			name: "unknown relationship kind", kind: "ownership", direction: "outgoing", aggregate: "list",
			want:    ErrorUnknownRelationship,
			message: `query Observatory::Traced column related does not support relationship kind "ownership"`,
		},
		{
			name: "invalid direction", kind: "specialization", direction: "sideways", aggregate: "list",
			want:    ErrorInvalidOperator,
			message: `query Observatory::Traced operation related-column column related does not support "sideways"`,
		},
		{
			name: "unsupported aggregate", kind: "specialization", direction: "outgoing", aggregate: "sum",
			want:    ErrorInvalidArgument,
			message: `query Observatory::Traced operation related-column column related has invalid argument aggregate`,
		},
		{
			name: "exhausted visit budget", kind: "specialization", direction: "outgoing", aggregate: "list",
			options: Options{VisitBudget: 1},
			want:    ErrorVisitBudget,
			message: `query Observatory::Traced exceeded its visit budget in column related`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.traced(t, mirror, test.kind, test.direction, 3, test.aggregate, test.options)
			execution := executionError(t, err, test.want)
			if execution.Property != "related" {
				t.Fatalf("error column = %q, want related", execution.Property)
			}
			if execution.Operation != "related-column" {
				t.Fatalf("error operation = %s, want related-column", execution.Operation)
			}
			if !execution.Origin.Located() {
				t.Fatal("error must carry the RelatedColumn's provenance")
			}
			if execution.Error() != test.message {
				t.Fatalf("message = %q, want %q", execution.Error(), test.message)
			}
		})
	}
}

func TestExecuteRelatedColumnChargesTableConstructionToTheVisitBudget(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	// Building the satisfaction table scans the workspace, so a tiny budget
	// fails even for a requirement nothing satisfies.
	_, err := fixture.traced(t, ElementValue(fixture.symbol(t, "dataRequirement")),
		"satisfaction", "incoming", 1, "count", Options{VisitBudget: 3})
	execution := executionError(t, err, ErrorVisitBudget)
	if execution.Property != "related" {
		t.Fatalf("error column = %q, want related", execution.Property)
	}
}

func TestExecuteRelatedColumnsFilterAndOrderDownstream(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	root := fixture.observatory(t)

	uncovered, err := fixture.execute(t, "Uncovered", Bindings{"root": {root}}, Options{})
	if err != nil {
		t.Fatalf("uncovered: %v", err)
	}
	if names := rowNames(uncovered); !equalStrings(names, []string{"Observatory::dataRequirement"}) {
		t.Fatalf("uncovered rows = %v", names)
	}
	// The filter keeps the projected schema and cells of the rows it selects.
	assertColumn(t, cellsByColumn(t, uncovered), "verifications", [][]string{{"0"}})

	unverified, err := fixture.execute(t, "Unverified", Bindings{"root": {root}}, Options{})
	if err != nil {
		t.Fatalf("unverified: %v", err)
	}
	if names := rowNames(unverified); !equalStrings(names, []string{"Observatory::dataRequirement"}) {
		t.Fatalf("unverified rows = %v", names)
	}

	ordered, err := fixture.execute(t, "MostVerified", Bindings{"root": {root}}, Options{})
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	if names := rowNames(ordered); !equalStrings(names, []string{
		"Observatory::massRequirement",
		"Observatory::pointingRequirement",
		"Observatory::dataRequirement",
	}) {
		t.Fatalf("ordered rows = %v", names)
	}
	assertColumn(t, cellsByColumn(t, ordered), "satisfiedBy", [][]string{
		{"telescope", "groundStation"},
		{"mount"},
		nil,
	})
}

func TestExecuteRelatedColumnTraversesFromAnObjectsDeclaration(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, traceMatrixFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	telescope, err := ctx.Instantiate(fixture.symbol(t, "telescope"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "telescope", Object: telescope}},
	}
	result, err := Execute(fixture.program(t, "Traced"), context, Bindings{
		"root":      {ObjectValue(telescope, "telescope")},
		"kind":      {StringValue("satisfaction")},
		"direction": {StringValue("outgoing")},
		"maxDepth":  {IntegerValue(1)},
		"aggregate": {StringValue("list")},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The object stands for the telescope usage, which satisfies the mass requirement.
	assertColumn(t, cellsByColumn(t, result), "related", [][]string{{"massRequirement"}})
}
