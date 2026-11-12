package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every apt install in CI goes through .github/actions/apt-install, which keeps the .deb set a run
// installed from in the Actions cache, so a mirror outage installs the kept set instead of failing the job.

const (
	aptInstallAction = "./.github/actions/apt-install"
	aptInstallScript = "scripts/ci-apt-install.sh"
)

type compositeAction struct {
	Inputs map[string]struct {
		Required bool `yaml:"required"`
	} `yaml:"inputs"`
	Runs struct {
		Using string        `yaml:"using"`
		Steps []actionsStep `yaml:"steps"`
	} `yaml:"runs"`
}

func loadCompositeAction(t *testing.T, path string) (compositeAction, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var action compositeAction
	if err := yaml.Unmarshal(raw, &action); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return action, string(raw)
}

// actionsSteps reads a workflow's or a composite action's steps, each with where it is.
func actionsSteps(t *testing.T, path string) map[string][]actionsStep {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	steps := map[string][]actionsStep{}
	if filepath.Base(path) == "action.yml" {
		var action compositeAction
		if err := yaml.Unmarshal(raw, &action); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		steps[path] = action.Runs.Steps
		return steps
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []actionsStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for name, job := range workflow.Jobs {
		steps[path+" job "+name] = job.Steps
	}
	return steps
}

func actionsFiles(t *testing.T) []string {
	t.Helper()
	workflows, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	actions, err := filepath.Glob("../../.github/actions/*/action.yml")
	if err != nil {
		t.Fatal(err)
	}
	files := append(workflows, actions...)
	if len(files) < 5 {
		t.Fatalf("found %d workflow and action files; the scan no longer finds them", len(files))
	}
	return files
}

// TestAptInstallsGoThroughTheCachedAction: no workflow or action step runs the installer itself,
// which would leave that step to fail on a mirror outage, and every use of the action names packages.
func TestAptInstallsGoThroughTheCachedAction(t *testing.T) {
	uses := 0
	for _, path := range actionsFiles(t) {
		if strings.HasSuffix(filepath.ToSlash(path), "/.github/actions/apt-install/action.yml") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "ci-apt-install.sh") && !strings.Contains(line, "ci-apt-install-test.sh") {
				t.Errorf("%s:%d runs %s directly; use `uses: %s` so the archives are cached", path, i+1, aptInstallScript, aptInstallAction)
			}
		}
		for where, steps := range actionsSteps(t, path) {
			for _, step := range steps {
				if step.Uses != aptInstallAction {
					continue
				}
				uses++
				if strings.TrimSpace(step.With["packages"]) == "" {
					t.Errorf("%s step %q uses %s without packages", where, step.Name, aptInstallAction)
				}
			}
		}
	}
	if uses < 5 {
		t.Errorf("found %d uses of %s; the scan no longer finds the callers", uses, aptInstallAction)
	}
}

// TestAptInstallActionCachesTheArchiveSet: the action restores the set into the directory the script
// seeds and refills, saves it under the restore key plus the set's digest when it changed, and pins as the workflows do.
func TestAptInstallActionCachesTheArchiveSet(t *testing.T) {
	action, _ := loadCompositeAction(t, "../../.github/actions/apt-install/action.yml")
	if action.Runs.Using != "composite" {
		t.Errorf("the action runs using %q, not composite", action.Runs.Using)
	}
	if !action.Inputs["packages"].Required {
		t.Error("the packages input is not required")
	}

	var restore, install, save *actionsStep
	for i := range action.Runs.Steps {
		step := &action.Runs.Steps[i]
		switch {
		case strings.HasPrefix(step.Uses, "actions/cache/restore@"):
			restore = step
		case strings.HasPrefix(step.Uses, "actions/cache/save@"):
			save = step
		case strings.Contains(step.Run, aptInstallScript):
			install = step
		}
	}
	if restore == nil || install == nil || save == nil {
		t.Fatalf("the action lacks a cache restore (%v), an install (%v) or a cache save (%v) step", restore != nil, install != nil, save != nil)
	}

	// The workflows pin the cache actions by commit; the action pins them to the same commits.
	for _, pin := range []struct {
		step   *actionsStep
		action string
	}{{restore, "actions/cache/restore"}, {save, "actions/cache/save"}} {
		got := strings.TrimPrefix(pin.step.Uses, pin.action+"@")
		pinned := regexp.MustCompile(regexp.QuoteMeta(pin.action) + `@([0-9a-f]{40})`)
		pins := 0
		for _, path := range actionsFiles(t) {
			if strings.HasSuffix(filepath.ToSlash(path), "/.github/actions/apt-install/action.yml") {
				continue
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range pinned.FindAllStringSubmatch(string(raw), -1) {
				pins++
				if match[1] != got {
					t.Errorf("the action pins %s to %s; %s pins it to %s", pin.action, got, path, match[1])
				}
			}
		}
		if pins == 0 {
			t.Errorf("no workflow pins %s; the action's pin %s has nothing to agree with", pin.action, got)
		}
	}

	path := restore.With["path"]
	if path == "" {
		t.Error("the restore step names no path")
	}
	if save.With["path"] != path {
		t.Errorf("the save step saves %q, the restore step restored %q", save.With["path"], path)
	}
	if install.Env["CI_APT_ARCHIVES"] != path {
		t.Errorf("the install step's CI_APT_ARCHIVES is %q, the restored path is %q", install.Env["CI_APT_ARCHIVES"], path)
	}
	key := restore.With["key"]
	for _, part := range []string{"steps.set.outputs.name", "runner.os", "steps.set.outputs.image"} {
		if !strings.Contains(key, part) {
			t.Errorf("the restore key %q does not vary with %s", key, part)
		}
	}
	if !strings.HasPrefix(save.With["key"], key) || save.With["key"] == key {
		t.Errorf("the save key %q is not the restore key %q plus the set's digest", save.With["key"], key)
	}
	if !strings.Contains(restore.With["restore-keys"], key) {
		t.Errorf("the restore keys %q do not fall back to the restore key %q", restore.With["restore-keys"], key)
	}
	if !strings.Contains(save.If, "cache-matched-key") {
		t.Errorf("the save step's condition %q does not compare with the matched key, so an unchanged set is saved again", save.If)
	}
}
