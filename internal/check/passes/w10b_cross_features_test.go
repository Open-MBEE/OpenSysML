package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const w10bCrossSrc = `package AssociationTest {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; feature c subsets b; }
	assoc A1 {
		end x : C1 crosses y.b;
		end y : C2 crosses y.b;
	}
	assoc A2 specializes A1 {
		end x : C1 crosses y.c;
		end y : C2 crosses x.a;
	}
}`

// An end crossing a feature of its own type chain fails both crossing
// constraints, as in validation/AssociationTest_CrossFeatures_invalid.
func TestW10BCrossFeatureTypeAndChain(t *testing.T) {
	diags := constraintDiagsKerML(t, w10bCrossSrc)
	for _, tc := range []struct{ code, msg string }{
		{"cross-feature-type", msgCrossFeatureType},
		{"cross-subsetting-chain", msgCrossSubsettingChain},
	} {
		got := only(diags, tc.code)
		if len(got) != 1 {
			t.Fatalf("%s: got %d diagnostics, want 1: %v", tc.code, len(got), got)
		}
		if got[0].Message != tc.msg {
			t.Errorf("%s: message = %q, want %q", tc.code, got[0].Message, tc.msg)
		}
		if got[0].Severity != diag.SeverityError {
			t.Errorf("%s: severity = %v, want an error", tc.code, got[0].Severity)
		}
		if text := spanText(w10bCrossSrc, got[0]); text != "y.b" {
			t.Errorf("%s: span text = %q, want %q", tc.code, text, "y.b")
		}
	}
}

// The cross feature's types must equal the end's, an untyped feature counting
// as typed by Anything (KerML validateFeatureCrossFeatureType, pilot-confirmed).
func TestW10BCrossFeatureTypeEffectiveTypes(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"untyped crossed feature", `package P {
			class A { feature x; }
			assoc S { end a : A; end b : A crosses a.x; }
		}`, "a.x"},
		{"untyped end", `package P {
			class A { feature x : A; }
			assoc S { end a : A; end b crosses a.x; }
		}`, "a.x"},
		{"owned cross feature typed differently", `package P {
			class A; class B;
			assoc S { end a : A { member feature ac : B [0..1]; } end b : A; }
		}`, "member feature ac : B [0..1];"},
		{"explicit type plus unrelated subsetted type", `package P {
			class A; class W;
			class Q { feature w : W; feature x : A subsets w; }
			assoc S { end a : Q; end b : A crosses a.x; }
		}`, "a.x"},
		{"untyped reference to a typed feature", `package P {
			class A; class B;
			class Q { feature y : B; feature r references y; }
			assoc S { end a : Q; end b : A crosses a.r; }
		}`, "a.r"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := constraintDiagsKerML(t, tc.src)
			expectCrossDiag(t, tc.src, diags, codeCrossFeatureType, msgCrossFeatureType, tc.want)
		})
	}
}

// Types inherited through redefinition, subsetting or referencing, and an
// owned cross feature's implied typing by its end, count toward the
// comparison, and a type is dropped once a more specific one is present.
func TestW10BCrossFeatureTypeEffectiveTypesClean(t *testing.T) {
	const src = `package P {
		class A { feature x : A; feature y subsets x; }
		class A2 specializes A { feature g : A; feature h : A2 subsets g; feature j : A subsets h; feature k references h; }
		assoc R { end a : A2; end b : A2 crosses a.h; }
		assoc R2 { end a : A2; end b : A2 crosses a.k; }
		assoc R3 { end a : A2; end b : A2 crosses a.j; }
		assoc S { end a : A; end b : A crosses a.x; }
		assoc T specializes S { end a2 redefines a; end b2 redefines b crosses a2.y; }
		assoc U { end a : A { member feature ac [0..1]; } end b : A { member feature bc : A [0..1]; } }
		class Z { feature z; }
		assoc V { end a : Z; end b crosses a.z; }
		class C specializes A { feature p : A; feature q : A; connector k { end ka ::> p; end kb ::> q crosses ka.x; } }
	}`
	expectNoCrossDiags(t, constraintDiagsKerML(t, src))
}

