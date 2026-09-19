package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestReferencedFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	if got := m.ReferencedFeature(takePhoto); got != takePicture {
		t.Fatalf("ReferencedFeature(takePhoto) = %v, want takePicture", got)
	}
	if got := m.ReferencedFeature(takePicture); got != nil {
		t.Fatalf("ReferencedFeature(takePicture) = %v, want nil", got)
	}
}

// Reference subsetting contributes members without making the referenced
// feature a supertype: it is not a conformance edge (KerML 8.3.3.3.9).
func TestReferenceContributesMembersNotSupertypes(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; action shoot; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	for _, name := range []string{"focus", "shoot"} {
		if _, ok := m.LookupMember(takePhoto, name); !ok {
			t.Errorf("LookupMember(takePhoto, %q) not found", name)
		}
	}
	if names := memberNames(m.MembersOf(takePhoto)); !names["focus"] || !names["shoot"] {
		t.Errorf("MembersOf(takePhoto) = %v, want focus and shoot", names)
	}

	for _, sup := range m.AllSupertypes(takePhoto) {
		if sup == takePicture {
			t.Fatalf("takePicture must not be a supertype of takePhoto")
		}
	}
	if !contains(m.MemberSources(takePhoto), takePicture) {
		t.Fatalf("MemberSources(takePhoto) = %v, want takePicture", m.MemberSources(takePhoto))
	}
}

