package semantics

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var (
	// ErrNotAUnit reports a feature used as a measurement reference that is not
	// one: it does not conform to MeasurementReferences::MeasurementUnit.
	ErrNotAUnit = errors.New("not a measurement unit")

	// ErrUnitExpr reports an expression in unit position that is not a product
	// of powers of measurement units.
	ErrUnitExpr = errors.New("not a measurement unit expression")

	// ErrUnitConversion reports a unit whose conversion to its reference unit is
	// declared but cannot be read as a number, so no factor can be derived.
	ErrUnitConversion = errors.New("undetermined unit conversion")

	// ErrUnitCycle reports a unit that is defined, directly or through other
	// units, in terms of itself.
	ErrUnitCycle = errors.New("cyclic unit definition")
)

// Names of the library elements the unit model is defined by (SysML v2
// Quantities and Units domain library).
const (
	fqnMeasurementUnit    = "MeasurementReferences::MeasurementUnit"
	fqnDerivedUnit        = "MeasurementReferences::DerivedUnit"
	fqnDimensionOneUnit   = "MeasurementReferences::DimensionOneUnit"
	fqnAngularMeasureUnit = "ISQSpaceTime::AngularMeasureUnit"

	memberUnitConversion   = "unitConversion"
	memberReferenceUnit    = "referenceUnit"
	memberConversionFactor = "conversionFactor"
	memberPrefix           = "prefix"
	memberUnitPowerFactors = "unitPowerFactors"
)

// Scale is a unit's scale factor as a ratio kept unevaluated, so a conversion
// whose factor is exact stays exact: `5.4 [km/h]` is `5.4·1000/3600 = 1.5 [m/s]`,
// where evaluating 1000/3600 first would answer 1.4999999999999998 and make a
// requirement's `<=` at its boundary come out wrong.
type Scale struct {
	Num float64
	Den float64
}

// UnitScale is the scale factor n, as a whole ratio.
func UnitScale(n float64) Scale { return Scale{Num: n, Den: 1} }

// IsZero reports whether the ratio is zero or undefined, which no unit's scale
// factor is.
func (s Scale) IsZero() bool { return s.Num == 0 || s.Den == 0 }

// Times returns the product of two ratios.
func (s Scale) Times(other Scale) Scale {
	return reduceScale(Scale{Num: s.Num * other.Num, Den: s.Den * other.Den})
}

// DividedBy returns the quotient of two ratios.
func (s Scale) DividedBy(other Scale) Scale {
	return reduceScale(Scale{Num: s.Num * other.Den, Den: s.Den * other.Num})
}

// Pow raises the ratio to an exponent. A negative exponent inverts the ratio
// rather than raising it to a negative power, which would evaluate it: `h^-1`
// stays 1/3600 instead of becoming 0.0002777777777777778.
func (s Scale) Pow(exp float64) Scale {
	if exp < 0 {
		return reduceScale(Scale{Num: math.Pow(s.Den, -exp), Den: math.Pow(s.Num, -exp)})
	}
	return reduceScale(Scale{Num: math.Pow(s.Num, exp), Den: math.Pow(s.Den, exp)})
}

// String renders the ratio, as a ratio when it is not whole.
func (s Scale) String() string {
	if s.Den == 1 {
		return fmt.Sprintf("%g", s.Num)
	}
	return fmt.Sprintf("%g/%g", s.Num, s.Den)
}

// reduceScale cancels a whole common divisor, keeping composed factors small,
// and normalizes a whole ratio to denominator one.
func reduceScale(s Scale) Scale {
	if s.Den == 1 || s.Num == 0 || s.Den == 0 {
		return s
	}
	if isWhole(s.Num) && isWhole(s.Den) {
		if g := gcd(math.Abs(s.Num), math.Abs(s.Den)); g > 1 {
			s.Num, s.Den = s.Num/g, s.Den/g
		}
	}
	if s.Den < 0 {
		s.Num, s.Den = -s.Num, -s.Den
	}
	return s
}

func isWhole(f float64) bool { return f == math.Trunc(f) && !math.IsInf(f, 0) }

