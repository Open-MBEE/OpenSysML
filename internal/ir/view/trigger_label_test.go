package view

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A rendering holding no source labels an accept by what it subsets, spelled
// as the trigger record holds it.
func TestTriggerLabelNamesAChainedSubsetWithoutSource(t *testing.T) {
	for _, subset := range []string{"alert", "fall.touchdown", "$::P::fall.'touch::d'"} {
		written := func() (lower.Trigger, bool) {
			return lower.Trigger{Kind: "accept", Subsetting: true, Subset: subset}, true
		}
		if got, want := (&Renderer{}).triggerLabel("", written), "accept :> "+subset; got != want {
			t.Errorf("triggerLabel(%s) = %q, want %q", subset, got, want)
		}
	}
}
