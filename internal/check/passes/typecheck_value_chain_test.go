package passes

import "testing"

// A feature chain is typed by its last feature, so a chained value is judged
// against the target the same way a plain name is. The pinned pilot reports
// every negative shape below as "Bound features should have conforming types".
func TestValueChainTypeIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A { part b : B; }
		part def B { part c : C; }
		part def C;
		part def D;
		part a : A;
		part x : D = a.b.c;
	}`, "cannot bind a value of type C to a feature typed by D")
}

func TestValueChainConformingIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		part def A { part b : B; }
		part def B { part c : C; }
		part def C0;
		part def C :> C0;
		part a : A;
		part x : C = a.b.c;
		part y : C0 = a.b.c;
		part z : B = a.b;
	}`)
}

// A one-segment chain is typed as a plain name, so the pre-existing behaviour
// for `= c` is unchanged.
func TestValuePlainNameStillJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def C;
		part def D;
		part c : C;
		part x : D = c;
	}`, "cannot bind a value of type C to a feature typed by D")
}

// A chain segment found by inheritance is typed where it was declared.
func TestValueChainThroughInheritedFeature(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A0 { part b : B; }
		part def A :> A0;
		part def B { part c : C; }
		part def C;
		part def D;
		part a : A;
		part x : D = a.b.c;
		part ok : C = a.b.c;
	}`, "cannot bind a value of type C to a feature typed by D")
}

// A redefinition narrows a segment's type, and the rest of the chain resolves
// through the redefining feature's type.
func TestValueChainThroughRedefinedFeature(t *testing.T) {
	wantOneDiag(t, `package P {
		part def B0 { part c : C0; }
		part def B :> B0 { part :>> c : C; }
		part def C0;
		part def C :> C0;
		part def D;
		part def A0 { part b : B0; }
		part def A :> A0 { part :>> b : B; }
		part a : A;
		part x : D = a.b.c;
		part ok : C0 = a.b.c;
		part ok2 : C = a.b.c;
	}`, "cannot bind a value of type C to a feature typed by D")
}

// A redefinition that declares no type of its own keeps the redefined
// feature's members, so the chain continues through it.
func TestValueChainThroughUntypedRedefinition(t *testing.T) {
	wantOneDiag(t, `package P {
		part def B { part c : C; }
		part def C;
		part def D;
		part def A0 { part b : B; }
		part def A :> A0 { part :>> b; }
		part a : A;
		part x : D = a.b.c;
	}`, "cannot bind a value of type C to a feature typed by D")
}

func TestValueChainThroughPort(t *testing.T) {
	wantOneDiag(t, `package P {
		part def C;
		part def D;
		port def Pt { ref item payload : C; }
		part def A { port p : Pt; }
		part a : A;
		item x : D = a.p.payload;
		item ok : C = a.p.payload;
	}`, "cannot bind a value of type C to a feature typed by D")
}

func TestValueChainThroughConjugatedPort(t *testing.T) {
	wantOneDiag(t, `package P {
		part def C;
		part def D;
		port def Pt { in ref item payload : C; }
		part def A { port p : ~Pt; }
		part a : A;
		item x : D = a.p.payload;
	}`, "cannot bind a value of type C to a feature typed by D")
}

func TestValueChainThroughConnectionEnd(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A { part b : B; }
		part def B;
		part def D;
		connection def Conn { end e1 : A; end e2 : B; }
		part a : A;
		part bb : B;
		connection conn : Conn connect a to bb;
		part x : D = conn.e1.b;
		part ok : B = conn.e1.b;
		part ok2 : B = conn.e2;
	}`, "cannot bind a value of type B to a feature typed by D")
}

// A chain whose segments are collection elements is judged element by element.
func TestValueChainInCollectionElements(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A { part b : B; part d : D; }
		part def B;
		part def D;
		part a : A;
		part xs : B[*] = (a.b, a.d);
	}`, "cannot bind a value of type D to a feature typed by B")
}

// `a#(i)` names one element of a, of a's own type, so an indexed segment or
// value is judged like the sequence it selects from; `a[i]` is an expression.
func TestValueIndexedChainIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A { part b : B[*]; }
		part def B { part c : C; }
		part def C;
		part def D;
		part a : A[*];
		part x : D = a#(1).b#(1).c;
		part ok : C = a#(1).b#(1).c;
		part ok2 : B = a.b#(1);
	}`, "cannot bind a value of type C to a feature typed by D")
}

func TestValueIndexedElementIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A;
		part def D;
		part a : A[*];
		part x : D = a#(1);
		part ok : A = a#(1);
	}`, "cannot bind a value of type A to a feature typed by D")
}

// An unresolved or untyped segment leaves the chain unjudged.
func TestValueChainUnresolvedIsNotJudged(t *testing.T) {
	wantNoDiags(t, `package P {
		part def A { part b : B; part untyped; }
		part def B;
		part def D;
		part a : A;
		part x : D = a.b.missing;
		part y : D = a.untyped;
	}`)
}

// Every consumer of the value-typing query sees the chain's type: a condition
// naming a chained part is reported as non-Boolean like a plain part is.
func TestChainedNonScalarConditionIsReported(t *testing.T) {
	wantOneDiag(t, `package P {
		part def A { part b : B; }
		part def B;
		part a : A;
		action def Act {
			if a.b { action x; }
		}
	}`, "must be Boolean, found B")
}

// A variant is implicitly typed by its variation (SysML v2 §7.20), so selecting
// one as the value of a feature typed by the variation conforms whether the
// variant is named outright or reached through a chain. The pinned pilot
// accepts every shape below.
func TestValueVariantConformsToItsVariation(t *testing.T) {
	wantNoDiags(t, `package P {
		part def Base;
		variation part def V {
			variant part v1 : Base;
			variant part v2 : Base;
		}
		part def Sel {
			part vp : V default = V::v1;
			part vq : V = vq.v2;
			part vr : V;
			part w : V = vr.v1;
		}
		part def Sel2 :> Sel {
			part :>> vp = V::v2;
		}
		variation attribute def A {
			variant attribute a1;
			variant attribute a2;
		}
		part def Sel3 {
			attribute ap : A = A::a1;
		}
	}`)
}

// A variant's declared type still has to relate to the target when the target
// is not the variation itself.
func TestValueVariantOfUnrelatedTypeIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Base;
		part def Other;
		variation part def V {
			variant part v1 : Base;
		}
		part def Sel {
			part vp : V;
			part o : Other = vp.v1;
		}
	}`, "cannot bind a value of type Base to a feature typed by Other")
}
