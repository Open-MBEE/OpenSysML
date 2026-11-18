package view

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A rendering holding no source labels an accept by what it subsets, a feature
// chain with every link and each name quoted on its own.
func TestTriggerLabelNamesAChainedSubsetWithoutSource(t *testing.T) {
	for want, subset := range map[string][]string{
		"accept :> alert":             {"alert"},
		"accept :> fall.touchdown":    {"fall", "touchdown"},
		"accept :> P::fall.'touch d'": {"P::fall", "touch d"},
	} {
		written := func() (lower.Trigger, bool) {
			return lower.Trigger{Kind: "accept", Subsetting: true, Subset: subset}, true
		}
		if got := (&Renderer{}).triggerLabel("", written); got != want {
			t.Errorf("triggerLabel(%v) = %q, want %q", subset, got, want)
		}
	}
}
