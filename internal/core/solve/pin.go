package solve

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrNotPinnable is returned for a value that cannot be fixed in a query: one
// the term language has no literal for, or one whose type or dimension does not
// match the variable it would fix. A pin is never dropped silently, since a
// query missing one would answer about values the model does not hold.
var ErrNotPinnable = errors.New("value cannot be fixed for solving")

// PinSource says where a fixed value came from, so a report distinguishes what
// an object holds from what the model declares and from what a caller chose.
type PinSource int

const (
	// PinHeld is a value an object holds.
	PinHeld PinSource = iota
	// PinDeclared is a value the model declares for a feature.
	PinDeclared
	// PinChosen is a value the caller supplied, such as a variant selection.
	PinChosen
)

// String names the source for a report a machine reads.
func (s PinSource) String() string {
	switch s {
	case PinHeld:
		return "held"
	case PinDeclared:
		return "declared"
	default:
		return "chosen"
	}
}

// phrase says who fixed the value, as a line about it reads.
func (s PinSource) phrase() string {
	switch s {
	case PinHeld:
		return "held by the object"
	case PinDeclared:
		return "declared by the model"
	default:
		return "chosen for this query"
	}
}

// Pin fixes one feature to a value a query asserts, rather than leaving the
// feature free for the solver to choose.
type Pin struct {
	// Feature is the feature declaration whose value is fixed.
	Feature *symbols.Symbol

	// Name is the feature as the element naming it writes it.
	Name string

	// Value is the value it is fixed to, read where the evaluator reads it.
	Value runtime.Value

	// Source says where the value came from.
	Source PinSource

	// Object is the object holding the value, 0 for a value no object holds.
	Object int64
}

// PinnedValue is a pin the query asserts: the variable it fixed, the value as
// the notation writes it, and where that assertion sits, so an unsat core naming
// the assertion names the pin.
type PinnedValue struct {
	// Var is the variable fixed.
	Var *Var

	// Value renders the fixed value as the notation writes it.
	Value string

	// Source says where the value came from.
	Source PinSource

	// Object is the object holding the value, 0 for a value no object holds.
	Object int64

	// Index is the assertion's position in Query.Assertions.
	Index int
}

// Unread is a fixed value no variable of the query reads. It is reported rather
// than dropped: the query answers about the conditions, which say nothing about
// this feature.
type Unread struct {
	// Pin is the value that was fixed.
	Pin Pin

	// Reason says why the query does not read it.
	Reason string
}

// Unfixed is a feature whose value could not be read, so the query leaves it
// free. Reading a declared value is an evaluation, and one that fails says
// nothing about the feature.
type Unfixed struct {
	// Feature is the feature declaration whose value was not read.
	Feature *symbols.Symbol

	// Name is the feature as the element naming it writes it.
	Name string

	// Reason says why no value was read.
	Reason string
}

// PinError says which value could not be fixed, why, and where the feature was
// declared. It unwraps to ErrNotPinnable.
type PinError struct {
	// Feature names the feature whose value was to be fixed.
	Feature string

	// Value renders the value as the notation writes it.
	Value string

	// Reason says why it cannot be fixed.
	Reason string

	// Source says where the value came from.
	Source PinSource

	// File and Span are where the feature was declared.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`, empty when unknown.
	Location string
}

// Error reports the refusal, naming the feature, the value and where it was
// declared.
func (e *PinError) Error() string {
	msg := fmt.Sprintf("%s = %s (%s): %s", e.Feature, e.Value, e.Source.phrase(), ErrNotPinnable)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Location != "" {
		msg += " at " + e.Location
	}
	return msg
}

// Unwrap returns ErrNotPinnable, so a caller tests the kind of failure rather
// than its text.
func (e *PinError) Unwrap() error { return ErrNotPinnable }

