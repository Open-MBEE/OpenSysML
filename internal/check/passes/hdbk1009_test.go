package passes

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// hdbkModel wraps body in a package importing the HDBK1009 library.
func hdbkModel(body string) string {
	return "package M {\n\tprivate import HDBK1009::*;" + body + "\n}"
}

var hdbkCodes = []string{
	CodeHDBK1009RequirementNotDerived,
	CodeHDBK1009ExpectationNotTraced,
	CodeHDBK1009MeasureNotRefining,
	CodeHDBK1009RequirementNotVerified,
	CodeHDBK1009VVRequirementNotRefined,
	CodeHDBK1009VVRequirementNoMethod,
	CodeHDBK1009ObjectiveNotValidated,
}

// Each level derives from the level above it once the model declares that level.
func TestHDBK1009RequirementNotDerived(t *testing.T) {
	src := hdbkModel(`
		#need requirement n;
		#goal requirement g;
		#goal requirement orphanGoal;
		#missionObjective requirement o;
		#systemRequirement requirement sys;
		#subsystemRequirement requirement sub;
		#subsystemRequirement requirement subOfSub;
		#componentRequirement requirement cmp;
		#derivation connection { end #original ::> n; end #derive ::> g; }
		#derivation connection { end #original ::> g; end #derive ::> o; }
		#derivation connection { end #original ::> o; end #derive ::> sys; }
		#derivation connection { end #original ::> sys; end #derive ::> sub; }
		#derivation connection { end #original ::> sub; end #derive ::> subOfSub; }
		#derivation connection { end #original ::> n; end #derive ::> cmp; }`)
	w8dWantLines(t, src, CodeHDBK1009RequirementNotDerived, 5, 10)
}

// A level whose level above is not declared yet is not reported.
func TestHDBK1009DerivationWaitsForTheLevelAbove(t *testing.T) {
	src := hdbkModel(`
		#goal requirement g;
		#systemRequirement requirement sys;
		#measureOfPerformance requirement mop;`)
	w8dWantLines(t, src, CodeHDBK1009RequirementNotDerived)
}

// A subsystem requirement is not the level above itself: alone it waits for a
// system requirement.
func TestHDBK1009LoneSubsystemRequirementIsNotItsOwnOrigin(t *testing.T) {
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement sub;`), CodeHDBK1009RequirementNotDerived)
	w8dWantLines(t, hdbkModel(`
		#systemRequirement requirement sys;
		#subsystemRequirement requirement sub;`), CodeHDBK1009RequirementNotDerived, 4)
}

// Several subsystem requirements with no system requirement above them are
// each other's possible origins: one that neither derives nor is derived from warns.
func TestHDBK1009SubsystemRequirementsWithoutASystemRequirement(t *testing.T) {
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement a;
		#subsystemRequirement requirement b;`), CodeHDBK1009RequirementNotDerived, 3, 4)
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement top;
		#subsystemRequirement requirement derived;
		#subsystemRequirement requirement loose;
		#derivation connection { end #original ::> top; end #derive ::> derived; }`),
		CodeHDBK1009RequirementNotDerived, 5)
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement a;
		#subsystemRequirement requirement b;
		#derivation connection { end #original ::> a; end #derive ::> a; }`),
		CodeHDBK1009RequirementNotDerived, 3, 4)
}

// Two parts typed by one definition each hold its requirement: deriving one
// part's from the other's relates two requirements, not one to itself.
func TestHDBK1009DerivationBetweenTwoPathsToOneInheritedRequirement(t *testing.T) {
	w8dWantLines(t, hdbkModel(`
		part def D { #subsystemRequirement requirement r; }
		part a : D;
		part b : D;
		#subsystemRequirement requirement loose;
		#derivation connection { end #original ::> a.r; end #derive ::> b.r; }`),
		CodeHDBK1009RequirementNotDerived, 6)
	w8dWantLines(t, hdbkModel(`
		part def D { #subsystemRequirement requirement r; }
		part a : D;
		part b : D;
		#subsystemRequirement requirement loose;
		#derivation connection { end #original ::> a::r; end #derive ::> b::r; }`),
		CodeHDBK1009RequirementNotDerived, 6)
	w8dWantLines(t, hdbkModel(`
		part def D { #subsystemRequirement requirement r; }
		part a : D;
		#subsystemRequirement requirement loose;
		#derivation connection { end #original ::> a.r; end #derive ::> a.r; }`),
		CodeHDBK1009RequirementNotDerived, 3, 5)
}

