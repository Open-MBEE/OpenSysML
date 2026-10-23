package solve

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// crateSource derives one attribute from three others nothing gives a value
// to, under bounds on the three and a condition on the derived one.
const crateSource = `package Box {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	part def Crate {
		attribute height : LengthValue;
		attribute width : LengthValue;
		attribute length : LengthValue;
		attribute volume : VolumeValue = height * width * length;
		assert constraint bounds {
			height >= 0.1 [m] and height <= 1.0 [m] and
			width >= 1.0 [m] and width <= 1.1 [m] and
			length >= 1.0 [m] and length <= 1.1 [m]
		}
		assert constraint fits { volume >= 1.0 [m**3] and volume <= 2.0 [m**3] }
	}
	part crateUnset : Crate;
	part crateBad : Crate {
		attribute :>> height = 0.1 [m];
		attribute :>> width = 1.0 [m];
		attribute :>> length = 1.0 [m];
	}
	part def Holder {
		part crate : Crate;
		assert constraint holds { crate.volume >= 1.0 [m**3] }
	}
	part holder : Holder;
}`

// derivedPin is the pin Fixed reads for name, which must carry a derived
// declaration, among pins.
func derivedPin(t *testing.T, pins []Pin, name string) Pin {
	t.Helper()
	for _, p := range pins {
		if p.Name == name {
			if p.Derived == nil {
				t.Fatalf("%s = %+v, want a derived declaration", name, p)
			}
			return p
		}
	}
	t.Fatalf("pins = %+v, want %s among them", pins, name)
	return Pin{}
}

// TestFixedReadsADerivedValueAsItsDeclaration: a declared value written over
// other features of the element is read as that expression, not fixed to the
// value it evaluates to — none, where a feature it reads holds nothing — and
// the query asserts it as the defining equation of its variable, with the
// declaration's provenance, so the solver chooses the value the model derives.
func TestFixedReadsADerivedValueAsItsDeclaration(t *testing.T) {
	ctx, idx := fixture(t, "crate.sysml", crateSource)
	crate := symbolNamed(t, idx, "Box::Crate")
	pins, unfixed := Fixed(ctx, crate, nil)
	volume := derivedPin(t, pins, "volume")
	if volume.Value.Kind != runtime.ValInvalid || volume.Source != PinDeclared {
		t.Fatalf("volume = %+v, want no value, declared", volume)
	}
	var noValue *runtime.NoValueError
	if !errors.As(volume.Derived.Err, &noValue) || noValue.Symbol != symbolNamed(t, idx, "Box::Crate::height") {
		t.Fatalf("volume.Derived.Err = %v, want the read of height to be what fails", volume.Derived.Err)
	}
	for _, p := range pins {
		switch p.Name {
		case "height", "width", "length":
			t.Fatalf("%s was fixed to %v, want it free", p.Name, p.Value)
		}
	}
	for _, u := range unfixed {
		if u.Name == "volume" {
			t.Fatalf("unfixed = %+v, want volume carried as a derived value", unfixed)
		}
	}

	fits := symbolNamed(t, idx, "Box::Crate::fits")
	q, err := ConstraintWith(ctx, fits, fits.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate fits: %v", err)
	}
	if len(q.Derived) != 1 || q.Derived[0].Var != varNamed(t, q, "Box::Crate::volume") ||
		q.Derived[0].Expression != "height * width * length" || q.Derived[0].Declared != volume.Feature {
		t.Fatalf("Derived = %+v, want volume = height * width * length", q.Derived)
	}
	for _, name := range []string{"height", "width", "length"} {
		varNamed(t, q, "Box::Crate::"+name)
	}
	if len(q.Pinned) != 0 || len(q.Unreadable) != 0 || !q.Nonlinear {
		t.Fatalf("Pinned = %+v, Unreadable = %+v, Nonlinear = %v, want the product asserted alone", q.Pinned, q.Unreadable, q.Nonlinear)
	}
	var equation *Assertion
	for i := range q.Assertions {
		if q.Assertions[i].From.Role == RoleDerived {
			equation = &q.Assertions[i]
		}
	}
	if equation == nil {
		t.Fatalf("no defining equation asserted; script:\n%s", Script(q))
	}
	from := equation.From
	if from.Condition != "Box::Crate::volume == height * width * length, declared by the model" ||
		from.Declared != volume.Feature || from.File != "crate.sysml" || from.Location != "crate.sysml:9:36" {
		t.Errorf("equation provenance = %+v", from)
	}
	if term := writeTerm(equation.Term); term != "(= |Box::Crate::volume| (* (* |Box::Crate::height| |Box::Crate::width|) |Box::Crate::length|))" {
		t.Errorf("equation = %s", term)
	}
	if !contextRole(RoleDerived) {
		t.Errorf("a defining equation is not a context role")
	}
	if err := UnfixedRead(ctx, q, unfixed); err != nil {
		t.Fatalf("UnfixedRead fits = %v, want nil: nothing the query reads failed", err)
	}
}

