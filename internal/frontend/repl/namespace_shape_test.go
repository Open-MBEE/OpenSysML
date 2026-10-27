package repl

import (
	"strings"
	"testing"
)

// A feature's values are a sequence its multiplicity constrains, so a
// namespace-level read holds one value in the feature's effective multiplicity
// exactly as the same feature read on an object does: a `[0..*]` feature valued
// `4` is `[4]` on both, an at-most-one feature holds its one value bare.
func TestEvalNamespaceReadHoldsEffectiveMultiplicity(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package P {
		private import ScalarValues::*;
		attribute xs : Integer[0..*] = 4;
		attribute ys : Integer[0..*] = (1, 2);
		attribute e : Integer[0..*];
		attribute one : Integer[1] = 4;
		attribute opt : Integer[0..1] = 4;
		attribute bare : Integer = 4;
		attribute two : Integer[2] = (4, 5);
		attribute seq : Integer[0..*] = (4);
		part def D {
			attribute xs : Integer[0..*] = 4;
			attribute e : Integer[0..*];
			attribute one : Integer[1] = 4;
			attribute opt : Integer[0..1] = 4;
			attribute bare : Integer = 4;
			attribute two : Integer[2] = (4, 5);
			attribute seq : Integer[0..*] = (4);
		}
		part d : D;
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	for expr, want := range map[string]string{
		"P::xs":      "= [4]",
		"P::d.xs":    "= [4]",
		"P::ys":      "= [1, 2]",
		"P::e":       "= []",
		"P::d.e":     "= []",
		"P::one":     "= 4",
		"P::d.one":   "= 4",
		"P::opt":     "= 4",
		"P::d.opt":   "= 4",
		"P::bare":    "= 4",
		"P::d.bare":  "= 4",
		"P::two":     "= [4, 5]",
		"P::d.two":   "= [4, 5]",
		"P::seq":     "= [4]",
		"P::d.seq":   "= [4]",
		"P::xs + 1":  "= 5",
		"P::xs == 4": "= true",
	} {
		wants(t, run(t, s, "%eval "+expr), want)
		lines, err := s.EvalBare(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		wants(t, strings.Join(lines, "\n"), want)
	}
}
