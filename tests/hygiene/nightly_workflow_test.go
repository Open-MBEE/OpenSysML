package hygiene

import (
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The nightly workflow publishes development snapshots of the clients beside the
// binaries. These tests hold it to what docs/project/nightly.md promises: every
// release it creates is a prerelease never marked latest, the wheel is stamped
// with the night's service digests before it is built and checked after, the
// registries are reached through trusted publishing with no stored token, npm
// gets a dist-tag that is not latest, and old nights are pruned by age.

type actionsWorkflow struct {
	RunName     string                `yaml:"run-name"`
	Permissions map[string]string     `yaml:"permissions"`
	Env         map[string]string     `yaml:"env"`
	Jobs        map[string]actionsJob `yaml:"jobs"`
}

type actionsJob struct {
	Needs       yaml.Node         `yaml:"needs"`
	If          string            `yaml:"if"`
	Environment string            `yaml:"environment"`
	Permissions map[string]string `yaml:"permissions"`
	Outputs     map[string]string `yaml:"outputs"`
	Steps       []actionsStep     `yaml:"steps"`
}

type actionsStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	If   string            `yaml:"if"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
}

func (j actionsJob) needs() []string {
	var one string
	if err := j.Needs.Decode(&one); err == nil && one != "" {
		return []string{one}
	}
	var many []string
	_ = j.Needs.Decode(&many)
	return many
}

func (j actionsJob) runSteps() []runStep {
	var runs []runStep
	for _, step := range j.Steps {
		if step.Run != "" {
			runs = append(runs, runStep{Name: step.Name, Command: step.Run})
		}
	}
	return runs
}

func loadNightlyWorkflow(t *testing.T) (actionsWorkflow, string) {
	t.Helper()
	raw, err := os.ReadFile("../../.github/workflows/nightly.yml")
	if err != nil {
		t.Fatalf("read .github/workflows/nightly.yml: %v", err)
	}
	var workflow actionsWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse .github/workflows/nightly.yml: %v", err)
	}
	return workflow, string(raw)
}

func nightlyJob(t *testing.T, workflow actionsWorkflow, name string) actionsJob {
	t.Helper()
	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatalf("nightly.yml has no %s job", name)
	}
	return job
}

func TestNightlyBuildsOnlyCommitsThatCarryItsScripts(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	for _, key := range []string{"BUILD_SCRIPT", "KERNEL_DIST_SCRIPT", "SNAPSHOT_SCRIPT", "MCP_RUNNER_TEST"} {
		script, ok := workflow.Env[key]
		if !ok {
			t.Fatalf("nightly.yml declares no %s", key)
		}
		if _, err := os.Stat("../../" + script); err != nil {
			t.Errorf("%s names %s, which the tree does not have: %v", key, script, err)
		}
	}
	selectJob := nightlyJob(t, workflow, "select")
	if stepIndex(selectJob.runSteps(), `"$BUILD_SCRIPT" "$KERNEL_DIST_SCRIPT" "$SNAPSHOT_SCRIPT" "$MCP_RUNNER_TEST"`, "git cat-file -e") < 0 {
		t.Error("the select job does not skip a green commit that lacks the build, kernel-distribution, snapshot-version or npm-runner script, which it could not build")
	}
}

func TestNightlyReleasesAreAlwaysPrereleasesNeverLatest(t *testing.T) {
	workflow, raw := loadNightlyWorkflow(t)
	if workflow.Permissions["contents"] != "read" {
		t.Errorf("the workflow's default contents permission is %q, not read", workflow.Permissions["contents"])
	}
	publish := nightlyJob(t, workflow, "publish")
	if publish.Permissions["contents"] != "write" {
		t.Errorf("the publish job's contents permission is %q; it creates releases", publish.Permissions["contents"])
	}
	alias := nightlyJob(t, workflow, "alias")
	if alias.Permissions["contents"] != "write" {
		t.Errorf("the alias job's contents permission is %q; it moves a tag and creates a release", alias.Permissions["contents"])
	}
	creates := 0
	for _, job := range workflow.Jobs {
		for _, step := range job.runSteps() {
			for _, line := range strings.Split(step.Command, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "gh release create") {
					creates++
				}
			}
		}
	}
	if creates != 2 {
		t.Errorf("nightly.yml creates releases %d times; the per-night release and the alias make 2", creates)
	}
	if strings.Contains(raw, "gh release edit") {
		t.Error("nightly.yml edits a release, which could mark one latest after creation")
	}
	// Each release is created in its own job, so a failure after the per-night release is
	// published is finished by re-running the failed jobs rather than by rebuilding it.
	night := stepIndex(publish.runSteps(), `gh release create "$VERSION"`, "--prerelease", "--latest=false")
	moved := stepIndex(alias.runSteps(), `gh release create "$TAG"`, "--prerelease", "--latest=false")
	if night < 0 {
		t.Fatal("the publish job does not create the per-night release as a prerelease that is not latest")
	}
	if moved < 0 {
		t.Fatal("the alias job does not create the alias as a prerelease that is not latest")
	}
	if !containsAll(publish.runSteps()[night].Command, "dist/*.whl") {
		t.Error("the per-night release does not carry the Python wheel beside the binaries")
	}
	if !containsAll(alias.runSteps()[moved].Command, `gh release download "$VERSION"`, `"${#assets[@]}" -ne "$expected"`) {
		t.Error("the alias is not recreated from the per-night release's own assets, checked complete")
	}
	if needs := alias.needs(); !slices.Contains(needs, "publish") {
		t.Errorf("the alias job needs %v, not the publish job whose release it copies", needs)
	}
	uploaded, released := -1, -1
	for i, step := range publish.Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			uploaded = i
			if step.With["overwrite"] != "true" {
				t.Errorf("step %q does not overwrite the artifact a re-run of the job already uploaded, so the re-run fails on it", step.Name)
			}
		}
		if strings.Contains(step.Run, `gh release create "$VERSION"`) {
			released = i
		}
	}
	if uploaded < 0 || released < uploaded {
		t.Error("the per-night release is published before the distributions are kept, so a failed upload leaves a night the registries cannot be re-run for")
	}
}

func TestNightlyPythonSnapshotIsStampedBeforeItIsBuiltAndCheckedAfter(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	steps := nightlyJob(t, workflow, "publish").runSteps()

	install := stepIndex(steps, "cd client/node", "npm install --ignore-scripts")
	version := stepIndex(steps, `"$SNAPSHOT_SCRIPT"`, "--stamp")
	binaries := stepIndex(steps, `"$BUILD_SCRIPT" dist`)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "$VERSION"`,
		"--from-binaries dist/grpc",
		"--table client/python/opensysml/release-digests.json",
	)
	build := stepIndex(steps, "python -m build")
	verify := stepIndex(steps, "release-digests.json", "zipfile", "tarfile", "pinned_digest", "built_against_releases")
	manifest := stepIndex(steps, `sha256sum "opensysml-${PYPI_VERSION}-py3-none-any.whl"`)
	sign := stepIndex(steps, "cosign sign-blob SHA256SUMS.txt")
	for name, index := range map[string]int{
		"client install": install, "version stamp": version, "binary build": binaries,
		"digest stamp": stamp, "python -m build": build, "distribution check": verify,
		"manifest entry": manifest, "manifest signing": sign,
	} {
		if index < 0 {
			t.Errorf("the publish job has no %s step", name)
		}
	}
	if t.Failed() {
		return
	}
	orders := []struct {
		what   string
		before int
		after  int
	}{
		{"the client is installed after the manifest is stamped, which breaks the lock-file check", install, version},
		{"the versions are stamped after the binaries are built, so the binaries report another version", version, binaries},
		{"the digests are stamped before the binaries exist", binaries, stamp},
		{"the digests are stamped after python -m build, so the wheel ships without them", stamp, build},
		{"the distribution is checked before it is built", build, verify},
		{"the wheel is listed in the manifest before it is checked", verify, manifest},
		{"the manifest is signed before the wheel is listed in it", manifest, sign},
	}
	for _, order := range orders {
		if order.before > order.after {
			t.Error(order.what)
		}
	}
	for _, asset := range serviceAssets {
		if !strings.Contains(steps[verify].Command, asset) {
			t.Errorf("the distribution check does not require %s", asset)
		}
	}
}

