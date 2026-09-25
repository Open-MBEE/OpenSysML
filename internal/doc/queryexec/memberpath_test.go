package queryexec

import (
	"errors"
	"slices"
	"testing"
)

const memberPathBody = `
attribute def Stat {
	attribute runs : Integer;
	attribute mean : Real;
	attribute tags : String[0..*];
	attribute spread : Real[0..1];
}
part def Sample {
	attribute size : Real;
}
package Results {
	individual part def A :> Sample {
		attribute :>> size = 1.0;
		attribute stat : Stat {
			attribute redefines runs = 5;
			attribute redefines mean = 18.0;
			attribute redefines tags = ("a", "b");
		}
		attribute 'Monte Carlo' : Stat {
			attribute redefines runs = 4;
		}
		attribute outer : Stat {
			attribute inner : Stat {
				attribute redefines runs = 9;
			}
		}
	}
	individual part def B :> Sample {
		attribute :>> size = 2.0;
		attribute stat : Stat {
			attribute redefines runs = 7;
			attribute redefines mean = 3.5;
		}
	}
	individual part def C :> Sample {
		attribute :>> size = 3.0;
	}
}
`

func memberPathFixture(t *testing.T, query string) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, memberPathBody+query)
}

// cellNumbers reads every integer or real value of the named column as a
// float, one slice per row; a row's slice is nil where the cell is empty.
func cellNumbers(t *testing.T, result *RowSet, column string) [][]float64 {
	t.Helper()
	position := slices.IndexFunc(result.Columns(), func(c Column) bool { return c.Name() == column })
	if position < 0 {
		t.Fatalf("no column %s in %v", column, result.Columns())
	}
	var out [][]float64
	for _, row := range result.Rows() {
		var nums []float64
		for _, value := range row.Cells()[position].Values() {
			if integer, ok := value.Integer(); ok {
				nums = append(nums, float64(integer))
				continue
			}
			real, ok := value.Real()
			if !ok {
				t.Fatalf("%s cell = %+v, want numbers", column, row.Cells()[position].Values())
			}
			nums = append(nums, real)
		}
		out = append(out, nums)
	}
	return out
}

func memberPathRows(t *testing.T, fixture executionFixture, query string) *RowSet {
	t.Helper()
	result, err := fixture.execute(t, query, Bindings{
		"root": {ElementValue(fixture.symbol(t, "Results"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute %s: %v", query, err)
	}
	return result
}

// A feature chain names a member nested in the row element: `stat.runs` reads
// stat's runs on each row, and a row without the member is an empty cell.
func TestExecuteComputedMemberPathReadsNestedFeature(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Runs :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "N", expression = stat.runs),
			Column(name = "m", expression = stat.mean)
		)
	)
}`)
	result := memberPathRows(t, fixture, "Runs")
	got := cellNumbers(t, result, "N")
	if !slices.EqualFunc(got, [][]float64{{5}, {7}, nil}, slices.Equal) {
		t.Fatalf("N cells = %v, want {5}, {7}, empty", got)
	}
	means := cellNumbers(t, result, "m")
	if !slices.EqualFunc(means, [][]float64{{18}, {3.5}, nil}, slices.Equal) {
		t.Fatalf("m cells = %v, want {18}, {3.5}, empty", means)
	}
}

// A chain head resolving to a query input parameter stays row-relative: a
// parameter cannot be read through, so stat.runs reads the row's member.
func TestExecuteComputedMemberPathParameterHeadIsRowRelative(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Runs :> Query {
	in root : Element;
	in stat : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "N", expression = stat.runs))
	)
}`)
	result, err := fixture.execute(t, "Runs", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Results"))},
		"stat": {ElementValue(fixture.symbol(t, "Results"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := cellNumbers(t, result, "N")
	if !slices.EqualFunc(got, [][]float64{{5}, {7}, nil}, slices.Equal) {
		t.Fatalf("N cells = %v, want {5}, {7}, empty", got)
	}
}

// A quoted head reads a member whose name is not a basic name.
func TestExecuteComputedMemberPathQuotedHead(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Carlo :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "N", expression = 'Monte Carlo'.runs ?? 0))
	)
}`)
	result := memberPathRows(t, fixture, "Carlo")
	got := cellNumbers(t, result, "N")
	if !slices.EqualFunc(got, [][]float64{{4}, {0}, {0}}, slices.Equal) {
		t.Fatalf("N cells = %v, want {4}, {0}, {0}", got)
	}
}

// A member the path reaches that declares no value is an empty cell, and a
// multi-valued member fills the cell with every value in order.
func TestExecuteComputedMemberPathEmptyAndMultiValued(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Cells :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "s", expression = stat.spread ?? null),
			Column(name = "t", expression = stat.tags)
		)
	)
}`)
	result := memberPathRows(t, fixture, "Cells")
	rows := result.Rows()
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	for i, row := range rows {
		if len(row.Cells()[1].Values()) != 0 {
			t.Fatalf("row %d spread cell = %+v, want empty", i, row.Cells()[1].Values())
		}
	}
	var tags [][]string
	for _, row := range rows {
		var cell []string
		for _, value := range row.Cells()[2].Values() {
			text, ok := value.String()
			if !ok {
				t.Fatalf("tags cell = %+v, want strings", row.Cells()[2].Values())
			}
			cell = append(cell, text)
		}
		tags = append(tags, cell)
	}
	if !slices.EqualFunc(tags, [][]string{{"a", "b"}, nil, nil}, slices.Equal) {
		t.Fatalf("tags cells = %v", tags)
	}
}

