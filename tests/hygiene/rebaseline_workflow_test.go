package hygiene

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The ci:rebaseline label re-records a pull request's generated baselines after
// merging develop (.github/workflows/rebaseline.yml). These tests hold the
// workflow to its guards: the label and the same-repository head, a conflict
// allowlist of generated paths that never names the asserted training
// expectation, every recorder the baselines need, and the movement check.

type rebaselineWorkflow struct {
	On          yaml.Node             `yaml:"on"`
	Permissions map[string]string     `yaml:"permissions"`
	Env         map[string]string     `yaml:"env"`
	Jobs        map[string]actionsJob `yaml:"jobs"`
}

const (
	rebaselineWorkflowPath = "../../.github/workflows/rebaseline.yml"
	trainingExpectation    = "tests/corpus/testdata/training_examples_expected.txt"
)

func loadRebaselineWorkflow(t *testing.T) (rebaselineWorkflow, actionsJob) {
	t.Helper()
	raw, err := os.ReadFile(rebaselineWorkflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", rebaselineWorkflowPath, err)
	}
	var workflow rebaselineWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse %s: %v", rebaselineWorkflowPath, err)
	}
	job, ok := workflow.Jobs["rebaseline"]
	if !ok {
		t.Fatalf("%s has no rebaseline job", rebaselineWorkflowPath)
	}
	return workflow, job
}

func rebaselineRuns(job actionsJob) string {
	var runs []string
	for _, step := range job.Steps {
		runs = append(runs, step.Run)
	}
	return strings.Join(runs, "\n")
}

func TestRebaselineRunsOnTheLabelOfARepositoryBranchOnly(t *testing.T) {
	workflow, job := loadRebaselineWorkflow(t)
	var triggers map[string]struct {
		Types []string `yaml:"types"`
	}
	if err := workflow.On.Decode(&triggers); err != nil {
		t.Fatalf("decode on: %v", err)
	}
	// pull_request_target: GitHub creates no pull_request run for a pull request in conflict with develop.
	if len(triggers) != 1 || len(triggers["pull_request_target"].Types) != 1 || triggers["pull_request_target"].Types[0] != "labeled" {
		t.Fatalf("rebaseline.yml triggers on %v, want pull_request_target [labeled] alone", triggers)
	}
	if !strings.Contains(job.If, "github.event.label.name == 'ci:rebaseline'") {
		t.Errorf("the job does not guard on the ci:rebaseline label: %q", job.If)
	}
	if !strings.Contains(job.If, "github.event.pull_request.head.repo.full_name == github.repository") {
		t.Errorf("the job does not refuse a fork's branch: %q", job.If)
	}
	if workflow.Permissions["contents"] != "read" || job.Permissions["contents"] != "write" || job.Permissions["pull-requests"] != "write" {
		t.Errorf("permissions: workflow %v, job %v; want read at the top and contents/pull-requests write on the job", workflow.Permissions, job.Permissions)
	}
}

func TestRebaselineConflictAllowlistIsGeneratedPathsOnly(t *testing.T) {
	workflow, job := loadRebaselineWorkflow(t)
	generated := strings.Fields(workflow.Env["GENERATED_PATHS"])
	if len(generated) == 0 {
		t.Fatal("GENERATED_PATHS is empty")
	}
	for _, path := range generated {
		if path == trainingExpectation {
			t.Errorf("GENERATED_PATHS names %s, which is asserted clean and never re-recorded", path)
		}
		if strings.HasPrefix(path, "tests/corpus/testdata/") && !strings.HasSuffix(path, "_expected.txt") {
			t.Errorf("GENERATED_PATHS names %s, not an expectation", path)
		}
	}
	for _, want := range []string{
		"docs/project/api-json-differential-baseline.json",
		"internal/semantic/metamodel/derived_census_baseline.json",
		"internal/semantic/metamodel/derived_census_records.tsv",
		"internal/translate/export/testdata/structure_differential_expected.txt",
		"docs/project/library-and-prefix-sweep-outcomes.txt",
		"tests/corpus/testdata/corpus_roundtrip_expected.txt",
		"tests/corpus/testdata/api_json_roundtrip_expected.txt",
		"tests/corpus/testdata/pilot_corpora_expected.txt",
		"tests/corpus/testdata/spec_examples_expected.txt",
		"tests/export/testdata/convert",
		"api/proto/sysml.pb.go",
	} {
		found := false
		for _, path := range generated {
			found = found || path == want
		}
		if !found {
			t.Errorf("GENERATED_PATHS does not list %s", want)
		}
	}
	runs := rebaselineRuns(job)
	if strings.Count(runs, "*/testdata/*.sysml|*/testdata/*.kerml|*/testdata/*.ttl) ok=0") != 2 {
		t.Error("the merge and the change check do not refuse to treat a source fixture under a golden directory as generated")
	}
	if !strings.Contains(runs, trainingExpectation) || !strings.Contains(runs, "never re-recorded") {
		t.Error("no step fails the run when the recorders change the training expectation")
	}
	if strings.Contains(runs, "-update-training") {
		t.Error("a step re-records the training expectation")
	}
}

func TestRebaselineRunsEveryRecorderThenChecksTheMovement(t *testing.T) {
	_, job := loadRebaselineWorkflow(t)
	runs := rebaselineRuns(job)
	for _, recorder := range []string{
		"make stdlib-snapshot",
		"make proto-buf proto-ts proto-rust python-proto proto-csharp",
		"grpcio-tools==",
		"go run -C tools ./cmd/pilot-apijson-diff -update",
		"go run -C tools ./cmd/derived-census -update",
		"-update-pilot-corpora", "-update-spec-examples", "-update-pssm-migration",
		"-update-corpus-roundtrip", "-update-api-json-roundtrip",
		"-update-structure-differential", "-update-library-prefix-sweep",
		"go test -count=1 ./tests/export -update",
		"make docs-counts",
		"scripts/rebaseline-movement.py",
		"--remove-label",
		"gh pr comment",
	} {
		if !strings.Contains(runs, recorder) {
			t.Errorf("no step runs %q", recorder)
		}
	}
	corpora := false
	for _, step := range job.Steps {
		corpora = corpora || step.Uses == "./.github/actions/corpora"
		if (strings.Contains(step.Run, "--remove-label") || strings.Contains(step.Run, "gh pr comment")) && !strings.Contains(step.If, "always()") {
			t.Errorf("step %q must run whatever the outcome, got if: %q", step.Name, step.If)
		}
	}
	if !corpora {
		t.Error("the corpora are not provisioned through .github/actions/corpora")
	}
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "rebaseline-movement.py") {
			for _, later := range job.Steps[i+1:] {
				if strings.Contains(later.Run, "-update") || strings.Contains(later.Run, "make docs-counts") {
					t.Errorf("recorder step %q runs after the movement check", later.Name)
				}
			}
			for _, earlier := range job.Steps[:i] {
				if strings.Contains(earlier.Run, "git push") {
					t.Errorf("step %q pushes before the movement check", earlier.Name)
				}
			}
		}
	}
}
