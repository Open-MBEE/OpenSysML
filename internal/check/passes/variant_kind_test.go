package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestVariantKindInconsistentWithVariation(t *testing.T) {
	const src = `package Mixed {
		part def P; action def A; attribute def V;
		variation part def PC :> P {
			variant action a1;
			variant part p1 : P;
		}
		variation part pv : P {
			variant action a2 : A;
			variant attribute x;
			variant part p2 : P;
		}
		variation action av : A {
			variant part p3 : P;
			variant action a3 : A;
			variant attribute v3;
		}
		variation part pu {
			variant action a4;
		}
		variation attribute def VC :> V {
			variant part p5;
		}
	}`
	w8dWantLines(t, src, "variant-kind", 4, 8, 9, 15, 18, 21)
	for _, want := range []string{
		"An action must be typed by action definitions. variant action `a1` specializes its owning variation part def `PC`.",
		"An attribute must be typed by attribute definitions. variant attribute `x` specializes its owning variation part `pv`, which is typed by part def `P`.",
		"An action must be typed by action definitions. variant action `a4` specializes its owning variation part `pu`, which is typed by part def `Part`.",
		"An occurrence, item or part must be typed by occurrence definitions. variant part `p5` specializes its owning variation attribute def `VC`.",
	} {
		found := false
		for _, d := range only(w8dDiags(t, src), "variant-kind") {
			if d.Message == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing message %q", want)
		}
	}
}

func TestVariantKindConsistentVariantsAreClean(t *testing.T) {
	const src = `package Clean {
		part def P; item def I; action def A; port def Q;
		part b1 : P;
		part smallEngine : P;
		variation part def PC :> P {
			variant b1;
			variant smallEngine;
			variant item i1;
			variant occurrence o1;
			variant ref part r1;
		}
		variation item def IC :> I {
			variant item i2;
		}
		variation action av : A {
			variant part p3 : P;
			variant item i3;
			variant action a3;
		}
		variation port def QC :> Q {
			variant port q4;
		}
		variation part pv : P {
			variant part p5;
			variant item i5;
			variant action a5 : P;
		}
	}`
	if got := w8dLines(t, src, "variant-kind"); len(got) != 0 {
		t.Fatalf("variant-kind on lines %v, want none", got)
	}
	if got := w8dLines(t, src, "usage-typing"); len(got) != 1 || got[0] != 26 {
		t.Fatalf("usage-typing on lines %v, want [26]", got)
	}
}

func TestVariantKindMessageNamesUnnamedVariation(t *testing.T) {
	const src = `package Anon {
		part def P;
		variation part : P {
			variant action a;
		}
	}`
	diags := only(w8dDiags(t, src), "variant-kind")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "owning variation part (unnamed)") {
		t.Fatalf("got %v", diags)
	}
}

// An occurrence-prefixed variant is an occurrence whatever its keyword, so a
// variation that is a data type cannot own one (validateOccurrenceUsageType).
func TestVariantKindOccurrencePrefixedVariants(t *testing.T) {
	const src = `package Occ {
		attribute def V; part def P; action def A;
		variation attribute def VC :> V {
			variant snapshot part s1;
			variant timeslice part t1;
			variant individual part i1;
		}
		variation attribute va : V {
			variant snapshot part s2;
		}
		variation part def PC :> P {
			variant snapshot part s3;
			variant timeslice item t3;
			variant individual part i3;
		}
		variation action av : A {
			variant snapshot action s4;
		}
	}`
	// An individual variant is the one-type rule's (neither VC nor PC is an
	// individual definition), and its error gates this pass off it.
	w8dWantLines(t, src, "variant-kind", 4, 5, 9)
	w8dWantLines(t, src, "individual-typing", 6, 14)
	want := "An occurrence, item or part must be typed by occurrence definitions. variant snapshot part `s1` specializes its owning variation attribute def `VC`."
	for _, d := range only(w8dDiags(t, src), "variant-kind") {
		if d.Message == want {
			return
		}
	}
	t.Errorf("missing message %q", want)
}

// A variation usage typed by a KerML type gives its variants that type, judged
// by the metaclass the keyword names (a behavior is a Class, not a DataType).
func TestVariantKindJudgesKerMLVariationTypes(t *testing.T) {
	const kerml = `package K { behavior B; datatype D; struct S; }`
	const src = `package A {
		private import K::*;
		variation part v : B {
			variant attribute va;
			variant part vp;
			variant action vact;
		}
		variation attribute x : D {
			variant part vx;
			variant attribute ax;
		}
		variation item w : S {
			variant item wi;
		}
	}`
	idx := newTestIndex()
	idx.AddDocument("k.kerml", parser.New(source.New("k.kerml", []byte(kerml))).ParseFile())
	root := parser.New(source.New("a.sysml", []byte(src))).ParseFile()
	idx.AddDocument("a.sysml", root)
	var got []int
	for _, d := range only(Analyze("a.sysml", root, nil, idx), "variant-kind") {
		got = append(got, w8dLine(src, d.Span))
	}
	if want := []int{4, 9}; !sameLines(got, want) {
		t.Fatalf("variant-kind on lines %v, want %v", got, want)
	}
}

func TestVariantKindReportedDespiteFaultyNestedMember(t *testing.T) {
	const src = `package Nested {
		part def P;
		variation part def PC :> P {
			variant action a1 { part q : Missing; }
			variant part p1 : P { part r : Missing; }
		}
	}`
	w8dWantLines(t, src, "unresolved", 4, 5)
	w8dWantLines(t, src, "variant-kind", 4)
}
