package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A feature subsetting a chain through itself (`ref p : Person :> p.asPresident`,
// the pilot's JohnIndividualExample) generalizes to the chain's final feature,
// read through its typing, without resolving its own generals again.
func TestSubsettingAChainThroughItselfGeneralizesToItsFinalFeature(t *testing.T) {
	root := parsedRoot(t, "a.sysml", `package P {
		item def Person { ref asPresident[0..1] : Person; }
		item def Country { ref presidentOfCountry[0..1] : Person :> presidentOfCountry.asPresident; }
	}`)
	idx := symbols.NewIndexFromDoc("a.sysml", root)
	r := New(idx)
	r.ResolveDocument("a.sysml", root)
	syms := idx.LookupQualified("P::Country::presidentOfCountry")
	if len(syms) != 1 {
		t.Fatalf("P::Country::presidentOfCountry: %d symbols", len(syms))
	}
	names := map[string]bool{}
	for _, general := range r.generalsOf(syms[0]) {
		names[general.Name] = true
	}
	if !names["Person"] || !names["asPresident"] {
		t.Fatalf("generals of presidentOfCountry = %v, want Person and asPresident", names)
	}
}