// The performed action is named by a feature chain, and members inherited by
// the referenced feature are reachable through it.
func TestReferenceThroughFeatureChain(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def ProvidePower { action generateTorque { action rev; } }
		action providePower : ProvidePower;
		part torqueGenerator { perform providePower.generateTorque; }
	}`)

	pkg := sym(t, root, "P")
	generator := sym(t, pkg.Scope, "torqueGenerator")
	perform := sym(t, generator.Scope, "generateTorque")

	if _, ok := m.LookupMember(perform, "rev"); !ok {
		t.Fatalf("LookupMember(generateTorque, rev) not found")
	}
}

// A perform statement binds the referenced feature's name in the scope the
// reference itself resolves in; the reference names the outer feature.
func TestReferenceResolvesOutsideItsOwnName(t *testing.T) {
	m, root := buildModel(t, `package P {
		action providePower { action generateTorque; }
		part vehicle { perform providePower; }
	}`)

	pkg := sym(t, root, "P")
	vehicle := sym(t, pkg.Scope, "vehicle")
	perform := sym(t, vehicle.Scope, "providePower")
	action := sym(t, pkg.Scope, "providePower")

	if perform == action {
		t.Fatalf("the perform statement should declare its own feature")
	}
	if got := m.ReferencedFeature(perform); got != action {
		t.Fatalf("ReferencedFeature(perform providePower) = %v, want the action", got)
	}
	if _, ok := m.LookupMember(perform, "generateTorque"); !ok {
		t.Fatalf("LookupMember(perform providePower, generateTorque) not found")
	}
}

// A reference cycle must not hang member lookup.
func TestReferenceCycleTerminates(t *testing.T) {
	m, root := buildModel(t, `package P {
		action a references b;
		action b references a;
	}`)

	pkg := sym(t, root, "P")
	if _, ok := m.LookupMember(sym(t, pkg.Scope, "a"), "nothing"); ok {
		t.Fatalf("unexpected member found")
	}
}

func TestGeneralizationKindExcludesReferences(t *testing.T) {
	if GeneralizationKind(ast.RelReferences) {
		t.Fatalf("reference subsetting must not be a generalization edge")
	}
}

func contains(syms []*symbols.Symbol, want *symbols.Symbol) bool {
	for _, s := range syms {
		if s == want {
			return true
		}
	}
	return false
}

// A perform statement declared beside the action it performs must still find
// that sibling: only its own effective-name binding is skipped, not the whole
// scope that binding lives in.
func TestReferenceFindsSiblingDeclaredAfterIt(t *testing.T) {
	m, root := buildModel(t, `package P {
		part vehicle {
			perform providePower;
			action providePower { action generateTorque; }
		}
	}`)

	pkg := sym(t, root, "P")
	vehicle := sym(t, pkg.Scope, "vehicle")

	var perform, action *symbols.Symbol
	for _, s := range vehicle.Scope.LookupLocalAll("providePower") {
		if usage, ok := s.Decl.(*ast.Usage); ok && usage.Ident.Name == "" {
			perform = s
		} else {
			action = s
		}
	}
	if perform == nil || action == nil {
		t.Fatalf("expected both a perform statement and an action named providePower")
	}
	if got := m.ReferencedFeature(perform); got != action {
		t.Fatalf("ReferencedFeature(perform) = %v, want the sibling action", got)
	}
	if _, ok := m.LookupMember(perform, "generateTorque"); !ok {
		t.Errorf("LookupMember(perform, \"generateTorque\") not found")
	}
}

// A reference resolved while another reference is in flight saw a truncated
// member view (the in-flight symbol's own reference is hidden by the cycle
// guard), so its result is provisional and must not be memoized.
func TestNestedReferenceResultIsNotCached(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	delete(m.referenced, takePhoto)
	m.resolvingRef[takePicture] = true
	got := m.ReferencedFeature(takePhoto)
	delete(m.resolvingRef, takePicture)

	if got != takePicture {
		t.Fatalf("ReferencedFeature(takePhoto) = %v, want takePicture", got)
	}
	if _, cached := m.referenced[takePhoto]; cached {
		t.Errorf("a result computed under an in-flight resolution was memoized")
	}
	if got := m.ReferencedFeature(takePhoto); got != takePicture {
		t.Errorf("ReferencedFeature(takePhoto) = %v after recompute, want takePicture", got)
	}
}

// A member view enumerated while a reference is in flight is provisional, and
// MemberSourcesStable says so; once the finished view is memoized it says stable.
func TestMemberSourcesStableTracksProvisionalViews(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	if m.MemberSourcesStable(takePhoto) {
		t.Fatalf("MemberSourcesStable true before any member query")
	}

	delete(m.memberSources, takePhoto)
	delete(m.referenced, takePhoto)
	m.resolvingRef[takePicture] = true
	m.MembersOf(takePhoto)
	if m.MemberSourcesStable(takePhoto) {
		t.Errorf("a member view computed under an in-flight resolution reported stable")
	}
	delete(m.resolvingRef, takePicture)

	if names := memberNames(m.MembersOf(takePhoto)); !names["focus"] {
		t.Fatalf("MembersOf(takePhoto) = %v after recompute, want focus", names)
	}
	if !m.MemberSourcesStable(takePhoto) {
		t.Errorf("a finished member view did not report stable")
	}
}

// Two perform statements for the same action in one body both take its
// effective name; neither may resolve to the other.
func TestRepeatedPerformResolvesToTheAction(t *testing.T) {
	m, root := buildModel(t, `package P {
		action increment { action bump; }
		action outer {
			perform increment;
			perform increment;
		}
	}`)

	pkg := sym(t, root, "P")
	outer := sym(t, pkg.Scope, "outer")
	increment := sym(t, pkg.Scope, "increment")

	performs := outer.Scope.LookupLocalAll("increment")
	if len(performs) != 2 {
		t.Fatalf("perform statements bound = %d, want 2", len(performs))
	}
	for i, perform := range performs {
		if got := m.ReferencedFeature(perform); got != increment {
			t.Errorf("ReferencedFeature(perform %d) = %v, want the action", i, got)
		}
	}
}

// A perform statement and a declaration may share a name in one scope; a
// qualified path through that scope names the declaration, not the statement.
func TestQualifiedNameThroughEffectiveNameIsNotAmbiguous(t *testing.T) {
	m, root := buildModel(t, `package P {
		part vehicle {
			perform providePower;
			action providePower { action generateTorque; }
		}
		part other { perform vehicle.providePower.generateTorque; }
	}`)

	pkg := sym(t, root, "P")
	vehicle := sym(t, pkg.Scope, "vehicle")
	other := sym(t, pkg.Scope, "other")
	action := sym(t, vehicle.Scope, "providePower")
	generateTorque := sym(t, action.Scope, "generateTorque")

	perform := other.Scope.LookupLocalAll("generateTorque")
	if len(perform) != 1 {
		t.Fatalf("perform statements bound in other = %d, want 1", len(perform))
	}
	if got := m.ReferencedFeature(perform[0]); got != generateTorque {
		t.Fatalf("ReferencedFeature(perform) = %v, want vehicle::providePower::generateTorque", got)
	}
}

// A perform statement may name an action its owner inherits from its type;
// hiding its own borrowed binding must not hide the scope's inherited members.
func TestPerformOfInheritedAction(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Vehicle { action providePower { action generateTorque; } }
		part vehicle : Vehicle { perform providePower; }
	}`)

	pkg := sym(t, root, "P")
	vehicleDef := sym(t, pkg.Scope, "Vehicle")
	vehicle := sym(t, pkg.Scope, "vehicle")
	providePower := sym(t, vehicleDef.Scope, "providePower")

	performs := vehicle.Scope.LookupLocalAll("providePower")
	if len(performs) != 1 {
		t.Fatalf("perform statements bound = %d, want 1", len(performs))
	}
	if got := m.ReferencedFeature(performs[0]); got != providePower {
		t.Fatalf("ReferencedFeature(perform) = %v, want the inherited action", got)
	}
	if _, ok := m.LookupMember(performs[0], "generateTorque"); !ok {
		t.Errorf("LookupMember(perform, \"generateTorque\") not found")
	}
}

