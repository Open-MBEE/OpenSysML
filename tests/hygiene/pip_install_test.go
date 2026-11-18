package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every pip install in CI goes through .github/actions/pip-install, which installs from a directory of
// wheels kept in the Actions cache, so a truncated answer from PyPI is retried and a warm run asks it nothing.

const (
	pipInstallAction = "./.github/actions/pip-install"
	pipInstallScript = "scripts/ci-pip-install.sh"
	pipCacheWorkflow = "../../.github/workflows/pip-cache.yml"
)

// pipCommand matches pip run as a command, not named in a message a step prints.
var pipCommand = regexp.MustCompile(`(?m)(?:(?:^|&&|;|\|\|)\s*@?(?:cd \S+ && )?(?:\S+\s+-m\s+)?|\(\s*)pip3?\s+(?:install|download)\b`)

// pipMakeTargets lists the Makefile targets whose recipe runs pip.
func pipMakeTargets(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	target := regexp.MustCompile(`^([A-Za-z0-9_-]+):`)
	var targets []string
	current := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if match := target.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if current != "" && strings.HasPrefix(line, "\t") && pipCommand.MatchString(strings.TrimPrefix(line, "\t")) {
			targets = append(targets, current)
			current = ""
		}
	}
	if len(targets) < 3 {
		t.Fatalf("found %d Makefile targets that run pip (%v); the scan no longer finds them", len(targets), targets)
	}
	return targets
}

// pipRequest is what one use of the action asks for, with the Python it asks it of.
type pipRequest struct {
	python, packages, requirements, projects string
}

func pipRequestOf(python string, with map[string]string) pipRequest {
	fields := func(value string) string { return strings.Join(strings.Fields(value), " ") }
	return pipRequest{python, fields(with["packages"]), fields(with["requirements"]), fields(with["projects"])}
}

// TestPipInstallsGoThroughTheCachedAction: a step that runs pip, or a make target that does, reads
// only the wheels a use of the action in its job provided, and every use of the action names a request.
func TestPipInstallsGoThroughTheCachedAction(t *testing.T) {
	var targets []string
	for _, target := range pipMakeTargets(t) {
		targets = append(targets, regexp.QuoteMeta(target))
	}
	pipTarget := regexp.MustCompile(`\bmake\b[^\n]*\s(` + strings.Join(targets, "|") + `)\b`)
	wheels := regexp.MustCompile(`steps\.([A-Za-z0-9_-]+)\.outputs\.wheels`)

	uses := 0
	for _, path := range actionsFiles(t) {
		if strings.HasSuffix(filepath.ToSlash(path), "/.github/actions/pip-install/action.yml") {
			continue
		}
		for where, steps := range actionsSteps(t, path) {
			provided := map[string]bool{}
			for _, step := range steps {
				if step.Uses == pipInstallAction {
					uses++
					provided[step.ID] = true
					if pipRequestOf("", step.With) == (pipRequest{}) {
						t.Errorf("%s step %q uses %s without packages, requirements or projects", where, step.Name, pipInstallAction)
					}
					continue
				}
				if strings.Contains(step.Run, pipInstallScript) {
					t.Errorf("%s step %q runs %s directly; use `uses: %s` so the wheels are cached", where, step.Name, pipInstallScript, pipInstallAction)
				}
				if !pipCommand.MatchString(step.Run) && !pipTarget.MatchString(step.Run) {
					continue
				}
				from := wheels.FindStringSubmatch(step.Env["PIP_FIND_LINKS"])
				if step.Env["PIP_NO_INDEX"] != "1" || from == nil || !provided[from[1]] {
					t.Errorf("%s step %q runs pip against PyPI; install through `uses: %s`, or set PIP_NO_INDEX to 1 and PIP_FIND_LINKS to the wheels output of an earlier use of it", where, step.Name, pipInstallAction)
				}
			}
		}
	}
	if uses < 8 {
		t.Errorf("found %d uses of %s; the scan no longer finds the callers", uses, pipInstallAction)
	}
}

// TestPipInstallActionInstallsFromTheRestoredWheels: the action restores, by the exact key alone, the
// directory the script installs from, and saves it under that key only where a caller asks it to.
func TestPipInstallActionInstallsFromTheRestoredWheels(t *testing.T) {
	action, raw := loadCompositeAction(t, "../../.github/actions/pip-install/action.yml")
	if action.Runs.Using != "composite" {
		t.Errorf("the action runs using %q, not composite", action.Runs.Using)
	}
	if !strings.Contains(raw, "    default: 'false'\n\noutputs:") {
		t.Error("the save input does not default to false, so a pull request's run would save entries of its own")
	}

	var restore, install, save *actionsStep
	for i := range action.Runs.Steps {
		step := &action.Runs.Steps[i]
		switch {
		case strings.HasPrefix(step.Uses, "actions/cache/restore@"):
			restore = step
		case strings.HasPrefix(step.Uses, "actions/cache/save@"):
			save = step
		case strings.Contains(step.Run, pipInstallScript):
			install = step
		}
	}
	if restore == nil || install == nil || save == nil {
		t.Fatalf("the action lacks a cache restore (%v), an install (%v) or a cache save (%v) step", restore != nil, install != nil, save != nil)
	}
	cachePinsAgreeWithTheWorkflows(t, "/.github/actions/pip-install/action.yml", restore, save)

	path := restore.With["path"]
	if path == "" {
		t.Error("the restore step names no path")
	}
	if save.With["path"] != path {
		t.Errorf("the save step saves %q, the restore step restored %q", save.With["path"], path)
	}
	if install.Env["CI_PIP_WHEELS"] != path {
		t.Errorf("the install step's CI_PIP_WHEELS is %q, the restored path is %q", install.Env["CI_PIP_WHEELS"], path)
	}
	if save.With["key"] != restore.With["key"] || restore.With["key"] == "" {
		t.Errorf("the save key %q is not the restore key %q", save.With["key"], restore.With["key"])
	}
	if restore.With["restore-keys"] != "" {
		t.Errorf("the restore step falls back to %q, a set saved for another request", restore.With["restore-keys"])
	}
	for _, part := range []string{"steps.restore.outputs.cache-hit != 'true'", "inputs.save == 'true'"} {
		if !strings.Contains(save.If, part) {
			t.Errorf("the save step's condition %q lacks %s", save.If, part)
		}
	}

	// The script, not the action, keeps PyPI out of the install and bounds what the download may cost.
	script, err := os.ReadFile("../../" + pipInstallScript)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`pip install --disable-pip-version-check --no-index --find-links "$wheels"`, "pip download --disable-pip-version-check --no-cache-dir", "CI_PIP_ATTEMPTS", "sha256sum --check --strict"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("%s lacks %q", pipInstallScript, want)
		}
	}
}