// Fixed reads the values the model already fixes for the features of sym: what
// inst holds where an object was created, else what the declaration states. The
// values are read through the evaluator, which is what a verdict about them
// reads, rather than through a second reader of the same declarations.
//
// The second result names the features whose value could not be read, which stay
// free rather than being reported as fixed to something.
func Fixed(ctx *runtime.Context, sym *symbols.Symbol, inst *runtime.Instance) ([]Pin, []Unfixed) {
	if ctx == nil || sym == nil {
		return nil, nil
	}
	var pins []Pin
	var unfixed []Unfixed
	for _, feat := range ctx.FeaturesOf(sym) {
		feature := feat
		if feature.Name == "" || feature.Symbol == nil {
			continue
		}
		// A feature typed by a definition holds an object, which no variable of a
		// query stands for; a variation holds the variant selected for it.
		if ctx.CompositeTypeOf(&feature) != nil && !ctx.IsVariationFeature(&feature) {
			continue
		}
		value, source, object, err := fixedValue(ctx, &feature, inst)
		switch {
		case err != nil:
			unfixed = append(unfixed, Unfixed{Feature: feature.Symbol, Name: feature.Name, Reason: err.Error()})
		case value.Kind == runtime.ValInvalid, ctx.HoldsNoValue(value):
			// No value is fixed, which is what leaves the feature free.
		default:
			pins = append(pins, Pin{
				Feature: feature.Symbol,
				Name:    feature.Name,
				Value:   value,
				Source:  source,
				Object:  object,
			})
		}
	}
	return pins, unfixed
}

// Fixing is what a query reads fixed values from: the element it is about, the
// element whose declarations its conditions were written among, and the object a
// verdict about it would be about, named by the definition this resolution
// declares — an object carried over a submission is bound to symbols of another
// scope tree, which are not the ones the query is translated from.
type Fixing struct {
	Element    *symbols.Symbol
	Owner      *symbols.Symbol
	Object     *runtime.Instance
	ObjectType *symbols.Symbol
}

// FixedFor gathers the values fixed for a query about the fixing's element: what
// its object holds for the features of its own definition, else what the owner —
// the element the conditions were written in — declares, and then what the
// element's own features declare. A feature fixed twice is fixed by the nearer
// source, which is what an evaluation about it would read.
func FixedFor(ctx *runtime.Context, f Fixing) ([]Pin, []Unfixed) {
	var pins []Pin
	var unfixed []Unfixed
	fixed := make(map[string]bool)
	inst := f.Object
	for _, from := range fixedFrom(ctx, f) {
		found, notRead := Fixed(ctx, from, inst)
		for _, p := range found {
			if !fixed[p.Name] {
				fixed[p.Name] = true
				pins = append(pins, p)
			}
		}
		for _, u := range notRead {
			if !fixed[u.Name] {
				fixed[u.Name] = true
				unfixed = append(unfixed, u)
			}
		}
	}
	return pins, unfixed
}

// fixedFrom are the types whose features a query reads values of, nearest first:
// the object's own definition, the element the conditions were written in, then
// the element itself. A type the runtime knows no feature of is left out, so the
// values are read through the declarations this runtime holds.
func fixedFrom(ctx *runtime.Context, f Fixing) []*symbols.Symbol {
	var froms []*symbols.Symbol
	add := func(candidate *symbols.Symbol) {
		if candidate == nil || len(ctx.FeaturesOf(candidate)) == 0 {
			return
		}
		for _, already := range froms {
			if already == candidate {
				return
			}
		}
		froms = append(froms, candidate)
	}
	if f.Object != nil {
		if f.ObjectType != nil {
			add(f.ObjectType)
		}
		add(f.Object.Type)
	}
	add(f.Owner)
	add(f.Element)
	return froms
}