// A specialized association's end must cross a feature specializing the cross
// feature of the end it redefines.
func TestW10BCrossFeatureSpecialization(t *testing.T) {
	diags := only(constraintDiagsKerML(t, w10bCrossSrc), "cross-feature-specialization")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgCrossSpecialization {
		t.Errorf("message = %q, want %q", diags[0].Message, msgCrossSpecialization)
	}
	if text := spanText(w10bCrossSrc, diags[0]); text != "x.a" {
		t.Errorf("span text = %q, want %q", text, "x.a")
	}
}

// Crossings that name a feature of the opposite end's type with the end's own
// type, and refine the inherited cross feature, are well-formed.
func TestW10BCrossFeaturesClean(t *testing.T) {
	const src = `package AssociationTest {
		class C1 { feature a : C2; }
		class C2 { feature b : C1; feature c subsets b; }
		assoc A1 {
			end x : C1 crosses y.b;
			end y : C2 crosses x.a;
		}
		assoc A2 specializes A1 {
			end x : C1 crosses y.c;
			end y : C2 crosses x.a;
		}
	}`
	for _, code := range []string{"cross-feature-type", "cross-subsetting-chain", "cross-feature-specialization"} {
		if diags := only(constraintDiagsKerML(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on a well-formed association: %v", code, diags)
		}
	}
}

// Malformed crossings must not panic and must not be adjudicated.
func TestW10BCrossFeaturesMalformedInput(t *testing.T) {
	const src = `package P {
		assoc A1 {
			end x crosses ;
			end y : crosses y.;
			end crosses ..b;
		}
	}`
	for _, code := range []string{"cross-feature-type", "cross-feature-specialization"} {
		if diags := only(constraintDiagsKerML(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on malformed input: %v", code, diags)
		}
	}
}

var w10bCrossCodes = []string{
	codeCrossSubsettingOwner, codeCrossSubsettingAtMostOne, codeCrossSubsettingChain,
	codeCrossFeatureType, codeCrossFeatureSpecialization,
}

// expectCrossDiag asserts exactly one diagnostic with code, its message, and
// the source text it points at.
func expectCrossDiag(t *testing.T, src string, diags []diag.Diagnostic, code, msg, text string) {
	t.Helper()
	got := only(diags, code)
	if len(got) != 1 {
		t.Fatalf("%s: got %d diagnostics, want 1: %v", code, len(got), got)
	}
	if got[0].Message != msg {
		t.Errorf("%s: message = %q, want %q", code, got[0].Message, msg)
	}
	if got[0].Severity != diag.SeverityError {
		t.Errorf("%s: severity = %v, want an error", code, got[0].Severity)
	}
	if spanned := strings.TrimSpace(spanText(src, got[0])); spanned != text {
		t.Errorf("%s: span text = %q, want %q", code, spanned, text)
	}
}

// expectNoCrossDiags asserts that none of the crossing rules fired.
func expectNoCrossDiags(t *testing.T, diags []diag.Diagnostic) {
	t.Helper()
	for _, code := range w10bCrossCodes {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired on a well-formed model: %v", code, got)
		}
	}
}

// A cross subsetting must be owned by an end feature: a plain feature of a
// class has none to cross from (KerML validateCrossSubsettingCrossingFeature).
func TestW10BCrossSubsettingFromNonEnd(t *testing.T) {
	const src = `package P {
		class A {
			feature p : A;
			feature q : A crosses p.q;
		}
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossSubsettingOwner, msgCrossSubsettingOwner, "p.q")
	for _, code := range []string{codeCrossSubsettingChain, codeCrossFeatureType, codeCrossFeatureSpecialization} {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired on a non-end feature, whose chain is not adjudicated: %v", code, got)
		}
	}
}

// An end of an association with a single end has no opposite end to cross.
func TestW10BCrossSubsettingSingleEnd(t *testing.T) {
	const src = `package P {
		class A { feature x : A; }
		assoc OneEnd { end a : A crosses a.x; }
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossSubsettingOwner, msgCrossSubsettingOwner, "a.x")
}

// A crossed feature must be a chain of two features; naming the opposite end
// itself, or chaining three features, is not one
// (KerML validateCrossSubsettingCrossedFeature).
func TestW10BCrossSubsettingNotAChain(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"opposite end itself", "a"},
		{"three features", "a.x.y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package P {
				class A { feature x : A; feature y : A; }
				assoc S {
					end a : A;
					end b : A crosses ` + tc.target + `;
				}
			}`
			diags := constraintDiagsKerML(t, src)
			expectCrossDiag(t, src, diags, codeCrossSubsettingChain, msgCrossSubsettingChain, tc.target)
			if got := only(diags, codeCrossSubsettingOwner); len(got) != 0 {
				t.Errorf("owner rule fired on an end of a two-ended association: %v", got)
			}
		})
	}
}

// With more than two ends the chain may start at any other end.
func TestW10BCrossSubsettingThreeEndsClean(t *testing.T) {
	const src = `package P {
		class A { feature x : A; }
		assoc Three {
			end a : A;
			end b : A;
			end c : A crosses a.x;
		}
	}`
	expectNoCrossDiags(t, constraintDiagsKerML(t, src))
}

// With more than two ends the chain starts at the feature cross-multiplying the
// other ends, which the crossing end owns (the spec's ProductSelection3 shape).
func TestW10BCrossSubsettingThreeEndsCrossMultiplyingRootClean(t *testing.T) {
	const src = `package P {
		class A; class B; class C;
		assoc Three {
			end a : A crosses a::bc.inA {
				member feature inA : A [0..1] featured by B_C {
					member feature B_C : C featured by B;
				}
				member feature bc : inA::B_C featured by Three { public import inA; }
			}
			end b : B;
			end c : C;
		}
	}`
	expectNoCrossDiags(t, constraintDiagsKerML(t, src))
}

// A feature owns at most one cross subsetting; each one after the first is
// reported (KerML validateFeatureOwnedCrossSubsetting).
func TestW10BCrossSubsettingAtMostOne(t *testing.T) {
	const src = `package P {
		class A { feature x : A; feature y : A; }
		assoc S {
			end a : A;
			end b : A crosses a.x crosses a.y;
		}
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossSubsettingAtMostOne, msgCrossSubsettingAtMostOne, "a.y")
	for _, code := range []string{codeCrossSubsettingOwner, codeCrossSubsettingChain, codeCrossFeatureType} {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired on the well-formed first crossing: %v", code, got)
		}
	}
}

