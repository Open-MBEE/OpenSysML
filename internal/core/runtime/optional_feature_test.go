package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// An optional composite feature fills to its lower bound like a collection:
// `part spare : Wheel[0..1]` holds no object of its own, reads as empty, and
// holds one only through a feature that subsets it.
func TestOptionalScalarFeatureHoldsOnlyContributions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part spare : Wheel[0..1];
			part wheels : Wheel[0..*];
			part trailer : Wheel[0..1];
			part hitch : Wheel[1] :> trailer;
		}
	}`))
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"spare", "wheels"} {
		fv, err := car.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("car.%s: %v", name, err)
		}
		if held := fv.HeldValue(); held.Kind == ValInstance || elementCount(&held) != 0 {
			t.Errorf("car.%s = %s, want no object", name, FormatValue(held))
		}
		read, err := fv.ReadValue(name)
		if err != nil || elementCount(&read) != 0 {
			t.Errorf("car.%s reads %s, %v; want the empty sequence", name, FormatValue(read), err)
		}
	}
	trailer, err := car.GetFeatureValue(ctx, "trailer")
	if err != nil {
		t.Fatalf("car.trailer: %v", err)
	}
	hitch := objectAt(t, ctx, car, "hitch")
	if held := trailer.HeldValue(); held.Kind != ValInstance || held.Instance != hitch.ID {
		t.Errorf("car.trailer = %s, want the hitch that subsets it (%d)", FormatValue(held), hitch.ID)
	}
	if n := len(ctx.instances); n != 2 {
		t.Errorf("instantiating Car made %d objects, want the car and its hitch", n)
	}
}

// A required abstract feature holds only what subsets it, so one nothing subsets
// violates its lower bound rather than holding an empty value; enough
// contributions satisfy it, scalar or collection alike.
func TestRequiredAbstractFeatureDemandsContributions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Bare {
			abstract part axle : Wheel[1];
			abstract part wheels : Wheel[2..*];
		}
		part def Fitted {
			abstract part axle : Wheel[1];
			part front : Wheel[1] :> axle;
			abstract part wheels : Wheel[2..*];
			part left : Wheel[1] :> wheels;
			part right : Wheel[1] :> wheels;
		}
	}`))
	bare, err := ctx.Instantiate(oneSymbol(t, idx, "test::Bare"))
	if err != nil {
		t.Fatalf("instantiate Bare: %v", err)
	}
	for _, name := range []string{"axle", "wheels"} {
		fv, err := bare.GetFeatureValue(ctx, name)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("bare.%s = %v, %v; want %v", name, fv, err, ErrMultiplicityViolation)
		}
	}
	fitted, err := ctx.Instantiate(oneSymbol(t, idx, "test::Fitted"))
	if err != nil {
		t.Fatalf("instantiate Fitted: %v", err)
	}
	axle, err := fitted.GetFeatureValue(ctx, "axle")
	if err != nil {
		t.Fatalf("fitted.axle: %v", err)
	}
	if held, front := axle.HeldValue(), objectAt(t, ctx, fitted, "front"); held.Kind != ValInstance || held.Instance != front.ID {
		t.Errorf("fitted.axle = %s, want the front wheel (%d)", FormatValue(held), front.ID)
	}
	wheels, err := fitted.GetFeatureValue(ctx, "wheels")
	if err != nil {
		t.Fatalf("fitted.wheels: %v", err)
	}
	if held := wheels.HeldValue(); elementCount(&held) != 2 {
		t.Errorf("fitted.wheels = %s, want the two wheels subsetting it", FormatValue(held))
	}
}

