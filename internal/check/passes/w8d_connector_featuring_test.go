package passes

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The reference reports each end that names a feature of a nested feature
// (Connector_Invalid.sysml.xt, BindingConnector_redefine.sysml.xt).
func TestW8DConnectorEndMustBeAccessible(t *testing.T) {
	const src = `package P {
		part def A {
			part x;
		}
		part def B {
			part y {
				part z;
			}
			connect A::x to y::z;
		}
		part b {
			part y : A {
				connect x to z;
			}
			part z;
			connect y::x to z;
		}
	}`
	w8dWantLines(t, src, "connector-type-featuring", 9, 9, 13, 16)
}

// A connector's end names a feature; a definition is a type, which the
// reference reports as no feature at all (Couldn't resolve reference to Feature).
func TestW8DConnectorEndMustBeAFeature(t *testing.T) {
	const src = `package P {
		part def A;
		action def B;
		part a : A;
		action b : B;
		allocate A to B;
		allocate a to b;
		connect A to a;
		allocation def AB { end x : A; end y : B; }
	}`
	w8dWantLines(t, src, "connector-end-referent", 6, 6, 8)
}

func TestW8DBindingEndMustBeAccessible(t *testing.T) {
	const src = `package P {
		port def P0 {
			in ref myDefIn;
		}
		part def B0 {
			port p0 : P0;
		}
		part v {
			part b0 : B0;
			bind B0::p0.myDefIn = b0.p0.myDefIn;
		}
	}`
	w8dWantLines(t, src, "connector-type-featuring", 10)
}

// Ends named with dot notation, and ends of a variant of a variation, are
// accessible, so a legal model stays silent.
func TestW8DAccessibleConnectorEndsStaySilent(t *testing.T) {
	const src = `package P {
		port def PingPort;
		part def A {
			part x;
		}
		part def B {
			port outPort : PingPort;
			port inPort : PingPort;
			port bypass : PingPort;
			variation interface link {
				variant interface direct connect outPort to inPort;
				variant interface indirect connect outPort to bypass;
			}
		}
		part b {
			part y : A;
			part z;
			connect y.x to z;
			bind y.x = z;
		}
	}`
	if diags := only(w8dDiags(t, src), "connector-type-featuring"); len(diags) != 0 {
		t.Fatalf("legal connector model reported: %v", diags)
	}
}

// An end resolves in the connector's enclosing scope, so an interface whose
// type declares ends named like the parts it connects still names those parts.
func TestW8DEndNamedLikeAnInheritedEndStaysSilent(t *testing.T) {
	const src = `package P {
		port def UPort;
		interface def UInterface {
			end plss : UPort;
			end psa : ~UPort;
		}
		part def PLSS { port umbilicalPort : UPort; }
		part def PSA { port umbilicalPort : ~UPort; }
		part def EMU {
			part psa : PSA;
			part plss : PLSS;
			interface suitToPLSS : UInterface connect plss.umbilicalPort to psa.umbilicalPort;
		}
	}`
	if diags := only(w8dDiags(t, src), "connector-type-featuring"); len(diags) != 0 {
		t.Fatalf("legal interface model reported: %v", diags)
	}
}

// A package-level connector has no featuring type, so it reaches only
// features that have none: an end spelled `Definition::feature` names a
// feature of the definition, which the pilot reports on both ends.
func TestW8DPackageLevelConnectorEndThroughDefinition(t *testing.T) {
	const src = `package P {
		action def ApplyHeat;
		action def ToastBread { action applyHeat : ApplyHeat; }
		part def HeatingSystem;
		part def Toaster {
			part heating : HeatingSystem;
			perform action toastBread : ToastBread;
		}
		allocation heatAllocation allocate ToastBread::applyHeat to Toaster::heating;
	}`
	w8dWantLines(t, src, "connector-type-featuring", 9, 9)
}

