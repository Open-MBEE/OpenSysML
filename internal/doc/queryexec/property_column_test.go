package queryexec

import (
	"slices"
	"testing"
)

const propertyColumnBody = `
part def Subsystem {
	attribute mass : Real;
}
part system {
	part a : Subsystem {
		doc /* first */
		doc /* second */
		attribute redefines mass = 2.5;
	}
	part b : Subsystem;
}
`

// A PropertyColumn holds its place among the other columns and reads the
// property as the properties list does: every value, an absent one empty; a
// null property, like an omitted one, is the column's name.
func TestExecutePropertyColumnsKeepOrderAndMultiplicity(t *testing.T) {
	fixture := loadExecutionFixture(t, propertyColumnBody+`
calc def Ledger :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (
			PropertyColumn(name = "name"),
			Column(name = "mass", expression = Subsystem::mass ?? ""),
			PropertyColumn(name = "Notes", property = "documentation"),
			PropertyColumn("qualifiedName", null)
		)
	)
}
`)
	result := computedRows(t, fixture, "Ledger")
	var names []string
	for _, column := range result.Columns() {
		names = append(names, column.Name())
	}
	if !slices.Equal(names, []string{"name", "mass", "Notes", "qualifiedName"}) {
		t.Fatalf("columns = %v", names)
	}
	rows := result.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("name = %v", got)
	}
	if got := cellTexts(t, result, 3); !slices.Equal(got, []string{"Observatory::system::a", "Observatory::system::b"}) {
		t.Fatalf("qualifiedName = %v", got)
	}
	notes := rows[0].Cells()[2].Values()
	if len(notes) != 2 {
		t.Fatalf("a's Notes = %d values, want the two docs", len(notes))
	}
	first, _ := notes[0].String()
	second, _ := notes[1].String()
	if first != "first" || second != "second" {
		t.Fatalf("a's Notes = %q %q", first, second)
	}
	if empty := rows[1].Cells()[2].Values(); len(empty) != 0 {
		t.Fatalf("b's Notes = %d values, want an empty cell", len(empty))
	}
}

// A PropertyColumn over a property no row has fails as a projected property does.
func TestExecutePropertyColumnUnknownPropertyFails(t *testing.T) {
	fixture := loadExecutionFixture(t, propertyColumnBody+`
calc def Ledger :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (PropertyColumn(name = "Weight", property = "weight"))
	)
}
`)
	_, err := fixture.execute(t, "Ledger", Bindings{
		"root": {ElementValue(fixture.symbol(t, "system"))},
	}, Options{})
	failure := executionError(t, err, ErrorUnknownProperty)
	if failure.Property != "weight" {
		t.Fatalf("property = %q, want weight", failure.Property)
	}
}
