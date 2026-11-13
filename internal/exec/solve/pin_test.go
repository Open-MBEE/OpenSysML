package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// declaredPins is what the model declares for the features of fqn, fixed. Every
// value a declaration of the fixture states must be read; one the library
// states that cannot be read where nothing gives its operand a value
// (Items::Item::isSolid reads voids) is unfixed, and no condition reads it.
func declaredPins(t *testing.T, ctx *runtime.Context, idx *symbols.Index, fqn string) []Pin {
	t.Helper()
	pins, unfixed := Fixed(ctx, symbolNamed(t, idx, fqn), nil)
	for _, u := range unfixed {
		if u.Feature == nil || strings.HasPrefix(symbols.FQNOf(u.Feature), "test::") {
			t.Fatalf("declared values could not be read: %+v", unfixed)
		}
	}
	return pins
}

// pinnedQuery translates one constraint of the panel fixture with the values the
// model declares for the part fixed.
func fixedQuery(t *testing.T, element string) (*runtime.Context, *symbols.Index, *Query) {
	t.Helper()
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	pins := declaredPins(t, ctx, idx, "test::Panel")
	sym := symbolNamed(t, idx, element)
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate %s with fixed values: %v", element, err)
	}
	return ctx, idx, q
}

// TestGoldenWithFixedValues locks the script written for a query whose fixed
// values the model declares, translation and writer together.
func TestGoldenWithFixedValues(t *testing.T) {
	cases := map[string]string{
		"panel_fits_pinned.smt2":   "test::Panel::fits",
		"panel_speed_pinned.smt2":  "test::Panel::speedIsBounded",
		"panel_finish_pinned.smt2": "test::Panel::polishedIsWide",
	}
	for golden, element := range cases {
		t.Run(golden, func(t *testing.T) {
			_, _, q := fixedQuery(t, element)
			compareGolden(t, golden, Script(q))
		})
	}
}

// TestFixedReadsTheValuesTheModelDeclares: every attribute stating a value is
// read, and one stating none stays free rather than being fixed to nothing.
func TestFixedReadsTheValuesTheModelDeclares(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	panel := symbolNamed(t, idx, "test::Panel")
	pins, unfixed := Fixed(ctx, panel, nil)
	for _, u := range unfixed {
		// Items::Item::isSolid reads voids, which nothing gives a value to at the
		// declaration: an inherited value that cannot be read, and no condition reads.
		if u.Feature == nil || u.Feature.Name != "isSolid" || !errors.Is(u.Err, runtime.ErrNoValue) {
			t.Fatalf("declared values could not be read: %+v", unfixed)
		}
	}
	got := map[string]runtime.ValueKind{}
	for _, p := range pins {
		got[p.Name] = p.Value.Kind
		if p.Source != PinDeclared {
			t.Errorf("%s came from %s, want a declared value", p.Name, p.Source)
		}
	}
	want := map[string]runtime.ValueKind{
		"width":    runtime.ValConst,
		"finish":   runtime.ValEnumLiteral,
		"maxSpeed": runtime.ValQuantity,
		"label":    runtime.ValString,
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s was read as %v, want %v", name, got[name], kind)
		}
	}
	if _, fixed := got["height"]; fixed {
		t.Error("height states no value, so nothing fixes it")
	}
}

// TestFixedForKeepsAFeatureNamedLikeItsOwners: a constraint's own feature is fixed beside its owner's of the same name.
func TestFixedForKeepsAFeatureNamedLikeItsOwners(t *testing.T) {
	ctx, idx := fixtureFile(t, "shadowed_pins.sysml")
	owner := symbolNamed(t, idx, "test::h")
	element := symbolNamed(t, idx, "test::h::k")
	pins, unfixed := FixedFor(ctx, Fixing{Element: element, Owner: owner})
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	got := map[*symbols.Symbol]float64{}
	for _, p := range pins {
		got[p.Feature] = p.Value.Const.AsReal()
	}
	for _, want := range []struct {
		feature string
		value   float64
	}{{"test::h::x", 2.0}, {"test::h::w", 3.0}, {"test::h::k::x", 1.0}} {
		sym := symbolNamed(t, idx, want.feature)
		if value, ok := got[sym]; !ok || value != want.value {
			t.Errorf("%s pinned to %v (present %v), want %v", want.feature, value, ok, want.value)
		}
	}
}