// pipRequests lists the requests the workflows make of the action, each under the Python its job
// set up last ("" for the runner's own), following a composite action into the jobs that use it.
func pipRequests(t *testing.T) map[pipRequest]string {
	t.Helper()
	enabled := regexp.MustCompile(`^inputs\.([a-z-]+) == 'true'$`)
	type inner struct {
		step actionsStep
		path string
	}
	composite := map[string][]inner{}
	for _, path := range actionsFiles(t) {
		slashed := filepath.ToSlash(path)
		if filepath.Base(path) != "action.yml" || strings.HasSuffix(slashed, "/.github/actions/pip-install/action.yml") {
			continue
		}
		name := "./" + strings.TrimSuffix(strings.TrimPrefix(slashed, "../../"), "/action.yml")
		for _, steps := range actionsSteps(t, path) {
			for _, step := range steps {
				if step.Uses == pipInstallAction {
					composite[name] = append(composite[name], inner{step, path})
				}
			}
		}
	}

	requests := map[pipRequest]string{}
	for _, path := range actionsFiles(t) {
		if filepath.Base(path) == "action.yml" || filepath.Base(path) == filepath.Base(pipCacheWorkflow) {
			continue
		}
		for where, steps := range actionsSteps(t, path) {
			python := ""
			for _, step := range steps {
				switch {
				case strings.HasPrefix(step.Uses, "actions/setup-python@"):
					python = step.With["python-version"]
				case step.Uses == pipInstallAction:
					requests[pipRequestOf(python, step.With)] = where
				}
				for _, use := range composite[step.Uses] {
					if input := enabled.FindStringSubmatch(use.step.If); use.step.If != "" && (input == nil || step.With[input[1]] != "true") {
						if input == nil {
							t.Errorf("%s step %q runs under %q, a condition this test cannot follow into its callers", use.path, use.step.Name, use.step.If)
						}
						continue
					}
					requests[pipRequestOf(python, use.step.With)] = where + " through " + use.path
				}
			}
		}
	}
	if len(requests) < 8 {
		t.Fatalf("found %d requests of %s; the scan no longer finds them", len(requests), pipInstallAction)
	}
	return requests
}

// TestPipCacheWarmsEveryRequest: the workflow that saves the wheels on develop downloads exactly the
// requests the other workflows make, so each of them starts from a saved entry and none is stale.
func TestPipCacheWarmsEveryRequest(t *testing.T) {
	raw, err := os.ReadFile(pipCacheWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
			} `yaml:"push"`
			Schedule []map[string]string `yaml:"schedule"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct {
					Include []map[string]string `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []actionsStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse %s: %v", pipCacheWorkflow, err)
	}
	if len(workflow.On.Push.Branches) != 1 || workflow.On.Push.Branches[0] != "develop" || len(workflow.On.Schedule) == 0 {
		t.Error("the workflow does not run on a push to develop and daily, so its entries are not develop's or are evicted")
	}
	warm := workflow.Jobs["warm"]

	saves, python := 0, false
	for _, step := range warm.Steps {
		if strings.HasPrefix(step.Uses, "actions/setup-python@") {
			python = step.With["python-version"] == "${{ matrix.python-version }}" && strings.Contains(step.If, "matrix.python-version != ''")
		}
		if step.Uses != pipInstallAction {
			continue
		}
		for _, input := range []string{"packages", "requirements", "projects"} {
			if step.With[input] != "${{ matrix."+input+" }}" {
				t.Errorf("step %q does not pass the matrix entry's %s to the action", step.Name, input)
			}
		}
		if step.With["save"] == "true" {
			saves++
		}
	}
	if !python {
		t.Error("the warm job does not set up the matrix entry's Python, or sets one up where the entry names none")
	}
	if saves != 1 {
		t.Errorf("%d steps of the warm job save the wheels, expected one", saves)
	}

	requests := pipRequests(t)
	warmed := map[pipRequest]bool{}
	for _, entry := range warm.Strategy.Matrix.Include {
		request := pipRequestOf(entry["python-version"], entry)
		if warmed[request] {
			t.Errorf("the matrix warms %+v twice", request)
		}
		warmed[request] = true
		if _, used := requests[request]; !used {
			t.Errorf("the matrix entry %q warms %+v, which no workflow asks for", entry["name"], request)
		}
	}
	for request, where := range requests {
		if !warmed[request] {
			t.Errorf("%s asks for %+v, which %s does not warm", where, request, pipCacheWorkflow)
		}
	}
}
