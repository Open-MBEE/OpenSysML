package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// w8cErrorSites lists the errors of analysing src as line:col code.
func w8cErrorSites(t *testing.T, name, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		if d.Severity == diag.SeverityError {
			before := src[:d.Span.Offset]
			line := strings.Count(before, "\n") + 1
			col := d.Span.Offset - strings.LastIndex(before, "\n")
			out = append(out, fmt.Sprintf("%d:%d %s", line, col, d.Code))
		}
	}
	return out
}

// A chain member another type features is not featured within the argument's result (KerML
// 8.3.4.8.4 validateFeatureChainExpressionConformance); lines and columns are the pinned validator's.
func TestW8CChainExpressionMemberNotFeaturedWithinArgument(t *testing.T) {
	cases := []struct {
		name, file, src string
		want            []string
	}{
		{"kerml alias and import", "<t>.kerml", `package P {
    class B { feature y : B; }
    class A { alias z for B::y; feature w : B; public import B::y; }
    feature k : A;
    feature r1 = k.z;
    feature r2 = k.y;
    feature r3 = k.w.y;
    class K {
        feature kk : A;
        feature s1 = kk.z;
        feature s2 = kk.w.y;
    }
}`, []string{"5:20 feature-reference-referent", "6:20 feature-reference-referent", "10:25 feature-reference-referent"}},
		{"sysml usage and definition bodies", "<t>.sysml", `package P {
    part def B { attribute y; }
    part def A { alias z for B::y; part w : B; attribute x; }
    part a : A;
    part def C {
        attribute r1 = a.z;
        attribute r3 = a.w.y;
        constraint def K { in p : A; p.z == 1 }
    }
}`, []string{"6:26 feature-reference-referent", "8:40 feature-reference-referent"}},
		{"member of an argument typed by its value", "<t>.sysml", `package P {
    part def B { attribute y; }
    part def A { alias z for B::y; attribute x; }
    part a : A;
    part v = a;
    attribute r1 = v.x;
    attribute r2 = v.z;
}`, []string{"7:22 feature-reference-referent"}},
		{"inherited alias behind a subtype, parentheses and an index", "<t>.sysml", `package I1 {
    part def B { attribute y : ScalarValues::Integer; }
    part def A { attribute x : ScalarValues::Integer; alias z for B::y; public import B::*; }
    part def A2 :> A;
    part def H { part a : A2; part l : A2[*] ordered; }
    part h : H;
    attribute t1 = h.a.z;
    attribute t3 = (h.a).z;
    attribute t4 = h.l#(1).z;
    part def Q { part a2 : A2; constraint { a2.z > 1 } }
}`, []string{"7:24 feature-reference-referent", "8:26 feature-reference-referent", "9:28 feature-reference-referent", "10:48 feature-reference-referent"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := w8cErrorSites(t, tc.file, tc.src)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("errors = %v, want %v", got, tc.want)
			}
		})
	}
}

// A member the argument's result does feature stays clean however the result came by it:
// inherited, redefined, aliased from a supertype, read through a value or `this`.
func TestW8CChainExpressionMemberFeaturedWithinArgument(t *testing.T) {
	cases := map[string]string{
		"alias of an inherited feature": `package P {
    private import ScalarValues::*;
    part def B { attribute y : Integer; }
    part def A :> B { alias z for B::y; part w : B; }
    part def A2 :> A { attribute :>> y; }
    part a : A;
    part a2 : A2;
    part v = a;
    part def C {
        attribute r1 = a.z;
        attribute r2 = a.w.y;
        attribute r3 = a2.y;
        attribute r4 = v.y;
        calc def E { in p : A; p.z }
    }
}`,
		"subject bound to a part": `package P {
    private import ScalarValues::*;
    part def Ship { attribute cost : Real = 5.0; }
    part ship : Ship;
    analysis asserted {
        subject s = ship;
        assert constraint cheap { s.cost > 100.0 }
        objective positive { require constraint { c > 0.0 } }
        out c : Real = s.cost;
    }
}`,
		"invocation result": `package P {
    private import ScalarValues::*;
    action def Base { in x : Integer default 3; out r : Integer; }
    action caller {
        out attribute result : Integer;
        first start;
        then action run = Base(x = 3);
        then action keep { assign result := run.r; }
        then done;
    }
}`,
		"this": `package P {
    private import ScalarValues::*;
    part def Rig {
        attribute id : Integer = 7;
        calc k { this.id + 1 }
        constraint c { this.id == 7 }
    }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errs := w8cLibraryErrorsIn(t, "<t>.sysml", src); len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}
