package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// disjointReports returns the disjoint-feature-types reports of src, failing on
// any other constraint report so the fixture isolates the rule.
func disjointReports(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range diagsIn(t, name, src, "constraint") {
		switch d.Code {
		case "disjoint-feature-types":
			out = append(out, d)
		default:
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
	return out
}

func wantDisjointReport(t *testing.T, diags []diag.Diagnostic, feature, typeA, typeB string) {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("disjoint-feature-types reports = %d, want 1: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Severity != diag.SeverityWarning {
		t.Errorf("severity = %v, want warning", d.Severity)
	}
	for _, want := range []string{feature, typeA, typeB, "disjoint", "can have no values"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q lacks %q", d.Message, want)
		}
	}
}

// Two unrelated types intersect on a redefinition (KerML 1.1 §8.3.3.3.4): the
// model is legal, and no disjoining is declared, so nothing is reported.
func TestDisjointFeatureTypes_UnrelatedTypesStaySilent(t *testing.T) {
	const src = `
		class A; class B;
		class X { feature f : A; }
		class Y :> X { feature :>> f : B; }
		class Z :> X { feature g : B :> f; }`
	if diags := disjointReports(t, "a.kerml", src); len(diags) != 0 {
		t.Fatalf("unrelated types reported: %v", diags)
	}
}

// Types related by disjoining share no instances (KerML 1.1 §7.3.2.5), so a
// redefinition typed by one over a feature typed by the other has no values.
func TestDisjointFeatureTypes_ExplicitDisjoiningOnRedefinition(t *testing.T) {
	const src = `
		class A; class B disjoint from A;
		class X { feature f : A; }
		class Y :> X { feature :>> f : B; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "A", "B")
}

// The disjoining may be written on either side.
func TestDisjointFeatureTypes_DisjoiningIsSymmetric(t *testing.T) {
	const src = `
		class B; class A disjoint from B;
		class X { feature f : A; }
		class Y :> X { feature :>> f : B; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "A", "B")
}

// A subtype's instances are its supertype's (KerML 1.1 §7.3.2.4), so a
// disjoining holds between every subtype of its two types.
func TestDisjointFeatureTypes_TransitiveThroughSpecialization(t *testing.T) {
	const src = `
		class A; class B disjoint from A;
		class A1 :> A; class B1 :> B; class B2 :> B1;
		class X { feature f : A1; }
		class Y :> X { feature :>> f : B2; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "A1", "B2")
}

// A subsetting feature's types are intersected with the subsetted feature's
// (KerML 1.1 §8.3.3.3.4), so the same rule applies to subsetting.
func TestDisjointFeatureTypes_Subsetting(t *testing.T) {
	const src = `
		class A; class B disjoint from A;
		class X {
			feature f : A;
			feature g : B :> f;
		}`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "g", "B", "A")
}

// A feature typed directly by two disjoint types has no values either.
func TestDisjointFeatureTypes_TwoTypings(t *testing.T) {
	const src = `
		class A; class B disjoint from A;
		class X { feature f : A, B; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "A", "B")
}

// Occurrences.kerml declares Occurrence disjoint from DataValue (KerML 1.1 §8.4.4.2,
// §8.4.4.3), so a class and a data type are disjoint with no disjoining in the model.
func TestDisjointFeatureTypes_LibraryDisjoiningClassFromDataType(t *testing.T) {
	const src = `
		datatype D; class C;
		class X { feature f : D; }
		class Y :> X { feature :>> f : C; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "D", "C")
}

// The same pair in SysML: an attribute definition classifies data values and a
// part definition classifies occurrences.
func TestDisjointFeatureTypes_LibraryDisjoiningInSysML(t *testing.T) {
	const src = `
		attribute def D; part def P;
		part def X { ref r : D; }
		part def Y :> X { ref :>> r : P; }`
	wantDisjointReport(t, disjointReports(t, "a.sysml", src), "r", "D", "P")
}

// A conforming redefinition is one type, and sibling members of a hierarchy with
// no disjoining are merely unrelated: neither is reported.
func TestDisjointFeatureTypes_ConformingAndUnrelatedHierarchiesStaySilent(t *testing.T) {
	const src = `
		class A; class A1 :> A; class A2 :> A;
		class X { feature f : A1; }
		class Y :> X { feature :>> f : A2; }
		class Z :> X { feature :>> f : A; }`
	if diags := disjointReports(t, "a.kerml", src); len(diags) != 0 {
		t.Fatalf("expected no report, got %v", diags)
	}
}

// connectedPortDefs declares ports whose definitions are unrelated (P1, P2),
// disjoint (P1, P3), conjugate (P1, ~P1) or siblings (D1, D2 under Base).
const connectedPortDefs = `package P {
	item def V;
	port def P1 { out item a : V; }
	port def P2 { in item b : V; }
	port def P3 disjoint from P1 { in item c : V; }
	port def Base; port def D1 :> Base; port def D2 :> Base;
	interface def Link { end a : P1; end b : ~P1; }
	connection def Pipe { end a : P1; end b : P2; }
	part def X { port p1 : P1; port d1 : D1; }
	part def Y { port p2 : P2; port p3 : P3; port p1c : ~P1; port p1s : P1; port d2 : D2; }
	part sys { part x : X; part y : Y; %s }
}`

// The specification constrains a connector's ends by featuring and arity only
// (KerML 1.1 §8.3.4.5.3), and SysML v2 §7.12.2 defines port conformance as the
// condition under which flows can pair every directed feature, not as a rule;
// the two ends of a link are distinct features, so even disjoint definitions
// at the two ends of a `connect` leave the link satisfiable.
func TestDisjointFeatureTypes_ConnectedPortsStaySilent(t *testing.T) {
	cases := map[string]string{
		"unrelated definitions":                `connect x.p1 to y.p2;`,
		"unrelated definitions, interface":     `interface connect x.p1 to y.p2;`,
		"unrelated definitions, flow":          `flow from x.p1 to y.p2;`,
		"disjoint definitions":                 `connect x.p1 to y.p3;`,
		"disjoint definitions, flow":           `flow from x.p1 to y.p3;`,
		"conjugate":                            `connect x.p1 to y.p1c;`,
		"same definition":                      `connect x.p1 to y.p1s;`,
		"common definition":                    `connect x.d1 to y.d2;`,
		"typed interface, conjugate ends":      `interface : Link connect x.p1 to y.p1c;`,
		"typed connection, unrelated ends":     `connection : Pipe connect x.p1 to y.p2;`,
		"typed connection end over conjugate":  `connection : Pipe connect y.p1c to y.p2;`,
		"typed interface end over common base": `interface : Link connect x.p1 to y.p1c;`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			src := fmt.Sprintf(connectedPortDefs, body)
			if diags := disjointReports(t, "a.sysml", src); len(diags) != 0 {
				t.Fatalf("reported: %v", diags)
			}
		})
	}
}

// An end of a connector typed by an interface or connection definition redefines
// that definition's end (SysML v2 §7.13.2) and references the port it attaches
// to, so it is typed by both (KerML 1.1 §8.3.3.3.4): disjoint, it has no values.
func TestDisjointFeatureTypes_TypedConnectorEndOverDisjointPort(t *testing.T) {
	cases := []struct {
		name, body, subject string
	}{
		{"interface", `interface : Link connect y.p3 to y.p1c;`, "the end of this interface at y.p3"},
		{"named connection", `connection c : Pipe connect y.p3 to y.p2;`, "the end of connection c at y.p3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(connectedPortDefs, tc.body)
			wantDisjointReport(t, disjointReports(t, "a.sysml", src), tc.subject, "P1", "P3")
		})
	}
}
