package model

import (
	"fmt"
	"testing"
)

// diagnosticsColdAndWarm analyses src twice against the same library cache —
// run 1 parses the stdlib, run 2 restores it — and returns both message lists.
func diagnosticsColdAndWarm(t *testing.T, name, src string) (cold, warm []string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for run := 1; run <= 2; run++ {
		ws := NewWorkspace()
		ws.Open(name, []byte(src), 1)
		var msgs []string
		for _, d := range ws.Diagnostics(name) {
			// An end-less connection definition relates fewer than two elements,
			// which the reference reports too; cache state is what is under test.
			if d.Message == "Must have at least two related elements" {
				continue
			}
			msgs = append(msgs, d.Message)
		}
		if run == 1 {
			cold = msgs
		} else {
			warm = msgs
		}
	}
	return cold, warm
}

// A redefinition whose target lives beyond the first cached library parent —
// on a grandparent the walk only reaches by following the restored symbol's
// own specialization edges — resolves the same on a cold cache and a warm one.
// One case per chain the deleted hardcoded parent map used to fake. A flow
// definition specializes Flows::Message only once it has two ends; with none it
// is a Flows::MessageAction, which has no sourceEvent or payloadNum.
func TestW5GRedefinitionThroughCachedLibraryChainIsCacheStateIndependent(t *testing.T) {
	cases := []struct{ name, member string }{
		{"Parts::Part to Items::Item", "part def P { item :>> shape; }"},
		{"Items::Item to Occurrences::Occurrence", "item def I { ref :>> localClock; }"},
		{"Flows::flows to Flows::Message", "flow f { in ref :>> sourceEvent; }"},
		{"Flows::Message to Transfers::Transfer", "flow def F2 { end a; end b; ref :>> payloadNum; }"},
		{"Connections::Connection to Links::Link", "connection def C { ref :>> participant; }"},
		{"Links::Link to Occurrences::Occurrence", "connection def C2 { ref :>> suboccurrences; }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf("package W5G {\n\t%s\n}\n", tc.member)
			cold, warm := diagnosticsColdAndWarm(t, "file:///w5g.sysml", src)
			if len(cold) != 0 {
				t.Fatalf("cold cache reports %v, want clean", cold)
			}
			if len(warm) != 0 {
				t.Fatalf("warm cache reports %v, cold was clean", warm)
			}
		})
	}
}

// A parameter of a step typed by a cached library behavior implicitly redefines
// that behavior's parameter at its position, so a redefinition inside it can
// name the parameter's inherited features — whether the library was parsed or
// restored. This is the 8-line reproducer of the wave-5G cold/warm divergence.
func TestW5GBehaviorParameterRedefinitionIsCacheStateIndependent(t *testing.T) {
	src := `package R {
	behavior b {
		step u : FeatureReferencingPerformances::FeatureWritePerformance {
			in onOccurrence {
				feature redefines startingAt;
			}
		}
	}
}
`
	cold, warm := diagnosticsColdAndWarm(t, "file:///w5g.kerml", src)
	if len(cold) != 0 {
		t.Fatalf("cold cache reports %v, want clean", cold)
	}
	if len(warm) != 0 {
		t.Fatalf("warm cache reports %v, cold was clean", warm)
	}
}
