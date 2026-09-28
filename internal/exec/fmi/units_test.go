package fmi

import "testing"

// TestParseBaseUnit reads unit spellings as base-unit products: every
// separator and exponent form, a denominator, and the spellings that are not
// products of base symbols.
func TestParseBaseUnit(t *testing.T) {
	for _, tc := range []struct {
		unit string
		want BaseUnit
		ok   bool
	}{
		{"m", BaseUnit{M: 1, Factor: 1}, true},
		{"m/s2", BaseUnit{M: 1, S: -2, Factor: 1}, true},
		{"m/s^2", BaseUnit{M: 1, S: -2, Factor: 1}, true},
		{"m/s^-2", BaseUnit{M: 1, S: 2, Factor: 1}, true},
		{"m/s²", BaseUnit{M: 1, S: -2, Factor: 1}, true},
		{"m³", BaseUnit{M: 3, Factor: 1}, true},
		{"kg.m/s2", BaseUnit{Kg: 1, M: 1, S: -2, Factor: 1}, true},
		{"kg*m/s^2", BaseUnit{Kg: 1, M: 1, S: -2, Factor: 1}, true},
		{"s*A", BaseUnit{S: 1, A: 1, Factor: 1}, true},
		{"mol*cd", BaseUnit{Mol: 1, Cd: 1, Factor: 1}, true},
		{"rad", BaseUnit{Rad: 1, Factor: 1}, true},
		{"m*K/mol", BaseUnit{M: 1, K: 1, Mol: -1, Factor: 1}, true},
		{"N", BaseUnit{}, false},
		{"degC", BaseUnit{}, false},
		{"", BaseUnit{}, false},
		{"m/", BaseUnit{}, false},
		{"x", BaseUnit{}, false},
		{"m^", BaseUnit{}, false},
		{"m^2.5", BaseUnit{}, false},
	} {
		got, ok := ParseBaseUnit(tc.unit)
		if ok != tc.ok {
			t.Errorf("ParseBaseUnit(%q) ok = %v, want %v", tc.unit, ok, tc.ok)
			continue
		}
		if ok && *got.exponents() != *tc.want.exponents() {
			t.Errorf("ParseBaseUnit(%q) = %+v, want %+v", tc.unit, got, tc.want)
		}
	}
}

// exponents is the base-unit exponent vector a BaseUnit holds.
func (b BaseUnit) exponents() *[8]int {
	return &[8]int{b.Kg, b.M, b.S, b.A, b.K, b.Mol, b.Cd, b.Rad}
}

// TestSIExpression spells exponents back as unit expressions, deterministic in
// order, positives before the slash.
func TestSIExpression(t *testing.T) {
	for _, tc := range []struct {
		base BaseUnit
		want string
	}{
		{BaseUnit{}, ""},
		{BaseUnit{M: 1}, "m"},
		{BaseUnit{M: 1, S: -1}, "m/s"},
		{BaseUnit{M: 1, S: -2}, "m/s^2"},
		{BaseUnit{S: -1}, "1/s"},
		{BaseUnit{Kg: 1, M: 1, S: -2}, "kg*m/s^2"},
		{BaseUnit{Kg: 1, M: -1, S: -2}, "kg/m/s^2"},
		{BaseUnit{S: 1, A: 1}, "s*A"},
		{BaseUnit{S: 4, A: 2, Kg: -1, M: -2}, "s^4*A^2/kg/m^2"},
		{BaseUnit{Rad: 1, S: -1}, "rad/s"},
		{BaseUnit{Kg: 1, M: -3}, "kg/m^3"},
		{BaseUnit{M: 1, S: -1, Mol: -1}, "m/s/mol"},
	} {
		if got := tc.base.SIExpression(); got != tc.want {
			t.Errorf("%+v.SIExpression() = %q, want %q", tc.base, got, tc.want)
		}
	}
}

// TestUnitExponents resolves a variable's unit: a declared BaseUnit first, the
// declared type's unit through v.Unit, the unit spelling itself when no unit
// is declared, false for a coherent-breaking factor or offset.
func TestUnitExponents(t *testing.T) {
	d := &Description{Units: []Unit{
		{Name: "m", Base: &BaseUnit{M: 1, Factor: 1}},
		{Name: "km", Base: &BaseUnit{M: 1, Factor: 1000}},
		{Name: "degC", Base: &BaseUnit{K: 1, Factor: 1, Offset: 273.15}},
		{Name: "rpm"},
	}}
	for _, tc := range []struct {
		name string
		v    Variable
		want BaseUnit
		ok   bool
	}{
		{"declared unit", Variable{Unit: "m"}, BaseUnit{M: 1}, true},
		{"non-coherent factor", Variable{Unit: "km"}, BaseUnit{}, false},
		{"non-coherent offset", Variable{Unit: "degC"}, BaseUnit{}, false},
		{"display unit parses the spelling", Variable{Unit: "m/s"}, BaseUnit{M: 1, S: -1}, true},
		{"unitless FMI 1.0 name parses", Variable{Unit: "kg.m/s2"}, BaseUnit{Kg: 1, M: 1, S: -2}, true},
		{"no unit", Variable{}, BaseUnit{}, false},
		{"unknown named unit", Variable{Unit: "N"}, BaseUnit{}, false},
	} {
		got, ok := tc.v.UnitExponents(d)
		if ok != tc.ok {
			t.Errorf("%s: UnitExponents ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && *got.exponents() != *tc.want.exponents() {
			t.Errorf("%s: UnitExponents = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
