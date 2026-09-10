package queryexec

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// quantityBody is the units repro: stages whose masses are quantities, with a
// gram-denominated stage, a length, and a mass no constant fold can reach.
const quantityBody = `
private import SI::*;
private import ISQ::*;

part def Stage {
	attribute mass :> ISQ::mass;
	attribute length :> ISQ::length;
}
part def FirstStage :> Stage {
	attribute :>> mass = 2290000 [kg];
	attribute :>> length = 42 [m];
}
part def UpperStage :> Stage {
	attribute :>> mass = 119000 [kg];
	attribute :>> length = 18 [m];
}
part def Probe :> Stage {
	attribute :>> mass = 500000 [g];
	attribute :>> length = 1 [m];
}
part def Payload :> Stage {
	action weigh { in reading : Real; }
	attribute :>> mass = weigh.reading [kg];
}
part def Cargo :> Stage {
	attribute dryMass :> ISQ::mass;
	attribute propellantMass :> ISQ::mass;
	attribute :>> mass = dryMass + propellantMass;
}
part rocket {
	part s1 : FirstStage;
	part s2 : UpperStage;
}
part fleet {
	part s1 : FirstStage;
	part probe : Probe;
}
part manifest {
	part s1 : FirstStage;
	part payload : Payload;
}
part hold {
	part s1 : FirstStage;
	part cargo : Cargo;
}
`

func quantityFixture(t *testing.T, query string) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, quantityBody+query)
}

func quantityRows(t *testing.T, fixture executionFixture, query, root string) *RowSet {
	t.Helper()
	result, err := fixture.execute(t, query, Bindings{
		"root": {ElementValue(fixture.symbol(t, root))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute %s: %v", query, err)
	}
	return result
}

// cellTexts renders one column of a result the way the CLI row listing does.
func cellTexts(t *testing.T, result *RowSet, column int) []string {
	t.Helper()
	var out []string
	for _, row := range result.Rows() {
		values := row.Cells()[column].Values()
		if len(values) != 1 {
			t.Fatalf("cell %d has %d values, want 1", column, len(values))
		}
		value := values[0]
		if quantity, ok := value.Quantity(); ok {
			out = append(out, quantity.String())
			continue
		}
		if text, ok := value.String(); ok {
			out = append(out, text)
			continue
		}
		if real, ok := value.Real(); ok {
			out = append(out, semantics.FormatReal(real))
			continue
		}
		t.Fatalf("cell %d holds an unexpected %s", column, value.Kind())
	}
	return out
}

func executionError(t *testing.T, err error, kind ErrorKind) *Error {
	t.Helper()
	var executionErr *Error
	if !errors.As(err, &executionErr) || executionErr.Kind != kind {
		t.Fatalf("error = %v, want %s", err, kind)
	}
	return executionErr
}

const massesQuery = `
calc def Masses :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "mass")
	)
}
`

// TestExecuteProjectsQuantityValues: the repro projects unit-bearing masses as
// quantities that keep their magnitude and unit.
func TestExecuteProjectsQuantityValues(t *testing.T) {
	fixture := quantityFixture(t, massesQuery)
	result := quantityRows(t, fixture, "Masses", "rocket")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"s1", "s2"}) {
		t.Fatalf("names = %v", got)
	}
	if got := cellTexts(t, result, 1); !slices.Equal(got, []string{"2290000 [kg]", "119000 [kg]"}) {
		t.Fatalf("masses = %v", got)
	}
	mass := result.Rows()[0].Cells()[1].Values()[0]
	if mass.Kind() != ValueQuantity {
		t.Fatalf("mass kind = %s, want quantity", mass.Kind())
	}
	magnitude, ok := mass.Magnitude()
	if n, isInt := magnitude.Integer(); !ok || !isInt || n != 2290000 {
		t.Fatalf("magnitude = %+v, want Integer 2290000", magnitude)
	}
	if !mass.Origin().Located() {
		t.Fatal("quantity cells must retain provenance")
	}
}

