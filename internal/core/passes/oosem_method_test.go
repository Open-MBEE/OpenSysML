package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// oosemModel wraps body in a package importing the OOSEM library.
func oosemModel(body string) string {
	return "package M {\n\tprivate import OOSEM::*;" + body + "\n}"
}

const oosemRequirementChain = `
		#stakeholderNeed requirement need;
		#missionRequirement requirement mission;
		#systemRequirement requirement sysA;
		#systemRequirement requirement sysB;
		#componentRequirement requirement comp;
		#derivation connection { end #original ::> need; end #derive ::> mission; }
		#derivation connection { end #original ::> mission; end #derive ::> sysA; }
		#derivation connection { end #original ::> sysA; end #derive ::> comp; }`

func TestOOSEMRequirementNotDerived(t *testing.T) {
	src := oosemModel(oosemRequirementChain)
	// sysB is the only requirement with a level above it and no derivation.
	w8dWantLines(t, src, CodeOOSEMRequirementNotDerived, 6)
	w8dWantLines(t, src, CodeOOSEMRequirementNotSatisfied)
}

// A model that has not reached the level above stays silent: system requirements
// alone are not told they derive from nothing.
func TestOOSEMRequirementDerivationWaitsForTheLevelAbove(t *testing.T) {
	src := oosemModel(`
		#systemRequirement requirement sysA;
		#componentRequirement requirement comp;
		#derivation connection { end #original ::> sysA; end #derive ::> comp; }`)
	w8dWantLines(t, src, CodeOOSEMRequirementNotDerived)
}

// A derivation counts only from the level above: one from the same level, one
// from an unclassified requirement, and a plain connection with the same ends
// leave the requirement underived.
func TestOOSEMRequirementDerivedFromTheWrongLevel(t *testing.T) {
	src := oosemModel(`
		#missionRequirement requirement mission;
		#systemRequirement requirement sysA;
		#systemRequirement requirement sysB;
		#systemRequirement requirement sysC;
		#systemRequirement requirement sysD;
		requirement plain;
		#derivation connection { end #original ::> mission; end #derive ::> sysA; }
		#derivation connection { end #original ::> sysA; end #derive ::> sysB; }
		#derivation connection { end #original ::> plain; end #derive ::> sysC; }
		connection { end #original ::> mission; end #derive ::> sysD; }`)
	w8dWantLines(t, src, CodeOOSEMRequirementNotDerived, 5, 6, 7)
}

// Classification follows typing as well as the keyword: a usage typed by a
// definition annotated `#systemRequirement` is a system requirement.
func TestOOSEMRequirementKindByTyping(t *testing.T) {
	src := oosemModel(`
		#missionRequirement requirement def Mission;
		#systemRequirement requirement def Sys;
		requirement mission : Mission;
		requirement sys : Sys;`)
	w8dWantLines(t, src, CodeOOSEMRequirementNotDerived, 6)
}

func TestOOSEMRequirementNotSatisfied(t *testing.T) {
	src := oosemModel(`
		#systemRequirement requirement sysA;
		#systemRequirement requirement sysB;
		#componentRequirement requirement comp;
		#missionRequirement requirement mission;
		part p;
		satisfy sysA by p;
		part q { satisfy requirement own : SystemRequirement; }`)
	// The mission requirement is not expected to be satisfied; sysA is, and the
	// declared satisfy is its own requirement.
	w8dWantLines(t, src, CodeOOSEMRequirementNotSatisfied, 4, 5)
}

// A requirement a satisfy declares is held to derivation like any other.
func TestOOSEMDeclaredSatisfyRequirementDerivation(t *testing.T) {
	src := oosemModel(`
		#missionRequirement requirement mission;
		part p {
			satisfy requirement own : SystemRequirement;
			satisfy requirement derived : SystemRequirement;
		}
		#derivation connection { end #original ::> mission; end #derive ::> p.derived; }`)
	w8dWantLines(t, src, CodeOOSEMRequirementNotDerived, 5)
}

