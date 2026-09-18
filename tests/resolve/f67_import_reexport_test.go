package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// f67Clean resolves src and requires it to produce no diagnostics.
func f67Clean(t *testing.T, name, src string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		r, _, _ := resolvedDoc(t, src)
		if len(r.Diagnostics) != 0 {
			for _, d := range r.Diagnostics {
				t.Logf("diag: %s", d.Message)
			}
			t.Error("unexpected diagnostics above")
		}
	})
}

// A name a wildcard import introduces can itself be the target of a sibling
// import (F67: `private import RiskLevelEnum::*;`).
func TestF67ImportOfImportIntroducedName(t *testing.T) {
	f67Clean(t, "wildcard-then-wildcard", `package M {
		package RiskMeta {
			enum def E { low; medium; high; }
		}
		package Use {
			private import RiskMeta::*;
			private import E::*;
			attribute x = low;
		}
	}`)
	f67Clean(t, "membership-then-wildcard", `package M {
		package RiskMeta {
			enum def E { low; medium; high; }
		}
		package Use {
			private import RiskMeta::E;
			private import E::*;
			attribute x = low;
		}
	}`)
}

// A public wildcard import re-exports, so the name reaches a second importer
// through the chain (F67: `Diameter` via `DesignModel::*` re-export).
func TestF67WildcardReexportChain(t *testing.T) {
	f67Clean(t, "two-level-chain", `package M {
		package Inner { part def D; }
		package Mid { public import Inner::*; }
		package Use {
			private import Mid::*;
			part x : D;
		}
	}`)
}

func TestF67WildcardReexportReachesRecursiveMembershipImport(t *testing.T) {
	f67Clean(t, "mixed-chain", `package M {
		package Metadata { metadata def Safety; }
		package Definitions { public import Metadata::**; }
		package Model { public import Definitions::*; }
		package Use {
			private import Model::*;
			part x { @Safety; }
		}
	}`)
}

// A private wildcard import does not re-export (KerML 8.2.3.3): the chained
// lookup must still miss when the middle import is private.
func TestF67PrivateImportDoesNotReexport(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package M {
		package Inner { part def D; }
		package Mid { private import Inner::*; }
		package Use {
			private import Mid::*;
			part x : D;
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected an unresolved-reference diagnostic through the private import chain")
	}
}

func TestF67PrivateMixedImportDoesNotReexport(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package M {
		package Metadata { metadata def Safety; }
		package Definitions { private import Metadata::**; }
		package Model { public import Definitions::*; }
		package Use {
			private import Model::*;
			part x { @Safety; }
		}
	}`)
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "unresolved reference: Safety") {
			return
		}
	}
	t.Fatalf("expected Safety to remain unresolved through the private import, got %v", r.Diagnostics)
}

func TestF67MixedImportCycleTerminates(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package M {
        package A { public import B::*; }
        package B {
            public import A::*;
            metadata def Safety;
        }
        package Use {
            private import A::*;
            part x { @Safety; @Missing; }
        }
    }`)
	var safety, missing bool
	for _, d := range r.Diagnostics {
		safety = safety || strings.Contains(d.Message, "unresolved reference: Safety")
		missing = missing || strings.Contains(d.Message, "unresolved reference: Missing")
	}
	if safety {
		t.Fatal("Safety did not resolve through the cyclic import chain")
	}
	if !missing {
		t.Fatal("expected Missing to remain unresolved")
	}
}

// A recursive membership import follows public imports of the namespace it
// enters, including when the re-exported member is nested below it.
func TestF67RecursiveMembershipReexportChain(t *testing.T) {
	f67Clean(t, "recursive-chain", `package M {
		package A { attribute def MassValue; }
		package B { public import A::*; }
		package T2 { public import B::**; attribute m2 : MassValue; }
		package T1 { public import B::*; attribute m1 : MassValue; }
	}`)
}

// A private import in the intermediate namespace is not part of its visible
// memberships, so a recursive import does not leak it onward.
func TestF67RecursivePrivateImportDoesNotReexport(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package M {
		package A { attribute def MassValue; }
		package B { private import A::*; }
		package T { public import B::**; attribute m : MassValue; }
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected private recursive import chain to leave MassValue unresolved")
	}
}

func TestF67ISQRecursiveMembershipReexport(t *testing.T) {
	idx := libs.NewModelIndex()
	const src = `package K {
		public import ISQ::**;
		part v { attribute totalMass : MassValue; }
	}
	package KStar {
		public import ISQ::*;
		attribute mass : MassValue;
	}`
	p := parser.New(source.New("isq.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse test document: %v", p.Diagnostics)
	}
	idx.AddDocument("isq.sysml", root)
	idx.ExpandWildcardImports()

	r := resolve.New(idx)
	r.ResolveDocument("isq.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected ISQ recursive and namespace imports to resolve, got %v", r.Diagnostics)
	}
}

// Subsetting via feature chain contributes the chain tip's members in the
// subsetter's body (F67: `part c subsets b.f { part aa subsets a; }`).
func TestF67FeatureChainSubsettingContributesMembers(t *testing.T) {
	f67Clean(t, "chain-subsetting", `package Q {
		part def F { part a; }
		part b { part f : F; }
		part c subsets b.f {
			part aa subsets a;
		}
	}`)
}

// An include is a reference subsetting (SysML.xtext IncludeUseCaseUsage), so
// the included use case's actors are redefinable in its body (F67).
func TestF67IncludeUseCaseContributesActors(t *testing.T) {
	f67Clean(t, "include-actor-redef", `package U {
		part def Person;
		part def Vehicle;
		use case 'add fuel' {
			subject v : Vehicle;
			actor fueler : Person;
		}
		use case 'drive vehicle' {
			subject v : Vehicle;
			actor driver : Person;
			include 'add fuel' {
				subject;
				actor :>> fueler = driver;
			}
		}
	}`)
}

// A bare `variant X` is a VariantReference (SysML.xtext:642): it subsets the
// like-named outer feature, whose members resolve in its body (F67).
func TestF67VariantReferenceContributesMembers(t *testing.T) {
	f67Clean(t, "variant-reference", `package V {
		port def AutoPort;
		part def Engine;
		package PartsTree {
			part engine : Engine {
				port autoPort : AutoPort;
			}
			part '6cylEngine' :> engine;
		}
		package Model150 {
			private import PartsTree::*;
			variation part def EngineChoices :> Engine {
				variant '4cylEngine';
				variant '6cylEngine' {
					variation port :>> autoPort {
						variant port autoPort1;
						variant port autoPort2;
					}
				}
			}
		}
	}`)
}

// A bare variant with no like-named outer feature stays a plain declaration:
// no reference is fabricated and nothing panics.
func TestF67VariantWithoutOuterNameNoPanic(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package V {
		part def Engine;
		variation part def EngineChoices :> Engine {
			variant onlyHere;
		}
	}`)
	_ = r.Diagnostics
}

// An unresolvable feature chain target must degrade to diagnostics, not panic.
func TestF67FeatureChainSubsettingUnresolvedNoPanic(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package Q {
		part c subsets nope.f {
			part aa subsets a;
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for an unresolved feature chain target")
	}
}