// TestDerivedValueThroughAChainStaysAReadValue: read through an object of the
// element, a derived value is the value that object derives — its equation is
// over the held object's features, not the element's — so none, here, is a
// failed read, refused where the read is unconditional; a constraint alone
// reads the chain as a free variable, as before.
func TestDerivedValueThroughAChainStaysAReadValue(t *testing.T) {
	ctx, idx := fixture(t, "crate.sysml", crateSource)
	holder := symbolNamed(t, idx, "Box::Holder")
	self, err := ctx.Instantiate(symbolNamed(t, idx, "Box::holder"))
	if err != nil {
		t.Fatalf("instantiate holder: %v", err)
	}
	pins, _ := Fixed(ctx, holder, self)
	_, err = AssertedOn(ctx, holder, holder.OwnerScope, pins, self)
	if err == nil || !errors.Is(err, runtime.ErrNoValue) || !strings.Contains(err.Error(), "crate.volume, whose value could not be read") {
		t.Fatalf("translate the holder's assertions = %v, want the read of crate.volume refused for want of height", err)
	}
	holds := symbolNamed(t, idx, "Box::Holder::holds")
	q, err := ConstraintWith(ctx, holds, holds.OwnerScope, nil)
	if err != nil || len(q.Derived) != 0 || len(q.Free()) != 1 {
		t.Fatalf("translate holds = %+v, %v, want crate.volume free and no equation", q, err)
	}
}

