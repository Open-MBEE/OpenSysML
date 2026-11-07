package queryexec

import (
	"slices"
	"testing"
)

// TestExecuteProjectsTheWildcard: `*` among Project's properties is every
// property the rows' metaclasses have that the evaluator serves, with the
// evaluator's own, in its place among the properties named; `path.*` those of
// the elements the path reaches. A library feature a metaclass inherits
// (dispatchScope) is none of them.
func TestExecuteProjectsTheWildcard(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel+`
calc def Everything :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "*", "owner.*"), tolerate = true)
}
`)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	result, err := fixture.execute(t, "Everything", bindings, Options{})
	if err != nil {
		t.Fatalf("Everything: %v", err)
	}
	var names []string
	for _, column := range result.Columns() {
		names = append(names, column.Name())
	}
	if names[0] != "name" || slices.Contains(names[1:], "name") {
		t.Fatalf("columns = %v, want name first and once", names)
	}
	for _, want := range []string{"@type", "isComposite", "qualifiedName", "owner.name", "owner.@type"} {
		if !slices.Contains(names, want) {
			t.Errorf("columns %v lack %s", names, want)
		}
	}
	for _, unwanted := range []string{"*", "owner.*", "dispatchScope", "owner.dispatchScope"} {
		if slices.Contains(names, unwanted) {
			t.Errorf("columns %v carry %s", names, unwanted)
		}
	}
	own := names[1:slices.Index(names, "owner.@id")]
	if !slices.IsSorted(own) || !slices.IsSorted(names[len(own)+1:]) {
		t.Errorf("the wildcard's columns are not sorted: %v", names)
	}
	rows := result.Rows()
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	cells := rows[1].Cells()
	if name, _ := cells[0].Values()[0].String(); name != "mirror" || len(cells) != len(names) {
		t.Fatalf("row 1 = %q with %d cells, want mirror with %d", name, len(cells), len(names))
	}
	if composite, _ := cells[slices.Index(names, "isComposite")].Values()[0].Boolean(); !composite {
		t.Fatalf("mirror's isComposite = %v, want true", composite)
	}
}

// TestExecuteWildcardIsOnlyWhatTheRowsCarry: a wildcard over no rows projects
// no columns, and over rows of one metaclass only that metaclass's properties.
func TestExecuteWildcardIsOnlyWhatTheRowsCarry(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel+`
calc def Nothing :> Query {
	in root : Element;
	Project(source = WhereName(source = PartUsages(root = root), operator = "=", value = "nobody"), properties = ("*"), tolerate = true)
}
`)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	result, err := fixture.execute(t, "Nothing", bindings, Options{})
	if err != nil {
		t.Fatalf("Nothing: %v", err)
	}
	if len(result.Rows()) != 0 || len(result.Columns()) != 0 {
		t.Fatalf("rows = %d, columns = %d, want none of either", len(result.Rows()), len(result.Columns()))
	}
}
