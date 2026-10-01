package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestScaleStaysExact: a conversion factor is kept as a ratio, so converting at
// the boundary of a comparison answers exactly — 5.4 [km/h] is 1.5 [m/s], which
// evaluating 1000/3600 first would answer 1.4999999999999998 for.
func TestScaleStaysExact(t *testing.T) {
	kmPerHour := UnitScale(1000).DividedBy(UnitScale(3600))
	if got := ConvertMagnitude(5.4, kmPerHour, UnitScale(1)); got != 1.5 {
		t.Errorf("5.4 km/h = %v m/s, want exactly 1.5", got)
	}
	if got := ConvertMagnitude(1.5, UnitScale(1), kmPerHour); got != 5.4 {
		t.Errorf("1.5 m/s = %v km/h, want exactly 5.4", got)
	}
}

// TestScalePowInverts: a negative exponent inverts the ratio rather than
// evaluating it, which is what keeps a composed unit such as `km/h` exact.
func TestScalePowInverts(t *testing.T) {
	inv := UnitScale(3600).Pow(-1)
	if inv.Num != 1 || inv.Den != 3600 {
		t.Fatalf("3600^-1 = %v, want 1/3600", inv)
	}
	if got := UnitScale(1000).Times(inv); ConvertMagnitude(5.4, got, UnitScale(1)) != 1.5 {
		t.Errorf("1000·(1/3600) = %v, which does not convert 5.4 to 1.5", got)
	}
}

// TestScaleReduces: a whole ratio is normalized and a common divisor cancelled,
// so equal scale factors compare equal.
func TestScaleReduces(t *testing.T) {
	if got := UnitScale(1000).DividedBy(UnitScale(3600)); got != (Scale{Num: 5, Den: 18}) {
		t.Errorf("1000/3600 = %v, want 5/18", got)
	}
	if got := UnitScale(6).DividedBy(UnitScale(3)); got != UnitScale(2) {
		t.Errorf("6/3 = %v, want 2", got)
	}
	if got := UnitScale(1).DividedBy(UnitScale(-2)); got != (Scale{Num: -1, Den: 2}) {
		t.Errorf("1/-2 = %v, want -1/2", got)
	}
}

// TestUnitExprTextReadsBack: the text of a unit expression, reparsed, is the
// same expression — grouping the notation needs is kept, redundant grouping dropped.
func TestUnitExprTextReadsBack(t *testing.T) {
	tests := []struct{ src, want string }{
		{"m", "m"},
		{"SI::km/SI::h", "SI::km/SI::h"},
		{"m*s**2", "m*s**2"},
		{"(m)**2", "m**2"},
		{"(m*s)**2", "(m*s)**2"},
		{"(m/s)**2", "(m/s)**2"},
		{"m/(s*kg)", "m/(s*kg)"},
		{"m/(s/kg)", "m/(s/kg)"},
		{"m/s/kg", "m/s/kg"},
		{"(m/s)*kg", "m/s*kg"},
		{"m*(s*kg)", "m*s*kg"},
		{"m**-1", "m**-1"},
		{"(m**2)**3", "(m**2)**3"},
		{"m**2**3", "m**2**3"},
		{"(m**2)/s", "m**2/s"},
		{"m/s**2", "m/s**2"},
		{"'A/m'", "'A/m'"},
		{"SI::'A/m'*m", "SI::'A/m'*m"},
		{"('A/m')**2", "'A/m'**2"},
		{"m/'A/m'", "m/'A/m'"},
		{"'°'/rad", "'°'/rad"},
	}
	parse := func(src string) ast.Node {
		p := parser.New(source.New("<unit>", []byte(src)))
		expr := p.ParseExpression()
		if expr == nil || len(p.Diagnostics) > 0 {
			t.Fatalf("parse %q: %v", src, p.Diagnostics)
		}
		return expr
	}
	for _, tc := range tests {
		got := UnitExprText(parse(tc.src))
		if got != tc.want {
			t.Errorf("UnitExprText(%q) = %q, want %q", tc.src, got, tc.want)
		}
		if again := UnitExprText(parse(got)); again != got {
			t.Errorf("UnitExprText(%q) reparsed = %q, does not read back", got, again)
		}
	}
}

