package semantics

import (
	"fmt"
	"math"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

// RangeIn is RangeOf evaluating bounds that name valued features in scope; a nil
// scope evaluates constants only.
func (m *Model) RangeIn(scope *symbols.Scope, mult *ast.Multiplicity) (Range, bool) {
	return m.multiplicityRangeIn(scope, mult)
}

// MultiplicityOf returns the extracted multiplicity range of a usage symbol, a
// subject or a requirement constraint included, or ok=false when the symbol is
// not a usage or declares none.
func (m *Model) MultiplicityOf(sym *symbols.Symbol) (Range, bool) {
	if sym.Recorded() {
		if sym.Kind == symbols.SymbolMultiplicity || sym.Facts.Node == symbols.NodeDefinition {
			return Range{}, false
		}
		return recordedRange(sym.Facts.Multiplicity)
	}
	mult := UsageMultiplicityOf(sym)
	if mult == nil {
		return Range{}, false
	}
	return m.multiplicityRange(mult)
}

// recordedRange is the range a multiplicity fact states, ok=false for none.
func recordedRange(facts *symbols.MultiplicityFacts) (Range, bool) {
	if facts == nil {
		return Range{}, false
	}
	bound := func(b symbols.BoundFacts) Bound {
		if b.Named {
			return Bound{}
		}
		return Bound{Value: b.Value, Known: b.Known, Infinite: b.Infinite}
	}
	return Range{Lower: bound(facts.Lower), Upper: bound(facts.Upper)}, true
}

// recordedRangeIn is recordedRange as multiplicityRangeIn would read the
// declaration in its scope: a bound written as a feature name has its value.
func recordedRangeIn(facts *symbols.MultiplicityFacts) (Range, bool) {
	if facts == nil {
		return Range{}, false
	}
	bound := func(b symbols.BoundFacts) Bound {
		return Bound{Value: b.Value, Known: b.Known, Infinite: b.Infinite}
	}
	return Range{Lower: bound(facts.Lower), Upper: bound(facts.Upper)}, true
}

// MultiplicityFactsOf states the multiplicity sym declares — as a usage or a
// definition does, or as a `multiplicity` member's range — as a record fact,
// nil when it declares none. A bound only the declaring scope evaluates is
// marked Named.
func (m *Model) MultiplicityFactsOf(sym *symbols.Symbol) *symbols.MultiplicityFacts {
	mult := UsageMultiplicityOf(sym)
	scope := declScope(sym)
	switch decl := sym.Decl.(type) {
	case *ast.MultiplicityDecl:
		mult, scope = decl.Range, sym.OwnerScope
	case *ast.Definition:
		mult = decl.Multiplicity
	}
	if mult == nil {
		return nil
	}
	plain, _ := m.multiplicityRangeIn(nil, mult)
	scoped, _ := m.multiplicityRangeIn(scope, mult)
	bound := func(plain, scoped Bound) symbols.BoundFacts {
		if plain.Known || !scoped.Known {
			return symbols.BoundFacts{Value: plain.Value, Known: plain.Known, Infinite: plain.Infinite}
		}
		return symbols.BoundFacts{Value: scoped.Value, Known: true, Infinite: scoped.Infinite, Named: true}
	}
	return &symbols.MultiplicityFacts{
		Lower: bound(plain.Lower, scoped.Lower),
		Upper: bound(plain.Upper, scoped.Upper),
	}
}

// SubsettedMultiplicity is the multiplicity a `multiplicity` member subsets
// (`multiplicity m subsets n;`), resolved; nil for any other declaration.
func (m *Model) SubsettedMultiplicity(sym *symbols.Symbol) *symbols.Symbol {
	decl, ok := sym.Decl.(*ast.MultiplicityDecl)
	if !ok || decl.Subsets == nil {
		return nil
	}
	w := &multiplicityWalk{m: m}
	return w.resolve(sym.OwnerScope, decl.Subsets)
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

// UnboundedRange is the multiplicity a parameter takes when nothing it
// redefines or subsets bounds it: [0..*] (SysML v2 §7.6.3).
func UnboundedRange() Range {
	return Range{
		Lower: Bound{Value: 0, Known: true},
		Upper: Bound{Infinite: true, Known: true},
	}
}

// ImplicitMultiplicityApplies reports whether a usage takes the implicit
// [1..1] of SysML v2 §7.6.3: written `attribute`, `item`, `part` or `port`,
// owned by a type, and subsetting or redefining no feature a type owns. The
// keyword, not the kind: KerML's `feature` parses to an attribute usage and
// takes no default multiplicity.
func (m *Model) ImplicitMultiplicityApplies(sym *symbols.Symbol) bool {
	if sym == nil || !featureOwnedByType(sym) {
		return false
	}
	switch sym.Keyword() {
	case "attribute", "item", "part", "port":
	default:
		return false
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		if target := m.RelationshipTarget(sym, rel); target != nil && featureOwnedByType(target) {
			return false
		}
	}
	return true
}

// featureOwnedByType reports whether a feature is owned by a definition or
// usage rather than by a package or namespace.
func featureOwnedByType(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	switch owner.Kind {
	case symbols.SymbolPackage, symbols.SymbolNamespace:
		return false
	}
	return true
}

// EffectiveParameterRange is the multiplicity a parameter is held to
// (SysML v2 §7.6.3): the nearest its redefinition chain or a feature it
// subsets declares, the implicit [1..1] where it qualifies, else [0..*].
func (m *Model) EffectiveParameterRange(sym *symbols.Symbol) Range {
	var subsetted []*symbols.Symbol
	for _, p := range m.ParameterRedefinitionChain(sym) {
		if r, ok := m.MultiplicityOf(p); ok {
			return r
		}
		if m.ImplicitMultiplicityApplies(p) {
			return AssumedRange()
		}
		for _, rel := range RelationshipsOf(p) {
			if rel != nil && rel.Kind == ast.RelSubsets && rel.Target != nil {
				if target := m.RelationshipTarget(p, rel); target != nil {
					subsetted = append(subsetted, target)
				}
			}
		}
	}
	visited := make(map[*symbols.Symbol]bool)
	for len(subsetted) > 0 {
		p := subsetted[0]
		subsetted = subsetted[1:]
		if visited[p] {
			continue
		}
		visited[p] = true
		if r, ok := m.MultiplicityOf(p); ok {
			return r
		}
		if m.ImplicitMultiplicityApplies(p) {
			return AssumedRange()
		}
		for _, rel := range RelationshipsOf(p) {
			if rel == nil || rel.Target == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
				continue
			}
			if target := m.RelationshipTarget(p, rel); target != nil {
				subsetted = append(subsetted, target)
			}
		}
	}
	return UnboundedRange()
}

// EffectiveMultiplicityOf returns the multiplicity governing a usage symbol: the
// one it declares, or the assumed 1..1 when it declares none.
func (m *Model) EffectiveMultiplicityOf(sym *symbols.Symbol) Range {
	if r, ok := m.MultiplicityOf(sym); ok {
		return r
	}
	return AssumedRange()
}

// AtMostOne reports whether the range is known to admit no more than one value,
// as `[1]` and `[0..1]` do; an unknown or unbounded upper bound is not.
func (r Range) AtMostOne() bool {
	return r.Upper.Known && !r.Upper.Infinite && r.Upper.Value <= 1
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

// AdmitsMore reports whether the range certainly admits more than count values.
func (r Range) AdmitsMore(count int64) bool {
	return r.Upper.Known && (r.Upper.Infinite || count < r.Upper.Value)
}

// MayAdmitMore reports whether the range does not certainly cap the values at count:
// it admits more, or its upper bound is not evaluable.
func (r Range) MayAdmitMore(count int64) bool {
	return !r.Upper.Known || r.AdmitsMore(count)
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
