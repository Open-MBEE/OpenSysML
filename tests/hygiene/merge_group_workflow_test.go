package hygiene

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The develop ruleset merges through a merge queue, which builds a commit on
// refs/heads/gh-readonly-queue/develop/... and asks for the required checks on
// it through the merge_group event. These tests hold .github/workflows/pr.yml,
// the workflow that produces those checks, to answering that event: it has to
// trigger on it, run the queue tier over the merge group's own diff, and give
// each queue commit a concurrency group of its own.

type prWorkflow struct {
	On          yaml.Node `yaml:"on"`
	Concurrency struct {
		Group string `yaml:"group"`
	} `yaml:"concurrency"`
	Jobs map[string]actionsJob `yaml:"jobs"`
}

func loadPRWorkflow(t *testing.T) (prWorkflow, string) {
	t.Helper()
	raw, err := os.ReadFile("../../.github/workflows/pr.yml")
	if err != nil {
		t.Fatalf("read .github/workflows/pr.yml: %v", err)
	}
	var workflow prWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse .github/workflows/pr.yml: %v", err)
	}
	return workflow, string(raw)
}

func prWorkflowStep(t *testing.T, workflow prWorkflow, job, name string) actionsStep {
	t.Helper()
	j, ok := workflow.Jobs[job]
	if !ok {
		t.Fatalf("pr.yml has no %s job", job)
	}
	for _, step := range j.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("pr.yml job %s has no step named %q", job, name)
	return actionsStep{}
}

func TestPRWorkflowTriggersOnMergeGroup(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	var triggers map[string]struct {
		Types []string `yaml:"types"`
	}
	if err := workflow.On.Decode(&triggers); err != nil {
		t.Fatalf("decode pr.yml on: %v", err)
	}
	if _, ok := triggers["pull_request"]; !ok {
		t.Fatal("pr.yml no longer triggers on pull_request")
	}
	mergeGroup, ok := triggers["merge_group"]
	if !ok {
		t.Fatal("pr.yml does not trigger on merge_group: the merge queue's commits get no checks and every entry times out")
	}
	if len(mergeGroup.Types) != 1 || mergeGroup.Types[0] != "checks_requested" {
		t.Fatalf("pr.yml merge_group types = %v, want [checks_requested]", mergeGroup.Types)
	}
}

// The group expression picks a form by event: a queue commit or a dispatch has no pull
// request number, and a key that collapsed every queue commit onto one group would cancel the queue.
func TestPRWorkflowConcurrencyGroupSeparatesMergeGroups(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	group := workflow.Concurrency.Group
	conditional := regexp.MustCompile(`^\$\{\{\s*github\.event_name == 'pull_request' && (.+?) \|\| (.+?)\s*\}\}$`)
	m := conditional.FindStringSubmatch(group)
	if m == nil {
		t.Fatalf("pr.yml concurrency.group = %q, want a github.event_name == 'pull_request' && <pull request form> || <ref form> expression", group)
	}
	pullRequestForm, refForm := m[1], m[2]
	if pullRequestForm == refForm {
		t.Fatalf("pr.yml concurrency.group uses %q for every event", refForm)
	}
	if !strings.Contains(refForm, "github.ref") && !strings.Contains(refForm, "merge_group.head_ref") {
		t.Fatalf("pr.yml concurrency.group queue form %q is not keyed on the queue ref", refForm)
	}
	if !strings.Contains(refForm, "github.event_name") {
		t.Fatalf("pr.yml concurrency.group queue form %q does not tell a queue commit's run from a dispatch of the same ref", refForm)
	}
	if !strings.Contains(pullRequestForm, "pull_request.number") {
		t.Fatalf("pr.yml concurrency.group pull-request form %q is not keyed on the pull request number", pullRequestForm)
	}
}

