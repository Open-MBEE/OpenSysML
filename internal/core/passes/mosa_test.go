package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// mosaModel wraps body in a package importing the MOSA library.
func mosaModel(body string) string {
	return "package M {\n\tprivate import MOSA::*;" + body + "\n}"
}

// mosaPlatform is a platform of two components joined by one interface, with
// the ports the interface attaches to.
const mosaPlatform = `
		port def P;
		#majorSystemComponent part def A { port p : P; }
		#majorSystemComponent part def B { port p : P; }
		#modularSystemInterface interface def Link { end a : P; end b : P; }`

var mosaCodes = []string{
	CodeMOSAInterfaceNoStandard,
	CodeMOSAInterfaceNoControl,
	CodeMOSAInterfaceNotTraced,
	CodeMOSAComponentNoDataRights,
	CodeMOSAProprietaryNoRationale,
	CodeMOSABoundaryNotDesignated,
}

// An interface conforming to no standard is reported once the model declares
// standards; one at a #conformant end, or marked proprietary, is not.
func TestMOSAInterfaceNoStandard(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#technicalStandard item std : Standard;
		part v {
			part a : A; part b : B; part c : A;
			interface open : Link connect a.p to b.p;
			interface closed : Link connect a.p to c.p;
			interface vendor : Link connect b.p to c.p {
				@Proprietary { rationale = "qualified"; }
			}
			#conformance connection { end #conformant ::> open; end #conformsTo ::> std; }
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNoStandard, 11)
}

// A #conformance whose #conformsTo end names no standard — no such end, or one
// naming an element that is not a standard — does not cover its #conformant end.
func TestMOSAConformanceNeedsAStandard(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#technicalStandard item std : Standard;
		item def Note; item note : Note;
		part v {
			part a : A; part b : B;
			interface unnamed : Link connect a.p to b.p;
			interface misnamed : Link connect a.p to b.p;
			interface named : Link connect a.p to b.p;
			#conformance connection { end #conformant ::> unnamed; }
			#conformance connection { end #conformant ::> misnamed; end #conformsTo ::> note; }
			#conformance connection { end #conformant ::> named; end #conformsTo ::> note; end #conformsTo ::> std; }
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNoStandard, 11, 12)
}

// Conformance stated on an interface definition covers its usages, and no
// standard declared means no interface is asked for one.
func TestMOSAInterfaceStandardByDefinition(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#technicalStandard item std : Standard;
		part v {
			part a : A; part b : B;
			interface link : Link connect a.p to b.p;
		}
		interface bus : Link;
		#conformance connection { end #conformant ::> bus; end #conformsTo ::> std; }
		part w {
			part a : A; part b : B;
			interface l2 :> bus connect a.p to b.p;
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNoStandard, 10)
	w8dWantLines(t, mosaModel(mosaPlatform+`
		part v { part a : A; part b : B; interface link : Link connect a.p to b.p; }`),
		CodeMOSAInterfaceNoStandard)
}

// An interface with no @InterfaceControl is reported once any interface names
// one; the annotation on its definition serves.
func TestMOSAInterfaceNoControl(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#modularSystemInterface interface def Owned { end a : P; end b : P; @InterfaceControl { authority = "ICWG"; } }
		part v {
			part a : A; part b : B;
			interface l1 : Link connect a.p to b.p { @InterfaceControl { authority = "ICWG"; } }
			interface l2 : Link connect a.p to b.p;
			interface l3 : Owned connect a.p to b.p;
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNoControl, 11)
	w8dWantLines(t, mosaModel(mosaPlatform+`
		part v { part a : A; part b : B; interface link : Link connect a.p to b.p; }`),
		CodeMOSAInterfaceNoControl)
}

// An @InterfaceControl naming no authority — bare, or with an empty string —
// names no control authority; one whose metadata type defaults the authority does.
func TestMOSAInterfaceControlNeedsAnAuthority(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		metadata def ProgramControl :> InterfaceControl { :>> authority = "ICWG"; }
		part v {
			part a : A; part b : B;
			interface named : Link connect a.p to b.p { @InterfaceControl { authority = "ICWG"; } }
			interface bare : Link connect a.p to b.p { @InterfaceControl; }
			interface blank : Link connect a.p to b.p { @InterfaceControl { authority = ""; } }
			interface defaulted : Link connect a.p to b.p { @ProgramControl; }
			interface silent : Link connect a.p to b.p;
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNoControl, 11, 12, 14)
	w8dWantLines(t, mosaModel(mosaPlatform+`
		part v {
			part a : A; part b : B;
			interface bare : Link connect a.p to b.p { @InterfaceControl; }
			interface silent : Link connect a.p to b.p;
		}`), CodeMOSAInterfaceNoControl)
}

