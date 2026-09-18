package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// w8dDiags analyses src with the default registry, as the CLI does.
func w8dDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	return Analyze("<t>", root, nil, idx)
}

// w8dLine returns the 1-based line of a span in src.
func w8dLine(src string, span source.Span) int {
	return strings.Count(src[:span.Offset], "\n") + 1
}

// w8dLines returns the 1-based lines carrying a diagnostic with code.
func w8dLines(t *testing.T, src, code string) []int {
	t.Helper()
	var lines []int
	for _, d := range only(w8dDiags(t, src), code) {
		lines = append(lines, w8dLine(src, d.Span))
	}
	return lines
}

func w8dWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	got := w8dLines(t, src, code)
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", code, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", code, got, want)
		}
	}
}

func TestW8DVariationMemberMustBeVariant(t *testing.T) {
	const src = `package P {
		part def D;
		variation attribute def AttributeChoices {
			variant attribute a1;
			attribute a2;
		}
		part c {
			variation part d : D {
				part d1;
				variant part d2;
			}
		}
	}`
	w8dWantLines(t, src, "variation-member-not-variant", 5, 9)
}

func TestW8DVariationMustNotSpecializeVariation(t *testing.T) {
	const src = `package P {
		variation attribute def AttributeChoices {
			variant attribute a1;
		}
		variation attribute def AttributeChoices1 :> AttributeChoices;
		variation attribute a3 : AttributeChoices1 {
			variant attribute a4;
		}
	}`
	w8dWantLines(t, src, "variation-specialization", 5, 6)
}

// An enumeration definition is a variation without the modifier, so one
// specializing another, or a declared variation on either side, is reported.
func TestW8DEnumerationDefinitionIsAVariation(t *testing.T) {
	const src = `package P {
		enum def E { a; b; }
		enum def F :> E;
		variation attribute def W :> E;
		variation attribute def V { variant attribute x; }
		enum def G :> V;
		variation attribute ve : E;
		attribute def Plain :> E;
		enum def N :> Plain;
		attribute e : E;
	}`
	w8dWantLines(t, src, "variation-specialization", 3, 4, 6, 7)
	diags := only(w8dDiags(t, src), "variation-specialization")
	want := []string{
		"enumeration definition `F` must not specialize enumeration definition `E`",
		"variation `W` must not specialize enumeration definition `E`",
		"enumeration definition `G` must not specialize variation `V`",
		"variation `ve` must not be typed by enumeration definition `E`",
	}
	for i, d := range diags {
		if !strings.Contains(d.Message, want[i]) || !strings.Contains(d.Message, "every enumeration definition is a variation") {
			t.Errorf("diagnostic %d = %q, want it to name %q and explain the implicit variation", i, d.Message, want[i])
		}
	}
	if !strings.Contains(diags[0].Message, "declare the enumerated values of `F` directly") {
		t.Errorf("an enumeration specializing one should be told the fix: %q", diags[0].Message)
	}
}

// An enumeration definition's values are its variants; any other owned usage is
// the non-variant member of a variation, and `variant` is not written there.
func TestW8DEnumerationDefinitionMembers(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		enum def H {
			attribute y : Integer;
			c;
		}
		enum def E { variant v; w; }
	}`
	w8dWantLines(t, src, "variation-member-not-variant", 4)
	if d := only(w8dDiags(t, src), "variation-member-not-variant")[0]; !strings.Contains(d.Message, "enumeration definition `H` cannot own `y`") || !strings.Contains(d.Message, "declare `y` as an enumerated value") {
		t.Errorf("message = %q", d.Message)
	}
	w8dWantLines(t, src, "variant-outside-variation", 7)
	if d := only(w8dDiags(t, src), "variant-outside-variation")[0]; !strings.Contains(d.Message, "`variant` is not written in enumeration definition `E`") {
		t.Errorf("message = %q", d.Message)
	}
}

// Legal enumerations stay silent: literals with and without values, anonymous,
// short-named or nameless ones stating a typing or multiplicity, the `enum`
// keyword form, a scalar or attribute-definition general, KerML-only keywords
// as names, globally qualified typings, and usages typed by an enumeration.
func TestW8DLegalEnumerationStaysSilent(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		enum def E { a; b; }
		enum def K :> Real { A = 4.0; B = 3.0; C := 2.0; D default = 1.0; }
		enum def L { enum m; = 1; doc /* the levels */ }
		attribute def AD { attribute q : Integer; }
		enum def M :> AD { x; y; <s1> z : M; <s2>; : M; [1]; private <s3> w; }
		metadata def Tag;
		enum def T { #Tag a; #Tag b : T; #Tag enum c; private #Tag d; #Tag <e> ee; #Tag f [1]; #Tag g :> a; #Tag = 1; #Tag; }
		enum def W { chains; type : W; namespace default = 1; class :> chains; k : $::P::W; }
		attribute def Plain :> E;
		enum def N :> Plain;
		part def P1 { attribute e : E = E::a; enum f : E; attribute g : Real; }
		variation part def VP { variant part v1; }
		part def Q :> VP;
	}`
	for _, code := range []string{"variation-member-not-variant", "variation-specialization", "variant-outside-variation"} {
		if diags := only(w8dDiags(t, src), code); len(diags) != 0 {
			t.Fatalf("legal enumeration model reported %s: %v", code, diags)
		}
	}
}

// A variation whose every usage is a variant and whose general is no variation
// stays silent, as does a variant nested in a variant.
func TestW8DLegalVariationStaysSilent(t *testing.T) {
	const src = `package P {
		part def Base;
		attribute def Choice;
		variation part def PartChoices :> Base {
			doc /* the choices */
			variant part p1;
			variant part p2 {
				part inner;
			}
			part def Nested;
		}
		part c {
			variation part d : Base {
				variant part d1;
			}
		}
	}`
	for _, code := range []string{"variation-member-not-variant", "variation-specialization"} {
		if diags := only(w8dDiags(t, src), code); len(diags) != 0 {
			t.Fatalf("legal variation model reported %s: %v", code, diags)
		}
	}
}
