package passes

import (
	"slices"
	"testing"
)

func TestW8CMultiplicityBoundNotNatural(t *testing.T) {
	cases := map[string]string{
		"boolean literal": "feature f [1..false];",
		"string literal":  "feature f [\"x\"..*];",
		"boolean valued":  "feature n = 0;\n\tfeature b = n > 3;\n\tfeature f [n..b];",
		"negative valued": "feature n = 0;\n\tfeature m = n - 1;\n\tfeature f [m..2];",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

func TestW8CMultiplicityBoundLegal(t *testing.T) {
	src := `package P {
	feature n = 0;
	feature m = n + 2;
	feature a [0..*];
	feature b [1];
	feature c [n..m];
	feature d [*];
}`
	if msgs := w8cMessages(t, src); len(msgs) != 0 {
		t.Errorf("expected no constraint diagnostics, got %v", msgs)
	}
}

func TestW8CMultiplicityBoundSpanIsTheBound(t *testing.T) {
	src := "package P {\n\tfeature f [1..false];\n}"
	var found bool
	for _, d := range constraintDiagsKerML(t, src) {
		if d.Message != msgMultiplicityBoundNatural {
			continue
		}
		found = true
		if got := src[d.Span.Offset:d.Span.End()]; got != "false" {
			t.Errorf("span covers %q, want %q", got, "false")
		}
	}
	if !found {
		t.Fatal("rule did not fire")
	}
}

// The corpus case semantic/k37-multiplicity-bound-not-natural.kerml and its
// relatives: a bound whose result is typed by a class, a data type or a
// non-integer scalar is rejected however the type was derived.
func TestW8CMultiplicityBoundResultTypeNotNatural(t *testing.T) {
	cases := map[string]string{
		"class typed":        "class C { feature n : C; feature f : C [n]; }",
		"class typed upper":  "class C { feature n : C; feature f : C [1..n]; }",
		"datatype typed":     "datatype D; class C { feature d : D; feature f [d]; }",
		"class typed lower":  "class C { feature n : C; feature f : C [n..*]; }",
		"real typed":         "class C { feature r : ScalarValues::Real; feature f [r]; }",
		"string typed":       "class C { feature s : ScalarValues::String; feature f [s]; }",
		"boolean typed":      "class C { feature b : ScalarValues::Boolean; feature f [b]; }",
		"real valued":        "class C { feature r = 3.5; feature f [r]; }",
		"class typed nested": "class C { feature n : C; feature f : C [n + 1]; }",
		"integer exponent":   "class C { feature i : ScalarValues::Integer; feature f [2 ** i]; }",
		"negated exponent":   "class C { feature n : ScalarValues::Natural; feature f [2 ** -n]; }",
		"real exponent":      "class C { feature r : ScalarValues::Real; feature f [2 ^ r]; }",
		"class typed subset": "class C { feature n : C; feature m subsets n; feature f [m]; }",
		"class typed redef":  "class C { feature n : C; } class D specializes C { feature :>> n; feature f [n]; }",
		"class typed chain":  "class C { feature n : C; feature m subsets n; feature k :> m; feature f [k]; }",
		"real typed subset":  "class C { feature r : ScalarValues::Real; feature s subsets r; feature f [s]; }",
		"bare step":          "step s; feature f [s];",
		"bare behavior":      "behavior b; feature f [b];",
		"bare connector":     "class C { connector c; feature f [c]; }",
		"bare step subset":   "step s; feature t subsets s; feature f [t];",
		"bare class":         "class C; feature f [C];",
		"anything typed":     "feature a : Base::Anything; feature f [a];",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

// A bare usage is typed by its kind's library base, so a part, item, port or
// action used as a bound is rejected like an explicitly class-typed feature.
func TestW8CMultiplicityBoundImplicitKindTypeSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	part p;
	part xs [p];
	item i;
	item ys [i];
	port q;
	part ws [q];
	action act;
	part us [act];
	part sub :> p;
	part vs [sub];
	attribute n : Natural;
	part ok [n];
	attribute a;
	part oka [a];
	ref r;
	part okr [r];
	attribute m :> n;
	part okm [0..m];
}`
	var got []string
	for _, d := range constraintDiags(t, src) {
		if d.Message == msgMultiplicityBoundNatural {
			got = append(got, src[d.Span.Offset:d.Span.End()])
		}
	}
	want := []string{"p", "i", "q", "act", "sub"}
	if !slices.Equal(got, want) {
		t.Errorf("bounds reported %v, want %v", got, want)
	}
}

// A computed bound — a call, a constructor, a quantity — is judged by the
// declared type of its result, not by the scalar lattice alone.
func TestW8CMultiplicityBoundComputedResultTypeNotNatural(t *testing.T) {
	cases := map[string]string{
		"class valued call":       "class C; function F { return : C; } class D { feature f [F()]; }",
		"class valued call upper": "class C; function F { return : C; } class D { feature f [0..F()]; }",
		"class valued call arith": "class C; function F { return : C; } class D { feature f [F() + 1]; }",
		"class valued exponent":   "class C; function F { return : C; } class D { feature f [2 ** F()]; }",
		"integer exponent call":   "function I { return : ScalarValues::Integer; } class D { feature f [2 ** I()]; }",
		"string valued call":      "function S { return : ScalarValues::String; } class D { feature f [S()]; }",
		"real valued call":        "function R { return : ScalarValues::Real; } class D { feature f [R()]; }",
		"constructor":             "class C; class D { feature f [new C()]; }",
		"quantity literal":        "class D { feature f [3 [SI::kg]]; }",
		"quantity feature":        "class D { feature n : ScalarValues::Natural = 3; feature f [n [SI::kg]]; }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

func TestW8CMultiplicityBoundComputedResultTypeSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	private import SI::kg;
	part def C;
	calc def F { return : C; }
	calc def N { return : Natural; }
	part def D {
		part xs : C [F()];
		part ys : C [0..N()];
		part zs : C [3 [kg]];
		part ws : C [N() + 1];
	}
}`
	var got int
	for _, d := range constraintDiags(t, src) {
		if d.Message == msgMultiplicityBoundNatural {
			got++
		}
	}
	if got != 2 {
		t.Errorf("want two %q (F() and 3 [kg]), got %d", msgMultiplicityBoundNatural, got)
	}
}

// Integer-conforming results are accepted, and a bound whose type cannot be
// resolved or was never declared stays silent for the name-resolution tier.
func TestW8CMultiplicityBoundResultTypeSilent(t *testing.T) {
	cases := map[string]string{
		"natural typed":         "class C { feature n : ScalarValues::Natural; feature f [n]; }",
		"integer typed":         "class C { feature i : ScalarValues::Integer; feature f [0..i]; }",
		"positive typed":        "class C { feature p : ScalarValues::Positive; feature f [p..*]; }",
		"natural valued":        "class C { feature n : ScalarValues::Natural = 3; feature f [n]; }",
		"integer valued":        "class C { feature i = 3; feature f [i]; }",
		"integer arithmetic":    "class C { feature n : ScalarValues::Natural; feature f [n + 1]; }",
		"natural exponent":      "class C { feature n : ScalarValues::Natural; feature f [2 ** n]; }",
		"positive exponent":     "class C { feature i : ScalarValues::Integer; feature p : ScalarValues::Positive; feature f [i ^ (p + 1)]; }",
		"literal exponent":      "class C { feature i : ScalarValues::Integer; feature f [i ** 2]; }",
		"natural subset":        "class C { feature n : ScalarValues::Natural; feature m subsets n; feature f [m]; }",
		"natural redef":         "class C { feature n : ScalarValues::Natural; } class D specializes C { feature :>> n; feature f [n]; }",
		"untyped":               "class C { feature u; feature f [u]; }",
		"untyped subset":        "class C { feature u; feature v subsets u; feature f [v]; }",
		"untyped package level": "feature u; feature f [u];",
		"unresolved subset":     "class C { feature q : Undeclared; feature v subsets q; feature f [v]; }",
		"unresolved type":       "class C { feature q : Undeclared; feature f [q]; }",
		"unresolved bound":      "class C { feature f [nothere]; }",
		"natural call":          "function N { return : ScalarValues::Natural; } class C { feature f [N()]; }",
		"integer call":          "function I { return : ScalarValues::Integer; } class C { feature f [0..I()]; }",
		"natural call arith":    "function N { return : ScalarValues::Natural; } class C { feature f [N() + 1]; }",
		"natural exponent call": "function N { return : ScalarValues::Natural; } class C { feature f [2 ** N()]; }",
		"untyped result call":   "function U { return r; } class C { feature f [U()]; }",
		"unresolved call":       "class C { feature f [Nowhere()]; }",
		"unresolved result":     "function G { return : Undeclared; } class C { feature f [G()]; }",
		"unresolved unit":       "class C { feature f [3 [nounit]]; }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 0 {
				t.Errorf("want no %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}
