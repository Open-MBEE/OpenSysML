package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestMultiplicitySingleBound(t *testing.T) {
	m, root := buildModel(t, "part def C { part wheels [4]; }")
	c := sym(t, root, "C")
	wheels, ok := c.Scope.LookupLocal("wheels")
	if !ok {
		t.Fatalf("wheels not found")
	}
	r, ok := m.MultiplicityOf(wheels)
	if !ok {
		t.Fatalf("MultiplicityOf(wheels) not ok")
	}
	if !r.Lower.Known || r.Lower.Value != 4 || !r.Upper.Known || r.Upper.Value != 4 {
		t.Fatalf("single bound [4] = %+v, want lower=upper=4", r)
	}
}

func TestMultiplicityRangeStar(t *testing.T) {
	m, root := buildModel(t, "part def C { part parts [0..*]; }")
	c := sym(t, root, "C")
	parts, _ := c.Scope.LookupLocal("parts")
	r, ok := m.MultiplicityOf(parts)
	if !ok {
		t.Fatalf("MultiplicityOf(parts) not ok")
	}
	if !r.Lower.Known || r.Lower.Value != 0 {
		t.Fatalf("lower = %+v, want 0", r.Lower)
	}
	if !r.Upper.Known || !r.Upper.Infinite {
		t.Fatalf("upper = %+v, want infinite", r.Upper)
	}
}

// A single unbounded bound is 0..*, not *..*: the lower bound follows the upper
// one only when the upper one is bounded (KerML 1.0 §8.2.5.11).
func TestMultiplicitySingleBoundStar(t *testing.T) {
	m, root := buildModel(t, "part def C { part parts [*]; part exact [*..*]; }")
	c := sym(t, root, "C")

	parts, _ := c.Scope.LookupLocal("parts")
	r, ok := m.MultiplicityOf(parts)
	if !ok {
		t.Fatalf("MultiplicityOf(parts) not ok")
	}
	if !r.Lower.Known || r.Lower.Infinite || r.Lower.Value != 0 {
		t.Fatalf("[*] lower = %+v, want 0", r.Lower)
	}
	if !r.Upper.Known || !r.Upper.Infinite {
		t.Fatalf("[*] upper = %+v, want infinite", r.Upper)
	}
	if valid, evalOK := r.LowerLeUpper(); !evalOK || !valid {
		t.Fatalf("[*] LowerLeUpper = %v, %v; want true, true", valid, evalOK)
	}

	exact, _ := c.Scope.LookupLocal("exact")
	r, ok = m.MultiplicityOf(exact)
	if !ok {
		t.Fatalf("MultiplicityOf(exact) not ok")
	}
	if !r.Lower.Infinite || !r.Upper.Infinite {
		t.Fatalf("[*..*] = %+v, want both bounds infinite", r)
	}
}

func TestMultiplicityLowerLeUpper(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [2..5]; part b [5..2]; part c [1..*]; }")
	c := sym(t, root, "C")

	for _, tc := range []struct {
		name      string
		wantValid bool
	}{
		{"a", true},
		{"b", false},
		{"c", true},
	} {
		u, _ := c.Scope.LookupLocal(tc.name)
		r, ok := m.MultiplicityOf(u)
		if !ok {
			t.Fatalf("%s: MultiplicityOf not ok", tc.name)
		}
		valid, evalOK := r.LowerLeUpper()
		if !evalOK {
			t.Fatalf("%s: LowerLeUpper not evaluable", tc.name)
		}
		if valid != tc.wantValid {
			t.Fatalf("%s: valid = %v, want %v", tc.name, valid, tc.wantValid)
		}
	}
}

func TestMultiplicityNoneWhenAbsent(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	if _, ok := m.MultiplicityOf(a); ok {
		t.Fatalf("expected no multiplicity for bare usage")
	}
}

// A feature that declares no multiplicity holds exactly one value, so the
// effective multiplicity of a bare usage is the assumed 1..1.
func TestEffectiveMultiplicityAssumesOne(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; part b [0..*]; }")
	c := sym(t, root, "C")

	a, _ := c.Scope.LookupLocal("a")
	if r := m.EffectiveMultiplicityOf(a); r != AssumedRange() {
		t.Errorf("EffectiveMultiplicityOf(a) = %+v, want %+v", r, AssumedRange())
	}
	if m.EffectiveMultiplicityOf(a).CountViolation(2) == "" {
		t.Error("two values conform to an undeclared multiplicity, want a violation")
	}

	// A declared multiplicity is the effective one, assumed nowhere.
	b, _ := c.Scope.LookupLocal("b")
	declared, ok := m.MultiplicityOf(b)
	if !ok {
		t.Fatal("MultiplicityOf(b) not ok")
	}
	if r := m.EffectiveMultiplicityOf(b); r != declared {
		t.Errorf("EffectiveMultiplicityOf(b) = %+v, want the declared %+v", r, declared)
	}
}