// fixedValue is the value the model fixes for one feature: the one an object
// holds, else the one its declaration states, evaluated where it was written.
func fixedValue(
	ctx *runtime.Context,
	feat *runtime.EffectiveFeature,
	inst *runtime.Instance,
) (runtime.Value, PinSource, int64, error) {
	if inst != nil {
		if fv, ok := inst.FeatureValues[feat.Name]; ok && fv != nil {
			if fv.Materialized {
				return fv.HeldValue(), PinHeld, inst.ID, nil
			}
			// Reading a value feature is what evaluating it does; a variation is
			// read instead from its declaration, since reading it would bind the
			// variant and materialize the object of it.
			if !ctx.IsVariationFeature(feat) {
				read, err := inst.GetFeatureValue(ctx, feat.Name)
				if err != nil {
					return runtime.Value{}, PinHeld, inst.ID, err
				}
				return read.HeldValue(), PinHeld, inst.ID, nil
			}
		}
	}
	if feat.DefaultValue == nil {
		return runtime.Value{}, PinDeclared, 0, nil
	}
	value, err := ctx.EvalWithScopeOn(feat.DefaultValue, feat.DefaultScope(), inst)
	if err != nil {
		return runtime.Value{}, PinDeclared, 0, err
	}
	return value, PinDeclared, 0, nil
}

// fix asserts the fixed values the query's variables read, and records the ones
// no variable reads. One value that cannot be fixed fails the whole query, as one
// untranslatable condition does.
func (t *translator) fix(pins []Pin) error {
	for _, p := range pins {
		v := t.pinnedVar(p)
		if v == nil {
			t.unread = append(t.unread, Unread{
				Pin:    p,
				Reason: "no condition of " + t.subject.Kind + " " + t.subject.Name + " reads it",
			})
			continue
		}
		term, text, err := t.pinTerm(p, v)
		if err != nil {
			return err
		}
		t.pins = append(t.pins, Assertion{
			Term: Binary(OpEq, Bool, VarTerm(v), term),
			From: Provenance{
				Kind:      "feature",
				Element:   v.Name,
				Condition: v.Name + " == " + text + ", " + p.Source.phrase(),
				Role:      RolePinned,
				Declared:  p.Feature,
				File:      v.File,
				Span:      v.Span,
				Location:  v.Location,
			},
		})
		t.pinned = append(t.pinned, PinnedValue{Var: v, Value: text, Source: p.Source, Object: p.Object})
	}
	return nil
}

// pinnedVar is the variable a fixed value fixes: the one standing for that very
// feature, read directly rather than through a chain from another object.
func (t *translator) pinnedVar(p Pin) *Var {
	if p.Feature == nil {
		return nil
	}
	v, ok := t.vars[t.fqn(p.Feature)]
	if !ok || v.Symbol != p.Feature {
		return nil
	}
	return v
}

// Openings of the refusals that report what a pinned feature holds.
const (
	msgValuesAre        = "the feature's values are "
	msgValuesMeasuredIn = "the feature's values are measured in "
)

// pinTerm builds the term a fixed value is asserted equal to, and the value as
// the notation writes it. A value the variable's sort cannot hold refuses.
func (t *translator) pinTerm(p Pin, v *Var) (*Term, string, error) {
	text := pinText(t, p.Value)
	switch p.Value.Kind {
	case runtime.ValConst:
		term, err := t.pinConst(p, v, text)
		return term, text, err
	case runtime.ValString:
		if v.Sort.Kind != SortString {
			return nil, text, t.pinRefusal(p, v, text, msgValuesAre+v.Sort.Name)
		}
		return StringTerm(p.Value.Str()), text, nil
	case runtime.ValQuantity:
		term, err := t.pinQuantity(p, v, text)
		return term, text, err
	case runtime.ValVariant:
		term, err := t.pinDatatype(p, v, text, p.Value.Variant())
		return term, text, err
	case runtime.ValEnumLiteral:
		term, err := t.pinDatatype(p, v, text, p.Value.Literal())
		return term, text, err
	case runtime.ValArray, runtime.ValVector, runtime.ValVectorQuantity, runtime.ValTensorQuantity:
		return nil, text, t.pinRefusal(p, v, text,
			"the term language has scalar variables only, and "+text+" is not a scalar")
	case runtime.ValMeasurementRef:
		return nil, text, t.pinRefusal(p, v, text,
			scalarTermsOnly(text, "a measurement reference"))
	case runtime.ValCoordinateFrame:
		return nil, text, t.pinRefusal(p, v, text,
			scalarTermsOnly(text, "a coordinate frame"))
	case runtime.ValCoordinateTransformation:
		return nil, text, t.pinRefusal(p, v, text,
			scalarTermsOnly(text, "a coordinate transformation"))
	}
	return nil, text, t.pinRefusal(p, v, text, "a "+p.Value.Kind.String()+" has no literal in the term language")
}