// TestNamedUnitProductSpellsOneName: a named unit stays one factor of a product —
// text that is no name is quoted, escaped where it holds what a quoted name cannot.
func TestNamedUnitProductSpellsOneName(t *testing.T) {
	tests := []struct{ name, want, timesM string }{
		{"m", "m", "m**2"},
		{"SI::m", "SI::m", "SI::m*m"},
		{"'A/m'", "'A/m'", "'A/m'*m"},
		{"SI::'A/m'", "SI::'A/m'", "SI::'A/m'*m"},
		{`'it\'s'`, `'it\'s'`, `'it\'s'*m`},
		{`SI::'a\\b'`, `SI::'a\\b'`, `SI::'a\\b'*m`},
		{"metres per second", "'metres per second'", "'metres per second'*m"},
		{"1000·metre", "'1000·metre'", "'1000·metre'*m"},
		{"in", "'in'", "'in'*m"},
		{"SI::in", "'SI::in'", "'SI::in'*m"},
		{"SI::'in'", "SI::'in'", "SI::'in'*m"},
		{"SI::km*km", "'SI::km*km'", "'SI::km*km'*m"},
		{"'SI::km*km'", "'SI::km*km'", "'SI::km*km'*m"},
		{"'A/m'*m", `'\'A/m\'*m'`, `'\'A/m\'*m'*m`},
		{"a'b*c", `'a\'b*c'`, `'a\'b*c'*m`},
		{`a\b`, `'a\\b'`, `'a\\b'*m`},
		{"metres\nper second", `'metres\nper second'`, `'metres\nper second'*m`},
		{"metres\r\nper second", `'metres\r\nper second'`, `'metres\r\nper second'*m`},
		{"'", `'\''`, `'\''*m`},
	}
	m := NamedUnitProduct(nil, "m", false)
	for _, tc := range tests {
		got := NamedUnitProduct(nil, tc.name, false)
		if got.String() != tc.want {
			t.Errorf("NamedUnitProduct(%q) = %q, want %q", tc.name, got, tc.want)
		}
		if product := got.Times(m).String(); product != tc.timesM {
			t.Errorf("NamedUnitProduct(%q) * m = %q, want %q", tc.name, product, tc.timesM)
		}
	}
}

// TestOpaqueUnitNamesReadBack: a product spelt from text the notation cannot read
// as a name parses back to that text as one factor, whatever characters it holds.
func TestOpaqueUnitNamesReadBack(t *testing.T) {
	m := NamedUnitProduct(nil, "m", false)
	for _, name := range []string{
		"metres per second", "it's", `a\b`, "a\nb", "a\r\nb", "'", `\`, "'A/m'*m", "a'b*c", `x\'y`, "tab\there",
		"in", "SI::in", "then", "in::m",
	} {
		spelt := NamedUnitProduct(nil, name, false).Times(m).Pow(2).String()
		p := parser.New(source.New("<unit>", []byte(spelt)))
		expr := p.ParseExpression()
		if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(spelt) {
			t.Errorf("%q spelt %q does not parse: %v", name, spelt, p.Diagnostics)
			continue
		}
		var names []string
		var model Model
		read, err := model.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
			names = append(names, source.StringValue(qn.Parts[0].Text))
			return nil, false
		})
		if err != nil {
			t.Errorf("%q spelt %q does not read as units: %v", name, spelt, err)
			continue
		}
		if !slices.Contains(names, name) || len(names) != 2 {
			t.Errorf("%q spelt %q names %q, want it and m", name, spelt, names)
		}
		if again := read.String(); again != spelt {
			t.Errorf("%q spelt %q reads back as %q", name, spelt, again)
		}
	}
}

// TestUnitTermAlgebra composes and compares terms over base units, which is what
// makes two quantities comparable.
func TestUnitTermAlgebra(t *testing.T) {
	metre := &symbols.Symbol{Name: "metre"}
	second := &symbols.Symbol{Name: "second"}
	m := UnitTerm{Scale: UnitScale(1), Factors: []UnitFactor{{Unit: metre, Exponent: 1}}}
	s := UnitTerm{Scale: UnitScale(1), Factors: []UnitFactor{{Unit: second, Exponent: 1}}}

	speed := m.DividedBy(s)
	if speed.Dimensionless() {
		t.Error("m/s has a dimension")
	}
	if speed.Commensurable(m) {
		t.Error("m/s and m do not measure the same thing")
	}
	if ratio := m.DividedBy(m); !ratio.Dimensionless() {
		t.Errorf("m/m = %v, want dimension one", ratio)
	}
	if area := m.Times(m); area.Factors[0].Exponent != 2 {
		t.Errorf("m·m = %v, want m^2", area)
	}
	if inv := speed.Pow(-1); !inv.Commensurable(s.DividedBy(m)) {
		t.Errorf("(m/s)^-1 = %v, want s/m", inv)
	}
}

