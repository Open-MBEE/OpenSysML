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

// The five Jupyter kernel binaries a release publishes beside the service;
// jupyter-opensysml-kernel pins them the same way.
var kernelAssets = []string{
	"sysml-jupyter-kernel-darwin-amd64",
	"sysml-jupyter-kernel-darwin-arm64",
	"sysml-jupyter-kernel-linux-amd64",
	"sysml-jupyter-kernel-linux-arm64",
	"sysml-jupyter-kernel-windows-amd64.exe",
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

// TestJupyterKernelDistributionIsStampedWithItsReleaseDigests holds the
// kernel's distributions to the Python wheel's contract: the kernel binaries
// are built into dist/jupyter beside the service, stamped into the kernel
// package's own table before the sdist and the five platform wheels are
// built from them, asserted after, and the manifest build-release signs is
// checked against every kernel wheel's pins.
func TestJupyterKernelDistributionIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)

	script, err := os.ReadFile("../../scripts/build-jupyter-kernel-dist.sh")
	if err != nil {
		t.Fatalf("the kernel distribution build script is missing: %v", err)
	}
	for _, platform := range []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"} {
		if !strings.Contains(string(script), platform) {
			t.Errorf("build-jupyter-kernel-dist.sh builds no wheel for %s", platform)
		}
	}
	for _, want := range []string{"JUPYTER_OPENSYSML_KERNEL_PLATFORM", "-m build --sdist", "-m build --wheel", "sha256sum --check"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("build-jupyter-kernel-dist.sh does not use %q", want)
		}
	}

	binaries, ok := config.Jobs["build-release-binaries"]
	if !ok {
		t.Fatal("no build-release-binaries job")
	}
	binarySteps := runSteps(t, binaries.Steps)
	build := stepIndex(binarySteps, "make build-jupyter-kernel", "dist/jupyter/sysml-jupyter-kernel-windows-amd64.exe")
	if build < 0 {
		t.Fatal("build-release-binaries does not build the sysml-jupyter-kernel binaries into dist/jupyter")
	}
	for _, asset := range kernelAssets {
		if !strings.Contains(binarySteps[build].Command, "dist/jupyter/"+asset) {
			t.Errorf("build-release-binaries does not build dist/jupyter/%s", asset)
		}
	}
	if stepIndex(binarySteps, "check-static-binaries.sh", "dist/jupyter/sysml-jupyter-kernel-linux-amd64", "dist/jupyter/sysml-jupyter-kernel-linux-arm64") < 0 {
		t.Error("build-release-binaries does not check the Linux kernel binaries are statically linked")
	}
	if stepIndex(binarySteps, "--version", "dist/jupyter/sysml-jupyter-kernel-linux-amd64", "dist/jupyter/sysml-jupyter-kernel-*") < 0 {
		t.Error("build-release-binaries does not verify the kernel binaries report the tag")
	}

	python, ok := config.Jobs["build-python-package"]
	if !ok {
		t.Fatal("no build-python-package job")
	}
	if stepIndex(runSteps(t, python.Steps), "check_version.py --jupyter-kernel") < 0 {
		t.Error("build-python-package does not check the kernel package declares the tag's version")
	}

	kernel, ok := config.Jobs["build-jupyter-kernel-package"]
	if !ok {
		t.Fatal("no build-jupyter-kernel-package job")
	}
	if !stepHasBareStep(kernel.Steps, "attach_workspace") {
		t.Error("build-jupyter-kernel-package does not attach the workspace that carries dist/jupyter")
	}
	kernelSteps := runSteps(t, kernel.Steps)
	version := stepIndex(kernelSteps, "check_version.py --jupyter-kernel")
	stamp := stepIndex(kernelSteps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "${CIRCLE_TAG}"`,
		"--from-binaries dist/jupyter",
		"--table client/jupyter-kernel/jupyter_opensysml_kernel/release-digests.json",
	)
	wheel := stepIndex(kernelSteps, "scripts/build-jupyter-kernel-dist.sh dist/jupyter client/jupyter-kernel/dist", "-py3-none-*.whl")
	assert := stepIndex(kernelSteps, "jupyter_opensysml_kernel/release-digests.json", "zipfile", "tarfile", "pinned_digest", "-py3-none-*.whl")
	switch {
	case version < 0:
		t.Error("build-jupyter-kernel-package does not resolve the version from the tag")
	case stamp < 0:
		t.Error("build-jupyter-kernel-package does not stamp the release's digests from dist/jupyter into the distribution's table")
	case wheel < 0:
		t.Error("build-jupyter-kernel-package does not build the sdist and the five platform wheels with scripts/build-jupyter-kernel-dist.sh")
	case stamp > wheel:
		t.Errorf("the digest stamp (step %d) runs after the distributions are built (step %d), so they ship without it", stamp, wheel)
	}
	switch {
	case assert < 0:
		t.Error("build-jupyter-kernel-package does not assert every built wheel and the sdist pin the release's kernel digests")
	case assert < wheel:
		t.Errorf("the distribution assertion (step %d) runs before the distributions are built (step %d)", assert, wheel)
	default:
		for _, asset := range kernelAssets {
			if !strings.Contains(kernelSteps[assert].Command, asset) {
				t.Errorf("the distribution assertion does not require %s", asset)
			}
		}
	}
	if stepIndex(kernelSteps, "python -m jupyter_opensysml_kernel kernelspec") < 0 {
		t.Error("build-jupyter-kernel-package does not render the kernelspec from the installed wheel")
	}
	if stepIndex(kernelSteps, "-py3-none-manylinux*_x86_64*.whl", "jupyter_client.kernelspecapp", `"share", "jupyter", "kernels", "sysml"`, `"jupyter_opensysml_kernel", "-version"`) < 0 {
		t.Error("build-jupyter-kernel-package does not install this platform's wheel alone and check it registers the sysml kernelspec and starts the bundled kernel")
	}

	releaseJob, ok := config.Jobs["build-release"]
	if !ok {
		t.Fatal("no build-release job")
	}
	releaseSteps := runSteps(t, releaseJob.Steps)
	manifest := stepIndex(releaseSteps, "> SHA256SUMS.txt", "jupyter_opensysml_kernel-*.whl", "cd jupyter && sha256sum sysml-jupyter-kernel-*")
	check := stepIndex(releaseSteps, "SHA256SUMS.txt", "jupyter_opensysml_kernel/release-digests.json", "zipfile", "nothing was signed", "jupyter_opensysml_kernel-*-py3-none-*.whl")
	sign := stepIndex(releaseSteps, "cosign sign-blob SHA256SUMS.txt")
	switch {
	case manifest < 0:
		t.Error("build-release does not write SHA256SUMS.txt over the kernel distribution and the kernel binaries")
	case check < 0:
		t.Error("build-release does not check every kernel wheel's pins against the manifest it signs")
	case sign < 0:
		t.Error("build-release does not sign SHA256SUMS.txt")
	case !(manifest < check && check < sign):
		t.Errorf("build-release must write the manifest (step %d), check the kernel wheel against it (step %d), then sign it (step %d)", manifest, check, sign)
	}
	if stepIndex(releaseSteps, "dist/jupyter_opensysml_kernel-*-py3-none-*.whl", "dist/jupyter_opensysml_kernel-[0-9]*.tar.gz", "not the five platform wheels", "Nothing was published") < 0 {
		t.Error("build-release does not verify the release carries the tag's kernel sdist and five platform wheels")
	}
	if stepIndex(releaseSteps, "cosign verify-blob-attestation", "jupyter_opensysml_kernel-*-py3-none-*.whl") < 0 {
		t.Error("build-release does not verify the provenance names the kernel wheels")
	}

	publish, ok := config.Jobs["publish-pypi-jupyter-kernel"]
	if !ok {
		t.Fatal("no publish-pypi-jupyter-kernel job")
	}
	publishSteps := runSteps(t, publish.Steps)
	index := stepIndex(publishSteps, "/pypi/jupyter-opensysml-kernel/", "already on")
	upload := stepIndex(publishSteps, "twine upload client/jupyter-kernel/dist/*")
	switch {
	case index < 0:
		t.Error("publish-pypi-jupyter-kernel does not refuse a version the index already has")
	case upload < 0:
		t.Error("publish-pypi-jupyter-kernel does not upload the kernel distribution")
	case upload < index:
		t.Errorf("twine upload (step %d) runs before the index check (step %d)", upload, index)
	}
	if upload >= 0 {
		for _, line := range strings.Split(publishSteps[upload].Command, "\n") {
			if trimmed := strings.TrimSpace(line); !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "--skip-existing") {
				t.Error("publish-pypi-jupyter-kernel uploads with --skip-existing, so a version that appeared since the check would pass silently")
			}
		}
	}

	github, ok := config.Jobs["publish-github-release"]
	if !ok {
		t.Fatal("no publish-github-release job")
	}
	if stepIndex(runSteps(t, github.Steps), "mv dist/jupyter/* dist/release/") < 0 {
		t.Error("publish-github-release does not publish the kernel binaries and their sidecars")
	}

	release := config.workflow(t, "release")
	requireAll(t, "Build Jupyter kernel distribution", requiresOf(t, release, "Build Jupyter kernel distribution"), "Build release binaries")
	requireAll(t, "Build release artifacts", requiresOf(t, release, "Build release artifacts"), "Build Jupyter kernel distribution")
	requireAll(t, "Publish jupyter-opensysml-kernel to PyPI", requiresOf(t, release, "Publish jupyter-opensysml-kernel to PyPI"), "Publish GitHub release", "Python client tests")
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
	// it, the sidecars the packages read, and the Python distributions it copies in.
	own := func(path string) bool {
		if !strings.HasPrefix(path, "dist/") {
			return false
		}
		switch {
		case strings.Contains(path, "SHA256SUMS.txt"),
			strings.Contains(path, "provenance.intoto.json"),
			strings.HasSuffix(path, ".sha256"),
			strings.HasSuffix(path, ".whl"),
			strings.HasPrefix(path, "dist/opensysml-[0-9]"),
			strings.HasPrefix(path, "dist/jupyter_opensysml_kernel-[0-9]"):
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
		"dist/jupyter/*.sha256",
		"dist/opensysml-*-py3-none-any.whl",
		"dist/opensysml-[0-9]*.tar.gz",
		"dist/jupyter_opensysml_kernel-*-py3-none-*.whl",
		"dist/jupyter_opensysml_kernel-[0-9]*.tar.gz",
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

// TestSharedTableIsPinnedAfterTheRelease holds the release workflow to the
// pull request that brings the committed table up to date: opened against
// develop once the GitHub release exists, stamping the shared table (so every
// client copy is synced) from the manifest the release signed.
func TestSharedTableIsPinnedAfterTheRelease(t *testing.T) {
	config := loadCircleConfig(t)
	job, ok := config.Jobs["pin-release-digests"]
	if !ok {
		t.Fatal("no pin-release-digests job")
	}
	if !stepHasBareStep(job.Steps, "attach_workspace") {
		t.Error("pin-release-digests does not attach the workspace carrying dist/SHA256SUMS.txt")
	}
	steps := runSteps(t, job.Steps)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		"--from-manifest dist/SHA256SUMS.txt",
	)
	if stamp < 0 {
		t.Fatal("pin-release-digests does not stamp the release from the manifest")
	}
	if strings.Contains(steps[stamp].Command, "--table") {
		t.Error("the stamp names a package-local table; the shared table, and every copy synced from it, is the target")
	}
	for _, want := range []string{"origin/develop", "scripts/sync-release-digests.py --check"} {
		if !strings.Contains(steps[stamp].Command, want) {
			t.Errorf("the stamp step does not use %q", want)
		}
	}
	if stepIndex(steps, "chore/pin-") < 0 {
		t.Error("pin-release-digests does not name its branch chore/pin-<tag>")
	}
	open := stepIndex(steps, "push --force", "refs/heads/", "/pulls", `"base": "develop"`)
	switch {
	case open < 0:
		t.Error("pin-release-digests does not push the branch and open a pull request against develop")
	case open < stamp:
		t.Errorf("the pull request (step %d) is opened before the stamp (step %d)", open, stamp)
	}

	release := config.workflow(t, "release")
	const name = "Pin the release in the shared digest table"
	requireAll(t, name, requiresOf(t, release, name), "Publish GitHub release")
}