// A requirement named bare at one end and through its declaring part at the
// other is one requirement, so the derivation between them gives it no origin.
func TestHDBK1009SelfDerivationThroughTheDeclaringPart(t *testing.T) {
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement loose;
		part p {
			#subsystemRequirement requirement r;
			#derivation connection { end #original ::> r; end #derive ::> p.r; }
		}`), CodeHDBK1009RequirementNotDerived, 3, 5)
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement loose;
		part p { #subsystemRequirement requirement r; }
		#derivation connection { end #original ::> p.r; end #derive ::> p::r; }`),
		CodeHDBK1009RequirementNotDerived, 3, 4)
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement loose;
		part def D { #subsystemRequirement requirement r; }
		part c : D { #derivation connection { end #original ::> r; end #derive ::> c.r; } }`),
		CodeHDBK1009RequirementNotDerived, 3, 4)
	for _, other := range []string{"$::M::a::r", "Q::a::r"} {
		w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement loose;
		part def D { #subsystemRequirement requirement r; }
		part a : D;
		package Q { public import M::a; }
		#derivation connection { end #original ::> a::r; end #derive ::> `+other+`; }`),
			CodeHDBK1009RequirementNotDerived, 3, 4)
	}
	w8dWantLines(t, hdbkModel(`
		#subsystemRequirement requirement loose;
		part def D { #subsystemRequirement requirement r; }
		part outer : D {
			part inner : D { #derivation connection { end #original ::> outer.r; end #derive ::> r; } }
		}`), CodeHDBK1009RequirementNotDerived, 3)
}

// However a derivation spells its two ends, one requirement named twice gives it
// no origin, and one requirement inherited by two parts is two requirements.
func TestHDBK1009DerivationEndSpellings(t *testing.T) {
	const model = `
		#subsystemRequirement requirement loose;
		part def D { #subsystemRequirement requirement r; }
		part a : D;
		part b : D;
		part p {
			#subsystemRequirement requirement q;
			part held { #subsystemRequirement requirement s; }
		}
		package Shown { public import p::q; }
		`
	for _, tc := range []struct {
		name, derivation string
		underived        []int
	}{
		{"root-qualified", `#derivation connection { end #original ::> $::M::a::r; end #derive ::> a::r; }`, []int{3, 4, 8, 9}},
		{"chain then qualified name", `#derivation connection { end #original ::> p.held::s; end #derive ::> p::held::s; }`, []int{3, 4, 8, 9}},
		{"re-exporting package", `#derivation connection { end #original ::> Shown::q; end #derive ::> p::q; }`, []int{3, 4, 8, 9}},
		{"this", `part p2 :> p { #derivation connection connect q to this.q; }`, []int{3, 4, 8, 9}},
		{"two inheriting parts", `#derivation connection { end #original ::> a.r; end #derive ::> b.r; }`, []int{3, 8, 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w8dWantLines(t, hdbkModel(model+tc.derivation), CodeHDBK1009RequirementNotDerived, tc.underived...)
		})
	}
}

// The peer that makes a lone subsystem requirement warn may sit in another document.
func TestHDBK1009SubsystemRequirementPeersAcrossDocuments(t *testing.T) {
	w := newSharedWorkspace()
	w.put("a.sysml", `package A {
		private import HDBK1009::*;
		#subsystemRequirement requirement a;
	}`)
	w.analyzeAll()
	if got := only(w.analyze("a.sysml"), CodeHDBK1009RequirementNotDerived); len(got) != 0 {
		t.Errorf("a.sysml: a lone subsystem requirement warned:\n%v", got)
	}
	w.put("b.sysml", `package B {
		private import HDBK1009::*;
		#subsystemRequirement requirement b;
	}`)
	w.analyzeAll()
	for _, name := range w.names() {
		got := w.analyze(name)
		if n := len(only(got, CodeHDBK1009RequirementNotDerived)); n != 1 {
			t.Errorf("%s: %d warnings with a peer in the other document, want 1", name, n)
		}
		if want := w.fresh(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: shared analysis\n%v\nfresh analysis\n%v", name, got, want)
		}
	}
}

