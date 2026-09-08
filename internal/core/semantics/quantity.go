package semantics

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ErrIncommensurableUnits reports an operation over quantities whose units
// measure different things, or whose conversion the library does not derive.
// It is never answered by comparing magnitudes.
var ErrIncommensurableUnits = errors.New("incommensurable units")

// Quantity is a scalar quantity value: a magnitude and the measurement
// reference it is expressed in (Quantities::ScalarQuantityValue is exactly a
// number `num` and a reference `mRef`). The unit travels with the value, so
// `1.5 [m/s]` is never mistaken for `1.5 [km/h]`.
type Quantity struct {
	Num  Value
	Unit Unit
}

// Unit is a measurement reference as a quantity carries it: its text as written or
// canonically composed, its product of named units, and its reduction to base units.
type Unit struct {
	Text    string
	Product UnitProduct
	Term    UnitTerm
}

// String renders the unit by its text, falling back to its reduction for a
// unit that has none.
func (u Unit) String() string {
	if u.Text != "" {
		return u.Text
	}
	return u.Term.String()
}

// None reports the unit a bare number is read with, which names nothing.
func (u Unit) None() bool { return u.Text == "" && u.Product.IsEmpty() }

// Clone returns a unit sharing no storage with u.
func (u Unit) Clone() Unit {
	return Unit{Text: u.Text, Product: u.Product.Clone(), Term: u.Term.Clone()}
}

// Clone returns a quantity sharing no storage with q.
func (q Quantity) Clone() Quantity {
	return Quantity{Num: q.Num, Unit: q.Unit.Clone()}
}

// UnitOne is the unit a bare number is read in: no named unit, scale one.
func UnitOne() Unit {
	return Unit{Term: UnitTerm{Scale: UnitScale(1)}}
}

// String renders the quantity as a magnitude in its unit: `1.5 [m/s]`.
func (q *Quantity) String() string {
	return q.TextWithMagnitude(FormatConst(q.Num))
}

// TextWithMagnitude renders the quantity from an already-rendered magnitude, so
// a caller with its own convention for numbers keeps it and still names the
// unit the same way. The stored magnitude is untouched.
func (q *Quantity) TextWithMagnitude(magnitude string) string {
	return fmt.Sprintf("%s [%s]", magnitude, q.Unit)
}

// BaseMagnitude is the quantity's magnitude expressed over the base units its
// unit reduces to, which is the form two commensurable quantities compare in.
func (q *Quantity) BaseMagnitude() float64 {
	return ConvertMagnitude(q.Num.AsReal(), q.Unit.Term.Scale, UnitScale(1))
}

// ConvertTo expresses the quantity in unit, which requires the two units to be
// commensurable: their reductions must be over the same base units, or the
// magnitudes measure different things and no factor relates them.
func (q *Quantity) ConvertTo(unit Unit) (float64, error) {
	if !q.Unit.Term.Commensurable(unit.Term) {
		return 0, fmt.Errorf("%w: cannot express %s (%s) in %s (%s)", ErrIncommensurableUnits,
			q.Unit, q.Unit.Term, unit, unit.Term)
	}
	if unit.Term.Scale.IsZero() {
		return 0, fmt.Errorf("%w: %s reduces to a zero scale factor", ErrIncommensurableUnits, unit)
	}
	return ConvertMagnitude(q.Num.AsReal(), q.Unit.Term.Scale, unit.Term.Scale), nil
}

// FormatReal renders a Real as the shortest decimal that reads back as the same
// float64, so no surface rounds a value away. A whole value keeps a ".0" so it
// is not mistaken for an Integer.
func FormatReal(f float64) string {
	// An ordinary magnitude reads in full rather than in exponent notation, which
	// 'g' would switch to well before a Real stops being readable as digits.
	format := byte('f')
	if abs := math.Abs(f); f != 0 && (abs < 1e-4 || abs >= 1e21) {
		format = 'g'
	}
	text := strconv.FormatFloat(f, format, -1, 64)
	if !strings.ContainsAny(text, ".eEnN") {
		text += ".0"
	}
	return text
}

// FormatConst renders a scalar constant using the user-facing numeric
// convention shared by the runtime and the query surfaces.
func FormatConst(c Value) string {
	switch c.Kind {
	case ValInt:
		return strconv.FormatInt(c.Int, 10)
	case ValReal:
		return FormatReal(c.Real)
	case ValBool:
		return strconv.FormatBool(c.Bool)
	case ValInfinity:
		return "*"
	default:
		return "<unknown const>"
	}
}