// scalarTermsOnly words the refusal of a value the term language has no literal for.
func scalarTermsOnly(text, kind string) string {
	return "the term language ranges over numbers, strings and datatype literals, and " + text + " is " + kind
}

// pinConst fixes a number or a boolean, widening an integer to the real sort the
// feature's values range over.
func (t *translator) pinConst(p Pin, v *Var, text string) (*Term, error) {
	c := p.Value.Const
	switch {
	case c.Kind == semantics.ValBool && v.Sort.Kind == SortBool:
		return BoolTerm(c.Bool), nil
	case c.Kind == semantics.ValInt && v.Sort.Kind == SortInt:
		return IntTerm(c.Int), nil
	case c.Kind == semantics.ValInt && v.Sort.Kind == SortReal:
		if v.Dimension != "" {
			return nil, t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+v.Dimension+", and a bare number states no unit")
		}
		return RealTerm(new(big.Rat).SetInt64(c.Int)), nil
	case c.Kind == semantics.ValReal && v.Sort.Kind == SortReal:
		if v.Dimension != "" {
			return nil, t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+v.Dimension+", and a bare number states no unit")
		}
		rat, ok := ratOfFloat(c.Real)
		if !ok {
			return nil, t.pinRefusal(p, v, text, "it is not an exact rational")
		}
		return RealTerm(rat), nil
	case c.Kind == semantics.ValInfinity:
		return nil, t.pinRefusal(p, v, text, "an infinite magnitude is outside the subset")
	}
	return nil, t.pinRefusal(p, v, text, msgValuesAre+v.Sort.Name)
}

// pinQuantity fixes a quantity, scaled to the base units the variable's
// magnitudes are expressed in exactly as the translator scales a written one, so
// a value stated in another unit of the same dimension fixes the same magnitude.
func (t *translator) pinQuantity(p Pin, v *Var, text string) (*Term, error) {
	if v.Sort.Kind != SortReal {
		return nil, t.pinRefusal(p, v, text, msgValuesAre+v.Sort.Name+" rather than a magnitude")
	}
	quantity := p.Value.Quantity()
	if quantity == nil {
		return nil, t.pinRefusal(p, v, text, "it carries no magnitude")
	}
	unit := quantity.Unit.Term.Normalized()
	if err := t.commensurable(p, v, unit, text); err != nil {
		return nil, err
	}
	scale, ok := ratOfScale(unit.Scale)
	if !ok {
		return nil, t.pinRefusal(p, v, text, "its unit reduces to a scale factor that is not an exact ratio")
	}
	magnitude, ok := ratOfConst(quantity.Num)
	if !ok {
		return nil, t.pinRefusal(p, v, text, "its magnitude is not an exact rational")
	}
	if dim, ok := t.model.DimensionOfUnit(unit); ok {
		t.recordBaseUnits(dim, unit)
	}
	return RealTerm(magnitude.Mul(magnitude, scale)), nil
}