// TestFixedForReadsAnObjectForItsOwnFeaturesOnly: the owner's object holds its
// x and w; the constraint's own x, another feature, is fixed as declared.
func TestFixedForReadsAnObjectForItsOwnFeaturesOnly(t *testing.T) {
	ctx, idx := fixtureFile(t, "shadowed_pins.sysml")
	owner := symbolNamed(t, idx, "test::h")
	element := symbolNamed(t, idx, "test::h::k")
	object, err := ctx.Instantiate(owner)
	if err != nil {
		t.Fatalf("instantiate h: %v", err)
	}
	pins, unfixed := FixedFor(ctx, Fixing{Element: element, Owner: owner, Object: object, ObjectType: owner})
	if len(unfixed) != 0 {
		t.Fatalf("values could not be read: %+v", unfixed)
	}
	got := map[*symbols.Symbol]Pin{}
	for _, p := range pins {
		got[p.Feature] = p
	}
	for _, want := range []struct {
		feature string
		value   float64
		source  PinSource
		bound   bool
	}{
		{"test::h::x", 2.0, PinHeld, false},
		{"test::h::w", 3.0, PinHeld, false},
		{"test::h::k::x", 1.0, PinDeclared, true},
	} {
		p, ok := got[symbolNamed(t, idx, want.feature)]
		if !ok {
			t.Errorf("%s not pinned", want.feature)
			continue
		}
		if value := p.Value.Const.AsReal(); value != want.value || p.Source != want.source || p.Bound != want.bound {
			t.Errorf("%s pinned to %v (%v, bound %v), want %v (%v, bound %v)",
				want.feature, value, p.Source, p.Bound, want.value, want.source, want.bound)
		}
	}
}

// TestFixedForFixesARedefinitionOverTheFeatureItRedefines: c6's `:>> a = 7.0` is
// the nearer declaration of a; c7's valueless `:>> b` reads b as the part holds or declares it.
func TestFixedForFixesARedefinitionOverTheFeatureItRedefines(t *testing.T) {
	ctx, idx := fixtureFile(t, "redefined_pins.sysml")
	owner := symbolNamed(t, idx, "test::p")
	type want struct {
		value  float64
		source PinSource
		bound  bool
	}
	check := func(t *testing.T, f Fixing, wants map[string]want, superseded ...string) {
		t.Helper()
		pins, unfixed := FixedFor(ctx, f)
		if len(unfixed) != 0 {
			t.Fatalf("values could not be read: %+v", unfixed)
		}
		got := map[*symbols.Symbol]Pin{}
		for _, p := range pins {
			got[p.Feature] = p
		}
		for feature, w := range wants {
			p, ok := got[symbolNamed(t, idx, feature)]
			if !ok {
				t.Errorf("%s not pinned", feature)
				continue
			}
			if value := p.Value.Const.AsReal(); value != w.value || p.Source != w.source || p.Bound != w.bound {
				t.Errorf("%s pinned to %v (%v, bound %v), want %v (%v, bound %v)",
					feature, value, p.Source, p.Bound, w.value, w.source, w.bound)
			}
		}
		for _, feature := range superseded {
			if p, ok := got[symbolNamed(t, idx, feature)]; ok {
				t.Errorf("%s pinned to %v (%v), want its redefinition pinned instead", feature, p.Value.Const.AsReal(), p.Source)
			}
		}
	}
	c6 := symbolNamed(t, idx, "test::p::c6")
	c7 := symbolNamed(t, idx, "test::p::c7")
	t.Run("declared", func(t *testing.T) {
		check(t, Fixing{Element: c6, Owner: owner}, map[string]want{
			"test::p::c6::a": {7.0, PinDeclared, true},
			"test::p::b":     {3.0, PinDeclared, true},
		}, "test::p::a")
		check(t, Fixing{Element: c7, Owner: owner}, map[string]want{
			"test::p::c7::b": {3.0, PinDeclared, true},
		}, "test::p::b")
	})
	t.Run("held", func(t *testing.T) {
		object, err := ctx.Instantiate(owner)
		if err != nil {
			t.Fatalf("instantiate p: %v", err)
		}
		check(t, Fixing{Element: c6, Owner: owner, Object: object, ObjectType: owner}, map[string]want{
			"test::p::c6::a": {7.0, PinDeclared, true},
			"test::p::b":     {3.0, PinHeld, false},
		}, "test::p::a")
		check(t, Fixing{Element: c7, Owner: owner, Object: object, ObjectType: owner}, map[string]want{
			"test::p::a":     {2.0, PinHeld, false},
			"test::p::c7::b": {3.0, PinHeld, false},
		}, "test::p::b")
	})
}