// The `[m]` an end writes ahead of its kind keyword is its cross feature's
// multiplicity, not the end's own; the end's own follows its type.
func TestMultiplicityOfEndIsNotItsCrossFeatures(t *testing.T) {
	m, root := buildModel(t, `part def A;
connection def C {
    end [0..*] item x : A;
    end [0..1] item cart : A [1];
    end x1 [2] item y : A;
}`)
	c := sym(t, root, "C")
	known := func(v int64) Bound { return Bound{Value: v, Known: true} }
	many := Range{known(0), Bound{Infinite: true, Known: true}}

	x, _ := c.Scope.LookupLocal("x")
	if _, ok := m.MultiplicityOf(x); ok {
		t.Error("MultiplicityOf(x) ok, want the crossing [0..*] left to the cross feature")
	}
	if r := m.EffectiveMultiplicityOf(x); r != AssumedRange() {
		t.Errorf("EffectiveMultiplicityOf(x) = %+v, want the assumed %+v", r, AssumedRange())
	}
	cross := m.OwnedCrossFeature(x)
	if cross == nil {
		t.Fatal("OwnedCrossFeature(x) = nil, want the anonymous [0..*] feature")
	}
	if r, ok := m.MultiplicityOf(cross); !ok || r != many {
		t.Errorf("MultiplicityOf(cross of x) = %+v, %v, want %+v", r, ok, many)
	}

	cart, _ := c.Scope.LookupLocal("cart")
	if r, ok := m.MultiplicityOf(cart); !ok || r != (Range{known(1), known(1)}) {
		t.Errorf("MultiplicityOf(cart) = %+v, %v, want its own [1]", r, ok)
	}
	if r, ok := m.MultiplicityOf(m.OwnedCrossFeature(cart)); !ok || r != (Range{known(0), known(1)}) {
		t.Errorf("MultiplicityOf(cross of cart) = %+v, %v, want [0..1]", r, ok)
	}

	y, _ := c.Scope.LookupLocal("y")
	if _, ok := m.MultiplicityOf(y); ok {
		t.Error("MultiplicityOf(y) ok, want the [2] left to the named cross feature x1")
	}
	x1, _ := y.Scope.LookupLocal("x1")
	if r, ok := m.MultiplicityOf(x1); !ok || r != (Range{known(2), known(2)}) {
		t.Errorf("MultiplicityOf(x1) = %+v, %v, want [2]", r, ok)
	}
}

// CountViolation is the shared wording for a count against a multiplicity: an
// unbounded or unknown bound admits any count, either side of a stated one does
// not.
func TestRangeCountViolation(t *testing.T) {
	known := func(v int64) Bound { return Bound{Value: v, Known: true} }
	for _, tc := range []struct {
		name  string
		r     Range
		count int64
		want  string
	}{
		{"exact conforms", Range{known(3), known(3)}, 3, ""},
		{"too few", Range{known(3), known(3)}, 1, "1 value(s) bound to a feature with multiplicity lower bound 3"},
		{"too many", Range{known(3), known(3)}, 4, "4 value(s) bound to a feature with multiplicity upper bound 3"},
		{"none against a lower bound", Range{known(1), known(3)}, 0, "0 value(s) bound to a feature with multiplicity lower bound 1"},
		{"unbounded upper admits any count", Range{known(0), Bound{Infinite: true, Known: true}}, 7, ""},
		{"one or more admits one", Range{known(1), Bound{Infinite: true, Known: true}}, 1, ""},
		{"one or more rejects none", Range{known(1), Bound{Infinite: true, Known: true}}, 0, "0 value(s) bound to a feature with multiplicity lower bound 1"},
		{"unknown bounds admit any count", Range{}, 5, ""},
	} {
		if got := tc.r.CountViolation(tc.count); got != tc.want {
			t.Errorf("%s: CountViolation(%d) = %q, want %q", tc.name, tc.count, got, tc.want)
		}
	}
}

