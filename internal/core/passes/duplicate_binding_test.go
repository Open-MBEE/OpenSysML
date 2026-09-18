package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// kermlLibraryDiags analyzes src as KerML against the bundled library and returns
// the diagnostics a binding reports, as libraryDiags does for SysML.
func kermlLibraryDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	idx := newTestIndex()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx.AddDocument("<t>.kerml", root)
	idx.ExpandWildcardImports()
	var out []diag.Diagnostic
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		if d.Source == "type" || d.Source == "name-resolution" {
			out = append(out, d)
		}
	}
	return out
}

// wantMessages checks that diags carry exactly the messages wanted, in order.
func wantMessages(t *testing.T, diags []diag.Diagnostic, want ...string) {
	t.Helper()
	if len(diags) != len(want) {
		t.Fatalf("expected %d diagnostic(s) %q, got %v", len(want), want, diags)
	}
	for i, d := range diags {
		if d.Code != "type.expr" || !strings.Contains(d.Message, want[i]) {
			t.Fatalf("expected diagnostic %d to be type.expr %q, got %s %q", i, want[i], d.Code, d.Message)
		}
	}
}

// A KerML invocation binds each parameter once, whatever name reaches it: a
// parameter's own name, its short name, an alias or a qualified name of an
// inherited one (validateInvocationExpressionNoDuplicateParameterRedefinition).
func TestInvocationDuplicateParameterBindingKerML(t *testing.T) {
	const model = `package P {
		class T;
		function F { in <xs> x : T; in y : T = null; return r : T; }
		function G :> F { in :>> y = null; }
		feature t : T;
		feature f : T = %s;
	}`
	for _, call := range []string{
		"F(x = t, y = t)", "F(xs = t, y = t)", "G(x = t, y = t)", "G(F::x = t, y = t)",
	} {
		if diags := kermlLibraryDiags(t, fmt.Sprintf(model, call)); len(diags) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", call, diags)
		}
	}
	for _, call := range []string{
		"F(x = t, x = t)", "F(x = t, xs = t)", "F(xs = t, x = t)", "G(x = t, x = t)", "G(F::x = t, x = t)", "G(x = t, y = t, G::y = t)",
	} {
		diags := kermlLibraryDiags(t, fmt.Sprintf(model, call))
		if len(diags) != 1 || diags[0].Code != "type.expr" || !strings.Contains(diags[0].Message, "binds parameter") ||
			!strings.HasSuffix(diags[0].Message, "twice") {
			t.Errorf("%s: expected one duplicate-binding diagnostic, got %v", call, diags)
		}
	}
	// A general's parameter that the function's own parameter redefines is no
	// parameter of the function, so naming it is an unknown name, not a duplicate.
	wantMessages(t, kermlLibraryDiags(t, fmt.Sprintf(model, "G(x = t, F::y = t)")), `G has no parameter named "F::y"`)
}

// A written name that only resolves within a later overload — a qualified or aliased
// parameter — selects that overload, and a second spelling of it is a duplicate there.
func TestInvocationOverloadSelectedByResolvedParameterName(t *testing.T) {
	const model = `package P {
		class T;
		package A { function f { in x : T; in a : T; return r : T; } }
		package B { function f { in x : T; in b : T; alias bs for b; return r : T; } }
		private import A::*;
		private import B::*;
		feature t : T;
		feature v : T = %s;
	}`
	for _, call := range []string{"f(x = t, B::f::b = t)", "f(x = t, bs = t)", "f(x = t, A::f::a = t)"} {
		if diags := kermlLibraryDiags(t, fmt.Sprintf(model, call)); len(diags) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", call, diags)
		}
	}
	wantMessages(t, kermlLibraryDiags(t, fmt.Sprintf(model, "f(x = t, B::f::b = t, b = t)")), `f binds parameter "b" twice`)
	wantMessages(t, kermlLibraryDiags(t, fmt.Sprintf(model, "f(x = t, bs = t, b = t)")), `f binds parameter "b" twice`)
	wantMessages(t, kermlLibraryDiags(t, fmt.Sprintf(model, "f(x = t, b = t, B::f::b = t)")), `f binds parameter "b" twice`)
}

// The rule reads through the receiver: a call of a feature typed by the function
// binds the function's parameters.
func TestInvocationDuplicateParameterBindingThroughUsage(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		calc def K { in <a> alpha : Integer; in b : Integer = 0; return r : Integer = alpha + b; }
		calc k : K;
		attribute v : Integer = %s;
	}`
	wantLibraryClean(t, fmt.Sprintf(model, "k(a = 1, b = 2)"))
	wantLibraryDiag(t, fmt.Sprintf(model, "k(a = 1, alpha = 2)"), "type.expr", `k binds parameter "alpha" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "K(alpha = 1, a = 2)"), "type.expr", `K binds parameter "alpha" twice`)
}

