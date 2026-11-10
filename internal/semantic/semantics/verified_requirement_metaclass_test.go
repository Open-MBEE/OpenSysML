package semantics

import (
	"testing"
)

// An objective's `verify r;` is the RequirementUsage its RequirementVerificationMembership owns
// (SysML 8.3.19), as the RDF writer and the pilot state it — not a SatisfyRequirementUsage.
func TestVerifiedRequirementMemberIsARequirementUsage(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		requirement massRequirement;
		verification def T { objective { verify massRequirement; } }
	}`)
	def := nestedSym(t, root, "P::T")
	for _, obj := range def.Scope.AllMembers() {
		if obj.Name != "" || !obj.IsFeature() || obj.Scope == nil {
			continue
		}
		for _, membership := range m.OwnedMemberships(obj) {
			if membership.Implicit == nil || !isVerifiedRequirementMember(membership.Implicit.Member) {
				continue
			}
			member := membership.Implicit.Member
			if meta := m.MetaclassOf(member); meta == nil || meta.Name != "RequirementUsage" {
				t.Fatalf("verify member's metaclass = %v, want RequirementUsage", meta)
			}
			if got := m.MetaclassOf(membership); got == nil || got.Name != "RequirementVerificationMembership" {
				t.Fatalf("verify membership's metaclass = %v, want RequirementVerificationMembership", got)
			}
			return
		}
	}
	t.Fatal("no verify member under T's objective")
}
