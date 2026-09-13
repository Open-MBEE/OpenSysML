package semantics_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// quantityModel is the units repro: stages whose masses are quantities, plus
// attributes covering every fold the const-folder is expected to make.
const quantityModel = `private import ScalarValues::*;
private import SI::*;
private import ISQ::*;

package Q {
	attribute base : Integer = 2;
	enum def Color { red; blue; }
	part def Stage {
		attribute mass :> ISQ::mass;
	}
	part def FirstStage :> Stage {
		attribute :>> mass = 2290000 [kg];
	}
	part def Folds {
		attribute scaled = 2 [kg] * 3;
		attribute summed = 1 [km] + 500 [m];
		attribute negated = -(3 [kg]);
		attribute affirmed = +(3 [kg]);
		attribute chosen = if 5 [kg] > 4000 [g] ? 1 [kg] else 2 [kg];
		attribute quotient = 6 [m] / 2 [s];
		attribute ratio = 4 [m] / 2 [m];
		attribute heavier = 5 [kg] > 4000 [g];
		attribute mixed = 1 [kg] + 1 [m];
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass;
		attribute total = dryMass + propellantMass;
		attribute alias = dryMass;
		attribute unit = SI::kg;
		attribute chained = Stage::mass;
		attribute shared = base;
		attribute color : Color = Color::red;
		attribute kind = Stage;
	}
}`

func quantityFixture(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("q.sysml", []byte(quantityModel)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument("q.sysml", root)
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// fold parses expr at the fixture's document root and folds it as a quantity.
func fold(t *testing.T, m *semantics.Model, idx *symbols.Index, expr string) semantics.Quantity {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	q, ok := m.EvalQuantity(idx.DocumentRoot("q.sysml"), node)
	if !ok {
		t.Fatalf("%q does not fold to a quantity", expr)
	}
	return q
}

func declaredValue(t *testing.T, m *semantics.Model, idx *symbols.Index, fqn, property string) symbols.FilterValue {
	t.Helper()
	matches := symbols.PreferDeclared(idx.LookupQualified(fqn))
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	values, present := m.DeclaredFeatureValues(matches[0], property)
	if !present {
		t.Fatalf("%s has no declared value for %s", fqn, property)
	}
	if len(values) != 1 {
		t.Fatalf("%s.%s has %d values, want 1", fqn, property, len(values))
	}
	return values[0]
}

// TestDeclaredQuantityValueKeepsItsUnit: `2290000 [kg]` folds to a quantity
// carrying the magnitude as written, the unit's spelling, and its reduction.
func TestDeclaredQuantityValueKeepsItsUnit(t *testing.T) {
	m, idx := quantityFixture(t)
	value := declaredValue(t, m, idx, "Q::FirstStage", "mass")
	if value.Kind != symbols.FilterValueQuantity {
		t.Fatalf("kind = %v, want quantity", value.Kind)
	}
	q, ok := semantics.QuantityOf(value)
	if !ok {
		t.Fatal("quantity value carries no quantity")
	}
	if q.Num.Kind != semantics.ValInt || q.Num.Int != 2290000 {
		t.Errorf("magnitude = %+v, want Integer 2290000", q.Num)
	}
	if q.Unit.Text != "kg" {
		t.Errorf("unit text = %q, want kg", q.Unit.Text)
	}
	if q.String() != "2290000 [kg]" {
		t.Errorf("String() = %q, want %q", q.String(), "2290000 [kg]")
	}
	if q.Unit.Term.String() != "1000·gram" {
		t.Errorf("unit term = %s, want 1000·gram", q.Unit.Term)
	}
}

// TestQuantityExpressionsFold: constant expressions over quantities fold with
// the runtime's unit rules, and a cancelled or compared unit leaves a scalar.
func TestQuantityExpressionsFold(t *testing.T) {
	m, idx := quantityFixture(t)
	cases := []struct {
		property string
		kind     symbols.FilterValueKind
		text     string
	}{
		{"scaled", symbols.FilterValueQuantity, "6 [kg]"},
		{"summed", symbols.FilterValueQuantity, "1.5 [km]"},
		{"negated", symbols.FilterValueQuantity, "-3 [kg]"},
		{"affirmed", symbols.FilterValueQuantity, "3 [kg]"},
		{"chosen", symbols.FilterValueQuantity, "1 [kg]"},
		{"quotient", symbols.FilterValueQuantity, "3.0 [m/s]"},
		{"ratio", symbols.FilterValueReal, "2.0"},
		{"heavier", symbols.FilterValueBool, "true"},
	}
	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			value := declaredValue(t, m, idx, "Q::Folds", tc.property)
			if value.Kind != tc.kind {
				t.Fatalf("kind = %v, want %v", value.Kind, tc.kind)
			}
			var got string
			switch value.Kind {
			case symbols.FilterValueQuantity:
				q, _ := semantics.QuantityOf(value)
				got = q.String()
			case symbols.FilterValueReal:
				got = semantics.FormatReal(value.Real)
			case symbols.FilterValueBool:
				if value.Bool {
					got = "true"
				} else {
					got = "false"
				}
			}
			if got != tc.text {
				t.Errorf("%s = %s, want %s", tc.property, got, tc.text)
			}
		})
	}
}