// TestFixedMarksAValueBoundWithEquals: a value written with `=` is Bound, derived,
// inherited or redefined over a default alike; one written `default` or `:=` is not.
func TestFixedMarksAValueBoundWithEquals(t *testing.T) {
	ctx, idx := fixtureFile(t, "bound_pins.sysml")
	pins, unfixed := Fixed(ctx, symbolNamed(t, idx, "test::Gauge"), nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	got := map[string]bool{}
	for _, p := range pins {
		got[p.Name] = p.Bound
	}
	want := map[string]bool{"bound": true, "inherited": true, "rebound": true, "fallback": false, "initial": false, "doubled": true}
	for name, bound := range want {
		pinned, ok := got[name]
		if !ok {
			t.Errorf("%s was not pinned", name)
		} else if pinned != bound {
			t.Errorf("%s Bound = %v, want %v", name, pinned, bound)
		}
	}
}

// TestNoPinsTranslateAsBefore: a translation given no fixed values writes the
// very script the plain translation writes, which is what keeps every existing
// golden and verdict standing.
func TestNoPinsTranslateAsBefore(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	for _, element := range []string{"test::Panel::fits", "test::Panel::speedIsBounded"} {
		sym := symbolNamed(t, idx, element)
		plain, err := Constraint(ctx, sym, sym.OwnerScope)
		if err != nil {
			t.Fatalf("translate %s: %v", element, err)
		}
		with, err := ConstraintWith(ctx, sym, sym.OwnerScope, nil)
		if err != nil {
			t.Fatalf("translate %s with no fixed values: %v", element, err)
		}
		if Script(plain) != Script(with) {
			t.Errorf("%s wrote a different script with no values fixed:\n%s\n%s", element, Script(plain), Script(with))
		}
		if with.Fixes() || len(with.Pinned) != 0 {
			t.Errorf("%s fixes %d values with none given", element, len(with.Pinned))
		}
	}
}

// TestPinnedValueIsAssertedAndReported: the value the model declares becomes an
// equality the query asserts, at the position the pin records, and the variable
// it fixes is no longer one the solver is free to choose.
func TestPinnedValueIsAssertedAndReported(t *testing.T) {
	_, _, q := fixedQuery(t, "test::Panel::fits")
	if len(q.Pinned) != 1 || q.Pinned[0].Var.Name != "test::Panel::width" {
		t.Fatalf("fixed %+v, want the declared width alone:\n%s", q.Pinned, Script(q))
	}
	pinned := q.Pinned[0]
	if pinned.Value != "4" || pinned.Source != PinDeclared {
		t.Errorf("width was fixed to %q from %s, want 4 declared", pinned.Value, pinned.Source)
	}
	if at := q.Assertions[pinned.Index]; at.From.Role != RolePinned {
		t.Errorf("assertion %d is %s, want the fixed value's own", pinned.Index, at.From.Role)
	}
	for _, v := range q.Free() {
		if v.Name == "test::Panel::width" {
			t.Error("width is fixed, so it is not left for the solver to choose")
		}
	}
	if !strings.Contains(Script(q), "(assert (= |test::Panel::width| 4))") {
		t.Errorf("the script does not fix the width:\n%s", Script(q))
	}
}

// TestPinnedQuantityIsScaledExactly: a value declared in km/h fixes the very
// magnitude the same quantity written in the condition scales to — 5.4 [km/h] is
// exactly 1.5 [m/s], with no rounding of the binary float it was read as.
func TestPinnedQuantityIsScaledExactly(t *testing.T) {
	_, _, q := fixedQuery(t, "test::Panel::speedIsBounded")
	script := Script(q)
	if !strings.Contains(script, "(assert (= |test::Panel::maxSpeed| 1.5))") {
		t.Errorf("the fixed speed is not exactly 1.5 in base units:\n%s", script)
	}
	if len(q.Pinned) != 1 || q.Pinned[0].Value != "5.4 [km/h]" {
		t.Errorf("fixed %+v, want the speed as it was declared", q.Pinned)
	}
}

// TestPinnedEnumerationNamesItsConstructor: an enumeration literal fixes the
// value of the datatype sort the writer declares for it.
func TestPinnedEnumerationNamesItsConstructor(t *testing.T) {
	_, _, q := fixedQuery(t, "test::Panel::polishedIsWide")
	script := Script(q)
	if !strings.Contains(script, "(assert (= |test::Panel::finish| |test::Finish::polished|))") {
		t.Errorf("the fixed finish is not the datatype's own value:\n%s", script)
	}
}

// TestPinnedStringIsAsserted: a declared string fixes the string the writer
// quotes.
func TestPinnedStringIsAsserted(t *testing.T) {
	_, _, q := fixedQuery(t, "test::Panel::labelled")
	if !strings.Contains(Script(q), `(assert (= |test::Panel::label| "left"))`) {
		t.Errorf("the fixed label is not asserted:\n%s", Script(q))
	}
}

// TestPinnedValueNoConditionReadsIsReported: a value fixed for a feature no
// condition of the element reads is reported as unread rather than dropped, and
// asserts nothing about conditions that say nothing about it.
func TestPinnedValueNoConditionReadsIsReported(t *testing.T) {
	_, _, q := fixedQuery(t, "test::Panel::labelled")
	unread := map[string]bool{}
	for _, u := range q.Unread {
		unread[u.Pin.Name] = true
		if u.Reason == "" {
			t.Errorf("%s is unread for no stated reason", u.Pin.Name)
		}
	}
	for _, name := range []string{"width", "finish", "maxSpeed"} {
		if !unread[name] {
			t.Errorf("%s is not read by this constraint, so it should be reported unread: %+v", name, q.Unread)
		}
	}
}

// TestFixedValuesAreOrderedByTheirVariable: the fixed values a query reports are
// in the order of the assertions fixing them, so the position each records is the
// assertion an unsat core names.
func TestFixedValuesAreOrderedByTheirVariable(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	panel := symbolNamed(t, idx, "test::Panel")
	pins, _ := Fixed(ctx, panel, nil)
	sym := symbolNamed(t, idx, "test::Panel::polishedIsWide")
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate with fixed values: %v", err)
	}
	if len(q.Pinned) != 2 {
		t.Fatalf("fixed %+v, want the finish and the width", q.Pinned)
	}
	previous := ""
	for _, p := range q.Pinned {
		if p.Var.Name < previous {
			t.Errorf("fixed values are out of order at %s", p.Var.Name)
		}
		previous = p.Var.Name
		at := q.Assertions[p.Index]
		if at.From.Role != RolePinned || at.From.Element != p.Var.Name {
			t.Errorf("assertion %d is %s of %s, want the value fixed for %s",
				p.Index, at.From.Role, at.From.Element, p.Var.Name)
		}
	}
}

