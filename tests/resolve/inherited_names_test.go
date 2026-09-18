package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A redefinition target is searched up the whole specialization chain, not only
// the immediate supertype (KerML 1.0 §7.3.4.5).
func TestRedefinitionTargetFoundTwoSupertypesUp(t *testing.T) {
	r, _, docRoot := resolvedDoc(t, `package P {
		part def Vehicle { part wheel; }
		part def Car :> Vehicle;
		part def Roadster :> Car {
			part frontWheel :>> wheel;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, docRoot, "P")
	roadster := local(t, pkg.Scope, "Roadster")
	want := local(t, local(t, pkg.Scope, "Vehicle").Scope, "wheel")

	target := redefinitionTarget(t, local(t, roadster.Scope, "frontWheel"))
	got, ok := r.ResolveQualified(roadster.Scope, target)
	if !ok {
		t.Fatal("the redefinition target did not resolve")
	}
	if got != want {
		t.Errorf("frontWheel redefines %s, want the wheel Vehicle declares", got.Name)
	}
}

func TestRedefinitionThroughAliasSupertype(t *testing.T) {
	r, _, docRoot := resolvedDoc(t, `package P {
		part def Base { attribute length; }
		alias BaseAlias for Base;
		alias BaseAlias2 for BaseAlias;
		part def Derived :> BaseAlias2 {
			attribute :>> length;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, docRoot, "P")
	derived := local(t, pkg.Scope, "Derived")
	base := local(t, pkg.Scope, "Base")
	want := local(t, base.Scope, "length")
	redefined := local(t, derived.Scope, "length")
	target := redefinitionTarget(t, redefined)
	got, ok := r.ResolveQualified(derived.Scope, target)
	if !ok || got != want {
		t.Fatalf("alias-supertype redefinition target = %v, %v; want %v", got, ok, want)
	}
}

// Redeclaring an inherited name is indistinguishable from it unless the
// declaration redefines what it shares the name with: a redefined feature is no
// longer inherited (SysML v2 §7.6.1, KerML §7.2.2). Reference subsetting keeps
// it, so the reference reports the duplicate there — matched runs, w6c.
func TestRedeclaringAnInheritedNameConflictsUnlessItRedefines(t *testing.T) {
	for _, tc := range []struct {
		name     string
		member   string
		conflict bool
	}{
		{"plain redeclaration", "part wheel;", true},
		{"redefinition", "part wheel :>> wheel;", false},
		{"reference", "ref part wheel references wheel;", true},
		{"subsetting", "part wheel :> wheel;", true},
		{"another name", "part spare;", false},
	} {
		r, _, _ := resolvedDoc(t, `package P {
			part def Vehicle { part wheel; }
			part def Car :> Vehicle { `+tc.member+` }
		}`)
		conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
		if got := len(conflicts) != 0; got != tc.conflict {
			t.Errorf("%s: name conflict reported = %v, want %v (%v)", tc.name, got, tc.conflict, r.Diagnostics)
		}
		if len(conflicts) != 0 && !strings.Contains(conflicts[0].Message, "'wheel' from Vehicle") {
			t.Errorf("%s: conflict message %q does not name the inherited feature", tc.name, conflicts[0].Message)
		}
	}
}

// A same-named explicit redefinition resolves against the inherited feature,
// not the member that borrows its name (KerML 8.3.3.1, 8.4.3.2).
func TestSameNamedRedefinitionReachesInheritedFeature(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def Vehicle {
			attribute mass;
			attribute dryMass;
		}
		part vehicle_b : Vehicle {
			attribute mass redefines mass;
			attribute dryMass redefines dryMass;
		}
	}`)
	if conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict); len(conflicts) != 0 {
		t.Fatalf("conflicts = %v, want same-named redefinitions to remove their inherited features", conflicts)
	}
}

// A member named by the feature it performs is `a` for its owner and for every
// chain reaching the owner, and duplicates the inherited `a` it does not
// redefine; a plain assertion derives no name at all — matched runs.
func TestReferenceDerivedNameIsAMemberInsideAndOut(t *testing.T) {
	r, _, docRoot := resolvedDoc(t, `package P {
		part def H { action a { attribute i; } part q; }
		part h : H { perform a; ref x = a.i; assert q; }
		part ctx { ref y = h.a.i; ref z = h.q; }
	}`)
	pkg := local(t, docRoot, "P")
	h := local(t, pkg.Scope, "h")
	a := local(t, h.Scope, "a")
	if a.Naming != symbols.NamedByReference {
		t.Errorf("a naming = %v, want the performed action's", a.Naming)
	}
	if _, ok := h.Scope.LookupLocal("q"); ok {
		t.Errorf("h members = %v; assert q must not be a member q", h.Scope.MemberNames())
	}
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 || conflicts[0].Message != "Duplicate of inherited member name 'a' from H" {
		t.Errorf("conflicts = %v, want the one performed a draws", r.Diagnostics)
	}
	if len(r.Diagnostics) != len(conflicts) {
		t.Errorf("diagnostics = %v, want the chains through h to resolve", r.Diagnostics)
	}
}

// A same-named subsetting target is the inherited feature: a feature cannot
// specialize itself. The fix belongs in resolveSpecialization ("Found, not fixed").
func TestSubsettingTargetIsTheInheritedFeature(t *testing.T) {
	t.Skip("known bug: a same-named subsetting target binds to the subsetting feature itself")
	r, _, docRoot := resolvedDoc(t, `package P {
		part def Vehicle { part wheel; }
		part def Car :> Vehicle { part wheel :> wheel; }
	}`)
	pkg := local(t, docRoot, "P")
	subsetting := local(t, local(t, pkg.Scope, "Car").Scope, "wheel")
	inherited := local(t, local(t, pkg.Scope, "Vehicle").Scope, "wheel")
	target := specializationTarget(t, subsetting)
	got, ok := r.ResolveQualified(local(t, pkg.Scope, "Car").Scope, target)
	if !ok {
		t.Fatal("the subsetting target did not resolve")
	}
	if got == subsetting {
		t.Error("the subsetting target names the feature declaring it")
	}
	if got != inherited {
		t.Errorf("the subsetting target names %s, want the wheel Vehicle declares", got.Name)
	}
}

func TestInheritedMembersThroughGeneralFeature(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		src  string
	}{
		{
			name: "KerML",
			file: "inherited.kerml",
			src: `package R10 {
				classifier A { feature f; }
				classifier B specializes A { feature redefines f { feature g; } }
				classifier C specializes A, B {
					feature subsets f { feature redefines g; }
				}
			}`,
		},
		{
			name: "SysML",
			file: "inherited.sysml",
			src: `package P {
				part def A { part f; }
				part def B :> A { part redefines f { part g; } }
				part def C :> A, B { part redefines f { part redefines g; } }
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _ := resolvedDocNamed(t, tc.file, tc.src)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
			}
		})
	}
}