// TestNightlyJupyterKernelSnapshotIsStampedBeforeItIsBuiltAndCheckedAfter
// holds the kernel's distribution to the same order as the client's: stamped
// from the kernels the build script wrote, built, checked against every
// kernel asset, and listed in the manifest release-pypi.yml publishes it from.
func TestNightlyJupyterKernelSnapshotIsStampedBeforeItIsBuiltAndCheckedAfter(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	steps := nightlyJob(t, workflow, "publish").runSteps()

	version := stepIndex(steps, `"$SNAPSHOT_SCRIPT"`, "--stamp", "jupyter_opensysml_kernel/_version.py")
	binaries := stepIndex(steps, `"$BUILD_SCRIPT" dist`)
	stamp := stepIndex(steps,
		"client/python/scripts/pin_release_checksums.py",
		`--version "$VERSION"`,
		"--from-binaries dist/jupyter",
		"--table client/jupyter-kernel/jupyter_opensysml_kernel/release-digests.json",
	)
	build := stepIndex(steps, `"$KERNEL_DIST_SCRIPT" dist/jupyter client/jupyter-kernel/dist`, "-py3-none-*.whl")
	labextension := stepIndex(steps, `pip install --only-binary ":all:" jupyterlab==`,
		`(cd editors/jupyterlab && npm version --no-git-tag-version "$NPM_VERSION")`,
		`"$KERNEL_DIST_SCRIPT" dist/jupyter client/jupyter-kernel/dist`,
		`labextensions/jupyterlab-opensysml/package.json`, `[ "$stamped" != "$NPM_VERSION" ]`,
	)
	verify := stepIndex(steps, "jupyter_opensysml_kernel/release-digests.json", "zipfile", "tarfile", "pinned_digest", "built_against_releases", "bundled_binary", "jupyter_client.kernelspecapp", `"jupyter_opensysml_kernel", "-version"`)
	manifest := stepIndex(steps, "sha256sum jupyter_opensysml_kernel-*.whl")
	sign := stepIndex(steps, "cosign sign-blob SHA256SUMS.txt")
	release := stepIndex(steps, "gh release create", "dist/jupyter/sysml-jupyter-kernel-*")
	for name, index := range map[string]int{
		"version stamp": version, "binary build": binaries, "digest stamp": stamp,
		"distribution build": build, "distribution check": verify, "manifest entry": manifest,
		"jupyterlab install and labextension manifest stamp before the distribution build, manifest version check after": labextension,
		"manifest signing": sign, "kernel assets in the release": release,
	} {
		if index < 0 {
			t.Errorf("the publish job has no %s step for the kernel", name)
		}
	}
	if t.Failed() {
		return
	}
	orders := []struct {
		what   string
		before int
		after  int
	}{
		{"the versions are stamped after the binaries are built", version, binaries},
		{"the kernel digests are stamped before the binaries exist", binaries, stamp},
		{"the kernel digests are stamped after the distributions are built, so they ship without them", stamp, build},
		{"the kernel distribution is checked before it is built", build, verify},
		{"the kernel wheel is listed in the manifest before it is checked", verify, manifest},
		{"the manifest is signed before the kernel wheel is listed in it", manifest, sign},
	}
	for _, order := range orders {
		if order.before > order.after {
			t.Error(order.what)
		}
	}
	for _, asset := range kernelAssets {
		if !strings.Contains(steps[verify].Command, asset) {
			t.Errorf("the kernel distribution check does not require %s", asset)
		}
	}
}

