package model

import (
	"flag"
	"testing"
)

var updatePilotCorpora = flag.Bool("update-pilot-corpora", false,
	"rewrite testdata/pilot_corpora_expected.txt from the current results")

// Byte-identical to the committed file's header, so regenerating without a
// movement rewrites it unchanged.
const pilotCorporaExpectedHeader = "# Files in the pinned OMG pilot corpora that report diagnostics of any\n" +
	"# severity, as \"<diagnostic count>\\t<root>/<path>\". These are our verdicts\n" +
	"# alone, not a comparison against the reference implementation; see\n" +
	"# docs/project/pilot-corpora.md. Regenerate with:\n" +
	"#   go test ./internal/core/model -run TestPilotCorporaDiagnostics -update-pilot-corpora\n"

// The three pinned OMG pilot corpora are a per-file ratchet on our own verdicts:
// every file's diagnostic count of every severity is recorded in
// testdata/pilot_corpora_expected.txt, so a file that starts reporting, stops
// reporting, changes its number of diagnostics, or appears or disappears fails
// this test. These roots are not clean under our implementation, which is why
// they ratchet where the training corpus asserts (docs/project/pilot-corpora.md).
// It says nothing about whether those diagnostics are right — that comparison is
// tools/referee/diff, which needs the pinned Java validators and stays out of CI.
// The corpora are not vendored, so this skips when they are absent unless
// OPENSYSML_REQUIRE_PILOT_CORPORA is set, as CI does.
func TestPilotCorporaDiagnostics(t *testing.T) {
	files := pilotCorporaGate.files(t)

	// Measure the implementation rather than the developer's machine: an empty
	// semantic cache makes the run index the standard library by parsing it,
	// which is what a fresh checkout, the LSP on a new machine, and CI all do.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := pilotCorporaGate.counts(t, files)

	if *updatePilotCorpora {
		pilotCorporaGate.writeExpected(t, pilotCorporaExpectedHeader, files, got)
		t.Logf("wrote %s: %d file(s) reporting diagnostics", pilotCorporaGate.expected, len(got))
		return
	}

	totals, want := pilotCorporaGate.readExpected(t)
	pilotCorporaGate.checkRootSizes(t, files, totals, "-update-pilot-corpora")

	for _, path := range sortedKeys(want) {
		switch gotCount, ok := got[path]; {
		case !ok:
			t.Errorf("%s: expected %d diagnostic(s) but the file is now clean; "+
				"regenerate with -update-pilot-corpora", path, want[path])
		case gotCount != want[path]:
			direction := "more"
			if gotCount < want[path] {
				direction = "fewer"
			}
			t.Errorf("%s: %d diagnostic(s), expected %d (%s than recorded); "+
				"adjudicate the change, then regenerate with -update-pilot-corpora",
				path, gotCount, want[path], direction)
		}
	}
	for _, path := range sortedKeys(got) {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %d new diagnostic(s), previously clean; "+
				"adjudicate the change, then regenerate with -update-pilot-corpora",
				path, got[path])
		}
	}

	for _, root := range pilotCorporaGate.roots {
		reporting := 0
		for _, rel := range files[root.name] {
			if got[pilotCorporaGate.key(root, rel)] > 0 {
				reporting++
			}
		}
		t.Logf("%s: %d/%d pilot corpus files clean",
			root.name, len(files[root.name])-reporting, len(files[root.name]))
	}
}
