package apijson

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
)

// The bridge keys Parts::Part's documentation as its first Documentation and
// first OwningMembership; our ids for the same unnamed member join those keys.
func TestAnonymousLibraryKeysJoinOurIdsToTheBridgesPositionalKeys(t *testing.T) {
	pilot := idKeys{
		"pilot-doc":        "Parts::Part/Documentation[0]",
		"pilot-membership": "Parts::Part/OwningMembership[0]",
		"pilot-part":       "Parts::Part",
	}
	keys := anonymousLibraryKeys(pilot)
	part := identity.ElementID(identity.SysML, "Parts::Part")
	if got := keys[identity.AnonymousMemberID(part, "Documentation", 0)]; got != "Parts::Part/Documentation[0]" {
		t.Errorf("documentation keyed %q, want Parts::Part/Documentation[0]", got)
	}
	if got := keys[identity.AnonymousMemberOwningMembershipID(part, "Documentation", 0)]; got != "Parts::Part/OwningMembership[0]" {
		t.Errorf("documentation's membership keyed %q, want Parts::Part/OwningMembership[0]", got)
	}
	if anonymousLibraryKeys(idKeys{"x": "Parts::Part"}) != nil {
		t.Error("named keys alone produce no anonymous keys")
	}
}