// An abstract feature declaring no multiplicity is bound by what it subsets, as
// `abstract action decisions :> controls` is by `controls[0..*]`; a concrete one stays 1..1.
func TestAbstractFeatureInheritsMultiplicityFromWhatItSubsets(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part wheels : Wheel[0..*];
			abstract part spares :> wheels;
			part one : Wheel :> wheels;
		}
		part def Bare {
			abstract part fitted : Wheel[1..*];
			abstract part needed :> fitted;
		}
	}`))
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	spares, err := car.GetFeatureValue(ctx, "spares")
	if err != nil {
		t.Fatalf("car.spares: %v", err)
	}
	if held := spares.HeldValue(); spares.Feature.Scalar() || elementCount(&held) != 0 {
		t.Errorf("car.spares = %s (scalar %t), want an empty collection", FormatValue(held), spares.Feature.Scalar())
	}
	one := objectAt(t, ctx, car, "one")
	wheels, err := car.GetFeatureValue(ctx, "wheels")
	if err != nil {
		t.Fatalf("car.wheels: %v", err)
	}
	if held := wheels.HeldValue(); elementCount(&held) != 1 || held.Sequence().Elements()[0].Instance != one.ID {
		t.Errorf("car.wheels = %s, want the one wheel (%d)", FormatValue(held), one.ID)
	}
	bare, err := ctx.Instantiate(oneSymbol(t, idx, "test::Bare"))
	if err != nil {
		t.Fatalf("instantiate Bare: %v", err)
	}
	if _, err := bare.GetFeatureValue(ctx, "needed"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("bare.needed: err = %v, want %v through fitted's [1..*]", err, ErrMultiplicityViolation)
	}
}

// A valueless optional declaration evaluates to the empty sequence whether it is
// named bare or qualified; a required one keeps its no-value error both ways.
func TestValuelessOptionalDeclarationEvaluatesEmptyHoweverSpelled(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def P { attribute tags : String[0..*]; attribute mass : Real; }
		attribute names : String[0..*];
		attribute weight : Real;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, src := range []string{"names", "test::names", "P::tags", "test::P::tags"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{"weight", "test::weight", "P::mass", "test::P::mass"} {
		if val, err := evalIn(t, ctx, pkg.Scope, src); err == nil {
			t.Errorf("%s = %s, want a no-value error", src, FormatValue(val))
		}
	}
}

// A declaration bound to an optional multiplicity by what it redefines or, if
// abstract, subsets evaluates as it does on an object: empty, bare or qualified.
func TestInheritedOptionalDeclarationEvaluatesEmpty(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part wheels : Wheel[0..*];
			abstract part spares :> wheels;
			part fitted : Wheel[1..*];
			abstract part needed :> fitted;
		}
		part def Van :> Car {
			part :>> wheels;
		}
		part wheels : Wheel[0..*];
		abstract part spares :> wheels;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	before := len(ctx.instances)
	for _, src := range []string{"spares", "test::spares", "Car::spares", "test::Car::spares", "Van::wheels"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "spares istype Wheel"); err != nil || val.Kind != ValConst || !val.Const.Bool {
		t.Errorf("spares istype Wheel = %s, %v; want true of nothing", FormatValue(val), err)
	}
	if made := len(ctx.instances) - before; made != 0 {
		t.Errorf("reading the inherited optional declarations made %d object(s), want none", made)
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "Car::needed"); err == nil {
		t.Errorf("Car::needed = %s, want an error through fitted's [1..*]", FormatValue(val))
	}
}

// Branches of inheritance converging on one ancestor each carry its multiplicity:
// `both :> front, rear` over `front, rear :> wheels[0..*]` is [0..*], not 1..1.
func TestDiamondInheritanceKeepsAncestorMultiplicity(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part wheels : Wheel[0..*];
			abstract part front :> wheels;
			abstract part rear :> wheels;
			abstract part both :> front, rear;
		}
	}`))
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	both, err := car.GetFeatureValue(ctx, "both")
	if err != nil {
		t.Fatalf("car.both: %v", err)
	}
	if mult := both.Feature.Multiplicity; mult.Lower.Value != 0 || !mult.Upper.Infinite {
		t.Errorf("car.both multiplicity = %+v, want [0..*] through both branches", mult)
	}
	if held := both.HeldValue(); elementCount(&held) != 0 {
		t.Errorf("car.both = %s, want an empty collection", FormatValue(held))
	}
}

