package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const featureValueOverridingCode = "feature-value-overriding"

// overridingDiags returns the binding-override diagnostics of src, asserting
// that each one is an error covering one of the wanted value parts.
func overridingDiags(t *testing.T, src string, wantSpans ...string) []diag.Diagnostic {
	t.Helper()
	diags := only(constraintDiags(t, src), featureValueOverridingCode)
	if len(diags) != len(wantSpans) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(wantSpans), diags)
	}
	for i, d := range diags {
		if d.Severity != diag.SeverityError {
			t.Errorf("severity = %v, want an error", d.Severity)
		}
		if got := spanText(src, d); got != wantSpans[i] {
			t.Errorf("span text = %q, want %q", got, wantSpans[i])
		}
	}
	return diags
}

// A value bound with `=` cannot be overridden by a redefinition, in a usage or
// in a specializing definition alike (KerML 1.0 §8.3.4.10.2).
func TestFeatureValueOverridingBindingIsReported(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def P { attribute a : Integer = 1; }
		part p : P { attribute :>> a = 2; }
		part def Q :> P { attribute :>> a = 3; }
	}`
	diags := overridingDiags(t, src, "= 2", "= 3")
	msg := diags[0].Message
	for _, want := range []string{"cannot override the binding value of P::P::a", "`default =`", "remove this value"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}
}

// A `default` value is there to be overridden.
func TestFeatureValueOverridingDefaultIsClean(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def P { attribute a : Integer default = 1; attribute b : Integer default 1; }
		part p : P { attribute :>> a = 2; attribute :>> b := 2; }
		part def Q :> P { attribute :>> a default = 3; }
	}`
	overridingDiags(t, src)
}

// A redefinition that states no value of its own, or only a body, overrides nothing.
func TestFeatureValueOverridingNoValueIsClean(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def P { attribute a : Integer = 1; attribute e : Integer; }
		part def Q :> P { attribute :>> a; attribute :>> e = 4; }
		part p : P { attribute :>> a { doc /* restated */ } }
	}`
	overridingDiags(t, src)
}

// The binding is found through the whole redefinition chain: past a redefinition
// that states no value, and past one that restates it as a default (itself an
// override of the binding).
func TestFeatureValueOverridingIsTransitive(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def P { attribute a : Integer = 1; }
		part def Q :> P { attribute :>> a; }
		part def R :> Q { attribute :>> a = 3; }
		part def Q2 :> P { attribute :>> a default = 2; }
		part def R2 :> Q2 { attribute :>> a = 4; }
	}`
	overridingDiags(t, src, "= 3", "default = 2", "= 4")
}

// A default written over a binding is still an override of the binding, and an
// initial value `:=` is a binding too, whichever side of the redefinition it is on.
func TestFeatureValueOverridingDefaultAndInitialOverBinding(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def P { attribute a : Integer = 1; attribute v : Integer := 1; }
		part p : P { attribute :>> a default = 2; attribute :>> v = 2; }
		part q : P { attribute :>> a := 3; }
	}`
	overridingDiags(t, src, "default = 2", "= 2", ":= 3")
}

// A parameter redefines the parameter at its position implicitly, so binding it
// again overrides the general behavior's binding; a default there is fine.
func TestFeatureValueOverridingImplicitParameterRedefinition(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		action def A { in x : Integer = 1; in y : Integer default = 1; }
		action a : A { in x = 2; in y = 2; }
		action def A2 :> A { in :>> x = 5; }
		calc def C { in i : Integer = 1; return r : Integer = i; }
		calc c : C { in i = 3; }
	}`
	overridingDiags(t, src, "= 2", "= 5", "= 3")
}

// An unnamed result parameter is named by its owner, not by a dangling `::`.
func TestFeatureValueOverridingUnnamedResult(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Real;
		calc def Base { in x : Real; return : Real = x; }
		calc def Own :> Base { return : Real = x + 1.0; }
	}`
	diags := overridingDiags(t, src, "= x + 1.0")
	if msg := diags[0].Message; !strings.Contains(msg, "of the unnamed feature of P::Base:") {
		t.Errorf("message %q does not name the result by its owner", msg)
	}
}

// Nested and named-redefinition shapes reach the same binding.
func TestFeatureValueOverridingNestedAndRenamed(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def N { part sub { attribute a : Integer = 1; } }
		part n : N { part :>> sub { attribute :>> a = 2; } }
		part def S :> N { part :>> sub { attribute z : Integer :>> a = 9; } }
	}`
	overridingDiags(t, src, "= 2", "= 9")
}

// A requirement's subject is a parameter too: a usage's subject, named, bound
// anonymously or redefining explicitly, overrides the definition's bound
// subject, while a `default` subject may be rebound.
func TestFeatureValueOverridingSubject(t *testing.T) {
	const src = `package P {
		part def V;
		part v0 : V;
		part v1 : V;
		requirement def R { subject v : V = v0; }
		requirement r1 : R { subject = v1; }
		requirement r2 : R { subject w = v1; }
		requirement def R2 :> R { subject :>> v default = v1; }
		requirement def RD { subject v : V default = v0; }
		requirement r3 : RD { subject :>> v = v1; }
		requirement r4 : RD { subject = v1; }
	}`
	overridingDiags(t, src, "= v1", "= v1", "default = v1")
}

