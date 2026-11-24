package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Maven Central answers a burst of resolutions with HTTP 429 and Maven gives up after three
// retries: every tree asks for eight, and the Java jobs start from a cache saved on develop.

// Maven 3.10 reads the transport name and 3.9 the connector name; each ignores the other.
var mavenBackoff = []string{
	"-Daether.transport.http.retryHandler.count=8",
	"-Daether.connector.http.retryHandler.count=8",
}

var mavenFile = regexp.MustCompile(`\bmvn\b[^\n]*?-f (\S+)/pom\.xml`)

// mavenTrees lists the directories whose pom.xml no parent directory's pom.xml aggregates.
func mavenTrees(t *testing.T) []string {
	t.Helper()
	var trees []string
	for _, pattern := range []string{"../../client/*/pom.xml", "../../editors/*/pom.xml"} {
		poms, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, pom := range poms {
			trees = append(trees, strings.TrimPrefix(filepath.ToSlash(filepath.Dir(pom)), "../../"))
		}
	}
	if len(trees) == 0 {
		t.Fatal("no Maven tree under client/ or editors/")
	}
	return trees
}

func TestEveryMavenTreeRetriesARateLimit(t *testing.T) {
	for _, tree := range mavenTrees(t) {
		config := filepath.Join("../..", tree, ".mvn", "maven.config")
		raw, err := os.ReadFile(config) // #nosec G304 -- a file of this repository
		if err != nil {
			t.Errorf("%s has no .mvn/maven.config, so a 429 from Maven Central fails its build after three retries: %v", tree, err)
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		for _, want := range mavenBackoff {
			found := false
			for _, line := range lines {
				found = found || line == want
			}
			if !found {
				t.Errorf("%s/.mvn/maven.config lacks the line %s", tree, want)
			}
		}
	}

	// The pilot validator is built in a clone of another repository, which has no such file.
	script, err := os.ReadFile("../../scripts/download-pilot-validator.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range mavenBackoff {
		if !strings.Contains(string(script), want) {
			t.Errorf("scripts/download-pilot-validator.sh does not pass %s to mvn", want)
		}
	}
}

// TestCIRunsMavenOnlyInATreeThatRetries fails on a mvn command whose -f names a pom.xml outside the trees.
func TestCIRunsMavenOnlyInATreeThatRetries(t *testing.T) {
	trees := map[string]bool{}
	for _, tree := range mavenTrees(t) {
		trees[tree] = true
	}
	configs, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	configs = append(configs, "../../.circleci/config.yml", "../../Makefile")
	seen := 0
	for _, path := range configs {
		raw, err := os.ReadFile(path) // #nosec G304 -- a file of this repository
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range mavenFile.FindAllStringSubmatch(string(raw), -1) {
			seen++
			tree := strings.Trim(match[1], `"`)
			if tree != "$$dir" && !trees[tree] {
				t.Errorf("%s runs mvn -f %s/pom.xml, which is not a tree with a .mvn/maven.config", filepath.Base(path), tree)
			}
		}
		for _, line := range strings.Split(string(raw), "\n") {
			command := strings.TrimSpace(line)
			command = strings.TrimPrefix(strings.TrimPrefix(command, "run: "), "command: ")
			if strings.HasPrefix(command, "mvn ") && !strings.Contains(command, " -f ") {
				t.Errorf("%s runs mvn without -f, so no tree's .mvn/maven.config applies: %s", filepath.Base(path), command)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no workflow runs mvn -f")
	}
}

const (
	mavenCacheAction = "./.github/actions/maven-cache"
	mavenCacheKey    = "maven-deps-${{ runner.os }}-${{ hashFiles('**/pom.xml', '.github/workflows/maven-cache.yml') }}"
)

// mavenWorkflow reads the steps of any workflow, whatever the shape of its other keys.
type mavenWorkflow struct {
	Jobs map[string]struct {
		TimeoutMinutes int `yaml:"timeout-minutes"`
		Steps          []struct {
			Name           string         `yaml:"name"`
			Uses           string         `yaml:"uses"`
			Run            string         `yaml:"run"`
			With           map[string]any `yaml:"with"`
			TimeoutMinutes int            `yaml:"timeout-minutes"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestMavenCacheIsSavedOnDevelopAlone fails when a job other than the warm one could save a partial entry.
func TestMavenCacheIsSavedOnDevelopAlone(t *testing.T) {
	workflow, raw := loadActionsWorkflow(t, "maven-cache.yml")
	for _, want := range []string{"branches: [develop]", "- '**/pom.xml'", "schedule:"} {
		if !strings.Contains(raw, want) {
			t.Errorf("maven-cache.yml does not trigger on %s", want)
		}
	}
	// A cancelled resolve leaves a partial repository, which the entry must never hold.
	if !regexp.MustCompile(`(?m)^concurrency:\n  group: maven-cache\n  cancel-in-progress: false$`).MatchString(raw) {
		t.Error("maven-cache.yml must queue behind a running warm rather than cancel it part-way")
	}
	saves, checkedOut := false, false
	resolved := map[string]bool{}
	for _, step := range workflow.Jobs["warm"].Steps {
		checkedOut = checkedOut || strings.HasPrefix(step.Uses, "actions/checkout@")
		if step.Uses == mavenCacheAction {
			saves = step.With["save"] == "true" && len(resolved) == 0 && checkedOut
		}
		for _, match := range mavenFile.FindAllStringSubmatch(step.Run, -1) {
			resolved[match[1]] = true
		}
	}
	if !saves {
		t.Errorf("maven-cache.yml's warm job must use %s with save after checkout and before it runs mvn", mavenCacheAction)
	}
	// Surefire resolves its provider and the JUnit launcher only when tests run.
	purged := false
	offline := map[string]bool{}
	for _, step := range workflow.Jobs["warm"].Steps {
		if strings.Contains(step.Run, "skipTests") || strings.Contains(step.Run, "maven.test.skip") {
			t.Errorf("maven-cache.yml step %q skips the tests, so the entry lacks what a test run resolves", step.Name)
		}
		purged = purged || strings.Contains(step.Run, "find ~/.m2/repository -name '*.lastUpdated' -delete")
		for _, line := range strings.Split(step.Run, "\n") {
			if match := mavenFile.FindStringSubmatch(line); match != nil && strings.Contains(line, "mvn -o ") {
				// Checked after the purge, so the saved entry is what the offline build read.
				offline[match[1]] = purged
			}
		}
	}
	if !purged {
		t.Error("maven-cache.yml must delete *.lastUpdated: a restored marker of a failed download stops the retry")
	}
	for _, tree := range mavenTrees(t) {
		if !resolved[tree] {
			t.Errorf("maven-cache.yml resolves no dependencies of %s, so the entry it saves lacks them", tree)
		}
		if !offline[tree] {
			t.Errorf("maven-cache.yml must build %s with mvn -o after the purge, so an incomplete entry is not saved", tree)
		}
	}

	_, action := loadCompositeAction(t, "../../.github/actions/maven-cache/action.yml")
	if strings.Count(action, "key: "+mavenCacheKey+"\n") != 2 {
		t.Error("the maven-cache action's restore and save steps must share one key over every pom.xml and the warm workflow")
	}

	workflows, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	readers := 0
	for _, path := range workflows {
		name := filepath.Base(path)
		if name == "maven-cache.yml" {
			continue
		}
		raw, err := os.ReadFile(path) // #nosec G304 -- a file of this repository
		if err != nil {
			t.Fatal(err)
		}
		var file mavenWorkflow
		if err := yaml.Unmarshal(raw, &file); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		// setup-java's own cache is saved per ref under one key, whichever tree the job resolved.
		if strings.Contains(string(raw), "cache: maven") {
			t.Errorf("%s sets cache: maven on setup-java; use %s instead", name, mavenCacheAction)
		}
		for id, job := range file.Jobs {
			restored, runsMaven, checkedOut := false, false, false
			for _, step := range job.Steps {
				checkedOut = checkedOut || strings.HasPrefix(step.Uses, "actions/checkout@")
				if step.Uses == mavenCacheAction {
					restored = true
					if !checkedOut {
						t.Errorf("%s job %s uses %s before checkout, so its key hashes no pom.xml", name, id, mavenCacheAction)
					}
					if _, saves := step.With["save"]; saves {
						t.Errorf("%s job %s passes save to %s: only maven-cache.yml saves the entry", name, id, mavenCacheAction)
					}
				}
				// nightly.yml builds a commit that may predate the action, so it restores by the key itself.
				if key, _ := step.With["key"].(string); strings.HasPrefix(key, "maven-deps-") {
					restored = true
					// Before checkout no pom.xml exists, so the key's hash is empty and any older entry restores.
					if !checkedOut {
						t.Errorf("%s job %s step %q restores the Maven cache before checkout, so its key hashes no pom.xml", name, id, step.Name)
					}
					if !strings.HasPrefix(step.Uses, "actions/cache/restore@") || key != mavenCacheKey || step.With["path"] != "~/.m2/repository" {
						t.Errorf("%s job %s step %q must only restore ~/.m2/repository under %s", name, id, step.Name, mavenCacheKey)
					}
				}
				if mavenFile.MatchString(step.Run) || strings.Contains(step.Run, "make audit-java") {
					runsMaven = true
					if !restored {
						t.Errorf("%s job %s runs mvn in step %q before %s", name, id, step.Name, mavenCacheAction)
					}
				}
			}
			if runsMaven {
				readers++
			}
		}
	}
	if readers == 0 {
		t.Fatal("no workflow job runs mvn")
	}
}

// TestJavaAuditFallsBackToAMirror holds the SBOM plugin, which the Maven cache lacks, behind the
// script that asks a mirror when Maven Central refuses it.
func TestJavaAuditFallsBackToAMirror(t *testing.T) {
	raw, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `scripts/maven-sbom.sh "$$dir" $(CYCLONEDX_MAVEN_VERSION)`) {
		t.Error("the Makefile's audit-java does not write the SBOM through scripts/maven-sbom.sh")
	}
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "mvn ") && strings.Contains(line, "cyclonedx-maven-plugin") {
			t.Errorf("Makefile:%d runs the CycloneDX plugin itself, so one refusal from Maven Central fails the audit", i+1)
		}
	}
	script, err := os.ReadFile("../../scripts/maven-sbom.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"maven-central-mirror-settings.xml", "MAVEN_SBOM_ATTEMPTS", "MAVEN_SBOM_TIMEOUT_SECONDS", `-f "$tree/pom.xml"`, "-DincludeTestScope=false"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("scripts/maven-sbom.sh lacks %q", want)
		}
	}
	settings, err := os.ReadFile("../../scripts/maven-central-mirror-settings.xml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), "<mirrorOf>central</mirrorOf>") {
		t.Error("scripts/maven-central-mirror-settings.xml mirrors something other than central alone")
	}
}

// TestJavaAuditKeepsToItsOwnBudget holds a pull request's audit of one Maven tree under a step bound
// shorter than its job's, and after the tests it would otherwise starve of the job's time.
func TestJavaAuditKeepsToItsOwnBudget(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/pr.yml")
	if err != nil {
		t.Fatal(err)
	}
	var file mavenWorkflow
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	audits := 0
	for id, job := range file.Jobs {
		audited := false
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "make audit-java") {
				audits++
				audited = true
				if step.TimeoutMinutes <= 0 || step.TimeoutMinutes >= job.TimeoutMinutes {
					t.Errorf("pr.yml job %s step %q has timeout-minutes %d; want one under the job's %d", id, step.Name, step.TimeoutMinutes, job.TimeoutMinutes)
				}
				continue
			}
			if audited && (strings.Contains(step.Run, "mvn ") || strings.Contains(step.Run, "conformance")) {
				t.Errorf("pr.yml job %s runs %q after the audit, whose wait for Maven Central spends that step's time", id, step.Name)
			}
		}
	}
	if audits == 0 {
		t.Fatal("no pr.yml job runs make audit-java")
	}
}
