package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A library usage's written range is an unnamed MultiplicityRange it owns (SysML v2
// 7.6.3); the identity derivation names it positionally like any declared unnamed member.
func TestImpliedMultiplicityRangeIsAnAnonymousMember(t *testing.T) {
	base := libs.SharedBase()
	resolver := resolve.New(base)
	model := semantics.NewModel(resolver)
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	owner := resolver.Index().LookupQualified("Parts::parts")[0]
	rangeMember := model.DeclaredMultiplicityMember(owner)
	if rangeMember == nil {
		t.Fatal("Parts::parts declares [0..*] but has no MultiplicityRange member")
	}
	member, ok := identity.AnonymousMemberOf(model, catalog, rangeMember)
	if !ok {
		t.Fatal("AnonymousMemberOf refuses Parts::parts's MultiplicityRange")
	}
	if member.Path != "Parts::parts/MultiplicityRange[0]" || member.MembershipMetaclass != "OwningMembership" {
		t.Errorf("paths %s / %s (%s), want Parts::parts/MultiplicityRange[0] through an OwningMembership", member.Path, member.MembershipPath, member.MembershipMetaclass)
	}
	indexed, ok := identity.AnonymousMemberIndex(model, catalog).Member(rangeMember)
	if !ok || indexed != member {
		t.Errorf("the index answers %+v, %v for the range; want the same member", indexed, ok)
	}
}
