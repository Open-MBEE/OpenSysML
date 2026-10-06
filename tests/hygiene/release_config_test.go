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

// persistedPaths is every path the steps persist to the workspace, including
// those inside when/unless blocks.
func persistedPaths(t *testing.T, steps []yaml.Node) []string {
	t.Helper()
	var paths []string
	for _, step := range steps {
		if step.Kind != yaml.MappingNode || len(step.Content) < 2 {
			continue
		}
		key, value := step.Content[0].Value, step.Content[1]
		switch key {
		case "persist_to_workspace":
			var persist struct {
				Root  string   `yaml:"root"`
				Paths []string `yaml:"paths"`
			}
			if err := value.Decode(&persist); err != nil {
				t.Fatalf("decode persist_to_workspace: %v", err)
			}
			if persist.Root != "." {
				t.Errorf("persist_to_workspace root is %q; the release jobs share root .", persist.Root)
			}
			paths = append(paths, persist.Paths...)
		case "when", "unless":
			var block struct{ Steps []yaml.Node }
			if err := value.Decode(&block); err != nil {
				t.Fatalf("decode %s block: %v", key, err)
			}
			paths = append(paths, persistedPaths(t, block.Steps)...)
		}
	}
	return paths
}

// TestReleaseJobsPersistDisjointWorkspaceLayers holds build-release to
// persisting only the files it writes. Workspace layers are additive, and a
// path persisted by two upstream jobs fails the attach in every job
// downstream of both, which is every publish job: build-release-binaries
// persists the whole dist tree, so build-release must not persist it again.
func TestReleaseJobsPersistDisjointWorkspaceLayers(t *testing.T) {
	config := loadCircleConfig(t)
	binaries, ok := config.Jobs["build-release-binaries"]
	if !ok {
		t.Fatal("no build-release-binaries job")
	}
	release, ok := config.Jobs["build-release"]
	if !ok {
		t.Fatal("no build-release job")
	}

	upstream := persistedPaths(t, binaries.Steps)
	requireAll(t, "build-release-binaries persists", upstream, "dist")

	// Everything build-release writes into dist: the manifest and what signs
	// it, the sidecars opensysml reads, and the Python distribution it copies in.
	own := func(path string) bool {
		if !strings.HasPrefix(path, "dist/") {
			return false
		}
		switch {
		case strings.Contains(path, "SHA256SUMS.txt"),
			strings.Contains(path, "provenance.intoto.json"),
			strings.HasSuffix(path, ".sha256"),
			strings.HasSuffix(path, ".whl"),
			strings.HasPrefix(path, "dist/opensysml-[0-9]"):
			return true
		}
		return false
	}
	persisted := persistedPaths(t, release.Steps)
	for _, path := range persisted {
		if !own(path) {
			t.Errorf("build-release persists %q, which build-release-binaries' layer already carries; persist only the files build-release writes", path)
		}
	}
	requireAll(t, "build-release persists", persisted,
		"dist/SHA256SUMS.txt",
		"dist/SHA256SUMS.txt.bundle",
		"dist/provenance.intoto.json.bundle",
		"dist/grpc/*.sha256",
		"dist/opensysml-*-py3-none-any.whl",
		"dist/opensysml-[0-9]*.tar.gz",
	)
}

