package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// With flakes enabled Nix downloads every source nix-emacs-ci evaluates over from its origin, so an
// outage of ftp.gnu.org fails the job; without them the sources come from the emacs-ci binary cache.
func TestEmacsJobsInstallNixWithoutFlakes(t *testing.T) {
	paths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("list .github/workflows: %v (%d files)", err, len(paths))
	}
	checked := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path) // #nosec G304 -- a workflow of this repository
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string            `yaml:"uses"`
					With map[string]string `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(raw, &workflow); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for jobName, job := range workflow.Jobs {
			where := filepath.Base(path) + " job " + jobName
			features, installed := "", false
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "cachix/install-nix-action@") {
					features, installed = nixExperimentalFeatures(step.With["extra_nix_config"]), true
				}
				if !strings.HasPrefix(step.Uses, "purcell/setup-emacs@") {
					continue
				}
				checked++
				if !installed {
					t.Errorf("%s: setup-emacs runs without an install-nix-action step before it", where)
				} else if features == "" || strings.Contains(features, "flakes") {
					t.Errorf("%s: Nix is installed with experimental-features %q; want them set, without flakes", where, features)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no workflow uses purcell/setup-emacs")
	}
}

// nixExperimentalFeatures returns the value a nix.conf fragment assigns last, as Nix reads it.
func nixExperimentalFeatures(config string) string {
	features := ""
	for _, line := range strings.Split(config, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == "experimental-features" {
			features = strings.TrimSpace(value)
		}
	}
	return features
}
