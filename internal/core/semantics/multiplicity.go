package semantics

import (
	"fmt"
	"math"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Bound is one end of a multiplicity range. Known is false when the bound
// expression is not model-level-evaluable (checks then skip it). Infinite marks
// the `*` unbounded upper.
type Bound struct {
	Value    int64
	Infinite bool
	Known    bool
}

// Range is an extracted multiplicity [lower..upper]. For the single-bound form
// `[n]`, Lower and Upper are both n, except for `[*]`, whose lower bound is 0
// (KerML 1.0 §8.2.5.11, multiplicity textual notation).
type Range struct {
	Lower Bound
	Upper Bound
}

// boundIn evaluates a multiplicity bound expression, reading through features it
// names when scope is non-nil: `*` is infinite, an integer known, else unknown.
func (m *Model) boundIn(scope *symbols.Scope, n ast.Node) Bound {
	if n == nil {
		return Bound{}
	}
	if _, isInf := n.(*ast.LiteralInfinity); isInf {
		return Bound{Infinite: true, Known: true}
	}
	var v Value
	var ok bool
	if scope != nil {
		v, ok = m.EvalIn(scope, n)
	} else {
		v, ok = m.Eval(n)
	}
	if !ok {
		return Bound{}
	}
	switch v.Kind {
	case ValInt:
		return Bound{Value: v.Int, Known: true}
	case ValInfinity:
		return Bound{Infinite: true, Known: true}
	default:
		return Bound{}
	}
}

// multiplicityRange extracts a Range from a parsed *ast.Multiplicity. ok is
// false when the multiplicity is nil.
func (m *Model) multiplicityRange(mult *ast.Multiplicity) (Range, bool) {
	return m.multiplicityRangeIn(nil, mult)
}

// multiplicityRangeIn is multiplicityRange evaluating bounds that name valued
// features (`[one]`) in scope; a nil scope evaluates constants only.
func (m *Model) multiplicityRangeIn(scope *symbols.Scope, mult *ast.Multiplicity) (Range, bool) {
	if mult == nil {
		return Range{}, false
	}
	if mult.IsRange {
		return Range{Lower: m.boundIn(scope, mult.Lower), Upper: m.boundIn(scope, mult.Upper)}, true
	}
	// Single-bound `[n]`: the parser stores the sole bound in Lower. The bound is
	// both bounds, unless it is unbounded, where the lower bound is 0.
	b := m.boundIn(scope, mult.Lower)
	if b.Infinite {
		return Range{Lower: Bound{Value: 0, Known: true}, Upper: b}, true
	}
	return Range{Lower: b, Upper: b}, true
}

// RangeOf extracts the multiplicity range declared on a usage node, or ok=false
// when it declares none.
func (m *Model) RangeOf(mult *ast.Multiplicity) (Range, bool) {
	return m.multiplicityRange(mult)
}

// MultiplicityOf returns the extracted multiplicity range of a usage symbol, a
// subject or a requirement constraint included, or ok=false when the symbol is
// not a usage or declares none.
func (m *Model) MultiplicityOf(sym *symbols.Symbol) (Range, bool) {
	mult := UsageMultiplicityOf(sym)
	if mult == nil {
		return Range{}, false
	}
	return m.multiplicityRange(mult)
}

// UsageMultiplicityOf returns the multiplicity a usage, subject, cross feature or
// owned constraint symbol declares as its own, or nil; an end's `end [m]` is its cross feature's.
func UsageMultiplicityOf(sym *symbols.Symbol) *ast.Multiplicity {
	if sym == nil {
		return nil
	}
	if oc, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return oc.Multiplicity
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		return decl.Multiplicity
	case *ast.SubjectMember:
		return decl.Multiplicity
	case *ast.CrossFeatureMember:
		return decl.Multiplicity
	}
	return nil
}

// AssumedRange is the multiplicity of a feature that declares none: a feature
// holds exactly one value unless it says otherwise (KerML 1.0 §7.4.5). It is
// the one notion of implicit multiplicity every layer holds a feature to.
func AssumedRange() Range {
	return Range{
		Lower: Bound{Value: 1, Known: true},
		Upper: Bound{Value: 1, Known: true},
	}
}

// EffectiveMultiplicityOf returns the multiplicity governing a usage symbol: the
// one it declares, or the assumed 1..1 when it declares none.
func (m *Model) EffectiveMultiplicityOf(sym *symbols.Symbol) Range {
	if r, ok := m.MultiplicityOf(sym); ok {
		return r
	}
	return AssumedRange()
}

// AllowsNone reports whether the range admits no value at all: a known lower
// bound of 0, as `[0..1]` and `[*]` declare.
func (r Range) AllowsNone() bool {
	return r.Lower.Known && !r.Lower.Infinite && r.Lower.Value == 0
}