// A `require q { ... }` member references q like a reference subsetting: it
// takes q's name and inherits q's members (SysML 7.19.3, KerML 7.3.4.5).
func TestRequireReferenceInheritsTheReferencedMembers(t *testing.T) {
	m, root := buildModel(t, `package P {
		constraint def Q;
		requirement def RD { constraint q : Q { attribute inner; } }
		requirement r : RD { require q { attribute inner2; } }
	}`)

	pkg := sym(t, root, "P")
	rd := sym(t, pkg.Scope, "RD")
	r := sym(t, pkg.Scope, "r")
	q := sym(t, rd.Scope, "q")

	requires := r.Scope.LookupLocalAll("q")
	if len(requires) != 1 {
		t.Fatalf("require members bound as q = %d, want 1", len(requires))
	}
	if got := m.ReferencedFeature(requires[0]); got != q {
		t.Fatalf("ReferencedFeature(require) = %v, want RD::q", got)
	}
	for _, name := range []string{"inner", "inner2"} {
		if _, ok := m.LookupMember(requires[0], name); !ok {
			t.Errorf("LookupMember(require, %q) not found", name)
		}
	}
}

// The body-less `require q;` and `assume q;` are the same reference form: the
// member is named q, references RD::q and inherits its members.
func TestBareRequireAndAssumeReferenceTheNamedConstraint(t *testing.T) {
	m, root := buildModel(t, `package P {
		constraint def Q;
		requirement def RD { constraint q : Q { attribute inner; } }
		requirement r : RD { require q; }
		requirement r2 : RD { assume q; }
	}`)

	pkg := sym(t, root, "P")
	q := sym(t, sym(t, pkg.Scope, "RD").Scope, "q")
	for _, owner := range []string{"r", "r2"} {
		r := sym(t, pkg.Scope, owner)
		refs := r.Scope.LookupLocalAll("q")
		if len(refs) != 1 || refs[0].Naming != symbols.NamedByReference {
			t.Fatalf("%s: members bound as q = %v, want one named by reference", owner, refs)
		}
		if got := m.ReferencedFeature(refs[0]); got != q {
			t.Errorf("%s: ReferencedFeature = %v, want RD::q", owner, got)
		}
		if got, ok := m.LookupMember(r, "q"); !ok || got != refs[0] {
			t.Errorf("%s: LookupMember(q) = %v, want the reference member", owner, got)
		}
		if _, ok := m.LookupMember(refs[0], "inner"); !ok {
			t.Errorf("%s: LookupMember(require, inner) not found", owner)
		}
	}
}

func TestChainedRequireAndAssumeReferenceTheChainsLastFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		constraint def Q;
		part def H { constraint rule : Q { attribute inner; } }
		part h : H;
		requirement r { require h.rule; }
		requirement r2 { assume h.rule; }
	}`)

	pkg := sym(t, root, "P")
	rule := sym(t, sym(t, pkg.Scope, "H").Scope, "rule")
	for _, owner := range []string{"r", "r2"} {
		r := sym(t, pkg.Scope, owner)
		refs := r.Scope.LookupLocalAll("rule")
		if len(refs) != 1 || refs[0].Naming != symbols.NamedByReference {
			t.Fatalf("%s: members bound as rule = %v, want one named by reference", owner, refs)
		}
		if got := m.ReferencedFeature(refs[0]); got != rule {
			t.Errorf("%s: ReferencedFeature = %v, want H::rule", owner, got)
		}
		if got, ok := m.LookupMember(r, "rule"); !ok || got != refs[0] {
			t.Errorf("%s: LookupMember(rule) = %v, want the reference member", owner, got)
		}
		if _, ok := m.LookupMember(refs[0], "inner"); !ok {
			t.Errorf("%s: LookupMember(require, inner) not found", owner)
		}
		if got := r.Scope.LookupLocalAll("h"); len(got) != 0 {
			t.Errorf("%s: the chain's head h is bound as a member: %v", owner, got)
		}
	}
}
