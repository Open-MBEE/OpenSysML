package hygiene

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEveryWorkflowJobDeclaresATimeout keeps a hung job from holding a runner for
// GitHub's six-hour default; a job that calls a reusable workflow has none to declare.
func TestEveryWorkflowJobDeclaresATimeout(t *testing.T) {
	paths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflows found: %v", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow struct {
			Jobs map[string]struct {
				Uses           string `yaml:"uses"`
				TimeoutMinutes int    `yaml:"timeout-minutes"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(raw, &workflow); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for name, job := range workflow.Jobs {
			if job.Uses == "" && job.TimeoutMinutes <= 0 {
				t.Errorf("%s: job %s declares no timeout-minutes", filepath.Base(path), name)
			}
		}
	}
}
