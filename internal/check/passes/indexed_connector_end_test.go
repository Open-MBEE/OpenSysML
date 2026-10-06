package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// indexedEndModel wraps one connector into the assembly the ends resolve in.
func indexedEndModel(member string) string {
	return `package F3Idx {
	private import ScalarValues::*;
	port def P { attribute value : Real default 0.0; }
	part def Source { port y : P[2]; }
	part def Sink { port u : P; }
	connection def C { end source[1] : P; end target[1] : P; }
	part def Asm {
		part s : Source;
		part k : Sink;
		attribute i : Integer = 2;
		attribute r : Real = 1.0;
		` + member + `
	}
}
`
}

// An indexed end is reported once per end, in every connector form that admits
// it, at the end as written.
func TestIndexedConnectorEndIsAnExtension(t *testing.T) {
	const want = "an indexed connector end"
	for _, tc := range []struct {
		name, member string
		ends         int
	}{
		{"connection", "connection : C connect [1] s.y#(1) to [1] k.u;", 1},
		{"connect", "connect s.y#(1) to k.u;", 1},
		{"both_ends", "connect s.y#(1) to k.u#(1);", 2},
		{"nary", "connect (s.y#(1), k.u, s.y#(2));", 2},
		{"interface", "interface s.y#(1) to k.u;", 1},
		{"allocate", "allocate s.y#(1) to k.u;", 1},
		{"bind", "bind s.y#(1) = k.u;", 1},
		{"flow", "flow s.y#(1) to k.u;", 1},
		{"flow_from_to", "flow f from s.y#(1) to k.u;", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := indexedEndModel(tc.member)
			wants := make([]string, tc.ends)
			for i := range wants {
				wants[i] = want
			}
			wantNotation(t, "a.sysml", src, CodeNonstandardNotation, wants...)
			root, pd, idx := analyzeInputs(t, "a.sysml", src)
			got := NonstandardNotationPass{}.Run(NewContext("a.sysml", idx, pd), "a.sysml", root)
			for _, d := range got {
				if text := src[d.Span.Offset:d.Span.End()]; !strings.HasPrefix(text, "s.y#(") && !strings.HasPrefix(text, "k.u#(") {
					t.Errorf("reported at %q, want the indexed end", text)
				}
			}
		})
	}
	// The KerML forms are reported in a KerML document too: neither grammar
	// admits the index.
	wantNotation(t, "a.kerml", `package K {
	class A { feature y[2]; feature u; }
	feature a : A;
	connector c from a.y#(1) to a.u;
	binding of a.y#(2) = a.u;
}`, CodeNonstandardNotation, want, want)
}

// A standard end, and `#(` where it is an expression operator, stay silent.
func TestWholeFeatureEndsAndIndexExpressionsAreSilent(t *testing.T) {
	for _, member := range []string{
		"connection : C connect [1] s.y to [1] k.u;",
		"interface s.y to k.u;",
		"flow s.y to k.u;",
		"bind s.y = k.u;",
		"attribute firstPort : P = s.y#(1);",
		"attribute v : Real = s.y#(i).value;",
	} {
		wantSilent(t, "a.sysml", indexedEndModel(member))
	}
}

// indexedEndErrors analyses a model and returns the diagnostics beyond the
// notation warning every indexed end carries.
func indexedEndErrors(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range analyzeAll(t, "a.sysml", src) {
		if d.Code == CodeNonstandardNotation {
			continue
		}
		out = append(out, d)
	}
	return out
}

// The index is typed: an Integer, within the feature's multiplicity when both
// are known; a well-typed index analyses clean, the end's multiplicity being one.
func TestIndexedConnectorEndIndexIsTyped(t *testing.T) {
	for _, member := range []string{
		"connection : C connect [1] s.y#(1) to [1] k.u;",
		"connection : C connect [1] s.y#(2) to [1] k.u;",
		"connection : C connect [1] s.y#(i) to [1] k.u;",
		"connection : C connect [1] s.y#(1 + 1) to [1] k.u;",
		"connection : C connect [1] s.y#(1) to [1] k.u#(1);",
		"interface s.y#(2) to k.u;",
		"flow s.y#(1) to k.u;",
		"flow f from s.y#(i) to k.u;",
		"bind s.y#(1) = k.u;",
		"connect s.y#(1) to k.u;",
	} {
		if got := indexedEndErrors(t, indexedEndModel(member)); len(got) != 0 {
			t.Errorf("%s: got %+v, want a clean analysis", member, got)
		}
	}
	for _, tc := range []struct{ member, want string }{
		{"connection : C connect [1] s.y#(3) to [1] k.u;", "index 3 is outside the multiplicity [2]"},
		{"connection : C connect [1] s.y#(1 + 2) to [1] k.u;", "index 3 is outside the multiplicity [2]"},
		{"connection : C connect [1] s.y#(0) to [1] k.u;", "index 0 selects no element"},
		{"connection : C connect [1] s.y#(r) to [1] k.u;", "must be an Integer, found Real"},
		{"connection : C connect [1] s.y#(1.5) to [1] k.u;", "must be an Integer, found Rational"},
		{"connection : C connect [1] s.y#(\"1\") to [1] k.u;", "must be an Integer, found String"},
		{"connection : C connect [1] s.y#(1) to [1] k.u#(2);", "index 2 is outside the multiplicity [1]"},
		{"interface s.y#(3) to k.u;", "index 3 is outside the multiplicity [2]"},
		{"flow s.y#(3) to k.u;", "index 3 is outside the multiplicity [2]"},
		{"flow f from s.y#(r) to k.u;", "must be an Integer, found Real"},
		{"bind s.y#(3) = k.u;", "index 3 is outside the multiplicity [2]"},
	} {
		got := indexedEndErrors(t, indexedEndModel(tc.member))
		if len(got) != 1 {
			t.Errorf("%s: got %d diagnostics %+v, want one", tc.member, len(got), got)
			continue
		}
		if got[0].Severity != diag.SeverityError || !strings.Contains(got[0].Message, tc.want) {
			t.Errorf("%s: got %+v, want an error containing %q", tc.member, got[0], tc.want)
		}
	}
}
