package fmi

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Dimension is one array dimension of an FMI 3.0 variable: fixed at start, or
// structural when it names the valueReference of a structural parameter.
type Dimension struct {
	Start          uint64 // meaningful only when HasStart
	HasStart       bool
	ValueReference uint32 // meaningful only when HasVR
	HasVR          bool
}

// UnitExponents resolves the variable's unit to coherent SI base-unit
// exponents: the declared unit's BaseUnit, else the unit spelling itself as a
// product of the base symbols. False for a variable with no unit, one that
// resolves to nothing, or a non-coherent unit (factor other than 1 or an
// offset, as km or degC carry).
func (v Variable) UnitExponents(d *Description) (BaseUnit, bool) {
	if v.Unit == "" {
		return BaseUnit{}, false
	}
	if u, ok := d.Unit(v.Unit); ok {
		if u.Base == nil || u.Base.Factor != 1 || u.Base.Offset != 0 {
			return BaseUnit{}, false
		}
		return *u.Base, true
	}
	return ParseBaseUnit(v.Unit)
}

// baseIndex orders the base units kg m s A K mol cd rad, the order SIExpression
// spells them in.
var baseIndex = map[string]int{"kg": 0, "m": 1, "s": 2, "A": 3, "K": 4, "mol": 5, "cd": 6, "rad": 7}

// ParseBaseUnit reads a unit spelled as a product of the SI base symbols alone:
// `kg`, `m`, `s`, `A`, `K`, `mol`, `cd` and `rad` joined by `*`, `.` or `/`,
// each optionally raised by `^n`, `^-n`, a trailing digit or a ²/³ superscript
// — `m/s2`, `m/s^2`, `m/s²` and `kg.m/s2` all read. Named units such as `N`
// are not parsed.
func ParseBaseUnit(unit string) (BaseUnit, bool) {
	s := strings.TrimSpace(unit)
	var b BaseUnit
	b.Factor, b.Offset = 1, 0
	if s == "" {
		return BaseUnit{}, false
	}
	exponents := &[8]int{}
	divide := false
	for i := 0; i < len(s); {
		name, next, ok := baseSymbol(s, i)
		if !ok {
			return BaseUnit{}, false
		}
		exponent, next, ok := baseExponent(s, next)
		if !ok {
			return BaseUnit{}, false
		}
		i = next
		if divide {
			exponent = -exponent
		}
		exponents[baseIndex[name]] += exponent
		if i == len(s) {
			break
		}
		switch s[i] {
		case '*', '.':
			i++
		case '/':
			divide, i = true, i+1
		default:
			return BaseUnit{}, false
		}
		if i == len(s) {
			return BaseUnit{}, false
		}
	}
	b.Kg, b.M, b.S, b.A, b.K, b.Mol, b.Cd, b.Rad = exponents[0], exponents[1], exponents[2], exponents[3], exponents[4], exponents[5], exponents[6], exponents[7]
	return b, true
}

// baseSymbol reads the SI base symbol at i, returning it and the index past it.
func baseSymbol(s string, i int) (name string, next int, ok bool) {
	switch {
	case strings.HasPrefix(s[i:], "mol"):
		return "mol", i + 3, true
	case strings.HasPrefix(s[i:], "rad"):
		return "rad", i + 3, true
	case strings.HasPrefix(s[i:], "kg"):
		return "kg", i + 2, true
	case strings.HasPrefix(s[i:], "cd"):
		return "cd", i + 2, true
	case s[i] == 'm', s[i] == 's', s[i] == 'A', s[i] == 'K':
		return string(s[i]), i + 1, true
	}
	return "", i, false
}

// baseExponent reads the exponent following a base symbol at i: `^n`, `^-n`,
// a trailing digit run or ²/³ superscripts, 1 when none. The exponent is
// signed by its own minus: `s^-2` in a denominator reads as s raised to +2,
// exactly as m/(s^-2) means.
func baseExponent(s string, i int) (exponent, next int, ok bool) {
	exponent = 1
	if i < len(s) && s[i] == '^' {
		i++
		neg := i < len(s) && s[i] == '-'
		if neg {
			i++
		}
		n, ok := digits(s, &i)
		if !ok {
			return 0, i, false
		}
		if neg {
			n = -n
		}
		return n, i, true
	}
	if i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n, ok := digits(s, &i)
		if !ok {
			return 0, i, false
		}
		exponent = n
	}
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '²':
			exponent *= 2
		case '³':
			exponent *= 3
		default:
			return exponent, i, true
		}
		i += size
	}
	return exponent, i, true
}

// digits reads a run of ASCII digits at i, advancing i past them.
func digits(s string, i *int) (int, bool) {
	start := *i
	for *i < len(s) && s[*i] >= '0' && s[*i] <= '9' {
		*i++
	}
	if *i == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[start:*i])
	return n, err == nil
}

// SIExpression spells the coherent KerML unit expression the exponents read
// as: the positive powers first, then `/` the negatives, in the order kg m s A
// K mol cd rad — `m/s^2` for acceleration, `kg*m/s^2` for force and `1/s` for
// frequency. A dimensionless unit spells as "" and reads as no unit at all.
func (b BaseUnit) SIExpression() string {
	exponents := [8]int{b.Kg, b.M, b.S, b.A, b.K, b.Mol, b.Cd, b.Rad}
	names := [8]string{"kg", "m", "s", "A", "K", "mol", "cd", "rad"}
	var positive, negative []string
	for i, e := range exponents {
		switch {
		case e > 0:
			positive = append(positive, power(names[i], e))
		case e < 0:
			negative = append(negative, power(names[i], -e))
		}
	}
	numerator := strings.Join(positive, "*")
	if numerator == "" {
		numerator = "1"
	}
	if len(negative) == 0 {
		if numerator == "1" && len(positive) == 0 {
			return ""
		}
		return numerator
	}
	return numerator + "/" + strings.Join(negative, "/")
}

// power spells one symbol raised to a positive exponent.
func power(name string, exponent int) string {
	if exponent == 1 {
		return name
	}
	return fmt.Sprintf("%s^%d", name, exponent)
}
