package hygiene

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The release's PyPI publication: .github/workflows/release-pypi.yml publishes the
// Python client and the Jupyter kernel package of a `v*` tag through trusted
// publishing, so every file carries an attestation, from the distributions CircleCI
// built, listed in the manifest it signed and published on the GitHub release.
// These tests hold the workflow to what docs/project/releasing.md promises — the
// upload waits for the gated release, publishes only bytes the signed manifest
// lists, runs the wheel against its release first, and reaches the index with the
// workflow's OIDC identity — and hold the CircleCI config to having handed the
// upload over.

const releasePyPIWorkflow = ".github/workflows/release-pypi.yml"

type releasePyPISpec struct {
	On struct {
		Push struct {
			Tags []string `yaml:"tags"`
		} `yaml:"push"`
		WorkflowDispatch struct {
			Inputs map[string]struct {
				Type     string `yaml:"type"`
				Required bool   `yaml:"required"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	actionsWorkflow `yaml:",inline"`
}

func loadReleasePyPIWorkflow(t *testing.T) (releasePyPISpec, string) {
	t.Helper()
	raw := readRepoFile(t, releasePyPIWorkflow)
	var workflow releasePyPISpec
	if err := yaml.Unmarshal([]byte(raw), &workflow); err != nil {
		t.Fatalf("parse %s: %v", releasePyPIWorkflow, err)
	}
	return workflow, raw
}

func releasePyPIJob(t *testing.T, workflow releasePyPISpec, name string) actionsJob {
	t.Helper()
	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatalf("%s has no %s job", releasePyPIWorkflow, name)
	}
	return job
}

func TestReleasePyPIRunsOnTheReleaseTagAndWritesNothingToTheRepository(t *testing.T) {
	workflow, _ := loadReleasePyPIWorkflow(t)
	if !reflect.DeepEqual(workflow.On.Push.Tags, []string{"v*"}) {
		t.Errorf("the workflow runs on tags %v, not v*; the release tag is what publishes the packages", workflow.On.Push.Tags)
	}
	inputs := workflow.On.WorkflowDispatch.Inputs
	if tag, ok := inputs["tag"]; !ok || !tag.Required {
		t.Error("the workflow cannot be dispatched for an existing tag, so a publication a transient failure interrupted cannot be retried")
	}
	if dry, ok := inputs["dry_run"]; !ok || dry.Type != "boolean" {
		t.Error("the workflow has no boolean dry_run input; the rehearsal of the publication fetches and verifies without uploading")
	}
	if workflow.Permissions["contents"] != "read" {
		t.Errorf("the workflow's contents permission is %q, not read; it publishes nothing to this repository", workflow.Permissions["contents"])
	}
	for _, job := range []string{"fetch", "pypi", "jupyter-kernel"} {
		if perms := releasePyPIJob(t, workflow, job).Permissions; perms["contents"] != "" && perms["contents"] != "read" {
			t.Errorf("the %s job's contents permission is %q", job, perms["contents"])
		}
	}
}

// TestReleasePyPIPublishesTheBytesCircleCISigned: the fetch job waits for the
// release CircleCI gated, takes the distributions from it, verifies the manifest
// under the pipeline's identity and every distribution against it, checks the
// metadata, runs the wheel against the release it names, and only then hands the
// files to the publishing jobs.
func TestReleasePyPIPublishesTheBytesCircleCISigned(t *testing.T) {
	workflow, _ := loadReleasePyPIWorkflow(t)
	fetch := releasePyPIJob(t, workflow, "fetch")
	runs := fetch.runSteps()

	version := stepIndex(runs, `check_version.py --tag "$TAG"`, `check_version.py --jupyter-kernel --tag "$TAG"`, `check_version.py --pre-release --tag "$TAG"`)
	gate := stepIndex(runs, `scripts/wait-for-release-gate.sh "$TAG" "$COMMIT"`)
	download := stepIndex(runs, `gh release download "$TAG"`, "--pattern SHA256SUMS.txt --pattern SHA256SUMS.txt.bundle",
		`--pattern "opensysml-${VERSION}-py3-none-any.whl"`, `--pattern "opensysml-${VERSION}.tar.gz"`,
		`--pattern "jupyter_opensysml_kernel-${VERSION}.tar.gz"`, `--pattern "jupyter_opensysml_kernel-${VERSION}-py3-none-*.whl"`)
	verify := stepIndex(runs, "cosign verify-blob SHA256SUMS.txt", "--bundle SHA256SUMS.txt.bundle",
		`--certificate-oidc-issuer "$MANIFEST_OIDC_ISSUER"`, `--certificate-identity-regexp "$MANIFEST_IDENTITY_REGEXP"`)
	check := stepIndex(runs, `[ $# -ne 5 ]`, `grep -q "  ${file}\$" SHA256SUMS.txt`, "sha256sum -c SHA256SUMS.txt --ignore-missing")
	metadata := stepIndex(runs, "twine check --strict client/python/dist/*", "twine check --strict client/jupyter-kernel/dist/*")
	smoke := stepIndex(runs, "pip install client/python/dist/opensysml-*.whl", "-u OPENSYSML_BINARY -u OPENSYSML_GRPC_VERSION",
		"opensysml.load(", `os.environ["RELEASE_TAG"]`, `cached["version"] == expected`)
	switch {
	case version < 0:
		t.Error("the fetch job does not resolve the version from the tag for both packages, nor route a pre-release")
	case gate < 0:
		t.Error("the fetch job does not wait for CircleCI to publish the gated release")
	case download < 0:
		t.Error("the fetch job does not download the two client distributions, the kernel sdist and wheels, the manifest and its signature from the release")
	case verify < 0:
		t.Error("the fetch job does not verify the manifest's signature under the pipeline's identity")
	case check < 0:
		t.Error("the fetch job does not check that the release carries five kernel wheels, that every distribution is a line of the signed manifest, and that the bytes match it")
	case metadata < 0:
		t.Error("the fetch job does not run twine check --strict over both distributions")
	case smoke < 0:
		t.Error("the fetch job does not run the wheel, with no binary configured, against the release the tag names")
	case !(version < gate && gate < download && download < verify && verify < check && check < metadata && metadata < smoke):
		t.Errorf("the fetch job must resolve the version (%d), wait for the release (%d), download (%d), verify the signature (%d), check the files (%d), check the metadata (%d), then run the wheel (%d)", version, gate, download, verify, check, metadata, smoke)
	}
	if check >= 0 && strings.Contains(runs[check].Command, "|| true") {
		t.Error("the manifest check tolerates a failure")
	}

	uploads := 0
	for _, step := range fetch.Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			uploads++
			if step.With["if-no-files-found"] != "error" {
				t.Errorf("artifact %s may be uploaded empty", step.With["name"])
			}
		}
	}
	if uploads != 2 {
		t.Errorf("the fetch job keeps %d artifacts, not the client's and the kernel's distributions", uploads)
	}

	// The identity the manifest must carry is the one the Python client pins.
	signing := readRepoFile(t, "client/python/opensysml/signing.py")
	issuer := regexp.MustCompile(`issuer='([^']+)'`).FindStringSubmatch(signing)
	project := regexp.MustCompile(`project='([^']+)'`).FindStringSubmatch(signing)
	if issuer == nil || project == nil {
		t.Fatal("signing.py no longer pins the release signer's issuer and project")
	}
	if workflow.Env["MANIFEST_OIDC_ISSUER"] != issuer[1] {
		t.Errorf("MANIFEST_OIDC_ISSUER is %q; the Python client pins %q", workflow.Env["MANIFEST_OIDC_ISSUER"], issuer[1])
	}
	identity, err := regexp.Compile(workflow.Env["MANIFEST_IDENTITY_REGEXP"])
	if err != nil {
		t.Fatalf("MANIFEST_IDENTITY_REGEXP does not compile: %v", err)
	}
	definition := "/pipeline-definitions/0f3a9c2e-1b4d-4e5f-8a6b-7c8d9e0f1a2b"
	if !identity.MatchString(project[1] + definition) {
		t.Errorf("MANIFEST_IDENTITY_REGEXP %q rejects a pipeline of the project the Python client pins, %s", identity, project[1])
	}
	if identity.MatchString(strings.Replace(project[1], "projects/", "projects/0", 1) + definition) {
		t.Errorf("MANIFEST_IDENTITY_REGEXP %q accepts a pipeline of another project", identity)
	}
}

// TestReleasePyPIReachesTheIndexThroughTrustedPublishing: the two publishing jobs
// hold no token, run in the environment the trusted publishers are registered for,
// refuse a version the index already has, and never skip an existing file.
func TestReleasePyPIReachesTheIndexThroughTrustedPublishing(t *testing.T) {
	workflow, raw := loadReleasePyPIWorkflow(t)
	for _, forbidden := range []string{"secrets.", "TWINE_PASSWORD", "PYPI_API_TOKEN", "twine upload", "--skip-existing", "attestations: false"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("%s mentions %q; the index is reached with the workflow's OIDC identity, every file attested, never a stored token or a silent skip", releasePyPIWorkflow, forbidden)
		}
	}
	nightly, _ := loadNightlyWorkflow(t)
	var nightlyPublish string
	for _, step := range nightlyJob(t, nightly, "pypi").Steps {
		if strings.HasPrefix(step.Uses, "pypa/gh-action-pypi-publish@") {
			nightlyPublish = step.Uses
		}
	}

	for name, packagesDir := range map[string]string{"pypi": "client/python/dist/", "jupyter-kernel": "client/jupyter-kernel/dist/"} {
		job := releasePyPIJob(t, workflow, name)
		if needs := job.needs(); len(needs) != 1 || needs[0] != "fetch" {
			t.Errorf("the %s job needs %v, not the fetch job alone; the two projects publish independently so neither refusing a re-run blocks the other", name, needs)
		}
		if job.Environment != "pypi" {
			t.Errorf("the %s job runs in environment %q; the trusted publishers are registered for `pypi`", name, job.Environment)
		}
		if job.Permissions["id-token"] != "write" {
			t.Errorf("the %s job cannot mint an OIDC token (id-token: %q)", name, job.Permissions["id-token"])
		}
		if job.Permissions["contents"] != "read" {
			t.Errorf("the %s job's contents permission is %q; publishing to a registry needs none", name, job.Permissions["contents"])
		}
		runs := job.runSteps()
		index := stepIndex(runs, "/pypi/", "/json", "already on", "exit 1", "publish=true")
		if index < 0 {
			t.Errorf("the %s job does not refuse a version the index already has before publishing", name)
		}
		published := false
		for _, step := range job.Steps {
			if !strings.HasPrefix(step.Uses, "pypa/gh-action-pypi-publish@") {
				continue
			}
			published = true
			if step.Uses != nightlyPublish {
				t.Errorf("the %s job uses %s, the nightly %s; one pin per action", name, step.Uses, nightlyPublish)
			}
			if _, ok := step.With["password"]; ok {
				t.Errorf("the %s job's publish step passes a password; trusted publishing needs none", name)
			}
			if step.If == "" {
				t.Errorf("the %s job's publish step is unconditional; it runs only when the index lacks the version and the run is not a dry run", name)
			}
			if step.With["packages-dir"] != packagesDir {
				t.Errorf("the %s job publishes %q, not %s", name, step.With["packages-dir"], packagesDir)
			}
			if !strings.Contains(step.With["repository-url"], "needs.fetch.outputs.repository-url") {
				t.Errorf("the %s job does not take the index from the fetch job, so a pre-release would not go to TestPyPI", name)
			}
		}
		if !published {
			t.Errorf("the %s job does not use pypa/gh-action-pypi-publish", name)
		}
	}
}

// TestCircleCIReleaseHandsThePyPIUploadToTheWorkflow: the CircleCI release
// workflow builds, signs and publishes the distributions on the GitHub release,
// and no longer uploads them to PyPI itself — an upload with its token would carry
// no attestation. The pysysml placeholder, a one-off on its own tag, is the one
// twine upload left.
func TestCircleCIReleaseHandsThePyPIUploadToTheWorkflow(t *testing.T) {
	config := loadCircleConfig(t)
	for _, name := range []string{"publish-pypi", "publish-pypi-jupyter-kernel"} {
		if _, ok := config.Jobs[name]; ok {
			t.Errorf("the CircleCI config has a %s job again; the release's PyPI upload is %s, with trusted publishing", name, releasePyPIWorkflow)
		}
	}
	for name, job := range config.Jobs {
		if name == "publish-pypi-pysysml" {
			continue
		}
		if stepIndex(runSteps(t, job.Steps), "twine upload") >= 0 {
			t.Errorf("CircleCI job %s uploads to PyPI with twine; a token upload carries no attestation", name)
		}
	}
	release := config.workflow(t, "release")
	for _, job := range release.Jobs {
		if job.Kind != yaml.MappingNode || len(job.Content) < 2 {
			continue
		}
		var spec struct {
			Name string `yaml:"name"`
		}
		if err := job.Content[1].Decode(&spec); err != nil {
			t.Fatalf("decode workflow job %s: %v", job.Content[0].Value, err)
		}
		if strings.Contains(spec.Name, "PyPI") {
			t.Errorf("the release workflow still has a job %q", spec.Name)
		}
	}
	github, ok := config.Jobs["publish-github-release"]
	if !ok {
		t.Fatal("no publish-github-release job")
	}
	if stepIndex(runSteps(t, github.Steps), "mv dist/*.whl dist/release/") < 0 {
		t.Error("publish-github-release does not put the wheels on the release the PyPI workflow publishes from")
	}
}
