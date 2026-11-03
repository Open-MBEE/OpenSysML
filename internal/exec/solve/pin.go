package solve

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	bareNumberNoUnit = ", and a bare number states no unit"
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

	// Var is the exact query variable the pin fixes, where it names one.
	Var string

	// Value is the value it is fixed to, read where the evaluator reads it.
	Value runtime.Value

	// Source says where the value came from.
	Source PinSource

	// Object is the object holding the value, 0 for a value no object holds.
	Object int64

	// Derived is the expression the model declares the value by, over other
	// features of the same element, for a value the object has not materialized;
	// nil for a literal default or a held value. A derived value is defined by
	// its equation in the query rather than fixed, Value standing in only where
	// the equation cannot be stated.
	Derived *Derived
}

// Derived is a declared value written as an expression over other features of
// the element: the expression, the scope it is read in, and the error reading
// it where the evaluator could not materialize it.
type Derived struct {
	Expr  ast.Node
	Scope *symbols.Scope
	Err   error
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

// DerivedValue is a defining equation the query asserts: the variable it defines
// and the expression the model declares for it, as written.
type DerivedValue struct {
	// Var is the variable the equation defines.
	Var *Var

	// Expression is the declared expression, as the model writes it.
	Expression string

	// Declared is the feature declaration stating the expression.
	Declared *symbols.Symbol
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

	// Var is the exact query variable reading the feature, where it names one:
	// a chain variable, which the feature declaration alone does not identify.
	Var string

	// Reason says why no value was read.
	Reason string

	// Err is the error returned while reading the feature's value.
	Err error
}

// UnreadableRead is a feature a query reads whose value could not be read, and
// which the evaluator reaches only under some condition. The query asserts that
// condition false, so its models never read the feature; Query.Reached asks
// whether any assignment the question is about does.
type UnreadableRead struct {
	// Var is the variable standing for the feature.
	Var *Var

	// Name is the feature as the element naming it writes it.
	Name string

	// Err is the error returned while reading the feature's value.
	Err error

	// Reached is the condition under which the evaluator reads the feature.
	Reached *Term

	// Guard is the term the query asserts to keep its models from reaching the
	// read: the negation of Reached.
	Guard *Term

	// Preceding are the definedness guards of the operations the evaluator
	// performs before reaching the read (Var.Preceding), which an assignment
	// reaching it satisfies.
	Preceding []*Term
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
	features := effectiveFeatures(ctx, sym)
	model := ctx.Semantics()
	for _, feat := range ctx.FeaturesOf(sym) {
		feature := feat
		if feature.Name == "" || feature.Symbol == nil {
			continue
		}
		// A feature typed by a definition holds an object, which no variable of a
		// query stands for; a variation holds the variant selected for it, and
		// one typed by a datatype (`height : LengthValue`) holds a value.
		if ctx.CompositeTypeOf(&feature) != nil && !ctx.IsVariationFeature(&feature) && !model.IsDataType(feature.Type) {
			continue
		}
		// Whether the object holds the value is read before reading it, which
		// materializes a declared value the object had not yet.
		derived := derivedDeclaration(ctx, &feature, inst, features)
		value, source, object, err := fixedValue(ctx, &feature, inst)
		if err == nil {
			// A declared value open for a feature nothing gives a value to does not
			// evaluate, as a failed read does; a feature holding nothing itself stays free.
			if unset := unsetOutside(value.Undetermined(), []*symbols.Symbol{feature.Symbol}); unset != nil {
				err = unsetRead(feature.Name, unset)
			}
		}
		if derived != nil {
			derived.Err = err
			if err != nil || value.Kind == runtime.ValInvalid || value.Kind == runtime.ValUndetermined || ctx.HoldsNoValue(value) {
				value = runtime.Value{}
			}
			pins = append(pins, Pin{Feature: feature.Symbol, Name: feature.Name, Value: value, Source: PinDeclared, Derived: derived})
			continue
		}
		switch {
		case err != nil:
			unfixed = append(unfixed, Unfixed{Feature: feature.Symbol, Name: feature.Name, Reason: err.Error(), Err: err})
		case value.Kind == runtime.ValInvalid, value.Kind == runtime.ValUndetermined, ctx.HoldsNoValue(value):
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

// derivedDeclaration is the expression the model declares feat's value by when
// it is derived — written over other features of the element itself, among
// features, and over literals — and inst holds no materialized value for it;
// nil for a literal default, a value the object holds or a variation, which
// stay fixed as they are. One reading anything else is not derived either: a
// chain (`inner.x + 1.0`), a qualified or imported name (`test::scale * r`)
// name values the query fixes none of, which an equation would leave to the
// solver, so the value the evaluator derives from them is what the query fixes.
// A unit (`[m]`) is a measurement reference, not a value read.
func derivedDeclaration(ctx *runtime.Context, feat *runtime.EffectiveFeature, inst *runtime.Instance, features map[string]*symbols.Symbol) *Derived {
	if feat.DefaultValue == nil || ctx.IsVariationFeature(feat) {
		return nil
	}
	if inst != nil {
		if fv, ok := inst.FeatureValues[feat.Name]; ok && fv != nil && fv.Materialized {
			return nil
		}
	}
	reads, outside := false, false
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch ref := n.(type) {
		case *ast.IndexExpr:
			if ref.Bracket {
				ast.Inspect(ref.Operand, visit)
				return false
			}
		case *ast.FeatureChainExpr:
			outside = true
			return false
		case *ast.FeatureReference:
			segments, ok := nameSegments(ref.Name)
			if sym, own := features[segments[0]]; ok && len(segments) == 1 && own && sym != feat.Symbol {
				reads = true
			} else {
				outside = true
			}
			return false
		}
		return !outside
	}
	ast.Inspect(feat.DefaultValue, visit)
	if !reads || outside {
		return nil
	}
	return &Derived{Expr: feat.DefaultValue, Scope: feat.DefaultScope()}
}

// fix asserts the fixed values the query's variables read, and records the ones
// no variable reads. One value that cannot be fixed fails the whole query, as one
// untranslatable condition does.
func (t *translator) fix(pins []Pin) error {
	// A defining equation reads the features it is written over, which may be
	// variables no condition read yet, so the equations are stated first, each
	// once the variable it defines exists, and the fixed values after.
	var derived, plain []Pin
	for _, p := range pins {
		if p.Derived != nil {
			derived = append(derived, p)
		} else {
			plain = append(plain, p)
		}
	}
	for progress := true; progress && len(derived) > 0; {
		progress = false
		var pending []Pin
		for _, p := range derived {
			v := t.pinnedVar(p)
			if v == nil {
				pending = append(pending, p)
				continue
			}
			if err := t.derive(p, v); err != nil {
				return err
			}
			progress = true
		}
		derived = pending
	}
	for _, p := range append(derived, plain...) {
		v := t.pinnedVar(p)
		if v == nil {
			t.unread = append(t.unread, Unread{
				Pin:    p,
				Reason: "no condition of " + t.subject.Kind + " " + t.subject.Name + " reads it",
			})
			continue
		}
		if err := t.fixOne(p, v); err != nil {
			return err
		}
	}
	return nil
}

// fixOne asserts v == the value p fixes, and records the fixed value for a report.
func (t *translator) fixOne(p Pin, v *Var) error {
	term, text, err := t.pinTerm(p, v)
	if err != nil {
		return err
	}
	t.pins = append(t.pins, Assertion{
		Term: Binary(OpEq, Bool, VarTerm(v), term),
		From: Provenance{
			Kind:      "feature",
			Element:   source.QualifiedNameText(v.Name),
			Condition: source.QualifiedNameText(v.Name) + " == " + ValueText(v, text) + ", " + p.Source.phrase(),
			Role:      RolePinned,
			Declared:  p.Feature,
			File:      v.File,
			Span:      v.Span,
			Location:  v.Location,
		},
	})
	t.pinned = append(t.pinned, PinnedValue{Var: v, Value: text, Source: p.Source, Object: p.Object})
	return nil
}

// derive asserts the defining equation of a derived declared value, v == the
// expression the model declares, so the solver chooses for v what the model
// derives from the other values. A value the evaluator failed to derive from
// values it had (a division by zero) is unreadable, as any failed read is: the
// equation stands in only for one it could not derive for want of a value. The
// equation is read where the evaluator reads v — under v's Reached, a division
// in it then refusing to hoist its guard, as one in a short-circuited operand
// does — and an expression the term language does not carry leaves the value
// the evaluator derived fixed, as before, else the read guarded or refused.
func (t *translator) derive(p Pin, v *Var) error {
	d := p.Derived
	if d.Err != nil && !errors.Is(d.Err, runtime.ErrNoValue) {
		return t.unreadable(v, p.Name, d.Err)
	}
	text := t.ctx.ExprText(d.Expr, d.Scope)
	name := source.QualifiedNameText(v.Name)
	t.condLabel = name + " = " + text
	t.condFile = p.Feature.DocName
	t.frame = frame{}
	term, err := t.under(v.Reached, func() (*Term, error) {
		if v.Reached != nil {
			t.branched++
			defer func() { t.branched-- }()
		}
		return t.expr(d.Expr, d.Scope)
	})
	if err == nil {
		term, err = t.definedAs(p, v, text, term)
	}
	if err != nil {
		if p.Value.Kind != runtime.ValInvalid {
			p.Derived = nil
			return t.fixOne(p, v)
		}
		if d.Err == nil {
			return nil
		}
		return t.unreadable(v, p.Name, d.Err)
	}
	span := d.Expr.Span()
	equation := Binary(OpEq, Bool, VarTerm(v), term)
	if v.Reached != nil {
		equation = Binary(OpImplies, Bool, v.Reached, equation)
	}
	t.equations = append(t.equations, Assertion{
		Term: equation,
		From: Provenance{
			Kind:      "feature",
			Element:   name,
			Condition: name + " == " + text + ", " + p.Source.phrase(),
			Role:      RoleDerived,
			Declared:  p.Feature,
			File:      p.Feature.DocName,
			Span:      span,
			Location:  t.ctx.SourceLocation(p.Feature.DocName, span),
		},
	})
	t.derived = append(t.derived, DerivedValue{Var: v, Expression: text, Declared: p.Feature})
	return nil
}

// definedAs is term as a value of v's sort: an integer widened to the real sort
// v ranges over, else a term of v's sort as it is; a term of another sort is
// refused, as a value of another sort pinned to v is.
func (t *translator) definedAs(p Pin, v *Var, text string, term *Term) (*Term, error) {
	switch {
	case term.Sort.Kind == v.Sort.Kind && (term.Sort.Kind != SortDatatype || term.Sort.Name == v.Sort.Name):
		return term, nil
	case v.Sort.Kind == SortReal && term.Sort.Kind == SortInt:
		return ToReal(term), nil
	}
	return nil, t.pinRefusal(p, v, text, msgValuesAre+v.Sort.Name+", and "+text+" yields "+term.Sort.Name)
}

// unreadable guards the read of v, whose value could not be read, by the
// condition under which the evaluator reaches it, so that no model reaches the
// read; a read on every path is refused, as no assignment avoids it.
func (t *translator) unreadable(v *Var, name string, err error) error {
	if v.Reached == nil {
		return fmt.Errorf("%s %s reads %s, whose value could not be read: %w", t.subject.Kind, t.subject.Name, name, err)
	}
	guard := Not(v.Reached)
	t.unreadables = append(t.unreadables, UnreadableRead{Var: v, Name: name, Err: err, Reached: v.Reached, Guard: guard, Preceding: v.Preceding})
	t.guards = append(t.guards, Assertion{Term: guard, From: Provenance{
		Kind:      t.subject.Kind,
		Element:   t.subject.Name,
		Condition: fmt.Sprintf("%s is not read, as its value could not be read: %v", name, err),
		Role:      RoleDefined,
		Declared:  v.Symbol,
		File:      v.File,
		Span:      v.Span,
		Location:  v.Location,
	}})
	return nil
}

// pinnedVar is the variable a fixed value fixes: the one standing for that very
// feature, read directly rather than through a chain from another object.
func (t *translator) pinnedVar(p Pin) *Var {
	if p.Var != "" {
		return t.vars[p.Var]
	}
	if p.Feature == nil {
		return nil
	}
	for _, sym := range append([]*symbols.Symbol{p.Feature}, t.model.AllRedefinedFeatures(p.Feature)...) {
		if v, ok := t.vars[t.fqn(sym)]; ok && v.Symbol == sym {
			return v
		}
	}
	// A value fixed on a redefinition fixes what it redefines too, since both
	// names read the one value — so it also fixes a chain var standing for one
	// of them. But a chain var names the feature through an object, so it is
	// pinned only when its root denotes the pinned object: a subject member
	// the object fills (`vehicle` where `by craft` binds it), or the very
	// usage the pin's feature belongs to (`hg` for `hg.power`). Another
	// usage's root is a different object — `other.power` is never `eng`'s.
	var best *Var
	for _, v := range t.vars {
		if v.Root == nil || v.Root == v.Symbol {
			continue
		}
		matched := v.Symbol == p.Feature
		if !matched {
			for _, redefined := range t.model.AllRedefinedFeatures(p.Feature) {
				if v.Symbol == redefined {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}
		subject := subjectDecl(v.Root)
		owned := p.Feature.OwnerScope != nil && p.Feature.OwnerScope.Owner() == v.Root
		if !subject && !owned {
			continue
		}
		if best == nil || v.Name < best.Name {
			best = v
		}
	}
	return best
}

// Openings of the refusals that report what a pinned feature holds.
const (
	msgValuesAre        = "the feature's values are "
	msgValuesMeasuredIn = "the feature's values are measured in "
)

// pinTerm builds the term a fixed value is asserted equal to, and the value as
// the notation writes it. A value the variable's sort cannot hold refuses.
func (t *translator) pinTerm(p Pin, v *Var) (*Term, string, error) {
	// A one-element collection is its element (KerML 1.0 §7.4.8), so a multi-valued
	// feature holding one value pins as that value.
	if element, ok := soleHeld(p.Value); ok {
		p.Value = element
	}
	text := pinText(t, p.Value)
	// A literal of a scalar-valued enumeration pins a datatype variable by its
	// identity, and an Integer or Real one by the scalar it equals.
	if lit := p.Value.EnumerationLiteral(); lit != nil && v.Sort.Kind == SortDatatype {
		term, err := t.pinDatatype(p, v, text, lit)
		return term, text, err
	}
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
		return BigIntTerm(c.BigInt()), nil
	case c.Kind == semantics.ValInt && v.Sort.Kind == SortReal:
		if v.Dimension != "" {
			return nil, t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+v.Dimension+bareNumberNoUnit)
		}
		return RealTerm(new(big.Rat).SetInt(c.BigInt())), nil
	case c.Kind == semantics.ValRational && v.Sort.Kind == SortReal:
		if v.Dimension != "" {
			return nil, t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+v.Dimension+bareNumberNoUnit)
		}
		return RealTerm(c.Rat()), nil
	case c.Kind == semantics.ValReal && v.Sort.Kind == SortReal:
		if v.Dimension != "" {
			return nil, t.pinRefusal(p, v, text,
				msgValuesMeasuredIn+v.Dimension+bareNumberNoUnit)
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

// ValueText writes a value stated for v as the notation does: a datatype's value is
// a qualified name, quoted where it needs quoting; any other is already written so.
func ValueText(v *Var, value string) string {
	if v != nil && v.Sort.Kind == SortDatatype {
		return source.QualifiedNameText(value)
	}
	return value
}

// pinText renders a fixed value as the notation writes it.
func pinText(t *translator, val runtime.Value) string {
	if lit := val.EnumerationLiteral(); lit != nil {
		return runtime.NewEnumLiteral(lit).LiteralText()
	}
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
		return c.FormatInt()
	case semantics.ValRational:
		return c.FormatRational()
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
		return new(big.Rat).SetInt(c.BigInt()), true
	case semantics.ValRational:
		return c.Rat(), true
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

// A Reader reads the value ref, written in scope, names, as the query's conditions read it.
type Reader func(scope *symbols.Scope, ref ast.Node) (runtime.Value, error)

// ObjectReader reads a reference in the scope of an object.
func ObjectReader(ctx *runtime.Context, self *runtime.Instance) Reader {
	return func(scope *symbols.Scope, ref ast.Node) (runtime.Value, error) {
		return runtime.NewEvalContextIn(ctx, scope, self).Eval(ref)
	}
}

// SatisfactionReader reads a reference with the requirement bindings of a satisfaction.
func SatisfactionReader(ctx *runtime.Context, a *runtime.SatisfyAssertion, subject *runtime.Instance) Reader {
	return func(scope *symbols.Scope, ref ast.Node) (runtime.Value, error) {
		return ctx.ReadInSatisfaction(a, subject, scope, ref)
	}
}

// ChainPins reads the value each unpinned chain variable names through read:
// a feature a chain of two or more steps reaches, which no fixed value names
// directly. A chain reaching a feature that holds nothing stays free. One whose
// read fails — a declared value that does not evaluate (the runtime marks it
// ErrFeatureValueMaterialization), or any other failure — is an Unfixed naming
// the variable, for UnfixedRead to guard or refuse as it does a direct read.
func ChainPins(ctx *runtime.Context, q *Query, read Reader, self *runtime.Instance) ([]Pin, []Unfixed) {
	pinned := make(map[string]bool, len(q.Pinned))
	for _, p := range q.Pinned {
		pinned[p.Var.Name] = true
	}
	var out []Pin
	var unfixed []Unfixed
	for _, v := range q.Vars {
		if len(v.Steps) < 2 || v.Ref == nil || v.Scope == nil || pinned[v.Name] {
			continue
		}
		value, err := read(v.Scope, v.Ref)
		if err != nil {
			if errors.Is(err, runtime.ErrNoValue) && !errors.Is(err, runtime.ErrFeatureValueMaterialization) {
				continue
			}
			unfixed = append(unfixed, Unfixed{Feature: v.Symbol, Name: v.Name, Var: v.Name, Reason: err.Error(), Err: err})
			continue
		}
		if unset := unsetOutside(value.Undetermined(), sharedSteps(ctx, v.Steps)); unset != nil {
			// A declared value open for a feature nothing gives a value to does not evaluate,
			// as a failed read does; a chain whose own step holds nothing stays free.
			spelled := symbols.LastSegment(v.Name)
			err := unsetRead(spelled, unset)
			unfixed = append(unfixed, Unfixed{Feature: v.Symbol, Name: v.Name, Var: v.Name, Reason: err.Error(), Err: err})
			continue
		}
		if value.Kind == runtime.ValInvalid || value.Kind == runtime.ValUndetermined || ctx.HoldsNoValue(value) {
			continue
		}
		source := PinDeclared
		var object int64
		if self != nil {
			source, object = PinHeld, self.ID
		}
		out = append(out, Pin{Feature: v.Symbol, Name: v.Name, Var: v.Name, Value: value, Source: source, Object: object})
	}
	return out, unfixed
}

// UnfixedRead is the read failure of an unfixed feature a variable of q reads on
// every path, nil when it reads none. A feature the evaluator reaches only under
// some condition — the right operand of a short-circuiting connective, a branch
// of a conditional, a condition after another required one — is guarded instead:
// q asserts that condition false in role RoleDefined and names the read in
// Query.Unreadable, so its models are the assignments the evaluator answers about
// without reading the feature. A failed default is never left free: an assignment
// reaching it has no value for it, and Query.Reached asks whether one exists.
func UnfixedRead(ctx *runtime.Context, q *Query, unfixed []Unfixed) error {
	t := &translator{ctx: ctx, model: ctx.Semantics(), subject: Subject{Kind: q.Kind, Name: q.Element}, vars: make(map[string]*Var, len(q.Vars))}
	for _, v := range q.Vars {
		t.vars[v.Name] = v
	}
	guarded := make(map[*Var]bool, len(q.Unreadable))
	for _, u := range q.Unreadable {
		guarded[u.Var] = true
	}
	for _, u := range unfixed {
		if u.Err == nil {
			continue
		}
		v := t.pinnedVar(Pin{Feature: u.Feature, Name: u.Name, Var: u.Var})
		if v == nil || guarded[v] {
			continue
		}
		if err := t.unreadable(v, u.Name, u.Err); err != nil {
			return err
		}
		guarded[v] = true
	}
	guards := t.guards
	q.Unreadable = append(q.Unreadable, t.unreadables...)
	if len(guards) == 0 {
		return nil
	}
	at := len(q.Assertions)
	for i, a := range q.Assertions {
		if !contextRole(a.From.Role) {
			at = i
			break
		}
	}
	assertions := make([]Assertion, 0, len(q.Assertions)+len(guards))
	assertions = append(assertions, q.Assertions[:at]...)
	assertions = append(assertions, guards...)
	q.Assertions = append(assertions, q.Assertions[at:]...)
	return nil
}

// ReadUnfixed is the subset of unfixed that some variable of q reads, in order:
// the features whose failed read bears on q, as UnfixedRead guards or refuses
// them. One no condition of q reads bears on no answer, as a fixed value no
// condition reads does not.
func ReadUnfixed(q *Query, unfixed []Unfixed) []Unfixed {
	if q == nil || len(unfixed) == 0 {
		return nil
	}
	t := &translator{vars: make(map[string]*Var, len(q.Vars))}
	for _, v := range q.Vars {
		t.vars[v.Name] = v
	}
	var out []Unfixed
	for _, u := range unfixed {
		if t.pinnedVar(Pin{Feature: u.Feature, Name: u.Name, Var: u.Var}) != nil {
			out = append(out, u)
		}
	}
	return out
}

// contextRole reports whether a role bounds the assignments a query is about
// rather than stating a condition the element claims.
func contextRole(role Role) bool {
	switch role {
	case RoleDomain, RolePinned, RoleDerived, RoleDefined:
		return true
	}
	return false
}

// Reached is the query whose models are the assignments of the free features
// under which the evaluator reads a feature whose value could not be read: q's
// domains, fixed values and assumptions, with the disjunction of the unreadable
// reads required — each read's condition together with the definedness guards
// of the operations the evaluator performs before reaching it. A guard hoisted
// from an operation after the read — a later required condition's divisor —
// constrains no assignment reaching it: the evaluator fails at the read and
// never performs the operation, so imposing the guard could hide the
// assignment and certify a false proof. Nil for a query reading no such
// feature. Unsat proves that every assignment the question is about evaluates
// without the read, so q's verdict is the evaluator's; sat witnesses an
// assignment on which the evaluator reaches no verdict, which leaves a proof or
// a refutation of q undecided (ReachedError).
func (q *Query) Reached() *Query {
	if len(q.Unreadable) == 0 {
		return nil
	}
	out := &Query{
		Kind:            q.Kind,
		Element:         q.Element,
		Sorts:           q.Sorts,
		Vars:            q.Vars,
		Nonlinear:       q.Nonlinear,
		IntegerDivision: q.IntegerDivision,
		Pinned:          q.Pinned,
	}
	reads := make([]*Term, 0, len(q.Unreadable))
	names := make([]string, 0, len(q.Unreadable))
	for _, u := range q.Unreadable {
		reads = append(reads, and(conjunction(u.Preceding), u.Reached))
		names = append(names, u.Name)
	}
	for _, a := range q.Assertions {
		if a.From.Role == RoleDefined {
			continue
		}
		if contextRole(a.From.Role) || a.From.Role == RoleAssumed {
			out.Assertions = append(out.Assertions, a)
		}
	}
	out.Assertions = append(out.Assertions, Assertion{Term: Or(reads...), From: Provenance{
		Kind:      q.Kind,
		Element:   q.Element,
		Condition: strings.Join(names, " or ") + " is read",
		Role:      RoleRequired,
	}})
	return out
}

// ReachedError is the error a question about q reports when Reached is
// satisfiable: some assignment of the free features reads a feature whose value
// could not be read, so the evaluator reaches no verdict there.
func (q *Query) ReachedError() error {
	if len(q.Unreadable) == 0 {
		return nil
	}
	if len(q.Unreadable) == 1 {
		u := q.Unreadable[0]
		return fmt.Errorf("%s %s reads %s under some assignment of its free features, whose value could not be read: %w",
			q.Kind, q.Element, u.Name, u.Err)
	}
	names := make([]string, 0, len(q.Unreadable))
	errs := make([]error, 0, len(q.Unreadable))
	for _, u := range q.Unreadable {
		names = append(names, u.Name)
		errs = append(errs, fmt.Errorf("%s: %w", u.Name, u.Err))
	}
	return fmt.Errorf("%s %s reads %s under some assignment of its free features, whose values could not be read: %w",
		q.Kind, q.Element, strings.Join(names, ", "), errors.Join(errs...))
}

// soleHeld is the one element a sequence or set of one holds, when it does.
func soleHeld(v runtime.Value) (runtime.Value, bool) {
	var elements []runtime.Value
	switch v.Kind {
	case runtime.ValSequence:
		if v.Sequence() != nil {
			elements = v.Sequence().Elements()
		}
	case runtime.ValSet:
		if v.Set() != nil {
			elements = v.Set().Elements()
		}
	default:
		return runtime.Value{}, false
	}
	if len(elements) != 1 {
		return runtime.Value{}, false
	}
	return elements[0], true
}

// unsetRead is the error reading a declared value fails with when a feature it
// reads holds nothing: the one the evaluator raises materialising the value.
func unsetRead(spelled string, unset *symbols.Symbol) error {
	noValue := &runtime.NoValueError{Feature: unset.Name, Symbol: unset}
	return &runtime.FeatureValueError{Err: fmt.Errorf("feature value %s: %w", spelled, noValue)}
}

// unsetOutside is the first feature with no value u is open for that is none of the
// chain's own steps, nil when u is determined or open only for its own steps or a count.
// sharedSteps is steps plus the features each step redefines: a redefinition
// shares its target's value, so a chain through a satisfy's `by` subject holds
// what the requirement's subject it redefines holds.
func sharedSteps(ctx *runtime.Context, steps []*symbols.Symbol) []*symbols.Symbol {
	model := ctx.Semantics()
	out := append([]*symbols.Symbol(nil), steps...)
	for _, step := range steps {
		out = append(out, model.RedefinedFeatures(step)...)
		out = append(out, model.ImplicitRoleRedefinitions(step)...)
	}
	return out
}

func unsetOutside(u *runtime.Undetermined, steps []*symbols.Symbol) *symbols.Symbol {
	for _, unset := range u.Unsets() {
		if !slices.Contains(steps, unset) {
			return unset
		}
	}
	return nil
}
