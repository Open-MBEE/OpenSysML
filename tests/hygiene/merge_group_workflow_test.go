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
// trigger on it, run the full tier over the merge group's own diff, and give
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

// The group expression picks one of two forms by event, and the forms must
// name different things: a queue commit has no pull request number, and a key
// that collapsed every queue commit onto one group would cancel the queue.
func TestPRWorkflowConcurrencyGroupSeparatesMergeGroups(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	group := workflow.Concurrency.Group
	conditional := regexp.MustCompile(`^\$\{\{\s*github\.event_name == 'merge_group' && (.+?) \|\| (.+?)\s*\}\}$`)
	m := conditional.FindStringSubmatch(group)
	if m == nil {
		t.Fatalf("pr.yml concurrency.group = %q, want a github.event_name == 'merge_group' && <queue form> || <pull request form> expression", group)
	}
	mergeGroupForm, pullRequestForm := m[1], m[2]
	if mergeGroupForm == pullRequestForm {
		t.Fatalf("pr.yml concurrency.group uses %q for both events", mergeGroupForm)
	}
	if !strings.Contains(mergeGroupForm, "github.ref") && !strings.Contains(mergeGroupForm, "merge_group.head_ref") {
		t.Fatalf("pr.yml concurrency.group merge-group form %q is not keyed on the queue ref", mergeGroupForm)
	}
	if !strings.Contains(pullRequestForm, "pull_request.number") {
		t.Fatalf("pr.yml concurrency.group pull-request form %q is not keyed on the pull request number", pullRequestForm)
	}
}

// Changed areas decides the tier from the draft flag and the diff from the
// base branch; a merge-group commit has neither a draft flag nor a base_ref,
// so both need a merge_group branch, and the diff is the merge group's own.
func TestPRWorkflowChangedAreasHandlesMergeGroups(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)

	tier := prWorkflowStep(t, workflow, "changes", "Decide the tier")
	for _, name := range []string{"DRAFT", "CI_FULL_LABEL"} {
		value, ok := tier.Env[name]
		if !ok {
			t.Fatalf("pr.yml Decide the tier has no %s env", name)
		}
		if !strings.Contains(value, "github.event_name != 'merge_group'") {
			t.Errorf("pr.yml Decide the tier %s = %q does not turn off for merge_group, so a queue commit could run the quick tier", name, value)
		}
	}

	areas := prWorkflowStep(t, workflow, "changes", "Decide which areas to run")
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
// the event, or a queue commit evaluates it to nothing.
func TestPRWorkflowPullRequestReadsHaveMergeGroupBranch(t *testing.T) {
	_, raw := loadPRWorkflow(t)
	pullRequestOnly := regexp.MustCompile(`github\.event\.pull_request\.|github\.base_ref`)
	for i, line := range strings.Split(raw, "\n") {
		if !pullRequestOnly.MatchString(line) {
			continue
		}
		if !strings.Contains(line, "github.event_name") || !strings.Contains(line, "merge_group") {
			t.Errorf("pr.yml:%d reads the pull request payload with no merge_group branch: %s", i+1, strings.TrimSpace(line))
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
