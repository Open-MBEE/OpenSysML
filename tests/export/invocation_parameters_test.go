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