// Expectation statements trace to stakeholders; needs, goals and objectives to
// expectation statements. A refinement is a dependency and so a trace.
func TestHDBK1009ExpectationNotTraced(t *testing.T) {
	src := hdbkModel(`
		#stakeholderActor part team;
		#stakeholderExpectationStatement requirement ses;
		#stakeholderExpectationStatement requirement untraced;
		#need requirement n;
		#goal requirement g;
		#missionObjective requirement o;
		dependency from ses to team;
		dependency from n to ses;
		#refinement dependency from g to ses;
		dependency from o to team;`)
	w8dWantLines(t, src, CodeHDBK1009ExpectationNotTraced, 5, 8)
}

// A measure of effectiveness refines an objective, a measure of performance a requirement.
func TestHDBK1009MeasureNotRefining(t *testing.T) {
	src := hdbkModel(`
		#missionObjective requirement o;
		#systemRequirement requirement sys;
		#measureOfEffectiveness requirement moe;
		#measureOfEffectiveness requirement looseMOE;
		#measureOfPerformance requirement mop;
		#measureOfPerformance requirement looseMOP;
		#refinement dependency from moe to o;
		#refinement dependency from mop to sys;
		#refinement dependency from looseMOP to o;
		dependency from looseMOE to o;`)
	w8dWantLines(t, src, CodeHDBK1009MeasureNotRefining, 6, 8)
}

// A technical requirement is verified in the objective of a verification case
// once the model declares verification requirements; a satisfy does not verify.
func TestHDBK1009RequirementNotVerified(t *testing.T) {
	src := hdbkModel(`
		#systemRequirement requirement sys;
		#systemRequirement requirement unverified;
		#verificationRequirement requirement ver { :>> verifyMethod = VerificationMethodKind::test; }
		part p;
		satisfy unverified by p;
		#vvCase verification c { objective { verify sys; } }
		#refinement dependency from c to ver;`)
	w8dWantLines(t, src, CodeHDBK1009RequirementNotVerified, 4)

	quiet := hdbkModel(`
		#systemRequirement requirement sys;`)
	w8dWantLines(t, quiet, CodeHDBK1009RequirementNotVerified)
}

// A V&V requirement states its method and is refined by a V&V case once any exists.
func TestHDBK1009VVRequirement(t *testing.T) {
	src := hdbkModel(`
		#verificationRequirement requirement ver { :>> verifyMethod = VerificationMethodKind::test; }
		#verificationRequirement requirement bare;
		#validationRequirement requirement val { :>> verifyMethod = VerificationMethodKind::demo; }
		#vvCase verification c;
		verification plain;
		#refinement dependency from c to ver;
		#refinement dependency from plain to val;`)
	w8dWantLines(t, src, CodeHDBK1009VVRequirementNoMethod, 4)
	w8dWantLines(t, src, CodeHDBK1009VVRequirementNotRefined, 4, 5)

	noCases := hdbkModel(`
		#verificationRequirement requirement ver { :>> verifyMethod = VerificationMethodKind::test; }`)
	w8dWantLines(t, noCases, CodeHDBK1009VVRequirementNotRefined)
}

// Validation requirements satisfy the objectives, and each objective is satisfied by one.
func TestHDBK1009ObjectiveNotValidated(t *testing.T) {
	src := hdbkModel(`
		#missionObjective requirement o;
		#missionObjective requirement unvalidated;
		#validationRequirement requirement val { :>> verifyMethod = VerificationMethodKind::demo; }
		#validationRequirement requirement loose { :>> verifyMethod = VerificationMethodKind::demo; }
		part p;
		satisfy o by val;
		satisfy unvalidated by p;`)
	w8dWantLines(t, src, CodeHDBK1009ObjectiveNotValidated, 4, 6)
}

