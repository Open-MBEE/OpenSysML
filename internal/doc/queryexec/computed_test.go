package queryexec

import (
	"errors"
	"slices"
	"testing"
)

const computedBody = `
part def Subsystem {
	attribute mass : Real;
	attribute alloc : Real;
	attribute count : Integer;
}
part system {
	part a : Subsystem {
		attribute redefines mass = 2.5;
		attribute redefines alloc = 4.0;
		attribute redefines count = 3;
	}
	part b : Subsystem {
		attribute redefines mass = 1.5;
		attribute redefines count = 2;
	}
}
`

func computedFixture(t *testing.T, query string) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, computedBody+query)
}

func computedRows(t *testing.T, fixture executionFixture, query string) *RowSet {
	t.Helper()
	result, err := fixture.execute(t, query, Bindings{
		"root": {ElementValue(fixture.symbol(t, "system"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute %s: %v", query, err)
	}
	return result
}

func TestExecuteComputedArithmeticAndConcatenation(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass),
			Column(name = "double", expression = Subsystem::count * 2),
			Column(name = "label", expression = "sub: " + Element::name)
		)
	)
}
`)
	result := computedRows(t, fixture, "Margins")
	var names []string
	for _, column := range result.Columns() {
		names = append(names, column.Name())
	}
	if !slices.Equal(names, []string{"name", "margin", "double", "label"}) {
		t.Fatalf("columns = %v", names)
	}
	rows := result.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	margins := make([]float64, 0, 2)
	doubles := make([]int64, 0, 2)
	labels := make([]string, 0, 2)
	for _, row := range rows {
		cells := row.Cells()
		margin, ok := cells[1].Values()[0].Real()
		if !ok {
			t.Fatalf("margin cell = %+v", cells[1].Values())
		}
		double, ok := cells[2].Values()[0].Integer()
		if !ok {
			t.Fatalf("double cell = %+v", cells[2].Values())
		}
		label, ok := cells[3].Values()[0].String()
		if !ok {
			t.Fatalf("label cell = %+v", cells[3].Values())
		}
		margins = append(margins, margin)
		doubles = append(doubles, double)
		labels = append(labels, label)
		if !cells[1].Origin().Located() {
			t.Fatal("computed cells must retain provenance")
		}
	}
	if !slices.Equal(margins, []float64{1.5, -1.5}) {
		t.Fatalf("margins = %v", margins)
	}
	if !slices.Equal(doubles, []int64{6, 4}) {
		t.Fatalf("doubles = %v", doubles)
	}
	if !slices.Equal(labels, []string{"sub: a", "sub: b"}) {
		t.Fatalf("labels = %v", labels)
	}
}

func TestExecuteOrdersAndGroupsByComputedColumns(t *testing.T) {
	fixture := computedFixture(t, `
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = Descendants(source = root, maxDepth = 1),
			properties = ("name"),
			columns = (Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass))
		),
		property = "margin",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
`)
	result := computedRows(t, fixture, "Ordered")
	rows := result.Rows()
	var names []string
	for _, row := range rows {
		name, _ := row.Cells()[0].Values()[0].String()
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"b", "a"}) {
		t.Fatalf("ordered names = %v", names)
	}
}

func TestExecuteComputedColumnsAreDeterministic(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass))
	)
}
`)
	first := computedRows(t, fixture, "Margins")
	second := computedRows(t, fixture, "Margins")
	for i, row := range first.Rows() {
		left, _ := row.Cells()[0].Values()[0].Real()
		right, _ := second.Rows()[i].Cells()[0].Values()[0].Real()
		if left != right {
			t.Fatalf("row %d: %v != %v", i, left, right)
		}
	}
}