// TestUnfoldableQuantityValuesStayUnknown: a sum of incommensurable units and
// an expression over unbound features are not constants, and say so.
func TestUnfoldableQuantityValuesStayUnknown(t *testing.T) {
	m, idx := quantityFixture(t)
	for _, property := range []string{"mixed", "total"} {
		value := declaredValue(t, m, idx, "Q::Folds", property)
		if value.Kind != symbols.FilterValueUnknown {
			t.Errorf("%s folded to %v, want unknown", property, value.Kind)
		}
	}
}

// TestDeclaredReferenceToAFeatureIsNotAConstant: a value that names an attribute
// reads that attribute — of the carrier, or of the package — so it does not
// fold; a value naming a unit, an enumeration literal or a definition is that
// element.
func TestDeclaredReferenceToAFeatureIsNotAConstant(t *testing.T) {
	m, idx := quantityFixture(t)
	for _, property := range []string{"alias", "chained", "shared"} {
		if value := declaredValue(t, m, idx, "Q::Folds", property); value.Kind != symbols.FilterValueUnknown {
			t.Errorf("%s folded to %v, want unknown", property, value.Kind)
		}
	}
	for property, want := range map[string]string{"unit": "SI::kilogram", "color": "Q::Color::red", "kind": "Q::Stage"} {
		if value := declaredValue(t, m, idx, "Q::Folds", property); value.Kind != symbols.FilterValueRef || value.RefFQN != want {
			t.Errorf("%s = %+v, want a reference to %s", property, value, want)
		}
	}
}

// TestCompareQuantitiesConvertsCommensurableUnits: ordering converts the right
// operand into the left unit, so 1 kg and 1000 g are equal and 999 g is less.
func TestCompareQuantitiesConvertsCommensurableUnits(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	grams := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [g]", n)) }
	equal, err := semantics.EqualQuantities(ast.OpEq, kilograms(1), grams(1000))
	if err != nil || !equal {
		t.Fatalf("1 kg == 1000 g: %v, %v", equal, err)
	}
	less, err := semantics.CompareQuantities(ast.OpLt, grams(999), kilograms(1))
	if err != nil || !less {
		t.Fatalf("999 g < 1 kg: %v, %v", less, err)
	}
	greater, err := semantics.CompareQuantities(ast.OpGt, grams(999), kilograms(1))
	if err != nil || greater {
		t.Fatalf("999 g > 1 kg: %v, %v", greater, err)
	}
}