// TestLiteralAndHeldValuesStayFixed: a literal default is fixed as before, a
// derived value the evaluator can derive is still read as its equation, and
// one the object has materialized is fixed to the value it holds.
func TestLiteralAndHeldValuesStayFixed(t *testing.T) {
	ctx, idx := fixture(t, "derived_held.sysml", `package test {
		private import ScalarValues::*;
		part def Item {
			attribute base : Real = 2.0;
			attribute scaled : Real = base * 3.0;
			attribute plain : Real = 1.0 + 2.0;
			assert constraint c { scaled > 0.0 and plain > 0.0 }
		}
		part item : Item;
	}`)
	item := symbolNamed(t, idx, "test::Item")
	pins, unfixed := Fixed(ctx, item, nil)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	byName := map[string]Pin{}
	for _, p := range pins {
		byName[p.Name] = p
	}
	if base := byName["base"]; base.Derived != nil || base.Value.Kind != runtime.ValConst {
		t.Errorf("base = %+v, want a literal fixed value", base)
	}
	if plain := byName["plain"]; plain.Derived != nil || plain.Value.Kind != runtime.ValConst {
		t.Errorf("plain = %+v, want an expression over no feature fixed to its value", plain)
	}
	scaled := derivedPin(t, pins, "scaled")
	if scaled.Value.Kind != runtime.ValConst || scaled.Derived.Err != nil {
		t.Errorf("scaled = %+v, want the value the evaluator derives carried along", scaled)
	}
	c := symbolNamed(t, idx, "test::Item::c")
	q, err := ConstraintWith(ctx, c, c.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate c: %v", err)
	}
	if len(q.Derived) != 1 || q.Derived[0].Expression != "base * 3.0" || len(q.Pinned) != 2 {
		t.Fatalf("Derived = %+v, Pinned = %+v, want scaled defined and base, plain fixed", q.Derived, q.Pinned)
	}

	inst, err := ctx.Instantiate(symbolNamed(t, idx, "test::item"))
	if err != nil {
		t.Fatalf("instantiate item: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "scaled"); err != nil {
		t.Fatalf("read scaled: %v", err)
	}
	held, _ := Fixed(ctx, item, inst)
	for _, p := range held {
		if p.Name == "scaled" && (p.Derived != nil || p.Source != PinHeld) {
			t.Fatalf("scaled = %+v, want the materialized value held by the object fixed", p)
		}
	}
	q, err = ConstraintWith(ctx, c, c.OwnerScope, held)
	if err != nil {
		t.Fatalf("translate c over the object: %v", err)
	}
	if len(q.Derived) != 0 || len(q.Pinned) != 2 || q.Pinned[0].Source != PinHeld || q.Pinned[1].Source != PinHeld {
		t.Fatalf("Derived = %+v, Pinned = %+v, want the two values c reads fixed as held", q.Derived, q.Pinned)
	}
}

// TestSolvedDerivedValueIsTheProduct: solved, the witness's volume is the
// product of the dimensions it chose, and over an object holding dimensions
// whose product no volume in range matches, the conflict names the held values
// and the equation.
func TestSolvedDerivedValueIsTheProduct(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "crate.sysml", crateSource)
	crate := symbolNamed(t, idx, "Box::Crate")
	unset, err := ctx.Instantiate(symbolNamed(t, idx, "Box::crateUnset"))
	if err != nil {
		t.Fatalf("instantiate crateUnset: %v", err)
	}
	pins, unfixed := Fixed(ctx, crate, unset)
	q, err := AssertedOn(ctx, crate, crate.OwnerScope, pins, unset)
	if err != nil {
		t.Fatalf("translate the crate's assertions: %v", err)
	}
	if err := UnfixedRead(ctx, q, unfixed); err != nil {
		t.Fatalf("UnfixedRead: %v", err)
	}
	values := modelValues(t, solved(t, solver, q, StatusSat))
	product := big.NewRat(1, 1)
	for _, name := range []string{"height", "width", "length"} {
		product.Mul(product, exactRat(t, values["Box::Crate::"+name]))
	}
	volume := exactRat(t, values["Box::Crate::volume"])
	if volume.Cmp(product) != 0 {
		t.Errorf("volume = %s, want the product %s of the dimensions in %v", volume, product, values)
	}
	if volume.Cmp(big.NewRat(1, 1)) < 0 || volume.Cmp(big.NewRat(2, 1)) > 0 {
		t.Errorf("volume = %s, want one fits admits", volume)
	}

	bad, err := ctx.Instantiate(symbolNamed(t, idx, "Box::crateBad"))
	if err != nil {
		t.Fatalf("instantiate crateBad: %v", err)
	}
	pins, _ = Fixed(ctx, crate, bad)
	derivedPin(t, pins, "volume")
	fits := symbolNamed(t, idx, "Box::Crate::fits")
	q, err = ConstraintWith(ctx, fits, fits.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate fits over crateBad: %v", err)
	}
	if len(q.Pinned) != 3 || len(q.Derived) != 1 {
		t.Fatalf("Pinned = %+v, Derived = %+v, want the three held dimensions fixed and volume defined", q.Pinned, q.Derived)
	}
	solved(t, solver, q, StatusUnsat)
	explained, err := solver.Explain(t.Context(), q)
	if err != nil || explained.Core == nil {
		t.Fatalf("explain = %+v, %v", explained, err)
	}
	roles := map[Role]int{}
	for _, m := range explained.Core.Members {
		roles[m.From.Role]++
	}
	if roles[RoleDerived] != 1 || roles[RolePinned] == 0 || roles[RoleRequired] != 1 {
		t.Errorf("core roles = %v, want the equation, a held value and fits", roles)
	}
}

// exactRat is the exact rational a rendered quantity or number carries.
func exactRat(t *testing.T, text string) *big.Rat {
	t.Helper()
	number := strings.Fields(text)[0]
	r, ok := new(big.Rat).SetString(number)
	if !ok {
		t.Fatalf("value %q is not an exact rational", text)
	}
	return r
}