// A view satisfying a viewpoint, or a negated satisfy, states no satisfaction.
func TestOOSEMSatisfactionIgnoresViewpointsAndNegations(t *testing.T) {
	silent := oosemModel(`
		#systemRequirement requirement sysA;
		view v : RequirementsView { satisfy RequirementsViewpoint; }`)
	w8dWantLines(t, silent, CodeOOSEMRequirementNotSatisfied)
	negated := oosemModel(`
		#systemRequirement requirement sysA;
		#systemRequirement requirement sysB;
		#systemRequirement requirement sysC;
		part p;
		satisfy sysA by p;
		assert not satisfy sysB by p;
		part q { satisfy sysC; }`)
	w8dWantLines(t, negated, CodeOOSEMRequirementNotSatisfied, 4)
}

func TestOOSEMSatisfactionWaitsForAnySatisfy(t *testing.T) {
	src := oosemModel(`
		#systemRequirement requirement sysA;
		#componentRequirement requirement comp;
		part p;`)
	w8dWantLines(t, src, CodeOOSEMRequirementNotSatisfied)
}

func TestOOSEMLogicalComponentNotAllocated(t *testing.T) {
	src := oosemModel(`
		#logical part def Sensing;
		#logical part def Processing;
		#logical part def Downlink;
		#logical part def Storage;
		part def Sys {
			#logical part sensing : Sensing;
			#logical part processing : Processing;
			#logical part downlink : Downlink;
			#logical part storage : Storage {
				#logical part cache : Storage;
			}
		}
		#node part def Space;
		#hardware part def Camera;
		part sys : Sys;
		part space : Space { #hardware part camera : Camera; }
		allocate sys.sensing to space.camera;
		allocate Processing to Space;
		allocate sys.storage to space;`)
	// downlink has no allocation; cache is covered by the allocation of storage;
	// processing by that of its type.
	w8dWantLines(t, src, CodeOOSEMLogicalNotAllocated, 10)
}

// Allocation stated with body ends counts as well as the `allocate x to y` form.
func TestOOSEMLogicalComponentAllocatedByBodyEnds(t *testing.T) {
	src := oosemModel(`
		#logical part def Sensing;
		#node part def Space;
		#logical part sensing : Sensing;
		#node part space : Space;
		allocation a { end ::> sensing; end ::> space; }`)
	w8dWantLines(t, src, CodeOOSEMLogicalNotAllocated)
}

// An allocation to something other than a node or physical component does not
// realise a logical component, in either allocation form.
func TestOOSEMLogicalComponentAllocatedToTheWrongThing(t *testing.T) {
	src := oosemModel(`
		#logical part def Sensing;
		#node part def Space;
		#logical part sensing : Sensing;
		#logical part detection : Sensing;
		#logical part downlink : Sensing;
		#node part space : Space;
		part plain;
		item data;
		allocate sensing to plain;
		allocation a { end ::> detection; end ::> data; }
		allocate downlink to space;`)
	w8dWantLines(t, src, CodeOOSEMLogicalNotAllocated, 5, 6)
}

// With no node or physical component yet, a logical architecture is complete
// on its own.
func TestOOSEMAllocationWaitsForPhysicalArchitecture(t *testing.T) {
	src := oosemModel(`
		#logical part def Sensing;
		part def Sys { #logical part sensing : Sensing; }`)
	w8dWantLines(t, src, CodeOOSEMLogicalNotAllocated)
}

// A logical component named as the subject of a requirement or an actor is not
// a component usage to allocate.
func TestOOSEMAllocationIgnoresSubjects(t *testing.T) {
	src := oosemModel(`
		#logical part def Sensing;
		#node part def Space;
		#componentRequirement requirement def R { subject s : Sensing; }`)
	w8dWantLines(t, src, CodeOOSEMLogicalNotAllocated)
}

