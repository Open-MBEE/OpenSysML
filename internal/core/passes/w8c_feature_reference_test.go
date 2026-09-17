package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// w8cLibraryDiagnostics analyzes src as the named document against the standard
// library.
func w8cLibraryDiagnostics(t *testing.T, name, src string) []Diagnostic {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	return Analyze(name, root, nil, idx)
}

func w8cLibraryMessagesIn(t *testing.T, name, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		out = append(out, d.Message)
	}
	return out
}

func TestW8CFeatureReferenceAccessibleAndValid(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V {
		feature n : Integer;
		struct S;
	}
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
	feature v1_S : Integer = v1.S;
	feature v1_V : Integer = V;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
		t.Errorf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
	}
	if w8cCount(msgs, msgReferentIsFeature) != 2 {
		t.Errorf("want two %q, got %v", msgReferentIsFeature, msgs)
	}
}

func TestW8CFeatureReferenceLocation(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
}`
	var spans []string
	for _, d := range w8cLibraryDiagnostics(t, "<t>.kerml", src) {
		if d.Message == msgSubsettingFeaturingTypes {
			spans = append(spans, strings.TrimSpace(src[d.Span.Offset:d.Span.End()]))
		}
	}
	if len(spans) != 1 || spans[0] != "v1::n" {
		t.Errorf("want the diagnostic at \"v1::n\", got %v", spans)
	}
}

func TestW8CFeatureReferenceEnumerationLiteralLegal(t *testing.T) {
	src := `package P {
	enum def Color { enum red; enum green; }
	attribute c : Color = Color::red;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
}

func TestW8CFeatureReferenceLegal(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1.n;
	feature m : Integer;
	feature m2 : Integer = m;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
	if w8cCount(msgs, msgReferentIsFeature) != 0 {
		t.Errorf("unexpected %q: %v", msgReferentIsFeature, msgs)
	}
}

// F20 validateSubsettingFeaturingTypes: a body writes references the pilot
// checks for accessibility just as it checks a value expression, so a feature
// named through its owning type's namespace is not reachable there.
func TestW8CFeatureReferenceBodyInaccessible(t *testing.T) {
	cases := map[string]string{
		"constraint def body": `package P {
	part def Q { attribute n = 1; }
	constraint def C { P::Q::n > 0 }
}`,
		"asserted constraint": `package P {
	part def Q { attribute n = 1; }
	part def R { assert constraint { P::Q::n > 0 } }
}`,
		"calc return": `package P {
	part def Q { attribute n = 1; }
	calc def C { return x = P::Q::n; }
}`,
		"transition guard": `package P {
	part def Q { attribute n = 1; }
	state def S { state a; state b; transition first a if P::Q::n > 0 then b; }
}`,
		"require constraint": `package P {
	part def Q { attribute n = 1; }
	requirement def R { require constraint { P::Q::n > 0 } }
}`,
		"assume constraint": `package P {
	part def Q { attribute n = 1; }
	requirement def R { assume constraint { P::Q::n > 0 } }
}`,
		"require constraint with a parameter": `package P {
	part def Q { attribute n = 1; }
	requirement def R { require constraint c { in y = 1; P::Q::n > y } }
}`,
		"implicit calc result": `package P {
	part def Q { attribute n = 1; }
	calc def C { P::Q::n }
}`,
		"assignment value": `package P {
	part def Q { attribute n = 1; }
	action def A { attribute v = 0; assign v := P::Q::n; }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
			if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
				t.Errorf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
			}
		})
	}
}

// The traps the reverted attempt fell into: a body reaches its own type's
// features, inherited ones included, and a dot path reaches a nested one.
func TestW8CFeatureReferenceBodyAccessible(t *testing.T) {
	cases := map[string]string{
		"own feature": `package P {
	part def Q { attribute n = 1; constraint c { n > 0 } }
}`,
		"inherited feature": `package P {
	part def Base { attribute n = 1; }
	part def D :> Base { constraint c { n > 0 } }
}`,
		"redefined feature": `package P {
	part def Base { attribute n = 1; }
	part def D :> Base { attribute n redefines n; constraint c { n > 0 } }
}`,
		"dotted subject": `package P {
	part def Widget { attribute mass = 1; }
	requirement def R { subject s : Widget; require constraint { s.mass > 0 } }
}`,
		"dotted part path": `package P {
	part def Q { attribute n = 1; }
	part def R { part q : Q; constraint c { q.n > 0 } }
}`,
		"nested constraint parameter": `package P {
	part def W;
	requirement def R {
		subject s : W;
		in x = 1;
		require constraint q { in y = 2; y > 0 and x > 0 }
		assume constraint { in z = 3; z > x }
	}
}`,
		"typed nested constraint parameter": `package P {
	part def W;
	constraint def Q { in v = 1; }
	requirement def R { subject s : W; require constraint w : Q { in v = 2; v > 0 } }
}`,
		"implicit calc result": `package P {
	calc def C { attribute m = 1; m }
}`,
		"trigger parameter in a guard": `package P {
	attribute def Temp;
	state def S {
		state a;
		state b;
		transition first a accept sig : Temp if sig > 0 then b;
	}
}`,
		"state entry local": `package P {
	state def S {
		state a {
			entry action e { attribute v = 1; assign v := v + 1; }
		}
	}
}`,
		"accept payload from a sibling node": `package P {
	attribute def Temp;
	action def A {
		action receiver accept msg : Temp;
		action processor { if msg > 0 { } }
	}
}`,
		"connector ends": `package P {
	part def Port;
	part def A { part x : Port; part y : Port; connect x to y; }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
			if w8cCount(msgs, msgSubsettingFeaturingTypes) != 0 {
				t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
			}
		})
	}
}

func TestW8CFeatureReferenceFilterConditions(t *testing.T) {
	t.Run("user feature path is inaccessible", func(t *testing.T) {
		src := `package R1 {
	private import ScalarValues::*;
	metaclass M { var feature a : Boolean[1]; }
}
package Q { filter R1::M::a; }`
		msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
		if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
			t.Fatalf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
		}
	})

	for _, tc := range []struct {
		name string
		src  string
	}{
		{
			"library metaclass path",
			`package Q {
	private import KerML::*;
	filter Element::name == "System" and not Type::isAbstract;
}`,
		},
		{
			"dot notation on a cast",
			`package Q {
	private import ScalarValues::*;
	metaclass X { var feature f : ScalarValues::Boolean[1]; }
	filter (as X).f;
}`,
		},
		{
			"metadata classification",
			`metadata def Safety;
package Q { filter @Safety; }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if msgs := w8cLibraryMessagesIn(t, "<t>.kerml", tc.src); len(msgs) != 0 {
				t.Fatalf("expected no diagnostics, got %v", msgs)
			}
		})
	}
}