func TestExecuteComputedResultsAreImmutableToCallers(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::count * 2))
	)
}
`)
	result := computedRows(t, fixture, "Margins")
	rows := result.Rows()
	rows[0] = Row{}
	values := result.Rows()[0].Cells()[0].Values()
	if _, ok := values[0].Integer(); !ok {
		t.Fatal("row set must be immutable to callers")
	}
}

func TestExecuteComputedBareAbsentFeatureIsTyped(t *testing.T) {
	fixture := computedFixture(t, `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "alloc", expression = Subsystem::alloc))
	)
}`)
	_, err := fixture.execute(t, "Bad", Bindings{
		"root": {ElementValue(fixture.symbol(t, "system"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnAbsent {
		t.Fatalf("error = %v", err)
	}
	if executionError.Property != "alloc" || executionError.Target == "" {
		t.Fatalf("error provenance = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("absent column errors must retain source provenance")
	}
}

func TestExecuteComputedColumnSkipsUnrelatedSameNamedFeatures(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Tank {
	attribute mass : Real;
}
part def Report {
	attribute mass : String;
}
part mixed {
	part tank : Tank {
		attribute redefines mass = 3.5;
	}
	part report : Report {
		attribute redefines mass = "heavy";
	}
}
calc def Masses :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "m", expression = Tank::mass ?? 0.0))
	)
}`)
	result, err := fixture.execute(t, "Masses", Bindings{
		"root": {ElementValue(fixture.symbol(t, "mixed"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Masses: %v", err)
	}
	values := make(map[string]Value)
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		values[name] = row.Cells()[1].Values()[0]
	}
	if mass, ok := values["tank"].Real(); !ok || mass != 3.5 {
		t.Fatalf("tank mass = %+v", values["tank"])
	}
	// Report::mass is unrelated to Tank::mass; the ?? default must apply.
	if mass, ok := values["report"].Real(); !ok || mass != 0.0 {
		t.Fatalf("report mass = %+v", values["report"])
	}
}

func TestExecuteComputedDefaultsWhenNoRowConforms(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Tank {
	attribute level : Real;
}
part def Crate;
part yard {
	part a : Crate;
	part b : Crate;
}
calc def Levels :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "level", expression = Tank::level ?? 0.0))
	)
}`)
	result, err := fixture.execute(t, "Levels", Bindings{
		"root": {ElementValue(fixture.symbol(t, "yard"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Levels: %v", err)
	}
	for _, row := range result.Rows() {
		if level, ok := row.Cells()[1].Values()[0].Real(); !ok || level != 0.0 {
			t.Fatalf("level cell = %+v", row.Cells()[1].Values())
		}
	}
}

func TestExecuteComputedMultiValuedParameterIsTyped(t *testing.T) {
	fixture := computedFixture(t, `
calc def Bad :> Query {
	in root : Element;
	in factors : Real[0..*];
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "f", expression = factors))
	)
}`)
	_, err := fixture.execute(t, "Bad", Bindings{
		"root":    {ElementValue(fixture.symbol(t, "system"))},
		"factors": {RealValue(1.0), RealValue(2.0)},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnCardinality {
		t.Fatalf("error = %v", err)
	}
	if executionError.Property != "f" || executionError.Actual != "2" {
		t.Fatalf("error provenance = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("cardinality errors must retain source provenance")
	}
}

func TestExecuteComputedMetaclassFeatureReadsDeclaration(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Widget;
part floor {
	abstract part a : Widget;
	attribute tag : String;
}
calc def Abstracts :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "abs", expression = SysML::PartUsage::isAbstract ?? false))
	)
}`)
	result, err := fixture.execute(t, "Abstracts", Bindings{
		"root": {ElementValue(fixture.symbol(t, "floor"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Abstracts: %v", err)
	}
	values := make(map[string]bool)
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		abs, ok := row.Cells()[1].Values()[0].Boolean()
		if !ok {
			t.Fatalf("abs cell for %s = %+v", name, row.Cells()[1].Values())
		}
		values[name] = abs
	}
	if !values["a"] {
		t.Fatal("abstract part must read isAbstract = true")
	}
	// The attribute's metaclass is not a PartUsage; the ?? default applies.
	if values["tag"] {
		t.Fatal("non-conforming row must take the ?? default")
	}
}

func TestExecuteComputedMetaclassModifierFlags(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Widget;
part floor {
	variation part v : Widget;
	ref part r : Widget;
	part p : Widget;
	attribute tag : String;
}
calc def Flags :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "variation", expression = SysML::Usage::isVariation ?? false),
			Column(name = "reference", expression = SysML::Usage::isReference ?? false)
		)
	)
}`)
	result, err := fixture.execute(t, "Flags", Bindings{
		"root": {ElementValue(fixture.symbol(t, "floor"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Flags: %v", err)
	}
	type flags struct{ variation, reference bool }
	values := make(map[string]flags)
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		variation, ok := row.Cells()[1].Values()[0].Boolean()
		if !ok {
			t.Fatalf("variation cell for %s = %+v", name, row.Cells()[1].Values())
		}
		reference, ok := row.Cells()[2].Values()[0].Boolean()
		if !ok {
			t.Fatalf("reference cell for %s = %+v", name, row.Cells()[2].Values())
		}
		values[name] = flags{variation, reference}
	}
	want := map[string]flags{
		"v": {variation: true},
		"r": {reference: true},
		"p": {},
		// Attribute usages are implicitly referential.
		"tag": {reference: true},
	}
	for name, expected := range want {
		if values[name] != expected {
			t.Fatalf("%s flags = %+v, want %+v", name, values[name], expected)
		}
	}
}

// An enumeration definition reads isVariation = true without declaring the
// modifier, as the metamodel derives it.
func TestExecuteComputedEnumerationVariationFlag(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package Palette {
	enum def Color { red; green; }
	attribute def Plain;
}
calc def Variations :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "variation", expression = SysML::Definition::isVariation ?? false))
	)
}`)
	result, err := fixture.execute(t, "Variations", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Palette"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Variations: %v", err)
	}
	values := make(map[string]bool)
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		value, ok := row.Cells()[1].Values()[0].Boolean()
		if !ok {
			t.Fatalf("variation cell for %s = %+v", name, row.Cells()[1].Values())
		}
		values[name] = value
	}
	if !values["Color"] || values["Plain"] {
		t.Fatalf("isVariation = %+v, want Color true and Plain false", values)
	}
}

