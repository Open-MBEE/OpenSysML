package hygiene

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

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
	Executors map[string]struct {
		Docker []struct{ Image string } `yaml:"docker"`
	} `yaml:"executors"`
	Jobs map[string]struct {
		Executor string      `yaml:"executor"`
		Steps    []yaml.Node `yaml:"steps"`
	} `yaml:"jobs"`
	Commands map[string]struct {
		Steps []yaml.Node `yaml:"steps"`
	} `yaml:"commands"`
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
	Name            string
	Command         string
	NoOutputTimeout string `yaml:"no_output_timeout"`
}

// jobRunsOnImage reports whether the named job's executor is backed by the
// Docker image.
func (c circleConfig) jobRunsOnImage(t *testing.T, job, image string) bool {
	t.Helper()
	executor, ok := c.Executors[c.Jobs[job].Executor]
	if !ok {
		t.Fatalf("%s names no executor of the config's", job)
	}
	for _, docker := range executor.Docker {
		if docker.Image == image {
			return true
		}
	}
	return false
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
			if value.Kind == yaml.ScalarNode {
				runs = append(runs, runStep{Command: value.Value})
				continue
			}
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
	// The JupyterLab extension is built once, before the sdist and the wheels,
	// and every distribution is opened and held to carry that one build.
	extension := strings.Index(string(script), "make jupyterlab-build")
	sdist := strings.Index(string(script), "-m build --sdist")
	switch {
	case extension < 0:
		t.Error("build-jupyter-kernel-dist.sh does not build the JupyterLab extension with make jupyterlab-build")
	case extension > sdist:
		t.Error("build-jupyter-kernel-dist.sh builds the JupyterLab extension after the sdist, which ships without it")
	}
	for _, want := range []string{"jupyter_opensysml_kernel.labextension import SHARED_DATA", `"package.json"`, `"install.json"`, "static/remoteEntry.", "ship the same labextension"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("build-jupyter-kernel-dist.sh does not check every distribution carries the labextension (%q)", want)
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
	if stepIndex(binarySteps, `cd dist/jupyter && for f in sysml-jupyter-kernel-*; do sha256sum "$f" > "$f.sha256"; done`) < 0 {
		t.Error("build-release-binaries does not write the kernel binaries' .sha256 sidecars, which the kernel distribution build checks the binaries it bundles against")
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
	// The extension is built with Node and `jupyter labextension build`, so
	// the job needs both before the distributions are built, and the
	// installed wheel is checked to have put the extension where JupyterLab
	// loads federated extensions from.
	if !config.jobRunsOnImage(t, "build-jupyter-kernel-package", "cimg/python:3.11-node") {
		t.Error("build-jupyter-kernel-package does not run on an image with Node, which make jupyterlab-build needs")
	}
	tooling := stepIndex(kernelSteps, "pip install", "jupyterlab==")
	switch {
	case tooling < 0:
		t.Error("build-jupyter-kernel-package does not install jupyterlab, which provides `jupyter labextension build`")
	case tooling > wheel:
		t.Error("build-jupyter-kernel-package installs jupyterlab after the distributions are built")
	}
	if stepIndex(kernelSteps, `"share", "jupyter", "labextensions", "jupyterlab-opensysml"`, `["jupyterlab"]["_build"]["load"]`, `"install.json"`) < 0 {
		t.Error("build-jupyter-kernel-package does not check the installed wheel registered a loadable jupyterlab-opensysml labextension")
	}

	releaseJob, ok := config.Jobs["build-release"]
	if !ok {
		t.Fatal("no build-release job")
	}
	releaseSteps := runSteps(t, releaseJob.Steps)
	manifest := stepIndex(releaseSteps, "> SHA256SUMS.txt", "jupyter_opensysml_kernel-*.whl", "cd jupyter && sha256sum --check --strict sysml-jupyter-kernel-*.sha256")
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
	// it, the SBOMs, the wasm sidecars, and the Python distributions it copies
	// in. The binaries' sidecars are written beside them by build-release-binaries.
	own := func(path string) bool {
		if !strings.HasPrefix(path, "dist/") {
			return false
		}
		switch {
		case strings.Contains(path, "SHA256SUMS.txt"),
			strings.Contains(path, "provenance.intoto.json"),
			path == "dist/sbom",
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
		"dist/sbom",
		"dist/SHA256SUMS.txt.bundle",
		"dist/provenance.intoto.json.bundle",
		"dist/wasm/*.sha256",
		"dist/opensysml-*-py3-none-any.whl",
		"dist/opensysml-[0-9]*.tar.gz",
		"dist/jupyter_opensysml_kernel-*-py3-none-*.whl",
		"dist/jupyter_opensysml_kernel-[0-9]*.tar.gz",
	)
}

// TestNodePackageIsStampedWithItsReleaseDigests holds the npm release to the
// same contract as the Python wheel: build-npm-package stamps the digests of
// the binaries in dist/grpc into the package's table before it packs the
// client, asserts the packed tarball pins all five assets for the tag, and
// persists the tarballs for build-release to list in the manifest it signs;
// publish-npm then stamps the same digests for its suite, verifies the
// tarballs against that manifest, and publishes them rather than packing again.
func TestNodePackageIsStampedWithItsReleaseDigests(t *testing.T) {
	config := loadCircleConfig(t)
	build, ok := config.Jobs["build-npm-package"]
	if !ok {
		t.Fatal("no build-npm-package job")
	}
	if !stepHasBareStep(build.Steps, "attach_workspace") {
		t.Error("build-npm-package does not attach the workspace that carries dist/grpc")
	}
	stampFragments := []string{
		"client/python/scripts/pin_release_checksums.py",
		`--version "${CIRCLE_TAG}"`,
		"--from-binaries dist/grpc",
		"--table client/node/release-digests.json",
	}
	buildSteps := runSteps(t, build.Steps)
	stamp := stepIndex(buildSteps, stampFragments...)
	pack := stepIndex(buildSteps, "cd client/node && npm pack --pack-destination")
	assert := stepIndex(buildSteps, "package/release-digests.json", "tarfile", "hashlib")
	switch {
	case stamp < 0:
		t.Error("build-npm-package does not stamp the release's digests from dist/grpc into the package's table")
	case pack < 0:
		t.Error("build-npm-package does not pack the client into a tarball")
	case stamp > pack:
		t.Errorf("the digest stamp (step %d) runs after npm pack (step %d), so the tarball ships without it", stamp, pack)
	}
	switch {
	case assert < 0:
		t.Fatal("build-npm-package does not assert the packed tarball pins the release's service digests")
	case assert != pack:
		t.Errorf("the tarball assertion (step %d) must run in the step that packs it (step %d)", assert, pack)
	}
	for _, asset := range serviceAssets {
		if !strings.Contains(buildSteps[assert].Command, asset) {
			t.Errorf("the tarball assertion does not require %s", asset)
		}
	}
	requireAll(t, "build-npm-package persists", persistedPaths(t, build.Steps), "dist/npm")

	npm, ok := config.Jobs["publish-npm"]
	if !ok {
		t.Fatal("no publish-npm job")
	}
	if !stepHasBareStep(npm.Steps, "attach_workspace") {
		t.Error("publish-npm does not attach the workspace that carries dist/grpc and dist/npm")
	}
	steps := runSteps(t, npm.Steps)
	stamp = stepIndex(steps, stampFragments...)
	test := stepIndex(steps, "npm test")
	verify := stepIndex(steps, "cd dist/npm && grep -E ' openmbee-[^ ]*\\.tgz$' ../SHA256SUMS.txt | sha256sum --check --strict")
	publish := stepIndex(steps, "npm publish", "--access public", "NPM_TARBALL")
	switch {
	case stamp < 0:
		t.Error("publish-npm does not stamp the release's digests from dist/grpc into the package's table")
	case test < 0:
		t.Error("publish-npm does not run the Node suite")
	case stamp > test:
		t.Errorf("the digest stamp (step %d) runs after npm test (step %d), so the suite cannot check it", stamp, test)
	}
	if test >= 0 && !strings.Contains(steps[test].Command, `OPENSYSML_EXPECT_PINNED_RELEASE="${CIRCLE_TAG}"`) {
		t.Error("publish-npm runs the Node suite without OPENSYSML_EXPECT_PINNED_RELEASE, so the suite does not assert the stamped pin")
	}
	switch {
	case verify < 0:
		t.Error("publish-npm does not verify the npm tarballs against the signed manifest")
	case publish < 0:
		t.Error("publish-npm does not publish the verified tarball; publishing the directory would pack it again unverified")
	case publish < verify:
		t.Errorf("npm publish (step %d) runs before the tarballs are verified (step %d)", publish, verify)
	}
	if stepIndex(steps, "npm pack --pack-destination") >= 0 {
		t.Error("publish-npm packs a tarball instead of publishing the ones the signed manifest lists")
	}
	if stepIndex(steps, "cd client/node", "npm pack --dry-run") >= 0 {
		t.Error("publish-npm packs the client a second time instead of using the verified tarball")
	}

	release := config.workflow(t, "release")
	requireAll(t, "Build npm packages", requiresOf(t, release, "Build npm packages"), "Build release binaries")
	requireAll(t, "Build release artifacts", requiresOf(t, release, "Build release artifacts"), "Build npm packages")
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

// TestMavenPublishStepOutlivesThePluginWait holds the publish-maven deploy step's no_output_timeout
// above the plugin's silent waitUntil=published poll (waitMaxTime plus one waitPollingInterval).
func TestMavenPublishStepOutlivesThePluginWait(t *testing.T) {
	pom, err := os.ReadFile("../../client/java/pom.xml")
	if err != nil {
		t.Fatalf("read client/java/pom.xml: %v", err)
	}
	plugin := regexp.MustCompile(`(?s)<artifactId>central-publishing-maven-plugin</artifactId>.*?</plugin>`).Find(pom)
	if plugin == nil {
		t.Fatal("client/java/pom.xml does not configure central-publishing-maven-plugin")
	}
	setting := func(name string) string {
		m := regexp.MustCompile(`<` + name + `>([^<]*)</` + name + `>`).FindSubmatch(plugin)
		if m == nil {
			t.Fatalf("central-publishing-maven-plugin does not set %s; the publish wait is unbounded by the pom", name)
		}
		return strings.TrimSpace(string(m[1]))
	}
	if got := setting("autoPublish"); got != "true" {
		t.Errorf("autoPublish is %q; the release job must publish the validated deployment itself", got)
	}
	if got := setting("waitUntil"); got != "published" {
		t.Errorf("waitUntil is %q; a green job must mean the version is on Central", got)
	}
	seconds := func(name string) int {
		value := setting(name)
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			t.Fatalf("%s is %q, not a positive number of seconds", name, value)
		}
		return n
	}
	waitMax, poll := seconds("waitMaxTime"), seconds("waitPollingInterval")
	// The plugin raises lower values to these floors with a warning, so a pom
	// stating less would not describe the wait the job actually makes.
	if waitMax < 1800 {
		t.Errorf("waitMaxTime is %d s; the plugin's floor is 1800 s and it silently raises a lower value", waitMax)
	}
	if poll < 5 {
		t.Errorf("waitPollingInterval is %d s; the plugin's floor is 5 s and it silently raises a lower value", poll)
	}

	config := loadCircleConfig(t)
	maven, ok := config.Jobs["publish-maven"]
	if !ok {
		t.Fatal("no publish-maven job")
	}
	steps := runSteps(t, maven.Steps)
	deploy := stepIndex(steps, "mvn -B -f client/java/pom.xml", "deploy -pl :opensysml -am")
	if deploy < 0 {
		t.Fatal("publish-maven does not deploy")
	}
	if steps[deploy].NoOutputTimeout == "" {
		t.Fatalf("the %q step has no no_output_timeout, so CircleCI's 10m default cuts the plugin's %d s wait", steps[deploy].Name, waitMax)
	}
	timeout, err := time.ParseDuration(steps[deploy].NoOutputTimeout)
	if err != nil {
		t.Fatalf("the %q step's no_output_timeout %q is not a duration: %v", steps[deploy].Name, steps[deploy].NoOutputTimeout, err)
	}
	bound := time.Duration(waitMax+poll) * time.Second
	if timeout <= bound {
		t.Errorf("the %q step's no_output_timeout %s does not outlive the plugin's wait of waitMaxTime+waitPollingInterval = %s", steps[deploy].Name, timeout, bound)
	}
}

// TestSharedTableIsPinnedAfterTheRelease holds the release workflow to the
// pull requests that bring the committed table up to date: one against develop
// and one against main, opened once the GitHub release exists, each stamping
// the shared table (so every client copy is synced) from the manifest the
// release signed. main's head is the tagged commit, which cannot hold its own
// pin, so without its pull request main's build-test stays red until the next
// release.
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
	for _, want := range []string{"git fetch --no-tags origin develop main", "stamp develop", "stamp main", "scripts/sync-release-digests.py --check"} {
		if !strings.Contains(steps[stamp].Command, want) {
			t.Errorf("the stamp step does not use %q", want)
		}
	}
	if stepIndex(steps, "chore/pin-") < 0 {
		t.Error("pin-release-digests does not name its develop branch chore/pin-<tag>")
	}
	if stepIndex(steps, "hotfix/pin-") < 0 {
		t.Error("pin-release-digests does not name its main branch hotfix/pin-<tag>; only hotfix/ branches target main")
	}
	open := stepIndex(steps, "push --force", "refs/heads/", "/pulls", `"base": base`)
	switch {
	case open < 0:
		t.Error("pin-release-digests does not push the branches and open a pull request against each base")
	case open < stamp:
		t.Errorf("the pull request (step %d) is opened before the stamp (step %d)", open, stamp)
	}

	release := config.workflow(t, "release")
	const name = "Pin the release in the shared digest table"
	requireAll(t, name, requiresOf(t, release, name), "Publish GitHub release")
}

// TestNpmRunnerIsPublishedWithTheClient checks the @openmbee/sysml-mcp
// contract: build-release writes the sidecars its platform packages are built
// against, publish-npm builds those packages from dist/mcp, packs the shim and
// proves the packed shim serves the protocol before anything is published, and
// publishes the platform packages before the shim that pins them, and the shim
// before its alias @openmbee/opensysml-mcp, which the generator derives from it.
func TestNpmRunnerIsPublishedWithTheClient(t *testing.T) {
	config := loadCircleConfig(t)
	release, ok := config.Jobs["build-release-binaries"]
	if !ok {
		t.Fatal("no build-release-binaries job")
	}
	if stepIndex(runSteps(t, release.Steps), "cd dist/mcp", `for f in sysml-mcp-*; do sha256sum "$f" > "$f.sha256"; done`) < 0 {
		t.Error("build-release-binaries writes no .sha256 sidecar beside the raw sysml-mcp binaries, so the platform packages cannot be verified")
	}

	pack, ok := config.Jobs["build-npm-package"]
	if !ok {
		t.Fatal("no build-npm-package job")
	}
	packSteps := runSteps(t, pack.Steps)
	packed := stepIndex(packSteps,
		"cd packaging/npm/sysml-mcp && npm pack --pack-destination ../../../dist/npm",
		"npm run platform-packages -- --mcp ../../dist/mcp",
		"cd client/node/packages/opensysml-mcp && npm pack --pack-destination ../../../../dist/npm",
	)
	if packed < 0 {
		t.Error("build-npm-package does not pack @openmbee/sysml-mcp and the alias the generator derives from it into dist/npm")
	}

	npm, ok := config.Jobs["publish-npm"]
	if !ok {
		t.Fatal("no publish-npm job")
	}
	steps := runSteps(t, npm.Steps)
	refuse := stepIndex(steps, "packaging/npm/sysml-mcp/package.json", "npm view", "-ne 14", "const alias = '"+mcpAliasPackage+"'")
	build := stepIndex(steps, "npm run platform-packages", "--mcp ../../dist/mcp")
	prove := stepIndex(steps, "openmbee-sysml-mcp-${NPM_VERSION}.tgz", "NPM_MCP_TARBALL", "scripts/npm-mcp-test.sh")
	proveAlias := stepIndex(steps, "openmbee-opensysml-mcp-${NPM_VERSION}.tgz", "NPM_MCP_ALIAS_TARBALL", `pkg.mcpName !== undefined`, `pkg.bin["opensysml-mcp"] === shim`)
	whoami := stepIndex(steps, "npm whoami")
	platforms := stepIndex(steps, "npm publish", "client/node/packages/sysml-mcp-*")
	shim := stepIndex(steps, "npm publish", `"$NPM_MCP_TARBALL"`)
	rehearsal := stepIndex(steps, "npm pack", "--dry-run", "client/node/packages/sysml-mcp-*")
	switch {
	case refuse < 0:
		t.Error("publish-npm does not refuse a published version of @openmbee/sysml-mcp, its five platform packages and @openmbee/opensysml-mcp (14 packages in all)")
	case build < 0:
		t.Error("publish-npm does not build the sysml-mcp platform packages from dist/mcp")
	case prove < 0:
		t.Error("publish-npm does not take the packed @openmbee/sysml-mcp and prove the shim serves the protocol")
	case prove < build:
		t.Errorf("the shim is packed and proved (step %d) before its platform packages are built (step %d)", prove, build)
	case proveAlias < 0:
		t.Error("publish-npm does not check the packed @openmbee/opensysml-mcp for its version, pins, both commands and the absent mcpName")
	case proveAlias < prove:
		t.Errorf("the alias is packed (step %d) before the shim it is derived from is proved (step %d)", proveAlias, prove)
	case whoami < proveAlias:
		t.Errorf("npm whoami (step %d) runs before the alias is proved (step %d); everything that can fail runs first", whoami, proveAlias)
	case platforms < 0:
		t.Error("publish-npm does not publish the sysml-mcp platform packages")
	case shim < 0:
		t.Error("publish-npm does not publish the verified @openmbee/sysml-mcp tarball")
	case shim < platforms:
		t.Errorf("@openmbee/sysml-mcp (step %d) is published before the platform packages it pins (step %d)", shim, platforms)
	case rehearsal < 0:
		t.Error("the release rehearsal does not pack the sysml-mcp platform packages")
	}
	if platforms >= 0 && shim >= 0 && stepIndex(steps[platforms:shim+1], `"$NPM_TARBALL"`) < 0 {
		t.Error("@openmbee/opensysml is not published between the platform packages and @openmbee/sysml-mcp")
	}
	if shim >= 0 {
		command := steps[shim].Command
		runner, alias := strings.Index(command, `"$NPM_MCP_TARBALL"`), strings.Index(command, `"$NPM_MCP_ALIAS_TARBALL"`)
		switch {
		case alias < 0:
			t.Error("publish-npm does not publish the verified @openmbee/opensysml-mcp tarball")
		case alias < runner:
			t.Error("@openmbee/opensysml-mcp is published before @openmbee/sysml-mcp, the runner it is the other name of")
		}
	}
}

// TestReleaseSBOMsAreSignedWithTheManifest holds build-release to writing an
// SPDX SBOM per asset before the checksum manifest, with a pinned syft, and to
// listing the SBOMs in that manifest, so the cosign signature and the
// provenance statement cover them; publish-github-release must then upload them.
func TestReleaseSBOMsAreSignedWithTheManifest(t *testing.T) {
	config := loadCircleConfig(t)
	release, ok := config.Jobs["build-release"]
	if !ok {
		t.Fatal("no build-release job")
	}
	releaseSteps := runSteps(t, release.Steps)

	install := stepIndex(releaseSteps, "anchore/syft/releases/download/v", "sha256sum --check --strict", "install -m 0755 /tmp/syft")
	manifest := stepIndex(releaseSteps, "> SHA256SUMS.txt")
	sign := stepIndex(releaseSteps, "cosign sign-blob SHA256SUMS.txt")
	attest := stepIndex(releaseSteps, "cosign verify-blob-attestation", ".spdx.json")
	if install < 0 {
		t.Fatal("build-release does not install a pinned, checksummed syft")
	}
	if manifest < 0 || sign < 0 {
		t.Fatal("build-release does not write and sign SHA256SUMS.txt")
	}
	if install > manifest {
		t.Error("syft is installed after the manifest step that needs it")
	}
	if attest < 0 || attest < sign {
		t.Error("build-release does not verify the provenance against an SBOM")
	}

	command := releaseSteps[manifest].Command
	writes := strings.Index(command, "release-sbom.py --version \"${CIRCLE_TAG}\" --out sbom")
	manifestWrite := strings.Index(command, "> SHA256SUMS.txt")
	lists := strings.Index(command, "cd sbom && sha256sum ./*.spdx.json | sed 's|\\./||' >> ../SHA256SUMS.txt")
	switch {
	case writes < 0:
		t.Error("the manifest step does not write the SBOMs with scripts/release-sbom.py")
	case writes > manifestWrite:
		t.Error("the SBOMs are written after the manifest they must be listed in")
	case lists < 0:
		t.Error("the manifest step does not list the SBOMs in SHA256SUMS.txt")
	}
	if !strings.Contains(command, "find grpc jupyter -type f ! -name '*.sha256' -exec") {
		t.Error("the raw service and kernel binaries get no SBOM")
	}
	wasm := strings.Index(command, "--out sbom wasm/sysml-wasm.wasm wasm/sysml-prod.wasm.gz")
	client := strings.Index(command, "--npm-lock ../client/node/package-lock.json npm/openmbee-opensysml-[0-9]*.tgz")
	runner := strings.Index(command, "--npm-lock ../packaging/npm/sysml-mcp/package-lock.json npm/openmbee-sysml-mcp-[0-9]*.tgz npm/openmbee-opensysml-mcp-[0-9]*.tgz")
	tarballs := strings.Index(command, "cd npm && sha256sum ./*.tgz | sed 's|\\./||' >> ../SHA256SUMS.txt")
	switch {
	case wasm < 0:
		t.Error("the WebAssembly modules get no SBOM")
	case wasm > manifestWrite:
		t.Error("the WebAssembly SBOMs are written after the manifest they must be listed in")
	}
	switch {
	case client < 0 || runner < 0:
		t.Error("the npm tarballs get no SBOM resolved through their lockfiles")
	case client > manifestWrite || runner > manifestWrite:
		t.Error("the npm SBOMs are written after the manifest they must be listed in")
	case tarballs < 0:
		t.Error("the npm tarballs themselves are not listed in SHA256SUMS.txt")
	}
	if lists >= 0 && tarballs > lists {
		t.Error("the npm tarballs are listed after the SBOMs; the manifest lists assets before their SBOMs")
	}
	// The SBOM of a binary shares its name prefix, so the wheel-pin checks must
	// read the manifest by exact binary name rather than by prefix.
	for _, check := range []string{"opensysml/release-digests.json", "jupyter_opensysml_kernel/release-digests.json"} {
		index := stepIndex(releaseSteps, check, "nothing was signed")
		if index < 0 || !containsAll(releaseSteps[index].Command, "fields[1] in assets", `"windows-amd64.exe")}`) {
			t.Errorf("the %s check does not select the five binaries by exact name from the manifest", check)
		}
	}

	github, ok := config.Jobs["publish-github-release"]
	if !ok {
		t.Fatal("no publish-github-release job")
	}
	githubSteps := runSteps(t, github.Steps)
	move := stepIndex(githubSteps, "mv dist/sbom/* dist/release/", "mv dist/npm/*.tgz dist/release/")
	upload := stepIndex(githubSteps, "ghr ")
	if move < 0 || upload < 0 || move > upload {
		t.Error("publish-github-release does not move the SBOMs and the npm tarballs into dist/release before ghr uploads it")
	}

	static, ok := config.Jobs["go-static"]
	if !ok {
		t.Fatal("no go-static job")
	}
	if stepIndex(runSteps(t, static.Steps), "python3 scripts/release-sbom-test.py") < 0 {
		t.Error("go-static does not run scripts/release-sbom-test.py")
	}
}

// TestWindowsInstallerSBOMIsDerivedFromItsInputs holds release-windows.yml to
// writing the MSI's SBOM where its inputs are: the build job, from the Go
// executables it built and the Z3 release packaging/msi/z3.pin names, with the
// syft the CircleCI release pins; the msi and publish-signed jobs then list the
// SBOM beside the installer's digest, under the installer's own name.
func TestWindowsInstallerSBOMIsDerivedFromItsInputs(t *testing.T) {
	windows := readRepoFile(t, ".github/workflows/release-windows.yml")
	circle := readRepoFile(t, ".circleci/config.yml")

	// The build job writes the unsigned installer's SBOM from the executables
	// it built; the sign job writes the signed installer's from the executables
	// SignPath returned, whose bytes differ. Both pin the syft CircleCI pins.
	syftURL := regexp.MustCompile(`https://github\.com/anchore/syft/releases/download/v[0-9.]+/syft_[0-9.]+_linux_amd64\.tar\.gz`)
	syftDigest := regexp.MustCompile(`echo "[0-9a-f]{64}  /tmp/syft\.tar\.gz" \| sha256sum --check --strict`)
	for _, pin := range []*regexp.Regexp{syftURL, syftDigest} {
		circlePins, windowsPins := pin.FindAllString(circle, -1), pin.FindAllString(windows, -1)
		if len(circlePins) != 1 || len(windowsPins) != 2 {
			t.Fatalf("syft is pinned %d times in .circleci/config.yml and %d times in release-windows.yml; expected once and twice (%s)", len(circlePins), len(windowsPins), pin)
		}
		for _, windowsPin := range windowsPins {
			if circlePins[0] != windowsPin {
				t.Errorf("release-windows.yml pins syft as %q, .circleci/config.yml as %q", windowsPin, circlePins[0])
			}
		}
	}

	requireFragments(t, ".github/workflows/release-windows.yml", windows,
		"source packaging/msi/z3.pin",
		`echo "${Z3_SHA256}  /tmp/${Z3_ZIP}" | sha256sum --check --strict`,
		`python3 scripts/release-sbom.py --version "$VERSION" --out dist/msi-sbom`,
		`--msi-inputs dist/unsigned --z3-pin packaging/msi/z3.pin --z3-zip "/tmp/${Z3_ZIP}"`,
		`--msi-inputs dist/signed --msi-signed --z3-pin packaging/msi/z3.pin --z3-zip "/tmp/${Z3_ZIP}"`,
		"name: ${{ env.MSI_SBOM_ARTIFACT }}",
		"name: ${{ env.SIGNED_MSI_SBOM_ARTIFACT }}",
	)
	for _, job := range []struct{ name, inputs string }{
		{"build", "--msi-inputs dist/unsigned "},
		{"sign", "--msi-inputs dist/signed --msi-signed "},
	} {
		section := windows[strings.Index(windows, "\n  "+job.name+":\n"):]
		if next := nextJobHeader.FindStringIndex(section[1:]); next != nil {
			section = section[:1+next[0]]
		}
		if !strings.Contains(section, job.inputs) {
			t.Errorf("the %s job does not write the installer SBOM from %s", job.name, strings.TrimSpace(job.inputs))
		}
	}
	for _, job := range []struct{ name, manifest string }{
		{"msi", `sha256sum ./*.msi ./*.spdx.json | sed 's|\./||' | tee SHA256SUMS-windows-msi.txt`},
		{"publish-signed", `sha256sum ./*.zip ./*.msi ./*.spdx.json sysml-grpc-windows-amd64-signed.exe | sed 's|\./||' > SHA256SUMS-windows-signed.txt`},
	} {
		section := windows[strings.Index(windows, "\n  "+job.name+":\n"):]
		if next := nextJobHeader.FindStringIndex(section[1:]); next != nil {
			section = section[:1+next[0]]
		}
		requireFragments(t, ".github/workflows/release-windows.yml ("+job.name+")", section,
			".spdx.json\"", // the SBOM must exist under the installer's name before it is listed
			job.manifest,
		)
		if strings.Index(section, ".spdx.json\"") > strings.Index(section, job.manifest) {
			t.Errorf("%s lists the SBOM before checking it is named after the installer", job.name)
		}
	}
}