func TestOOSEMUseCaseSubject(t *testing.T) {
	src := oosemModel(`
		#enterprise part def Corp;
		#system part def Sat;
		#systemContext part def Ctx;
		#systemUseCase use case def Observe { subject ctx : Ctx; }
		#systemUseCase use case def Fail { subject sat : Sat; }
		#enterpriseUseCase use case def Run { subject corp : Corp; }
		#enterpriseUseCase use case def Wrong { subject ctx : Ctx; }
		#systemUseCase use case untyped { subject s; }
		use case observe : Observe;`)
	w8dWantLines(t, src, CodeOOSEMUseCaseSubject, 7, 9)
}

// A subject typed through redefinition or subsetting is judged by its effective
// types, not only by an explicit `:`.
func TestOOSEMUseCaseSubjectInherited(t *testing.T) {
	src := oosemModel(`
		#enterprise part def Corp;
		#systemContext part def Ctx;
		part def Sat;
		part sat : Sat;
		part ctx : Ctx;
		use case def Base { subject s : Sat; }
		#systemUseCase use case def Bad :> Base { subject :>> s; }
		#systemUseCase use case def Good :> Base { subject :>> s : Ctx; }
		#systemUseCase use case def Sub { subject s :> ctx; }
		#enterpriseUseCase use case def Wrong { subject s :> sat; }`)
	w8dWantLines(t, src, CodeOOSEMUseCaseSubject, 9, 12)
}

// A bodyless OOSEM use case inherits its subject from a plain base and is judged
// by it; one inheriting from an OOSEM use case of its kind is judged there only.
func TestOOSEMUseCaseSubjectInheritedWithoutBody(t *testing.T) {
	src := oosemModel(`
		#systemContext part def Ctx;
		part def Sat;
		use case def OnSat { subject s : Sat; }
		use case def OnCtx { subject s : Ctx; }
		#systemUseCase use case def Bad :> OnSat;
		#systemUseCase use case def Good :> OnCtx;
		#systemUseCase use case bad : OnSat;
		#systemUseCase use case good : OnCtx;
		use case fromBad : Bad;
		use case fromGood : Good;`)
	w8dWantLines(t, src, CodeOOSEMUseCaseSubject, 7, 9)
}

// With a classified and a plain base, only the plain base's subject is judged here.
func TestOOSEMUseCaseSubjectMixedInheritance(t *testing.T) {
	src := oosemModel(`
		#systemContext part def Ctx;
		part def Sat;
		use case def OnSat { subject s : Sat; }
		use case def OnCtx { subject t : Ctx; }
		#systemUseCase use case def Bare;
		#systemUseCase use case def Bad :> Bare, OnSat;
		#systemUseCase use case def Good :> Bare, OnCtx;
		#systemUseCase use case def Judged { subject u : Sat; }
		#systemUseCase use case def Once :> Judged, OnCtx;`)
	w8dWantLines(t, src, CodeOOSEMUseCaseSubject, 8, 10)
}

// A model that does not touch the OOSEM library gets no OOSEM finding.
func TestOOSEMSilentWithoutTheLibrary(t *testing.T) {
	const src = `package P {
		requirement def R;
		requirement r : R;
		part p;
		satisfy r by p;
		part def L; part l : L;
		allocate l to p;
	}`
	for _, code := range []string{CodeOOSEMRequirementNotDerived, CodeOOSEMRequirementNotSatisfied, CodeOOSEMLogicalNotAllocated, CodeOOSEMUseCaseSubject} {
		w8dWantLines(t, src, code)
	}
}

// A workspace package named OOSEM is not the method vocabulary: without the
// bundled library nothing is an OOSEM artefact, however it is named.
func TestOOSEMSilentForAUserPackageNamedOOSEM(t *testing.T) {
	const src = `package OOSEM {
		requirement def MissionRequirement;
		requirement def SystemRequirement;
		part def LogicalComponent;
		part def Node;
	}
	package M {
		private import OOSEM::*;
		requirement mission : MissionRequirement;
		requirement sys : SystemRequirement;
		part logical : LogicalComponent;
		part node : Node;
	}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == oosemSource {
			t.Errorf("unexpected OOSEM finding: %s: %s", d.Code, d.Message)
		}
	}
}