func gcd(a, b float64) float64 {
	for b != 0 {
		a, b = b, math.Mod(a, b)
	}
	return a
}

// ConvertMagnitude expresses a magnitude given over the scale factor from over
// the scale factor to, as one ratio so that no intermediate rounding enters.
func ConvertMagnitude(magnitude float64, from, to Scale) float64 {
	return magnitude * (from.Num * to.Den) / (from.Den * to.Num)
}

// UnitFactor is one base unit raised to an exponent.
type UnitFactor struct {
	Unit     *symbols.Symbol
	Exponent float64
}

// UnitTerm is a measurement unit reduced to a scale factor over base units: the
// product of Scale and each base unit raised to its exponent. `km/h` reduces to
// Scale 1000/3600 over `SI::m` and `SI::s^-1`, which is what makes two units
// comparable — they are commensurable when their factors agree, and a magnitude
// converts between them by the ratio of their scales.
//
// Factors are ordered by base-unit qualified name and carry no zero exponents,
// so two terms over the same base units compare element-wise.
type UnitTerm struct {
	Scale   Scale
	Factors []UnitFactor
}

// Dimensionless reports whether the term has no base units, as a count or a
// ratio of like quantities has.
func (t UnitTerm) Dimensionless() bool { return len(t.Factors) == 0 }

// Commensurable reports whether both terms are expressed over the same base
// units with the same exponents, so a magnitude in one converts to the other.
func (t UnitTerm) Commensurable(other UnitTerm) bool {
	if len(t.Factors) != len(other.Factors) {
		return false
	}
	for i, f := range t.Factors {
		g := other.Factors[i]
		if f.Unit != g.Unit || f.Exponent != g.Exponent {
			return false
		}
	}
	return true
}

// Same reports whether two terms are one reduction: commensurable at equal scale.
func (t UnitTerm) Same(other UnitTerm) bool {
	return t.Commensurable(other) && t.Scale.Num*other.Scale.Den == other.Scale.Num*t.Scale.Den
}

// DimensionKey identifies the base units the term is over, exponents included
// but scale excluded: two terms share a key exactly when they are commensurable.
func (t UnitTerm) DimensionKey() string {
	out := ""
	for _, f := range t.Factors {
		out += fmt.Sprintf("%s^%g;", f.Unit.Name, f.Exponent)
	}
	return out
}

// String renders the term over its base units ("1000·m·s⁻¹"), for a diagnostic
// that has to say what a unit reduced to.
func (t UnitTerm) String() string {
	out := ""
	if t.Scale != UnitScale(1) {
		out = t.Scale.String()
	}
	for _, f := range t.Factors {
		if out != "" {
			out += "·"
		}
		out += f.Unit.Name
		if f.Exponent != 1 {
			out += fmt.Sprintf("^%g", f.Exponent)
		}
	}
	if out == "" {
		return "1"
	}
	return out
}

// Clone returns a term sharing no storage with t.
func (t UnitTerm) Clone() UnitTerm {
	return UnitTerm{Scale: t.Scale, Factors: slices.Clone(t.Factors)}
}

// Times returns the product of two terms.
func (t UnitTerm) Times(other UnitTerm) UnitTerm {
	return combine(t, other, 1)
}

// DividedBy returns the quotient of two terms.
func (t UnitTerm) DividedBy(other UnitTerm) UnitTerm {
	return combine(t, other, -1)
}

// Pow raises the term to an exponent, scale included.
func (t UnitTerm) Pow(exp float64) UnitTerm {
	out := UnitTerm{Scale: t.Scale.Pow(exp)}
	for _, f := range t.Factors {
		out.Factors = append(out.Factors, UnitFactor{Unit: f.Unit, Exponent: f.Exponent * exp})
	}
	return normalizeTerm(out)
}

