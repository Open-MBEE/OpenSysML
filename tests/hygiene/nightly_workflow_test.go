package hygiene

import (
	"os"
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
	Permissions map[string]string     `yaml:"permissions"`
	Env         map[string]string     `yaml:"env"`
	Jobs        map[string]actionsJob `yaml:"jobs"`
}

type actionsJob struct {
	Needs       yaml.Node         `yaml:"needs"`
	Environment string            `yaml:"environment"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []actionsStep     `yaml:"steps"`
}

type actionsStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	If   string            `yaml:"if"`
	With map[string]string `yaml:"with"`
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
	for _, key := range []string{"BUILD_SCRIPT", "SNAPSHOT_SCRIPT"} {
		script, ok := workflow.Env[key]
		if !ok {
			t.Fatalf("nightly.yml declares no %s", key)
		}
		if _, err := os.Stat("../../" + script); err != nil {
			t.Errorf("%s names %s, which the tree does not have: %v", key, script, err)
		}
	}
	select_ := nightlyJob(t, workflow, "select")
	if stepIndex(select_.runSteps(), `"$BUILD_SCRIPT" "$SNAPSHOT_SCRIPT"`, "git cat-file -e") < 0 {
		t.Error("the select job does not skip a green commit that lacks the build or snapshot-version script, which it could not build")
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
	release := stepIndex(publish.runSteps(), "gh release create", "--prerelease", "--latest=false")
	if release < 0 {
		t.Fatal("no step creates the releases as prereleases that are not latest")
	}
	command := publish.runSteps()[release].Command
	creates := strings.Count(command, "gh release create")
	if creates != 2 {
		t.Errorf("the release step creates %d releases; the per-night release and the alias make 2", creates)
	}
	for _, step := range publish.runSteps() {
		if step.Command != command && strings.Contains(step.Command, "gh release create") {
			t.Errorf("step %q also creates a release, outside the step that marks them prereleases", step.Name)
		}
	}
	if n := strings.Count(command, "--latest=false"); n != creates {
		t.Errorf("%d of the %d release creations pass --latest=false", n, creates)
	}
	if n := strings.Count(command, "--prerelease"); n != creates {
		t.Errorf("%d of the %d release creations pass --prerelease", n, creates)
	}
	if strings.Contains(raw, "gh release edit") {
		t.Error("nightly.yml edits a release, which could mark one latest after creation")
	}
	if !containsAll(command, `gh release create "$VERSION"`, `gh release create "$TAG"`) {
		t.Error("the release step does not create both the per-night release and the alias")
	}
	if strings.Index(command, `gh release create "$VERSION"`) > strings.Index(command, `gh release create "$TAG"`) {
		t.Error("the alias is created before the per-night release the clients pin")
	}
	if !containsAll(command, "dist/*.whl") {
		t.Error("the releases do not carry the Python wheel beside the binaries")
	}
}

func TestNightlyPythonSnapshotIsStampedBeforeItIsBuiltAndCheckedAfter(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	steps := nightlyJob(t, workflow, "publish").runSteps()

	install := stepIndex(steps, "npm ci")
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
	manifest := stepIndex(steps, "sha256sum opensysml-*.whl")
	sign := stepIndex(steps, "cosign sign-blob SHA256SUMS.txt")
	for name, index := range map[string]int{
		"npm ci": install, "version stamp": version, "binary build": binaries,
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
		{"npm ci runs after the manifest is stamped, which breaks the lock-file check", install, version},
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

func TestNightlyClientsReachTheRegistriesThroughTrustedPublishing(t *testing.T) {
	workflow, raw := loadNightlyWorkflow(t)
	for _, forbidden := range []string{"secrets.", "TWINE_PASSWORD", "NODE_AUTH_TOKEN", "NPM_TOKEN", "PYPI_API_TOKEN"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("nightly.yml mentions %s; the registries are reached with the workflow's OIDC identity, never a stored token", forbidden)
		}
	}
	for _, name := range []string{"pypi", "npm"} {
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

	pypi := nightlyJob(t, workflow, "pypi")
	published := false
	for _, step := range pypi.Steps {
		if strings.HasPrefix(step.Uses, "pypa/gh-action-pypi-publish@") {
			published = true
			if _, ok := step.With["password"]; ok {
				t.Error("the PyPI publish step passes a password; trusted publishing needs none")
			}
			if step.If == "" {
				t.Error("the PyPI publish step is unconditional; a version the index has must be skipped, not re-uploaded")
			}
		}
	}
	if !published {
		t.Error("the pypi job does not use pypa/gh-action-pypi-publish")
	}
	if stepIndex(pypi.runSteps(), "pypi.org/pypi/", "/json", "publish=false") < 0 {
		t.Error("the pypi job does not look the version up on the index before publishing")
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

func TestNightlyPrunesPerNightReleasesByAge(t *testing.T) {
	workflow, _ := loadNightlyWorkflow(t)
	days, err := strconv.Atoi(workflow.Env["RETENTION_DAYS"])
	if err != nil || days != 14 {
		t.Errorf("RETENTION_DAYS is %q; the snapshot page promises 14 days", workflow.Env["RETENTION_DAYS"])
	}
	steps := nightlyJob(t, workflow, "publish").runSteps()
	release := stepIndex(steps, "gh release create")
	prune := stepIndex(steps, "gh release delete", "--cleanup-tag", `"$RETENTION_DAYS days ago"`, "^nightly-[0-9]{8}-[0-9a-f]+$")
	switch {
	case prune < 0:
		t.Error("the publish job does not delete per-night releases, tag and all, older than $RETENTION_DAYS by their tag form")
	case prune < release:
		t.Error("old releases are pruned before this night's is published, so a failed publish leaves fewer nights than promised")
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
	release := stepIndex(steps, "gh release create")
	if release < 0 {
		t.Fatal("no step creates the releases")
	}
	command := steps[release].Command
	if strings.Contains(command, `gh release delete "$VERSION" --cleanup-tag`) {
		t.Error("the release step deletes a published night, tag and all, orphaning the packages that pin it")
	}
	if !containsAll(command, `gh release view "$VERSION" --json isDraft`, "already published", "exit 1") {
		t.Error("the release step does not refuse to replace a published night")
	}
}
