package repl

import (
	"strings"
	"testing"
)

// calcRedefinitionModel declares one pure calc the compiled tier takes; factor
// is the multiplier its body applies, so a redefinition is told apart by result.
func calcRedefinitionModel(factor string) string {
	return `package M {
		private import ScalarValues::*;
		calc def Scale { in k : Integer; return : Integer = k * ` + factor + `; }
		calc def Twice { in k : Integer; return : Integer = Scale(k) + Scale(k); }
	}`
}

// runCalcValue runs invocation and returns the one value it produced.
func runCalcValue(t *testing.T, s *Session, invocation string) string {
	t.Helper()
	v := s.RunCalc(invocation)
	if !v.Holds() {
		t.Fatalf("%%calc %s: %s", invocation, strings.Join(v.Lines, "\n"))
	}
	if len(v.Values) != 1 {
		t.Fatalf("%%calc %s produced %d values, want 1: %v", invocation, len(v.Values), v.Values)
	}
	return v.Values[0].Value
}

// A calc invoked once has a compiled body; redefining it must invoke the new
// definition, directly and through a caller compiled against the old one.
func TestRunCalcRedefinitionReplacesCompiledBody(t *testing.T) {
	s := NewSession()
	if res := s.Submit(calcRedefinitionModel("2")); hasSyntaxError(res) {
		t.Fatalf("model does not parse: %v", res.Diagnostics)
	}
	if got := runCalcValue(t, s, "M::Scale 4"); got != "8" {
		t.Fatalf("Scale(4) = %s, want 8", got)
	}
	if got := runCalcValue(t, s, "M::Twice 4"); got != "16" {
		t.Fatalf("Twice(4) = %s, want 16", got)
	}

	if res := s.Submit(calcRedefinitionModel("3")); hasSyntaxError(res) {
		t.Fatalf("redefinition does not parse: %v", res.Diagnostics)
	}
	if got := runCalcValue(t, s, "M::Scale 4"); got != "12" {
		t.Fatalf("Scale(4) after redefinition = %s, want 12", got)
	}
	if got := runCalcValue(t, s, "M::Twice 4"); got != "24" {
		t.Fatalf("Twice(4) after redefinition = %s, want 24", got)
	}
}

// An expression evaluated at the prompt invokes the redefined calc too.
func TestEvalExprRedefinitionReplacesCompiledBody(t *testing.T) {
	s := NewSession()
	s.Submit(calcRedefinitionModel("2"))
	lines, err := s.EvalExpr("M::Twice(5)")
	if err != nil {
		t.Fatalf("%%eval M::Twice(5): %v", err)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "20") {
		t.Fatalf("%%eval M::Twice(5) = %v, want 20", lines)
	}

	s.Submit(calcRedefinitionModel("5"))
	lines, err = s.EvalExpr("M::Twice(5)")
	if err != nil {
		t.Fatalf("%%eval M::Twice(5) after redefinition: %v", err)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "50") {
		t.Fatalf("%%eval M::Twice(5) after redefinition = %v, want 50", lines)
	}
}
