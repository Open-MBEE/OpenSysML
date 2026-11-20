package semantics

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func assertNames(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", label, got, want)
	}
}

// assertMetaclassIn checks that fixtures classify the named symbol as want in a
// document and an index record alike.
func assertMetaclassIn(t *testing.T, name string, kind source.Kind, src string, want map[string]string) {
	t.Helper()
	for _, fixture := range reflectiveFixtures(t, name, kind, src) {
		for symName, metaclass := range want {
			assertRelMetaclass(t, fixture.model, relFixtureSymbol(t, fixture, symName), metaclass)
		}
	}
}

func TestExhibitStateUsageMetaclass(t *testing.T) {
	assertMetaclassIn(t, "exhibit-state.sysml", source.KindSysML, `package P {
    state def T;
    part def Ctx {
        exhibit state s : T;
        state plain : T;
    }
    part def Ctx2 {
        state s2 : T;
        exhibit s2;
    }
}`, map[string]string{"s": "ExhibitStateUsage", "plain": "StateUsage"})
	for _, fixture := range reflectiveFixtures(t, "exhibit-shorthand.sysml", source.KindSysML, `package P {
    state def T;
    part def Ctx {
        state s2 : T;
        exhibit s2;
    }
}`) {
		ctx := relFixtureSymbol(t, fixture, "Ctx")
		var got []string
		for _, membership := range fixture.model.OwnedMemberships(ctx) {
			got = append(got, relMetaName(fixture.model, membership.Implicit.Member))
		}
		assertNames(t, "exhibit s2 members", got, []string{"StateUsage", "ExhibitStateUsage"})
	}
}

func TestIncludeUseCaseUsageMetaclass(t *testing.T) {
	assertMetaclassIn(t, "include-use-case.sysml", source.KindSysML, `package P {
    use case def T;
    part def Ctx {
        include use case u : T;
        use case plain : T;
    }
}`, map[string]string{"u": "IncludeUseCaseUsage", "plain": "UseCaseUsage"})
	for _, fixture := range reflectiveFixtures(t, "include-shorthand.sysml", source.KindSysML, `package P {
    use case def T;
    part def Ctx {
        use case u2 : T;
        include u2;
    }
}`) {
		ctx := relFixtureSymbol(t, fixture, "Ctx")
		var got []string
		for _, membership := range fixture.model.OwnedMemberships(ctx) {
			got = append(got, relMetaName(fixture.model, membership.Implicit.Member))
		}
		assertNames(t, "include u2 members", got, []string{"UseCaseUsage", "IncludeUseCaseUsage"})
	}
}

func TestLibraryPackageMetaclass(t *testing.T) {
	assertMetaclassIn(t, "library-package.sysml", source.KindSysML, `package P {
    library package L;
    package Q;
}`, map[string]string{"L": "LibraryPackage", "Q": "Package"})
	assertMetaclassIn(t, "library-package.kerml", source.KindKerML, `package P {
    library package L;
}`, map[string]string{"L": "LibraryPackage"})
}

func TestExtendedUsageMetaclass(t *testing.T) {
	// SysML.xtext ExtendedUsage returns SysML::Usage; a kindless usage
	// without prefix metadata is a DefaultReferenceUsage.
	assertMetaclassIn(t, "extended-usage-plain.sysml", source.KindSysML, `package P {
    metadata def M;
    #M x;
    part def PD {
        in y;
    }
}`, map[string]string{"x": "Usage", "y": "ReferenceUsage"})
	for i, fixture := range reflectiveFixtures(t, "extended-usage-composite.sysml", source.KindSysML, `package P {
    metadata def M;
    part def PD { #M x; ref #M y; }
}`) {
		for name, composite := range map[string]bool{"x": true, "y": false} {
			sym := relFixtureSymbol(t, fixture, name)
			for feature, want := range map[string]bool{"isReference": !composite, "isComposite": composite} {
				if got, ok := fixture.model.ReflectiveFeatureValue(sym, feature); !ok || got != boolValue(want) {
					t.Errorf("fixture %d: %s %s = %v (%v), want %v: a plain Usage is composite unless declared ref", i, name, feature, got, ok, want)
				}
			}
			if got := fixture.model.EffectiveIsComposite(sym); got != composite {
				t.Errorf("fixture %d: EffectiveIsComposite(%s) = %v, want %v", i, name, got, composite)
			}
		}
	}
}

func TestKerMLSuccessionFlowMetaclass(t *testing.T) {
	assertMetaclassIn(t, "succession-flow.kerml", source.KindKerML, `package P {
    feature fa { feature x; }
    feature fb { feature y; }
    succession flow f from fa.x to fb.y;
    flow g from fa.x to fb.y;
}`, map[string]string{"f": "SuccessionFlow", "g": "Flow"})
	assertMetaclassIn(t, "succession-flow.sysml", source.KindSysML, `package P {
    part def PX { attribute x; }
    part p1 : PX;
    part p2 : PX;
    succession flow f from p1.x to p2.x;
}`, map[string]string{"f": "SuccessionFlowUsage"})
}