// TestCompareMagnitudesKeepsLargeIntegersExact: Integer magnitudes above 2^53,
// which one float64 cannot tell apart, stay ordered in one unit and across a
// whole scale ratio; a Real operand or a fractional scale still compares as
// float64, so 1 m and 100 cm remain equal.
func TestCompareMagnitudesKeepsLargeIntegersExact(t *testing.T) {
	m, idx := quantityFixture(t)
	q := func(expr string) semantics.Quantity { return fold(t, m, idx, expr) }
	cases := []struct {
		left, right string
		want        int
	}{
		{"9007199254740993 [kg]", "9007199254740992 [kg]", 1},
		{"9007199254740992 [kg]", "9007199254740993 [kg]", -1},
		{"9007199254740992 [kg]", "9007199254740992 [kg]", 0},
		{"9007199254740993 [kg]", "9007199254740992000 [g]", 1},
		{"9007199254740992000 [g]", "9007199254740993 [kg]", -1},
		{"9007199254740992000 [g]", "9007199254740992 [kg]", 0},
		{"9007199254740993 [kg]", "9007199254740992.0 [kg]", 0},
		{"1 [m]", "100 [cm]", 0},
		{"1 [m]", "101 [cm]", -1},
	}
	for _, c := range cases {
		got, err := semantics.CompareMagnitudes(q(c.left), q(c.right))
		if err != nil || got != c.want {
			t.Errorf("CompareMagnitudes(%s, %s) = %d, %v; want %d", c.left, c.right, got, err, c.want)
		}
	}
}

// TestIncommensurableQuantitiesAreTypedErrors: comparison and addition across
// dimensions report ErrIncommensurableUnits instead of comparing magnitudes.
func TestIncommensurableQuantitiesAreTypedErrors(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	metres := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [m]", n)) }
	if _, err := semantics.CompareQuantities(ast.OpLt, kilograms(1), metres(2)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg < 2 m: err = %v, want ErrIncommensurableUnits", err)
	}
	if _, err := semantics.EqualQuantities(ast.OpEq, kilograms(1), metres(1)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg == 1 m: err = %v, want ErrIncommensurableUnits", err)
	}
	if _, err := semantics.QuantityBinary(ast.OpAdd, kilograms(1), metres(1)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg + 1 m: err = %v, want ErrIncommensurableUnits", err)
	}
}

// TestBareZeroComparesInTheQuantitysUnit: a bare zero is the null quantity of
// every dimension in a comparison, while a non-zero bare number, a sum with
// zero and a zero naming another unit stay incommensurable.
func TestBareZeroComparesInTheQuantitysUnit(t *testing.T) {
	m, idx := quantityFixture(t)
	q := func(expr string) semantics.Quantity { return fold(t, m, idx, expr) }
	holds := []struct {
		op          ast.OperatorKind
		left, right string
		want        bool
	}{
		{ast.OpGt, "1 [m]", "0", true},
		{ast.OpLt, "0", "1 [m]", true},
		{ast.OpGe, "0 [m]", "0.0", true},
		{ast.OpLe, "1 [m]", "0", false},
		{ast.OpEq, "0 [kg]", "0", true},
		{ast.OpNeq, "-0", "1 [kg]", true},
	}
	for _, c := range holds {
		got, err := semantics.QuantityBinary(c.op, q(c.left), q(c.right))
		if err != nil || got.Num.Kind != semantics.ValBool || got.Num.Bool != c.want {
			t.Errorf("%s %v %s = %v, %v; want %v", c.left, c.op, c.right, got.Num, err, c.want)
		}
	}
	fails := []struct {
		op          ast.OperatorKind
		left, right string
	}{
		{ast.OpGt, "1 [m]", "1"},
		{ast.OpEq, "1", "1 [m]"},
		{ast.OpAdd, "1 [m]", "0"},
		{ast.OpLt, "0 [kg]", "1 [m]"},
		{ast.OpEq, "0 [m]", "0 [kg]"},
	}
	for _, c := range fails {
		if _, err := semantics.QuantityBinary(c.op, q(c.left), q(c.right)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
			t.Errorf("%s %v %s: err = %v, want ErrIncommensurableUnits", c.left, c.op, c.right, err)
		}
	}
}

