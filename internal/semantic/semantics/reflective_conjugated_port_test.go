package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A port definition owns the ConjugatedPortDefinition it implies as its first
// member, and the conjugate owns one PortConjugation back (SysML 8.3.11.3).
func TestPortDefinitionOwnsItsConjugate(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			port def <q> Q { attribute x; }
			port def <s>;
			part def R;
		}
	`)
	pkg, _ := root.LookupLocal("P")
	q := member(t, m, pkg, "Q")
	r := member(t, m, pkg, "R")

	memberships := m.OwnedMemberships(q)
	if len(memberships) != 2 {
		t.Fatalf("owned memberships of Q = %d, want the conjugate's then x's", len(memberships))
	}
	membership := memberships[0]
	conjugate := membership.Implicit.Member
	if conjugate == nil || !conjugate.IsConjugatedPortDefinition() || !conjugate.Implied() {
		t.Fatalf("first membership's member = %v, want the implied conjugate", conjugate)
	}
	if conjugate.Name != "~Q" || conjugate.ShortName != "" {
		t.Errorf("conjugate named %q <%s>, want ~Q with no short name", conjugate.Name, conjugate.ShortName)
	}
	if got := m.MetaclassOf(conjugate); got == nil || got.Name != "ConjugatedPortDefinition" {
		t.Errorf("conjugate metaclass = %v, want ConjugatedPortDefinition", got)
	}
	if got := m.MetaclassOf(membership); got == nil || got.Name != "OwningMembership" {
		t.Errorf("membership metaclass = %v, want OwningMembership", got)
	}
	if got, _ := m.ReflectiveElements(q, "ownedMember"); len(got) != 2 || got[0] != conjugate {
		t.Errorf("ownedMember of Q = %v, want the conjugate first", got)
	}
	if got, ok := m.LookupMember(q, "~Q"); !ok || got != conjugate {
		t.Errorf("Q::'~Q' resolves to %v, want the conjugate", got)
	}

	rels := m.OwnedRelationships(conjugate)
	if len(rels) != 1 {
		t.Fatalf("owned relationships of ~Q = %d, want its PortConjugation", len(rels))
	}
	conjugation := rels[0]
	if got := m.MetaclassOf(conjugation); got == nil || got.Name != "PortConjugation" {
		t.Errorf("conjugation metaclass = %v, want PortConjugation", got)
	}
	for feature, want := range map[string]*symbols.Symbol{
		"source": conjugate, "conjugatedType": conjugate, "owningRelatedElement": conjugate,
		"target": q, "originalType": q, "originalPortDefinition": q,
	} {
		if got, ok := m.ReflectiveElements(conjugation, feature); !ok || len(got) != 1 || got[0] != want {
			t.Errorf("%s of the conjugation = %v, want %s", feature, got, want.Name)
		}
	}
	if got, ok := m.ReflectiveElements(conjugation, "relatedElement"); !ok || len(got) != 2 || got[0] != conjugate || got[1] != q {
		t.Errorf("relatedElement of the conjugation = %v, want [~Q Q]", got)
	}
	if got, ok := m.ReflectiveElements(conjugate, "ownedConjugator"); !ok || len(got) != 1 || got[0] != conjugation {
		t.Errorf("ownedConjugator of ~Q = %v, want its PortConjugation", got)
	}
	if got, ok := m.ReflectiveFeatureValue(conjugate, "isConjugated"); !ok || !got.Bool {
		t.Errorf("isConjugated of ~Q = %v, %v; want true", got, ok)
	}
	if got, ok := m.ReflectiveFeatureValue(q, "isConjugated"); !ok || got.Bool {
		t.Errorf("isConjugated of Q = %v, %v; want false", got, ok)
	}
	if symbols.KeyOf(q) == symbols.KeyOf(conjugate) || symbols.KeyOf(q).String() == symbols.KeyOf(conjugate).String() {
		t.Errorf("Q and ~Q share the element key %s", symbols.KeyOf(q))
	}
	if got := m.OwnedMemberships(conjugate); len(got) != 0 {
		t.Errorf("the conjugate owns %d memberships, want none", len(got))
	}
	short, ok := pkg.Scope.LookupLocal("s")
	if !ok {
		t.Fatal("port def <s> not found")
	}
	if got := m.OwnedMemberships(short); len(got) != 1 || got[0].Implicit.Member.ShortName != "" || got[0].Implicit.Member.Name != "" {
		t.Errorf("a short-named port definition owns %v, want its anonymous conjugate first", got)
	}
	if got := m.OwnedMemberships(r); len(got) != 0 {
		t.Errorf("a part definition owns %d memberships, want none", len(got))
	}
}

func TestConjugatedPortDefinitionNamesFollowTheOriginalName(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
	port def <s>;
	port def <q> Q;
	port def R;
	part def U {
		port a : ~s;
		port b : ~Q;
		port c : ~R;
	}
}`)
	pkg, _ := root.LookupLocal("P")
	short := member(t, m, pkg, "s")
	named := member(t, m, pkg, "Q")
	plain := member(t, m, pkg, "R")
	u := member(t, m, pkg, "U")
	shortConjugate := m.OwnedMemberships(short)[0].Implicit.Member
	namedConjugate, _ := m.LookupMember(named, "~Q")
	plainConjugate, _ := m.LookupMember(plain, "~R")
	if got, ok := m.ReflectiveElements(short, "ownedMember"); !ok || len(got) == 0 || got[0] != shortConjugate {
		t.Errorf("ownedMember of <s> = %v, want its conjugate first", got)
	}

	for feature, want := range map[string]string{
		"name": "", "declaredName": "", "shortName": "", "declaredShortName": "", "qualifiedName": "",
	} {
		assertReflectiveString(t, m, shortConjugate, feature, want)
	}
	// Native qualifiedName spells `~Q` unquoted; the pilot uses `P::Q::'~Q'`.
	for feature, want := range map[string]string{
		"name": "~Q", "declaredName": "~Q", "shortName": "", "declaredShortName": "",
		"qualifiedName": "P::Q::~Q",
	} {
		assertReflectiveString(t, m, namedConjugate, feature, want)
	}
	for feature, want := range map[string]string{
		"name": "~R", "declaredName": "~R", "shortName": "", "declaredShortName": "",
	} {
		assertReflectiveString(t, m, plainConjugate, feature, want)
	}

	for _, tc := range []struct {
		port     string
		original *symbols.Symbol
	}{
		{"a", short}, {"b", named}, {"c", plain},
	} {
		port := member(t, m, u, tc.port)
		rels := m.ImplicitRelationships(port)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(%s) = %d, want one conjugated typing", tc.port, len(rels))
		}
		if got := m.MetaclassOf(rels[0]); got == nil || got.Name != "ConjugatedPortTyping" {
			t.Fatalf("%s typing metaclass = %v, want ConjugatedPortTyping", tc.port, got)
		}
		if got, ok := m.ReflectiveElements(rels[0], "portDefinition"); !ok || len(got) != 1 || got[0] != tc.original {
			t.Errorf("%s portDefinition = %v, want original %s", tc.port, got, tc.original.Name)
		}
		if got, conjugated := m.portTypeOf(port); got != tc.original || !conjugated {
			t.Errorf("%s port type = %v (conjugated %t), want conjugated %s", tc.port, got, conjugated, tc.original.Name)
		}
	}
	if len(m.resolver.Diagnostics) != 0 {
		t.Fatalf("model has resolver diagnostics after resolving port typings: %v", m.resolver.Diagnostics)
	}
}

func assertReflectiveString(t *testing.T, m *Model, sym *symbols.Symbol, feature, want string) {
	t.Helper()
	got, ok := m.ReflectiveFeatureValue(sym, feature)
	if !ok {
		t.Fatalf("%s on %s is unsupported", feature, sym.Name)
	}
	if want == "" {
		if got.Kind != symbols.FilterValueEmpty {
			t.Errorf("%s on %s = %v, want null", feature, sym.Name, got)
		}
		return
	}
	if got.Kind != symbols.FilterValueString || got.Str != want {
		t.Errorf("%s on %s = %v, want %q", feature, sym.Name, got, want)
	}
}
