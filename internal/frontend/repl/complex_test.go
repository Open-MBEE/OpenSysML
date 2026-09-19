package repl

import "testing"

// A Complex is shown as one number, `re + imi`, wherever the REPL shows a
// value: a %calc result, a %features value and a bare %eval.
func TestComplexValuesDisplayAsOneNumber(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package C {
		private import ScalarValues::*;
		private import ComplexFunctions::*;

		calc def Unit { return : Complex = i; }
		calc def Rotate { in z : Complex; return : Complex = z * i; }
		part def P {
			attribute z : Complex = rect(3.0, -4.0);
			attribute zs : Complex[2] = (i, rect(1.0, 0.0));
			attribute r : Real = abs(z);
		}
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}

	wants(t, run(t, s, "%calc C::Unit"), "= 0.0 + 1.0i")
	wants(t, run(t, s, "%eval C::Rotate(C::Unit())"), "= -1.0 + 0.0i")
	wants(t, run(t, s, "%eval ComplexFunctions::rect(1.0, 2.0) == ComplexFunctions::rect(1.0, 2.0)"), "= true")

	run(t, s, "%instantiate C::P")
	got := run(t, s, "%features C::P")
	wants(t, got, "z = 3.0 - 4.0i", "zs = [0.0 + 1.0i, 1.0 + 0.0i]", "r = 5.0")
	rejects(t, got, "(3.0, -4.0)", "[3.0, -4.0]")
}