// A subject states its multiplicity like any usage: `subject items : Item[0..*]`
// reads as empty when nothing binds it and holds every value a binding gives it.
func TestSubjectDeclaresItsMultiplicity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		item def Item;
		item a : Item;
		item b : Item;
		requirement def Unbound { subject items : Item[0..*]; }
		requirement def Bound { subject items : Item[0..*] = (a, b); }
	}`))
	unbound, err := ctx.Instantiate(oneSymbol(t, idx, "test::Unbound"))
	if err != nil {
		t.Fatalf("instantiate Unbound: %v", err)
	}
	items, err := unbound.GetFeatureValue(ctx, "items")
	if err != nil {
		t.Fatalf("unbound.items: %v", err)
	}
	if mult := items.Feature.Multiplicity; mult.Lower.Value != 0 || !mult.Upper.Infinite {
		t.Errorf("unbound.items multiplicity = %+v, want the declared [0..*]", mult)
	}
	if read, err := items.ReadValue("items"); err != nil || elementCount(&read) != 0 {
		t.Errorf("unbound.items reads %s, %v; want the empty sequence", FormatValue(read), err)
	}
	bound, err := ctx.Instantiate(oneSymbol(t, idx, "test::Bound"))
	if err != nil {
		t.Fatalf("instantiate Bound: %v", err)
	}
	items, err = bound.GetFeatureValue(ctx, "items")
	if err != nil {
		t.Fatalf("bound.items: %v", err)
	}
	if held := items.HeldValue(); elementCount(&held) != 2 {
		t.Errorf("bound.items = %s, want both bound items", FormatValue(held))
	}
}

// A subsetting that resolves to a feature outside the object names no feature of
// it, even when a derived type declares an unrelated feature under that name;
// only a declaration masking the inherited target is named in its place.
func TestSubsettingOutsideTheObjectNamesNoSameNamedMember(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Thing;
		package Ext { abstract part pool : Thing[0..*]; }
		part def Base {
			private import Ext::*;
			part picked : Thing :> pool;
			abstract part tags : Thing[0..*];
			part tagged : Thing :> tags;
		}
		part def Derived :> Base {
			abstract part pool : Thing[0..*];
			abstract part :>> tags;
		}
	}`))
	derived, err := ctx.Instantiate(oneSymbol(t, idx, "test::Derived"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	pool, err := derived.GetFeatureValue(ctx, "pool")
	if err != nil {
		t.Fatalf("derived.pool: %v", err)
	}
	if held := pool.HeldValue(); elementCount(&held) != 0 {
		t.Errorf("derived.pool = %s, want nothing: picked subsets Ext::pool, not this pool", FormatValue(held))
	}
	tags, err := derived.GetFeatureValue(ctx, "tags")
	if err != nil {
		t.Fatalf("derived.tags: %v", err)
	}
	if held := tags.HeldValue(); elementCount(&held) != 1 {
		t.Errorf("derived.tags = %s, want the one tagged part through the masking redefinition", FormatValue(held))
	}
}

// A subsetting keeps its resolved target when another inherited branch carries
// an unrelated feature under the same name: only a declaration of the owner's
// own masks the target, so the multiplicity it lends stays that of the target.
func TestSubsettingKeepsTargetAcrossInheritedNameCollision(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Thing;
		part def Left {
			abstract part <s> slots : Thing[1..*];
			abstract part needed :> s;
		}
		part def Right { abstract part slots : Thing[0..*]; }
		part def Both :> Right, Left;
		part def Masking :> Right, Left { abstract part slots : Thing[0..*] :>> Left::slots, Right::slots; }
		part def ShortMasking :> Right, Left { abstract part s : Thing[0..*]; }
	}`))
	both := oneSymbol(t, idx, "test::Both")
	needed := oneSymbol(t, idx, "test::Left::needed")
	leftSlots := oneSymbol(t, idx, "test::Left::slots")
	if related := ctx.relatedFeatures(needed, both, ast.RelSubsets); len(related) != 1 || related[0] != leftSlots {
		t.Errorf("Both: needed subsets %s, want Left::slots alone", symbolNames(ctx, related))
	}
	inst, err := ctx.Instantiate(both)
	if err != nil {
		t.Fatalf("instantiate Both: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "needed"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("both.needed: err = %v, want %v through Left::slots' [1..*]", err, ErrMultiplicityViolation)
	}
	masking := oneSymbol(t, idx, "test::Masking")
	if related := ctx.relatedFeatures(needed, masking, ast.RelSubsets); len(related) != 1 || related[0] != oneSymbol(t, idx, "test::Masking::slots") {
		t.Errorf("Masking: needed subsets %s, want the owner's redefining slots", symbolNames(ctx, related))
	}
	shortMasking := oneSymbol(t, idx, "test::ShortMasking")
	if related := ctx.relatedFeatures(needed, shortMasking, ast.RelSubsets); len(related) != 1 || related[0] != oneSymbol(t, idx, "test::ShortMasking::s") {
		t.Errorf("ShortMasking: needed subsets %s, want the owner's s masking the short name", symbolNames(ctx, related))
	}
	inst, err = ctx.Instantiate(shortMasking)
	if err != nil {
		t.Fatalf("instantiate ShortMasking: %v", err)
	}
	if fv, err := inst.GetFeatureValue(ctx, "needed"); err != nil {
		t.Errorf("shortMasking.needed: %v, want the empty collection through s' [0..*]", err)
	} else if held := fv.HeldValue(); fv.Feature.Scalar() || elementCount(&held) != 0 {
		t.Errorf("shortMasking.needed = %s (scalar %t), want an empty collection", FormatValue(held), fv.Feature.Scalar())
	}
}

// A bare restatement (`:>> slots`) masks nothing on the redefinition walk that
// starts from it, so the multiplicity of the chain's root is reached.
func TestRedefinitionChainReachesPastOwnRestatement(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Base { attribute slots : Integer[0..*]; }
		part def Middle :> Base { attribute :>> slots; }
		part def Leaf :> Middle { attribute :>> slots; }
		part deep : Leaf { :>> slots = (1, 2, 3); }
	}`))
	deep := oneSymbol(t, idx, "test::deep")
	slots, ok := ctx.model.semantics.LookupMember(deep, "slots")
	if !ok {
		t.Fatal("deep declares no slots")
	}
	if mult := ctx.featureMultiplicity(slots, deep); mult.Text() != "[0..*]" {
		t.Errorf("deep.slots multiplicity = %s, want [0..*] from Base::slots", mult.Text())
	}
	inst, err := ctx.Instantiate(deep)
	if err != nil {
		t.Fatalf("instantiate deep: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "slots")
	if err != nil {
		t.Fatalf("deep.slots: %v", err)
	}
	if held := fv.HeldValue(); elementCount(&held) != 3 {
		t.Errorf("deep.slots = %s, want the three values", FormatValue(held))
	}
}

func symbolNames(ctx *Context, syms []*symbols.Symbol) []string {
	names := make([]string, 0, len(syms))
	for _, sym := range syms {
		names = append(names, ctx.qualifiedSymbolName(sym))
	}
	return names
}

// An unbound optional subject evaluated by name, bare or qualified, reads as
// empty like any valueless declaration with lower bound zero; a required or a
// bound subject keeps its behaviour.
func TestOptionalSubjectDeclarationEvaluatesEmpty(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		item def Item;
		item a : Item;
		requirement def Unbound { subject items : Item[0..*]; }
		requirement def Required { subject it : Item; }
		requirement def Bound { subject it : Item = a; }
	}`))
	pkg := oneSymbol(t, idx, "test").Scope
	for _, src := range []string{"items", "Unbound::items", "test::Unbound::items"} {
		scope := pkg
		if src == "items" {
			scope = oneSymbol(t, idx, "test::Unbound").Scope
		}
		val, err := evalIn(t, ctx, scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("eval %s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg, "Required::it"); err == nil {
		t.Errorf("eval Required::it = %s, want an error for an unbound required subject", FormatValue(val))
	}
	if val, err := evalIn(t, ctx, pkg, "Bound::it"); err != nil || val.Kind != ValInstance {
		t.Errorf("eval Bound::it = %s, %v; want the bound item", FormatValue(val), err)
	}
}

// A feature whose lower bound is infinite admits no count, so holding nothing
// does not read as empty.
func TestInfiniteLowerBoundDoesNotReadAsEmpty(t *testing.T) {
	fv := &FeatureValue{Feature: &EffectiveFeature{Name: "p", Multiplicity: semantics.Range{
		Lower: semantics.Bound{Known: true, Infinite: true},
		Upper: semantics.Bound{Known: true, Infinite: true},
	}}}
	if read, err := fv.ReadValue("p"); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("reading unset [*..*] p: %s, %v; want %v", FormatValue(read), err, ErrUninitializedFeatureValue)
	}
}

// A required feature holding nothing is still uninitialized when read.
func TestRequiredUnsetFeatureReadsAsUninitialized(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def P { attribute mass : Real; attribute tags : String[0..*]; }
	}`))
	p, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	mass, err := p.GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("p.mass: %v", err)
	}
	if _, err := mass.ReadValue("mass"); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("reading unset mass: err = %v, want %v", err, ErrUninitializedFeatureValue)
	}
	tags, err := p.GetFeatureValue(ctx, "tags")
	if err != nil {
		t.Fatalf("p.tags: %v", err)
	}
	if read, err := tags.ReadValue("tags"); err != nil || elementCount(&read) != 0 {
		t.Errorf("reading unset tags: %s, %v; want the empty sequence", FormatValue(read), err)
	}
}