// An interface satisfying no requirement is reported once the model declares
// interface requirements; a satisfy naming it or its definition traces it.
func TestMOSAInterfaceNotTraced(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#interfaceRequirement requirement def R;
		requirement r1 : R; requirement r2 : R; requirement r3 : R;
		part v {
			part a : A; part b : B;
			interface l1 : Link connect a.p to b.p;
			interface l2 : Link connect a.p to b.p;
			interface l3 : Link connect a.p to b.p { satisfy r3; }
			interface l4 : Link connect a.p to b.p;
		}
		satisfy r1 by v.l1;
		not satisfy r2 by v.l4;`)
	w8dWantLines(t, src, CodeMOSAInterfaceNotTraced, 12, 14)
	w8dWantLines(t, mosaModel(mosaPlatform+`
		requirement def R; requirement r : R;
		part v { part a : A; part b : B; interface link : Link connect a.p to b.p; }`),
		CodeMOSAInterfaceNotTraced)
}

// A component with no @DataRights is reported once the model records any;
// the annotation on its definition serves, and platforms are not components.
func TestMOSAComponentNoDataRights(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#majorSystemComponent part def C { @DataRights { kind = DataRightsKind::unlimited; } }
		#majorSystemPlatform part v {
			part a : A { @DataRights { kind = DataRightsKind::limited; } }
			part b : B;
			part c : C;
			#modularSystem part m;
			part plain;
		}`)
	w8dWantLines(t, src, CodeMOSAComponentNoDataRights, 10, 12)
	w8dWantLines(t, mosaModel(mosaPlatform+`
		part v { part a : A; part b : B; }`), CodeMOSAComponentNoDataRights)
}

// A @Proprietary annotation states a non-empty rationale, wherever it is written.
func TestMOSAProprietaryNoRationale(t *testing.T) {
	src := mosaModel(`
		part def V { @Proprietary { owner = "vendor"; } }
		part v : V;
		part w { @Proprietary { owner = "vendor"; rationale = ""; } }
		part x { @Proprietary { rationale = "qualified with the vendor's isolation"; } }
		part y;
		metadata Proprietary about y;`)
	w8dWantLines(t, src, CodeMOSAProprietaryNoRationale, 3, 5, 7)
}

// An element with several @Proprietary annotations is judged by all of them:
// one stating a rationale suffices in either order, none stating one warns once.
func TestMOSAProprietaryRationaleAcrossAnnotations(t *testing.T) {
	src := mosaModel(`
		part a { @Proprietary { rationale = "qualified"; } @Proprietary { owner = "vendor"; } }
		part b { @Proprietary { owner = "vendor"; } @Proprietary { rationale = "qualified"; } }
		part c { @Proprietary { owner = "vendor"; } @Proprietary { owner = "vendor"; } }
		part d { @Proprietary { owner = "vendor"; } @Proprietary { rationale = ""; } }`)
	w8dWantLines(t, src, CodeMOSAProprietaryNoRationale, 5, 6)
}

// An undesignated connector between two distinct components is reported once
// the model designates any; those within one component or to a platform are not.
func TestMOSABoundaryNotDesignated(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#majorSystemPlatform part v {
			part a : A; part b : B;
			#modularSystem part m { port p : P; port q : P; }
			part plain { port p : P; }
			port p : P;
			interface link : Link connect a.p to b.p;
			connect a.p to b.p;
			connection c connect b.p to m.p;
			connect m.p to m.q;
			connect a.p to p;
			connect a.p to plain.p;
		}`)
	w8dWantLines(t, src, CodeMOSABoundaryNotDesignated, 13, 14)
	w8dWantLines(t, mosaModel(`
		port def P;
		#majorSystemComponent part def A { port p : P; }
		part v { part a : A; part b : A; connect a.p to b.p; }`), CodeMOSABoundaryNotDesignated)
}

// A connector stating its ends as body members is judged like one with a
// `connect` clause, whichever way the ends name their features.
func TestMOSABoundaryNotDesignatedWithBodyEnds(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#majorSystemPlatform part v {
			part l : A; part r : B;
			#modularSystem part m { port p : P; port q : P; }
			port p : P;
			interface link : Link connect l.p to r.p;
			connection c1 { end ::> l.p; end ::> r.p; }
			connection c2 { end x ::> l.p; end y ::> m.p; }
			interface i1 { end ::> l.p; end ::> r.p; }
			connection { end ::> m.p; end ::> m.q; }
			connection { end ::> l.p; end ::> p; }
			interface designated : Link { end ::> l.p; end ::> r.p; }
		}`)
	w8dWantLines(t, src, CodeMOSABoundaryNotDesignated, 12, 13, 14)
}