// BaseProduct is the term's base units as a product named by their symbols
// (`m`, `s`); a magnitude over it is the term's magnitude times the term's scale.
func (t UnitTerm) BaseProduct() UnitProduct {
	out := UnitProduct{}
	for _, f := range t.Factors {
		name := f.Unit.ShortName
		if name == "" {
			name = f.Unit.Name
		}
		out.Powers = append(out.Powers, UnitPower{Unit: f.Unit, Name: lexer.NameText(name), Exponent: f.Exponent})
	}
	return normalizeProduct(out)
}

// Normalized restores the term's invariant: repeated base units summed, those
// that cancel dropped, the rest ordered by name. For a term built from outside.
func (t UnitTerm) Normalized() UnitTerm { return normalizeTerm(t) }

// combine multiplies two terms, with the exponents of the second one signed by
// sign, so that division shares multiplication's accumulation.
func combine(a, b UnitTerm, sign float64) UnitTerm {
	scale := a.Scale.Times(b.Scale)
	if sign < 0 {
		scale = a.Scale.DividedBy(b.Scale)
	}
	out := UnitTerm{Scale: scale}
	out.Factors = append(out.Factors, a.Factors...)
	for _, f := range b.Factors {
		out.Factors = append(out.Factors, UnitFactor{Unit: f.Unit, Exponent: f.Exponent * sign})
	}
	return normalizeTerm(out)
}

// normalizeTerm sums the exponents of repeated base units, drops those that
// cancel, and orders the rest by qualified name.
func normalizeTerm(t UnitTerm) UnitTerm {
	exponents := make(map[*symbols.Symbol]float64, len(t.Factors))
	order := make([]*symbols.Symbol, 0, len(t.Factors))
	for _, f := range t.Factors {
		if _, seen := exponents[f.Unit]; !seen {
			order = append(order, f.Unit)
		}
		exponents[f.Unit] += f.Exponent
	}
	out := UnitTerm{Scale: t.Scale}
	for _, unit := range order {
		if exponents[unit] == 0 {
			continue
		}
		out.Factors = append(out.Factors, UnitFactor{Unit: unit, Exponent: exponents[unit]})
	}
	sort.SliceStable(out.Factors, func(i, j int) bool {
		return out.Factors[i].Unit.Name < out.Factors[j].Unit.Name
	})
	return out
}

// IsMeasurementUnit reports whether sym is a feature typed by a measurement
// unit, which is what may stand in the unit position of a quantity expression.
func (m *Model) IsMeasurementUnit(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	unitDef := m.libSymbol(fqnMeasurementUnit)
	if unitDef == nil {
		// Without the Quantities and Units library there is no unit model to
		// check against, so a recorded reduction is the only evidence.
		return sym.Facts != nil && sym.Facts.Unit != nil
	}
	return m.Conforms(sym, unitDef)
}

// IsMeasurementScale reports whether sym is a feature typed by a measurement
// scale (`Time::UTC`, `SI::'°C_abs'`): a scalar reference that is not a unit.
func (m *Model) IsMeasurementScale(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	scale := m.libSymbol(fqnMeasurementScale)
	return scale != nil && m.Conforms(sym, scale)
}

// MeasurementUnitOf is the measurement unit sym names: sym itself, or the unit
// an alias such as SI::'m/s²' stands for; false for anything else.
func (m *Model) MeasurementUnitOf(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	if m.resolver != nil {
		if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = target
		}
	}
	if !m.IsMeasurementUnit(sym) {
		return nil, false
	}
	return sym, true
}

// IsAngularMeasureUnit reports whether sym is a unit of plane angle (rad, °, ′):
// one of the dimension-one units a trigonometric function may take.
func (m *Model) IsAngularMeasureUnit(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	angleDef := m.libSymbol(fqnAngularMeasureUnit)
	return angleDef != nil && m.Conforms(sym, angleDef)
}