// TestQuantityCellsAreImmutable: mutating a quantity read from a result leaves
// the stored value, factors and powers included, as it was.
func TestQuantityCellsAreImmutable(t *testing.T) {
	fixture := quantityFixture(t, massesQuery)
	result := quantityRows(t, fixture, "Masses", "rocket")
	read := func() semantics.Quantity {
		quantity, ok := result.Rows()[0].Cells()[1].Values()[0].Quantity()
		if !ok {
			t.Fatal("mass is not a quantity")
		}
		return quantity
	}
	before := read()
	if len(before.Unit.Term.Factors) == 0 || len(before.Unit.Product.Powers) == 0 {
		t.Fatalf("fixture quantity %s has no factors or powers to mutate", before.String())
	}
	mutated := read()
	mutated.Num.Int = 1
	mutated.Unit.Text = "lb"
	mutated.Unit.Term.Scale = semantics.UnitScale(7)
	mutated.Unit.Term.Factors[0].Exponent = 9
	mutated.Unit.Product.Powers[0].Exponent = 9
	mutated.Unit.Product.Powers[0].Name = "lb"

	after := read()
	if after.String() != before.String() {
		t.Fatalf("stored quantity became %s, was %s", after.String(), before.String())
	}
	if !after.Unit.Term.Same(before.Unit.Term) || !after.Unit.Product.Equal(before.Unit.Product) {
		t.Fatalf("stored unit changed: term %+v -> %+v, product %+v -> %+v",
			before.Unit.Term, after.Unit.Term, before.Unit.Product, after.Unit.Product)
	}
	if after.Unit.Term.Factors[0].Exponent == 9 || after.Unit.Product.Powers[0].Exponent == 9 {
		t.Fatal("stored unit shares slice storage with the returned copy")
	}
}

// TestExecuteFiltersQuantitiesByBareMagnitude: a bare numeric value compares
// against the magnitude in each row's own unit.
func TestExecuteFiltersQuantitiesByBareMagnitude(t *testing.T) {
	fixture := quantityFixture(t, `
calc def Heavy :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			'feature' = "mass",
			operator = ">=",
			value = "1000000"
		),
		properties = ("name", "mass")
	)
}
calc def Exact :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			'feature' = "mass",
			operator = "=",
			value = "119000"
		),
		properties = ("name")
	)
}
calc def UnitInValue :> Query {
	in root : Element;
	WhereFeature(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		'feature' = "mass",
		operator = "=",
		value = "119000 [kg]"
	)
}
`)
	heavy := quantityRows(t, fixture, "Heavy", "rocket")
	if got := cellTexts(t, heavy, 0); !slices.Equal(got, []string{"s1"}) {
		t.Fatalf("heavy names = %v", got)
	}
	if got := cellTexts(t, heavy, 1); !slices.Equal(got, []string{"2290000 [kg]"}) {
		t.Fatalf("heavy masses = %v", got)
	}
	exact := quantityRows(t, fixture, "Exact", "rocket")
	if got := cellTexts(t, exact, 0); !slices.Equal(got, []string{"s2"}) {
		t.Fatalf("exact names = %v", got)
	}
	_, err := fixture.execute(t, "UnitInValue", Bindings{
		"root": {ElementValue(fixture.symbol(t, "rocket"))},
	}, Options{})
	executionError(t, err, ErrorInvalidArgument)
}

const orderedQuery = `
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass")
		),
		property = "mass",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
calc def ByLength :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass", "length")
		),
		property = "length",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
`

// TestExecuteOrdersQuantitiesAcrossCommensurableUnits: sorting converts
// commensurable units, so 500000 g sorts before 2290000 kg and both keep their
// spelling.
func TestExecuteOrdersQuantitiesAcrossCommensurableUnits(t *testing.T) {
	fixture := quantityFixture(t, orderedQuery)
	rocket := quantityRows(t, fixture, "Ordered", "rocket")
	if got := cellTexts(t, rocket, 0); !slices.Equal(got, []string{"s2", "s1"}) {
		t.Fatalf("rocket order = %v", got)
	}
	fleet := quantityRows(t, fixture, "Ordered", "fleet")
	if got := cellTexts(t, fleet, 0); !slices.Equal(got, []string{"probe", "s1"}) {
		t.Fatalf("fleet order = %v", got)
	}
	if got := cellTexts(t, fleet, 1); !slices.Equal(got, []string{"500000 [g]", "2290000 [kg]"}) {
		t.Fatalf("fleet masses = %v", got)
	}
	lengths := quantityRows(t, fixture, "ByLength", "rocket")
	if got := cellTexts(t, lengths, 2); !slices.Equal(got, []string{"42 [m]", "18 [m]"}) {
		t.Fatalf("lengths = %v", got)
	}
}

