package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestFirstMarkerMembershipHasItsOwnElementID: the Membership `first start;` states is identified
// on the query leg as the conversion mints it, so a reflective query reads one id for it.
func TestFirstMarkerMembershipHasItsOwnElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	action observe {
		first start;
		then action acquire;
		then done;
	}
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	observe := fixture.symbol(t, "observe")
	memberships, _ := fixture.model.ReflectiveElements(observe, "ownedMembership")
	if len(memberships) == 0 {
		t.Fatal("observe owns no membership")
	}
	first := memberships[0]
	if meta := fixture.model.MetaclassOf(first); meta == nil || meta.Name != "Membership" {
		t.Fatalf("the first membership is %v, want Membership", meta)
	}
	// The conversion mints the marker's Membership positionally among the body's anonymous members.
	if id, err := ElementIDOf(read, fixture.index, first); err != nil || id != "Observatory__observe___400" {
		t.Fatalf("first marker id %q, %v; want Observatory__observe___400", id, err)
	}
}