// TestPinRefusesAnIncommensurableQuantity: a value measured in another dimension
// fixes nothing, since no scaling reconciles it; the refusal is typed and names
// the feature.
func TestPinRefusesAnIncommensurableQuantity(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	length := quantityValue(t, ctx, idx, "test::Panel::clearance")
	sym := symbolNamed(t, idx, "test::Panel::speedIsBounded")
	speed := symbolNamed(t, idx, "test::Panel::maxSpeed")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: speed, Name: "maxSpeed", Value: length, Source: PinHeld, Object: 7,
	}})
	var refusal *PinError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a typed refusal to fix the value", err)
	}
	if !strings.Contains(refusal.Feature, "maxSpeed") || refusal.Reason == "" {
		t.Errorf("refusal %+v does not name the feature and why", refusal)
	}
}

// TestPinRefusesAValueWithNoLiteral: a value the term language cannot write is a
// refusal, never a query quietly missing what it was asked to fix.
func TestPinRefusesAValueWithNoLiteral(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: runtime.Value{Kind: runtime.ValSequence}, Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a value with no literal", err)
	}
}

// TestPinRefusesATensorQuantity: the term language has scalar variables only, so
// a tensor is refused by name rather than flattened or silently dropped.
func TestPinRefusesATensorQuantity(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	metre := quantityValue(t, ctx, idx, "test::Panel::clearance").Quantity().Unit
	num := []semantics.Value{{Kind: semantics.ValReal, Real: 1}, {Kind: semantics.ValReal, Real: 0}, {Kind: semantics.ValReal, Real: 0}, {Kind: semantics.ValReal, Real: 1}}
	tensor := runtime.NewTensorQuantityValue([]int64{2, 2}, num, []runtime.Unit{metre, metre, metre, metre})
	sym := symbolNamed(t, idx, "test::Panel::speedIsBounded")
	speed := symbolNamed(t, idx, "test::Panel::maxSpeed")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: speed, Name: "maxSpeed", Value: tensor, Source: PinChosen,
	}})
	var refusal *PinError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a typed refusal to fix a tensor", err)
	}
	if !strings.Contains(refusal.Reason, "is not a scalar") || !strings.Contains(refusal.Value, "Tensor(2, 2)") {
		t.Errorf("refusal %+v does not name the tensor and why", refusal)
	}
}

