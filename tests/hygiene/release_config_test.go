package hygiene

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The five service binaries every release publishes. A package released from a
// tag ships a digest table pinning all of them for that tag, and the release
// jobs assert it after packaging; this test holds the config to that.
var serviceAssets = []string{
	"sysml-grpc-darwin-amd64",
	"sysml-grpc-darwin-arm64",
	"sysml-grpc-linux-amd64",
	"sysml-grpc-linux-arm64",
	"sysml-grpc-windows-amd64.exe",
}

type circleConfig struct {
	Jobs map[string]struct{ Steps []yaml.Node } `yaml:"jobs"`
	// A workflows entry is a workflow, apart from the `version` key.
	Workflows map[string]yaml.Node `yaml:"workflows"`
}

type workflow struct{ Jobs []yaml.Node }

func (c circleConfig) workflow(t *testing.T, name string) workflow {
	t.Helper()
	node, ok := c.Workflows[name]
	if !ok {
		t.Fatalf("no %s workflow", name)
	}
	var w workflow
	if err := node.Decode(&w); err != nil {
		t.Fatalf("decode %s workflow: %v", name, err)
	}
	return w
}

type runStep struct {
	Name    string
	Command string
}

func loadCircleConfig(t *testing.T) circleConfig {
	t.Helper()
	raw, err := os.ReadFile("../../.circleci/config.yml")
	if err != nil {
		t.Fatalf("read .circleci/config.yml: %v", err)
	}
	var config circleConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatalf("parse .circleci/config.yml: %v", err)
	}
	return config
}

// runSteps flattens a job's steps, descending into when/unless blocks, into the
// run steps in the order they execute.
func runSteps(t *testing.T, steps []yaml.Node) []runStep {
	t.Helper()
	var runs []runStep
	for _, step := range steps {
		if step.Kind != yaml.MappingNode || len(step.Content) < 2 {
			continue
		}
		key, value := step.Content[0].Value, step.Content[1]
		switch key {
		case "run":
			var run runStep
			if err := value.Decode(&run); err != nil {
				t.Fatalf("decode run step: %v", err)
			}
			runs = append(runs, run)
		case "when", "unless":
			var block struct{ Steps []yaml.Node }
			if err := value.Decode(&block); err != nil {
				t.Fatalf("decode %s block: %v", key, err)
			}
			runs = append(runs, runSteps(t, block.Steps)...)
		}
	}
	return runs
}

// stepIndex is the position of the first run step whose command contains every
// fragment, or -1.
func stepIndex(runs []runStep, fragments ...string) int {
	for i, run := range runs {
		if containsAll(run.Command, fragments...) {
			return i
		}
	}
	return -1
}

func containsAll(text string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}

func stepHasBareStep(steps []yaml.Node, name string) bool {
	for _, step := range steps {
		if step.Kind == yaml.ScalarNode && step.Value == name {
			return true
		}
		if step.Kind == yaml.MappingNode && len(step.Content) >= 1 && step.Content[0].Value == name {
			return true
		}
	}
	return false
}

// requiresOf is what the named workflow job requires.
func requiresOf(t *testing.T, workflow workflow, name string) []string {
	t.Helper()
	for _, job := range workflow.Jobs {
		if job.Kind != yaml.MappingNode || len(job.Content) < 2 {
			continue
		}
		var spec struct {
			Name     string   `yaml:"name"`
			Requires []string `yaml:"requires"`
		}
		if err := job.Content[1].Decode(&spec); err != nil {
			t.Fatalf("decode workflow job %s: %v", job.Content[0].Value, err)
		}
		if spec.Name == name {
			return spec.Requires
		}
	}
	t.Fatalf("workflow has no job named %q", name)
	return nil
}

func requireAll(t *testing.T, what string, have []string, want ...string) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not require %q (requires %q)", what, w, have)
		}
	}
}