// Changed areas diffs a merge-group commit over its own base_sha..head_sha (it has no base_ref) and
// tells scripts/ci-changed-areas.sh the event, the label and a dispatch's tier, which decide the tier.
func TestPRWorkflowChangedAreasHandlesMergeGroups(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)

	areas := prWorkflowStep(t, workflow, "changes", "Decide which areas to run")
	for env, want := range map[string]string{
		"CI_EVENT":      "${{ github.event_name }}",
		"CI_FULL_LABEL": "'ci:full'",
		"CI_TIER":       "github.event_name == 'workflow_dispatch' && inputs.tier",
	} {
		value, ok := areas.Env[env]
		if !ok {
			t.Fatalf("pr.yml Decide which areas to run has no %s env", env)
		}
		if !strings.Contains(value, want) {
			t.Errorf("pr.yml Decide which areas to run %s = %q does not carry %s to the script", env, value, want)
		}
	}
	tests := prWorkflowStep(t, workflow, "go-tests", "Run Go tests")
	for tier, target := range map[string]string{"full": "make test-shard", "queue": "make test-queue", "*": "make test-quick"} {
		if !strings.Contains(tests.Run, tier+") "+target) {
			t.Errorf("pr.yml Run Go tests = %q does not run %s for the %s tier", tests.Run, target, tier)
		}
	}
	shards := prWorkflowStep(t, workflow, "changes", "Pick the Go shards")
	if !strings.Contains(shards.Env["TIER"], "steps.areas.outputs.tier") {
		t.Errorf("pr.yml Pick the Go shards TIER = %q does not read the tier the script decided", shards.Env["TIER"])
	}

	for env, sha := range map[string]string{"DIFF_BASE": "base_sha", "DIFF_HEAD": "head_sha"} {
		value, ok := areas.Env[env]
		if !ok {
			t.Fatalf("pr.yml Decide which areas to run has no %s env", env)
		}
		if !strings.Contains(value, "github.event_name == 'merge_group' && github.event.merge_group."+sha) {
			t.Errorf("pr.yml Decide which areas to run %s = %q does not use the merge group's %s", env, value, sha)
		}
	}
	if !strings.Contains(areas.Env["DIFF_BASE"], "github.base_ref") {
		t.Errorf("pr.yml Decide which areas to run DIFF_BASE = %q no longer diffs a pull request against its base branch", areas.Env["DIFF_BASE"])
	}
	if !strings.Contains(areas.Run, `"$DIFF_BASE" "$DIFF_HEAD"`) {
		t.Errorf("pr.yml Decide which areas to run = %q does not pass the diff range to scripts/ci-changed-areas.sh", areas.Run)
	}
}

// Every read of the pull-request payload or the base ref has to be guarded by
// the event, or a queue commit or a dispatch evaluates it to nothing.
func TestPRWorkflowPullRequestReadsHaveMergeGroupBranch(t *testing.T) {
	_, raw := loadPRWorkflow(t)
	pullRequestOnly := regexp.MustCompile(`github\.event\.pull_request\.|github\.base_ref`)
	for i, line := range strings.Split(raw, "\n") {
		if !pullRequestOnly.MatchString(line) {
			continue
		}
		if !strings.Contains(line, "github.event_name") || !strings.Contains(line, "merge_group") && !strings.Contains(line, "'pull_request'") {
			t.Errorf("pr.yml:%d reads the pull request payload with no event guard: %s", i+1, strings.TrimSpace(line))
		}
	}
}

// The protobuf baseline is where the pull request forked from its base branch (a merge
// group's base_sha), not the branch's tip, or a field added there since reads as deleted.
func TestPRWorkflowProtoBaselineIsTheMergeBase(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	step := prWorkflowStep(t, workflow, "static-and-integrity", "Check the protobuf schema for breaking changes")
	if !strings.Contains(step.Env["BASE_SHA"], "github.event_name == 'merge_group' && github.event.merge_group.base_sha") {
		t.Errorf("pr.yml protobuf breaking check BASE_SHA = %q does not use the merge group's base_sha", step.Env["BASE_SHA"])
	}
	if !strings.Contains(step.Env["HEAD_SHA"], "github.event.pull_request.head.sha") {
		t.Errorf("pr.yml protobuf breaking check HEAD_SHA = %q does not name the pull request's head", step.Env["HEAD_SHA"])
	}
	if !strings.Contains(step.Run, `git merge-base "origin/$BASE_REF" "$HEAD_SHA"`) {
		t.Errorf("pr.yml protobuf breaking check = %q does not compare against the merge base", step.Run)
	}
	if strings.Contains(step.Run, `BUF_BREAKING_REF="origin/`) {
		t.Errorf("pr.yml protobuf breaking check = %q compares against the base branch's tip", step.Run)
	}
}

func TestPRWorkflowAggregateReportsOnEveryEvent(t *testing.T) {
	workflow, raw := loadPRWorkflow(t)
	aggregate, ok := workflow.Jobs["build-and-test"]
	if !ok {
		t.Fatal("pr.yml has no build-and-test job")
	}
	if len(aggregate.Steps) == 0 {
		t.Fatal("pr.yml build-and-test has no steps")
	}
	// The job-level if and name are not in actionsJob; read them where they are written.
	jobHeader := regexp.MustCompile(`(?m)^  build-and-test:\n    name: Build and test\n    if: always\(\)\n`)
	if !jobHeader.MatchString(raw) {
		t.Fatal("pr.yml build-and-test is not the always-running job named Build and test, the check the develop ruleset requires")
	}
}