// Every cross subsetting is checked for its owner and chain shape, not only
// the first; the type rule reads Feature::crossFeature, which the first defines.
func TestW10BCrossSubsettingLaterClauseChain(t *testing.T) {
	const src = `package P {
		class A { feature x : A; feature y : A; }
		assoc S {
			end a : A;
			end b : A crosses a.x crosses b.y;
		}
		class C {
			feature p : A;
			feature q : A crosses p.x crosses p.y;
		}
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossSubsettingChain, msgCrossSubsettingChain, "b.y")
	if got := only(diags, codeCrossSubsettingAtMostOne); len(got) != 2 {
		t.Errorf("at-most-one: got %d diagnostics, want one per excess clause: %v", len(got), got)
	}
	if got := only(diags, codeCrossSubsettingOwner); len(got) != 2 {
		t.Errorf("owner: got %d diagnostics, want one per clause of the non-end: %v", len(got), got)
	}
	for _, code := range []string{codeCrossFeatureType, codeCrossFeatureSpecialization} {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired although the first crossing is well-formed: %v", code, got)
		}
	}
}

// An end that redefines an end of a general association, by name or by
// position, must cross a feature specializing the redefined end's cross
// feature (KerML validateFeatureCrossFeatureSpecialization).
func TestW10BCrossFeatureSpecializationRedefinedEnds(t *testing.T) {
	for _, tc := range []struct{ name, ends string }{
		{"explicit redefinition", "end a2 : A redefines a; end b2 : A redefines b crosses a2.y;"},
		{"positional redefinition", "end a2 : A; end b2 : A crosses a2.y;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package P {
				class A { feature x : A; feature y : A; }
				assoc S { end a : A; end b : A crosses a.x; }
				assoc T specializes S { ` + tc.ends + ` }
			}`
			diags := constraintDiagsKerML(t, src)
			expectCrossDiag(t, src, diags, codeCrossFeatureSpecialization, msgCrossSpecialization, "a2.y")
			if got := only(diags, codeCrossSubsettingChain); len(got) != 0 {
				t.Errorf("chain rule fired on a chain through the opposite end: %v", got)
			}
		})
	}
}

