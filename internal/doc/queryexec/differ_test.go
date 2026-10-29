package queryexec

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const differModel = `
package Before {
	part a { attribute mass = 1; }
	part b { attribute mass = 2; }
	part c { attribute mass = 3; }
}
package After {
	part a { attribute mass = 1; }
	part b { attribute mass = 5; }
	part d { attribute mass = 4; }
}
package Twice {
	part x { attribute mass = 1; }
	part y { attribute mass = 1; }
}
package NestedBefore {
	part outer { attribute mass = 1; part inner { attribute mass = 2; } }
}
package NestedAfter {
	part outer { attribute mass = 5; part inner { attribute mass = 3; } }
}
package Once {
	part x { attribute mass = 1; }
	part z { attribute mass = 2.5; }
}
package RealKeys {
	part p { attribute mass = 1.0; }
	part q { attribute mass = 2.50; }
}
calc def PartMasses :> Query {
	in root : Element;
	Project(source = WhereType(source = OwnedElements(source = root), type = "PartUsage"), properties = ("name", "mass"))
}
calc def PartNames :> Query {
	in root : Element;
	Project(source = WhereType(source = OwnedElements(source = root), type = "PartUsage"), properties = ("name"))
}
calc def ChangedMasses :> Query {
	in left : Element;
	in right : Element;
	Differ(left = PartMasses(root = left), right = PartMasses(root = right), key = "name", column = "mass")
}
calc def ChangedNamesByMass :> Query {
	in left : Element;
	in right : Element;
	Differ(left = PartMasses(root = left), right = PartMasses(root = right), key = "mass", column = "name")
}
calc def ReservedKey :> Query {
	in left : Element;
	in right : Element;
	Differ(left = PartMasses(root = left), right = PartMasses(root = right), key = "left", column = "mass")
}
calc def NestedMasses :> Query {
	in root : Element;
	Project(source = Tree(source = WhereType(source = Descendants(source = root), type = "PartUsage")), properties = ("name", "mass"))
}
calc def ChangedNestedMasses :> Query {
	in left : Element;
	in right : Element;
	Differ(left = NestedMasses(root = left), right = NestedMasses(root = right), key = "name", column = "mass")
}
calc def MassesAgainstNames :> Query {
	in left : Element;
	in right : Element;
	Differ(left = PartMasses(root = left), right = PartNames(root = right), key = "name", column = "mass")
}
`

// TestExecuteDifferReportsChangedCellsByKey: Differ emits one (key, left, right) row per
// key on both sides whose column differs; keys on one side only are not rows.
func TestExecuteDifferReportsChangedCellsByKey(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	bindings := Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Before"))},
		"right": {ElementValue(fixture.symbol(t, "After"))},
	}
	result, err := fixture.execute(t, "ChangedMasses", bindings, Options{})
	if err != nil {
		t.Fatalf("ChangedMasses: %v", err)
	}
	var columns []string
	for _, column := range result.Columns() {
		columns = append(columns, column.Name())
	}
	if got := strings.Join(columns, " "); got != "name left right" {
		t.Fatalf("columns = %q, want the key then each side's cell", got)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("ChangedMasses returned %d rows, want the one part whose mass changed", len(rows))
	}
	sym, _ := rows[0].Element().Element()
	if sym.Name != "b" {
		t.Fatalf("row names %s, want b", sym.Name)
	}
	var cells []string
	for _, cell := range rows[0].Cells() {
		var texts []string
		for _, value := range cell.Values() {
			texts = append(texts, differValueText(t, value))
		}
		cells = append(cells, strings.Join(texts, ","))
	}
	if got := strings.Join(cells, " | "); got != "b | 2 | 5" {
		t.Fatalf("cells = %q, want the key and both masses", got)
	}

	same, err := fixture.execute(t, "ChangedMasses", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Before"))},
		"right": {ElementValue(fixture.symbol(t, "Before"))},
	}, Options{})
	if err != nil {
		t.Fatalf("ChangedMasses against itself: %v", err)
	}
	if n := len(same.Rows()); n != 0 {
		t.Fatalf("a table against itself differs in %d rows, want none", n)
	}
}

// TestExecuteDifferMatchesKeysUnderValueEquality: keys match as valuesEqual does — an
// integer 1 and a real 1.0 are one key — not by their spelling.
func TestExecuteDifferMatchesKeysUnderValueEquality(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	result, err := fixture.execute(t, "ChangedNamesByMass", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Once"))},
		"right": {ElementValue(fixture.symbol(t, "RealKeys"))},
	}, Options{})
	if err != nil {
		t.Fatalf("ChangedNamesByMass over Once and RealKeys: %v", err)
	}
	var rows []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		cells := row.Cells()
		left, _ := cells[1].Values()[0].String()
		right, _ := cells[2].Values()[0].String()
		rows = append(rows, sym.Name+":"+left+"->"+right)
	}
	if got := strings.Join(rows, " "); got != "x:x->p z:z->q" {
		t.Fatalf("rows = %q, want both masses matched across integer and real spellings", got)
	}
}

