package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The packages pinning platform packages at their own version, which a release branch's lock cannot
// lock before the tag publishes them; npm 11.6+'s `npm ci` refuses that, `npm install` skips them.
var unpublishablePackageDirs = []string{"client/node", "packaging/npm/sysml-mcp"}

const lockedInstall = "npm install --ignore-scripts --no-audit --no-fund"

func mentionsUnpublishablePackage(text string) bool {
	for _, dir := range unpublishablePackageDirs {
		if strings.Contains(text, dir) {
			return true
		}
	}
	return strings.Contains(text, "$(NODE_DIR)") || strings.Contains(text, "$(MCP_NPM_DIR)")
}

// TestClientInstallsSurviveUnpublishedPlatformPackages: every Makefile, CircleCI and workflow consumer of
// the two packages installs with `npm install`, and every CI step proves the lock unchanged, as `npm ci` did.
func TestClientInstallsSurviveUnpublishedPlatformPackages(t *testing.T) {
	installs, recipes := 0, 0

	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(makefile), "\n") {
		if !strings.HasPrefix(line, "\t") || !mentionsUnpublishablePackage(line) || !strings.Contains(line, "npm") {
			continue
		}
		recipes++
		if strings.Contains(line, "npm ci") {
			t.Errorf("Makefile:%d runs `npm ci` in a package whose platform packages a release branch cannot lock; use `%s`", i+1, lockedInstall)
		}
		if strings.Contains(line, lockedInstall) {
			installs++
		}
	}
	if recipes == 0 {
		t.Error("no Makefile recipe runs npm in client/node or packaging/npm/sysml-mcp; the scan no longer finds them")
	}

	config := loadCircleConfig(t)
	for name, job := range config.Jobs {
		for _, step := range runSteps(t, job.Steps) {
			checkInstallStep(t, ".circleci/config.yml job "+name+" step "+step.Name, step.Command, &installs)
		}
	}
	for name, command := range config.Commands {
		for _, step := range runSteps(t, command.Steps) {
			checkInstallStep(t, ".circleci/config.yml command "+name+" step "+step.Name, step.Command, &installs)
		}
	}

	workflows, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil || len(workflows) == 0 {
		t.Fatalf("no GitHub workflows found: %v", err)
	}
	for _, file := range workflows {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// Only the fields the scan needs: the workflows' other fields take several shapes.
		var workflow struct {
			Jobs map[string]struct {
				Defaults struct {
					Run struct {
						WorkingDirectory string `yaml:"working-directory"`
					} `yaml:"run"`
				} `yaml:"defaults"`
				Steps []struct {
					Name             string `yaml:"name"`
					Run              string `yaml:"run"`
					WorkingDirectory string `yaml:"working-directory"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(raw, &workflow); err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for name, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if step.Run == "" {
					continue
				}
				dir := step.WorkingDirectory
				if dir == "" {
					dir = job.Defaults.Run.WorkingDirectory
				}
				where := filepath.Base(file) + " job " + name + " step " + step.Name
				checkInstallStep(t, where, "cd "+dir+"\n"+step.Run, &installs)
			}
		}
	}
	if installs < 6 {
		t.Errorf("found %d installs of client/node with `%s`; the Makefile, CircleCI and the GitHub workflows have more consumers than that, so the scan misses some", installs, lockedInstall)
	}
}

// checkInstallStep: a CI step that installs one of the packages does so with `npm install`
// and then asserts the lockfile is unchanged.
func checkInstallStep(t *testing.T, where, command string, installs *int) {
	t.Helper()
	if !mentionsUnpublishablePackage(command) {
		return
	}
	if strings.Contains(command, "npm ci") {
		t.Errorf("%s runs `npm ci` in a package whose platform packages a release branch cannot lock; use `%s`", where, lockedInstall)
	}
	if !strings.Contains(command, lockedInstall) {
		return
	}
	*installs++
	if !strings.Contains(command, "git diff --exit-code package-lock.json") {
		t.Errorf("%s installs with `npm install` but does not assert the lockfile is unchanged (`git diff --exit-code package-lock.json`), the check `npm ci` made", where)
	}
}