// A `.`-joined property string is the same member path.
func TestExecuteProjectMemberPathStringProperty(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Runs :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name", "stat.runs")
	)
}`)
	result := memberPathRows(t, fixture, "Runs")
	got := cellNumbers(t, result, "stat.runs")
	if !slices.EqualFunc(got, [][]float64{{5}, {7}, nil}, slices.Equal) {
		t.Fatalf("stat.runs cells = %v, want {5}, {7}, empty", got)
	}
}

// A member path orders rows by the nested value; the projected column name
// orders them the same way after a projection.
func TestExecuteOrderByMemberPath(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Ordered :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = Descendants(source = root, maxDepth = 1),
			property = "stat.runs",
			direction = "descending",
			missing = "last",
			multiple = "error"
		),
		properties = ("name")
	)
}`)
	result := memberPathRows(t, fixture, "Ordered")
	var names []string
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"B", "A", "C"}) {
		t.Fatalf("ordered names = %v, want [B A C]", names)
	}
}

func TestExecuteOrderByProjectedMemberPathColumn(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = Descendants(source = root, maxDepth = 1),
			properties = ("name"),
			columns = (Column(name = "N", expression = stat.runs))
		),
		property = "N",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}`)
	result := memberPathRows(t, fixture, "Ordered")
	var names []string
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"B", "A", "C"}) {
		t.Fatalf("ordered names = %v, want [B A C]", names)
	}
}

// `??` defaults a row whose member or member's value is absent.
func TestExecuteComputedMemberPathCoalesce(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Runs :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "N", expression = stat.runs ?? 0))
	)
}`)
	result := memberPathRows(t, fixture, "Runs")
	got := cellNumbers(t, result, "N")
	if !slices.EqualFunc(got, [][]float64{{5}, {7}, {0}}, slices.Equal) {
		t.Fatalf("N cells = %v, want {5}, {7}, {0}", got)
	}
}

// A path no row reaches is an unknown property, spelled as a string or a chain.
func TestExecuteMemberPathUnknownOnEveryRow(t *testing.T) {
	for _, test := range []struct {
		name  string
		query string
	}{
		{
			name: "string property",
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name", "zzz.runs")
	)
}`,
		},
		{
			name: "feature chain",
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "N", expression = zzz.runs ?? 0))
	)
}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := memberPathFixture(t, test.query)
			_, err := fixture.execute(t, "Bad", Bindings{
				"root": {ElementValue(fixture.symbol(t, "Results"))},
			}, Options{})
			var executionError *Error
			if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownProperty {
				t.Fatalf("error = %v, want %v", err, ErrorUnknownProperty)
			}
		})
	}
}

// A path of three segments walks two members deep.
func TestExecuteComputedMemberPathTwoLevels(t *testing.T) {
	fixture := memberPathFixture(t, `
calc def Deep :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "N", expression = outer.inner.runs ?? 0))
	)
}`)
	result := memberPathRows(t, fixture, "Deep")
	got := cellNumbers(t, result, "N")
	if !slices.EqualFunc(got, [][]float64{{9}, {0}, {0}}, slices.Equal) {
		t.Fatalf("N cells = %v, want {9}, {0}, {0}", got)
	}
}

// A nested member holding more values than its multiplicity admits fails the
// column as a direct feature column does; a member declared [0..*] holds them.
func TestExecuteComputedMemberPathCardinality(t *testing.T) {
	fixture := loadExecutionFixture(t, `
attribute def Bounded { attribute runs : Integer; }
attribute def Unbounded { attribute counts : Integer[0..*]; }
part def Row;
package Results {
	individual part def Over :> Row {
		attribute stat : Bounded {
			attribute redefines runs = (1, 2);
		}
	}
	individual part def Plenty :> Row {
		attribute stat : Unbounded {
			attribute redefines counts = (1, 2);
		}
	}
}
calc def Over :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "N", expression = stat.runs))
	)
}
calc def Plenty :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "C", expression = stat.counts))
	)
}
`)
	_, err := fixture.execute(t, "Over", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Results"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnCardinality {
		t.Fatalf("error = %v, want %v", err, ErrorColumnCardinality)
	}
	if executionError.Expected != "1..1" {
		t.Fatalf("Expected = %q, want %q", executionError.Expected, "1..1")
	}
	result := memberPathRows(t, fixture, "Plenty")
	got := cellNumbers(t, result, "C")
	if !slices.EqualFunc(got, [][]float64{nil, {1, 2}}, slices.Equal) {
		t.Fatalf("C cells = %v, want empty, {1,2}", got)
	}
}