// TestDerivedEquationIsReadWhereTheEvaluatorReadsIt: a derived value a
// condition reads only on some paths is defined where it is reached, and a
// division in it refuses to hoist its guard there, as one in a short-circuited
// operand does — so over a divisor fixed to zero the skipped read is guarded
// rather than ruling the condition out; the read on every path is refused with
// the evaluator's own failure, as a failed read is.
func TestDerivedEquationIsReadWhereTheEvaluatorReadsIt(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "derived_reach.sysml", `package test {
		private import ScalarValues::*;
		part def Item {
			attribute x : Real = 0.0;
			attribute free : Real;
			attribute z : Real = 1.0 / x;
			attribute w : Real = 1.0 / free;
			assert constraint skipped { true or z > 0.0 }
			assert constraint always { z > 0.0 }
			assert constraint sometimes { free > 0.0 or w > 0.0 }
			assert constraint divides { w > 0.0 }
			assert constraint tautology { true and w > 0.0 }
		}
	}`)
	item := symbolNamed(t, idx, "test::Item")
	pins, unfixed := Fixed(ctx, item, nil)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none: a derived value carries its own failure", unfixed)
	}
	z := derivedPin(t, pins, "z")
	if z.Value.Kind != runtime.ValInvalid || !errors.Is(z.Derived.Err, runtime.ErrDivisionByZero) {
		t.Fatalf("z = %+v, want the evaluator's division by zero carried", z)
	}

	skipped := symbolNamed(t, idx, "test::Item::skipped")
	q, err := ConstraintWith(ctx, skipped, skipped.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate skipped: %v", err)
	}
	if len(q.Derived) != 0 || len(q.Unreadable) != 1 || q.Unreadable[0].Name != "z" || !errors.Is(q.Unreadable[0].Err, runtime.ErrDivisionByZero) {
		t.Fatalf("Derived = %+v, Unreadable = %+v, want the skipped read of z guarded", q.Derived, q.Unreadable)
	}
	solved(t, solver, q, StatusSat)

	always := symbolNamed(t, idx, "test::Item::always")
	if _, err := ConstraintWith(ctx, always, always.OwnerScope, pins); err == nil ||
		!errors.Is(err, runtime.ErrDivisionByZero) || !strings.Contains(err.Error(), "reads z, whose value could not be read") {
		t.Fatalf("translate always = %v, want the read on every path refused by the evaluator's failure", err)
	}

	// A division by a free operand is an equation where the read is on every
	// path, its divisor guarded non-zero; where the read is conditional the
	// guard cannot be hoisted, so the read is guarded by its path instead.
	divides := symbolNamed(t, idx, "test::Item::divides")
	q, err = ConstraintWith(ctx, divides, divides.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate divides: %v", err)
	}
	if len(q.Derived) != 1 || q.Derived[0].Expression != "1.0 / free" || len(q.Unreadable) != 0 {
		t.Fatalf("Derived = %+v, Unreadable = %+v, want w = 1.0 / free", q.Derived, q.Unreadable)
	}
	var guarded bool
	for _, a := range q.Assertions {
		guarded = guarded || (a.From.Role == RoleDefined && strings.Contains(writeTerm(a.Term), "test::Item::free"))
	}
	if !guarded {
		t.Fatalf("no non-zero guard on free; script:\n%s", Script(q))
	}
	values := modelValues(t, solved(t, solver, q, StatusSat))
	if free := exactRat(t, values["test::Item::free"]); free.Sign() <= 0 {
		t.Errorf("free = %s, want positive for w = 1.0 / free > 0.0", free)
	}
	sometimes := symbolNamed(t, idx, "test::Item::sometimes")
	q, err = ConstraintWith(ctx, sometimes, sometimes.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate sometimes: %v", err)
	}
	if len(q.Derived) != 0 || len(q.Unreadable) != 1 || q.Unreadable[0].Name != "w" {
		t.Fatalf("Derived = %+v, Unreadable = %+v, want the conditional read of w guarded", q.Derived, q.Unreadable)
	}
	// An operand the evaluator always passes (`true and …`) narrows nothing, so
	// the read behind it is on every path and the equation is stated.
	tautology := symbolNamed(t, idx, "test::Item::tautology")
	q, err = ConstraintWith(ctx, tautology, tautology.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate tautology: %v", err)
	}
	if len(q.Derived) != 1 || len(q.Unreadable) != 0 || varNamed(t, q, "test::Item::w").Reached != nil {
		t.Fatalf("Derived = %+v, Unreadable = %+v, want w = 1.0 / free read on every path", q.Derived, q.Unreadable)
	}
	solved(t, solver, q, StatusSat)
}