// TestQuantityArithmeticFailures: a zero divisor and an Integer overflow are
// typed errors, never a silent infinity or wrap-around.
func TestQuantityArithmeticFailures(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int64) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	metres := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [m]", n)) }
	zero := semantics.Quantity{Num: semantics.Value{Kind: semantics.ValInt}, Unit: semantics.UnitOne()}
	if _, err := semantics.QuantityBinary(ast.OpDiv, kilograms(1), zero); !errors.Is(err, semantics.ErrDivisionByZero) {
		t.Errorf("1 kg / 0: err = %v, want ErrDivisionByZero", err)
	}
	if _, err := semantics.QuantityBinary(ast.OpMul, kilograms(1<<62), kilograms(4)); !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Errorf("2^62 kg * 4 kg: err = %v, want ErrArithmeticOverflow", err)
	}
	exponent := semantics.Quantity{Num: semantics.Value{Kind: semantics.ValInt, Int: 2}, Unit: semantics.UnitOne()}
	squared, err := semantics.QuantityBinary(ast.OpPow, metres(3), exponent)
	if err != nil || squared.String() != "9 [m**2]" {
		t.Errorf("3 m ^ 2 = %v, %v; want 9 [m**2]", squared.String(), err)
	}
	if _, err := semantics.QuantityBinary(ast.OpPow, metres(3), metres(2)); !errors.Is(err, semantics.ErrQuantityOperand) {
		t.Errorf("3 m ^ 2 m: err = %v, want ErrQuantityOperand", err)
	}
}

// parseExpr parses expr as an expression, failing the test on a diagnostic.
func parseExpr(t *testing.T, expr string) ast.Node {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	return node
}

// TestMeasurementScaleIsNoUnitTerm: a measurement scale is refused in unit
// position and never enters a composed unit term as a factor.
func TestMeasurementScaleIsNoUnitTerm(t *testing.T) {
	m, idx := quantityFixture(t)
	scope := idx.DocumentRoot("q.sysml")
	for _, expr := range []string{"SI::'°C_abs'", "Time::UTC", "SI::'°C_abs' * s", "s / Time::UTC", "SI::'°C_abs' ** 2"} {
		if term, err := m.UnitTermOfExpr(scope, parseExpr(t, expr)); !errors.Is(err, semantics.ErrNotAUnit) {
			t.Errorf("UnitTermOfExpr(%s) = %v, %v; want ErrNotAUnit", expr, term, err)
		} else if !strings.Contains(err.Error(), "is a measurement scale") {
			t.Errorf("UnitTermOfExpr(%s): err = %v, want it to name the scale", expr, err)
		}
	}
	for _, expr := range []string{"SI::'°C'", "K", "s * K"} {
		if _, err := m.UnitTermOfExpr(scope, parseExpr(t, expr)); err != nil {
			t.Errorf("UnitTermOfExpr(%s) = %v, want a unit", expr, err)
		}
	}
}

// TestMeasurementScaleOfUnitTerm: MeasurementScaleOf reports a scale that is a
// term's one factor; a unit, a scaled or a composed term is on no scale.
func TestMeasurementScaleOfUnitTerm(t *testing.T) {
	m, idx := quantityFixture(t)
	celsius := lookupOne(t, idx, "SI::°C_abs")
	point := semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: celsius, Exponent: 1}}}
	if scale, ok := m.MeasurementScaleOf(point); !ok || scale != celsius {
		t.Errorf("MeasurementScaleOf('°C_abs') = %v, %v; want the scale", scale, ok)
	}
	kelvin := lookupOne(t, idx, "SI::K")
	for name, term := range map[string]semantics.UnitTerm{
		"K":             {Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: kelvin, Exponent: 1}}},
		"'°C_abs'**2":   {Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: celsius, Exponent: 2}}},
		"'°C_abs'*K":    {Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: celsius, Exponent: 1}, {Unit: kelvin, Exponent: 1}}},
		"1000·'°C_abs'": {Scale: semantics.UnitScale(1000), Factors: []semantics.UnitFactor{{Unit: celsius, Exponent: 1}}},
		"one":           semantics.UnitOne().Term,
	} {
		if scale, ok := m.MeasurementScaleOf(term); ok {
			t.Errorf("MeasurementScaleOf(%s) = %v, want no scale", name, scale)
		}
	}
}