func TestNightlyClientsReachTheRegistriesThroughTrustedPublishing(t *testing.T) {
	workflow, raw := loadNightlyWorkflow(t)
	for _, forbidden := range []string{"secrets.", "TWINE_PASSWORD", "NODE_AUTH_TOKEN", "NPM_TOKEN", "PYPI_API_TOKEN"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("nightly.yml mentions %s; the registries are reached with the workflow's OIDC identity, never a stored token", forbidden)
		}
	}
	for _, name := range []string{"npm"} {
		job := nightlyJob(t, workflow, name)
		if needs := job.needs(); len(needs) != 1 || needs[0] != "publish" {
			t.Errorf("the %s job needs %v, not the publish job that built and released the snapshot", name, needs)
		}
		if job.Environment != "nightly" {
			t.Errorf("the %s job runs in environment %q; the trusted publishers are registered for `nightly`", name, job.Environment)
		}
		if job.Permissions["id-token"] != "write" {
			t.Errorf("the %s job cannot mint an OIDC token (id-token: %q)", name, job.Permissions["id-token"])
		}
		if job.Permissions["contents"] != "read" {
			t.Errorf("the %s job's contents permission is %q; publishing to a registry needs none", name, job.Permissions["contents"])
		}
	}

	npm := nightlyJob(t, workflow, "npm")
	if workflow.Env["NPM_DIST_TAG"] != "nightly" {
		t.Errorf("NPM_DIST_TAG is %q, not nightly", workflow.Env["NPM_DIST_TAG"])
	}
	publish := stepIndex(npm.runSteps(), "npm publish", `--tag "$NPM_DIST_TAG"`)
	if publish < 0 {
		t.Fatal("the npm job does not publish under $NPM_DIST_TAG")
	}
	if strings.Contains(raw, "--tag latest") || strings.Contains(raw, "dist-tag add") {
		t.Error("nightly.yml can move the latest dist-tag")
	}
	if stepIndex(npm.runSteps(), "npm view", "version > /dev/null") < 0 {
		t.Error("the npm job does not look each version up on the registry before publishing")
	}
	if stepIndex(npm.runSteps(), "dist-tags.latest", `"$latest" = "$NPM_VERSION"`) <= publish {
		t.Error("the npm job does not check, after publishing, that latest still names the stable release")
	}
}