// An anonymous subject may carry any value operator; each overrides a binding
// like `=` does, and none overrides a default.
func TestFeatureValueOverridingAnonymousSubjectOperators(t *testing.T) {
	const src = `package P {
		part def V;
		part v0 : V;
		part v1 : V;
		requirement def R { subject v : V = v0; }
		requirement r1 : R { subject := v1; }
		requirement r2 : R { subject default v1; }
		requirement r3 : R { subject default = v1; }
		requirement r4 : R { subject default := v1; }
		requirement def RD { subject v : V default = v0; }
		requirement r5 : RD { subject := v1; }
		requirement r6 : RD { subject default = v1; }
	}`
	overridingDiags(t, src, ":= v1", "default v1", "default = v1", "default := v1")
}

// A subject redefines the subject of every general requirement by position, so
// naming one general's subject explicitly does not exempt another's binding.
func TestFeatureValueOverridingSubjectOfEveryGeneral(t *testing.T) {
	const src = `package P {
		part def X;
		part x0 : X;
		part x1 : X;
		part x2 : X;
		requirement def A { subject s : X default = x0; }
		requirement def B { subject s : X = x1; }
		requirement def C :> A, B { subject s :>> A::s = x2; }
		requirement def D :> A { subject s :>> A::s = x2; }
	}`
	diags := overridingDiags(t, src, "= x2")
	if !strings.Contains(diags[0].Message, "P::B::s") {
		t.Errorf("message %q does not name P::B::s", diags[0].Message)
	}
}

// A named constraint a requirement owns through `require`/`assume constraint` is
// a feature of the requirement, so rebinding it in a specialization or a usage
// overrides its binding, whether the redefinition is named or borrows the name;
// a `default` binding may be rebound.
func TestFeatureValueOverridingOwnedConstraint(t *testing.T) {
	const src = `package P {
		constraint def C;
		constraint c0 : C;
		constraint c1 : C;
		requirement def R {
			require constraint c : C = c0;
			assume constraint a : C default = c0;
		}
		requirement def S :> R {
			require constraint :>> c = c1;
			assume constraint :>> a = c1;
		}
		requirement r : R { require constraint :>> c = c1; }
		requirement def T :> R { require constraint x :>> c = c1; }
		requirement def U :> R { require constraint :>> c default = c1; }
	}`
	diags := overridingDiags(t, src, "= c1", "= c1", "= c1", "default = c1")
	for _, d := range diags {
		if !strings.Contains(d.Message, "P::R::c") {
			t.Errorf("message %q does not name P::R::c", d.Message)
		}
	}
}

// The binding of an owned constraint is found past a specialization that
// restates the constraint without a value. The restatement declares the name: a
// requirement constraint derives none from what it redefines (pilot 2026-07).
func TestFeatureValueOverridingOwnedConstraintIsTransitive(t *testing.T) {
	const src = `package P {
		constraint def C;
		constraint c0 : C;
		constraint c1 : C;
		requirement def R { require constraint c : C = c0; }
		requirement def S :> R { require constraint c :>> c; }
		requirement def T :> S { require constraint :>> c = c1; }
		requirement def RD { assume constraint a : C default = c0; }
		requirement def SD :> RD { assume constraint a :>> a; }
		requirement def TD :> SD { assume constraint :>> a = c1; }
	}`
	overridingDiags(t, src, "= c1")
}

// An owned constraint no name and no lone naming feature names — one redefining
// two features — is still a feature, so its value is checked against every
// binding it redefines.
func TestFeatureValueOverridingAnonymousOwnedConstraint(t *testing.T) {
	const src = `package P {
		constraint def C;
		constraint k0 : C;
		constraint k1 : C;
		requirement def R {
			require constraint c1 : C default = k0;
			require constraint c2 : C = k0;
		}
		requirement def S :> R { require constraint :>> c1, c2 = k1; }
		requirement def T :> R { require constraint :>> c1, c2 default = k1; }
		requirement def U :> R { assume constraint :>> c1 = k1; }
	}`
	diags := overridingDiags(t, src, "= k1", "default = k1")
	for _, d := range diags {
		if !strings.Contains(d.Message, "P::R::c2") {
			t.Errorf("message %q does not name P::R::c2", d.Message)
		}
	}
}

// The parameters of a require/assume constraint redefine the parameters of the
// constraint it redefines by position, as those of any step do: a lone `in limit`
// overrides the first parameter's binding, whatever its name (pilot 2026-07 agrees).
func TestFeatureValueOverridingOwnedConstraintParameters(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Real;
		constraint def Below { in x : Real; in limit : Real; x < limit }
		requirement def Base {
			attribute m : Real = 300.0;
			require constraint n : Below { in x = m; in limit default = 400.0; }
			assume constraint a : Below { in x = m; in limit = 400.0; }
		}
		requirement def S1 :> Base { require constraint :>> n { in limit = 200.0; } }
		requirement def S2 :> Base { assume constraint :>> a { in x = m; in limit = 200.0; } }
		requirement def S3 :> Base { require constraint :>> n { in x; in limit = 200.0; } }
		requirement def S4 :> Base { require constraint :>> n { in :>> x; in :>> limit = 200.0; } }
		requirement def S5 :> Base { require constraint :>> n { in x = m; in limit = 200.0; } }
	}`
	overridingDiags(t, src, "= 200.0", "= m", "= 200.0", "= m")
}