// TestPinRefusesAValueOfTheWrongType: a string does not fix a number, whatever
// the model states.
func TestPinRefusesAValueOfTheWrongType(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: runtime.NewStringValue("wide"), Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a number to a string", err)
	}
}

// TestPinnedBareNumberRefusesAMeasuredFeature: a plain number states no unit, so
// it cannot fix a feature whose values are magnitudes of a dimension.
func TestPinnedBareNumberRefusesAMeasuredFeature(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::speedIsBounded")
	speed := symbolNamed(t, idx, "test::Panel::maxSpeed")
	bare := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: speed, Name: "maxSpeed", Value: bare, Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a magnitude to a bare number", err)
	}
}

// quantityValue evaluates the declared value of a feature of the fixture, which
// is how a test gets a runtime quantity without building one by hand.
func quantityValue(t *testing.T, ctx *runtime.Context, idx *symbols.Index, fqn string) runtime.Value {
	t.Helper()
	for _, p := range declaredPins(t, ctx, idx, "test::Panel") {
		if p.Feature != nil && strings.HasSuffix(fqn, "::"+p.Name) {
			return p.Value
		}
	}
	t.Fatalf("%s declares no value to read", fqn)
	return runtime.Value{}
}

// TestPinRefusesAMeasurementReference: a unit is not a number, so the reference
// its quantity was measured in fixes nothing; the refusal is typed and names it.
func TestPinRefusesAMeasurementReference(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	length := quantityValue(t, ctx, idx, "test::Panel::clearance")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: runtime.NewMeasurementRefValue(length.Quantity().Unit), Source: PinChosen,
	}})
	var refusal *PinError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a typed refusal to fix a measurement reference", err)
	}
	if !strings.Contains(refusal.Reason, "is a measurement reference") {
		t.Errorf("refusal %+v does not say the value is a measurement reference", refusal)
	}
}

