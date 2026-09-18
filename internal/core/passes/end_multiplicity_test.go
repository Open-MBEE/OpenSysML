package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const endFeatureMultiplicityCode = "end-feature-multiplicity"

// endMultiplicityTexts is the source text under each end-multiplicity warning.
func endMultiplicityTexts(t *testing.T, diags []diag.Diagnostic, src string) []string {
	t.Helper()
	var out []string
	for _, d := range only(diags, endFeatureMultiplicityCode) {
		if d.Severity != diag.SeverityWarning {
			t.Errorf("severity = %v, want a warning", d.Severity)
		}
		if d.Message != msgEndFeatureMultiplicity {
			t.Errorf("message = %q, want %q", d.Message, msgEndFeatureMultiplicity)
		}
		out = append(out, strings.TrimSpace(spanText(src, d)))
	}
	return out
}

func wantEndMultiplicity(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics, want %d:\n got  %q\n want %q", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("diagnostic %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A KerML end whose own multiplicity is not exactly one warns, whatever the
// spelling: a range, a single bound, `*`, or a `multiplicity` body member.
func TestKerMLEndOwnMultiplicityNotOne(t *testing.T) {
	const src = `package P {
		classifier A;
		feature n : ScalarValues::Natural;
		feature one : ScalarValues::Natural = 1;
		feature two : ScalarValues::Natural = 2;
		assoc R {
			end feature a : A [1];
			end feature b : A [1..1];
			end feature c : A [2];
			end feature d : A [0..1];
			end feature e : A [*];
			end feature f : A [1..*];
			end feature g : A [1] ordered;
			end feature h : A [one];
			end feature i : A [two];
			end feature j : A [n];
			end feature k : A [n..1];
			end feature l : A [1..n];
			end feature m : A { multiplicity [1]; }
			end feature o : A { multiplicity [2]; }
			end feature p : A { multiplicity :> exactlyOne; }
			end feature q : A { multiplicity :> zeroOrMore; }
		}
		multiplicity exactlyOne [1];
		multiplicity zeroOrMore [0..*];
	}`
	got := endMultiplicityTexts(t, constraintDiagsKerML(t, src), src)
	wantEndMultiplicity(t, got, []string{
		"end feature c : A [2];",
		"end feature d : A [0..1];",
		"end feature e : A [*];",
		"end feature f : A [1..*];",
		"end feature i : A [two];",
		"end feature j : A [n];",
		"end feature l : A [1..n];",
		"end feature o : A { multiplicity [2]; }",
		"end feature q : A { multiplicity :> zeroOrMore; }",
	})
}

// An end's own multiplicity is the only one it has (KerML 1.1 Type::multiplicities),
// so a stated non-one range warns even when a redefined or subsetted end is one.
func TestKerMLEndOwnMultiplicityShadowsGenerals(t *testing.T) {
	const src = `package P {
		class A;
		assoc G {
			end feature a1 : A [1];
			end feature a2 : A [1];
		}
		assoc S specializes G {
			end feature b1 : A [0..*] redefines a1;
			end feature b2 : A [2] subsets G::a2;
		}
	}`
	got := endMultiplicityTexts(t, constraintDiagsKerML(t, src), src)
	wantEndMultiplicity(t, got, []string{
		"end feature b1 : A [0..*] redefines a1;",
		"end feature b2 : A [2] subsets G::a2;",
	})
}

// A KerML end declaring no multiplicity takes its generals': the feature it
// subsets, references, redefines, chains to, or is typed by, and the end at its
// position of each type its owner specializes. Any one of them being exactly one
// suffices; none being so warns. A specialization cycle terminates and warns. A
// binary association's ends redefine `Links::BinaryLink`'s (`[1]`) and are silent.
func TestKerMLEndInheritedMultiplicity(t *testing.T) {
	const src = `package P {
		classifier A;
		classifier K {
			feature one : A [1];
			feature many : A [*];
			feature onePlus : A [1..*];
		}
		feature k : K;
		feature f1 : A [1];
		feature fn : A [*];
		classifier One [1];
		classifier Many [*];
		assoc Base { end feature x : A [1]; end feature y : A [*]; }
		assoc R {
			end feature a : A;
			end feature b :> f1;
			end feature c :> fn;
			end feature d references f1;
			end feature e references fn;
			end feature f chains k.one;
			end feature g chains k.many;
			end feature h : One;
			end feature i : Many;
			end feature j :> fn, f1;
			end feature l :> k.onePlus;
		}
		assoc S :> Base {
			end feature x1 redefines x;
			end feature y1 redefines y;
		}
		assoc T :> Base {
			end feature p : A;
			end feature q : A;
			end feature r : A;
		}
		assoc U :> Base {
			end feature p : A [1];
			end feature q : A [1];
		}
		assoc V :> Base {
			end feature p;
			end feature q [1];
		}
		classifier Cyc :> Cyc2;
		classifier Cyc2 :> Cyc;
		assoc W { end feature z : Cyc; end feature w : Cyc2; end feature v : Cyc; }
		feature fc :> fc2;
		feature fc2 :> fc;
		assoc W2 { end feature z2 :> fc; end feature w2 :> fc2; end feature v2 :> fc; }
		assoc B2 { end feature a : A; end feature b : A; }
		classifier C2 { end feature a : A; end feature b : A; }
	}`
	got := endMultiplicityTexts(t, constraintDiagsKerML(t, src), src)
	wantEndMultiplicity(t, got, []string{
		"end feature y : A [*];",
		"end feature a : A;",
		"end feature c :> fn;",
		"end feature e references fn;",
		"end feature g chains k.many;",
		"end feature i : Many;",
		"end feature l :> k.onePlus;",
		"end feature y1 redefines y;",
		"end feature q : A;",
		"end feature r : A;",
		"end feature z : Cyc;",
		"end feature w : Cyc2;",
		"end feature v : Cyc;",
		"end feature z2 :> fc;",
		"end feature w2 :> fc2;",
		"end feature v2 :> fc;",
		"end feature a : A;",
		"end feature b : A;",
	})
}

// A KerML connector's ends redefine by position the ends of the association it
// is typed by, or of `Links::binaryLinks` (`source`/`target`, both `[1]`) when
// it is untyped; a written `[m]` is the cross feature's, never the end's own.
func TestKerMLConnectorEndMultiplicity(t *testing.T) {
	const src = `package P {
		classifier A;
		classifier K {
			feature one : A [1];
			feature many : A [*];
		}
		feature k : K;
		struct S {
			feature a : A [*];
			feature b : A [1];
			step s1 { out feature x : A [*]; }
			step s2 { in feature y : A [*]; }
			connector c1 (a, b);
			connector c2 (a, a);
			binding bd of a = b;
			succession sc first a then b;
			succession sc2 first s1 then s2;
			flow fl from s1.x to s2.y;
			connector c3 (e1 references a, e2 references b);
			connector c4 (e3 references a, e4 references a);
			connector c5 ([1] a, [*] a);
			connector c6 ([1] m1 references a, [*] m2 references a);
			connector c7 { end feature p references a; end feature q references b; }
			connector c8 { end feature p [1] references a; end feature q [1] references a; }
		}
		assoc As { end feature x : A [1]; end feature y : A [*]; }
		connector c9 : As from k.many to k.many;
		connector c10 : As from k.one to k.one;
		connector c11 : As (k.many, k.many, k.many);
	}`
	got := endMultiplicityTexts(t, constraintDiagsKerML(t, src), src)
	wantEndMultiplicity(t, got, []string{
		"end feature y : A [*];",
		"k.many",
		"k.many",
	})
}

// A SysML end usage has the default multiplicity 1..1 when it declares none, so
// only a declared own multiplicity other than exactly one warns. The crossing
// `[m]` before the end's keyword is the cross feature's and is silent.
func TestSysMLEndUsageMultiplicity(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		attribute one : ScalarValues::Natural = 1;
		attribute two : ScalarValues::Natural = 2;
		part x : A [1];
		part y : B [*];
		connection def CD {
			end part a : A;
			end part b : B;
		}
		connection def CD2 {
			end part a : A [1];
			end part b : B [*];
			end part c : A [0..1];
			end part d : A [2];
			end part e : A [1..1] ordered;
			end part f : A [one];
			end part g : A [two];
		}
		connection def CD3 :> CD2 {
			end part a;
			end part b;
			end part c;
		}
		connection def CD4 :> CD2 {
			end part a : A [2];
			end part b : B [1];
		}
		connection def CD5 {
			end [0..*] part a : A;
			end [1] part b : B;
			end [0..1] item cart : A [1];
			end [0..1] item line : A [*];
			end a1 [2..*] : A;
		}
		interface def ID { end port p : A; end port q : A [*]; }
		part def K {
			end part e : A;
			end part f : A :> x;
			end part g : B :> y;
			end part h : A [3];
			end item i : A [0..*];
			end ref j : A [1];
			end k : A [2..5];
		}
		connection c1 : CD connect x to y;
		connection c2 connect (x, y);
		connection c3 : CD2 connect (x, y, x);
		connection c4 connect (a4 ::> x, b4 ::> y);
		connection c5 connect x to y;
		connection c6 connect ([1] x, [*] y);
	}`
	got := endMultiplicityTexts(t, constraintDiags(t, src), src)
	wantEndMultiplicity(t, got, []string{
		"end part b : B [*];",
		"end part c : A [0..1];",
		"end part d : A [2];",
		"end part g : A [two];",
		"end part a : A [2];",
		"end [0..1] item line : A [*];",
		"end a1 [2..*] : A;",
		"end port q : A [*];",
		"end part h : A [3];",
		"end item i : A [0..*];",
		"end k : A [2..5];",
	})
}