// A name inherited from two supertypes at once is ambiguous with no member of
// the subtype at fault, so the reference reports it on the subtype itself
// (SysML v2 §7.6.1) — matched run, w6c.
func TestNameInheritedFromTwoSupertypesIsReportedOnTheSubtype(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def L1 { attribute p; }
		part def R1 { attribute p; }
		part def D2 :> L1, R1;
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one for the diamond-inherited name", r.Diagnostics)
	}
	if want := "Duplicate of inherited member name 'p' from L1, R1"; conflicts[0].Message != want {
		t.Errorf("message = %q, want %q", conflicts[0].Message, want)
	}
	if !conflicts[0].Warning {
		t.Error("the diamond conflict is an error, want a warning")
	}
}

// Two inherited redefinitions of one ancestor feature are still two features
// under one name: neither redefines the other, so the subtype and a usage
// typed by one while subsetting the other both conflict — matched against the
// pinned validate-sysml, which reports the same two warnings.
func TestInheritedRedefinitionsOfACommonAncestorConflict(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def S { attribute d; }
		part def A :> S { attribute :>> d = 1; }
		part def B :> S { attribute :>> d = 2; }
		part def C {
			part b : B;
			part x : A :> b;
		}
		part def D :> A, B;
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %v, want two", r.Diagnostics)
	}
	for _, c := range conflicts {
		if want := "Duplicate of inherited member name 'd' from A, B"; c.Message != want {
			t.Errorf("message = %q, want %q", c.Message, want)
		}
	}
}

