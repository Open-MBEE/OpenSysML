package passes

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const msgMultiplicityBoundForm = "a multiplicity bound must be a literal or a feature name (KerML.xtext MultiplicityExpressionMember)"

// Bounds outside MultiplicityExpressionMember are syntax errors; the checker still judges the recovered tree.
func boundSyntaxErrors(t *testing.T, name, src string) int {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	p.ParseFile()
	n := 0
	for _, d := range p.Diagnostics {
		if d.Message == msgMultiplicityBoundForm {
			n++
		}
	}
	return n
}

func TestW8CMultiplicityBoundNotNatural(t *testing.T) {
	cases := []struct {
		name, body string
		syntax     int
	}{
		{"boolean literal", "feature f [1..false];", 0},
		{"string literal", "feature f [\"x\"..*];", 0},
		{"boolean valued", "feature n = 0;\n\tfeature b = n > 3;\n\tfeature f [n..b];", 0},
		{"negative valued", "feature n = 0;\n\tfeature m = n - 1;\n\tfeature f [m..2];", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P {\n\t" + tt.body + "\n}"
			if got := boundSyntaxErrors(t, "<t>.kerml", src); got != tt.syntax {
				t.Errorf("multiplicity-bound syntax errors = %d, want %d", got, tt.syntax)
			}
			msgs := w8cMessages(t, src)
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

func TestW8CMultiplicityBoundEvaluableMustHaveValue(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		want   int
		syntax int
	}{
		{
			name: "package feature without value",
			src: `package P {
	feature k : ScalarValues::Natural;
	feature d [k];
}`,
			want:   1,
			syntax: 0,
		},
		{
			name: "package feature with value",
			src: `package P {
	feature j : ScalarValues::Natural = 3;
	feature e [j];
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "indirect package feature without value",
			src: `package P {
	private import ScalarValues::*;
	feature u : Natural;
	feature k : Natural = u;
	feature d [k];
}`,
			want:   1,
			syntax: 0,
		},
		{
			name: "indirect operator reads feature without value",
			src: `package P {
	private import ScalarValues::*;
	feature u : Natural;
	feature k : Natural = u + 1;
	feature d [k];
}`,
			want:   1,
			syntax: 0,
		},
		{
			name: "conditional skips unselected valueless branch",
			src: `package P {
	feature u : ScalarValues::Natural;
	feature k : ScalarValues::Natural = if true ? (2 as ScalarValues::Natural) else u;
	feature d [k];
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "conditional selects valueless branch",
			src: `package P {
	feature u : ScalarValues::Natural;
	feature k : ScalarValues::Natural = if false ? (2 as ScalarValues::Natural) else u;
	feature d [k];
}`,
			want:   1,
			syntax: 0,
		},
		{
			name: "indirect operator reads valued feature",
			src: `package P {
	private import ScalarValues::*;
	feature j : Natural = 2;
	feature k : Natural = j + 1;
	feature d [k];
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "class member indirectly reads package feature",
			src: `package P {
	private import ScalarValues::*;
	feature u : Natural;
	class T {
		feature k : Natural = u;
		feature d [k];
	}
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "cyclic feature values",
			src: `package P {
	private import ScalarValues::*;
	feature a : Natural = b;
	feature b : Natural = a;
	feature d [a];
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "package feature valued by cast",
			src: `package P {
	feature k : ScalarValues::Natural = 2 as ScalarValues::Natural;
	feature d [k];
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "evaluable cast bound",
			src: `package P {
	feature d [0..(2 as ScalarValues::Integer)];
}`,
			want:   0,
			syntax: 1,
		},
		{
			name: "type member without value",
			src: `package P {
	class T {
		feature k : ScalarValues::Natural;
		feature d [k];
	}
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "package feature with negative value",
			src: `package P {
	feature n : ScalarValues::Integer = -1;
	feature f [n];
}`,
			want:   1,
			syntax: 0,
		},
		{
			name: "type member with negative value",
			src: `package P {
	class U {
		feature n : ScalarValues::Integer = -1;
		feature g [n];
	}
}`,
			want:   0,
			syntax: 0,
		},
		{
			name: "untyped package feature",
			src: `package P {
	feature u;
	feature f [u];
}`,
			want:   1,
			syntax: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := boundSyntaxErrors(t, "<t>.kerml", tt.src); got != tt.syntax {
				t.Errorf("multiplicity-bound syntax errors = %d, want %d", got, tt.syntax)
			}
			msgs := w8cMessages(t, tt.src)
			if w8cCount(msgs, msgMultiplicityBoundNatural) != tt.want {
				t.Errorf("want %d %q, got %v", tt.want, msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

func TestW8CMultiplicityBoundEvaluableMustHaveValueSysML(t *testing.T) {
	src := `package P {
	attribute k : ScalarValues::Natural;
	attribute d [k];
}`
	if got := boundSyntaxErrors(t, "<t>", src); got != 0 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 0", got)
	}
	var got int
	for _, d := range constraintDiags(t, src) {
		if d.Message == msgMultiplicityBoundNatural {
			got++
		}
	}
	if got != 1 {
		t.Errorf("want one %q, got %d", msgMultiplicityBoundNatural, got)
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
	if got := boundSyntaxErrors(t, "<t>.kerml", src); got != 0 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 0", got)
	}
	if msgs := w8cMessages(t, src); len(msgs) != 0 {
		t.Errorf("expected no constraint diagnostics, got %v", msgs)
	}
}

func TestW8CMultiplicityBoundSpanIsTheBound(t *testing.T) {
	src := "package P {\n\tfeature f [1..false];\n}"
	if got := boundSyntaxErrors(t, "<t>.kerml", src); got != 0 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 0", got)
	}
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
	cases := []struct {
		name, body string
		syntax     int
	}{
		{"class typed", "class C { feature n : C; feature f : C [n]; }", 0},
		{"class typed upper", "class C { feature n : C; feature f : C [1..n]; }", 0},
		{"datatype typed", "datatype D; class C { feature d : D; feature f [d]; }", 0},
		{"class typed lower", "class C { feature n : C; feature f : C [n..*]; }", 0},
		{"real typed", "class C { feature r : ScalarValues::Real; feature f [r]; }", 0},
		{"string typed", "class C { feature s : ScalarValues::String; feature f [s]; }", 0},
		{"boolean typed", "class C { feature b : ScalarValues::Boolean; feature f [b]; }", 0},
		{"real valued", "class C { feature r = 3.5; feature f [r]; }", 0},
		{"class typed nested", "class C { feature n : C; feature f : C [n + 1]; }", 1},
		{"integer exponent", "class C { feature i : ScalarValues::Integer; feature f [2 ** i]; }", 1},
		{"negated exponent", "class C { feature n : ScalarValues::Natural; feature f [2 ** -n]; }", 1},
		{"real exponent", "class C { feature r : ScalarValues::Real; feature f [2 ^ r]; }", 1},
		{"class typed subset", "class C { feature n : C; feature m subsets n; feature f [m]; }", 0},
		{"class typed redef", "class C { feature n : C; } class D specializes C { feature :>> n; feature f [n]; }", 0},
		{"class typed chain", "class C { feature n : C; feature m subsets n; feature k :> m; feature f [k]; }", 0},
		{"real typed subset", "class C { feature r : ScalarValues::Real; feature s subsets r; feature f [s]; }", 0},
		{"bare step", "step s; feature f [s];", 0},
		{"bare behavior", "behavior b; feature f [b];", 0},
		{"bare connector", "class C { connector c; feature f [c]; }", 0},
		{"bare step subset", "step s; feature t subsets s; feature f [t];", 0},
		{"bare class", "class C; feature f [C];", 0},
		{"anything typed", "feature a : Base::Anything; feature f [a];", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P {\n\t" + tt.body + "\n}"
			if got := boundSyntaxErrors(t, "<t>.kerml", src); got != tt.syntax {
				t.Errorf("multiplicity-bound syntax errors = %d, want %d", got, tt.syntax)
			}
			msgs := w8cMessages(t, src)
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
	part def H {
		attribute n : Natural;
		part ok [n];
		attribute a;
		part oka [a];
		ref r;
		part okr [r];
		attribute m :> n;
		part okm [0..m];
	}
}`
	if got := boundSyntaxErrors(t, "<t>", src); got != 0 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 0", got)
	}
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

func TestW8CMultiplicityBoundEvaluableKindTypeSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute n : Natural;
	part ok [n];
	attribute a;
	part oka [a];
	ref r;
	part okr [r];
	attribute m :> n;
	part okm [0..m];
}`
	if got := boundSyntaxErrors(t, "<t>", src); got != 0 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 0", got)
	}
	var got []string
	for _, d := range constraintDiags(t, src) {
		if d.Message == msgMultiplicityBoundNatural {
			got = append(got, src[d.Span.Offset:d.Span.End()])
		}
	}
	want := []string{"n", "a", "r", "m"}
	if !slices.Equal(got, want) {
		t.Errorf("bounds reported %v, want %v", got, want)
	}
}

// A computed bound — a call, a constructor, a quantity — is judged by the
// declared type of its result, not by the scalar lattice alone.
func TestW8CMultiplicityBoundComputedResultTypeNotNatural(t *testing.T) {
	cases := []struct {
		name, body string
	}{
		{"class valued call", "class C; function F { return : C; } class D { feature f [F()]; }"},
		{"class valued call upper", "class C; function F { return : C; } class D { feature f [0..F()]; }"},
		{"class valued call arith", "class C; function F { return : C; } class D { feature f [F() + 1]; }"},
		{"class valued exponent", "class C; function F { return : C; } class D { feature f [2 ** F()]; }"},
		{"integer exponent call", "function I { return : ScalarValues::Integer; } class D { feature f [2 ** I()]; }"},
		{"string valued call", "function S { return : ScalarValues::String; } class D { feature f [S()]; }"},
		{"real valued call", "function R { return : ScalarValues::Real; } class D { feature f [R()]; }"},
		{"constructor", "class C; class D { feature f [new C()]; }"},
		{"quantity literal", "class D { feature f [3 [SI::kg]]; }"},
		{"quantity feature", "class D { feature n : ScalarValues::Natural = 3; feature f [n [SI::kg]]; }"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P {\n\t" + tt.body + "\n}"
			if got := boundSyntaxErrors(t, "<t>.kerml", src); got != 1 {
				t.Errorf("multiplicity-bound syntax errors = %d, want 1", got)
			}
			msgs := w8cMessages(t, src)
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
	if got := boundSyntaxErrors(t, "<t>", src); got != 4 {
		t.Errorf("multiplicity-bound syntax errors = %d, want 4", got)
	}
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
	cases := []struct {
		name, body string
		syntax     int
	}{
		{"natural typed", "class C { feature n : ScalarValues::Natural; feature f [n]; }", 0},
		{"integer typed", "class C { feature i : ScalarValues::Integer; feature f [0..i]; }", 0},
		{"positive typed", "class C { feature p : ScalarValues::Positive; feature f [p..*]; }", 0},
		{"natural valued", "class C { feature n : ScalarValues::Natural = 3; feature f [n]; }", 0},
		{"integer valued", "class C { feature i = 3; feature f [i]; }", 0},
		{"integer arithmetic", "class C { feature n : ScalarValues::Natural; feature f [n + 1]; }", 1},
		{"natural exponent", "class C { feature n : ScalarValues::Natural; feature f [2 ** n]; }", 1},
		{"positive exponent", "class C { feature i : ScalarValues::Integer; feature p : ScalarValues::Positive; feature f [i ^ (p + 1)]; }", 1},
		{"literal exponent", "class C { feature i : ScalarValues::Integer; feature f [i ** 2]; }", 1},
		{"natural subset", "class C { feature n : ScalarValues::Natural; feature m subsets n; feature f [m]; }", 0},
		{"natural redef", "class C { feature n : ScalarValues::Natural; } class D specializes C { feature :>> n; feature f [n]; }", 0},
		{"untyped", "class C { feature u; feature f [u]; }", 0},
		{"untyped subset", "class C { feature u; feature v subsets u; feature f [v]; }", 0},
		{"unresolved subset", "class C { feature q : Undeclared; feature v subsets q; feature f [v]; }", 0},
		{"unresolved type", "class C { feature q : Undeclared; feature f [q]; }", 0},
		{"unresolved bound", "class C { feature f [nothere]; }", 0},
		{"natural call", "function N { return : ScalarValues::Natural; } class C { feature f [N()]; }", 1},
		{"integer call", "function I { return : ScalarValues::Integer; } class C { feature f [0..I()]; }", 1},
		{"natural call arith", "function N { return : ScalarValues::Natural; } class C { feature f [N() + 1]; }", 1},
		{"natural exponent call", "function N { return : ScalarValues::Natural; } class C { feature f [2 ** N()]; }", 1},
		{"untyped result call", "function U { return r; } class C { feature f [U()]; }", 1},
		{"unresolved call", "class C { feature f [Nowhere()]; }", 1},
		{"unresolved result", "function G { return : Undeclared; } class C { feature f [G()]; }", 1},
		{"unresolved unit", "class C { feature f [3 [nounit]]; }", 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P {\n\t" + tt.body + "\n}"
			if got := boundSyntaxErrors(t, "<t>.kerml", src); got != tt.syntax {
				t.Errorf("multiplicity-bound syntax errors = %d, want %d", got, tt.syntax)
			}
			msgs := w8cMessages(t, src)
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 0 {
				t.Errorf("want no %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}
