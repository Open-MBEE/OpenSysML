package corpus

import (
	"flag"
	"testing"
)

var updateSpecExamples = flag.Bool("update-spec-examples", false,
	"rewrite testdata/spec_examples_expected.txt from the current results")

// Byte-identical to the committed file's header, so regenerating without a
// movement rewrites it unchanged.
const specExamplesExpectedHeader = "# Files extracted from the pinned OMG specification examples that report\n" +
	"# diagnostics of any severity, as \"<diagnostic count>\\t<root>/<path>\". These\n" +
	"# are our verdicts alone, not a comparison against the reference\n" +
	"# implementation; see docs/project/spec-examples.md. Regenerate with:\n" +
	"#   go test ./tests/corpus -run TestSpecExamplesDiagnostics -update-spec-examples\n"

// The textual-notation examples of the SysML and KerML specifications,
// extracted by scripts/download-spec-examples.sh, are a per-file ratchet on our
// own verdicts in the shape of the pilot corpora (TestPilotCorporaDiagnostics):
// every file's diagnostic count of every severity is recorded in
// testdata/spec_examples_expected.txt, so a file that starts reporting, stops
// reporting, changes its number of diagnostics, or appears or disappears fails
// this test. Many examples are fragments the specification completes in prose,
// so the roots are not clean; docs/project/spec-examples.md adjudicates every
// reporting file against the text and the pilot. The examples are not
// vendored, so this skips when they are absent unless
// OPENSYSML_REQUIRE_SPEC_EXAMPLES is set, as CI does.
func TestSpecExamplesDiagnostics(t *testing.T) {
	files := specExamplesGate.files(t)

	// Measure the implementation rather than the developer's machine: an empty
	// semantic cache makes the run index the standard library by parsing it,
	// which is what a fresh checkout, the LSP on a new machine, and CI all do.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := specExamplesGate.counts(t, files)

	if *updateSpecExamples {
		specExamplesGate.writeExpected(t, specExamplesExpectedHeader, files, got)
		t.Logf("wrote %s: %d file(s) reporting diagnostics", specExamplesGate.expected, len(got))
		return
	}

	totals, want := specExamplesGate.readExpected(t)
	specExamplesGate.checkRootSizes(t, files, totals, "-update-spec-examples")

	for _, path := range sortedKeys(want) {
		switch gotCount, ok := got[path]; {
		case !ok:
			t.Errorf("%s: expected %d diagnostic(s) but the file is now clean; "+
				"regenerate with -update-spec-examples", path, want[path])
		case gotCount != want[path]:
			direction := "more"
			if gotCount < want[path] {
				direction = "fewer"
			}
			t.Errorf("%s: %d diagnostic(s), expected %d (%s than recorded); "+
				"adjudicate the change, then regenerate with -update-spec-examples",
				path, gotCount, want[path], direction)
		}
	}
	for _, path := range sortedKeys(got) {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %d new diagnostic(s), previously clean; "+
				"adjudicate the change, then regenerate with -update-spec-examples",
				path, got[path])
		}
	}

	for _, root := range specExamplesGate.roots {
		reporting := 0
		for _, rel := range files[root.name] {
			if got[specExamplesGate.key(root, rel)] > 0 {
				reporting++
			}
		}
		t.Logf("%s: %d/%d specification examples clean",
			root.name, len(files[root.name])-reporting, len(files[root.name]))
	}
}