// UnitTermOf reduces a measurement unit to base units. A unit declared with a
// conversion to a reference unit contributes that conversion's factor; a unit
// declared as an expression of other units, or by the power factors it lists,
// reduces through them; a unit of dimension one reduces to no base unit at all;
// and a unit that is declared in terms of nothing else is itself a base unit.
//
// The reduction is memoized per symbol, and read from the facts installed for a
// library symbol rather than derived again.
func (m *Model) UnitTermOf(sym *symbols.Symbol) (UnitTerm, error) {
	if m == nil || sym == nil {
		return UnitTerm{}, ErrNotAUnit
	}
	if cached, ok := m.unitTerms[sym]; ok {
		return cached, nil
	}
	if m.reducingUnit[sym] {
		return UnitTerm{}, fmt.Errorf("%w: %s", ErrUnitCycle, sym.Name)
	}
	if !m.IsMeasurementUnit(sym) {
		return UnitTerm{}, fmt.Errorf("%w: %s", ErrNotAUnit, sym.Name)
	}

	m.reducingUnit[sym] = true
	term, err := m.reduceUnit(sym)
	delete(m.reducingUnit, sym)
	if err != nil {
		return UnitTerm{}, err
	}
	m.unitTerms[sym] = term
	return term, nil
}

// reduceUnit computes the reduction UnitTermOf memoizes.
func (m *Model) reduceUnit(sym *symbols.Symbol) (UnitTerm, error) {
	if sym.Facts != nil && sym.Facts.Unit != nil {
		return m.recordedUnitTerm(sym.Facts.Unit, sym.Name)
	}
	if term, declared, err := m.convertedUnitTerm(sym); declared || err != nil {
		return term, err
	}
	if dimOne := m.libSymbol(fqnDimensionOneUnit); dimOne != nil && m.Conforms(sym, dimOne) {
		return UnitTerm{Scale: UnitScale(1)}, nil
	}
	if term, declared, err := m.powerFactorsUnitTerm(sym); declared || err != nil {
		return term, err
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
		return m.UnitTermOfExpr(sym.OwnerScope, usage.Value)
	}
	return UnitTerm{Scale: UnitScale(1), Factors: []UnitFactor{{Unit: sym, Exponent: 1}}}, nil
}

// powerFactorsUnitTerm reduces a derived unit through the `unitPowerFactors` it
// lists, each unit raised to its exponent; declared is false for a unit listing none.
func (m *Model) powerFactorsUnitTerm(sym *symbols.Symbol) (UnitTerm, bool, error) {
	factors, declared := m.unitPowerFactorSymbols(sym)
	if !declared {
		return UnitTerm{}, false, nil
	}
	term := UnitTerm{Scale: UnitScale(1)}
	for _, factor := range factors {
		if factor == nil {
			return UnitTerm{}, true, fmt.Errorf("%w: %s lists an unresolvable unit power factor", ErrUnitExpr, sym.Name)
		}
		reduced, err := m.unitPowerFactor(sym, factor)
		if err != nil {
			return UnitTerm{}, true, err
		}
		term = term.Times(reduced)
	}
	return term, true, nil
}

// unitPowerFactorSymbols resolves the `unitPowerFactors` a derived unit lists, in
// order, nil for one resolving to nothing; declared is false for a unit listing none.
func (m *Model) unitPowerFactorSymbols(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	factors, ok := m.LookupMember(sym, memberUnitPowerFactors)
	if !ok {
		return nil, false
	}
	listed := usageValue(factors)
	if listed == nil {
		return nil, false
	}
	var out []*symbols.Symbol
	for _, ref := range sequenceElements(listed) {
		var factor *symbols.Symbol
		if m.resolver != nil {
			if resolved, ok := m.resolver.ResolveTarget(scopeOf(factors), ref); ok {
				factor = resolved
			}
		}
		out = append(out, factor)
	}
	return out, true
}