// An optional part or item declaration names no object of its own: read directly,
// bare or qualified, it is the empty sequence — as through an instantiated owner.
func TestOptionalOccurrenceDeclarationEvaluatesEmpty(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Wheel { attribute radius : Real = 0.3; }
		item def Tag;
		part def Car {
			part spare : Wheel[0..1];
			item label : Tag[0..1];
			part fitted : Wheel;
		}
		part spare : Wheel[0..1];
		item label : Tag[0..1];
		part fitted : Wheel;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	before := len(ctx.instances)
	for _, src := range []string{"spare", "test::spare", "label", "test::label", "Car::spare", "test::Car::label", "spare.radius"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "spare istype Wheel"); err != nil || val.Kind != ValConst || !val.Const.Bool {
		t.Errorf("spare istype Wheel = %s, %v; want true of nothing", FormatValue(val), err)
	}
	if made := len(ctx.instances) - before; made != 0 {
		t.Errorf("reading the optional declarations made %d object(s), want none", made)
	}
	for _, src := range []string{"fitted", "test::fitted"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || val.Kind != ValInstance {
			t.Errorf("%s = %s, %v; want the one object", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "fitted.radius"); err != nil || val.Kind != ValConst || val.Const.Real != 0.3 {
		t.Errorf("fitted.radius = %s, %v; want 0.3", FormatValue(val), err)
	}
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate Car: %v", err)
	}
	for _, name := range []string{"spare", "label"} {
		fv, err := car.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("car.%s: %v", name, err)
		}
		if read, err := fv.ReadValue(name); err != nil || elementCount(&read) != 0 {
			t.Errorf("car.%s reads %s, %v; want the empty sequence", name, FormatValue(read), err)
		}
	}
}
