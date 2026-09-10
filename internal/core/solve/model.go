package solve

import (
	"bufio"
	"fmt"
	"math/big"
	"strings"
)

// ModelValue is one variable's value in a model, read back exactly from the
// solver's own text: a consumer decoding a witness reads it rather than parsing
// the rendered Assignment.
type ModelValue struct {
	// Kind is the sort of the value, the variable's own.
	Kind SortKind

	// Bool holds a SortBool value.
	Bool bool

	// Number holds a SortInt value, which is integral, or a SortReal value, exact.
	Number *big.Rat

	// Text holds a SortString value, or the name of a SortDatatype value: the
	// qualified name of the literal or variant, as the sort declares it.
	Text string
}

// DecodeValue reads an assignment back exactly. A value the variable's sort does
// not hold — a datatype value the sort does not declare, an algebraic number the
// solver writes as a root, a term rather than a literal — is an error.
func DecodeValue(a Assignment) (ModelValue, error) {
	if a.Var == nil {
		return ModelValue{}, fmt.Errorf("a value of no variable")
	}
	value, err := readSexpr(bufio.NewReader(strings.NewReader(a.Raw)))
	if err != nil {
		return ModelValue{}, fmt.Errorf("unreadable value %s", a.Raw)
	}
	kind := a.Var.Sort.Kind
	switch kind {
	case SortBool:
		if !value.IsList && (value.Atom == "true" || value.Atom == "false") {
			return ModelValue{Kind: kind, Bool: value.Atom == "true"}, nil
		}
	case SortString:
		if !value.IsList && value.Quoted {
			return ModelValue{Kind: kind, Text: value.Atom}, nil
		}
	case SortDatatype:
		if value.IsList {
			break
		}
		name := smtName(value.Atom)
		if !contains(a.Var.Sort.Values, name) {
			return ModelValue{}, fmt.Errorf("%s is not a value of %s", name, a.Var.Sort.Name)
		}
		return ModelValue{Kind: kind, Text: name}, nil
	case SortInt:
		rat, ok := ratOfSexpr(value)
		if !ok || !rat.IsInt() {
			return ModelValue{}, fmt.Errorf("no integer in %s", a.Raw)
		}
		return ModelValue{Kind: kind, Number: rat}, nil
	case SortReal:
		rat, ok := ratOfSexpr(value)
		if !ok {
			return ModelValue{}, fmt.Errorf("no rational in %s", a.Raw)
		}
		return ModelValue{Kind: kind, Number: rat}, nil
	}
	return ModelValue{}, fmt.Errorf("unreadable value %s", a.Raw)
}

// literal is the term denoting a decoded value, which is what denies a model in
// an enumeration. An integer outside int64 has no literal in the term language.
func (v ModelValue) literal(sort Sort) (*Term, error) {
	switch v.Kind {
	case SortBool:
		return BoolTerm(v.Bool), nil
	case SortString:
		return StringTerm(v.Text), nil
	case SortDatatype:
		return ValueTerm(sort, v.Text), nil
	case SortInt:
		if !v.Number.Num().IsInt64() {
			return nil, fmt.Errorf("%s is outside the Integer range", v.Number.Num().String())
		}
		return IntTerm(v.Number.Num().Int64()), nil
	case SortReal:
		return RealTerm(v.Number), nil
	}
	return nil, fmt.Errorf("a value of no sort")
}

// assign renders one variable's solver value in the notation's own terms, keeping
// the solver's S-expression for a value the notation cannot write.
func assign(v *Var, value sexpr) Assignment {
	raw := value.String()
	text, ok := renderValue(v, value)
	if !ok {
		return Assignment{Var: v, Value: raw, Raw: raw}
	}
	return Assignment{Var: v, Value: text, Raw: raw, Rendered: true}
}

// renderValue writes a value as the notation does: a quantity with its unit, an
// enumeration or variant by name, a number, a boolean or a string.
func renderValue(v *Var, value sexpr) (string, bool) {
	switch v.Sort.Kind {
	case SortBool:
		if !value.IsList && (value.Atom == "true" || value.Atom == "false") {
			return value.Atom, true
		}
	case SortString:
		if !value.IsList && value.Quoted {
			return `"` + value.Atom + `"`, true
		}
	case SortDatatype:
		if !value.IsList {
			name := smtName(value.Atom)
			for _, candidate := range v.Sort.Values {
				if candidate == name {
					return name, true
				}
			}
		}
	case SortInt:
		if rat, ok := ratOfSexpr(value); ok && rat.IsInt() {
			return rat.Num().String(), true
		}
	case SortReal:
		if rat, ok := ratOfSexpr(value); ok {
			return withUnit(renderRat(rat), v.Unit, v.Dimension), true
		}
	}
	return "", false
}

// withUnit writes a magnitude in the base units it is expressed in, as a quantity
// is written; a dimension whose base units are unnamed is named as the dimension
// it is, since the magnitude is not in the unit any literal was written in.
func withUnit(magnitude, unit, dimension string) string {
	switch {
	case unit != "":
		return magnitude + " [" + unit + "]"
	case dimension != "":
		return magnitude + " (in the base units of " + dimension + ")"
	}
	return magnitude
}

// renderRat writes an exact rational as the notation writes a number: a decimal
// when it has a terminating one, else the quotient it is.
func renderRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String() + ".0"
	}
	if digits, ok := decimalDigits(r.Denom()); ok {
		return r.FloatString(digits)
	}
	return r.Num().String() + "/" + r.Denom().String()
}

// ratOfSexpr reads a numeral, a decimal, or the negation, quotient or widening a
// solver writes a numeric model value with.
func ratOfSexpr(value sexpr) (*big.Rat, bool) {
	if !value.IsList {
		if value.Quoted {
			return nil, false
		}
		rat, ok := new(big.Rat).SetString(value.Atom)
		if !ok || strings.ContainsAny(value.Atom, "eE/") {
			// SetString accepts exponents and quotients, which SMT-LIB numerals
			// and decimals are not.
			return nil, false
		}
		return rat, true
	}
	switch {
	case len(value.List) == 2 && value.List[0].Atom == "-":
		inner, ok := ratOfSexpr(value.List[1])
		if !ok {
			return nil, false
		}
		return inner.Neg(inner), true
	case len(value.List) == 2 && value.List[0].Atom == "to_real":
		return ratOfSexpr(value.List[1])
	case len(value.List) == 3 && value.List[0].Atom == "/":
		num, ok := ratOfSexpr(value.List[1])
		if !ok {
			return nil, false
		}
		den, okDen := ratOfSexpr(value.List[2])
		if !okDen || den.Sign() == 0 {
			return nil, false
		}
		return num.Quo(num, den), true
	}
	return nil, false
}