// unitPowerFactor reduces one listed UnitPowerFactor: its `unit` raised to its
// `exponent`. unit names the derived unit listing it, for errors.
func (m *Model) unitPowerFactor(unit, factor *symbols.Symbol) (UnitTerm, error) {
	named, ok := m.LookupMember(factor, memberUnit)
	if !ok || usageValue(named) == nil {
		return UnitTerm{}, fmt.Errorf("%w: %s lists a unit power factor naming no unit", ErrUnitExpr, unit.Name)
	}
	term, err := m.UnitTermOfExpr(scopeOf(named), usageValue(named))
	if err != nil {
		return UnitTerm{}, err
	}
	exponent, ok := m.LookupMember(factor, memberExponent)
	if !ok || usageValue(exponent) == nil {
		return UnitTerm{}, fmt.Errorf("%w: %s lists a unit power factor stating no exponent", ErrUnitExpr, unit.Name)
	}
	value, ok := m.Eval(usageValue(exponent))
	if !ok || !value.IsNumeric() {
		return UnitTerm{}, fmt.Errorf("%w: %s lists a unit power factor with a non-numeric exponent", ErrUnitExpr, unit.Name)
	}
	return term.Pow(value.AsReal()), nil
}

// recordedUnitTerm rebuilds a reduction recorded for a library symbol, resolving
// each base unit by qualified name. name identifies the unit in errors.
func (m *Model) recordedUnitTerm(facts *symbols.UnitFacts, name string) (UnitTerm, error) {
	if facts.Irreducible {
		return UnitTerm{}, fmt.Errorf("%w: %s reduces to no base unit", ErrUnitConversion, name)
	}
	scale := Scale{Num: facts.ScaleNum, Den: facts.ScaleDen}
	if scale.IsZero() {
		return UnitTerm{}, fmt.Errorf("%w: %s reduces to a zero scale factor", ErrUnitConversion, name)
	}
	term := UnitTerm{Scale: scale}
	for _, f := range facts.Factors {
		base := m.libSymbol(f.FQN)
		if base == nil {
			return UnitTerm{}, fmt.Errorf("%w: %s reduces to unknown base unit %s", ErrNotAUnit, name, f.FQN)
		}
		term.Factors = append(term.Factors, UnitFactor{Unit: base, Exponent: f.Exponent})
	}
	return normalizeTerm(term), nil
}

// convertedUnitTerm reduces a unit that binds a conversion to a reference unit:
// the reference unit's reduction scaled by the conversion factor. It reports
// declared=false for a unit that binds none — every measurement unit inherits
// the optional `unitConversion` feature, so only one that names a reference
// unit converts to anything.
func (m *Model) convertedUnitTerm(sym *symbols.Symbol) (UnitTerm, bool, error) {
	conv, ok := m.LookupMember(sym, memberUnitConversion)
	if !ok {
		return UnitTerm{}, false, nil
	}
	refUnit, ok := m.LookupMember(conv, memberReferenceUnit)
	if !ok {
		return UnitTerm{}, false, nil
	}
	refExpr := usageValue(refUnit)
	if refExpr == nil {
		return UnitTerm{}, false, nil
	}
	refTerm, err := m.UnitTermOfExpr(scopeOf(refUnit), refExpr)
	if err != nil {
		return UnitTerm{}, true, err
	}
	factor, err := m.conversionFactor(sym, conv)
	if err != nil {
		return UnitTerm{}, true, err
	}
	return refTerm.Times(UnitTerm{Scale: factor}), true, nil
}

// conversionFactor reads the factor of a unit conversion: the factor it states,
// or the one its prefix stands for (ConversionByPrefix derives the factor from
// the prefix rather than stating it).
func (m *Model) conversionFactor(sym, conv *symbols.Symbol) (Scale, error) {
	if factor, ok := m.numericMember(conv, memberConversionFactor); ok {
		return factor, nil
	}
	if prefix, ok := m.LookupMember(conv, memberPrefix); ok {
		if named := m.referencedUnitSymbol(prefix); named != nil {
			if factor, ok := m.numericMember(named, memberConversionFactor); ok {
				return factor, nil
			}
		}
	}
	return Scale{}, fmt.Errorf("%w: %s states no conversion factor", ErrUnitConversion, sym.Name)
}