// TestExecuteRefusesToOrderIncommensurableQuantities: keys in kg and m are a
// typed error naming both units, never a silent ordering by magnitude.
func TestExecuteRefusesToOrderIncommensurableQuantities(t *testing.T) {
	fixture := quantityFixture(t, `
part mixed {
	part s1 : FirstStage;
	part rod {
		attribute mass = 3 [m];
	}
}
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass")
		),
		property = "mass",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
`)
	_, err := fixture.execute(t, "Ordered", Bindings{
		"root": {ElementValue(fixture.symbol(t, "mixed"))},
	}, Options{})
	executionErr := executionError(t, err, ErrorInvalidOrder)
	if executionErr.Expected != "kg" || executionErr.Actual != "m" {
		t.Fatalf("units = %q and %q, want kg and m", executionErr.Expected, executionErr.Actual)
	}
	want := "query Observatory::Ordered cannot order property mass across incommensurable units kg and m"
	if executionErr.Error() != want {
		t.Fatalf("message = %q, want %q", executionErr.Error(), want)
	}
}

// TestExecuteOrdersLargeIntegerQuantitiesExactly: Integer magnitudes above 2^53,
// which one float64 could not tell apart, order exactly in one unit and across
// a conversion.
func TestExecuteOrdersLargeIntegerQuantitiesExactly(t *testing.T) {
	fixture := quantityFixture(t, `
part heavy {
	part above {
		attribute mass = 9007199254740993 [kg];
	}
	part at {
		attribute mass = 9007199254740992 [kg];
	}
	part grams {
		attribute mass = 9007199254740992000 [g];
	}
}
calc def Ascending :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass")
		),
		property = "mass",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
calc def Descending :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass")
		),
		property = "mass",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
`)
	ascending := quantityRows(t, fixture, "Ascending", "heavy")
	if got := cellTexts(t, ascending, 0); !slices.Equal(got, []string{"at", "grams", "above"}) {
		t.Fatalf("ascending order = %v", got)
	}
	descending := quantityRows(t, fixture, "Descending", "heavy")
	if got := cellTexts(t, descending, 0); !slices.Equal(got, []string{"above", "at", "grams"}) {
		t.Fatalf("descending order = %v", got)
	}
}

// TestExecuteComputesQuantityColumns: column arithmetic keeps units, composes
// them under * and /, and a ratio of like quantities is a bare number.
func TestExecuteComputesQuantityColumns(t *testing.T) {
	fixture := quantityFixture(t, `
calc def Derived :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name"),
		columns = (
			Column(name = "twice", expression = Stage::mass * 2),
			Column(name = "tonnes", expression = Stage::mass / 1000),
			Column(name = "half", expression = Stage::mass - Stage::mass / 2),
			Column(name = "density", expression = Stage::mass / Stage::length),
			Column(name = "ratio", expression = Stage::length / Stage::length),
			Column(name = "sum", expression = Stage::mass + Stage::mass)
		)
	)
}
`)
	result := quantityRows(t, fixture, "Derived", "rocket")
	cases := []struct {
		column int
		want   []string
	}{
		{1, []string{"4580000 [kg]", "238000 [kg]"}},
		{2, []string{"2290.0 [kg]", "119.0 [kg]"}},
		{3, []string{"1145000.0 [kg]", "59500.0 [kg]"}},
		{4, []string{"54523.80952380953 [SI::'kg⋅m⁻¹']", "6611.111111111111 [SI::'kg⋅m⁻¹']"}},
		{5, []string{"1.0", "1.0"}},
		{6, []string{"4580000 [kg]", "238000 [kg]"}},
	}
	for _, tc := range cases {
		if got := cellTexts(t, result, tc.column); !slices.Equal(got, tc.want) {
			t.Errorf("column %s = %v, want %v", result.Columns()[tc.column].Name(), got, tc.want)
		}
	}
}

// TestExecuteRefusesIncommensurableColumnArithmetic: `mass + length` is a
// typed error naming the column, row and both units.
func TestExecuteRefusesIncommensurableColumnArithmetic(t *testing.T) {
	fixture := quantityFixture(t, `
calc def Nonsense :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name"),
		columns = (Column(name = "nonsense", expression = Stage::mass + Stage::length))
	)
}
`)
	_, err := fixture.execute(t, "Nonsense", Bindings{
		"root": {ElementValue(fixture.symbol(t, "rocket"))},
	}, Options{})
	executionErr := executionError(t, err, ErrorColumnIncommensurable)
	if executionErr.Property != "nonsense" || executionErr.Target != "Observatory::rocket::s1" {
		t.Fatalf("column %q for %q, want nonsense for Observatory::rocket::s1", executionErr.Property, executionErr.Target)
	}
	if executionErr.Actual != "kg and m" {
		t.Fatalf("units = %q, want %q", executionErr.Actual, "kg and m")
	}
}