// An owned member redefining every colliding inherited namesake owns the name;
// one redefining only some leaves the rest in conflict — matched against the
// pinned validate-sysml.
func TestOwnedRedefinitionOfCollidingInheritedNames(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def S { attribute d; }
		part def A :> S { attribute :>> d = 1; }
		part def B :> S { attribute :>> d = 2; }
		part def L1 { attribute p; }
		part def R1 { attribute p; }
		part def C {
			part b : B;
			part x : A :> b { attribute :>> A::d, b::d; }
		}
		part def D :> L1, R1 { attribute :>> L1::p, R1::p; }
		part def E :> L1, R1 { attribute :>> L1::p; }
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 || len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one conflict for E's unredefined p", r.Diagnostics)
	}
	if want := "Duplicate of inherited member name 'p' from R1"; conflicts[0].Message != want {
		t.Errorf("message = %q, want %q", conflicts[0].Message, want)
	}
}

// A supertype's non-private imports are memberships it has, so a subtype
// inherits the imported names too (KerML §8.4.3.2) — Xpect
// ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.
func TestNameImportedByASupertypeIsInherited(t *testing.T) {
	for _, tc := range []struct {
		name       string
		visibility string
		conflict   bool
	}{
		{"public import", "public ", true},
		{"private import", "private ", false},
	} {
		r, _, _ := resolvedDoc(t, `package P {
			package Outer { part def B; }
			part def Inner { `+tc.visibility+`import Outer::*; }
			part def Sub :> Inner { part B; }
		}`)
		conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
		if !tc.conflict {
			if len(conflicts) != 0 {
				t.Errorf("%s: conflicts = %v, want none", tc.name, conflicts)
			}
			continue
		}
		if len(conflicts) != 1 {
			t.Fatalf("%s: conflicts = %v, want one for the inherited import", tc.name, r.Diagnostics)
		}
		if want := "Duplicate of inherited member name 'B' from Outer"; conflicts[0].Message != want {
			t.Errorf("%s: message = %q, want %q", tc.name, conflicts[0].Message, want)
		}
	}
}

// A feature an intermediate supertype redefines is still inherited by that
// supertype's own subtypes under its name, so redeclaring it there conflicts.
func TestNameRedefinedByAnIntermediateSupertypeStillConflicts(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def B { attribute p; }
		part def M :> B { attribute :>> p; }
		part def D :> M { attribute p; }
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one for the name M redefines", r.Diagnostics)
	}
	if want := "Duplicate of inherited member name 'p' from M"; conflicts[0].Message != want {
		t.Errorf("message = %q, want %q", conflicts[0].Message, want)
	}
}

// Redefining one side of a diamond drops only what that side redefines: the
// other side's redefinition of the shared feature is still inherited under its
// name, while the shared feature itself is not — Xpect Redefinition_Diamond1.
func TestRedefiningOneSideOfADiamondKeepsTheOtherSidesName(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def A { part p[*]; }
		part def A1 :> A { part p1 :>> p; }
		part def A2 :> A { part p :>> p; }
		part def B :> A1, A2 { part p2 :>> p1; part p; }
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one for the p A2 still contributes", r.Diagnostics)
	}
	if want := "Duplicate of inherited member name 'p' from A2"; conflicts[0].Message != want {
		t.Errorf("message = %q, want %q", conflicts[0].Message, want)
	}
}

// A redefinition of one inherited feature also stops what that feature redefines
// from being inherited, so a subtype redefining the nearer of two inherited
// namesakes owns the name alone — matched run.
func TestRedefiningTheNearerOfTwoInheritedNamesakesOwnsTheName(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def A { part f { part a; } }
		part def B :> A { part redefines f { part g; } }
		part def C :> A, B { part redefines f { part redefines g; part redefines a; } }
		part def D :> A, B { ref x :> f.g; ref y :> f.a; }
	}`)
	if len(r.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", r.Diagnostics)
	}
}

// A parameter inherited twice under one name is one feature when the owner of
// one specializes the owner of the other, which implicitly redefines it —
// matched run, w6c.
func TestParameterInheritedThroughItsOwnSupertypeIsNotAmbiguous(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		action def A { in p; }
		action def Outer { action a : A { in p; } }
		action def Sub :> Outer { action :>> a : A; }
	}`)
	if conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict); len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none for the implicitly redefined parameter", conflicts)
	}
}