// numericMember reads the declared value of sym's named member as a scale
// factor. A quotient of numbers — a conversion factor is often written as one —
// is kept as a ratio rather than evaluated.
func (m *Model) numericMember(sym *symbols.Symbol, name string) (Scale, bool) {
	member, ok := m.LookupMember(sym, name)
	if !ok {
		return Scale{}, false
	}
	expr := usageValue(member)
	if expr == nil {
		return Scale{}, false
	}
	if op, ok := expr.(*ast.OperatorExpr); ok && op.Operator == ast.OpDiv && len(op.Operands) == 2 {
		num, numOK := m.Eval(op.Operands[0])
		den, denOK := m.Eval(op.Operands[1])
		if numOK && denOK && num.IsNumeric() && den.IsNumeric() && den.AsReal() != 0 {
			return reduceScale(Scale{Num: num.AsReal(), Den: den.AsReal()}), true
		}
	}
	val, ok := m.Eval(expr)
	if !ok || !val.IsNumeric() {
		return Scale{}, false
	}
	return UnitScale(val.AsReal()), true
}

// referencedUnitSymbol resolves the feature a member's value names, for a value
// that names another declaration (a prefix, a reference unit) rather than
// computing one.
func (m *Model) referencedUnitSymbol(sym *symbols.Symbol) *symbols.Symbol {
	expr := usageValue(sym)
	if expr == nil {
		return nil
	}
	qn := qualifiedNameOf(expr)
	if qn == nil || m.resolver == nil {
		return nil
	}
	target, ok := m.resolver.ResolveQualified(scopeOf(sym), qn)
	if !ok {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(target); aliasOK {
		target = resolved
	} else {
		return nil
	}
	return target
}

// UnitTermOfExpr reduces an expression in unit position — a measurement unit,
// or a product, quotient or power of them — resolving names in scope.
func (m *Model) UnitTermOfExpr(scope *symbols.Scope, node ast.Node) (UnitTerm, error) {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return m.unitTermOfName(scope, n.Name)
	case *ast.QualifiedName:
		return m.unitTermOfName(scope, n)
	case *ast.OperatorExpr:
		return m.unitTermOfOperator(scope, n)
	case *ast.LiteralInteger:
		// `1` is the unit of dimension one written as a number, as `m/m` is.
		if val, ok := m.Eval(n); ok && val.Kind == ValInt && val.Int == 1 {
			return UnitTerm{Scale: UnitScale(1)}, nil
		}
	}
	return UnitTerm{}, fmt.Errorf("%w: %T", ErrUnitExpr, node)
}

