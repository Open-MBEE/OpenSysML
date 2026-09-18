package corpus

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

var updateTraining = flag.Bool("update-training", false,
	"rewrite the corpus size recorded in testdata/training_examples_expected.txt")

// Byte-identical to the committed file's header, so regenerating a clean corpus
// rewrites it unchanged.
const trainingExpectedHeader = "# Files in the pinned OMG training corpus that still report semantic errors,\n" +
	"# as \"<error count>\\t<path>\". See docs/project/training-examples.md for why each one\n" +
	"# fails; regenerate with:\n" +
	"#   go test ./tests/corpus -run TestTrainingExamplesSemanticErrors -update-training\n"

// The OMG training corpus is an assertion, not a baseline: every file resolves
// without a semantic error, so testdata/training_examples_expected.txt records
// the corpus size and no per-file counts, and an error there cannot be ratcheted
// in (see writeTrainingExpected). The three other roots ratchet instead, in
// pilot_corpora_test.go; docs/project/pilot-corpora.md says why. The corpus is
// not vendored, so this skips when it is absent unless
// OPENSYSML_REQUIRE_TRAINING_CORPUS is set, as CI does.
func TestTrainingExamplesSemanticErrors(t *testing.T) {
	files := trainingGate.files(t)
	root := trainingGate.roots[0].name

	// Measure the implementation rather than the developer's machine: an empty
	// semantic cache makes the run index the standard library by parsing it,
	// which is what a fresh checkout, the LSP on a new machine, and CI all do.
	// TestCorpusGatesCacheStateIndependent pins that the restored-from-cache run
	// agrees with it.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := trainingGate.counts(t, files)

	if *updateTraining {
		writeTrainingExpected(t, files, got)
		return
	}

	totals, want := trainingGate.readExpected(t)
	if len(want) > 0 {
		t.Errorf("%s records %d per-file count(s) (%v); the training corpus is asserted clean, "+
			"not ratcheted, so an error there must be fixed rather than recorded",
			trainingGate.expected, len(want), sortedKeys(want))
	}
	trainingGate.checkRootSizes(t, files, totals, "-update-training")

	for _, path := range sortedKeys(got) {
		t.Errorf("%s: %d semantic error(s); the training corpus must stay clean, see the log above",
			path, got[path])
	}

	t.Logf("%d/%d training files clean", len(files[root])-len(got), len(files[root]))
}

// writeTrainingExpected refreshes the recorded corpus size only: recording a
// per-file count is what would turn this gate into a ratchet, so it refuses.
func writeTrainingExpected(t *testing.T, files map[string][]string, got map[string]int) {
	t.Helper()

	if len(got) > 0 {
		t.Fatalf("%d file(s) report errors (%v); the training gate asserts a clean corpus, "+
			"so -update-training cannot record them", len(got), sortedKeys(got))
	}
	trainingGate.writeExpected(t, trainingExpectedHeader, files, got)
	t.Logf("wrote %s: %d/%d files clean",
		trainingGate.expected, len(files[trainingGate.roots[0].name]), len(files[trainingGate.roots[0].name]))
}

// TestRequirementDefinitionsFile pins one training file that exercises
// requirement definitions end to end, because the corpus gate above only counts
// errors per file.
func TestRequirementDefinitionsFile(t *testing.T) {
	const name = "32. Requirements/Requirement Definitions.sysml"

	content, err := os.ReadFile(filepath.Join(trainingGate.roots[0].dir, name))
	if os.IsNotExist(err) {
		trainingGate.skip(t, name+" is missing")
	}
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	ws := model.NewWorkspace()
	ws.Open(name, content, 1)

	for _, d := range ws.Diagnostics(name) {
		if d.Severity == passes.SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}
