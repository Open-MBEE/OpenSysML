package queryexec

import (
	"errors"
	"slices"
	"testing"
)

const emptyStepBody = `
package Fleet {
	part def Crewed { part pilot; part gunner; }
	part def Drone;
}
`

// stringCells reads the named column as strings, one slice per row; nil where the cell is empty.
func stringCells(t *testing.T, result *RowSet, column string) [][]string {
	t.Helper()
	position := slices.IndexFunc(result.Columns(), func(c Column) bool { return c.Name() == column })
	if position < 0 {
		t.Fatalf("no column %s in %v", column, result.Columns())
	}
	var out [][]string
	for _, row := range result.Rows() {
		var cell []string
		for _, v := range row.Cells()[position].Values() {
			text, ok := v.String()
			if !ok {
				t.Fatalf("%s cell = %+v, want strings", column, row.Cells()[position].Values())
			}
			cell = append(cell, text)
		}
		out = append(out, cell)
	}
	return out
}

// A member path whose inner step names a feature the row has but holds no element is an empty
// cell, not a missing feature: `ownedMember.name` on a definition with no members.
func TestExecuteMemberPathEmptyInnerStepIsAnEmptyCell(t *testing.T) {
	for _, test := range []struct {
		name, column, query string
	}{
		{
			name:   "string property",
			column: "ownedMember.name",
			query: `
calc def MemberNames :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name", "ownedMember.name")
	)
}`,
		},
		{
			name:   "feature chain",
			column: "members",
			query: `
calc def MemberNames :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "members", expression = ownedMember.name))
	)
}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadExecutionFixture(t, emptyStepBody+test.query)
			result, err := fixture.execute(t, "MemberNames", Bindings{
				"root": {ElementValue(fixture.symbol(t, "Fleet"))},
			}, Options{})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := stringCells(t, result, test.column)
			want := [][]string{{"pilot", "gunner"}, nil}
			if !slices.EqualFunc(got, want, slices.Equal) {
				t.Fatalf("%s cells = %q, want %q", test.column, got, want)
			}
		})
	}
}

// A filter over such a path drops the row whose step is empty instead of failing the query.
func TestExecuteWhereFeatureMemberPathEmptyInnerStepDropsTheRow(t *testing.T) {
	fixture := loadExecutionFixture(t, emptyStepBody+`
calc def Piloted :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = Descendants(source = root, maxDepth = 1),
			'feature' = "ownedMember.name", operator = "=", value = "pilot"),
		properties = ("name")
	)
}`)
	result, err := fixture.execute(t, "Piloted", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Fleet"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stringCells(t, result, "name"); !slices.EqualFunc(got, [][]string{{"Crewed"}}, slices.Equal) {
		t.Fatalf("rows = %q, want Crewed alone", got)
	}
}

// A step naming neither a member nor a metaclass feature still fails the query; an empty step on
// another row is no evidence that the rest of the path names anything.
func TestExecuteMemberPathUnknownStepIsStillUnknown(t *testing.T) {
	for _, path := range []string{"zzz.name", "ownedMember.zzz.name", "ownedMember.nmae"} {
		for name, operation := range map[string]string{
			"property": `Project(source = Descendants(source = root, maxDepth = 1), properties = ("name", "` + path + `"))`,
			"column": `Project(source = Descendants(source = root, maxDepth = 1), properties = ("name"),
				columns = (Column(name = "c", expression = ` + path + `)))`,
			"filter": `WhereFeature(source = Descendants(source = root, maxDepth = 1), 'feature' = "` + path + `", operator = "=", value = "pilot")`,
		} {
			fixture := loadExecutionFixture(t, emptyStepBody+`
calc def Bad :> Query {
	in root : Element;
	`+operation+`
}`)
			_, err := fixture.execute(t, "Bad", Bindings{
				"root": {ElementValue(fixture.symbol(t, "Fleet"))},
			}, Options{})
			var executionError *Error
			if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownProperty {
				t.Errorf("%s %s: error = %v, want %v", name, path, err, ErrorUnknownProperty)
			}
		}
	}
}

// An empty step yields to a property path that reads further: a redefinition binding no value
// has no valuation of its own, yet the value it inherits is still read by name.
func TestExecuteMemberPathEmptyStepYieldsToPropertyPath(t *testing.T) {
	fixture := loadExecutionFixture(t, `
enum def Mode { on; off; }
part def Tank { attribute mode : Mode = Mode::on; }
part def Spare :> Tank { attribute :>> mode; }
calc def Modes :> Query {
	Project(
		source = OwnedElements(source = Named(qualifiedName = "Observatory::Spare")),
		properties = ("name", "valuation.name"))
}`)
	result, err := fixture.execute(t, "Modes", nil, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := stringCells(t, result, "valuation.name"), [][]string{{"on"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("valuation.name = %q, want %q", got, want)
	}
}

// A root-level element reads no owning namespace, so a string path through it is an empty cell.
func TestExecuteMemberPathThroughRootOwnershipIsAnEmptyCell(t *testing.T) {
	fixture := loadExecutionSource(t, `
part def RootDef;
package Observatory {`+executionImports+`
	calc def Path :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "owningNamespace.ownedMembership"))
	}
}`)
	matches := fixture.index.LookupQualified("RootDef")
	if len(matches) != 1 {
		t.Fatalf("lookup RootDef: got %d symbols", len(matches))
	}
	result, err := fixture.execute(t, "Path", Bindings{"root": {ElementValue(matches[0])}}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if cells := result.Rows()[0].Cells(); len(cells[1].Values()) != 0 {
		t.Fatalf("owningNamespace.ownedMembership = %+v, want an empty cell", cells[1].Values())
	}
}