// unitTermOfName reduces the unit a qualified name refers to. The name is an
// ordinary feature reference: it resolves to the nearest declaration (KerML
// 8.2.3.5.4), and one that is not a measurement unit is a typing error rather
// than a reason to keep searching (KerML 8.2.3.5.1).
func (m *Model) unitTermOfName(scope *symbols.Scope, qn *ast.QualifiedName) (UnitTerm, error) {
	if qn == nil || m.resolver == nil {
		return UnitTerm{}, ErrUnitExpr
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return UnitTerm{}, fmt.Errorf("%w: unresolved unit %s", ErrNotAUnit, QualifiedNameText(qn))
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	if !m.IsMeasurementUnit(sym) {
		return UnitTerm{}, m.shadowedUnit(qn, sym)
	}
	return m.UnitTermOf(sym)
}

// ShadowedUnitError reports a name in unit position that resolved to a
// declaration which is not a measurement unit, naming the unit it hid.
type ShadowedUnitError struct {
	Name     string          // the name as written in unit position
	Resolved *symbols.Symbol // the declaration the name resolved to
	Shadowed *symbols.Symbol // the measurement unit that declaration hid, or nil
	// ShadowedName is the qualified name of Shadowed, which is how a message names
	// a unit the model did not write.
	ShadowedName string
	Namespace    string // qualified name of the namespace Resolved was declared in
	Suggestion   string // qualified spelling that names the shadowed unit
}

func (e *ShadowedUnitError) Error() string {
	where := ""
	if e.Namespace != "" {
		where = " declared in " + e.Namespace
	}
	msg := fmt.Sprintf("%s: %s resolves to the %s %s%s", ErrNotAUnit, e.Name,
		e.Resolved.Kind, e.Resolved.Name, where)
	if e.Shadowed == nil {
		return msg
	}
	if e.Suggestion == "" {
		return fmt.Sprintf("%s, shadowing the measurement unit %s", msg, e.ShadowedName)
	}
	return fmt.Sprintf("%s, shadowing the measurement unit %s — write %s to name the unit",
		msg, e.ShadowedName, e.Suggestion)
}

// Unwrap reports the error as a not-a-unit error, which is the condition a
// caller tests for.
func (e *ShadowedUnitError) Unwrap() error { return ErrNotAUnit }

// shadowedUnit describes a unit-position name that resolved to a non-unit,
// including the unit that resolution hid. Only a simple name can shadow one, so
// a qualified name is reported without that explanation.
func (m *Model) shadowedUnit(qn *ast.QualifiedName, sym *symbols.Symbol) error {
	err := &ShadowedUnitError{Name: QualifiedNameText(qn), Resolved: sym, Namespace: namespacePath(sym.OwnerScope)}
	if len(qn.Parts) != 1 {
		return err
	}
	if outer := m.unitOutside(sym); outer != nil {
		err.Shadowed, err.ShadowedName = outer, m.fqnOf(outer)
		// The written name qualified by the unit's namespace, which is the
		// spelling that reaches the unit from inside the shadowing namespace. A
		// unit owned by no namespace has no such spelling to offer.
		if qualified := qualifyAs(m.fqnOf(outer), err.Name); qualified != err.Name {
			err.Suggestion = qualified
		}
	}
	return err
}

// unitOutside resolves sym's own name where sym does not declare it, reporting
// it only when it is a measurement unit — the unit sym hid from its siblings.
func (m *Model) unitOutside(sym *symbols.Symbol) *symbols.Symbol {
	if m.resolver == nil || sym.OwnerScope == nil {
		return nil
	}
	found, ok := m.resolver.LookupNameExcluding(sym.OwnerScope, sym.Name, sym.Decl)
	if !ok || found == nil || found == sym {
		return nil
	}
	if alias, ok := m.resolver.ResolveAliasTarget(found); ok {
		found = alias
	}
	if !m.IsMeasurementUnit(found) {
		return nil
	}
	return found
}

// fqnOf returns the qualified name the index knows a symbol by, or its own name.
func (m *Model) fqnOf(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if m.resolver == nil || m.resolver.Index() == nil {
		return sym.Name
	}
	if fqn := m.resolver.Index().GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}

// qualifyAs re-spells a qualified name with its last segment replaced by name,
// so a unit found as SI::metre is suggested as the SI::m the model wrote.
func qualifyAs(fqn, name string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i+2] + name
	}
	return name
}

// namespacePath renders the qualified name of the namespace a scope belongs to,
// so a message can say where a declaration lives.
func namespacePath(scope *symbols.Scope) string {
	var parts []string
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil || owner.Name == "" {
			continue
		}
		parts = append([]string{owner.Name}, parts...)
	}
	return strings.Join(parts, "::")
}

// unitTermOfOperator reduces a product, quotient or power of units.
func (m *Model) unitTermOfOperator(scope *symbols.Scope, n *ast.OperatorExpr) (UnitTerm, error) {
	switch n.Operator {
	case ast.OpMul, ast.OpDiv:
		if len(n.Operands) != 2 {
			return UnitTerm{}, fmt.Errorf("%w: %v takes 2 operands, got %d", ErrUnitExpr, n.Operator, len(n.Operands))
		}
		left, err := m.UnitTermOfExpr(scope, n.Operands[0])
		if err != nil {
			return UnitTerm{}, err
		}
		right, err := m.UnitTermOfExpr(scope, n.Operands[1])
		if err != nil {
			return UnitTerm{}, err
		}
		if n.Operator == ast.OpMul {
			return left.Times(right), nil
		}
		return left.DividedBy(right), nil
	case ast.OpPow:
		if len(n.Operands) != 2 {
			return UnitTerm{}, fmt.Errorf("%w: %v takes 2 operands, got %d", ErrUnitExpr, n.Operator, len(n.Operands))
		}
		base, err := m.UnitTermOfExpr(scope, n.Operands[0])
		if err != nil {
			return UnitTerm{}, err
		}
		exp, ok := m.Eval(n.Operands[1])
		if !ok || !exp.IsNumeric() {
			return UnitTerm{}, fmt.Errorf("%w: unit exponent is not a constant number", ErrUnitExpr)
		}
		return base.Pow(exp.AsReal()), nil
	default:
		return UnitTerm{}, fmt.Errorf("%w: operator %v", ErrUnitExpr, n.Operator)
	}
}