// Intersecting ranges keeps the tighter of each bound, a known bound standing in
// for an unknown one, so a feature bound by two specializations conforms to both.
func TestRangeIntersect(t *testing.T) {
	known := func(v int64) Bound { return Bound{Value: v, Known: true} }
	star := Bound{Infinite: true, Known: true}
	for _, tc := range []struct {
		name string
		a, b Range
		want Range
	}{
		{"tighter lower and upper", Range{known(0), star}, Range{known(1), known(3)}, Range{known(1), known(3)}},
		{"mixed", Range{known(2), star}, Range{known(0), known(5)}, Range{known(2), known(5)}},
		{"unknown defers", Range{}, Range{known(1), known(1)}, Range{known(1), known(1)}},
		{"infinite upper loses", Range{known(0), known(4)}, Range{known(0), star}, Range{known(0), known(4)}},
		{"both unknown", Range{}, Range{}, Range{}},
	} {
		if got := tc.a.Intersect(tc.b); got != tc.want {
			t.Errorf("%s: %v ∩ %v = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
		if got := tc.b.Intersect(tc.a); got != tc.want {
			t.Errorf("%s reversed: %v ∩ %v = %v, want %v", tc.name, tc.b, tc.a, got, tc.want)
		}
	}
}

// A bound that is not a literal is not evaluable, so the ordering check reports
// ok=false and a caller skips it rather than diagnosing an unknown bound.
func TestMultiplicityWithANonEvaluableBound(t *testing.T) {
	m, root := buildModel(t, "part def C { attribute n; part a [n..5]; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	r, ok := m.MultiplicityOf(a)
	if !ok {
		t.Fatal("MultiplicityOf(a) not ok: the usage declares a multiplicity")
	}
	if r.Lower.Known {
		t.Errorf("lower = %+v, want unknown", r.Lower)
	}
	if _, evalOK := r.LowerLeUpper(); evalOK {
		t.Error("LowerLeUpper is evaluable with an unknown lower bound")
	}
	if msg := r.CountViolation(0); msg != "" {
		t.Errorf("CountViolation against an unknown lower bound = %q, want none", msg)
	}
}

// An infinite lower bound only orders against an infinite upper one, so `[*..2]`
// is a stated, evaluable, invalid range rather than an unknown one.
func TestMultiplicityInfiniteLowerWithFiniteUpper(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [*..2]; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	r, ok := m.MultiplicityOf(a)
	if !ok {
		t.Fatal("MultiplicityOf(a) not ok")
	}
	valid, evalOK := r.LowerLeUpper()
	if !evalOK {
		t.Fatal("LowerLeUpper not evaluable: both bounds are stated")
	}
	if valid {
		t.Errorf("[*..2] = %+v reported a valid range", r)
	}
}

// Multiplicity is a property of a usage, so a definition and a nil symbol have
// none and take the assumed range.
func TestMultiplicityOfANonUsage(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [2]; }")
	c := sym(t, root, "C")
	for name, s := range map[string]*symbols.Symbol{"definition": c, "nil": nil} {
		if _, ok := m.MultiplicityOf(s); ok {
			t.Errorf("%s: MultiplicityOf reported a multiplicity", name)
		}
		if r := m.EffectiveMultiplicityOf(s); r != AssumedRange() {
			t.Errorf("%s: EffectiveMultiplicityOf = %+v, want the assumed range", name, r)
		}
	}
}

// Counts derived from a range with a bound that is not evaluable keep what the other
// bounds fix: a sum or product is at least what the evaluable lower bounds give and
// admits an unknown upper count; an unknown upper bound may admit more values.
func TestRangesWithUnknownBounds(t *testing.T) {
	known := func(n int64) Bound { return Bound{Value: n, Known: true} }
	unknown := Range{}
	oneToUnknown := Range{Lower: known(1)}
	two := CountRange(2)
	for name, tc := range map[string]struct{ got, want Range }{
		"[?..?] + [2]":        {unknown.Plus(two), Range{Lower: known(2)}},
		"[1..?] + [1..?]":     {oneToUnknown.Plus(oneToUnknown), Range{Lower: known(2)}},
		"[?..?] × [1]":        {unknown.Times(CountRange(1)), Range{Lower: known(0)}},
		"[?..?] × [0]":        {unknown.Times(CountRange(0)), CountRange(0)},
		"[2] × [1..?]":        {two.Times(oneToUnknown), Range{Lower: known(2)}},
		"[?..?] covering [2]": {unknown.Covering(two), Range{Lower: known(0)}},
		"[1..?] covering [2]": {oneToUnknown.Covering(two), Range{Lower: known(1)}},
		"[2] covering [0..3]": {two.Covering(Range{Lower: known(0), Upper: known(3)}), Range{Lower: known(0), Upper: known(3)}},
		"[2] + [3]":           {two.Plus(CountRange(3)), CountRange(5)},
		"[1] × [2]":           {AssumedRange().Times(two), CountRange(2)},
		"[0..*] × [2]":        {Range{Lower: known(0), Upper: unbounded}.Times(two), Range{Lower: known(0), Upper: unbounded}},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %s (%+v), want %s", name, tc.got.Text(), tc.got, tc.want.Text())
		}
	}
	if unknown.AdmitsMore(0) || !unknown.MayAdmitMore(0) {
		t.Error("[?..?] certainly admits more than 0 values, or may not: want neither")
	}
	if !two.AdmitsMore(1) || two.MayAdmitMore(2) {
		t.Error("[2] does not admit a second value, or may admit a third: want neither")
	}
	if _, exact := unknown.Exactly(); exact {
		t.Error("[?..?] reported an exact count")
	}
}