// An end attached outside any component neither forms a boundary nor hides one
// that two other ends form.
func TestMOSABoundaryNotDesignatedWithMixedEnds(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		#majorSystemPlatform part v {
			part l : A; part r : B;
			part plain { port p : P; }
			port p : P;
			interface link : Link connect l.p to r.p;
			connection { end ::> l.p; end ::> r.p; end ::> p; }
			connection { end ::> l.p; end ::> plain.p; end ::> r.p; }
			connection { end ::> l.p; end ::> plain.p; end ::> p; }
			connection { end ::> l.p; end ::> l.p; end ::> plain.p; }
		}`)
	w8dWantLines(t, src, CodeMOSABoundaryNotDesignated, 12, 13)
}

// Metadata specializing a MOSA metadata definition counts as that metadata:
// a program's own rights, control, proprietary and conformance keywords are recognised.
func TestMOSASpecializedMetadataIsRecognised(t *testing.T) {
	src := mosaModel(mosaPlatform + `
		metadata def ProgramRights :> DataRights;
		metadata def ProgramControl :> InterfaceControl;
		metadata def VendorOwned :> Proprietary;
		metadata def <verifiedAgainst> Verified :> StandardConformanceMetadata;
		metadata def <subject> Subject :> ConformantMetadata;
		#technicalStandard item std : Standard;
		part v {
			part a : A { @ProgramRights { kind = DataRightsKind::limited; } }
			part b : B;
			interface l1 : Link connect a.p to b.p { @ProgramControl { authority = "ICWG"; } }
			interface l2 : Link connect a.p to b.p { @VendorOwned { rationale = "qualified"; } }
			interface l3 : Link connect a.p to b.p { @VendorOwned; }
			interface l4 : Link connect a.p to b.p;
			#verifiedAgainst connection { end #subject ::> l4; end #conformsTo ::> std; }
		}`)
	w8dWantLines(t, src, CodeMOSAComponentNoDataRights, 15)
	w8dWantLines(t, src, CodeMOSAInterfaceNoControl, 17, 18, 19)
	w8dWantLines(t, src, CodeMOSAProprietaryNoRationale, 18)
	w8dWantLines(t, src, CodeMOSAInterfaceNoStandard, 16)
}

// A usage typed by a #modularSystemInterface definition and a #keyInterface
// usage are both modular system interfaces.
func TestMOSAKeyInterfaceIsAModularSystemInterface(t *testing.T) {
	src := mosaModel(`
		port def P;
		#majorSystemComponent part def A { port p : P; }
		interface def Plain { end a : P; end b : P; }
		#interfaceRequirement requirement def R; requirement r : R;
		part v {
			part a : A; part b : A;
			#keyInterface interface k : Plain connect a.p to b.p;
			#modularSystemInterface interface m : Plain connect a.p to b.p;
			interface plain : Plain connect a.p to b.p;
		}`)
	w8dWantLines(t, src, CodeMOSAInterfaceNotTraced, 9, 10)
	w8dWantLines(t, src, CodeMOSABoundaryNotDesignated, 11)
}

// A model that does not touch the MOSA library gets no MOSA finding.
func TestMOSASilentWithoutTheLibrary(t *testing.T) {
	const src = `package P {
		port def P;
		part def A { port p : P; }
		interface def Link { end a : P; end b : P; }
		requirement def R; requirement r : R;
		part v { part a : A; part b : A; interface l : Link connect a.p to b.p; connect a.p to b.p; }
		satisfy r by v.l;
	}`
	for _, code := range mosaCodes {
		w8dWantLines(t, src, code)
	}
}

// A workspace package named MOSA is not the approach's vocabulary: without the
// bundled library nothing is a MOSA element, however it is named.
func TestMOSASilentForAUserPackageNamedMOSA(t *testing.T) {
	const src = `package MOSA {
		part def MajorSystemComponent;
		interface def ModularSystemInterface;
		item def Standard;
		metadata def DataRights;
		metadata def Proprietary;
	}
	package M {
		private import MOSA::*;
		part a : MajorSystemComponent { @DataRights; }
		part b : MajorSystemComponent;
		part c { @Proprietary; }
		item s : Standard;
		interface i : ModularSystemInterface;
	}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == mosaSource {
			t.Errorf("unexpected MOSA finding: %s: %s", d.Code, d.Message)
		}
	}
}