// TestPinRefusesACoordinateFrame: a frame and the transformation placing it are
// not numbers either; each refusal is typed and says what the value is.
func TestPinRefusesACoordinateFrame(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	length := quantityValue(t, ctx, idx, "test::Panel::clearance")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	frame := &runtime.CoordinateFrame{Dimensions: []int64{2}, Axes: []runtime.Unit{length.Quantity().Unit, length.Quantity().Unit}, Text: "panelCF"}
	for _, tc := range []struct {
		value runtime.Value
		what  string
	}{
		{runtime.NewCoordinateFrameValue(frame), "is a coordinate frame"},
		{runtime.NewCoordinateTransformationValue(&runtime.CoordinateTransformation{Source: frame, Target: frame}), "is a coordinate transformation"},
	} {
		_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{Feature: width, Name: "width", Value: tc.value, Source: PinChosen}})
		var refusal *PinError
		if !errors.As(err, &refusal) || !errors.Is(err, ErrNotPinnable) {
			t.Fatalf("error %v, want a typed refusal to fix %s", err, runtime.FormatValue(tc.value))
		}
		if !strings.Contains(refusal.Reason, tc.what) {
			t.Errorf("refusal %+v does not say the value %s", refusal, tc.what)
		}
	}
}

// TestPinnedScalarValuedLiteralIsItsIdentity: a literal of an enumeration that
// specializes Integer fixes a datatype variable by the literal it is, while the
// scalar it equals still fixes an Integer feature holding it.
func TestPinnedScalarValuedLiteralIsItsIdentity(t *testing.T) {
	ctx, idx := fixture(t, "levels.sysml", `
package test {
	private import ScalarValues::*;
	enum def Level :> Integer { low = 1; high = 3; }
	part def Rover {
		attribute level : Level = Level::high;
		attribute cast : Level[0..1] = 3 as Level;
		attribute n : Integer = Level::high;
		assert constraint highIsHigh { level == Level::high and cast == Level::high and n >= 3 }
	}
}`)
	pins := declaredPins(t, ctx, idx, "test::Rover")
	sym := symbolNamed(t, idx, "test::Rover::highIsHigh")
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate with fixed values: %v", err)
	}
	script := Script(q)
	for _, want := range []string{
		"(assert (= |test::Rover::level| |test::Level::high|))",
		"(assert (= |test::Rover::cast| |test::Level::high|))",
		"(assert (= |test::Rover::n| 3))",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the script lacks %s:\n%s", want, script)
		}
	}
	for _, p := range q.Pinned {
		if p.Var.Name != "test::Rover::n" && p.Value != "Level::high" {
			t.Errorf("%s was fixed to %q, want the literal Level::high", p.Var.Name, p.Value)
		}
	}
}

// TestChainPinsReadTheValuesAChainNames: a feature a chain of steps reaches
// gets the value the evaluator reads there, fixed to the variable the chain
// names — and a chain reading no value stays free.
func TestChainPinsReadTheValuesAChainNames(t *testing.T) {
	ctx, idx := fixture(t, "chain_pins.sysml", `package test {
		private import ScalarValues::*;
		part hg { attribute power : Real = 5.0; attribute free : Real; }
		assert constraint positive { hg.power > 0.0 }
		assert constraint bounded { hg.free < 1.0 }
	}`)
	sym := symbolNamed(t, idx, "test::positive")
	q, err := ConstraintViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	pins, unfixed := ChainPins(ctx, q, ObjectReader(ctx, nil), nil)
	if len(unfixed) != 0 {
		t.Fatalf("ChainPins unfixed = %+v, want none", unfixed)
	}
	if len(pins) != 1 || !strings.HasSuffix(pins[0].Var, "hg.power") {
		t.Fatalf("chain pins = %+v, want one pinning hg.power", pins)
	}
	sym = symbolNamed(t, idx, "test::bounded")
	q, err = ConstraintViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate bounded: %v", err)
	}
	pins, unfixed = ChainPins(ctx, q, ObjectReader(ctx, nil), nil)
	if len(pins) != 0 || len(unfixed) != 0 {
		t.Fatalf("bounded chain pins = %+v unfixed = %+v, want none — hg.free carries no value", pins, unfixed)
	}
}