// Two parameters of unrelated supertypes are still ambiguous — matched run.
func TestParameterInheritedFromTwoUnrelatedSupertypesIsAmbiguous(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		action def L { in p; }
		action def R { in p; }
		action def D :> L, R;
	}`)
	conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one for the diamond-inherited parameter", r.Diagnostics)
	}
	if want := "Duplicate of inherited member name 'p' from L, R"; conflicts[0].Message != want {
		t.Errorf("message = %q, want %q", conflicts[0].Message, want)
	}
}

// The subject, actors and stakeholders of a requirement redefine the inherited
// ones by name, which is not modelled here, so redeclaring them is no conflict.
func TestRequirementParametersAreNotNameConflicts(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def Vehicle;
		requirement def Weight { subject vehicle : Vehicle; }
		requirement def MaxWeight :> Weight { subject vehicle : Vehicle; }
	}`)
	if conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict); len(conflicts) != 0 {
		t.Errorf("name conflicts reported for a requirement's subject: %v", conflicts)
	}
}

// A redefinition removes what its owner inherits, never a feature declared
// beside it: a chain through a feature typed by the owner still reaches the
// sibling (KerML 8.3.3.3.6) — pilot-refereed, both cases.
func TestRedefiningASiblingMasksNothingThroughAChain(t *testing.T) {
	for _, tc := range []struct {
		name     string
		src      string
		resolves bool
	}{
		{
			name: "sibling",
			src: `package K {
				classifier C {
					feature xs;
					feature sub : C[0..1];
					feature slice redefines xs;
					inv { sub.xs }
				}
			}`,
			resolves: true,
		},
		{
			name: "inherited",
			src: `package K {
				classifier G { feature xs; }
				classifier C specializes G {
					feature sub : C[0..1];
					feature slice redefines xs;
					inv { sub.xs }
				}
			}`,
			resolves: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root, docRoot := resolvedDocNamed(t, "k.kerml", tc.src)
			var chains []resolve.Reference
			for _, ref := range resolve.References(root, docRoot) {
				if ref.Chain != nil && nameText(ref.QN) == "xs" {
					chains = append(chains, ref)
				}
			}
			if len(chains) != 1 {
				t.Fatalf("chain references = %d, want the one sub.xs", len(chains))
			}
			got, ok := r.ResolveReference(chains[0])
			if ok != tc.resolves {
				t.Fatalf("sub.xs resolved = %v, want %v", ok, tc.resolves)
			}
			if !ok {
				return
			}
			want := local(t, local(t, local(t, docRoot, "K").Scope, "C").Scope, "xs")
			if got != want {
				t.Errorf("sub.xs = %s, want the xs C declares", got.Name)
			}
		})
	}
}

// local is the single member scope holds under name.
func local(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s is not a local member", name)
	}
	return sym
}

// redefinitionTarget is the name the redefinition relationship of sym names.
func redefinitionTarget(t *testing.T, sym *symbols.Symbol) *ast.QualifiedName {
	t.Helper()
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("%s is declared by %T, want a usage", sym.Name, sym.Decl)
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil {
			return qn
		}
	}
	t.Fatalf("%s declares no redefinition of a name", sym.Name)
	return nil
}

// specializationTarget is the name the subsetting relationship of sym names.
func specializationTarget(t *testing.T, sym *symbols.Symbol) *ast.QualifiedName {
	t.Helper()
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("%s is declared by %T, want a usage", sym.Name, sym.Decl)
	}
	for _, rel := range usage.Relationships {
		if rel == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelSpecializes) {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil {
			return qn
		}
	}
	t.Fatalf("%s declares no subsetting of a name", sym.Name)
	return nil
}

func diagnosticsWithCode(r *resolve.Resolver, code string) []resolve.Diagnostic {
	var out []resolve.Diagnostic
	for _, d := range r.Diagnostics {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