// UnitExprText renders an expression in unit position as written, so a
// diagnostic, a printed value or an exported type fact names the unit the model
// used rather than its reduction. Grouping and quoting the text needs to read
// back are kept: `'A/m'` is one unit, `A/m` a quotient of two.
func UnitExprText(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return UnitNameText(n.Name)
	case *ast.QualifiedName:
		return UnitNameText(n)
	case *ast.OperatorExpr:
		switch {
		case len(n.Operands) == 2:
			left, right := UnitExprText(n.Operands[0]), UnitExprText(n.Operands[1])
			if unitOperandGrouped(n.Operator, n.Operands[0], false) {
				left = "(" + left + ")"
			}
			if unitOperandGrouped(n.Operator, n.Operands[1], true) {
				right = "(" + right + ")"
			}
			return left + n.Operator.String() + right
		case len(n.Operands) == 1:
			return n.Operator.String() + UnitExprText(n.Operands[0])
		}
	case *ast.LiteralInteger:
		return n.Value
	case *ast.LiteralReal:
		return n.Value
	}
	return ""
}

// unitOperandGrouped reports whether operand, under op, must be parenthesised
// to read back as itself: a product or quotient under `**` or right of `/`, and
// a power left of `**`, which associates to the right.
func unitOperandGrouped(op ast.OperatorKind, operand ast.Node, right bool) bool {
	inner, ok := operand.(*ast.OperatorExpr)
	if !ok || len(inner.Operands) != 2 {
		return false
	}
	switch inner.Operator {
	case ast.OpMul, ast.OpDiv:
		return op == ast.OpPow || (op == ast.OpDiv && right)
	case ast.OpPow:
		return op == ast.OpPow && !right
	}
	return false
}

// libSymbol resolves a library element by qualified name: the one bundled
// library declaration, else the one declaration there is, else nothing.
func (m *Model) libSymbol(fqn string) *symbols.Symbol {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	if cached, ok := m.libSymbols[fqn]; ok {
		return cached
	}
	idx := m.resolver.Index()
	matches := idx.LookupQualified(fqn)
	var found *symbols.Symbol
	for _, sym := range matches {
		if !idx.Library(sym) {
			continue
		}
		if found != nil {
			found = nil
			break
		}
		found = sym
	}
	if found == nil && len(matches) == 1 {
		found = matches[0]
	}
	m.libSymbols[fqn] = found
	return found
}

// usageValue returns the value expression a usage declares, or nil.
func usageValue(sym *symbols.Symbol) ast.Node {
	if sym == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	return usage.Value
}

// scopeOf returns the scope a symbol's declaration was written in, which is
// where the names in its value expression resolve.
func scopeOf(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	return sym.OwnerScope
}

// qualifiedNameOf returns the qualified name an expression is, unwrapping a
// feature reference, or nil for an expression that names nothing.
func qualifiedNameOf(node ast.Node) *ast.QualifiedName {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return n.Name
	case *ast.QualifiedName:
		return n
	default:
		return nil
	}
}

// UnitNameText renders a unit's qualified name as the notation spells it, a
// segment that is not a basic name in quotes: `SI::'A/m'`.
func UnitNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		parts[i] = lexer.NameText(part.Text)
	}
	return strings.Join(parts, "::")
}

// QualifiedNameText renders a qualified name as "A::B::C".
func QualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	out := ""
	for i, part := range qn.Parts {
		if i > 0 {
			out += "::"
		}
		out += part.Text
	}
	return out
}