// A night's Python distributions reach PyPI through release-pypi.yml, run on its tag and
// watched to its end, so a snapshot and a release name one publisher and a lost upload fails the night.
func TestNightlyHandsThePythonClientToTheReleaseWorkflow(t *testing.T) {
	workflow, raw := loadNightlyWorkflow(t)
	if strings.Contains(raw, "pypa/gh-action-pypi-publish") {
		t.Error("nightly.yml uploads to PyPI itself; release-pypi.yml is the one trusted publisher")
	}
	pypi := nightlyJob(t, workflow, "pypi")
	if needs := pypi.needs(); len(needs) != 1 || needs[0] != "publish" {
		t.Errorf("the pypi job needs %v, not the publish job whose release the distributions are taken from", needs)
	}
	if pypi.Permissions["actions"] != "write" {
		t.Errorf("the pypi job cannot dispatch a workflow (actions: %q)", pypi.Permissions["actions"])
	}
	if pypi.Permissions["contents"] != "read" {
		t.Errorf("the pypi job's contents permission is %q; dispatching a workflow needs none", pypi.Permissions["contents"])
	}
	if pypi.Permissions["id-token"] != "" {
		t.Error("the pypi job mints an OIDC token; release-pypi.yml is the publisher, not this job")
	}
	steps := pypi.runSteps()
	dispatch := stepIndex(steps, "gh workflow run release-pypi.yml", `--ref "$BRANCH"`, `--field "tag=$VERSION"`,
		`--created ">=$started"`, `Release to PyPI $VERSION`, "gh run watch", "--exit-status")
	if dispatch < 0 {
		t.Error("the pypi job does not dispatch release-pypi.yml on develop with the night's tag, find the run by its name and watch it to its end")
	}
	for _, step := range pypi.Steps {
		if strings.Contains(step.Run, "gh workflow run release-pypi.yml") && step.If != "" {
			t.Error("the dispatch of release-pypi.yml is conditional; every published night goes to PyPI")
		}
	}
	for _, step := range nightlyJob(t, workflow, "publish").Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") && strings.Contains(step.With["path"], "client/") {
			t.Errorf("the publish job keeps %s for a PyPI upload the nightly no longer makes", step.With["name"])
		}
	}

	releasePyPI, _ := loadReleasePyPIWorkflow(t)
	if releasePyPI.RunName != "Release to PyPI ${{ inputs.tag || github.ref_name }}" {
		t.Errorf("release-pypi.yml runs are named %q; the pypi job finds its run as `Release to PyPI <tag>`", releasePyPI.RunName)
	}
	version := stepIndex(releasePyPIJob(t, releasePyPI, "fetch").runSteps(), "nightly-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9a-f]*)",
		`gh release view "$TAG" --json assets`, `opensysml-\(.*\)-py3-none-any\.whl`, `*.dev"$date")`, `IDENTITY_REGEXP="$SNAPSHOT_IDENTITY_REGEXP"`)
	if version < 0 {
		t.Error("release-pypi.yml does not take a night's version from the wheel its release carries, under the nightly's signing identity")
	}
}

