package semantics

import (
	"slices"
	"strings"
	"testing"

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

func TestFlowEndMetaclass(t *testing.T) {
	fixture := reflectiveFixtures(t, "flow-ends.sysml", source.KindSysML, `package P {
    part def PX { attribute x; }
    part p1 : PX;
    part p2 : PX;
    flow f from p1.x to p2.x;
}`)[0]
	f := relFixtureSymbol(t, fixture, "f")
	assertNames(t, "flow memberships", synthesizedMembershipNames(fixture.model, f), []string{
		"EndFeatureMembership/FlowEnd",
		"EndFeatureMembership/FlowEnd",
	})
	for i, membership := range fixture.model.OwnedMemberships(f) {
		end := membership.Implicit.Member
		if owner := fixture.model.ownerOf(end); owner != f {
			t.Errorf("end %d owner = %v, want f", i, owner)
		}
		if end.Expr.Slot != flowEndSlots[i] {
			t.Errorf("end %d slot = %q, want %q", i, end.Expr.Slot, flowEndSlots[i])
		}
	}
	kerml := reflectiveFixtures(t, "flow-ends.kerml", source.KindKerML, `package P {
    feature fa { feature x; }
    feature fb { feature y; }
    succession flow f from fa.x to fb.y;
}`)[0]
	assertNames(t, "kerml flow memberships", synthesizedMembershipNames(kerml.model, relFixtureSymbol(t, kerml, "f")), []string{
		"EndFeatureMembership/FlowEnd",
		"EndFeatureMembership/FlowEnd",
	})
}