// IsOptionalParameter reports whether a parameter may be left without an
// argument: its declared multiplicity admits no value (KerML 1.0 §7.4.7.2, an
// input parameter with lower bound 0). One declaring none holds one value.
func (m *Model) IsOptionalParameter(usage *ast.Usage) bool {
	if usage == nil {
		return false
	}
	r, ok := m.multiplicityRange(usage.Multiplicity)
	return ok && r.AllowsNone()
}

// Intersect returns the range both ranges allow: the greater lower bound and the
// lesser upper bound, an unknown bound deferring to a known one.
func (r Range) Intersect(o Range) Range {
	return Range{Lower: greaterBound(r.Lower, o.Lower), Upper: lesserBound(r.Upper, o.Upper)}
}

func greaterBound(a, b Bound) Bound {
	switch {
	case !a.Known:
		return b
	case !b.Known, a.Infinite:
		return a
	case b.Infinite, b.Value > a.Value:
		return b
	}
	return a
}

func lesserBound(a, b Bound) Bound {
	switch {
	case !a.Known:
		return b
	case !b.Known, b.Infinite:
		return a
	case a.Infinite, b.Value < a.Value:
		return b
	}
	return a
}

// CountViolation returns why count values do not conform to the range, phrased
// for a diagnostic, or "" when they conform or a bound is not evaluable. It is
// the one wording for a count against a multiplicity, shared by the static
// check on a bound value and the runtime check on a materialized default.
func (r Range) CountViolation(count int64) string {
	if r.Upper.Known && !r.Upper.Infinite && count > r.Upper.Value {
		return fmt.Sprintf("%d value(s) bound to a feature with multiplicity upper bound %d", count, r.Upper.Value)
	}
	if r.Lower.Known && !r.Lower.Infinite && count < r.Lower.Value {
		return fmt.Sprintf("%d value(s) bound to a feature with multiplicity lower bound %d", count, r.Lower.Value)
	}
	return ""
}

// HeldViolation is CountViolation for a value whose count is only bounded: reported where even
// its fewest values exceed the upper bound, or its most fall short of the lower.
func (r Range) HeldViolation(held Range) string {
	if n, ok := held.Exactly(); ok {
		return r.CountViolation(n)
	}
	if r.Upper.Known && !r.Upper.Infinite && held.Lower.Known && (held.Lower.Infinite || held.Lower.Value > r.Upper.Value) {
		fewest := "at least " + held.Lower.Text()
		if held.Lower.Infinite {
			fewest = fmt.Sprintf("more than %d", int64(math.MaxInt64))
		}
		return fmt.Sprintf("%s value(s) bound to a feature with multiplicity upper bound %d", fewest, r.Upper.Value)
	}
	if r.Lower.Known && !r.Lower.Infinite && held.Upper.Known && !held.Upper.Infinite && held.Upper.Value < r.Lower.Value {
		return fmt.Sprintf("at most %d value(s) bound to a feature with multiplicity lower bound %d", held.Upper.Value, r.Lower.Value)
	}
	return ""
}

// Exactly is the one count the range admits, ok where both bounds are that finite count.
func (r Range) Exactly() (int64, bool) {
	if !r.Lower.Known || !r.Upper.Known || r.Lower.Infinite || r.Upper.Infinite || r.Lower.Value != r.Upper.Value {
		return 0, false
	}
	return r.Lower.Value, true
}

// HasBounds reports whether the range is exactly lower..upper — the spec's
// multiplicityHasBounds (SysML v2 8.3.3.1). ok is false when a bound is not
// evaluable, so callers can skip the check.
func (r Range) HasBounds(lower, upper int64) (holds bool, ok bool) {
	if !r.Lower.Known || !r.Upper.Known {
		return false, false
	}
	if r.Lower.Infinite || r.Upper.Infinite {
		return false, true
	}
	return r.Lower.Value == lower && r.Upper.Value == upper, true
}

// Text renders the range in multiplicity notation: `[1]`, `[0..1]`, `[1..*]`.
// A bound that is not evaluable renders as `?`.
func (r Range) Text() string {
	lower, upper := r.Lower.Text(), r.Upper.Text()
	if lower == upper && r.Lower.Known && !r.Lower.Infinite {
		return "[" + lower + "]"
	}
	return "[" + lower + ".." + upper + "]"
}

// Text renders one bound: a number, `*`, or `?` when it is not evaluable.
func (b Bound) Text() string {
	switch {
	case !b.Known:
		return "?"
	case b.Infinite:
		return "*"
	}
	return strconv.FormatInt(b.Value, 10)
}

// LowerLeUpper reports whether a range's lower bound does not exceed its upper
// bound. It returns ok=false when either bound is unknown (not evaluable), so
// callers can skip the check. An infinite upper always satisfies the ordering;
// an infinite lower is only valid when the upper is also infinite.
func (r Range) LowerLeUpper() (valid bool, ok bool) {
	if !r.Lower.Known || !r.Upper.Known {
		return false, false
	}
	if r.Upper.Infinite {
		return true, true
	}
	if r.Lower.Infinite {
		// finite upper with infinite lower is invalid.
		return false, true
	}
	return r.Lower.Value <= r.Upper.Value, true
}
