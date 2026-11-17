package hygiene

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type checkoutStep struct {
	ID              string            `yaml:"id"`
	Uses            string            `yaml:"uses"`
	If              string            `yaml:"if"`
	TimeoutMinutes  int               `yaml:"timeout-minutes"`
	ContinueOnError bool              `yaml:"continue-on-error"`
	With            map[string]string `yaml:"with"`
}

type checkoutWorkflow struct {
	On   yaml.Node `yaml:"on"`
	Jobs map[string]struct {
		TimeoutMinutes int            `yaml:"timeout-minutes"`
		Steps          []checkoutStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// A stalled fetch hangs until the job's limit cancels it, and a merge-queue group with it. Every checkout of a
// workflow a pull request or the queue runs is bounded and tried once more, inside what its job allows.
func TestPullRequestCheckoutsAreBoundedAndRetried(t *testing.T) {
	paths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("list .github/workflows: %v (%d files)", err, len(paths))
	}
	checked := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var workflow checkoutWorkflow
		if err := yaml.Unmarshal(raw, &workflow); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		var triggers map[string]yaml.Node
		if err := workflow.On.Decode(&triggers); err != nil {
			continue
		}
		_, pr := triggers["pull_request"]
		_, target := triggers["pull_request_target"]
		_, queue := triggers["merge_group"]
		if !pr && !target && !queue {
			continue
		}
		name := filepath.Base(path)
		for jobName, job := range workflow.Jobs {
			for i := 0; i < len(job.Steps); i++ {
				first := job.Steps[i]
				if !strings.HasPrefix(first.Uses, "actions/checkout@") {
					continue
				}
				checked++
				where := name + " job " + jobName
				want := 3
				if first.With["fetch-depth"] == "0" {
					want = 10
				}
				if first.ID != "checkout" || !first.ContinueOnError || first.If != "" || first.TimeoutMinutes != want {
					t.Errorf("%s: checkout has id %q, continue-on-error %v, if %q, timeout-minutes %d; want checkout, true, none, %d",
						where, first.ID, first.ContinueOnError, first.If, first.TimeoutMinutes, want)
				}
				if job.TimeoutMinutes != 0 && job.TimeoutMinutes <= 2*want {
					t.Errorf("%s: timeout-minutes %d leaves nothing after two %d-minute checkouts", where, job.TimeoutMinutes, want)
				}
				if i+1 == len(job.Steps) {
					t.Errorf("%s: checkout is not followed by its retry", where)
					continue
				}
				i++
				retry := job.Steps[i]
				if retry.Uses != first.Uses || retry.If != "steps.checkout.outcome == 'failure'" || retry.ContinueOnError ||
					retry.TimeoutMinutes != want || !maps.Equal(retry.With, first.With) {
					t.Errorf("%s: the step after checkout is not its retry: uses %q, if %q, continue-on-error %v, timeout-minutes %d, with %v",
						where, retry.Uses, retry.If, retry.ContinueOnError, retry.TimeoutMinutes, retry.With)
				}
			}
		}
	}
	if checked < 27 {
		t.Fatalf("found %d checkouts in the pull-request and merge-queue workflows, want at least 27", checked)
	}
}
