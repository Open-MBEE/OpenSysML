package diff

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
)

// The note must describe both implementations, including the case the overlay
// exists to surface: a correction that changes the pilot's verdict too.
func TestFindingNoteDescribesBothImplementations(t *testing.T) {
	entry := errata.Entry{ID: "F82", Path: "corpus/a.sysml", Line: 3}
	at := func(count int) []diagnostic {
		out := make([]diagnostic, count)
		for i := range out {
			out[i] = diagnostic{File: "a.sysml", Line: entry.Line}
		}
		return out
	}

	cases := []struct {
		name                                             string
		oursPub, oursCorrected, pilotPub, pilotCorrected int
		wantNote                                         string
		wantVerdictChanged                               bool
	}{
		{"ours survives", 1, 1, 0, 0, "our diagnostic survives the correction", false},
		{"pilot still reports", 1, 0, 1, 1, "our diagnostic is cleared while the pilot still reports here: a finding, not a fix", false},
		{"both cleared", 1, 0, 1, 0, "both diagnostics are cleared: the correction changes the pilot's verdict too", true},
		{"neither reported", 0, 0, 0, 0, "neither implementation reported here as published", false},
		{"pilot silent throughout", 1, 0, 0, 0, "our diagnostic is cleared and the pilot is silent on both texts", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel := entry.Path
			got := finding(entry, "root", rel,
				map[string][]diagnostic{rel: at(tc.oursPub)},
				map[string][]diagnostic{rel: at(tc.pilotPub)},
				map[string][]diagnostic{rel: at(tc.oursCorrected)},
				map[string][]diagnostic{rel: at(tc.pilotCorrected)})
			if got.Note != tc.wantNote {
				t.Errorf("note = %q, want %q", got.Note, tc.wantNote)
			}
			if got.PilotVerdictNew != tc.wantVerdictChanged {
				t.Errorf("pilotVerdictChanged = %v, want %v", got.PilotVerdictNew, tc.wantVerdictChanged)
			}
		})
	}
}
