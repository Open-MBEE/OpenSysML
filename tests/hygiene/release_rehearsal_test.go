package hygiene

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRehearsalSuiteJobsRunUnderTheStandInTag: the suite jobs that tag their checkout as the
// release must export the stand-in CIRCLE_TAG with it; a local tag alone reads as a release already out.
func TestRehearsalSuiteJobsRunUnderTheStandInTag(t *testing.T) {
	raw, err := os.ReadFile("../../.circleci/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Commands map[string]struct {
			Steps []yaml.Node `yaml:"steps"`
		} `yaml:"commands"`
		Jobs map[string]struct {
			Steps []yaml.Node `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatalf("parse .circleci/config.yml: %v", err)
	}

	checkout, ok := config.Commands["rehearsal-checkout-tag"]
	if !ok {
		t.Fatal("no rehearsal-checkout-tag command")
	}
	if len(checkout.Steps) == 0 || checkout.Steps[0].Kind != yaml.ScalarNode || checkout.Steps[0].Value != "rehearsal-tag" {
		t.Error("rehearsal-checkout-tag does not export the stand-in CIRCLE_TAG (its first step is not rehearsal-tag) before tagging the checkout")
	}

	for _, name := range []string{"go-static", "go-vulncheck", "go-race-test", "go-coverage", "go-gates"} {
		job, ok := config.Jobs[name]
		if !ok {
			t.Errorf("no %s job", name)
			continue
		}
		found := false
		for _, step := range job.Steps {
			if step.Kind == yaml.ScalarNode && step.Value == "rehearsal-checkout-tag" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not run rehearsal-checkout-tag, so a rehearsal runs its suite as a branch build", name)
		}
	}
}