// synthesizedMembershipNames lists the metaclass of each owned membership of
// owner and of the member it owns, as "Membership/Member".
func synthesizedMembershipNames(model *Model, owner *symbols.Symbol) []string {
	var out []string
	for _, membership := range model.OwnedMemberships(owner) {
		out = append(out, relMetaName(model, membership)+"/"+relMetaName(model, membership.Implicit.Member))
	}
	return out
}

func TestElementFilterMembershipReflected(t *testing.T) {
	fixture := reflectiveFixtures(t, "element-filter.sysml", source.KindSysML, `package P {
    metadata def M;
    filter @M;
    part def PD;
}`)[0]
	p := relFixtureSymbol(t, fixture, "P")
	var got []string
	for _, membership := range fixture.model.OwnedMemberships(p) {
		got = append(got, relMetaName(fixture.model, membership))
	}
	assertNames(t, "filter memberships", got, []string{"OwningMembership", "ElementFilterMembership", "OwningMembership"})
	condition := fixture.model.OwnedMemberships(p)[1].Implicit.Member
	if !fixture.model.reflectiveMetaclassConforms(condition, "Expression") {
		t.Errorf("filter condition metaclass = %s, want an Expression", relMetaName(fixture.model, condition))
	}
	if owner := fixture.model.ownerOf(condition); owner != p {
		t.Errorf("filter condition owner = %v, want P", owner)
	}
}

func TestElementFilterConditionsKeepDistinctIdentities(t *testing.T) {
	fixture := reflectiveFixtures(t, "element-filters.sysml", source.KindSysML, `package P {
    metadata def M;
    metadata def N;
    filter @M;
    filter @N;
}`)[0]
	p := relFixtureSymbol(t, fixture, "P")
	memberships := fixture.model.OwnedMemberships(p)
	if len(memberships) != 4 {
		t.Fatalf("P owns %d memberships, want 4", len(memberships))
	}
	first, second := memberships[2].Implicit.Member, memberships[3].Implicit.Member
	if symbols.KeyOf(first) == symbols.KeyOf(second) {
		t.Errorf("the two filter conditions share the key %v", symbols.KeyOf(first))
	}
	if *first.Expr == *second.Expr {
		t.Errorf("the two filter conditions share the expression record %+v", *first.Expr)
	}
}

// unresolvedReflectiveFixture indexes src over the standard library without
// resolving it, so the reflective layer resolves the names it reads itself.
func unresolvedReflectiveFixture(t *testing.T, name string, kind source.Kind, src string) reflectiveFixture {
	t.Helper()
	idx := stdlibIndex(t)
	addTestDoc(t, idx, name, src)
	resolver := resolve.New(idx)
	model := NewModel(resolver)
	resolver.SetModel(model)
	return reflectiveFixture{model: model, root: idx.DocumentRoot(name)}
}

func TestElementFilterConditionResolvesInItsNamespaceBeforeResolution(t *testing.T) {
	fixture := unresolvedReflectiveFixture(t, "lazy-filter.sysml", source.KindSysML, `package Outer {
    attribute x : ScalarValues::Boolean;
    package P {
        attribute x : ScalarValues::Boolean;
        filter x;
    }
}`)
	m := fixture.model
	outer := sym(t, fixture.root, "Outer")
	p := sym(t, outer.Scope, "P")
	inner := sym(t, p.Scope, "x")
	condition := m.OwnedMemberships(p)[1].Implicit.Member
	if got := m.expressionScopeOf(condition); got != p.Scope {
		t.Fatalf("filter condition scope = %v, want P's own scope", got)
	}
	var referents []*symbols.Symbol
	for _, rel := range m.OwnedRelationships(condition) {
		if relMetaName(m, rel) == "Membership" && rel.Implicit != nil && rel.Implicit.Member != nil {
			referents = append(referents, rel.Implicit.Member)
		}
	}
	if len(referents) != 1 || referents[0] != inner {
		t.Errorf("the filter condition refers to %v, want P::x", referents)
	}
}

func TestResultExpressionMembershipReflected(t *testing.T) {
	fixture := reflectiveFixtures(t, "result-expression.sysml", source.KindSysML, `package P {
    calc def K { 1 }
    calc k { in x : ScalarValues::Integer; x + 1 }
    constraint def C { 1 > 0 }
}`)[0]
	for name, want := range map[string]string{
		"K": "ResultExpressionMembership/LiteralInteger",
		"k": "ResultExpressionMembership/OperatorExpression",
		"C": "ResultExpressionMembership/OperatorExpression",
	} {
		owner := relFixtureSymbol(t, fixture, name)
		var results []string
		for _, membership := range synthesizedMembershipNames(fixture.model, owner) {
			if strings.HasPrefix(membership, "ResultExpressionMembership/") {
				results = append(results, membership)
			}
		}
		assertNames(t, name+" result memberships", results, []string{want})
	}
}
