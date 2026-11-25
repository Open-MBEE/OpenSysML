package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const unboundParameters = `package Unbound {
    calc def F {
        in a : ScalarValues::Real;
        in b : ScalarValues::Real default = 1.0;
        return r : ScalarValues::Real = a + b;
    }
    attribute x : ScalarValues::Real = F(2.0);
    attribute y : ScalarValues::Real = F(2.0, 3.0);
    attribute z : ScalarValues::Real = F(b = 3.0);
    calc def G {
        in a : ScalarValues::Real;
        out b : ScalarValues::Real;
        return r : ScalarValues::Real = a;
    }
    attribute v : ScalarValues::Real = G(1.0);
}
`

func inputsOf(t *testing.T, g *rdf.Graph, feature, predicate string) []string {
	t.Helper()
	root := rdf.ExpressionIRI(rdf.ElementIRI(feature), "value")
	var out []string
	for _, term := range g.Objects(root, rdf.SysML+predicate) {
		out = append(out, term.Value)
	}
	return out
}

// An invocation lists under input and parameter the callee's `in` parameters no
// argument binds — the callee's own feature — after the features it owns, the result last of those.
func TestInvocationListsTheParametersNoArgumentBinds(t *testing.T) {
	g := turtleOf(t, "unbound", unboundParameters)
	for _, tc := range []struct {
		feature   string
		predicate string
		want      string
	}{
		{"Unbound::x", "input", "urn:opensysml:expr:Unbound__x_pvalue_pin0 urn:sysmlv2:element:Unbound__F__b"},
		{"Unbound::x", "parameter", "urn:opensysml:expr:Unbound__x_pvalue_pin0 urn:opensysml:expr:Unbound__x_pvalue_pout urn:sysmlv2:element:Unbound__F__b"},
		{"Unbound::y", "input", "urn:opensysml:expr:Unbound__y_pvalue_pin0 urn:opensysml:expr:Unbound__y_pvalue_pin1"},
		{"Unbound::z", "input", "urn:opensysml:expr:Unbound__z_pvalue_pin0 urn:sysmlv2:element:Unbound__F__a"},
		{"Unbound::v", "input", "urn:opensysml:expr:Unbound__v_pvalue_pin0"},
		{"Unbound::v", "parameter", "urn:opensysml:expr:Unbound__v_pvalue_pin0 urn:opensysml:expr:Unbound__v_pvalue_pout urn:sysmlv2:element:Unbound__G__b"},
	} {
		if got := strings.Join(inputsOf(t, g, tc.feature, tc.predicate), " "); got != tc.want {
			t.Errorf("%s value's %s = %q, want %q", tc.feature, tc.predicate, got, tc.want)
		}
	}
}

// A library function's unbound parameter is listed by the library's own identity:
// `DurationOf(o)` leaves `Time::DurationOf::clock` to its default.
func TestInvocationListsTheLibraryParameterNoArgumentBinds(t *testing.T) {
	g := turtleOf(t, "clocked", `package Clocked {
	private import Time::*;
	occurrence o;
	attribute d : DurationValue = DurationOf(o);
}
`)
	got := inputsOf(t, g, "Clocked::d", "input")
	if len(got) != 2 || got[0] != "urn:opensysml:expr:Clocked__d_pvalue_pin0" ||
		strings.HasPrefix(got[1], "urn:opensysml:expr:") || !strings.HasPrefix(got[1], "urn:sysmlv2:element:") {
		t.Fatalf("DurationOf(o) input = %q, want the owned argument then the library's clock", got)
	}
}

// The listed library parameters restate the callee's signature: the notation is
// rebuilt from the graph alone and states the same graph again.
func TestUnboundParametersSurviveTheRoundTrip(t *testing.T) {
	turtle, err := convert.Convert("unbound.sysml", []byte(unboundParameters), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := withoutSourceText(t, turtle)
	back, err := convert.Convert("unbound.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the graph alone: %v", err)
	}
	if string(back) != unboundParameters {
		t.Errorf("notation was not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", unboundParameters, back)
	}
	again, err := convert.Convert("unbound.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(withoutSourceText(t, again)) != string(stripped) {
		t.Errorf("the rebuilt notation states another graph:\n%s", firstLineDifference(stripped, withoutSourceText(t, again)))
	}
}

const selectedCallee = `package Selected {
    calc def F {
        in x : ScalarValues::Real;
        in extra : ScalarValues::Real default = 2.0;
        return r : ScalarValues::Real = x + extra;
    }
    part q {
        calc f : F;
    }
    attribute w : ScalarValues::Real = q.f(1.0);
    part def A {
        calc def g {
            in a : ScalarValues::Real;
            in b : ScalarValues::Real default = 1.0;
            return r : ScalarValues::Real = a + b;
        }
    }
    part def B {
        calc def g {
            in a : ScalarValues::Boolean;
            in c : ScalarValues::Boolean default = true;
            return r : ScalarValues::Boolean = a and c;
        }
    }
    part def C :> A, B {
        attribute one : ScalarValues::Real = g(1.0);
        attribute two : ScalarValues::Boolean = g(false);
    }
    requirement def R {
        subject s : ScalarValues::Real[0..1];
        in a : ScalarValues::Real;
    }
    attribute checked : ScalarValues::Boolean = R(a = 1.0);
    calc def h {
        in a : ScalarValues::Real;
        in outerOption : ScalarValues::Real default = 1.0;
        return r : ScalarValues::Real = a + outerOption;
    }
    attribute shadowed : ScalarValues::Real = h(1.0) {
        calc def h {
            in a : ScalarValues::Real;
            in innerOption : ScalarValues::Real default = 2.0;
            return r : ScalarValues::Real = a + innerOption;
        }
    }
}
`

// A receiver chain's call lists the parameters of the Function typing its final link that
// no argument binds; an overloaded name lists only the parameters of the overload the arguments' types select;
// a requirement's unbound subject is its first input parameter; a value's call is read in the
// feature's declaring scope, so a body member of the same name does not shadow the callee.
func TestInvocationListsTheSelectedCalleesUnboundParameters(t *testing.T) {
	g := turtleOf(t, "selected", selectedCallee)
	for _, tc := range []struct {
		feature   string
		predicate string
		want      string
	}{
		{"Selected::w", "input", "urn:opensysml:expr:Selected__w_pvalue_pin0 urn:sysmlv2:element:Selected__F__extra"},
		{"Selected::w", "parameter", "urn:opensysml:expr:Selected__w_pvalue_pin0 urn:opensysml:expr:Selected__w_pvalue_pout urn:sysmlv2:element:Selected__F__extra"},
		{"Selected::C::one", "input", "urn:opensysml:expr:Selected__C__one_pvalue_pin0 urn:sysmlv2:element:Selected__A__g__b"},
		{"Selected::C::two", "input", "urn:opensysml:expr:Selected__C__two_pvalue_pin0 urn:sysmlv2:element:Selected__B__g__c"},
		{"Selected::checked", "input", "urn:opensysml:expr:Selected__checked_pvalue_pin0 urn:sysmlv2:element:Selected__R__s"},
		{"Selected::shadowed", "input", "urn:opensysml:expr:Selected__shadowed_pvalue_pin0 urn:sysmlv2:element:Selected__h__outerOption"},
	} {
		if got := strings.Join(inputsOf(t, g, tc.feature, tc.predicate), " "); got != tc.want {
			t.Errorf("%s value's %s = %q, want %q", tc.feature, tc.predicate, got, tc.want)
		}
	}
}