// TestPythonDistributionIsStampedWithItsReleaseDigests holds the release
// workflow to the order the Python wheel depends on: the service binaries are
// built first, their digests are stamped into the table the distribution ships
// before `python -m build`, the built wheel and sdist are asserted to pin all
// five assets for the tag, and the manifest build-release signs is written over
// the final distribution and checked against the wheel's pins before signing.
func TestPythonDistributionIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)

	binaries, ok := config.Jobs["build-release-binaries"]
	if !ok {
		t.Fatal("no build-release-binaries job: the binaries must be built before the Python distribution")
	}
	binarySteps := runSteps(t, binaries.Steps)
	if stepIndex(binarySteps, "make build-grpc", "dist/grpc/sysml-grpc-windows-amd64.exe") < 0 {
		t.Error("build-release-binaries does not build the sysml-grpc binaries into dist/grpc")
	}
	if stepIndex(binarySteps, "SHA256SUMS.txt") >= 0 {
		t.Error("build-release-binaries writes the checksum manifest, which must list the wheel built after it")
	}

	python, ok := config.Jobs["build-python-package"]
	if !ok {
		t.Fatal("no build-python-package job")
	}
	if !stepHasBareStep(python.Steps, "attach_workspace") {
		t.Error("build-python-package does not attach the workspace that carries dist/grpc")
	}
	pythonSteps := runSteps(t, python.Steps)
	stamp := stepIndex(pythonSteps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "${CIRCLE_TAG}"`,
		"--from-binaries dist/grpc",
		"--table client/python/opensysml/release-digests.json",
	)
	build := stepIndex(pythonSteps, "python -m build")
	assert := stepIndex(pythonSteps, "release-digests.json", "zipfile", "tarfile", "pinned_digest")
	switch {
	case stamp < 0:
		t.Error("build-python-package does not stamp the release's digests from dist/grpc into the distribution's table")
	case build < 0:
		t.Error("build-python-package does not run python -m build")
	case stamp > build:
		t.Errorf("the digest stamp (step %d) runs after python -m build (step %d), so the wheel ships without it", stamp, build)
	}
	switch {
	case assert < 0:
		t.Error("build-python-package does not assert the built wheel and sdist pin the release's service digests")
	case assert < build:
		t.Errorf("the distribution assertion (step %d) runs before python -m build (step %d)", assert, build)
	default:
		for _, asset := range serviceAssets {
			if !strings.Contains(pythonSteps[assert].Command, asset) {
				t.Errorf("the distribution assertion does not require %s", asset)
			}
		}
	}

	releaseJob, ok := config.Jobs["build-release"]
	if !ok {
		t.Fatal("no build-release job")
	}
	releaseSteps := runSteps(t, releaseJob.Steps)
	manifest := stepIndex(releaseSteps, "> SHA256SUMS.txt", "opensysml-*.whl")
	check := stepIndex(releaseSteps, "SHA256SUMS.txt", "release-digests.json", "zipfile", "nothing was signed")
	sign := stepIndex(releaseSteps, "cosign sign-blob SHA256SUMS.txt")
	switch {
	case manifest < 0:
		t.Error("build-release does not write SHA256SUMS.txt over the Python distribution")
	case sign < 0:
		t.Error("build-release does not sign SHA256SUMS.txt")
	case check < 0:
		t.Error("build-release does not check the wheel's pins against the manifest it signs")
	case !(manifest < check && check < sign):
		t.Errorf("build-release must write the manifest (step %d), check the wheel against it (step %d), then sign it (step %d)", manifest, check, sign)
	}
	if stepIndex(releaseSteps, "make build-grpc") >= 0 {
		t.Error("build-release rebuilds binaries, so the bytes signed are not the bytes the distribution was stamped from")
	}

	release := config.workflow(t, "release")
	requireAll(t, "Build Python distribution", requiresOf(t, release, "Build Python distribution"), "Build release binaries")
	requireAll(t, "Build release artifacts", requiresOf(t, release, "Build release artifacts"), "Build release binaries", "Build Python distribution")
	requireAll(t, "Publish GitHub release", requiresOf(t, release, "Publish GitHub release"), "Build release artifacts")
}

// TestRustCrateIsStampedWithItsReleaseDigests holds publish-crates to the same
// contract: stamp from the signed manifest before cargo package, then assert
// the packaged crate pins all five assets for the tag.
func TestRustCrateIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)
	crates, ok := config.Jobs["publish-crates"]
	if !ok {
		t.Fatal("no publish-crates job")
	}
	steps := runSteps(t, crates.Steps)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		"--from-manifest dist/SHA256SUMS.txt",
		"--table client/rust/opensysml/release-digests.json",
	)
	pack := stepIndex(steps, "cargo package")
	assert := stepIndex(steps, "release-digests.json", "tarfile")
	switch {
	case stamp < 0:
		t.Error("publish-crates does not stamp the release's digests into the crate's table")
	case pack < 0:
		t.Error("publish-crates does not run cargo package")
	case stamp > pack:
		t.Errorf("the digest stamp (step %d) runs after cargo package (step %d)", stamp, pack)
	}
	if assert < 0 {
		t.Fatal("publish-crates does not assert the packaged crate pins the release's service digests")
	}
	for _, asset := range serviceAssets {
		if !strings.Contains(steps[assert].Command, asset) {
			t.Errorf("the crate assertion does not require %s", asset)
		}
	}
}
