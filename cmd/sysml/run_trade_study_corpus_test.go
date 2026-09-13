package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// The three published trade studies, with the environment variable that turns
// each corpus's absence into a failure, as CI does.
const (
	trainingTradeStudy = "../../examples/sysml-v2-training/33. Analysis/Trade Study Analysis Example.sysml"
	trainingRequireEnv = "OPENSYSML_REQUIRE_TRAINING_CORPUS"
	pilotTradeStudy    = "../../examples/pilot-corpora/sysml-examples/Simple Tests/TradeStudyTest.sysml"
	pilotTradeOff      = "../../examples/pilot-corpora/sysml-validation/10-Analysis and Trades/10b-Trade-off Among Alternative Configurations.sysml"
)

// runCorpusFile runs the binary on a corpus file, skipping when the corpus is
// absent unless requireEnv demands it.
func runCorpusFile(t *testing.T, binary, path, requireEnv string, args ...string) runOutcome {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		if os.Getenv(requireEnv) != "" {
			t.Fatalf("%s is set but the corpus is absent: %v", requireEnv, err)
		}
		t.Skipf("corpus absent (%s); run the download script under scripts/", path)
	}
	cmd := exec.Command(binary, append(args, path)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	got := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		got.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", args, err, got.output())
	}
	return got
}

// TestRunTradeStudyTrainingExample runs the training corpus's engine trade
// study. Its subject restates no multiplicity, so it inherits the library's
// [1..*] and both engines are alternatives; its rollup calcs declare results
// they never bind, so evaluating the first alternative stops at the first of
// them with the missing body named, and the objective is undecided.
func TestRunTradeStudyTrainingExample(t *testing.T) {
	binary := buildCLI(t)
	got := runCorpusFile(t, binary, trainingTradeStudy, trainingRequireEnv,
		"-analysis", "'Trade Study Analysis Example'::engineTradeStudy")
	wantReport(t, got, 2,
		"sysml: analysis run failed: analysis Trade Study Analysis Example::engineTradeStudy",
		"objective tradeStudyObjective: undecided",
		"evaluationFunction(Trade Study Analysis Example::engine4cyl (object #1)): error:",
		"no result expression: calc Trade Study Analysis Example::engineTradeStudy::evaluationFunction::powerRollup has no return expression")
}

// TestRunTradeStudyPilotSimpleTest runs the pilot's TradeStudyTest, whose
// evaluation function is declared without a body: the first alternative's
// evaluation reports the missing body, nothing is selected, and the objective
// is undecided.
func TestRunTradeStudyPilotSimpleTest(t *testing.T) {
	binary := buildCLI(t)
	got := runCorpusFile(t, binary, pilotTradeStudy, pilotRequireEnv,
		"-analysis", "TradeStudyTest::engineTradeStudy")
	wantReport(t, got, 2,
		"sysml: analysis run failed: analysis TradeStudyTest::engineTradeStudy",
		"objective tradeStudyObjective: undecided",
		"evaluationFunction(TradeStudyTest::engine1 (object #1)): error: no result expression: calc TradeStudyTest::engineTradeStudy::evaluationFunction has no return expression")
}

// TestRunTradeStudyPilotTradeOff runs the pilot validation model whose subject
// is `all engineChoice`, the extent of a variation: its two variants, in
// declaration order. The first is evaluated, its evaluation function's rollups
// declare no bodies (as the training example's do), so nothing is selected and
// the objective is undecided.
func TestRunTradeStudyPilotTradeOff(t *testing.T) {
	binary := buildCLI(t)
	got := runCorpusFile(t, binary, pilotTradeOff, pilotRequireEnv,
		"-analysis", "'10b-Trade-off Among Alternative Configurations'::Analysis::engineTradeStudy")
	wantReport(t, got, 2,
		"sysml: analysis run failed: analysis 10b-Trade-off Among Alternative Configurations::Analysis::engineTradeStudy",
		"objective tradeStudyObjective: undecided",
		"evaluationFunction(4cylEngine (Instance ID: 1)): error:",
		"no result expression: calc 10b-Trade-off Among Alternative Configurations::Analysis::engineTradeStudy::evaluationFunction::powerRollup has no return expression")
}