// TestDeclarationOverAChainIsNotDerived: a declared value reading through a
// chain (`inner.x + 1.0`) is fixed to the value the evaluator derives from the
// held object, as before, since a query about a condition alone fixes no value
// of the held object for the chain to be read against.
func TestDeclarationOverAChainIsNotDerived(t *testing.T) {
	ctx, idx := fixture(t, "derived_chain.sysml", `package test {
		private import ScalarValues::*;
		part def Inner { attribute x : Real = 2.0; }
		part def Outer {
			part inner : Inner;
			attribute z : Real = inner.x + 1.0;
			assert constraint c { z == 5.0 }
		}
		part outer : Outer;
	}`)
	outer := symbolNamed(t, idx, "test::Outer")
	self, err := ctx.Instantiate(symbolNamed(t, idx, "test::outer"))
	if err != nil {
		t.Fatalf("instantiate outer: %v", err)
	}
	pins, unfixed := Fixed(ctx, outer, self)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	for _, p := range pins {
		if p.Name == "z" {
			if p.Derived != nil || p.Value.Kind != runtime.ValConst || p.Source != PinHeld {
				t.Fatalf("z = %+v, want the value derived from the held inner.x fixed", p)
			}
			c := symbolNamed(t, idx, "test::Outer::c")
			q, err := ConstraintWith(ctx, c, c.OwnerScope, pins)
			if err != nil || len(q.Derived) != 0 || len(q.Pinned) != 1 || q.Pinned[0].Value != "3" {
				t.Fatalf("translate c = %+v, %v, want z fixed to 3", q, err)
			}
			return
		}
	}
	t.Fatalf("pins = %+v, want z among them", pins)
}

// TestDerivedDetectionResolvesReferencesAsTheTranslatorDoes: a declared value
// is derived when a name of one segment reads the element's feature, as the
// translator resolves it; a qualified name reads a scoped element, which
// alone derives nothing, and a chain through an object is not derived.
func TestDerivedDetectionResolvesReferencesAsTheTranslatorDoes(t *testing.T) {
	ctx, idx := fixture(t, "derived_names.sysml", `package test {
		private import ScalarValues::*;
		attribute scale : Real = 2.0;
		part def Inner { attribute x : Real = 1.0; }
		part def Item {
			attribute r : Real;
			attribute scaled : Real = test::scale * r;
			attribute constant : Real = test::scale * 3.0;
			part inner : Inner;
			attribute nested : Real = inner.x * r;
			assert constraint c { scaled > 0.0 and constant > 0.0 }
		}
	}`)
	pins, _ := Fixed(ctx, symbolNamed(t, idx, "test::Item"), nil)
	byName := map[string]Pin{}
	for _, p := range pins {
		byName[p.Name] = p
	}
	if scaled := derivedPin(t, pins, "scaled"); scaled.Value.Kind != runtime.ValInvalid {
		t.Errorf("scaled = %+v, want derived with no value for want of r", scaled)
	}
	if constant := byName["constant"]; constant.Derived != nil || constant.Value.Kind != runtime.ValConst {
		t.Errorf("constant = %+v, want a value: it reads no feature of the element", constant)
	}
	if nested, ok := byName["nested"]; ok && nested.Derived != nil {
		t.Errorf("nested = %+v, want not derived: it reads through a chain", nested)
	}
	c := symbolNamed(t, idx, "test::Item::c")
	q, err := ConstraintWith(ctx, c, c.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate c: %v", err)
	}
	if len(q.Derived) != 1 || q.Derived[0].Expression != "test::scale * r" || len(q.Pinned) != 1 {
		t.Fatalf("Derived = %+v, Pinned = %+v, want scaled = test::scale * r and constant fixed", q.Derived, q.Pinned)
	}
}
