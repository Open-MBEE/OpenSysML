package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests hold the ci:rebaseline workflow to its guards: the label, the same-repository head, the
// generated-path allowlist, every recorder, the movement check, and a re-record over each merge it pushes.

type rebaselineWorkflow struct {
	On          yaml.Node             `yaml:"on"`
	Permissions map[string]string     `yaml:"permissions"`
	Env         map[string]string     `yaml:"env"`
	Jobs        map[string]actionsJob `yaml:"jobs"`
}

const (
	rebaselineWorkflowPath = "../../.github/workflows/rebaseline.yml"
	rebaselineRecordAction = "./.github/actions/rebaseline-record"
	rebaselineMergeScript  = "rebaseline-merge.sh"
	// rebaselineMergeRun is how a step runs develop's copy of the merge script on a head.
	rebaselineMergeRun  = `"$RUNNER_TEMP/` + rebaselineMergeScript + `" "$`
	trainingExpectation = "tests/corpus/testdata/training_examples_expected.txt"
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

// loadRebaselineRecord is the steps of the action that records one merge.
func loadRebaselineRecord(t *testing.T) []actionsStep {
	t.Helper()
	path := filepath.Join("../..", rebaselineRecordAction, "action.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var action struct {
		Runs struct {
			Steps []actionsStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(raw, &action); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return action.Runs.Steps
}

// rebaselineRuns is every script the job runs: its own steps, the record action's and the merge script.
func rebaselineRuns(t *testing.T, job actionsJob) string {
	t.Helper()
	merge, err := os.ReadFile(filepath.Join("../../scripts", rebaselineMergeScript))
	if err != nil {
		t.Fatal(err)
	}
	runs := []string{string(merge)}
	for _, step := range slices.Concat(job.Steps, loadRebaselineRecord(t)) {
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
	if workflow.Permissions["contents"] != "read" || job.Permissions["contents"] != "write" || job.Permissions["pull-requests"] != "write" || job.Permissions["statuses"] != "write" {
		t.Errorf("permissions: workflow %v, job %v; want read at the top and contents/pull-requests/statuses write on the job", workflow.Permissions, job.Permissions)
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
	runs := rebaselineRuns(t, job)
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
	runs := rebaselineRuns(t, job)
	for _, recorder := range []string{
		"make stdlib-snapshot",
		"make proto-buf proto-ts proto-rust python-proto",
		"go run -C tools ./cmd/pilot-apijson-diff -update -pilot-record build/pilot-apijson-record",
		"go run -C tools ./cmd/derived-census -update -recorded-pilot ||",
		"go run -C tools ./cmd/derived-census -update\n",
		"-update-pilot-corpora", "-update-spec-examples", "-update-pssm-migration",
		"-update-corpus-roundtrip", "-update-api-json-roundtrip",
		"-update-structure-differential", "-update-library-prefix-sweep",
		"go test -count=1 ./tests/export -run ",
		"make docs-counts",
		"scripts/rebaseline-movement.py",
		"--remove-label",
		"gh pr comment",
		"/statuses/",
	} {
		if !strings.Contains(runs, recorder) {
			t.Errorf("no step runs %q", recorder)
		}
	}
	corpora, stubs := false, false
	for _, step := range job.Steps {
		corpora = corpora || step.Uses == "./.github/actions/corpora"
		stubs = stubs || step.Uses == pipInstallAction && strings.Contains(step.With["packages"], "grpcio-tools==")
		if (strings.Contains(step.Run, "--remove-label") || strings.Contains(step.Run, "gh pr comment")) && !strings.Contains(step.If, "always()") {
			t.Errorf("step %q must run whatever the outcome, got if: %q", step.Name, step.If)
		}
	}
	if !corpora {
		t.Error("the corpora are not provisioned through .github/actions/corpora")
	}
	if !stubs {
		t.Errorf("no step installs a pinned grpcio-tools through %s", pipInstallAction)
	}
	record := loadRebaselineRecord(t)
	for i, step := range record {
		if strings.Contains(step.Run, "rebaseline-movement.py") {
			for _, later := range record[i+1:] {
				if strings.Contains(later.Run, "-update") || strings.Contains(later.Run, "make docs-counts") {
					t.Errorf("recorder step %q runs after the movement check", later.Name)
				}
			}
			for _, earlier := range record[:i] {
				if strings.Contains(earlier.Run, "git commit") {
					t.Errorf("step %q commits before the movement check", earlier.Name)
				}
			}
		}
	}
}

// Each push is of one merge the recorders ran over, and the second only when the branch or develop moved under the first.
func TestRebaselinePushesWhatWasRecordedOverEachMerge(t *testing.T) {
	_, job := loadRebaselineWorkflow(t)
	var order []string
	for _, step := range job.Steps {
		kind := ""
		switch {
		case strings.Contains(step.Run, rebaselineMergeRun):
			kind = "merge"
		case step.Uses == rebaselineRecordAction:
			kind = "record"
		case strings.Contains(step.Run, "git push"):
			kind = "push"
		default:
			continue
		}
		if len(order) >= 3 && step.If != "steps.push.outputs.restart" {
			t.Errorf("step %q runs whether or not the first push was held back: if: %q", step.Name, step.If)
		}
		order = append(order, kind)
	}
	if want := []string{"merge", "record", "push", "merge", "record", "push"}; !slices.Equal(order, want) {
		t.Errorf("the job runs %v, want %v", order, want)
	}
	for _, step := range job.Steps {
		if step.ID == "push" && !strings.Contains(step.Run, `"$(git rev-parse FETCH_HEAD)" = "$HEAD_SHA"`) {
			t.Error("the first push does not tell a moved branch from any other refusal")
		}
		if step.ID == "remerge" && !strings.Contains(step.Run, `git reset -q --hard "$RESTART"`) {
			t.Error("the second merge does not start again from the branch's head")
		}
		if step.ID == "push" && !strings.Contains(step.Run, "git merge-tree --write-tree HEAD origin/develop") {
			t.Error("the first push does not look for a conflict with the develop of the moment")
		}
	}
}

// tests/export is re-recorded by name, so the names are held to the tests that record: the callers of checkGolden.
func TestRebaselineNamesEveryExportRecorder(t *testing.T) {
	_, job := loadRebaselineWorkflow(t)
	named := regexp.MustCompile(`go test -count=1 \./tests/export -run '\^\(([A-Za-z|]+)\)\$' -update\n`).FindStringSubmatch(rebaselineRuns(t, job))
	if named == nil {
		t.Fatal("no step re-records tests/export through an anchored -run alternation")
	}
	got := strings.Split(named[1], "|")
	slices.Sort(got)

	files, err := filepath.Glob("../export/*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("tests/export test files: %v, %v", files, err)
	}
	var want []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			records := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if callee, ok := call.Fun.(*ast.Ident); ok && callee.Name == "checkGolden" {
						records = true
					}
				}
				return !records
			})
			if !records {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "Test") {
				t.Errorf("%s: %s calls checkGolden but is no test, so the tests reaching it cannot be named", file, fn.Name.Name)
				continue
			}
			want = append(want, fn.Name.Name)
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "*update") && filepath.Base(file) != "golden_test.go" {
			t.Errorf("%s reads -update outside checkGolden, so a recorder may go unnamed", file)
		}
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("rebaseline re-records tests/export through %v, but the tests that record are %v", got, want)
	}
}