func TestNightlyPrunesPerNightReleasesByAge(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	days, err := strconv.Atoi(workflow.Env["RETENTION_DAYS"])
	if err != nil || days != 14 {
		t.Errorf("RETENTION_DAYS is %q; the snapshot page promises 14 days", workflow.Env["RETENTION_DAYS"])
	}
	steps := nightlyJob(t, workflow, "alias").runSteps()
	release := stepIndex(steps, `gh release create "$TAG"`)
	prune := stepIndex(steps, "gh release delete", "--cleanup-tag", `"$RETENTION_DAYS days ago"`, "^nightly-[0-9]{8}-[0-9a-f]+$")
	switch {
	case prune < 0:
		t.Error("the alias job does not delete per-night releases, tag and all, older than $RETENTION_DAYS by their tag form")
	case prune < release:
		t.Error("old releases are pruned before the alias moves to this night, so a failed move leaves fewer nights than promised")
	case !containsAll(steps[prune].Command, `"$tag" == "$VERSION"`):
		t.Error("pruning does not spare this night's release, which the alias was just recreated at")
	case strings.Contains(steps[prune].Command, "alias_sha"):
		t.Error("pruning spares nights by the alias's commit, which keeps every expired night forced from that commit")
	}
}

func TestNightlyNeverRebuildsAPublishedNight(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	pick := nightlyJob(t, workflow, "select").runSteps()
	if stepIndex(pick, `gh release view "$night"`, "isDraft", "publish=false") < 0 {
		t.Error("the select job does not skip a night already published; the packages on PyPI and npm pin binaries a rebuild would not reproduce")
	}
	steps := nightlyJob(t, workflow, "publish").runSteps()
	version := stepIndex(steps, `"$SNAPSHOT_SCRIPT"`, "--stamp")
	if version < 0 || !containsAll(steps[version].Command, `"$SNAPSHOT_DATE"`, `tag=$NIGHT`) {
		t.Error("the version step does not derive the night the select job found unpublished, by its date and name")
	}
	release := stepIndex(steps, `gh release create "$VERSION"`)
	if release < 0 {
		t.Fatal("no step creates the per-night release")
	}
	command := steps[release].Command
	if strings.Contains(command, `gh release delete "$VERSION" --cleanup-tag`) {
		t.Error("the release step deletes a published night, tag and all, orphaning the packages that pin it")
	}
	if !containsAll(command, `gh release view "$VERSION" --json isDraft`, "already published", "exit 1") {
		t.Error("the release step does not refuse to replace a published night")
	}
}