func TestUnfixedReadRefusesOnlyReadFailures(t *testing.T) {
	ctx, idx := fixture(t, "unfixed_read.sysml", `package test {
		private import ScalarValues::*;
		part def Item {
			attribute broken : Real = 1.0 / 0.0;
			attribute free : Real;
			attribute input : Real;
			attribute dependent : Real = input + 1.0;
			attribute opaque : Real = input ** 2.0;
			assert constraint readsBroken { broken >= 0.0 }
			assert constraint readsFree { free >= 0.0 }
			assert constraint readsDependent { dependent > 0.0 }
			assert constraint readsOpaque { opaque > 0.0 }
		}
		part item : Item;
	}`)
	owner := symbolNamed(t, idx, "test::Item")
	broken := symbolNamed(t, idx, "test::Item::readsBroken")
	item, err := ctx.Instantiate(symbolNamed(t, idx, "test::item"))
	if err != nil {
		t.Fatalf("instantiate item: %v", err)
	}
	pins, unfixed := FixedFor(ctx, Fixing{Element: broken, Owner: owner, Object: item, ObjectType: owner})
	unfixedByName := func(name string) (Unfixed, bool) {
		for _, u := range unfixed {
			if u.Name == name {
				return u, true
			}
		}
		return Unfixed{}, false
	}
	brokenUnfixed, ok := unfixedByName("broken")
	if !ok || brokenUnfixed.Err == nil {
		t.Fatalf("unfixed = %+v, want failed read of broken", unfixed)
	}
	if brokenUnfixed.Reason != brokenUnfixed.Err.Error() {
		t.Fatalf("Reason = %q, Err = %q", brokenUnfixed.Reason, brokenUnfixed.Err)
	}
	q, err := ConstraintWith(ctx, broken, broken.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate readsBroken: %v", err)
	}
	readErr := UnfixedRead(ctx, q, unfixed)
	if readErr == nil || !errors.Is(readErr, brokenUnfixed.Err) || !strings.Contains(readErr.Error(), "broken") {
		t.Fatalf("UnfixedRead readsBroken = %v, want wrapped read error naming broken", readErr)
	}

	irrelevant := symbolNamed(t, idx, "test::Item::readsFree")
	freeQuery, err := ConstraintWith(ctx, irrelevant, irrelevant.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate readsFree: %v", err)
	}
	if err := UnfixedRead(ctx, freeQuery, unfixed); err != nil {
		t.Fatalf("UnfixedRead readsFree = %v, want nil for unread broken feature", err)
	}
	if _, ok := unfixedByName("free"); ok {
		t.Fatalf("unfixed = %+v, want free omitted", unfixed)
	}
	noValue := []Unfixed{{
		Feature: brokenUnfixed.Feature,
		Name:    brokenUnfixed.Name,
		Err:     errors.Join(errors.New("wrapped"), runtime.ErrNoValue),
	}}
	if err := UnfixedRead(ctx, q, noValue); err == nil || !errors.Is(err, runtime.ErrNoValue) {
		t.Fatalf("UnfixedRead wrapped missing dependency = %v, want error wrapping ErrNoValue", err)
	}

	// A derived value whose operand has no value is a defining equation over the
	// free operand, not a failed read; one whose equation the term language does
	// not carry, and that the evaluator could not derive either, is refused as
	// a failed read where the read is unconditional.
	if _, ok := unfixedByName("dependent"); ok {
		t.Fatalf("unfixed = %+v, want dependent defined by its equation rather than unfixed", unfixed)
	}
	dependent := symbolNamed(t, idx, "test::Item::readsDependent")
	dependentQuery, err := ConstraintWith(ctx, dependent, dependent.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate readsDependent: %v", err)
	}
	if err := UnfixedRead(ctx, dependentQuery, unfixed); err != nil {
		t.Fatalf("UnfixedRead readsDependent = %v, want nil: dependent is defined by its equation", err)
	}
	if len(dependentQuery.Derived) != 1 || dependentQuery.Derived[0].Expression != "input + 1.0" {
		t.Fatalf("Derived = %+v, want dependent = input + 1.0", dependentQuery.Derived)
	}
	if _, ok := unfixedByName("opaque"); ok {
		t.Fatalf("unfixed = %+v, want opaque carried as a derived value whose read failed", unfixed)
	}
	opaque := symbolNamed(t, idx, "test::Item::readsOpaque")
	if _, err := ConstraintWith(ctx, opaque, opaque.OwnerScope, pins); err == nil ||
		!errors.Is(err, runtime.ErrNoValue) || !strings.Contains(err.Error(), "reads opaque, whose value could not be read") {
		t.Fatalf("translate readsOpaque = %v, want the unconditional read of opaque refused for want of input", err)
	}
}