// A satisfy or verify naming a requirement is not another requirement of its kind.
func TestHDBK1009SatisfyIsNotARequirement(t *testing.T) {
	src := hdbkModel(`
		#missionObjective requirement o;
		#systemRequirement requirement sys;
		#derivation connection { end #original ::> o; end #derive ::> sys; }
		part p { satisfy sys; }`)
	for _, code := range hdbkCodes {
		w8dWantLines(t, src, code)
	}
}

// A satisfy declaring its requirement declares one the rules judge.
func TestHDBK1009SatisfyDeclaringARequirementIsJudged(t *testing.T) {
	src := hdbkModel(`
		#missionObjective requirement o;
		part p { satisfy requirement sys : SystemRequirement; }`)
	w8dWantLines(t, src, CodeHDBK1009RequirementNotDerived, 4)
}

// `not satisfy` states that the requirement is not satisfied: it validates nothing.
func TestHDBK1009NegatedSatisfyDoesNotValidate(t *testing.T) {
	src := hdbkModel(`
		#missionObjective requirement o;
		#validationRequirement requirement val { :>> verifyMethod = VerificationMethodKind::test; }
		not satisfy o by val;`)
	w8dWantLines(t, src, CodeHDBK1009ObjectiveNotValidated, 3, 4)
}

// A model that does not touch the HDBK1009 library gets no finding.
func TestHDBK1009SilentWithoutTheLibrary(t *testing.T) {
	const src = `package P {
		requirement def Objective;
		requirement o : Objective;
		requirement r;
		part p;
		satisfy r by p;
	}`
	for _, code := range hdbkCodes {
		w8dWantLines(t, src, code)
	}
}

// A workspace package named HDBK1009 is not the Handbook's vocabulary.
func TestHDBK1009SilentForAUserPackageNamedHDBK1009(t *testing.T) {
	const src = `package HDBK1009 {
		requirement def Goal;
		requirement def Need;
		requirement def VerificationRequirement;
	}
	package M {
		private import HDBK1009::*;
		requirement n : Need;
		requirement g : Goal;
		requirement v : VerificationRequirement;
	}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == hdbkSource {
			t.Errorf("unexpected HDBK1009 finding: %s: %s", d.Code, d.Message)
		}
	}
}

// A relationship another document states answers the rule, and withdrawing it
// reaches the document that read it.
func TestHDBK1009ReadsRelationshipsAcrossDocuments(t *testing.T) {
	w := newSharedWorkspace()
	w.put("reqs.sysml", `package Reqs {
		private import HDBK1009::*;
		#missionObjective requirement o;
		#systemRequirement requirement sys;
		#measureOfEffectiveness requirement moe;
		#verificationRequirement requirement ver { :>> verifyMethod = VerificationMethodKind::test; }
	}`)
	const links = `package Links {
		private import HDBK1009::*;
		private import Reqs::*;
		#derivation connection { end #original ::> o; end #derive ::> sys; }
		#refinement dependency from moe to o;
		#vvCase verification c { objective { verify sys; } }
		#refinement dependency from c to ver;
	}`
	w.put("links.sysml", links)
	w.analyzeAll()
	for _, code := range hdbkCodes {
		if got := only(w.analyze("reqs.sysml"), code); len(got) != 0 {
			t.Errorf("reqs.sysml: %s reported though links.sysml states the relationship:\n%v", code, got)
		}
	}
	w.put("links.sysml", `package Links {
		private import HDBK1009::*;
		#vvCase verification c;
	}`)
	for _, code := range []string{
		CodeHDBK1009RequirementNotDerived, CodeHDBK1009MeasureNotRefining,
		CodeHDBK1009RequirementNotVerified, CodeHDBK1009VVRequirementNotRefined,
	} {
		if got := only(w.analyze("reqs.sysml"), code); len(got) != 1 {
			t.Errorf("reqs.sysml: %s reported %d times with the relationship withdrawn, want 1", code, len(got))
		}
	}
	w.put("links.sysml", links)
	for _, name := range w.names() {
		if got, want := w.analyze(name), w.fresh(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: shared analysis\n%v\nfresh analysis\n%v", name, got, want)
		}
	}
}
