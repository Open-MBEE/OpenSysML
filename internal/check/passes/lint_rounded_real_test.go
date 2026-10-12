package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestRoundedRealLiteralLint(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		wants [][]string
	}{
		{"declared value", `package P { private import ScalarValues::*; attribute x : Real = 0.1; }`,
			[][]string{{"0.1 is rounded to the nearest Real, 0.10000000000000001", "type the feature by Rational"}}},
		{"negated", `package P { private import ScalarValues::*; attribute x : Real = -0.1; }`,
			[][]string{{"-0.1 is rounded to the nearest Real, -0.10000000000000001"}}},
		{"exponent", `package P { private import ScalarValues::*; attribute x : Real = 1.1e-1; }`,
			[][]string{{"1.1e-1 is rounded"}}},
		{"each element", `package P { private import ScalarValues::*; attribute xs : Real[3] = (0.25, 0.1, 0.3); }`,
			[][]string{{"0.1 is rounded"}, {"0.3 is rounded"}}},
		{"inherited type", `package P { private import ScalarValues::*; part def A { attribute x : Real; } part a : A { attribute :>> x = 0.2; } }`,
			[][]string{{"0.2 is rounded"}}},
		{"rounding past seventeen digits", `package P { private import ScalarValues::*; attribute x : Real = 0.01; }`,
			[][]string{{"0.01 is rounded to the nearest Real, 0.0100000000000000002,"}}},
		{"calc parameter", `package P { private import ScalarValues::*; calc def F { in g : Real = 9.81; return r : Real = g; } }`,
			[][]string{{"9.81 is rounded to the nearest Real, 9.8100000000000005"}}},
		{"assignment", `package P { private import ScalarValues::*; action a { attribute x : Real := 0.0; assign x := 0.7; } }`,
			[][]string{{"0.7 is rounded"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantLints(t, c.src, CodeRoundedRealLiteral, c.wants...)
		})
	}
}

func TestRoundedRealLiteralLintSilent(t *testing.T) {
	cases := []struct{ name, src string }{
		{"exact binary fraction", `package P { private import ScalarValues::*; attribute x : Real = 0.25; }`},
		{"whole decimal", `package P { private import ScalarValues::*; attribute x : Real = 1.5e3; }`},
		{"Integer literal", `package P { private import ScalarValues::*; attribute x : Real = 3; }`},
		{"Rational feature", `package P { private import ScalarValues::*; attribute x : Rational = 0.1; }`},
		{"untyped feature", `package P { attribute x = 0.1; }`},
		{"computed value", `package P { private import ScalarValues::*; attribute x : Real = 1 / 3; attribute y : Real = 0.1 + 0.2; }`},
		{"beyond the Real range", `package P { private import ScalarValues::*; attribute x : Real = 1e400; }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, mode := range []diag.ConformanceMode{diag.ConformanceDefault, diag.ConformanceStrict} {
				if got := lintDiags(t, c.src, CodeRoundedRealLiteral, mode); len(got) != 0 {
					t.Errorf("%s: got %+v, want no finding", mode, got)
				}
			}
		})
	}
}

func TestRoundedRealLiteralLintDisabled(t *testing.T) {
	src := `package P { private import ScalarValues::*; attribute x : Real = 0.1; }`
	got := lintDiags(t, src, CodeRoundedRealLiteral, diag.ConformanceDefault)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if kept := WithoutLints(got, map[string]bool{CodeRoundedRealLiteral: true}, map[string]bool{CodeRoundedRealLiteral: true}); len(kept) != 0 {
		t.Errorf("disabled lint kept %+v", kept)
	}
}