// TestNodePackageIsStampedWithItsReleaseDigests holds publish-npm to the same
// contract as the Python wheel: the digests of the binaries in dist/grpc are
// stamped into the package's table before it is packed, the packed tarball is
// asserted to pin all five assets for the tag, and that verified tarball is
// what is published.
func TestNodePackageIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)
	npm, ok := config.Jobs["publish-npm"]
	if !ok {
		t.Fatal("no publish-npm job")
	}
	if !stepHasBareStep(npm.Steps, "attach_workspace") {
		t.Error("publish-npm does not attach the workspace that carries dist/grpc")
	}
	steps := runSteps(t, npm.Steps)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "${CIRCLE_TAG}"`,
		"--from-binaries dist/grpc",
		"--table client/node/release-digests.json",
	)
	test := stepIndex(steps, "npm test")
	pack := stepIndex(steps, "npm pack --pack-destination")
	assert := stepIndex(steps, "package/release-digests.json", "tarfile", "hashlib")
	publish := stepIndex(steps, "npm publish", "--access public", "NPM_TARBALL")
	switch {
	case stamp < 0:
		t.Error("publish-npm does not stamp the release's digests from dist/grpc into the package's table")
	case test < 0:
		t.Error("publish-npm does not run the Node suite")
	case stamp > test:
		t.Errorf("the digest stamp (step %d) runs after npm test (step %d), so the suite cannot check it", stamp, test)
	case pack < 0:
		t.Error("publish-npm does not pack the client into a tarball")
	case stamp > pack:
		t.Errorf("the digest stamp (step %d) runs after npm pack (step %d), so the tarball ships without it", stamp, pack)
	}
	if test >= 0 && !strings.Contains(steps[test].Command, `OPENSYSML_EXPECT_PINNED_RELEASE="${CIRCLE_TAG}"`) {
		t.Error("publish-npm runs the Node suite without OPENSYSML_EXPECT_PINNED_RELEASE, so the suite does not assert the stamped pin")
	}
	switch {
	case assert < 0:
		t.Fatal("publish-npm does not assert the packed tarball pins the release's service digests")
	case assert != pack:
		t.Errorf("the tarball assertion (step %d) must run in the step that packs it (step %d)", assert, pack)
	}
	for _, asset := range serviceAssets {
		if !strings.Contains(steps[assert].Command, asset) {
			t.Errorf("the tarball assertion does not require %s", asset)
		}
	}
	switch {
	case publish < 0:
		t.Error("publish-npm does not publish the verified tarball; publishing the directory would pack it again unverified")
	case publish < assert:
		t.Errorf("npm publish (step %d) runs before the tarball is verified (step %d)", publish, assert)
	}
	if stepIndex(steps, "cd client/node", "npm pack --dry-run") >= 0 {
		t.Error("publish-npm packs the client a second time instead of using the verified tarball")
	}

	release := config.workflow(t, "release")
	requireAll(t, "Publish opensysml to npm", requiresOf(t, release, "Publish opensysml to npm"), "Publish GitHub release")
}

// TestJavaJarIsStampedWithItsReleaseDigests holds publish-maven to the same
// contract: attach the workspace that carries dist/grpc, stamp the digests
// into the jar's resource before mvn package, assert the packaged jar pins all
// five assets for the tag, and only then deploy.
func TestJavaJarIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)
	maven, ok := config.Jobs["publish-maven"]
	if !ok {
		t.Fatal("no publish-maven job")
	}
	if !stepHasBareStep(maven.Steps, "attach_workspace") {
		t.Error("publish-maven does not attach the workspace that carries dist/grpc")
	}
	steps := runSteps(t, maven.Steps)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "${CIRCLE_TAG}"`,
		"--from-binaries dist/grpc",
		"--table client/java/opensysml-client/src/main/resources/release-digests.json",
	)
	pack := stepIndex(steps, "mvn -B -f client/java/pom.xml", "package -pl :opensysml -am")
	assert := stepIndex(steps, "release-digests.json", "zipfile", "hashlib")
	deploy := stepIndex(steps, "mvn -B -f client/java/pom.xml", "deploy -pl :opensysml -am")
	switch {
	case stamp < 0:
		t.Error("publish-maven does not stamp the release's digests from dist/grpc into the jar's resource")
	case pack < 0:
		t.Error("publish-maven does not run mvn package before deploying")
	case stamp > pack:
		t.Errorf("the digest stamp (step %d) runs after mvn package (step %d), so the jar ships without it", stamp, pack)
	}
	if pack >= 0 {
		if !strings.Contains(steps[pack].Command, `OPENSYSML_EXPECT_PINNED_RELEASE="${CIRCLE_TAG}"`) {
			t.Error("publish-maven packages without OPENSYSML_EXPECT_PINNED_RELEASE, so ReleaseAssetsTest does not assert the stamped pin")
		}
		if !strings.Contains(steps[pack].Command, "-Dtest=ReleaseAssetsTest") {
			t.Error("publish-maven packages without re-running ReleaseAssetsTest over the stamped resource")
		}
	}
	switch {
	case assert < 0:
		t.Fatal("publish-maven does not assert the packaged jar pins the release's service digests")
	case assert != pack:
		t.Errorf("the jar assertion (step %d) must run in the step that packages it (step %d)", assert, pack)
	}
	for _, asset := range serviceAssets {
		if !strings.Contains(steps[assert].Command, asset) {
			t.Errorf("the jar assertion does not require %s", asset)
		}
	}
	switch {
	case deploy < 0:
		t.Error("publish-maven does not deploy")
	case deploy < assert:
		t.Errorf("mvn deploy (step %d) runs before the jar is verified (step %d)", deploy, assert)
	}

	release := config.workflow(t, "release")
	requireAll(t, "Publish opensysml to Maven Central", requiresOf(t, release, "Publish opensysml to Maven Central"), "Publish GitHub release")
}