// TestExecuteNamesTheRowOfAnUnevaluableQuantity: a mass that depends on an
// action's `in` parameter is a typed error naming the query, the feature, the
// row and the parameter no declaration binds.
func TestExecuteNamesTheRowOfAnUnevaluableQuantity(t *testing.T) {
	fixture := quantityFixture(t, massesQuery)
	_, err := fixture.execute(t, "Masses", Bindings{
		"root": {ElementValue(fixture.symbol(t, "manifest"))},
	}, Options{})
	executionErr := executionError(t, err, ErrorUnevaluableFeature)
	want := "query Observatory::Masses cannot evaluate feature mass of Observatory::manifest::payload: "
	if !strings.HasPrefix(executionErr.Error(), want) || !strings.Contains(executionErr.Error(), "reading") {
		t.Fatalf("message = %q, want prefix %q naming reading", executionErr.Error(), want)
	}
	if !errors.Is(err, runtime.ErrNoValue) {
		t.Fatalf("error = %v, want the runtime's no-value cause", err)
	}
}

// TestExecuteReadsAMassOverUnboundFeaturesAsAbsent: a mass declared over
// features nothing binds is an empty cell, as a value-less feature is.
func TestExecuteReadsAMassOverUnboundFeaturesAsAbsent(t *testing.T) {
	fixture := quantityFixture(t, massesQuery)
	result := quantityRows(t, fixture, "Masses", "hold")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"s1", "cargo"}) {
		t.Fatalf("names = %v", got)
	}
	if values := result.Rows()[1].Cells()[1].Values(); len(values) != 0 {
		t.Fatalf("cargo mass = %v, want absent", values)
	}
}

// boundQuantity folds a quantity expression in the fixture's package for a binding.
func boundQuantity(t *testing.T, fixture executionFixture, expr string) Value {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	quantity, ok := fixture.model.EvalQuantity(fixture.symbol(t, "rocket").Scope, node)
	if !ok {
		t.Fatalf("%q does not fold to a quantity", expr)
	}
	return QuantityValue(quantity)
}

// TestExecuteBindsQuantityParameters: a quantity binds to a parameter typed by
// a quantity value type of its dimension, and flows into column arithmetic;
// another dimension, or a parameter outside the quantity types, is a typed error.
func TestExecuteBindsQuantityParameters(t *testing.T) {
	fixture := quantityFixture(t, `
calc def Margin :> Query {
	in root : Element;
	in budget : MassValue;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name"),
		columns = (Column(name = "margin", expression = budget - Stage::mass))
	)
}
calc def Labelled :> Query {
	in root : Element;
	in label : String;
	WhereFeature(source = Descendants(source = root, maxDepth = 1), 'feature' = "name", operator = "=", value = label)
}
`)
	root := ElementValue(fixture.symbol(t, "rocket"))
	result, err := fixture.execute(t, "Margin", Bindings{
		"root":   {root},
		"budget": {boundQuantity(t, fixture, "2500000 [kg]")},
	}, Options{})
	if err != nil {
		t.Fatalf("execute with a mass binding: %v", err)
	}
	if got := cellTexts(t, result, 1); !slices.Equal(got, []string{"210000 [kg]", "2381000 [kg]"}) {
		t.Fatalf("margins = %v", got)
	}
	_, err = fixture.execute(t, "Margin", Bindings{
		"root":   {root},
		"budget": {boundQuantity(t, fixture, "2500 [m]")},
	}, Options{})
	executionErr := executionError(t, err, ErrorBindingType)
	if executionErr.Parameter != "budget" || executionErr.Actual != string(ValueQuantity) {
		t.Fatalf("length-for-mass binding error = %v", err)
	}
	_, err = fixture.execute(t, "Labelled", Bindings{
		"root":  {root},
		"label": {boundQuantity(t, fixture, "1 [kg]")},
	}, Options{})
	executionErr = executionError(t, err, ErrorBindingType)
	if executionErr.Parameter != "label" || executionErr.Actual != string(ValueQuantity) {
		t.Fatalf("quantity-for-string binding error = %v", err)
	}
}