// The same shape on every connector kind: connect, bind, flow, interface, and
// a KerML connector, binding and flow.
func TestW8DPackageLevelConnectorKindsEndThroughDefinition(t *testing.T) {
	const src = `package P {
		port def Pt;
		attribute def Sig;
		part def A { part x; port p : Pt; attribute v : Sig; }
		part def B { part y; port q : Pt; attribute w : Sig; }
		connect A::x to B::y;
		bind A::v = B::w;
		flow from A::v to B::w;
		interface def I { end e1 : Pt; end e2 : Pt; }
		interface i : I connect A::p to B::q;
		allocate A::x to B::y;
		part def Other {
			connect A::x to B::y;
		}
	}`
	w8dWantLines(t, src, "connector-type-featuring", 6, 6, 7, 7, 8, 8, 10, 10, 11, 11, 13, 13)

	const kerml = `package K {
		class A { feature x; }
		class B { feature y; }
		connector k1 from A::x to B::y;
		binding k3 of A::x = B::y;
		flow k4 from A::x to B::y;
	}`
	root := parser.New(source.New("<t>.kerml", []byte(kerml))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.kerml", root)
	var lines []int
	for _, d := range only(Analyze("<t>.kerml", root, nil, idx), "connector-type-featuring") {
		lines = append(lines, w8dLine(kerml, d.Span))
	}
	if want := []int{4, 4, 5, 5, 6, 6}; !slices.Equal(lines, want) {
		t.Fatalf("kerml: got lines %v, want %v", lines, want)
	}
}

// The legal shapes analyse clean at every tier, so no lower-tier error can
// mask a false positive: the nested dot-notation form of the allocation above,
// package-level connectors between package-level usages, ends reached through
// a nested package, inherited ends, the end parameters of an interface or
// connection definition, a variant named through its variation, and an
// enumeration literal.
func TestW8DPackageLevelConnectorLegalEndsStaySilent(t *testing.T) {
	cases := map[string]string{
		"nested dot notation": `package P {
			action def ApplyHeat;
			action def ToastBread { action applyHeat : ApplyHeat; }
			part def HeatingSystem;
			part def Toaster {
				part heating : HeatingSystem;
				perform action toastBread : ToastBread;
				allocation heatAllocation allocate toastBread.applyHeat to heating;
			}
		}`,
		"package-level usages": `package P {
			port def Pt;
			attribute def Sig;
			part def A { part x; port p : Pt; attribute v : Sig; }
			part def B { part y; port q : Pt; attribute w : Sig; }
			part a : A;
			part b : B;
			connect a.x to b.y;
			bind a.v = b.w;
			flow from a.v to b.w;
			interface def I { end e1 : Pt; end e2 : Pt; }
			interface i : I connect a.p to b.q;
			allocate a.x to b.y;
			allocate a to b;
			action def Act { action sub; }
			action act : Act;
			allocate act.sub to a.x;
			part sys {
				part a2 : A;
				part b2 : B;
				connect a2.x to b2.y;
			}
			connect sys.a2.x to sys.b2.y;
			connect P::a.x to P::b.y;
			connect P::sys.a2 to b;
		}`,
		"nested package": `package P {
			part def A { part x; }
			package Q { part qa : A; }
			part a : A;
			connect Q::qa.x to a.x;
			connect Q::qa to a;
		}`,
		"inherited and own ends": `package P {
			port def Pt;
			part def A { part x; port p : Pt; }
			part def Sub :> A {
				connect x to p;
				connect A::x to p;
				connect Sub::x to p;
			}
		}`,
		"definition end parameters": `package P {
			port def Pt;
			part def A;
			part def B;
			interface def I { end e1 : Pt; end e2 : Pt; connect e1 to e2; }
			interface def J :> I { connect I::e1 to e2; }
			connection def CD { end ea : A; end eb : B; }
			part a : A;
			part b : B;
			connection cd : CD connect a to b;
		}`,
		"variant and enumeration literal": `package P {
			enum def Color { red; green; }
			part def A { part x; attribute c : Color; }
			part def B { part y; }
			variation part def VA :> A { variant part va1 : A; variant part va2 : A; }
			part a : A;
			part b : B;
			bind a.c = Color::red;
			connect VA::va1.x to b.y;
		}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			for _, d := range w8dDiags(t, src) {
				if d.Severity == diag.SeverityError {
					t.Errorf("line %d: %s [%s]", w8dLine(src, d.Span), d.Message, d.Code)
				}
			}
		})
	}
}
