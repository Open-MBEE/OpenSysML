package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// unresolvedNames returns the name each unresolved diagnostic reports, in order.
func unresolvedNames(t *testing.T, got []diag.Diagnostic) []string {
	t.Helper()
	var out []string
	for _, d := range got {
		if d.Code != "unresolved" {
			t.Fatalf("unexpected diagnostic %+v", d)
		}
		_, name, _ := strings.Cut(d.Message, ": ")
		name, _, _ = strings.Cut(name, " ")
		out = append(out, name)
	}
	return out
}

// An alias of a redefined feature names nothing in the redefining type; the unnamed
// redefinition answers to its name and short name (KerML 8.3.3.1.10; pilot alike).
func TestAliasOfRedefinedFeatureIsUnresolvedInTheRedefiningType(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def A { part <s> a; alias aa for a; }
		part def B :> A { part :>> a; }
		part x : B;
		part viaName :> x.a;
		part viaShort :> x.s;
		part viaChain :> x.aa;
		part viaQualified :> B::aa;
		part def C :> B { part viaBare :> aa; }
	}`)
	if names := unresolvedNames(t, got); strings.Join(names, " ") != "aa B::aa aa" {
		t.Fatalf("unresolved %v, want the three alias references only", names)
	}
}

// An inherited feature redefining what an owned feature directly redefines is
// removed with it (KerML 8.3.3.1.10 removeRedefinedFeatures, condition 2).
func TestInheritedRedefinitionOfADirectlyRedefinedFeatureIsUnresolved(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def A { part a; }
		part def B :> A { part b :>> a; }
		part def C :> B { part c :>> A::a; }
		part w : C;
		part kept :> w.c;
		part viaChain :> w.b;
		part viaQualified :> C::b;
	}`)
	if names := unresolvedNames(t, got); strings.Join(names, " ") != "b C::b" {
		t.Fatalf("unresolved %v, want b through C only", names)
	}
}

// A private redefinition and one a general itself lost leave the feature inherited
// along another specialization (KerML 8.3.3.1.10 inheritableMemberships; pilot alike).
func TestFeatureStaysInheritedPastARedefinitionNotPassedOn(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def A { part a; part a2; }
		part def Private :> A { private part b :>> a; }
		part def Plain :> A;
		part def E :> Private, Plain;
		part e : E;
		part keptPastPrivate :> e.a;
		part def B :> A { part b :>> a, a2; }
		part def C :> B { part c :>> A::a; }
		part def D :> C, Plain;
		part d : D;
		part keptPastLost :> d.a2;
		part removed :> d.a;
	}`)
	if names := unresolvedNames(t, got); strings.Join(names, " ") != "a" {
		t.Fatalf("unresolved %v, want d.a only", names)
	}
}