// commensurable refuses a quantity whose dimension is not the one the feature's
// values are measured in, which no scaling could reconcile.
func (t *translator) commensurable(p Pin, v *Var, unit semantics.UnitTerm, text string) error {
	dim, known := t.model.DimensionOfUnit(unit)
	if !known {
		return t.pinRefusal(p, v, text, "the dimension of its unit is not determined statically")
	}
	if want, ok := t.model.DimensionOfFeature(v.Symbol); ok {
		if !want.Term.Commensurable(dim.Term) {
			return t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+dimensionUnits(want)+", not in "+dimensionUnits(dim))
		}
		return nil
	}
	if v.Dimension != dimensionUnits(dim) {
		return t.pinRefusal(p, v, text,
			"the conditions read the feature as "+readAs(v.Dimension)+", not as "+readAs(dimensionUnits(dim)))
	}
	return nil
}

// readAs names a dimension as a message about a mismatch reads it.
func readAs(dimension string) string {
	if dimension == "" {
		return "a plain number"
	}
	return "a magnitude in " + dimension
}

// pinDatatype fixes an enumeration literal or a variant, as the value of the
// finite sort the writer declares for it.
func (t *translator) pinDatatype(p Pin, v *Var, text string, value *symbols.Symbol) (*Term, error) {
	if value == nil {
		return nil, t.pinRefusal(p, v, text, "it names no declaration")
	}
	if v.Sort.Kind != SortDatatype {
		return nil, t.pinRefusal(p, v, text, msgValuesAre+v.Sort.Name)
	}
	name := t.fqn(value)
	for _, candidate := range v.Sort.Values {
		if candidate == name {
			return ValueTerm(v.Sort, name), nil
		}
	}
	return nil, t.pinRefusal(p, v, text, "it is not one of the values of "+v.Sort.Name)
}

// pinRefusal builds the typed error a value that cannot be fixed refuses with.
func (t *translator) pinRefusal(p Pin, v *Var, text, reason string) error {
	feature := p.Name
	if v != nil && v.Name != "" {
		feature = v.Name
	}
	err := &PinError{Feature: feature, Value: text, Reason: reason, Source: p.Source}
	if v != nil {
		err.File, err.Span, err.Location = v.File, v.Span, v.Location
	}
	return err
}

// pinText renders a fixed value as the notation writes it.
func pinText(t *translator, val runtime.Value) string {
	switch val.Kind {
	case runtime.ValConst:
		return constText(val.Const)
	case runtime.ValString:
		return strconv.Quote(val.Str())
	case runtime.ValQuantity:
		if val.Quantity() == nil {
			return "<quantity without a magnitude>"
		}
		return val.Quantity().String()
	case runtime.ValVariant:
		if val.Variant() == nil {
			return "<unknown variant>"
		}
		return t.fqn(val.Variant())
	case runtime.ValEnumLiteral:
		return val.LiteralText()
	case runtime.ValArray, runtime.ValVector, runtime.ValVectorQuantity, runtime.ValTensorQuantity, runtime.ValMeasurementRef,
		runtime.ValCoordinateFrame, runtime.ValCoordinateTransformation:
		return runtime.FormatValue(val)
	default:
		return "a " + val.Kind.String()
	}
}

// constText renders a numeric constant as the notation writes it, exactly.
func constText(c semantics.Value) string {
	switch c.Kind {
	case semantics.ValInt:
		return strconv.FormatInt(c.Int, 10)
	case semantics.ValReal:
		return strconv.FormatFloat(c.Real, 'g', -1, 64)
	case semantics.ValBool:
		return strconv.FormatBool(c.Bool)
	case semantics.ValInfinity:
		return "*"
	default:
		return "<invalid>"
	}
}

// ratOfConst reads a numeric constant as an exact rational.
func ratOfConst(c semantics.Value) (*big.Rat, bool) {
	switch c.Kind {
	case semantics.ValInt:
		return new(big.Rat).SetInt64(c.Int), true
	case semantics.ValReal:
		return ratOfFloat(c.Real)
	}
	return nil, false
}

// ratOfFloat reads a magnitude as the exact rational its shortest decimal form
// denotes, which is the decimal the notation wrote — 5.4 is 27/5, not the binary
// float nearest it, so scaling it by a unit's exact ratio stays exact.
func ratOfFloat(f float64) (*big.Rat, bool) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, false
	}
	return new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
}