// TestNightlyPublishesTheNpmRunnerWithTheClient checks that a night stamps,
// builds, proves and publishes @openmbee/sysml-mcp, its five platform
// packages and its alias @openmbee/opensysml-mcp beside the client's seven:
// fourteen tarballs, platform packages before the shim that pins them, the
// shim before the alias derived from it.
func TestNightlyPublishesTheNpmRunnerWithTheClient(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	publish := nightlyJob(t, workflow, "publish").runSteps()
	if stepIndex(publish, "git diff --stat", "packaging/npm/sysml-mcp/package.json") < 0 {
		t.Error("the publish job does not show the stamped packaging/npm/sysml-mcp/package.json")
	}
	pack := stepIndex(publish, "npm run platform-packages", "--mcp ../../dist/mcp", "../../packaging/npm/sysml-mcp ./packages/opensysml-mcp", "-ne 14")
	switch {
	case pack < 0:
		t.Fatal("the publish job does not build the sysml-mcp platform packages and pack @openmbee/sysml-mcp and @openmbee/opensysml-mcp as the thirteenth and fourteenth tarballs")
	case !strings.Contains(publish[pack].Command, "scripts/npm-mcp-test.sh"):
		t.Error("the publish job does not prove the packed @openmbee/sysml-mcp serves the protocol")
	case !strings.Contains(publish[pack].Command, "openmbee-sysml-mcp-${NPM_VERSION}.tgz"):
		t.Error("the publish job does not check that @openmbee/sysml-mcp pins its platform packages at the snapshot version")
	case !containsAll(publish[pack].Command, "openmbee-opensysml-mcp-${NPM_VERSION}.tgz", `pkg.bin["opensysml-mcp"] === shim`, "pkg.mcpName !== undefined"):
		t.Error("the publish job does not check that @openmbee/opensysml-mcp pins the runner's platform packages at the snapshot version with both commands and no mcpName")
	}

	npm := nightlyJob(t, workflow, "npm").runSteps()
	step := stepIndex(npm, "npm publish", `--tag "$NPM_DIST_TAG"`, "-ne 14")
	if step < 0 {
		t.Fatal("the npm job does not publish fourteen tarballs")
	}
	command := npm[step].Command
	platforms := strings.Index(command, `openmbee-sysml-mcp-linux-x64-"$NPM_VERSION".tgz`)
	shim := strings.Index(command, `openmbee-sysml-mcp-"$NPM_VERSION".tgz`)
	alias := strings.Index(command, `openmbee-opensysml-mcp-"$NPM_VERSION".tgz`)
	switch {
	case platforms < 0:
		t.Error("the npm job does not publish the sysml-mcp platform packages")
	case shim < 0:
		t.Error("the npm job does not publish @openmbee/sysml-mcp")
	case shim < platforms:
		t.Error("the npm job publishes @openmbee/sysml-mcp before the platform packages it pins")
	case alias < 0:
		t.Error("the npm job does not publish @openmbee/opensysml-mcp")
	case alias < shim:
		t.Error("the npm job publishes @openmbee/opensysml-mcp before @openmbee/sysml-mcp, the runner it is the other name of")
	}
	if stepIndex(npm, "dist-tags.latest", "@openmbee/sysml-mcp", "@openmbee/opensysml-mcp") < 0 {
		t.Error("the npm job does not check, after publishing, that the latest of @openmbee/sysml-mcp and @openmbee/opensysml-mcp still names the stable release")
	}
}

// The clients are versioned as development builds of the release the snapshot leads
// to; the extension must be too, or a night's vsix (0.10.1-nightly-…) ranks below the
// 0.10.1 already released and VS Code offers the release as an upgrade from it.
func TestNightlyVSIXIsVersionedAsTheSnapshotCoreNotTheManifest(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	publish := nightlyJob(t, workflow, "publish")
	derive := stepIndex(publish.runSteps(), `"$SNAPSHOT_SCRIPT"`, `--stamp`)
	if derive < 0 || !containsAll(publish.runSteps()[derive].Command, `echo "$versions"`, `>> "$GITHUB_OUTPUT"`) {
		t.Fatalf("the publish job does not write the snapshot script's versions (core= included) to its outputs")
	}
	var packaged *actionsStep
	for i := range publish.Steps {
		if publish.Steps[i].Name == "Package the VS Code extension" {
			packaged = &publish.Steps[i]
		}
	}
	if packaged == nil {
		t.Fatalf("the publish job has no 'Package the VS Code extension' step")
	}
	if packaged.Env["CORE"] != "${{ steps.version.outputs.core }}" {
		t.Errorf("the vsix step reads CORE=%q, not the version step's core output", packaged.Env["CORE"])
	}
	if !containsAll(packaged.Run, `expected="${CORE}-${VERSION}"`, `"$stamped" != "$expected"`) {
		t.Errorf("the vsix step does not package and check the extension as <core>-<snapshot tag>:\n%s", packaged.Run)
	}
	if strings.Contains(packaged.Run, "editors/vscode/package.json').version") {
		t.Errorf("the vsix step still versions the extension from its manifest, which names the release already made:\n%s", packaged.Run)
	}
}

