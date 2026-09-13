package passes

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The reference judges each argument of an operator or invocation as a binding to
// the parameter it fills: a part bound to a Real parameter draws the warning, a
// conforming or widening argument (Integer into Real, anything into Anything)
// does not. The pinned validate-sysml agrees line for line on this model.
func TestW9CArgumentBindingsSysML(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Wheel;
	part def Axle {
		part wheels : Wheel[2];
		part wheel : Wheel;
		attribute i : Integer;
		attribute r : Real;
		attribute anyValue : Base::Anything;
		calc def Half { in x : Real; return : Real; x / 2 }
		calc def Pair { in a : Real; in b : Real; return : Real; a + b }
		attribute a1 = wheel + 1;
		attribute a2 = 1 + wheel;
		attribute a3 = i + 1;
		attribute a4 = i + r;
		attribute a5 = Half(wheel);
		attribute a6 = Half(i);
		attribute a7 = Pair(wheel, wheel);
		attribute a8 = Half(r);
		attribute a9 = Half(anyValue);
		attribute a10 = wheels + 1;
		attribute a11 = -wheel;
		attribute a12 = wheel < 1;
		attribute a13 = wheel ?? 1;
		attribute a14 = wheel == 1;
		attribute a15 = if wheel == 1 ? wheel else wheel;
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 12, 13, 16, 18, 18, 21, 22, 23)
}

// The same judgment reads KerML: a class instance bound to a Real parameter of
// `+` or of a function draws the warning, an Integer does not.
func TestW9CArgumentBindingsKerML(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	class Wheel;
	class Axle {
		feature wheel : Wheel;
		feature i : Integer;
		feature r : Real;
		function Half { in x : Real; return : Real; x / 2 }
		function Named { in a : Real; in b : Integer; return : Real; a + b }
		feature a1 = wheel + 1;
		feature a2 = i + 1;
		feature a3 = Half(wheel);
		feature a4 = Half(i);
		feature a5 = Named(a = wheel, b = i);
		feature a6 = Half(Half(wheel));
		feature a7 = wheel == 1;
	}
}`
	w9cKerMLWantLines(t, src, "bound-feature-types", 10, 12, 14, 15)
}

// Each warning sits where the reference puts it: an invocation's at the argument
// written, a named one at `name = value`, a receiver's at the type it is passed
// to, an operator's over the whole expression (BindingConnector_Invalid2.sysml.xt:42).
func TestW9CArgumentBindingLocations(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Wheel;
	part def Axle {
		part wheel : Wheel;
		attribute r : Real;
		calc def Half { in x : Real; return : Real; x / 2 }
		calc def Named { in a : Real; in b : Integer; return : Real; a + b }
		calc def Rec { in self : Wheel; in n : Real; return : Real; n }
		attribute a1 = wheel + 1;
		attribute a2 = Half(wheel);
		attribute a3 = Named(a = wheel, b = wheel);
		attribute a4 = wheel->Rec(wheel);
		attribute a5 = Half(Half(wheel));
		attribute a6 = 1 + Half(wheel);
	}
}`
	var got []string
	for _, d := range only(w8dDiags(t, src), "bound-feature-types") {
		line, col := lineCol(src, d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d %q", line, col, src[d.Span.Offset:d.Span.End()]))
	}
	sort.Strings(got)
	want := []string{
		`10:18 "wheel + 1"`,
		`11:23 "wheel"`,
		`12:24 "a = wheel"`,
		`12:35 "b = wheel"`,
		`13:29 "wheel"`,
		`14:28 "wheel"`,
		`15:27 "wheel"`,
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An argument the type checker rejects precisely is not warned about as well:
// the error says what the binding warning would, and more.
func TestW9CArgumentBindingYieldsToTypeError(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Wheel;
	part def Axle {
		part wheel : Wheel;
		attribute i : Integer;
		calc def Take { in w : Wheel; return : Boolean; true }
		attribute a1 = Take(i);
		attribute a2 = wheel and true;
		attribute a3 = not wheel;
	}
}`
	diags := w8dDiags(t, src)
	if got := only(diags, "bound-feature-types"); len(got) != 0 {
		t.Errorf("warned beside the type errors: %v", got)
	}
	if errs := severityOf(diags, SeverityError); len(errs) != 3 {
		t.Errorf("got %d type errors, want one per argument: %v", len(errs), errs)
	}
}

// Not judged: an argument of unknown type, a call no overload is selected for,
// a sequence or collection value, and a parameter typed Element or Collection,
// which the type checker binds to any argument.
func TestW9CArgumentBindingsNotJudged(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	part def Wheel;
	part def Axle {
		part wheels : Wheel[2];
		part wheel : Wheel;
		attribute i : Integer;
		attribute untyped;
		calc def Half { in x : Real; return : Real; x / 2 }
		calc def Elem { in e : KerML::Element; return : Boolean; true }
		calc def Col { in items : Collections::Collection; return : Boolean; true }
		calc def Both { in a : Real; in b : Integer; return : Real; a + b }
		calc def Both { in a : Real; in b : String; return : Real; a }
		attribute a1 = Half(untyped);
		attribute a2 = Elem(wheel);
		attribute a3 = Col(wheel);
		attribute a4 = Col((wheel, wheel));
		attribute a5 = Both(wheel, wheel);
		attribute a6 = Half((wheel, wheel));
		attribute a7 = Half(wheels->select { in w; true });
	}
}`
	diags := w8dDiags(t, src)
	if errs := severityOf(diags, SeverityError); len(errs) != 0 {
		t.Fatalf("the model does not analyze: %v", errs)
	}
	if got := only(diags, "bound-feature-types"); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// severityOf returns the findings of one severity.
func severityOf(diags []Diagnostic, severity Severity) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Severity == severity {
			out = append(out, d)
		}
	}
	return out
}

// A call among overloads is judged only by the one its arguments select: when
// none fits, no signature is assumed, where a lone declaration is judged as named.
func TestW9CArgumentBindingsNeedASelectedOverload(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Wheel;
	part def Axle {
		part wheel : Wheel;
		attribute i : Integer;
		attribute s : String;
		calc def Both { in a : Real; in b : Integer; return : Real; a + b }
		calc def Both { in a : Real; in b : String; return : Real; a }
		calc def One { in a : Real; in b : Integer; return : Real; a + b }
		attribute a1 = Both(wheel, i);
		attribute a2 = Both(wheel, s);
		attribute a3 = One(wheel, i);
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 13)
}

// A state's entry action counting in a part's attribute is judged like any
// operator expression: `Integer + 1` conforms, `Wheel + 1` does not.
func TestW9CArgumentBindingsInStateAssignment(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Wheel;
	part def Unit {
		part wheel : Wheel;
		attribute accepted : Integer = 0;
		attribute r : Real;
		exhibit state duty {
			entry; then working;
			state working {
				entry action count {
					assign accepted := accepted + 1;
					assign r := wheel + 1;
				}
			}
		}
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 13)
}