// TestPointsOnScalesDoNotFold: a point on a scale and every operator over one
// stay unfolded, its anchor being a value evaluation reads.
func TestPointsOnScalesDoNotFold(t *testing.T) {
	m, idx := quantityFixture(t)
	scope := idx.DocumentRoot("q.sysml")
	for _, expr := range []string{
		"26.85 [SI::'°C_abs'] + 10.0 [SI::'°C']",
		"10.0 [K] + 26.85 [SI::'°C_abs']",
		"26.85 [SI::'°C_abs'] - 10.0 [SI::'°C_abs']",
		"2 * 26.85 [SI::'°C_abs']",
		"26.85 [SI::'°C_abs'] * 2.0 [s]",
		"26.85 [SI::'°C_abs'] / 2",
		"26.85 [SI::'°C_abs'] ** 2",
		"-(26.85 [SI::'°C_abs'])",
		"5.0 [Time::UTC] + 3.0 [s]",
		"5.0 [Time::UTC] - 3.0 [Time::UTC]",
	} {
		if q, ok := m.EvalQuantity(scope, parseExpr(t, expr)); ok {
			t.Errorf("%s folded to %s, want no fold", expr, q.String())
		}
	}
	if q, ok := m.EvalQuantity(scope, parseExpr(t, "26.85 [SI::'°C_abs']")); ok {
		t.Errorf("a point itself folded to %s, want no fold", q.String())
	}
	if q := fold(t, m, idx, "300.0 [K] + 10.0 [SI::'°C']"); q.String() != "310.0 [K]" {
		t.Errorf("300.0 [K] + 10.0 [SI::'°C'] = %s, want 310.0 [K]", q.String())
	}
}

// TestPointDimensions: point ± magnitude keeps the scale, point − point is a
// magnitude, refused operations have no dimension, a typed feature is undecided.
func TestPointDimensions(t *testing.T) {
	m, idx := quantityFixture(t)
	scope := idx.DocumentRoot("q.sysml")
	celsius := lookupOne(t, idx, "SI::°C_abs")
	utc := lookupOne(t, idx, "Time::UTC")
	for _, tc := range []struct {
		expr  string
		dim   string
		scale *symbols.Symbol
	}{
		{"26.85 [SI::'°C_abs']", "Θ", celsius},
		{"+(26.85 [SI::'°C_abs'])", "Θ", celsius},
		{"26.85 [SI::'°C_abs'] + 10.0 [SI::'°C']", "Θ", celsius},
		{"26.85 [SI::'°C_abs'] - 10.0 [K]", "Θ", celsius},
		{"10.0 [K] + 26.85 [SI::'°C_abs']", "Θ", celsius},
		{"26.85 [SI::'°C_abs'] - 10.0 [SI::'°C_abs']", "Θ", nil},
		{"5.0 [Time::UTC]", "T", utc},
		{"5.0 [Time::UTC] + 3.0 [s]", "T", utc},
		{"5.0 [Time::UTC] - 3.0 [Time::UTC]", "T", nil},
		{"Q::FirstStage::mass - 1 [kg]", "M", nil},
	} {
		dim, ok := m.DimensionOfExpr(scope, parseExpr(t, tc.expr))
		if !ok {
			t.Errorf("DimensionOfExpr(%s): unknown, want %s", tc.expr, tc.dim)
			continue
		}
		if dim.String() != tc.dim || dim.Scale != tc.scale {
			t.Errorf("DimensionOfExpr(%s) = %s on %v, want %s on %v", tc.expr, dim, dim.Scale, tc.dim, tc.scale)
		}
	}
	for _, expr := range []string{
		"26.85 [SI::'°C_abs'] + 10.0 [SI::'°C_abs']",
		"300.0 [K] - 26.85 [SI::'°C_abs']",
		"2 * 26.85 [SI::'°C_abs']",
		"26.85 [SI::'°C_abs'] * 2.0 [s]",
		"2.0 [s] / 5.0 [Time::UTC]",
		"26.85 [SI::'°C_abs'] ** 2",
		"-(26.85 [SI::'°C_abs'])",
	} {
		if dim, ok := m.DimensionOfExpr(scope, parseExpr(t, expr)); ok {
			t.Errorf("DimensionOfExpr(%s) = %s, want unknown", expr, dim)
		}
	}
	feature, ok := m.DimensionOfExpr(scope, parseExpr(t, "Q::FirstStage::mass"))
	if !ok || !feature.MaybePoint || feature.IsPoint() || feature.IsMagnitude() {
		t.Errorf("a typed feature's dimension = %+v, %v; want one that may be a point", feature, ok)
	}
}