// SysML spellings of an invocation are one rule: a calc call, an action usage
// valued by a call, a `perform action` valued by one.
func TestInvocationDuplicateParameterBindingSysML(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		action def A { in x : Integer; in y : Integer = 0; }
		calc def K { in a : Integer; in b : Integer = 0; return r : Integer = a + b; }
		action def Owner {
			%s
		}
	}`
	for _, member := range []string{
		"perform action pa = A(x = 1, y = 2);",
		"action a = A(x = 1, y = 2);",
		"attribute k = K(a = 1, b = 2);",
	} {
		wantLibraryClean(t, fmt.Sprintf(model, member))
	}
	wantLibraryDiag(t, fmt.Sprintf(model, "perform action pa = A(x = 1, x = 2);"), "type.expr", `A binds parameter "x" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "action a = A(x = 1, x = 2);"), "type.expr", `A binds parameter "x" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "attribute k = K(a = 1, a = 2);"), "type.expr", `K binds parameter "a" twice`)
}

// An invocation heading a feature chain is argument-checked as a bare one is, under
// one segment or several: duplicates, excess and unknown parameters alike, and a
// default-less parameter left unbound is the same advisory in either position.
func TestInvocationHeadingFeatureChain(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		calc def K { in <a> alpha : Integer; in b : Integer = 0; return r : Integer = alpha + b; }
		item def Sig { attribute p : Integer; ref item inner : Sig; }
		calc def M { in n : Integer; return r : Sig; }
		part def Recv;
		action def Owner {
			part rcv : Recv;
			%s
		}
	}`
	for _, member := range []string{
		"attribute v = K(a = 1, b = 2).r;",
		"item s = M(n = 1).r.inner.inner;",
		"send M(n = 1).r.inner to rcv;",
	} {
		wantLibraryClean(t, fmt.Sprintf(model, member))
	}
	wantLibraryDiag(t, fmt.Sprintf(model, "attribute v = K(a = 1, alpha = 2).r;"), "type.expr", `K binds parameter "alpha" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "item s = M(n = 1, n = 2).r.inner.inner;"), "type.expr", `M binds parameter "n" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send M(n = 1, M::n = 2).r.inner to rcv;"), "type.expr", `M binds parameter "n" twice`)
	wantLibraryDiag(t, fmt.Sprintf(model, "attribute v = K(c = 1).r;"), "type.expr", `K has no parameter named "c"`)
	wantLibraryDiag(t, fmt.Sprintf(model, "item s = M(1, 2).r.inner;"), "type.expr", `M takes 1 argument(s), found 2`)
	for _, member := range []string{
		"attribute v = K();",
		"attribute v = K(b = 2);",
		"attribute v = K().r;",
		"attribute v = K(b = 2).r;",
	} {
		wantLibraryWarning(t, fmt.Sprintf(model, member), CodeUnboundParameter,
			"K leaves parameter alpha unbound, so the call cannot be evaluated")
	}
}

// A constructor binds each feature of the instantiated type once, whatever name
// reaches it: an inherited feature, a redefining one, a short name, an alias or a
// qualified name (validateConstructorExpressionNoDuplicateFeatureRedefinition).
func TestConstructorDuplicateFeatureBindingKerML(t *testing.T) {
	const model = `package P {
		class T;
		class Base { feature x : T; feature y : T; }
		class Sub :> Base { feature :>> x; feature z : T; }
		class C { feature x : T; alias xa for x; feature <sn> longName : T; }
		feature t : T;
		feature c = %s;
	}`
	for _, call := range []string{
		"new Sub(t, t, t)", "new Sub()", "new Sub(x = t, y = t, z = t)", "new Sub(Base::y = t, z = t)",
		"new C(x = t, longName = t)", "new C(xa = t, sn = t)",
	} {
		if diags := kermlLibraryDiags(t, fmt.Sprintf(model, call)); len(diags) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", call, diags)
		}
	}
	for _, call := range []string{
		"new Sub(x = t, x = t)", "new Sub(y = t, Base::y = t)", "new Sub(Sub::x = t, x = t)", "new Sub(z = t, y = t, z = t)",
		"new C(x = t, xa = t)", "new C(sn = t, longName = t)", "new C(C::x = t, x = t)",
	} {
		diags := kermlLibraryDiags(t, fmt.Sprintf(model, call))
		if len(diags) != 1 || diags[0].Code != "type.expr" ||
			!strings.Contains(diags[0].Message, "is already bound by an earlier argument") {
			t.Errorf("%s: expected one duplicate-binding diagnostic, got %v", call, diags)
		}
	}
	wantMessages(t, kermlLibraryDiags(t, fmt.Sprintf(model, "new C(x = t, x = t, xa = t)")),
		"x of C is already bound by an earlier argument", "x of C is already bound by an earlier argument")
}