// TestIsIdentityUnit: `MeasurementReferences::one` is the identity of unit
// products by its structure — a plain DimensionOneUnit at scale one, as is any
// unit declared so — while an angular unit, a unit scaling `one` by convention
// and a unit of a base dimension are not.
func TestIsIdentityUnit(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package test {
			private import ISQ::*;
			private import SI::*;
			private import MeasurementReferences::*;
			attribute unity : DimensionOneUnit = one;
			attribute another : DimensionOneUnit = new DimensionOneUnit();
			attribute <'%'> percent : DimensionOneUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = one; :>> conversionFactor = 0.01; } }
			attribute halfTurn : AngularMeasureUnit = rad;
			attribute halfMetre : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 0.5; } }
		}
	`)
	lib := func(fqn string) *symbols.Symbol {
		matches := m.resolver.Index().LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("%s: %d matches", fqn, len(matches))
		}
		return matches[0]
	}
	cases := []struct {
		sym  *symbols.Symbol
		want bool
	}{
		{lib("MeasurementReferences::one"), true},
		{nestedSym(t, root, "test::unity"), true},
		{nestedSym(t, root, "test::another"), true},
		{nestedSym(t, root, "test::percent"), false},
		{nestedSym(t, root, "test::halfTurn"), false},
		{nestedSym(t, root, "test::halfMetre"), false},
		{lib("SI::rad"), false},
		{lib("SI::sr"), false},
		{lib("SI::m"), false},
		{lib("SI::J"), false},
	}
	for _, tc := range cases {
		if got := m.IsIdentityUnit(tc.sym); got != tc.want {
			t.Errorf("IsIdentityUnit(%s) = %v, want %v", tc.sym.Name, got, tc.want)
		}
	}
	if m.IsIdentityUnit(nil) {
		t.Error("IsIdentityUnit(nil) = true")
	}
}

// TestUnitProductAbsorbsIdentity: the identity power leaves any product it shares
// with another unit, from either side of a product or quotient, and is the unit
// again once every other power cancels, however the operations are grouped; alone
// it stays, to the first power, whatever power it is raised to, with every root
// itself. Two identity units a model spells differently are one unit.
func TestUnitProductAbsorbsIdentity(t *testing.T) {
	one := NamedUnitProduct(nil, "one", true)
	one.Powers[0].Identity = true
	unity := NamedUnitProduct(nil, "unity", true)
	unity.Powers[0].Identity = true
	m := NamedUnitProduct(nil, "m", false)
	rad := NamedUnitProduct(nil, "rad", true)
	cases := []struct {
		got  UnitProduct
		want string
	}{
		{one.Times(m), "m"},
		{m.Times(one), "m"},
		{m.DividedBy(one), "m"},
		{one.DividedBy(m), "1/m"},
		{one.Times(one), "one"},
		{one.DividedBy(one), "one"},
		{one.Pow(3), "one"},
		{one.Pow(-2), "one"},
		{one.Pow(0), "one"},
		{one.Times(rad), "rad"},
		{rad.Times(one).Times(m), "m*rad"},
		{one.Times(m).Times(one).DividedBy(one), "m"},
		{one.Times(m).DividedBy(m), "one"},
		{one.Times(m.DividedBy(m)), "one"},
		{m.DividedBy(m), "1"},
		{m.Times(one).Pow(2).DividedBy(m.Pow(2)), "one"},
		{one.Times(rad).DividedBy(rad), "one"},
		{one.Times(m).DividedBy(one.Times(m)), "one"},
		{one.Times(unity), "one"},
		{unity.Times(one), "one"},
		{unity.Times(m).DividedBy(m), "unity"},
	}
	for _, tc := range cases {
		if s := tc.got.String(); s != tc.want {
			t.Errorf("got %s, want %s", s, tc.want)
		}
	}
	if !one.Equal(unity) || !unity.Times(m).Equal(one.Times(m)) {
		t.Error("two identity units are not one unit")
	}
	if !one.Times(m).Equal(m) || one.Times(m).ShortSpelling().Identity == nil {
		t.Error("the identity a product absorbed is not kept aside from its powers")
	}
	root, ok := one.Root(2)
	if !ok || root.String() != "one" {
		t.Errorf("one.Root(2) = %s, %v; want one, true", root, ok)
	}
	if _, ok := rad.Root(2); ok {
		t.Error("rad.Root(2) succeeded; rad has no square root")
	}
}