// A redefining end whose cross feature subsets the inherited one, or crosses
// the same feature, is well-formed.
func TestW10BCrossFeatureSpecializationClean(t *testing.T) {
	const src = `package P {
		class A { feature x : A; feature y : A subsets x; }
		assoc S { end a : A; end b : A crosses a.x; }
		assoc T specializes S { end a2 : A redefines a; end b2 : A redefines b crosses a2.y; }
		assoc U specializes S { end a2 : A; end b2 : A crosses a2.x; }
	}`
	expectNoCrossDiags(t, constraintDiagsKerML(t, src))
}

// A cross feature declared in an end's body implicitly subsets the cross
// feature of each end its owner redefines, so crossing it is well-formed.
func TestW10BCrossFeatureSpecializationOwnedCrossFeatureClean(t *testing.T) {
	const src = `package P {
		class A; class B; class C;
		assoc S {
			end a : A crosses a::via.ax { member feature ax : A; member feature via : A { public import ax; } }
			end b : B;
			end c : C;
		}
		assoc T specializes S {
			end a2 : A redefines a crosses a2::via.ax2 { member feature ax2 : A; member feature via : A { public import ax2; } }
			end b2 : B redefines b;
			end c2 : C redefines c;
		}
		assoc U specializes T {
			end a3 : A crosses a3::via.ax3 { member feature ax3 : A; member feature via : A { public import ax3; } }
			end b3 : B;
			end c3 : C;
		}
		assoc V specializes S {
			end a4 : A redefines a crosses a4::via.other { member feature ax4 : A; feature other : A; member feature via : A { public import other; } }
			end b4 : B redefines b;
			end c4 : C redefines c;
		}
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossFeatureSpecialization, msgCrossSpecialization, "a4::via.other")
	for _, code := range []string{codeCrossSubsettingOwner, codeCrossSubsettingAtMostOne, codeCrossSubsettingChain, codeCrossFeatureType} {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired on well-formed ends: %v", code, got)
		}
	}
}

// A cross feature an end declares inline ahead of itself (`end x1 [0..1]
// feature x`) is the cross feature a redefining end must specialize; one the
// redefining end declares inline itself implicitly does.
func TestW10BCrossFeatureSpecializationInlineCrossFeature(t *testing.T) {
	const src = `package P {
		class C1;
		class C2 { feature g : C1; }
		assoc A {
			end x1 [0..1] feature x : C1;
			end feature y : C2;
		}
		assoc B specializes A {
			end feature x2 : C1 redefines x crosses y2.g;
			end feature y2 : C2 redefines y;
		}
		assoc B2 specializes A {
			end x3 [0..1] feature x2 : C1 redefines x;
			end feature y2 : C2 redefines y;
		}
		assoc B3 specializes A {
			end feature x2 : C1 redefines x { member feature x4 : C1; }
			end feature y2 : C2 redefines y;
		}
	}`
	diags := constraintDiagsKerML(t, src)
	expectCrossDiag(t, src, diags, codeCrossFeatureSpecialization, msgCrossSpecialization, "y2.g")
	if got := only(diags, codeCrossFeatureSpecialization); len(got) != 1 {
		t.Errorf("expected the one crossing of an unrelated feature, got %v", got)
	}
	for _, code := range []string{codeCrossSubsettingOwner, codeCrossSubsettingAtMostOne, codeCrossSubsettingChain, codeCrossFeatureType} {
		if got := only(diags, code); len(got) != 0 {
			t.Errorf("%s fired on well-formed ends: %v", code, got)
		}
	}
}

// The inline cross feature spelling is shared with SysML connection ends.
func TestW10BCrossFeatureSpecializationInlineCrossFeatureSysML(t *testing.T) {
	const src = `package P {
		part def Person { part friends : Person[*]; }
		part def Pet { part owners : Person[*]; }
		connection def Owns {
			end owners [0..*] part owner : Person;
			end ref pet : Pet;
		}
		connection def Sub :> Owns {
			end ref owner2 : Person redefines owner crosses pet2.owners;
			end ref pet2 : Pet redefines pet;
		}
		connection def Sub2 :> Owns {
			end owners2 [0..*] part owner2 : Person redefines owner;
			end ref pet2 : Pet redefines pet;
		}
	}`
	diags := constraintDiags(t, src)
	expectCrossDiag(t, src, diags, codeCrossFeatureSpecialization, msgCrossSpecialization, "pet2.owners")
	if got := only(diags, codeCrossFeatureSpecialization); len(got) != 1 {
		t.Errorf("expected the one crossing of an unrelated feature, got %v", got)
	}
}

// The constraints are KerML-owned but reachable from SysML connection
// definitions and part usages.
func TestW10BCrossSubsettingSysML(t *testing.T) {
	const src = `package P {
		part def Person { part pets : Pet[*]; part friends : Person[*]; }
		part def Pet { part owners : Person[*]; }
		connection def Owns {
			end owner : Person crosses pet.owners;
			end pet : Pet crosses owner.pets;
		}
		connection def NotAChain {
			end owner : Person crosses pet;
			end pet : Pet crosses owner.pets crosses owner.friends;
		}
		part def Holder {
			part p : Person;
			part q : Person crosses p.friends;
		}
		connection def Sub :> Owns {
			end owner2 : Person redefines owner crosses pet2.owners;
			end pet2 : Pet redefines pet crosses owner2.friends;
		}
	}`
	diags := constraintDiags(t, src)
	expectCrossDiag(t, src, diags, codeCrossSubsettingChain, msgCrossSubsettingChain, "pet")
	expectCrossDiag(t, src, diags, codeCrossSubsettingAtMostOne, msgCrossSubsettingAtMostOne, "owner.friends")
	expectCrossDiag(t, src, diags, codeCrossSubsettingOwner, msgCrossSubsettingOwner, "p.friends")
	expectCrossDiag(t, src, diags, codeCrossFeatureType, msgCrossFeatureType, "owner2.friends")
	expectCrossDiag(t, src, diags, codeCrossFeatureSpecialization, msgCrossSpecialization, "owner2.friends")
}

// A connection definition may redefine one inherited end and cross through the
// other, which it inherits; the inherited end counts toward the two required.
func TestW10BCrossSubsettingSysMLInheritedEndClean(t *testing.T) {
	const src = `package P {
		part def Person { part pets : Pet[*]; }
		part def Pet { part owners : Person[*]; }
		connection def Owns {
			end owner : Person crosses pet.owners;
			end pet : Pet crosses owner.pets;
		}
		connection def Sub :> Owns {
			end owner2 : Person redefines owner crosses pet.owners;
		}
	}`
	expectNoCrossDiags(t, constraintDiags(t, src))
}

// A cross feature declared ahead of its end's kind keyword is compared by its
// own types: typing it more narrowly than the end is an error at the cross
// feature, while the same type plus its own subsetting is well-formed
// (pilot-confirmed with validate-kerml and validate-sysml-batch).
func TestW10BNamedCrossFeatureTypeAheadOfKind(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"kerml operators", `package P {
			class C1; class C2;
			class Sub1 :> C1;
			feature g : C1;
			assoc A {
				end x1 : Sub1 [0..1] :> g feature x : C1;
				end y1 [0..1] feature y : C2;
			}
		}`, "x1 : Sub1 [0..1] :> g"},
		{"kerml keywords", `package P {
			class C1; class C2;
			class Sub1 :> C1;
			feature g : C1;
			assoc A {
				end x1 [0..1] typed by Sub1 subsets g feature x : C1;
				end y1 [0..1] feature y : C2;
			}
		}`, "x1 [0..1] typed by Sub1 subsets g"},
		{"kerml same type", `package P {
			class C1; class C2;
			class Sub1 :> C1;
			feature g : C1;
			assoc A {
				end x1 : C1 [0..1] :> g feature x : C1;
				end y1 [0..1] feature y : C2;
			}
		}`, ""},
		{"sysml operators", `package P {
			part def C1; part def C2;
			part def Sub1 :> C1;
			item g : C1;
			connection def A {
				end x1 : Sub1 [0..1] :> g item x : C1;
				end y1 [0..1] item y : C2;
			}
		}`, "x1 : Sub1 [0..1] :> g"},
		{"sysml same type", `package P {
			part def C1; part def C2;
			part def Sub1 :> C1;
			item g : C1;
			connection def A {
				end x1 : C1 [0..1] :> g item x : C1;
				end y1 [0..1] item y : C2;
			}
		}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diags []diag.Diagnostic
			if strings.HasPrefix(tc.name, "kerml") {
				diags = constraintDiagsKerML(t, tc.src)
			} else {
				diags = constraintDiags(t, tc.src)
			}
			if tc.want == "" {
				expectNoCrossDiags(t, diags)
				return
			}
			expectCrossDiag(t, tc.src, diags, codeCrossFeatureType, msgCrossFeatureType, tc.want)
		})
	}
}