// A constructed SysML payload — a part or attribute value, a sent message — binds
// each feature once, however deeply the payload expression nests the constructor.
func TestConstructorDuplicateFeatureBindingSysML(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		item def Sig { attribute p : Integer; attribute q : Integer; ref item inner : Sig; }
		part def Base { attribute x : Integer; }
		part def Sub :> Base { attribute :>> x; ref part inner : Sub; }
		part def Recv;
		action def SendM { in item m : Sig; }
		action def Owner {
			part rcv : Recv;
			attribute flag : Boolean;
			item msg : Sig;
			%s
		}
	}`
	for _, member := range []string{
		"send new Sig(p = 1, q = 2) to rcv;",
		"send SendM(msg) to rcv;",
		"send (if flag ? SendM(msg) else SendM(msg)) to rcv;",
		"send SendM(new Sig(p = 1, q = 2)) to rcv;",
		"send (1, 2).{in i : Integer; SendM(msg)} to rcv;",
		"part s : Sub = new Sub(x = 1);",
		"part s : Sub = new Sub(x = 1).inner.inner;",
		"item a : Sig = new Sig(p = 1, q = 2);",
	} {
		wantLibraryClean(t, fmt.Sprintf(model, member))
	}
	wantLibraryDiag(t, fmt.Sprintf(model, "send new Sig(p = 1, p = 2) to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send (if flag ? new Sig(p = 1, p = 2) else new Sig(p = 1)) to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send (new Sig(p = 1), new Sig(q = 1, q = 2)) to rcv;"), "type.expr", `q of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send SendM(new Sig(p = 1, p = 2)) to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send new Sig(p = 1, p = 2).p to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send new Sig(p = 1, p = 2).SendM() to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send new Sig(p = 1, p = 2).inner.inner.p to rcv;"), "type.expr", `p of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "part s : Sub = new Sub(x = 1, x = 2).inner.inner;"), "type.expr", `x of Sub is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "send (1, 2).{in i : Integer; new Sig(q = i, q = i)} to rcv;"), "type.expr", `q of Sig is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "part s : Sub = new Sub(x = 1, x = 2);"), "type.expr", `x of Sub is already bound by an earlier argument`)
	wantLibraryDiag(t, fmt.Sprintf(model, "item a : Sig = new Sig(p = 1, Sig::p = 2);"), "type.expr", `p of Sig is already bound by an earlier argument`)
}

// An argument binding no `in` parameter of the invoked type — a name that is no
// parameter, an `out` or `return` parameter, a positional argument past the last
// parameter — is reported at the argument with the pilot's wording
// (validateInvocationExpressionParameterRedefinition); inherited, redefined and
// receiver-typed parameters bind.
func TestInvocationParameterRedefinitionKerML(t *testing.T) {
	const model = `package P {
		datatype T;
		function F { in x : T; in y : T; return r : T; }
		function G :> F { in redefines x; }
		function H :> F { in z : T; }
		step S { in a : T; out b : T; }
		feature t : T;
		feature f : F;
		feature v = %s;
	}`
	for _, call := range []string{
		"F(t, t)", "F(x = t, y = t)", "G(x = t, y = t)", "H(z = t, y = t)", "f(x = t, y = t)", "S(a = t)", "S(t)",
	} {
		if diags := kermlLibraryDiags(t, fmt.Sprintf(model, call)); len(diags) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", call, diags)
		}
	}
	for call, want := range map[string]struct{ at, msg string }{
		"F(w = t)":        {"w", `F has no parameter named "w"`},
		"F(r = t)":        {"r", `F has no parameter named "r"`},
		"S(b = t)":        {"b", `S has no parameter named "b"`},
		"f(x = t, w = t)": {"w", `f has no parameter named "w"`},
		"H(x = t, z = t)": {"x", `H has no parameter named "x"`},
		"F(t, t, t)":      {"t", "F takes 2 argument(s), found 3"},
		"S(t, t)":         {"t", "S takes 1 argument(s), found 2"},
	} {
		src := fmt.Sprintf(model, call)
		diags := kermlLibraryDiags(t, src)
		if len(diags) != 1 {
			t.Errorf("%s: expected one diagnostic, got %v", call, diags)
			continue
		}
		d := diags[0]
		if d.Code != "type.expr" || !strings.HasPrefix(d.Message, msgInvocationParameterRedefinition+": "+want.msg) {
			t.Errorf("%s: expected %q, got %s %q", call, want.msg, d.Code, d.Message)
		}
		if spanned := strings.TrimSpace(src[d.Span.Offset:d.Span.End()]); spanned != want.at {
			t.Errorf("%s: expected the report at %q, got %q", call, want.at, spanned)
		}
	}
}
