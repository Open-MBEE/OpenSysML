package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

func TestMavenCacheIsSavedOnDevelop(t *testing.T) {
	workflow, raw := loadActionsWorkflow(t, "maven-cache.yml")
	for _, want := range []string{"branches: [develop]", "- '**/pom.xml'"} {
		if !strings.Contains(raw, want) {
			t.Errorf("maven-cache.yml does not trigger on %s", want)
		}
	}
	steps := workflow.Jobs["warm"].Steps
	if len(steps) < 2 || !strings.HasPrefix(steps[0].Uses, "actions/checkout@") ||
		!strings.HasPrefix(steps[1].Uses, "actions/setup-java@") || steps[1].With["cache"] != "maven" {
		t.Fatal("maven-cache.yml's warm job must check out and then set up Java with cache: maven, before anything adds a pom.xml to the key")
	}
	resolved := map[string]bool{}
	for _, step := range steps {
		for _, match := range mavenFile.FindAllStringSubmatch(step.Run, -1) {
			resolved[match[1]] = true
		}
	}
	for _, tree := range mavenTrees(t) {
		if !resolved[tree] {
			t.Errorf("maven-cache.yml resolves no dependencies of %s, so its job starts cold", tree)
		}
	}

	// A reader takes the entry only under the key the warm job saved: the same action, the same input.
	pr, _ := loadPRWorkflow(t)
	readers := 0
	for name, job := range pr.Jobs {
		runsMaven := false
		for _, step := range job.Steps {
			runsMaven = runsMaven || mavenFile.MatchString(step.Run) || strings.Contains(step.Run, "make audit-java")
		}
		if !runsMaven {
			continue
		}
		readers++
		cached := false
		for _, step := range job.Steps {
			cached = cached || (step.Uses == steps[1].Uses && step.With["cache"] == "maven")
		}
		if !cached {
			t.Errorf("pr.yml job %s runs mvn without the setup-java step (cache: maven) that maven-cache.yml fills", name)
		}
	}
	if readers == 0 {
		t.Fatal("no pr.yml job runs mvn")
	}
}