// TestExecuteDifferKeepsTreeDepth: a changed row of a Tree-arranged left side keeps its depth.
func TestExecuteDifferKeepsTreeDepth(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	result, err := fixture.execute(t, "ChangedNestedMasses", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "NestedBefore"))},
		"right": {ElementValue(fixture.symbol(t, "NestedAfter"))},
	}, Options{})
	if err != nil {
		t.Fatalf("ChangedNestedMasses: %v", err)
	}
	var rows []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		rows = append(rows, fmt.Sprintf("%s@%d", sym.Name, row.Depth()))
	}
	if got := strings.Join(rows, " "); got != "outer@0 inner@1" {
		t.Fatalf("rows = %q, want the outer part at depth 0 and the inner nested under it", got)
	}
}

// TestExecuteDifferReservedKeyIsInvalid: a key named left or right would collide with the
// result's side columns, so it is a typed invalid argument.
func TestExecuteDifferReservedKeyIsInvalid(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	_, err := fixture.execute(t, "ReservedKey", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Before"))},
		"right": {ElementValue(fixture.symbol(t, "After"))},
	}, Options{})
	var failure *Error
	if !errors.As(err, &failure) || failure.Kind != ErrorInvalidArgument || failure.Parameter != "key" {
		t.Fatalf("ReservedKey: %v, want %s on key", err, ErrorInvalidArgument)
	}
}

// TestExecuteDifferDuplicateKeyIsAModelError: a key value on two rows of one side is the
// duplicate-key model error naming every offending row, not a silent choice.
func TestExecuteDifferDuplicateKeyIsAModelError(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	_, err := fixture.execute(t, "ChangedNamesByMass", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Twice"))},
		"right": {ElementValue(fixture.symbol(t, "After"))},
	}, Options{})
	var failure *Error
	if !errors.As(err, &failure) || failure.Kind != ErrorDuplicateKey {
		t.Fatalf("ChangedNamesByMass over Twice: %v, want %s", err, ErrorDuplicateKey)
	}
	if failure.Outcome == nil || failure.Outcome.Model == nil || len(failure.Outcome.Model.Findings) != 1 {
		t.Fatalf("outcome = %+v, want one duplicateKey finding", failure.Outcome)
	}
	finding := failure.Outcome.Model.Findings[0]
	if finding.Constraint != ConstraintDuplicateKey || finding.Element != "Observatory::Twice::y" || finding.Step.Label() != "Differ(mass, name)" {
		t.Fatalf("finding = %+v, want duplicateKey on Observatory::Twice::y at Differ(mass, name)", finding)
	}
	if !strings.Contains(finding.Observed, "Observatory::Twice::x") {
		t.Fatalf("observed %q does not name the other carrier Twice::x", finding.Observed)
	}
}

// TestExecuteDifferUnknownColumnIsAPathError: a side that projects no such column is the
// unknown-column path error at the Differ step, naming the side and its columns.
func TestExecuteDifferUnknownColumnIsAPathError(t *testing.T) {
	fixture := loadExecutionFixture(t, differModel)
	_, err := fixture.execute(t, "MassesAgainstNames", Bindings{
		"left":  {ElementValue(fixture.symbol(t, "Before"))},
		"right": {ElementValue(fixture.symbol(t, "After"))},
	}, Options{})
	var failure *Error
	if !errors.As(err, &failure) || failure.Kind != ErrorUnknownColumn {
		t.Fatalf("MassesAgainstNames: %v, want %s", err, ErrorUnknownColumn)
	}
	if failure.Parameter != "right" || failure.Property != "mass" || strings.Join(failure.Candidates, ",") != "name" {
		t.Fatalf("error = %+v, want the right side lacking mass among name", failure)
	}
	if failure.Outcome == nil || failure.Outcome.Path == nil || failure.Outcome.Step().Label() != "Differ(name, mass)" {
		t.Fatalf("outcome = %+v, want a path error at Differ(name, mass)", failure.Outcome)
	}
}

// differValueText spells a string or integer cell value for a test expectation.
func differValueText(t *testing.T, value Value) string {
	t.Helper()
	if text, ok := value.String(); ok {
		return text
	}
	if integer, ok := value.IntegerConst(); ok {
		return integer.FormatInt()
	}
	t.Fatalf("unexpected cell value of kind %s", value.Kind())
	return ""
}