// An append whose glob matched the bundles once listed them twice and broke every nightly
// install: no append may match an asset already listed, and the manifest is checked before signing.
func TestNightlyManifestListsEachAssetOnce(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	steps := nightlyJob(t, workflow, "publish").runSteps()
	published := []string{
		"opensysml-linux-amd64.tar.gz", "opensysml-darwin-arm64.tar.gz", "opensysml-windows-amd64.zip",
		"sysml-mcp-linux-amd64.tar.gz", "sysml-mcp-windows-amd64.zip",
		"sysml-grpc-linux-amd64", "sysml-grpc-windows-amd64.exe",
		"sysml-jupyter-kernel-linux-amd64", "sysml-jupyter-kernel-windows-amd64.exe",
		"sysml-wasm.wasm", "sysml-prod.wasm.gz", "wasm_exec.js", "opensysml-sysml.vsix",
		"Magic_MDK.zip", "syson-mdk-backend.jar", "syson-mdk-frontend.tgz",
	}
	appends := 0
	last := -1
	for i, step := range steps {
		for _, line := range strings.Split(step.Command, "\n") {
			if !strings.Contains(line, ">> SHA256SUMS.txt") {
				continue
			}
			appends++
			last = i
			_, args, found := strings.Cut(line, "sha256sum ")
			if !found {
				t.Errorf("step %q appends to SHA256SUMS.txt without sha256sum: %s", step.Name, line)
				continue
			}
			args, _, _ = strings.Cut(args, " >>")
			for _, arg := range strings.Fields(args) {
				arg = strings.Trim(arg, `"'`)
				if !strings.ContainsAny(arg, "*?[") {
					continue
				}
				for _, name := range published {
					if matched, _ := path.Match(arg, name); matched {
						t.Errorf("step %q appends %s to SHA256SUMS.txt, a pattern that also matches %s, which the build script listed already", step.Name, arg, name)
					}
				}
			}
		}
	}
	if appends == 0 {
		t.Fatal("no publish step appends to SHA256SUMS.txt")
	}
	check := stepIndex(steps, "uniq -d", "sha256sum --check --strict", "SHA256SUMS.txt")
	sign := stepIndex(steps, "cosign sign-blob SHA256SUMS.txt")
	switch {
	case check < 0:
		t.Error("no publish step checks SHA256SUMS.txt for an asset listed twice and against the files it names")
	case check < last:
		t.Error("SHA256SUMS.txt is checked before the last append to it")
	case sign >= 0 && sign < check:
		t.Error("SHA256SUMS.txt is signed before it is checked")
	}
}

