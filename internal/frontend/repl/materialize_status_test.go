package repl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// unmaterializableModel binds two values to a feature declaring no multiplicity,
// so reading the feature value finds a default that does not conform to 1..1.
const unmaterializableModel = `package Demo { attribute def X { attribute bad : ScalarValues::Real = (1.0, 2.0); }
               part def R { attribute b : X; } }
`

// conformingModel reads clean, so nothing about it is left unanswered.
const conformingModel = `package Demo { part def R { attribute b : ScalarValues::Real = 1.0; } }
`

func submitted(t *testing.T, src string) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(src); len(res.Diagnostics) > 0 {
		t.Fatalf("model has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A feature value a command could not materialize is a finding about the model, so it
// reaches the session status a non-interactive run exits on — as the typed error
// the runtime reported, not as the rendering the command printed.
func TestFeatureValuesCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	if s.HasErrors() {
		t.Fatalf("a model that analyses clean has errors before any command: %v", s.DiagnosticLines())
	}

	run(t, s, "%instantiate Demo::R")
	wants(t, run(t, s, "%features Demo::R"), "bad: <error:", "multiplicity violation")

	if !s.HasErrors() {
		t.Error("a rendered materialization failure did not reach the session status")
	}
	failures := s.MaterializationFailures()
	if len(failures) != 1 {
		t.Fatalf("materialization failures = %v, want one", failures)
	}
	if !errors.Is(failures[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("failure %v is not the multiplicity violation the runtime reported", failures[0])
	}
}

// The JSON listing reads the same feature values, so a failure it serializes as the
// API's `error` field reaches the status as the text listing's does.
func TestFeatureValuesJSONCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	run(t, s, "%instantiate Demo::R")
	wants(t, run(t, s, "%features Demo::R json"), `"error":`, "multiplicity violation")

	if !s.HasErrors() {
		t.Error("a serialized materialization failure did not reach the session status")
	}
	failures := s.MaterializationFailures()
	if len(failures) != 1 {
		t.Fatalf("materialization failures = %v, want one", failures)
	}
	if !errors.Is(failures[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("failure %v is not the multiplicity violation the runtime reported", failures[0])
	}
}

// %eval reads a feature value too, so a value it could not materialize is the same finding.
func TestEvalCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	wants(t, run(t, s, "%instantiate Demo::R"), "Created instance")

	wants(t, run(t, s, "%eval bad"), "error: evaluation failed", "multiplicity violation")
	if !s.HasErrors() {
		t.Error("a failed evaluation of a feature value did not reach the session status")
	}
	if got := s.MaterializationFailures(); len(got) == 0 ||
		!errors.Is(got[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("materialization failures = %v, want the multiplicity violation", got)
	}
}

// A pinned %eval reads the same feature value through the object it names, so which form
// of the command surfaced the failure does not decide whether it is recorded.
func TestPinnedEvalCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	wants(t, run(t, s, "%instantiate Demo::R"), "Created instance")

	wants(t, run(t, s, "%eval in Demo::R::b : bad"), "error:", "multiplicity violation")
	if !s.HasErrors() {
		t.Error("a pinned evaluation of a feature value that does not materialize did not reach the session status")
	}
	if got := s.MaterializationFailures(); len(got) == 0 ||
		!errors.Is(got[0], runtime.ErrFeatureValueMaterialization) {
		t.Errorf("materialization failures = %v, want the feature value the runtime could not materialize", got)
	}
}

// A name that is no feature value of the object is a request the command got wrong, not a
// feature value that failed to materialize, so it decides nothing about the model.
func TestEvalOfAnUnknownFeatureValueIsNoMaterializationFailure(t *testing.T) {
	s := submitted(t, conformingModel)
	run(t, s, "%instantiate Demo::R")
	run(t, s, "%eval nosuch")

	if got := s.MaterializationFailures(); len(got) != 0 {
		t.Errorf("materialization failures = %v, want none", got)
	}
}

// A model whose feature values all materialize leaves the session with nothing unanswered.
func TestFeatureValuesOfAConformingModelLeaveNoFailure(t *testing.T) {
	s := submitted(t, conformingModel)
	run(t, s, "%instantiate Demo::R")
	wants(t, run(t, s, "%features Demo::R"), "b = 1")

	if s.HasErrors() {
		t.Errorf("a conforming model was reported wrong: %v", s.MaterializationFailures())
	}
	if got := s.MaterializationFailures(); len(got) != 0 {
		t.Errorf("materialization failures = %v, want none", got)
	}
}

// The record is of what the session answered, so a later command about a
// different object does not clear it.
func TestMaterializationFailureStandsAfterALaterCommand(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	run(t, s, "%instantiate Demo::R")
	run(t, s, "%features Demo::R")
	run(t, s, "%list")

	if !s.HasErrors() {
		t.Error("a later command cleared what the session could not answer")
	}
}

// The prompt is unaffected: the rendering is printed and the loop goes on taking
// lines, so a user sees the error and keeps working.
func TestPromptContinuesAfterAMaterializationFailure(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	var out strings.Builder
	if err := Loop(&scriptReader{lines: []string{"%instantiate Demo::R", "%features Demo::R", "%eval 1 + 1"}}, &out, s); err != nil {
		t.Fatalf("Loop error: %v", err)
	}

	got := out.String()
	wants(t, got, "bad: <error:", "multiplicity violation", "= 2")
	if !s.HasErrors() {
		t.Error("the failure the prompt rendered did not reach the session status")
	}
}

// A load reports what analysis found, so a command's materialization failure does
// not turn an earlier load into a failed one.
func TestLoadReportsAnalysisAlone(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	run(t, s, "%instantiate Demo::R")
	run(t, s, "%features Demo::R")

	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(conformingModel), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := s.LoadPathsReport([]string{path})
	if err != nil {
		t.Fatalf("LoadPathsReport: %v", err)
	}
	if report.Errors {
		t.Error("a load of a model that analyses clean was reported as failed")
	}
}
