# Releasing OpenSysML

A release is cut by pushing a `v*` tag. Everything after that is CircleCI: the
`release` workflow runs the test suite, cross-compiles `sysml`, `sysml-lsp` and
`sysml-grpc` for five platforms, and publishes them to a GitHub release. Nothing
is published from a laptop.

The Python client is released separately, by a `opensysml-v*` tag, which runs the
`release-python` workflow and uploads `opensysml` to PyPI — see
[Releasing opensysml to PyPI](#releasing-opensysml-to-pypi). The two are independent
on purpose: `opensysml` resolves a `sysml-grpc` binary at runtime from whichever
release the caller names, not from a release matching its own version.

A third tag, `pysysml-v*`, publishes the one-off final release of the client's
pre-rename PyPI name — see [The final `pysysml` release](#the-final-pysysml-release).

The other clients are released on tags of their own, and none of them has been
published yet: [the Node client](#releasing-opensysmlclient-to-npm) on
`client-node-v*`, [the Java client](#releasing-the-java-client-to-maven-central)
on `opensysml-java-v*`, and [the Rust client](#releasing-the-rust-client-to-cratesio)
on `opensysml-rust-v*`. The public Go API in `client/opensysml` has no release of its
own: it is part of this module, so the core's `v*` tag is what a Go program pins.

## Before tagging

Run the full gate on the commit you intend to tag:

```bash
gofmt -l .            # must print nothing
go build ./...
go vet ./...
make lint             # staticcheck + gosec, as CircleCI runs
go test -race -count=1 ./...
go test -run TestStdlibConformance ./internal/core/libs
```

Run the Python client the way CircleCI's `python-test` job does, since a
`opensysml-v*` release gates on the same suite:

```bash
make build-grpc && mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/
pip install -e clients/python/ && pip install pytest pytest-mock psutil
pytest clients/python/tests/ -v
```

The OMG training-corpus gate skips while the corpus is absent, so fetch it and
run it explicitly — the expected result is the pinned baseline, currently
100/100 files clean:

```bash
./scripts/download-training-examples.sh
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 go test -count=1 ./internal/core/model -run TestTrainingExamples
```

A change in that count is a finding to adjudicate file by file, never a
baseline to regenerate.

Then check the release-facing text:

- The changelog fragments under `changes/unreleased/` are folded into a dated entry for this
  version: `python3 scripts/changelog.py release X.Y.Z` (or `--date YYYY-MM-DD`) appends them
  to the `## Unreleased` section, renames it, and deletes the fragments. Commit `CHANGELOG.md`
  and the deletions together. The previous version's entry stays unchanged.
- `README.md` and `docs/guide/` transcripts match what the binary prints.
  Build it (`make build-sysml`) and paste a few commands through it.
- `python3 scripts/check-doc-links.py` reports no broken link (CI gates on it too).
- Test counts match a real run and agree across the four surfaces allowed to repeat them
  (`docs/project/spec-compliance.md`, `README.md`, `docs/project/roadmap.md`,
  `docs/project/training-examples.md` — everything else links to the first, per CONTRIBUTING.md),
  and no compliance row claims more than the implementation does. Count first-level subtests:
  a case that registers sub-subtests, like `variant_connection_per_owner`, otherwise counts twice.

## Tagging

The tag is the version: CircleCI passes `CIRCLE_TAG` to the build as
`VERSION`, so `sysml --version` reports it.

```bash
git checkout main && git pull
git tag -a v0.0.5 -m "v0.0.5"
git push origin v0.0.5
```

The tag belongs on the repository the releases live on. v0.0.1–v0.0.7 are
releases of `Open-MBEE/OpenSysML`, while development happens on
`JPL-Devin/OpenSysML`, which has no tags at all — so cutting a release means
promoting `main` upstream first (v0.0.4 came through Open-MBEE PR #47) and
tagging there. Tagging the development repository would build a release nobody
consumes.

Tags are matched by `/^v.*/` in `.circleci/config.yml`. A tag on a commit that
fails the suite fails the release workflow before anything is published.

## What CircleCI publishes

`build-release` produces, in `dist/`:

- per-binary archives — `sysml-<os>-<arch>.tar.gz`,
  `sysml-lsp-<os>-<arch>.tar.gz` (`.zip` on Windows);
- bundle archives — `opensysml-<os>-<arch>.tar.gz` holding both binaries under
  their plain names, which is the layout Homebrew and a PATH install expect,
  plus their section 1 manual pages under `share/man/man1` (the Unix archives
  only; the Windows bundle has no use for them, and `sysml-grpc.1` stays out
  of a bundle that does not carry `sysml-grpc`);
- `sysml-grpc-<os>-<arch>`, published raw with a `.sha256` sidecar rather than
  archived, because that is what `opensysml` downloads and verifies
  (`clients/python/opensysml/binary.py`) when it starts the service for a Python caller;
- `SHA256SUMS.txt` over every archive and every `sysml-grpc` binary.

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64.

Before any of it is stored or published, `build-release` runs each host-platform
binary and fails the release unless `--version` reports `CIRCLE_TAG`. The ldflags
are the only thing stamping the tag into a binary, and a binary reporting `dev`
or a stale tag looks the same on the release page as a correct one — that is how
an artifact whose version disagreed with its tag reached a release once already.
The check runs the linux/amd64 builds; the cross-compiled ones cannot run on the
executor, so each is checked for the tag string the ldflags write into it.

`publish-github-release` uploads them with `ghr`, using a token from
`GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project settings.
It runs with `-replace`, so re-running the workflow for the same tag replaces
that release's assets rather than appending duplicates, and leaves everything
else on the release alone: notes, title and the prerelease/latest flags survive.
A tag that has no release yet still gets one created.

Do not go back to `-delete`. It is an alias of `-recreate`: it deletes the
existing release *and its tag* and creates an empty one, which wipes
hand-written release notes (the notes must therefore be on a published release —
`ghr` does not see a draft release for the tag and would publish a second, empty
one alongside it).

## After the release

1. **Verify a download** on at least one platform:

   ```bash
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/opensysml-linux-amd64.tar.gz
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/SHA256SUMS.txt
   sha256sum -c SHA256SUMS.txt --ignore-missing
   tar xzf opensysml-linux-amd64.tar.gz && ./sysml --version
   ```

   `--version` must report the tag, not `dev`.

   Then check the path `opensysml` takes, since it reads the sidecar rather than
   `SHA256SUMS.txt`:

   ```bash
   OPENSYSML_GITHUB_REPO=Open-MBEE/OpenSysML python -c \
     "from opensysml.binary import download_binary; print(download_binary('latest'))"
   ~/.opensysml/bin/sysml-grpc -version
   ```

   A checksum mismatch there means the sidecar and the binary came from
   different builds.

   For a release no published `opensysml` pins, that path depends on the
   signature, so check it the way the client does:

   ```bash
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/SHA256SUMS.txt.bundle
   cosign verify-blob SHA256SUMS.txt --bundle SHA256SUMS.txt.bundle \
     --certificate-oidc-issuer https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62 \
     --certificate-identity-regexp '^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f-]+$'
   ```

   A missing bundle means `build-release` did not sign — re-run the tag's
   workflow rather than pinning around it.

2. **Let the Homebrew tap pick the release up.** The tap repository
   `Open-MBEE/homebrew-tap` updates itself: a scheduled workflow there resolves
   the latest `Open-MBEE/OpenSysML` release, renders `Formula/opensysml.rb` from
   this repository's `scripts/render-homebrew-formula.sh` and formula template at
   that tag, and commits only when the file changed. Nothing here triggers it, so
   the formula follows the release within the workflow's schedule interval.

   If it does not, check the workflow run in the tap repository. The render reads
   the release's `SHA256SUMS.txt`, so a release missing that asset (or missing a
   `opensysml-<os>-<arch>.tar.gz` line in it) fails the run loudly instead of
   committing a broken formula — re-run `publish-github-release` for the tag and
   then the tap workflow (`workflow_dispatch`). Rendering by hand still works:

   ```bash
   scripts/render-homebrew-formula.sh v0.0.5 > Formula/opensysml.rb
   ```

   See [packaging/homebrew/README.md](../../packaging/homebrew/README.md).

3. **Say what is not signed.** macOS binaries are not Developer ID signed or
   notarized, so a browser download trips Gatekeeper. Point release notes at
   [MACOS_DISTRIBUTION.md](macos-distribution.md), which gives the workarounds
   and what signing would take. Windows binaries are Authenticode signed through
   SignPath Foundation once the application below is approved; until then, and
   for a release whose signing request nobody approved, only the unsigned
   Windows assets exist and SmartScreen warns — say so in the notes.

4. **Approve the Windows signing request.** When SignPath is configured, the
   tag also runs [`release-windows.yml`](../../.github/workflows/release-windows.yml),
   which parks a signing request in SignPath until an Approver approves it
   (see [Windows Authenticode signing](#windows-authenticode-signing)). No
   approval, no `*-signed*` assets on the release.

5. **Check the Windows installer landed.** The same workflow builds the MSI
   (see [The Windows installer](#the-windows-installer)) once CircleCI has
   published the release: `opensysml-<x.y.z>-windows-amd64.msi` with
   `SHA256SUMS-windows-msi.txt` when SignPath is not configured, or
   `opensysml-<x.y.z>-windows-amd64-signed.msi` listed in
   `SHA256SUMS-windows-signed.txt` when it is. A release with neither means the
   workflow failed (WiX, the Z3 download, ICE validation or the signing
   request); re-run it after fixing the cause.

6. **Render the Windows package-manager manifests** when a maintainer wants
   to (re)submit them externally. Nothing here submits anything:

   ```bash
   scripts/render-scoop-manifest.sh v0.0.5 > opensysml.json
   scripts/render-winget-manifests.sh v0.0.5 out/
   scripts/render-msys2-pkgbuild.sh v0.0.5 > PKGBUILD
   ```

   The procedure for each external repository is in
   [packaging/scoop](../../packaging/scoop/README.md),
   [packaging/winget](../../packaging/winget/README.md) and
   [packaging/msys2](../../packaging/msys2/README.md).

### The signed checksum manifest

`build-release` signs `dist/SHA256SUMS.txt` — the manifest covering every
published artifact, including each `sysml-grpc-<os>-<arch>` — with cosign
keyless, and `publish-github-release` uploads the sigstore bundle beside it as
`SHA256SUMS.txt.bundle`. The certificate identity comes from the job's CircleCI
OIDC token exchanged with Fulcio, so **no signing key exists anywhere**: nothing
to provision, rotate or leak. The job then verifies its own bundle and fails the
release rather than publish a signature clients would reject.

That signature is what lets the Python and Node clients install a core release
published after them. For a release a client pins no digest for, it downloads
the manifest and the bundle, verifies the bundle, and takes the asset's digest
from the verified manifest. The only signature accepted is this pipeline's:

| | |
|---|---|
| OIDC issuer | `https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62` |
| Certificate subject | `https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/<pipeline definition>` |

The issuer is CircleCI's OIDC issuer for the organization that owns this
project, and the subject is the pipeline definition that ran the job, which is
what CircleCI puts in the certificate. The client currently accepts any pipeline
definition of that project, because no signature of the real one exists to read
the identifier off yet — the verify step in `build-release` prints it, so after
the first signed release set it as `definition=` on the signer in
`clients/python/opensysml/signing.py` and in `clients/node/src/node/signing.ts`
to narrow the pin to the one pipeline.

Anything short of a verified manifest is refused exactly as an unpinned release
is today: no bundle asset, a bundle that does not verify, another signer, a
manifest changed after signing, an expired certificate, or `sigstore` not
installed. The `.sha256` served beside a binary is still never a reason to trust
it — same origin as the binary — and remains behind
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

### Windows Authenticode signing

The policy users see is the [Code signing policy](../../README.md#code-signing-policy)
in the README; this is the maintainer side of it. SignPath Foundation signs
open-source Windows binaries for free, on two conditions this section keeps
satisfied: the binaries must be built by a build system SignPath can verify the
origin of, and each signing request must be approved by hand.

**Why GitHub Actions, and why CircleCI stays.** SignPath verifies the origin of
an artifact through a *trusted build system* connector, and its supported
systems are GitHub Actions, GitLab, Jenkins, Azure DevOps, TeamCity and
AppVeyor — not CircleCI. So the Windows binaries a release signs are rebuilt by
[`.github/workflows/release-windows.yml`](../../.github/workflows/release-windows.yml)
on the same `v*` tag, with the Makefile targets and the exact `VERSION`,
`COMMIT`, `BUILD_TIME` and `GO_VERSION` derivation `build-release` uses, so
both builds stamp the same version, commit and Windows `VERSIONINFO` (only the
build timestamp differs).
CircleCI keeps publishing everything it publishes today, unsigned Windows zips
included, together with `SHA256SUMS.txt` and its cosign bundle.

The signed files are **additional** assets — `sysml-windows-amd64-signed.zip`,
`sysml-lsp-windows-amd64-signed.zip`, `sysml-grpc-windows-amd64-signed.exe`
(with a `.sha256` sidecar, as the unsigned one has) and
`opensysml-windows-amd64-signed.zip`, listed in `SHA256SUMS-windows-signed.txt`.
They do not replace the unsigned ones, on purpose: the cosign-signed manifest is
what the Python and Node clients trust, and its certificate identity is this
project's CircleCI pipeline. A GitHub Actions job cannot re-sign that manifest
under the CircleCI identity, and overwriting `sysml-windows-amd64.zip` with a
signed zip would leave `SHA256SUMS.txt` describing bytes that are no longer on
the release. The `-signed` names keep every line of the manifest true and every
existing download link and client pin working. `SHA256SUMS-windows-signed.txt`
is a convenience for humans; the verifiable statement about a signed file is its
Authenticode signature (`Get-AuthenticodeSignature` in PowerShell, or
`osslsigncode verify`), and `opensysml` keeps downloading the unsigned
`sysml-grpc-windows-amd64.exe` it can verify against the manifest.

**Applying.** A maintainer applies once at <https://signpath.org/apply>
with the repository URL `https://github.com/Open-MBEE/OpenSysML`. The
conditions at <https://signpath.org/terms> ask for what the README's policy
section provides: an OSI-approved license (Apache-2.0), the sentence naming
SignPath.io and SignPath Foundation, the Authors / Reviewers / Approvers roles
with links to the GitHub teams that hold them, the privacy statement, and MFA
for everyone in those roles. Before applying, make sure the three teams the
README links to actually exist in the Open-MBEE organization (or edit the
README to the teams that do) and that every member has MFA enabled on GitHub.

**Configuring, once approved.** SignPath creates an organization for the
project; in it, create a project for this repository with an *artifact
configuration* describing a zip of `.exe` files to be Authenticode-signed (the
workflow uploads the three executables as one artifact, and the metadata
restriction should require `ProductName` `OpenSysML`), a *release* signing
policy with manual approval, and a trusted build system link to GitHub Actions
for `Open-MBEE/OpenSysML` following
<https://docs.signpath.io/trusted-build-systems/github>. Then, in the GitHub
repository settings:

| Where | Name | Value |
|---|---|---|
| Secret | `SIGNPATH_API_TOKEN` | the API token of the SignPath CI user for the project |
| Variable | `SIGNPATH_ORGANIZATION_ID` | the SignPath organization ID (a GUID) |
| Variable | `SIGNPATH_PROJECT_SLUG` | the project slug, e.g. `OpenSysML` |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | the release policy slug, e.g. `release-signing` |
| Variable | `SIGNPATH_MSI_ARTIFACT_CONFIGURATION_SLUG` | the artifact configuration for the MSI (see [The Windows installer](#the-windows-installer)) |

With any of the first four missing the workflow builds the binaries, checks
their `VERSIONINFO` against the tag, keeps them as a workflow artifact, builds
and publishes the **unsigned** MSI, and stops: nothing is submitted to SignPath.
With the four present but the fifth missing, the `msi-signed` job fails on
purpose rather than publish an MSI of signed executables under a name that
claims the MSI itself is signed. Try it before the
first real tag with **Run workflow** (`workflow_dispatch`), which stamps the
`version` input instead of a tag and never publishes; tick `submit` to also
exercise the SignPath round trip, which creates a real signing request an
Approver has to approve or deny.

**Every release needs an approval.** On a `v*` tag the workflow first waits
(up to 90 minutes) for `publish-github-release` to put `SHA256SUMS.txt.bundle`
on the release and checks that the tag still resolves to the commit it built —
CircleCI publishes only after the suite and `build-release` passed on the tag,
so a tag CircleCI rejected is never signed. It then submits the artifact and
waits (up to about four hours) for the request to complete.
An Approver — a member of the Approvers team listed in the README, with MFA on
their SignPath account — opens the request in SignPath, checks that it points at
the expected commit and workflow run, and approves it. The job then downloads
the signed executables, checks their `VERSIONINFO` still carries the tag,
packages the `-signed` assets and uploads them to the release with
`softprops/action-gh-release`, overwriting only assets of those names. If the
request is denied or the wait times out, the job fails and the release simply
has no signed Windows assets; re-run the job after the request is approved, or
leave it unsigned and say so in the notes. SignPath also revokes signing for
projects whose Authors, Reviewers or Approvers do not keep MFA enabled, so keep
the team membership current.

**VERSIONINFO.** SignPath enforces the metadata a signed file carries.
`packaging/windows/<cmd>.winres.json` holds the static fields (`ProductName`
`OpenSysML`, `CompanyName`, `FileDescription`, `LegalCopyright`,
`OriginalFilename`), and the Makefile's `build-sysml`, `build-lsp` and
`build-grpc` targets run `go-winres` (pinned by `GO_WINRES_VERSION`, a build
tool that ends up nowhere in the product) for `GOOS=windows` only, writing
`cmd/<cmd>/rsrc_windows_<arch>.syso` with `ProductVersion` and `FileVersion`
set to the same `VERSION` the `-ldflags` carry. The `.syso` files are ignored
by Git and by every non-Windows build. `make windows-versioninfo-check
EXE=dist/sysml-windows-amd64.exe VERSION=v0.5.0` extracts the resource from a
built binary and fails unless all of that is true; the workflow runs it on the
unsigned and again on the signed executables.

### The Windows installer

`packaging/msi/opensysml.wxs` (WiX Toolset v5, plain MSI, no bootstrapper) and
`scripts/build-msi.sh` produce `opensysml-<x.y.z>-windows-amd64.msi`: a
per-machine x64 installer of `sysml.exe`, `sysml-lsp.exe`, `LICENSE.txt` and,
as separately deselectable features, `sysml-grpc.exe` and the Z3 solver
(`z3\z3.exe` with its runtime DLLs and `LICENSE-z3.txt`), both directories on
the system `PATH`. `MajorUpgrade` makes a newer MSI replace an older install;
the `ProductVersion` is the tag without `v` and without any pre-release suffix
(`v0.4.0-rc1` and `v0.4.0` are both `0.4.0`, and the later one wins). Details,
feature ids and the `msiexec` incantations are in
[packaging/msi/README.md](../../packaging/msi/README.md).

**Where it is built, and why not in CircleCI.** WiX v5 runs only on Windows (its
cabinet builder is a Win32 executable and the toolset rejects Linux paths), so
the MSI cannot come out of the `cimg/go` release job. It is built by
`release-windows.yml` on a `windows-latest` runner, in two shapes:

- **SignPath not configured:** the `msi` job builds the MSI from the unsigned
  executables the workflow built, runs `wix msi validate` (ICE), and the
  `publish-msi` job uploads it with `SHA256SUMS-windows-msi.txt` — after the
  same CircleCI gate the signing job uses, so the MSI never lands on a release
  CircleCI did not publish. Both jobs run on every tag and on
  `workflow_dispatch` (which never publishes).
- **SignPath configured:** the `msi-signed` job rebuilds the MSI from the
  SignPath-signed executables, validates it, submits the MSI itself to SignPath
  under `SIGNPATH_MSI_ARTIFACT_CONFIGURATION_SLUG`, and `publish-signed`
  uploads it as `opensysml-<x.y.z>-windows-amd64-signed.msi`, listed in
  `SHA256SUMS-windows-signed.txt` with the other `-signed` assets. The unsigned
  MSI is then kept only as a workflow artifact, so a release never carries two
  installers whose contents differ only by signature.

The trade-off is the one the `-signed` assets already make: the MSI is not in
`SHA256SUMS.txt` or the cosign bundle, because those are produced by CircleCI
from the bytes CircleCI built, and an MSI built elsewhere (and, when signed,
from different executable bytes) must not be described by a manifest that did
not hash it. The unsigned MSI has its own `SHA256SUMS-windows-msi.txt`; for the
signed one the verifiable statement is its Authenticode signature. Nothing the
clients download or pin changes.

**SignPath and the MSI.** Add a second artifact configuration to the SignPath
project: a single `.msi` file, Authenticode-signed, and put its slug in the
`SIGNPATH_MSI_ARTIFACT_CONFIGURATION_SLUG` variable. The MSI's executables are
already signed by the first request, so this second request signs only the
installer. **`z3.exe` and its DLLs are never signed**: SignPath Foundation's
terms allow unsigned upstream open-source binaries inside a signed installer but
not signing them with the Foundation certificate, so the artifact configuration
for the MSI must not descend into its contents. Each release therefore parks
two signing requests for an Approver (executables, then the MSI).

**Updating the bundled Z3.** `packaging/msi/z3.pin` pins the Z3 release
(`Z3_VERSION`, the `z3-<ver>-x64-win.zip` asset name and its SHA256) in one
place; the build script refuses a zip whose hash differs. The update procedure —
fetch the new zip, hash it yourself, update the pin, check the zip's `bin/` still
has the files the `.wxs` lists — is in
[packaging/msi/README.md](../../packaging/msi/README.md#updating-the-z3-pin). Bump
it deliberately, in its own PR with a changelog fragment naming the new Z3
version: it changes what every installer ships.

### Pinned release digests

The table in `clients/release-digests.json`, which every client ships a synced
copy of, still covers the releases published before signing existed, and it
stays the override: where a pin exists
it wins, and a verified manifest that disagrees with a pin is an error rather
than a downgrade. **Per release there is now nothing to do** — pinning a release
signed by the pipeline is optional. Pinning still works, and is worth doing for
a release clients on an older `opensysml` should be able to install:

```bash
export GITHUB_TOKEN=...   # must be able to read this repository's releases
python clients/python/scripts/pin_release_checksums.py --version v0.0.8 --write
```

The token is required, not an optimization: the script reads the release's assets
through the GitHub releases API, and unauthenticated calls are rate-limited per
address and fail as an opaque HTTP 403. `GH_TOKEN` is read as well. The scope
needed is read access to this repository's releases — `public_repo` for a classic
token, `Contents: read` for a fine-grained one; nothing is written through the
API. Without either variable the script fails immediately with
`MissingTokenError` naming the variable, rather than at the first request.

## The SonarCloud scan

Not a release step — the `scan` job runs in the `build-test` workflow on every
commit, after `build-and-test`, `python-test`, `java-test` and `node-test` —
but it is documented here with the other CircleCI credential plumbing.

It waits on the three client jobs because each writes a coverage report the
scan reads: a language whose report is absent has every one of its lines
counted as uncovered, which is what dropped new-code coverage to 54.8% on the
0.4.0 analysis while the suites themselves were passing.

The job references the organization context named exactly `SonarCloud`, which
supplies `SONAR_TOKEN` (the same context `Open-MBEE/flexo-mms-layer1-service`
uses, so no new credential is provisioned). It reads
`sonar-project.properties` at the repository root and four coverage reports
persisted to the workspace — `coverage.txt` from `build-and-test`,
`coverage-python.xml` from `python-test`, `coverage-node.lcov` from
`node-test`, and JaCoCo's `jacoco.xml` per Java module — and it un-shallows the
clone because SonarCloud needs full history for blame and new-code detection.

The Go profile is written with `-coverpkg=./...` so a package is credited for
the code it exercises elsewhere; without it `internal/core/ast/dump.go`
measures 21% though the parser's golden tests run 90% of it.

`java-test` also persists each module's `target/classes`, `target/test-classes`
and a `target/dependency` directory it fills with `dependency:copy-dependencies`
(the Maven repository itself is that job's cache, not the workspace). The Java
sensor resolves types from those, and without them it warns about missing
`sonar.java.binaries`/`sonar.java.libraries` and degrades to a syntactic
analysis, so the `scan` job fails if any of the six directories is empty.
`sonar.python.version` names the range `clients/python/pyproject.toml` declares,
because unset the Python sensor assumes every Python 3 version and drops the
rules that depend on one.

On a forked PR the context is withheld, so `SONAR_TOKEN` is empty; the job
halts successfully rather than failing every outside contribution. When the
token is present, a failing scan fails the job.

The job checks out with `method: full`, and that is load-bearing: CircleCI's
default checkout is a *blobless* partial clone, and Sonar blames every file with
JGit, which cannot fetch a blob on demand — against a blobless clone the scan
dies with `MissingObjectException: Missing blob ...` in the SCM publisher. A
step after the checkout fails the job if the clone is partial or missing an
object reachable from `HEAD`, so a blame that would silently date every issue to
the import is reported as the configuration error it is. Other jobs read only
the current tree and keep the faster default.

The job runs on a `large` container with `SONAR_SCANNER_OPTS: -Xmx4g`, which is
not tuning for its own sake: Sonar's Go sensor parses one directory at a time
and holds that directory's parser output in memory, so a large package
(`internal/core/runtime`) exhausted the scanner's default heap with
`java.lang.OutOfMemoryError`. If a new package makes it fail there again, raise
that heap rather than splitting the package. Note also that the launcher in the
pinned scanner CLI passes `$SONAR_SCANNER_OPTS` and not
`SONAR_SCANNER_JAVA_OPTS`, so the newer variable name has no effect here.

### What counts as new code

The quality gate is mostly conditions on *new* code — coverage, the security and
reliability ratings, duplication — so which lines are new decides whether it is
red, and the gate says nothing useful if that set is wrong. The project uses
SonarCloud's default period, **previous version**, whose baseline is the last
analysis carrying a version other than the current one. An analysis that names
no version carries the placeholder `not provided`: every analysis then looks
like the same version, no earlier one differs, and the period silently falls
back to the first analysis ever run. That happened here — the analysis of
2026-08-27 counted 105,050 of 110,609 lines as new, so "coverage on new code"
was whole-project coverage measured against a 70% threshold that project-wide
coverage is not held to, and the gate was red for it.

The `scan` job therefore derives the version from the nearest release tag that
is an ancestor of `HEAD` (`git describe --tags --abbrev=0 --match 'v[0-9]*'`,
without the `v`) and passes it as `-Dsonar.projectVersion` in
`SONAR_SCANNER_OPTS`. New code is then what has been committed since that
release was first analyzed, and cutting a release moves the baseline forward on
its own: the first analysis after a `v*` tag reports a version the previous
analyses did not, which is exactly the boundary the period wants. The job fetches
tags explicitly, because CircleCI's checkout fetches only the ref being built.

Two failure modes are worth recognizing, since neither fails the job. If no `v*`
tag is an ancestor of `HEAD` the step says so and leaves the version unset,
which is the fallback above. And if the version stops reaching the server, the
period collapses again: check it with

```bash
curl -s "https://sonarcloud.io/api/project_analyses/search?project=Open-MBEE_OpenSysML&ps=1"
```

whose `projectVersion` must be the release number, not `not provided`.

One-time maintainer step (already done for `Open-MBEE_OpenSysML`, but true of
any future project): SonarCloud does not create a project from a CI-run scan
(the scanner sends branch parameters, and Cloud cannot provision from those —
the first run fails with `Could not find a default branch for project with key
'...'`). Create the project under the organization first, either from the
SonarCloud UI or with `POST api/projects/create` followed by
`POST api/project_branches/rename`, using a token that has Create Projects in
that organization.

## Releasing opensysml to PyPI

The Python client in `clients/python/` is published to PyPI as
[`opensysml`](https://pypi.org/project/opensysml/) by the `release-python` workflow,
which runs on a tag matching `/^opensysml-v.*/` — for example `opensysml-v0.3.0`.
The first release under the new name is 0.3.0: the version line carries on from
`pysysml` 0.2.0, which was the same client, so no version number is reused.
Nothing is uploaded from a laptop, and a `v*` core release tag publishes no
package.

### Why its own tag

`opensysml` does not ship the service: it downloads a `sysml-grpc` binary at
runtime for whatever release the caller names (`version=`,
`$OPENSYSML_GRPC_VERSION`, or `latest`), verifying it against the digest it pins
for that release (its copy of `clients/release-digests.json`) or, for a
release it pins nothing for, against the digest in the release's signed
`SHA256SUMS.txt` (see [the signed checksum manifest](#the-signed-checksum-manifest)),
which is why a core release published after a client release needs no new client
release. Its version therefore says nothing about which core release it runs
against, and tying the two together would put a new, immutable PyPI version on
every core release and would block a client-only fix behind a core release.

Keeping them apart also protects the `v*` path: `publish-github-release` runs
`ghr -replace`, so re-running a core release is an ordinary operation, while a
PyPI version can be yanked but never re-uploaded. A re-run must never have an
irreversible upload hanging off it.

### The version, in one place

`clients/python/opensysml/_version.py` is the only declaration:

- `clients/python/pyproject.toml` has `dynamic = ["version"]` and reads
  `opensysml._version.VERSION` (there is no `setup.py` any more —
  `pyproject.toml` declares the build);
- `opensysml.__version__` reports that declaration, which ships beside the module
  and is therefore the version of the code being imported. A wheel's metadata is
  generated from it, so the two agree there; an editable install's dist-info is
  written once, at install time, and a checkout that bumps `VERSION` afterwards
  would otherwise report the version it had when `pip install -e` ran.

`clients/python/tests/test_version.py` fails if a second version literal reappears
anywhere under `clients/python/`, or if the declaration, the installed metadata and
`__version__` stop agreeing. Where the install is editable, the tests locate the
package through the install's own PEP 610 record (`opensysml/_dist.py`) rather than
the dist-info's directory, which for an editable install is a site-packages path
holding no `opensysml/` at all.

The tag must name the declared version. `clients/python/scripts/check_version.py` is run
by the job before anything is built, and fails loudly otherwise:

```bash
python clients/python/scripts/check_version.py --tag opensysml-v0.3.0   # prints 0.3.0
```

So a release is: bump `VERSION` in `clients/python/opensysml/_version.py`, land it, then
tag `opensysml-v<that version>`.

### What the job needs

The token lives in a **restricted context**, not in project environment
variables, so only the release path can read it:

1. In CircleCI, **Organization Settings → Contexts**, in the context named
   `PyPI` (create it if the organization does not have it yet). A context
   reference in the config is matched exactly, so the name must be spelled with
   the same case in both places.
2. **Restrict it to a security group** (Contexts → `PyPI` → *Add security
   group*) so only that group's members can run a job that uses it. A context
   with no group restriction is readable by every project job.
3. Add the token as `PYPI_API_TOKEN` (an *environment variable* in that
   context). `TWINE_USERNAME` is `__token__`, set by the job; only the token
   value belongs in the context.
4. Optionally add `TEST_PYPI_API_TOKEN`, a TestPyPI token, which is what a
   pre-release tag uses (see the dry run below).

`.circleci/config.yml` references the context from the job in the workflow:

```yaml
      - publish-pypi:
          context:
            - PyPI
```

Any other variables that context happens to carry are ignored. In particular a
`PYPI_USERNAME`/`PYPI_PASSWORD` pair cannot publish to PyPI at all: uploads from
an account with 2FA have required an API token or a trusted publisher since
2023-06-01, and 2FA has been mandatory for every account since 2024-01-01, so a
password is answered with a 403.

The job refuses to run `twine` when the variable it needs is absent, naming the
variable and the context, rather than letting PyPI answer with a 403 that reads
like a permissions problem. It never echoes the token and never prints the
environment.

### The token the job uses

`opensysml` exists on PyPI (0.3.0 and 0.3.1 are published), so the token in the
`PyPI` context as `PYPI_API_TOKEN` must be **scoped to the `opensysml` project**,
never account-scoped: an account-scoped token in CI can publish anything the
account owns. Keep a second owner/maintainer on the PyPI project as well, so it is
not tied to one account.

PyPI trusted publishing (OIDC) is not an option: the supported providers are
GitHub Actions, Google Cloud, ActiveState and GitLab CI/CD, and CircleCI support
is still open upstream ([pypi/warehouse#13888](https://github.com/pypi/warehouse/issues/13888)).
An API token is the authentication CircleCI has.

### What the job does, in order

1. `check_version.py` — the tag must name the declared version.
2. Requires the token for the index it will use.
3. Refuses to continue if that index already has this version (a re-run of an
   already-published version fails here, deliberately: it cannot be replaced,
   and `--skip-existing` would let a half-intended re-run look successful).
4. `python -m build` — wheel *and* sdist.
5. `twine check --strict dist/*` — the metadata a broken listing comes from.
6. Installs the built wheel into a clean virtualenv, imports it, and checks
   `opensysml.__version__` is the version being published.
7. `twine upload` with `TWINE_USERNAME=__token__` and the token from the
   context.

### Dry run on TestPyPI

A **pre-release version publishes to TestPyPI instead of PyPI** — that is the
whole rule, so the happy path has no extra switch to forget. To rehearse a
release:

```bash
# 1. Declare a pre-release version, e.g. VERSION = "0.3.0rc1"
$EDITOR clients/python/opensysml/_version.py
# 2. Land it, then tag it
git tag -a opensysml-v0.3.0rc1 -m "opensysml 0.3.0rc1" && git push origin opensysml-v0.3.0rc1
```

The job resolves the version, sees a PEP 440 pre-release, requires
`TEST_PYPI_API_TOKEN`, and uploads to `https://test.pypi.org/legacy/`. Verify it
the same way as a real release, pointing pip at TestPyPI but taking the
dependencies from PyPI:

```bash
python -m venv /tmp/opensysml-rc && . /tmp/opensysml-rc/bin/activate
pip install --index-url https://test.pypi.org/simple/ \
            --extra-index-url https://pypi.org/simple/ opensysml==0.3.0rc1
python -c "import opensysml; print(opensysml.__version__)"
```

Then set `VERSION` to the final version and tag `opensysml-v0.3.0`.

Nothing about the pre-release path is required for a normal release; if you skip
it, no TestPyPI token is needed at all.

### Verifying an upload

In a clean virtualenv, from the index — not from the source tree:

```bash
python -m venv /tmp/opensysml-verify && . /tmp/opensysml-verify/bin/activate
pip install opensysml==0.3.0
python -c "import opensysml; print(opensysml.__version__)"    # must print 0.3.0
```

Then check the client end to end against a published core release, since that
is what a user gets:

```bash
export OPENSYSML_GRPC_VERSION=v0.0.5          # a released core tag
python -c "import opensysml; print(opensysml.load('examples/state-machine-demo.sysml').diagnostics)"
```

Finally, read the project page: the description, the license, the project URLs
and the Python versions are the metadata `twine check --strict` accepted, not
metadata anyone reviewed.

### If an upload goes wrong

A PyPI version cannot be replaced. Yank it
(PyPI → project → *Manage* → *Releases* → *Yank*, which hides it from resolvers
without breaking a pin that already names it), bump `VERSION`, and tag again.
Deleting a release frees nothing: the version number stays used.

## Releasing @opensysml/client to npm

The Node client in `clients/node/` is published to npm as `@opensysml/client` by
the `release-node` workflow, which runs on a tag matching `/^client-node-v.*/` —
for example `client-node-v0.1.0`. Nothing is published from a laptop, and a `v*`
core release tag publishes no package. **Nothing has been published yet**: the
first release needs the `@opensysml` scope to exist on npm, an automation token
for it, and the `npm` context below.

### Six packages, one tag

`@opensysml/client` carries no binary. The service binary comes from one of five
per-platform packages it names in `optionalDependencies`, which npm installs by
matching their `os`/`cpu` metadata:

| package | os | cpu |
| --- | --- | --- |
| `@opensysml/sysml-grpc-linux-x64` | linux | x64 |
| `@opensysml/sysml-grpc-linux-arm64` | linux | arm64 |
| `@opensysml/sysml-grpc-darwin-x64` | darwin | x64 |
| `@opensysml/sysml-grpc-darwin-arm64` | darwin | arm64 |
| `@opensysml/sysml-grpc-win32-x64` | win32 | x64 |

All six share the version in `clients/node/package.json`, because the
`optionalDependencies` name that exact version. The platform packages are
published first, so `@opensysml/client` is never on the registry naming a
version of them that is not. Where no package matches — a platform with no
release build — the client falls back to `$OPENSYSML_BINARY`, a binary in
`~/.opensysml/bin/`, a release download into that cache, `sysml-grpc` on
`$PATH`, or an explicit external service. That download is the Python client's:
the same shared cache and metadata, the same pinned digests, and the same
signed-manifest verification, refusing a release it can neither pin nor verify.
See `clients/node/README.md`.

`build-node-binaries` cross-compiles the five binaries from the tagged revision
and writes a `.sha256` sidecar beside each; `npm run platform-packages` refuses
to package a binary whose bytes disagree with its sidecar, or that has none. The
bytes therefore never leave the pipeline that publishes them, which is the job
the Python client's pinned digests do for a download. npm's `--provenance` is not
used: the CLI mints attestations only on GitHub Actions and GitLab CI/CD.

### Why its own tag

The same reason as the Python client: the package resolves a service binary at
run time rather than being lockstep with a core release, so a client-only fix
should not wait for a core release, and a core release should not force an
immutable npm version. An npm version can be deprecated or (within 72 hours,
and only under conditions) unpublished, but never replaced — keeping that off
the re-runnable `v*` path is deliberate.

### What the job needs

1. The `@opensysml` **scope** on npm, with the publishing account a member of it.
2. An **automation** access token for that account (npm → *Access Tokens* →
   *Generate new token* → *Automation*, which bypasses 2FA for CI as a granular
   token restricted to the `@opensysml` scope).
3. A CircleCI **restricted context** named `npm` (Organization Settings →
   Contexts) holding it as `NPM_TOKEN`, restricted to a security group so only
   that group can run a job that reads it. A context reference is matched
   exactly, so the name is lower-case in both places.

### Releasing

```bash
# 1. Bump "version" in clients/node/package.json, land it.
# 2. Tag the version it declares.
git tag client-node-v0.1.0
git push origin client-node-v0.1.0
```

The job fails before publishing anything if the tag and
`clients/node/package.json` disagree, if any of the six versions is already on
the registry, if the Go suite or the client's own gate (`node-test`: build,
typecheck, lint, tests, conformance, and the mutation check that proves the
conformance runner is not vacuous) fails, or if a binary's digest does not match
its sidecar.

### If a publish goes wrong

An npm version cannot be replaced. `npm deprecate '@opensysml/client@0.1.0' '...'`
marks it, `npm unpublish` is possible only within 72 hours and only if nothing
depends on it, and either way the version number stays used: bump
`clients/node/package.json` and tag again.

## The final `pysysml` release

The client was published as [`pysysml`](https://pypi.org/project/pysysml/) up to
0.2.0, before the project was renamed. That name cannot be deleted and its last
version still installs and works, so `pip install pysysml` would otherwise go on
silently handing out a pre-rename client indefinitely.

`packaging/pypi-pysysml/` is the answer: `pysysml` 0.2.1, a distribution of the
same name whose only module raises `ImportError` naming `opensysml`. Being above
0.2.0 is what makes resolvers prefer it. It is not a compatibility shim — it does
not re-export `opensysml`, and it declares no dependency on it, since installing
the new client as a side effect would keep the old import working.

`pip install pysysml==0.2.0` is the escape hatch while migrating. An exact pin is
the only one that avoids the placeholder whatever version it carries: any range
that does not exclude it (`>=0.2`, `~=0.2.0`, `<1.0`) resolves to it, which is
the whole point.

It is released by its own tag, which runs the `release-pysysml-placeholder`
workflow:

```bash
git tag pysysml-v0.2.1    # must match the version in packaging/pypi-pysysml/pyproject.toml
git push origin pysysml-v0.2.1
```

The job resolves the version from the tag, refuses a version PyPI already has,
builds the wheel and sdist, and — the check that matters — installs the wheel
into a clean virtualenv and **fails if importing `pysysml` succeeds**. A
placeholder that imports cleanly is the alias this release exists not to be.
`clients/python/tests/test_legacy_pysysml_placeholder.py` asserts the same contract from
source on every run.

This is expected to happen exactly once. Nothing further should be published
under the old name; a client fix goes to `opensysml`.

## Releasing the Java client to Maven Central

**Nothing has been published, and nothing in CI publishes it.** The Java client
in `clients/java/` builds a complete, signable artifact today, and the steps
below are what a maintainer does once the accounts exist. Until then
`mvn -f clients/java/pom.xml install` is the way to consume it, and the version
is `0.1.0-SNAPSHOT`, which Central refuses by design.

### What a maintainer must obtain first

None of these can be provisioned from a checkout:

1. **A verified namespace.** Register `org.openmbee` at
   [central.sonatype.com](https://central.sonatype.com/) → *Namespaces* → *Add
   Namespace*. A DNS-verified namespace is proved by a TXT record on
   `openmbee.org` that the portal names. It is the `groupId` the client and its
   Java package (`org.openmbee.opensysml`) already declare, and it is in every
   consumer's build file, so every future Java artifact belongs under it.
2. **A published GPG key.** Central requires a detached signature per artifact,
   verified against a public keyserver:

   ```bash
   gpg --quick-generate-key 'Open-MBEE Release Signing <release@openmbee.org>' rsa4096 sign 2y
   gpg --keyserver keys.openpgp.org --send-keys <KEY_ID>
   ```

   The private key and its passphrase belong in a **restricted CircleCI
   context**, as `GPG_PRIVATE_KEY` (ASCII-armoured, `gpg --export-secret-keys
   --armor`) and `GPG_PASSPHRASE`, restricted to a security group the way the
   `PyPI` context is (see [what the job needs](#what-the-job-needs)). A key with
   an expiry needs rotating before it expires; signatures already published stay
   verifiable.
3. **Portal tokens.** Central portal → *View Account* → *Generate User Token*
   gives a username/password pair for a `<server>` with `<id>central`. In CI they
   are `CENTRAL_TOKEN_USERNAME`/`CENTRAL_TOKEN_PASSWORD` in the same context,
   written into `~/.m2/settings.xml` by the job.

### The version, and the tag

`clients/java/pom.xml` declares the version once, and both modules inherit it
from the parent. A release drops `-SNAPSHOT`, lands, and is tagged
`opensysml-java-v<version>` — its own tag, for the reason the Python client has
one: the client does not ship the service, so its version says nothing about
which core release it runs against, and a Maven Central version can never be
replaced, so it must not hang off a `v*` core tag that `ghr -replace` re-runs.

Like the Python client, it downloads a `sysml-grpc` binary at runtime for
whatever release the caller names (`ConnectionOptions.downloadVersion()`,
`$OPENSYSML_GRPC_VERSION`, or `latest`) and verifies it against the digest its
own copy of `clients/release-digests.json` — shipped in the jar as
`release-digests.json` — pins for that release, or, for a release it pins
nothing for, against the digest in the release's signed `SHA256SUMS.txt` (see
[the signed checksum manifest](#the-signed-checksum-manifest)). A core release
published after a client release therefore needs no new client release. The
`dev.sigstore:sigstore-java` dependency is what verifies that bundle, so a
consumer that excludes it can install only pinned releases. See
`clients/java/README.md`.

### What the build already produces

`mvn -f clients/java/pom.xml install` attaches everything Central validates:

- `opensysml-client-<version>.jar`, `-sources.jar` and `-javadoc.jar` (the
  `maven-source-plugin` and `maven-javadoc-plugin` executions are in the default
  build, not the release profile, so a missing one fails long before a release);
- POM metadata Central requires: `name`, `description`, `url`, `licenses`,
  `developers`, `scm`;
- `opensysml-conformance`, which sets `maven.deploy.skip` — it is a test harness,
  not a published artifact.

The `release` profile adds what only a release needs: `maven-gpg-plugin` signing
at `verify`, and `central-publishing-maven-plugin` with **`autoPublish=false`**,
so `mvn deploy` uploads a deployment that then sits in the portal until a human
releases it. That is the equivalent of the staging repository the old OSSRH
workflow had, and it is deliberate: the publication is the irreversible step.

### The procedure

```bash
# 1. Drop -SNAPSHOT in clients/java/pom.xml, land it, then from that commit:
make build                                            # the service the tests start
mvn -f clients/java/pom.xml clean verify              # tests, javadoc, sources
mvn -f clients/java/pom.xml -Prelease verify          # + signatures, no upload
gpg --verify clients/java/opensysml-client/target/*.jar.asc   # check one by hand

# 2. Upload a deployment (still not published):
mvn -f clients/java/pom.xml -Prelease deploy -pl opensysml-client

# 3. central.sonatype.com → Deployments → review the validation report →
#    "Publish". Availability on Maven Central follows within ~30 minutes,
#    search indexing later.

# 4. Tag what was published, and bump to the next -SNAPSHOT.
git tag opensysml-java-v0.1.0 && git push origin opensysml-java-v0.1.0
```

A deployment that fails validation can be dropped from the portal and re-uploaded
under the same version; one that has been **published** cannot be replaced or
deleted, so a mistake needs a new version.

### A CI job, when there is something to publish

There is no `publish-maven` job yet, and adding one before the namespace exists
would be a job that can only fail. When it is added it should mirror
`publish-pypi`: triggered by `/^opensysml-java-v.*/` only, running on a restricted
context, refusing to run when a variable it needs is absent rather than letting
Central answer with a 401, checking the tag names the declared version, and
stopping at an unpublished deployment. `java-test` already runs the client's
tests and the conformance suite on every commit.

## Releasing the Rust client to crates.io

Nothing has been published, and the first publish is a decision rather than a
step: `opensysml` is a common enough name that its availability on crates.io must
be checked before the crate is promised anywhere, and a name taken means renaming
the crate rather than the client. `clients/rust/README.md` documents the path and Git
dependency forms that work today.

What the crate is ready for, and what it is not:

- `cargo package -p opensysml` must succeed cleanly before any publish — it is
  what proves the manifest carries the metadata crates.io requires and that the
  packaged file list builds on its own, outside this workspace.
- The manifest declares `license`, `description`, `repository`, `homepage`,
  `documentation`, `keywords`, `categories` and `rust-version = "1.83"`, so a
  published crate documents its own minimum supported Rust version.
- `opensysml-conformance` is a workspace member and a runner, not a library, and
  is **not** published: it reads `conformance/scenarios` from this repository.
- The client downloads a `sysml-grpc` release binary when `$OPENSYSML_GRPC_VERSION`
  asks for one, and verifies it against `clients/rust/opensysml/release-digests.json`,
  which the crate embeds with `include_str!` and its `include` list ships — so a
  release whose digests are not in the published crate is refused rather than
  installed. Unlike the Python client it does not verify the signed
  `SHA256SUMS.txt` manifest, so publishing a release also means shipping a crate
  version that pins it if Rust callers are to install it; see
  `clients/rust/README.md`.

The procedure, once the name is settled:

```bash
# 1. Bump "version" in clients/rust/opensysml/Cargo.toml, land it, then from that commit:
cargo package -p opensysml --manifest-path clients/rust/Cargo.toml   # must be clean
cargo publish -p opensysml --manifest-path clients/rust/Cargo.toml   # maintainer, with a crates.io token

# 2. Tag what was published.
git tag opensysml-rust-v0.1.0 && git push origin opensysml-rust-v0.1.0
```

**`cargo publish` is a maintainer action and CI never runs it.** A crates.io
version cannot be replaced or deleted, only yanked (`cargo yank --version 0.1.0`,
which stops new resolutions and leaves existing lockfiles working), so a mistake
needs a new version. `rust-test` already runs the client's tests, lints and the
conformance suite on every commit that touches it.