// The editor plugins are release assets of the night: built in the publish job around the sysml-grpc it
// just built, listed in SHA256SUMS.txt before it is checked and signed, and skipped on a commit without them.
func TestNightlyPublishesTheEditorPluginsAsReleaseAssets(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	for _, name := range []string{"cameo-plugin", "syson-plugin"} {
		if _, ok := workflow.Jobs[name]; ok {
			t.Errorf("nightly.yml still has a %s job; the plugins are built in the publish job so the signed manifest lists them", name)
		}
	}
	publish := nightlyJob(t, workflow, "publish")
	steps := publish.runSteps()
	gated := stepIndex(steps, `grep -q 'opensysml.binaries' editors/mdk/dist/pom.xml`, "cameo=false", "editors/syson/frontend/package-lock.json", "syson=false")
	cameo := stepIndex(steps, `-Dopensysml.binaries="$PWD/dist/grpc"`, "-Dopensysml.requireService=true", "dist/cameo/Magic_MDK.zip", ">> SHA256SUMS.txt")
	syson := stepIndex(steps, "dist/syson/syson-mdk-backend.jar", "dist/syson/syson-mdk-frontend.tgz", ">> SHA256SUMS.txt")
	check := stepIndex(steps, "uniq -d", "sha256sum --check --strict", "SHA256SUMS.txt")
	release := stepIndex(steps, `gh release create "$VERSION"`)
	if gated < 0 {
		t.Fatal("the publish job does not check whether the selected commit carries the Cameo distribution's -Dopensysml.binaries and the SysON plugin")
	}
	if cameo < 0 {
		t.Fatal("the publish job does not package the Cameo plugin around the binaries it built, with its service-backed tests required, into dist/cameo/Magic_MDK.zip listed in SHA256SUMS.txt")
	}
	if syson < 0 {
		t.Fatal("the publish job does not collect dist/syson/syson-mdk-backend.jar and dist/syson/syson-mdk-frontend.tgz into SHA256SUMS.txt")
	}
	if check < 0 || release < 0 {
		t.Fatal("the publish job does not check SHA256SUMS.txt and create the per-night release")
	}
	if gated > cameo || gated > syson {
		t.Error("a plugin is packaged before the publish job checked the commit carries it")
	}
	if cameo > check || syson > check {
		t.Error("a plugin is appended to SHA256SUMS.txt after the manifest is checked, so the check does not cover it")
	}
	if !containsAll(steps[check].Command, "for dir in . grpc jupyter wasm cameo syson; do") {
		t.Error("the manifest check does not look under dist/cameo and dist/syson for the plugin assets it lists")
	}
	if !containsAll(steps[release].Command, "dist/cameo/Magic_MDK.zip", "dist/syson/syson-mdk-backend.jar", "dist/syson/syson-mdk-frontend.tgz") {
		t.Error("the per-night release is not created with the three plugin assets")
	}
	for _, step := range publish.Steps {
		switch {
		case strings.Contains(step.Run, `-Dopensysml.binaries="$PWD/dist/grpc"`):
			if step.If != "steps.layout.outputs.cameo == 'true'" {
				t.Errorf("the Cameo plugin is packaged under %q, not only when the commit carries its distribution module", step.If)
			}
			if !strings.Contains(step.Run, `-Dopensysml.version="$CORE-$VERSION"`) || step.Env["CORE"] != "${{ steps.version.outputs.core }}" || step.Env["VERSION"] != "${{ steps.version.outputs.tag }}" {
				t.Errorf("the Cameo plugin is versioned by CORE=%q VERSION=%q, not the snapshot core with the night as its pre-release part, as the vsix is", step.Env["CORE"], step.Env["VERSION"])
			}
			if !containsAll(step.Run, "bin/DIGESTS", "dist/SHA256SUMS.txt", "-ne 5", "diff ") {
				t.Error("the Cameo zip's DIGESTS are not checked against the five sysml-grpc lines of the manifest")
			}
		case strings.Contains(step.Run, "dist/syson/syson-mdk-frontend.tgz") && strings.Contains(step.Run, ">> SHA256SUMS.txt"):
			if step.If != "steps.layout.outputs.syson == 'true'" {
				t.Errorf("the SysON plugin is collected under %q, not only when the commit carries it", step.If)
			}
		case strings.Contains(step.Run, `gh release create "$VERSION"`):
			if step.Env["CAMEO"] != "${{ steps.layout.outputs.cameo }}" || step.Env["SYSON"] != "${{ steps.layout.outputs.syson }}" {
				t.Error("the release step does not take CAMEO and SYSON from the layout check, so it cannot require exactly the plugins the commit carries")
			}
		case strings.HasPrefix(step.Uses, "actions/upload-artifact@") && step.With["name"] != "npm-dist":
			t.Errorf("the publish job keeps %q as a workflow artifact; the plugins are release assets", step.With["name"])
		}
	}
	if workflow.Jobs["publish"].Outputs["core"] != "${{ steps.version.outputs.core }}" {
		t.Error("the publish job does not expose the snapshot core")
	}
}
