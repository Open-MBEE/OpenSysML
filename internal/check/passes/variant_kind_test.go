package passes

import (
	"strings"
	"testing"
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