func TestExecuteComputedMultiValuedFeatureIsTyped(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Box {
	attribute sizes : Real[0..*];
}
part shed {
	part b : Box {
		attribute redefines sizes = (1.0, 2.0);
	}
}
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "s", expression = Box::sizes))
	)
}`)
	_, err := fixture.execute(t, "Bad", Bindings{
		"root": {ElementValue(fixture.symbol(t, "shed"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnCardinality {
		t.Fatalf("error = %v", err)
	}
	if executionError.Property != "s" || executionError.Actual != "2" {
		t.Fatalf("error provenance = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("cardinality errors must retain source provenance")
	}
}

func TestExecuteComputedUserFeatureNamedLikeMetadata(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Widget {
	attribute name : String;
}
part shelf {
	part gadget : Widget {
		attribute redefines name = "custom label";
	}
}
calc def Labels :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "label", expression = Widget::name))
	)
}`)
	result, err := fixture.execute(t, "Labels", Bindings{
		"root": {ElementValue(fixture.symbol(t, "shelf"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Labels: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	cells := rows[0].Cells()
	metadata, _ := cells[0].Values()[0].String()
	label, ok := cells[1].Values()[0].String()
	if !ok || label != "custom label" {
		t.Fatalf("label cell = %+v", cells[1].Values())
	}
	if metadata == label {
		t.Fatalf("metadata name %q must stay distinct from the declared feature", metadata)
	}
}

func TestExecuteComputedColumnFailuresAreTyped(t *testing.T) {
	cases := []struct {
		name  string
		query string
		kind  ErrorKind
	}{
		{
			name: "division by zero",
			kind: ErrorColumnDivisionByZero,
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::count / 0))
	)
}`,
		},
		{
			name: "absent operand without default",
			kind: ErrorColumnOperand,
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::alloc - Subsystem::mass))
	)
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := computedFixture(t, test.query)
			_, err := fixture.execute(t, "Bad", Bindings{
				"root": {ElementValue(fixture.symbol(t, "system"))},
			}, Options{})
			var executionError *Error
			if !errors.As(err, &executionError) || executionError.Kind != test.kind {
				t.Fatalf("error = %v", err)
			}
			if executionError.Property != "bad" || executionError.Target == "" {
				t.Fatalf("error provenance = %+v", executionError)
			}
			if !executionError.Origin.Located() {
				t.Fatal("computed column errors must retain source provenance")
			}
		})
	}
}