func TestSatisfactionChainPinsReadTheSubject(t *testing.T) {
	ctx, idx := fixture(t, "satisfaction_chain_pins.sysml", `package test {
		private import ScalarValues::*;
		part def Inner { attribute power : Real; }
		part def Sub { attribute power : Real; part inner : Inner; }
		part def Thing { part sub : Sub; }
		requirement def R {
			subject vehicle : Thing;
			require constraint {
				vehicle.sub.power > 0.0 and vehicle.sub.inner.power > 0.0
			}
		}
		requirement r : R;
		part craft : Thing {
			part :>> sub {
				attribute :>> power = 5.0;
				part :>> inner { attribute :>> power = 7.0; }
			}
		}
		part analysis { assert satisfy r by craft; }
	}`)
	var assertion *runtime.SatisfyAssertion
	for _, a := range ctx.SatisfyAssertionsIn(idx.DocumentRoot("satisfaction_chain_pins.sysml")) {
		if a.Text() == "satisfy r by craft" {
			assertion = a
			break
		}
	}
	if assertion == nil {
		t.Fatal("satisfaction assertion not found")
	}
	subject, err := ctx.SatisfySubject(assertion)
	if err != nil {
		t.Fatalf("SatisfySubject: %v", err)
	}
	q, err := Satisfaction(ctx, assertion)
	if err != nil {
		t.Fatalf("translate satisfaction: %v", err)
	}
	pins, unfixed := ChainPins(ctx, q, SatisfactionReader(ctx, assertion, subject), subject)
	if len(unfixed) != 0 {
		t.Fatalf("ChainPins with satisfaction reader: unfixed %+v, want none", unfixed)
	}
	want := map[string]float64{"vehicle.sub.power": 5.0, "vehicle.sub.inner.power": 7.0}
	for _, pin := range pins {
		for suffix, value := range want {
			if strings.HasSuffix(pin.Name, suffix) {
				if pin.Value.Kind != runtime.ValConst || pin.Value.Const.Real != value {
					t.Errorf("%s = %+v, want %v", pin.Name, pin.Value, value)
				}
				delete(want, suffix)
			}
		}
	}
	if len(want) != 0 {
		t.Errorf("satisfaction chain pins %+v do not include %v", pins, want)
	}
	objectPins, objectUnfixed := ChainPins(ctx, q, ObjectReader(ctx, nil), nil)
	if len(objectPins) != 0 || len(objectUnfixed) != 0 {
		t.Errorf("object-only reader pinned %+v and found unreadable %+v, want neither unbound chain", objectPins, objectUnfixed)
	}
}
