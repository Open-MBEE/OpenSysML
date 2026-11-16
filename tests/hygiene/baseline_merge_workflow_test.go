package hygiene

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A pull request re-measures our side against the recorded pilot side, the count baselines merge
// cell by cell, and the nightly re-exports the pilot; these tests hold the workflows to that division.

const (
	censusOursOnly  = "go run -C tools ./cmd/derived-census -check -recorded-pilot"
	censusWithPilot = "go run -C tools ./cmd/derived-census -check -require-exporter"
	pilotRecordPath = "build/pilot-apijson-record"
	hideoCheck      = "go run -C tools ./cmd/hideo-suite -check"
)

func TestPRWorkflowChecksTheDerivedCensusAgainstTheRecordedPilotColumn(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	job, ok := workflow.Jobs["derived-census"]
	if !ok {
		t.Fatal("pr.yml has no derived-census job: nothing reproduces the census records before a change merges")
	}
	if !strings.Contains(job.If, "needs.changes.outputs.go == 'true'") {
		t.Errorf("pr.yml derived-census.if = %q, want it gated on the go area", job.If)
	}
	checks, corpora := false, false
	for _, step := range job.Steps {
		checks = checks || strings.TrimSpace(step.Run) == censusOursOnly
		corpora = corpora || step.Uses == "./.github/actions/corpora"
	}
	if !checks || !corpora {
		t.Errorf("pr.yml derived-census: runs %q = %v, provisions the corpora = %v", censusOursOnly, checks, corpora)
	}
	requiredByTheGate(t, workflow, "derived-census", "DERIVED_CENSUS_RESULT")
}

// The suite's outcomes were reproduced nightly only, so develop drifted from its baseline unseen.
func TestPRWorkflowChecksTheCrossMDKSuite(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	job, ok := workflow.Jobs["hideo-suite"]
	if !ok {
		t.Fatal("pr.yml has no hideo-suite job: nothing reproduces the suite's baseline before a change merges")
	}
	for _, area := range []string{"go", "python"} {
		if !strings.Contains(job.If, "needs.changes.outputs."+area+" == 'true'") {
			t.Errorf("pr.yml hideo-suite.if = %q, want it to run on the %s area", job.If, area)
		}
	}
	downloads, checks := false, false
	for _, step := range job.Steps {
		downloads = downloads || strings.TrimSpace(step.Run) == "./scripts/download-hideo-suite.sh"
		if strings.TrimSpace(step.Run) == hideoCheck {
			checks = step.Env["OPENSYSML_REQUIRE_HIDEO_SUITE"] == "1"
		}
	}
	if !downloads || !checks {
		t.Errorf("pr.yml hideo-suite: downloads the suite = %v, runs %q requiring it = %v", downloads, hideoCheck, checks)
	}
	requiredByTheGate(t, workflow, "hideo-suite", "HIDEO_SUITE_RESULT")
}

// requiredByTheGate fails unless build-and-test, the one required check, waits for job and refuses its failure.
func requiredByTheGate(t *testing.T, workflow prWorkflow, job, result string) {
	t.Helper()
	gate := workflow.Jobs["build-and-test"]
	needed := false
	var needs []string
	if err := gate.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	for _, need := range needs {
		needed = needed || need == job
	}
	required := false
	for _, step := range gate.Steps {
		required = required || strings.Contains(step.Run, `! ok "$`+result+`"`)
	}
	if !needed || !required {
		t.Errorf("pr.yml build-and-test: needs %s = %v, requires its result = %v", job, needed, required)
	}
}

func TestPRWorkflowReadsTheRecordedPilotExport(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	restored := false
	for _, step := range workflow.Jobs["apijson-differential"].Steps {
		restored = restored || strings.HasPrefix(step.Uses, "actions/cache/restore@") && step.With["path"] == pilotRecordPath
	}
	if !restored {
		t.Errorf("pr.yml apijson-differential restores no %s, so every run exports the pilot again", pilotRecordPath)
	}
}

// The nightly is where the pilot's side is produced: it re-exports, records and saves the export,
// and holds the census records to a fresh pilot column.
func TestNightlyReexportsThePilotSideAndRecordsIt(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/oracle-reproduction.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]actionsJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	fresh, saved, census := false, false, false
	for _, step := range workflow.Jobs["reproduce"].Steps {
		fresh = fresh || strings.Contains(step.Run, "rm -rf "+pilotRecordPath) && strings.Contains(step.Run, apijsonCheck)
		saved = saved || strings.HasPrefix(step.Uses, "actions/cache/save@") && step.With["path"] == pilotRecordPath
		census = census || strings.TrimSpace(step.Run) == censusWithPilot
	}
	if !fresh || !saved || !census {
		t.Errorf("oracle-reproduction.yml: re-exports into an empty record = %v, saves it = %v, runs %q = %v", fresh, saved, censusWithPilot, census)
	}
}

// The rebaseline merge uses develop's driver, and a merged census is kept only if it reproduces.
func TestRebaselineMergesCellWiseAndAcceptsOnlyWhatReproduces(t *testing.T) {
	_, job := loadRebaselineWorkflow(t)
	driver, merge := -1, -1
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "git config merge.opensysml-cells.driver") && strings.Contains(step.Run, "./cmd/baseline-merge") {
			driver = i
		}
		if strings.Contains(step.Run, "git merge --no-ff --no-commit origin/develop") {
			merge = i
		}
	}
	if driver < 0 || merge < 0 || driver > merge {
		t.Fatalf("the merge driver is configured at step %d and the merge runs at step %d; want the driver first", driver, merge)
	}
	runs := rebaselineRuns(job)
	for _, want := range []string{
		censusOursOnly + " ||",
		apijsonCheck + " ||",
		"grep 'merge=opensysml-cells'",
	} {
		if !strings.Contains(runs, want) {
			t.Errorf("no step runs %q", want)
		}
	}
}

// Every path the driver merges is a generated path the rebaseline workflow may re-record.
func TestCellMergedPathsAreGeneratedPaths(t *testing.T) {
	attributes, err := os.ReadFile("../../.gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	workflow, _ := loadRebaselineWorkflow(t)
	generated := strings.Fields(workflow.Env["GENERATED_PATHS"])
	merged := 0
	for _, line := range strings.Split(string(attributes), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "merge=opensysml-cells" {
			continue
		}
		merged++
		if fields[0] == "docs/project/pilot-differential-baseline.json" {
			continue // recorded by pilot-diff -update, which needs the validators the workflow does not provision
		}
		claimed := false
		for _, path := range generated {
			claimed = claimed || fields[0] == path || strings.HasPrefix(fields[0], path+"/")
		}
		if !claimed {
			t.Errorf(".gitattributes merges %s cell by cell, which GENERATED_PATHS does not list", fields[0])
		}
	}
	if merged == 0 {
		t.Error(".gitattributes names no merge=opensysml-cells path")
	}
}
