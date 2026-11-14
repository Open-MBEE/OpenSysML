package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The Membership naming a transition's `first` source and its link parameter read the ids the conversion
// mints ahead of the trigger (transitionHeadMembers): `<transition>_psourcemember`, `_linkparam`, `_linkparam_om`.
func TestTransitionHeadMembershipsReadTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    state def S {
        state idle;
        state busy;
        transition go first idle then busy;
    }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	transition := fixture.symbol(t, "S::go")
	owned, _ := fixture.model.ReflectiveElements(transition, "ownedRelationship")
	want := "Observatory__S__go_psourcemember Observatory__S__go_linkparam_om Observatory__S__go_succession_om"
	if got := strings.Join(elementIDs(elementID, owned), " "); got != want {
		t.Fatalf("ownedRelationship ids = %q, want %q", got, want)
	}
	parameters, _ := fixture.model.ReflectiveElements(transition, "ownedFeature")
	if got := strings.Join(elementIDs(elementID, parameters), " "); got != "Observatory__S__go_linkparam Observatory__S__go_succession" {
		t.Fatalf("ownedFeature ids = %q", got)
	}
}
