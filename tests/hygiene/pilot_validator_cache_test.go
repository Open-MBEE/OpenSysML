package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	pilotValidatorAction = "./.github/actions/pilot-validator"
	pilotValidatorKey    = "pilot-validator-${{ hashFiles('scripts/download-pilot-validator.sh', 'scripts/pilot-pin.sh') }}"
	pilotValidatorPaths  = "build/pilot-validator\n~/.m2/repository\n"
)

// TestPilotValidatorActionRestoresBuildsAndSaves fails when the action's two cache steps could name different entries.
func TestPilotValidatorActionRestoresBuildsAndSaves(t *testing.T) {
	raw, err := os.ReadFile("../../.github/actions/pilot-validator/action.yml")
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Runs struct {
			Steps []struct {
				ID   string            `yaml:"id"`
				If   string            `yaml:"if"`
				Uses string            `yaml:"uses"`
				Run  string            `yaml:"run"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(raw, &action); err != nil {
		t.Fatal(err)
	}
	steps := action.Runs.Steps
	if len(steps) != 3 {
		t.Fatalf("the pilot-validator action has %d steps, want restore, build, save", len(steps))
	}
	restore, build, save := steps[0], steps[1], steps[2]
	if !strings.HasPrefix(restore.Uses, "actions/cache/restore@") || restore.ID != "cache" {
		t.Errorf("the first step must be actions/cache/restore with id cache, got %s (id %q)", restore.Uses, restore.ID)
	}
	// The prefix is what lets a moved pin start from the last build's Maven repository.
	if restore.With["restore-keys"] != "pilot-validator-" {
		t.Errorf("the restore step's restore-keys is %q, want the stable prefix pilot-validator-", restore.With["restore-keys"])
	}
	if strings.TrimSpace(build.Run) != "./scripts/download-pilot-validator.sh" {
		t.Errorf("the second step must run scripts/download-pilot-validator.sh, got %q", build.Run)
	}
	if !strings.HasPrefix(save.Uses, "actions/cache/save@") {
		t.Errorf("the last step must be actions/cache/save, so a later failing step does not lose the build: %s", save.Uses)
	}
	if save.If != "inputs.save == 'true' && steps.cache.outputs.cache-hit != 'true'" {
		t.Errorf("the save step must run only with save and without an exact hit, got if: %s", save.If)
	}
	if _, fallback := save.With["restore-keys"]; fallback {
		t.Error("the save step must not name restore-keys")
	}
	// A cache entry is addressed by its key and its paths together.
	for name, step := range map[string]map[string]string{"restore": restore.With, "save": save.With} {
		if step["key"] != pilotValidatorKey {
			t.Errorf("the %s step's key is %q, want %s", name, step["key"], pilotValidatorKey)
		}
		if step["path"] != pilotValidatorPaths {
			t.Errorf("the %s step's path is %q, want the build and the Maven repository", name, step["path"])
		}
	}

	script, err := os.ReadFile("../../scripts/download-pilot-validator.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "\nmvn --version\n") {
		t.Error("scripts/download-pilot-validator.sh must log mvn --version: the retry property read depends on it")
	}
}

// TestPilotValidatorIsProvisionedThroughItsAction fails on a job that builds the validator without the shared cache.
func TestPilotValidatorIsProvisionedThroughItsAction(t *testing.T) {
	// Every script that delegates to download-pilot-validator.sh, and that script itself.
	scripts, err := filepath.Glob("../../scripts/download-pilot-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	var builders []string
	for _, path := range scripts {
		raw, err := os.ReadFile(path) // #nosec G304 -- a file of this repository
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "scripts/download-pilot-validator.sh\"\n") || filepath.Base(path) == "download-pilot-validator.sh" {
			builders = append(builders, "scripts/"+filepath.Base(path))
		}
	}
	if len(builders) < 2 {
		t.Fatalf("found %d scripts that build the pilot validator, want the script and those that delegate to it", len(builders))
	}

	workflows, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	// Whether each workflow that provisions the validator saves the entry.
	saves := map[string]bool{}
	for _, path := range workflows {
		name := filepath.Base(path)
		raw, err := os.ReadFile(path) // #nosec G304 -- a file of this repository
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "key: pilot-validator-") {
			t.Errorf("%s names the pilot-validator cache itself; use %s, which owns the key and the paths", name, pilotValidatorAction)
		}
		var file mavenWorkflow
		if err := yaml.Unmarshal(raw, &file); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for id, job := range file.Jobs {
			checkedOut, javaSetUp, provisioned := false, false, false
			for _, step := range job.Steps {
				checkedOut = checkedOut || strings.HasPrefix(step.Uses, "actions/checkout@")
				javaSetUp = javaSetUp || strings.HasPrefix(step.Uses, "actions/setup-java@")
				if step.Uses == pilotValidatorAction {
					provisioned = true
					saves[name] = step.With["save"] == "true"
					// The key hashes the two scripts, and a miss builds with Maven.
					if !checkedOut || !javaSetUp {
						t.Errorf("%s job %s uses %s before checkout and setup-java", name, id, pilotValidatorAction)
					}
				}
				for _, builder := range builders {
					if strings.Contains(step.Run, builder) && !provisioned {
						t.Errorf("%s job %s runs %s in step %q before %s", name, id, builder, step.Name, pilotValidatorAction)
					}
				}
			}
		}
	}

	// develop's entry is the one a pull request, a merge group and rebaseline.yml can restore.
	for _, name := range []string{"pilot-validator-cache.yml", "oracle-reproduction.yml", "pr.yml"} {
		if !saves[name] {
			t.Errorf("%s must use %s with save", name, pilotValidatorAction)
		}
	}
	// It runs a pull request's scripts under pull_request_target, so in develop's cache scope.
	if save, uses := saves["rebaseline.yml"]; !uses || save {
		t.Errorf("rebaseline.yml must use %s without save", pilotValidatorAction)
	}
	_, prime := loadActionsWorkflow(t, "pilot-validator-cache.yml")
	if !strings.Contains(prime, "  push:\n    branches: [develop]\n  workflow_dispatch:") {
		t.Error("pilot-validator-cache.yml must run on every push to develop, with no path filter")
	}
	_, nightly := loadActionsWorkflow(t, "oracle-reproduction.yml")
	if !strings.Contains(nightly, "  schedule:") {
		t.Error("oracle-reproduction.yml must stay scheduled: its nightly read keeps develop's entry from eviction")
	}
}
