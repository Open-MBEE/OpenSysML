# Releasing OpenSysML

A release is cut by pushing a `v*` tag. Everything after that is CircleCI: the
`release` workflow runs the test suite, cross-compiles `sysml`, `sysml-lsp` and
`sysml-grpc` for five platforms, builds the combined `sysml-wasm` module and the
Python client's wheel and sdist; release assets go to GitHub and the Python
package to PyPI. Nothing is
published from a laptop.

The Python client is released in lockstep with the core: the same `v<version>` tag
publishes `opensysml` `<version>` to PyPI, and the workflow refuses to build anything
unless `client/python/opensysml/_version.py` declares that version — see
[Releasing opensysml to PyPI](#releasing-opensysml-to-pypi). A caller who pins one
version therefore gets the package and the `sysml-grpc` binary that were tested
together.

The same `v*` tag also publishes the Node client to npm as `@openmbee/opensysml`
at the same version — see
[Releasing @openmbee/opensysml to npm](#releasing-openmbeeopensysml-to-npm) — and
the Java client to Maven Central as `org.openmbee:opensysml` — see
[Releasing the Java client to Maven Central](#releasing-the-java-client-to-maven-central) —
and the Rust client to crates.io as `opensysml`, all at the same version — see
[Releasing the Rust client to crates.io](#releasing-the-rust-client-to-cratesio).
No client keeps a tag of its own any more.

A second tag, `pysysml-v*`, publishes the one-off final release of the client's
pre-rename PyPI name — see [The final `pysysml` release](#the-final-pysysml-release).

The public Go API in `client/opensysml` has no release of its
own: it is part of this module, so the core's `v*` tag is what a Go program pins.

Between releases, `.github/workflows/nightly.yml` builds the newest green `develop`
commit every night with `scripts/build-release-artifacts.sh` — the same targets,
platforms and layout as `build-release` below — together with the Python wheel and the
seven npm packages, and publishes it as the prerelease `nightly-<yyyymmdd>-<commit>`,
kept for 14 days, with the moving alias `nightly` at the newest night; both are never
marked latest and are signed by the workflow's own GitHub identity rather than the
CircleCI one the clients pin. The clients go to PyPI and npm as development versions
(`<next>.dev<yyyymmdd>`; `<next>-nightly.<yyyymmdd>.g<commit>` under the `nightly`
dist-tag) through trusted publishing, so no default install and no `latest` moves. It
touches nothing described on this page: the `nightly*` tags match neither the `v*` filter
of the `release` workflow nor the Windows signing workflow, `releases/latest` keeps
resolving to the stable line, and the stable client publication below stays with
CircleCI and its tokens. [Nightly snapshots](nightly.md) documents it for a user, with the
one-time trusted-publisher setup; when `build-release` or the client packaging changes
what it produces, change the nightly workflow so the two stay the same.

## Before tagging

Run the full gate on the commit you intend to tag:

```bash
gofmt -l .            # must print nothing
go build ./...
go vet ./...
make lint             # staticcheck + gosec, as CircleCI runs
go test -race -count=1 ./...
go test -run TestStdlibConformance ./internal/workspace/libs
```

Run the Python client the way CircleCI's `python-test` job does, since a release
gates on the same suite:

```bash
make build-grpc && mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/
pip install -e client/python/ && pip install pytest pytest-mock psutil
pytest client/python/tests/ -v
```

The OMG training-corpus gate skips while the corpus is absent, so fetch it and
run it explicitly — the expected result is the pinned baseline, currently
100/100 files clean:

```bash
./scripts/download-training-examples.sh
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 go test -count=1 ./tests/corpus -run TestTrainingExamples
```

A change in that count is a finding to adjudicate file by file, never a
baseline to regenerate.

Then check the release-facing text:

- The changelog fragments under `changes/unreleased/` are folded into a dated entry for this
  version: `python3 scripts/changelog.py release X.Y.Z` (or `--date YYYY-MM-DD`) appends them
  to the `## Unreleased` section, renames it, and deletes the fragments. Commit `CHANGELOG.md`
  and the deletions together. The previous version's entry stays unchanged. Whether `X.Y.Z`
  bumps the patch or the minor segment is decided by model compatibility, as
  [CONTRIBUTING.md § Versioning](../../CONTRIBUTING.md#versioning) states.
- `README.md` and `docs/guide/` transcripts match what the binary prints.
  Build it (`make build-sysml`) and paste a few commands through it.
- `python3 scripts/check-doc-links.py` reports no broken link (CI gates on it too).
- Test counts match a real run in the two surfaces that type them at a release
  (`docs/project/roadmap.md`'s gate table and `docs/project/training-examples.md`; the compliance
  map's test inventory is counted from the tree when the site is built, and everything else links
  to it, per CONTRIBUTING.md), and no compliance row claims more than the implementation does. Count first-level subtests:
  a case that registers sub-subtests, like `variant_connection_per_owner`, otherwise counts twice.

### Rehearsing the release

The `release` workflow also runs on a branch — `release/X.Y.Z` — when the
boolean pipeline parameter `release_rehearsal` is true. Every job runs with
every pre-upload check live: the suite, the version lockstep, the registry
availability checks, the credential checks, the npm `whoami`, the GitHub and
Central token probes, the GPG key import and test-sign, the npm packs,
`cargo package` and the Maven build-and-sign. Only the irreversible commands
are skipped: cosign keyless signing and attestation (which write to the public
Rekor log), `ghr` (the GitHub release), `twine upload`, `npm publish`,
`mvn deploy` and `cargo publish`. A rehearsal exports a stand-in `CIRCLE_TAG`
from `_version.py` before any step reads it, so the version bumps must already
be on the branch.

Trigger it from the CircleCI UI with *Trigger pipeline*: choose the release
branch for both the config and the checkout source, then add the boolean
parameter `release_rehearsal` = true. Or by API:

```bash
curl -X POST "https://circleci.com/api/v2/project/<project-slug>/pipeline/run" \
  -H "Circle-Token: $CCI_TOKEN" -H 'Content-Type: application/json' \
  -d '{"definition_id": "<pipeline definition id>",
       "config": {"branch": "release/X.Y.Z"},
       "checkout": {"branch": "release/X.Y.Z"},
       "parameters": {"release_rehearsal": true}}'
```

The project slug and the pipeline definition id are under *Project Settings →
Project Setup*. Whoever triggers it must be authorized for all four contexts
(`PyPI`, `npm`, `Maven Central`, `crates.io`), because the same jobs run with
the same contexts. The built binaries are kept only as the pipeline's CircleCI
artifacts; nothing reaches a registry or a GitHub release.

A green rehearsal proves the tag will not fail on builds, tests, version
lockstep, registry availability, credential presence, npm/Central/GitHub
token auth, the GPG key and passphrase with real Maven signing, npm packing,
`cargo package`, or the release-digest stamps and the assertions that the
wheel, sdist and crate pin the rehearsed tag's binaries. It cannot prove the PyPI and crates.io token validity (no
read-only check exists for either), cosign keyless signing (skipped because it
writes to Rekor), the uploads themselves, Central publish permission beyond
token auth, or a version someone publishes between the rehearsal and the tag.

## The release branch

Day-to-day work merges into `develop`; `main` carries releases only (see
[CONTRIBUTING.md § Branches](../../CONTRIBUTING.md#branches)). A release is a
branch that moves the integration state onto `main`:

1. Cut `release/x.y.z` from `develop`:

   ```bash
   git checkout develop && git pull
   git checkout -b release/0.0.5
   ```

2. Fold the changelog fragments on that branch — `python3 scripts/changelog.py release 0.0.5`,
   as [Before tagging](#before-tagging) describes — and commit `CHANGELOG.md` together with the
   deleted fragments. Set `VERSION` in `client/python/opensysml/_version.py` to `x.y.z` as
   well: the tag publishes `opensysml` at the core version, and the release workflow fails
   before building anything when the two disagree (see
   [Releasing opensysml to PyPI](#releasing-opensysml-to-pypi)). Also set `"version"` in
   `client/node/package.json` — the five platform packages in
   `optionalDependencies` and `@openmbee/opensysml-wasm` in `peerDependencies` —
   to the SemVer spelling of the same version (`0.9.1`;
   `0.9.0-rc.1` for `0.9.0rc1`), and run `npm install --package-lock-only` in
   `client/node` so the lockfile agrees; the release workflow fails before
   building anything when package.json disagrees. `client/java/pom.xml` follows
   the same version too: set the parent pom's `<version>`, both modules'
   `<parent><version>`, and the client version in `editors/mdk/pom.xml`
   (`opensysml.client.version`) and `editors/syson/backend/pom.xml` to the same
   spelling as package.json. `client/rust/opensysml/Cargo.toml` follows too:
   set `[package] version` to the same spelling and run
   `cargo update -p opensysml` in `client/rust` so the lockfile
   agrees. The editors carry the same spelling too, though nothing publishes them:
   `"version"` in `editors/vscode/package.json` and
   `editors/syson/frontend/package.json`, each lock regenerated with
   `npm install --package-lock-only` in that directory; `<version>` in
   `editors/mdk/pom.xml` and `editors/syson/pom.xml`; and `<parent><version>`
   in their child poms (`editors/mdk/plugin`, `editors/mdk/tools`,
   `editors/mdk/openapi-stubs`, `editors/mdk/dist`, `editors/syson/backend`
   and `editors/syson/syson-api-stubs`). `check_version.py --editors` in
   `build-python-package` fails the release early when any of them disagrees.
   Anything else the release
   needs (a doc that names the version) lands here too; a feature does not. Check the wire compatibility
   against the released schema, not the branch's own source:
   `make proto-breaking BUF_BREAKING_REF=origin/main` (the default baseline is
   `origin/develop`; the pull-request workflow uses the base branch, so the PR to `main`
   makes the same comparison).

3. Open a pull request from `release/x.y.z` to `main` and merge it once the
   pull-request workflow is green. Merging into `main` runs CircleCI's
   `build-test` workflow over the merged tree.

4. Tag `main` as [Tagging](#tagging) describes.

5. Merge the `chore/pin-vx.y.z` pull request the tag's pipeline opens against
   `develop` (see [Pinned release digests](#pinned-release-digests)), then merge
   `main` back into `develop` — a plain merge, no rebase — so the folded
   changelog, and any hotfix that landed on `main` in the meantime, flow down:

   ```bash
   git checkout develop && git pull
   git merge main
   git push origin develop
   ```

A `hotfix/` branch follows the same path from `main`: cut from `main`, pull
request to `main` (`make proto-breaking BUF_BREAKING_REF=origin/main` locally, as above),
tag, merge back into `develop`.

## Tagging

The tag is the version: CircleCI passes `CIRCLE_TAG` to the build as
`VERSION`, so `sysml --version` reports it. Rehearse first — see
[Rehearsing the release](#rehearsing-the-release).

```bash
git checkout main && git pull
git tag -a v0.0.5 -m "v0.0.5"
git push origin v0.0.5
```

The tag belongs on `Open-MBEE/OpenSysML`, the repository the releases live on
and where development happens: every release from v0.0.1 on is tagged on its
`main`. The clients resolve releases from that repository
(`DEFAULT_GITHUB_REPO` in `client/python/opensysml/binary.py`), so a tag pushed
to a fork builds a release nobody consumes.

Tags are matched by `/^v.*/` in `.circleci/config.yml`. A tag on a commit that
fails the suite fails the release workflow before anything is published, and so
does a tag whose version `client/python/opensysml/_version.py`,
`client/node/package.json`, `client/java/pom.xml` or
`client/rust/opensysml/Cargo.toml` or an editor manifest does not declare.

## What CircleCI publishes

`build-release-binaries` builds the Go binaries for every platform into `dist/`,
and `build-release` assembles the release from them and the Python distribution,
so that `dist/` holds:

- per-binary archives — `sysml-<os>-<arch>.tar.gz`,
  `sysml-lsp-<os>-<arch>.tar.gz` (`.zip` on Windows);
- bundle archives — `opensysml-<os>-<arch>.tar.gz` holding both binaries under
  their plain names, which is the layout Homebrew and a PATH install expect,
  plus their section 1 manual pages under `share/man/man1` (the Unix archives
  only; the Windows bundle has no use for them, and `sysml-grpc.1` stays out
  of a bundle that does not carry `sysml-grpc`);
- `sysml-grpc-<os>-<arch>`, published raw with a `.sha256` sidecar rather than
  archived, because that is what `opensysml` downloads and verifies
  (`client/python/opensysml/binary.py`) when it starts the service for a Python caller;
- `sysml-jupyter-kernel-<os>-<arch>`, the Jupyter kernel, published raw with a
  `.sha256` sidecar for the same reason: `jupyter-opensysml-kernel`'s sdist downloads
  and verifies it (`client/jupyter-kernel/jupyter_opensysml_kernel/binary.py`) when it
  registers the kernelspec, and its platform wheels bundle it;
- `wasm/sysml-wasm.wasm` and `wasm/wasm_exec.js`, the combined WebAssembly
  module and matching Go runtime, each with a `.sha256` sidecar;
- the Python client's distribution, `opensysml-<x.y.z>-py3-none-any.whl` and
  `opensysml-<x.y.z>.tar.gz`, built by `build-python-package` from those
  `sysml-grpc` binaries' digests and the same files `publish-pypi` uploads (see
  [Releasing opensysml to PyPI](#releasing-opensysml-to-pypi));
- the Jupyter kernel's distributions, `jupyter_opensysml_kernel-<x.y.z>.tar.gz` and one
  `jupyter_opensysml_kernel-<x.y.z>-py3-none-<platform>.whl` per released platform, built
  by `build-jupyter-kernel-package` from the `sysml-jupyter-kernel` binaries and their
  digests (see
  [Releasing jupyter-opensysml-kernel to PyPI](#releasing-jupyter-opensysml-kernel-to-pypi));
- `SHA256SUMS.txt` over every archive, every wheel and sdist, every `sysml-grpc` and
  `sysml-jupyter-kernel` binary and both WebAssembly assets,
  with its cosign signature `SHA256SUMS.txt.bundle` (see
  [The signed checksum manifest](#the-signed-checksum-manifest));
- `provenance.intoto.json`, the SLSA provenance statement naming every artifact
  the manifest lists, and `provenance.intoto.json.bundle`, its cosign
  attestation (see [The release provenance](#the-release-provenance)).

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64.

The two jobs share one workspace: `build-release-binaries` persists the whole
`dist/` tree, the binaries' `.sha256` sidecars included, and `build-release`
persists only what it added to it — the manifest, its signature and provenance
bundles, the WebAssembly sidecars and the Python distributions. Workspace layers are additive, and a path persisted by two
upstream jobs fails the attach in every job downstream of both, which is every
publish job.

Before any of it is stored or published, `build-release-binaries` runs each host-platform
binary and fails the release unless `--version` reports `CIRCLE_TAG`. The ldflags
are the only thing stamping the tag into a binary, and a binary reporting `dev`
or a stale tag looks the same on the release page as a correct one — that is how
an artifact whose version disagreed with its tag reached a release once already.
The check runs the linux/amd64 builds; the cross-compiled ones cannot run on the
executor, so each is checked for the tag string the ldflags write into it. The
wheel and sdist are checked by name: both carry the version in their file name,
and `build-python-package` has already imported the wheel and compared
`opensysml.__version__` to the tag.

`publish-github-release` uploads them with `ghr`, using a token from
`GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project settings.
It runs with `-replace`, so re-running the workflow for the same tag replaces
that release's assets rather than appending duplicates, and leaves everything
else on the release alone: notes, title and the prerelease/latest flags survive.
A tag that has no release yet still gets one created.

`publish-pypi` runs after it, off the same built artifacts, and is the one step
of a release that cannot be repeated: PyPI never accepts a version twice, so on a
re-run of a published tag it fails by design while the GitHub assets are replaced
(see [What the jobs do, in order](#what-the-jobs-do-in-order)). Running it after
the GitHub release means the package version never exists without the release
it names; if the GitHub upload fails, nothing irreversible has happened yet.

`publish-npm` runs beside it, also after the GitHub release and also not
repeatable: npm never accepts a version twice. It publishes
`@openmbee/opensysml-wasm` from `dist/wasm` and the five
`@openmbee/opensysml-sysml-grpc-<os>-<cpu>` platform packages built from
`build-release`'s release assets, then the `@openmbee/opensysml` client (see
[Releasing @openmbee/opensysml to npm](#releasing-openmbeeopensysml-to-npm)).

`publish-maven` runs beside them, in the same position and with the same
one-way property: a Central version can never be replaced. It signs, uploads
and publishes `org.openmbee:opensysml` and its `opensysml-parent` pom,
waiting until Central reports the deployment published (see
[Releasing the Java client to Maven Central](#releasing-the-java-client-to-maven-central)).

`publish-crates` runs beside them, in the same position and with the same
one-way property: a crates.io version cannot be replaced, only yanked. It
packages and publishes `opensysml` (see
[Releasing the Rust client to crates.io](#releasing-the-rust-client-to-cratesio)).

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

   The provenance is checked the same way, against the downloaded archive
   rather than the manifest:

   ```bash
   curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.5/provenance.intoto.json.bundle
   cosign verify-blob-attestation opensysml-linux-amd64.tar.gz \
     --bundle provenance.intoto.json.bundle --type slsaprovenance1 \
     --certificate-oidc-issuer https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62 \
     --certificate-identity-regexp '^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f-]+$'
   ```

   Then install the Python client the release published, from the index rather
   than the source tree, and run it against the release's own `sysml-grpc` — the
   pairing a user who pins one version gets (see
   [Verifying an upload](#verifying-an-upload)):

   ```bash
   python -m venv /tmp/opensysml-verify && . /tmp/opensysml-verify/bin/activate
   pip install opensysml==0.0.5
   OPENSYSML_GRPC_VERSION=v0.0.5 python -c \
     "import opensysml; print(opensysml.__version__, opensysml.load('examples/state-machine-demo.sysml').diagnostics)"
   ```

2. **Verify the npm upload.** Check the registry sees all seven packages at the
   version and the right dist-tag:

   ```bash
   npm view @openmbee/opensysml@0.0.5 version dist-tags
   npm view @openmbee/opensysml-wasm@0.0.5 version
   ```

   Then install it in a temp dir and load a model with `OPENSYSML_BINARY` unset,
   so the per-platform package is what supplies the binary:

   ```bash
   mkdir /tmp/npm-verify && cd /tmp/npm-verify && npm init -y
   npm install @openmbee/opensysml@0.0.5
   node --input-type=module -e "
     import { loads } from '@openmbee/opensysml';
     const model = await loads('package V { part def P; }');
     console.log(model.diagnostics);
     await model.close();"
   ```

3. **Verify the Maven Central upload.** Central can take up to ~30 minutes to
   answer (search indexing later), so check the pom's URL:

   ```bash
   curl -sI https://repo1.maven.org/maven2/org/openmbee/opensysml/0.0.5/opensysml-0.0.5.pom
   ```

   Then a consumption check resolves it the way a consumer does:

   ```bash
   mvn dependency:get -Dartifact=org.openmbee:opensysml:0.0.5
   ```

4. **Verify the crates.io upload.** Check the API sees the version:

   ```bash
   curl -s -H 'User-Agent: OpenSysML release (https://github.com/Open-MBEE/OpenSysML)' \
     https://crates.io/api/v1/crates/opensysml/0.0.5
   ```

   Then a consumption check resolves it the way a consumer does, in a throwaway
   crate:

   ```bash
   cargo new /tmp/crates-verify && cd /tmp/crates-verify
   cargo add opensysml@=0.0.5 && cargo fetch
   ```

5. **Verify the pin pull request landed.** `pin-release-digests`, the last job
   of the tag's workflow, stamps the release into `client/release-digests.json`
   from the manifest, syncs every client copy, and opens
   `chore/pin-vx.y.z` against `develop`. Merge it (step 5 of
   [The release branch](#the-release-branch) does so before the back-merge). A
   missing pull request means the job failed — read its log; the token is the
   usual cause, see [Pinned release digests](#pinned-release-digests) — and
   until the pin lands, `tests/hygiene` fails the next release branch rather
   than this one.

6. **Let the Homebrew tap pick the release up.** The tap repository
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

7. **Say what is not signed.** macOS binaries are not Developer ID signed or
   notarized, so a browser download trips Gatekeeper. Point release notes at
   [MACOS_DISTRIBUTION.md](macos-distribution.md), which gives the workarounds
   and what signing would take. Windows binaries are Authenticode signed through
   SignPath Foundation once the application below is approved; until then, and
   for a release whose signing request nobody approved, only the unsigned
   Windows assets exist and SmartScreen warns — say so in the notes.

8. **Approve the Windows signing request.** When SignPath is configured, the
   tag also runs [`release-windows.yml`](../../.github/workflows/release-windows.yml),
   which parks a signing request in SignPath until an Approver approves it
   (see [Windows Authenticode signing](#windows-authenticode-signing)). No
   approval, no `*-signed*` assets on the release.

9. **Check the Windows installer landed.** The same workflow builds the MSI
   (see [The Windows installer](#the-windows-installer)) once CircleCI has
   published the release: `opensysml-<x.y.z>-windows-amd64.msi` with
   `SHA256SUMS-windows-msi.txt` when SignPath is not configured, or
   `opensysml-<x.y.z>-windows-amd64-signed.msi` listed in
   `SHA256SUMS-windows-signed.txt` when it is. A release with neither means the
   workflow failed (WiX, the Z3 download, ICE validation or the signing
   request); re-run it after fixing the cause — see the recovery path in
   [The Windows installer](#the-windows-installer).

10. **Render the Windows package-manager manifests** when a maintainer wants
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
`client/python/opensysml/signing.py` and in `client/node/src/node/signing.ts`
to narrow the pin to the one pipeline.

Anything short of a verified manifest is refused exactly as an unpinned release
is today: no bundle asset, a bundle that does not verify, another signer, a
manifest changed after signing, an expired certificate, or `sigstore` not
installed. The `.sha256` served beside a binary is still never a reason to trust
it — same origin as the binary — and remains behind
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

### The release provenance

`build-release` also writes a [SLSA provenance](https://slsa.dev/spec/v1.0/provenance)
statement over the same artifacts and signs it under the same identity.
`scripts/release-provenance.py` reads `dist/SHA256SUMS.txt` and writes
`dist/provenance.intoto.json`: an in-toto Statement v1 whose subjects are every
artifact the manifest lists, with the manifest's digest, and whose predicate
(`https://slsa.dev/provenance/v1`) records what built them — the repository
and tag (`externalParameters`), the commit the tag resolved to
(`resolvedDependencies`), the CircleCI organization, project and workflow
(`internalParameters`), the project as the builder (`runDetails.builder.id`)
and the job's URL as the invocation. The build type,
`https://github.com/Open-MBEE/OpenSysML/.circleci/build-release/v1`, names this
repository's own job; its version moves when what the job does changes. The
script refuses to write a statement from an empty or malformed manifest, or
without every one of the CircleCI variables it describes the build from, so a
vaguer statement is never published in place of the intended one.

`cosign attest-blob --statement` then signs that statement as it stands — every
subject kept, nothing re-derived — into a DSSE envelope in a sigstore bundle,
`provenance.intoto.json.bundle`, keylessly under the job's CircleCI OIDC
identity, exactly as the manifest is signed. The job verifies its own
attestation against three published artifacts (a bundle archive, a `sysml-grpc`
binary and the wheel) under the identity the clients pin, and checks that the
subjects are the manifest's lines, no more and no fewer, before anything is
stored; `publish-github-release` uploads the statement and the bundle beside
`SHA256SUMS.txt`.

What this is, and is not. The statement is produced by the build that produced
the artifacts, on CircleCI's hosted runners, and signed with an identity only
that pipeline can hold, so a verifier learns which repository, tag and commit a
downloaded file was built from and which job built it — SLSA Build L2. It is not
Build L3: CircleCI does not itself issue provenance, so the statement is
generated by the job it describes rather than by the platform outside it, and
nothing stops a change to `.circleci/config.yml` from changing what is written.
That is why the buildType is versioned and why the trust anchor stays the
certificate identity: a statement signed by anything but this project's
pipeline verifies as nothing. A provenance workflow that hashes downloaded
assets on another platform would attest that platform's download, not this
build, and is not what this is.

The unsigned `provenance.intoto.json` is a convenience for reading; the
authoritative statement is the bundle's payload:

```bash
jq -r '.dsseEnvelope.payload' provenance.intoto.json.bundle | base64 -d | jq .
```

The Python and Node clients keep reading the signed manifest, not the
provenance; nothing in them changes.

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
`OriginalFilename`), and the Makefile's `build-sysml`, `build-lsp`,
`build-grpc` and `build-jupyter-kernel` targets run `go-winres` (pinned by `GO_WINRES_VERSION`, a build
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
  `workflow_dispatch` (which publishes only when its `tag` input names one).
- **SignPath configured:** the `msi-signed` job rebuilds the MSI from the
  SignPath-signed executables, validates it, submits the MSI itself to SignPath
  under `SIGNPATH_MSI_ARTIFACT_CONFIGURATION_SLUG`, and `publish-signed`
  uploads it as `opensysml-<x.y.z>-windows-amd64-signed.msi`, listed in
  `SHA256SUMS-windows-signed.txt` with the other `-signed` assets. The unsigned
  MSI is then kept only as a workflow artifact, so a release never carries two
  installers whose contents differ only by signature.

**Re-running it for an existing tag.** When the workflow failed on a tag —
including at `git checkout`, since the Windows runners cannot check out a
tracked path Windows forbids — fix the cause on a `hotfix/` branch to `main`,
then re-run it against the tag with `gh workflow run release-windows.yml
--ref main -f tag=v0.9.1`. The dispatch builds the tagged commit on the Linux
job, and the Windows MSI jobs check nothing out — they consume `packaging/msi`,
`scripts/build-msi.sh` and `LICENSE`, uploaded by that job as a workflow
artifact — so even a tag whose tree Windows cannot check out gets its
installer. The release gate still requires the tag to resolve to the built
commit, and `overwrite_files` touches only the MSI assets, so the
CircleCI-published release and its assets are never rewritten.

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

The committed copies of `client/release-digests.json` stay in sync and still
cover releases published before signing existed. A pin remains an override:
where a client has a pin, it wins, and clients that verify signed manifests
refuse a disagreement rather than downgrade. **Per release there is nothing to
do by hand**: the tag's pipeline opens the pull request that pins it (below).
Pinning by hand still works, for a release the table lacks:

```bash
export GITHUB_TOKEN=...   # must be able to read this repository's releases
python client/python/scripts/pin_release_checksums.py --version v0.0.8 --write
```

The token is required, not an optimization: the script reads the release's assets
through the GitHub releases API, and unauthenticated calls are rate-limited per
address and fail as an opaque HTTP 403. `GH_TOKEN` is read as well. The scope
needed is read access to this repository's releases — `public_repo` for a classic
token, `Contents: read` for a fine-grained one; nothing is written through the
API. Without either variable the script fails immediately with
`MissingTokenError` naming the variable, rather than at the first request.

Each client's package ships the pins of its own release, stamped at release
time by `client/python/scripts/pin_release_checksums.py` and never committed to
the synced client tables. `build-python-package` runs it with
`--from-binaries dist/grpc` — hashing the `sysml-grpc` binaries
`build-release-binaries` built, before `SHA256SUMS.txt` exists — against the
copy of the table the wheel and sdist package, so `pip install opensysml==X.Y.Z`
verifies the service it downloads against a digest inside the wheel, with no
environment variable and no `sigstore` at run time. `publish-npm` and
`publish-maven` run the same `--from-binaries dist/grpc` stamp against
`client/node/release-digests.json` and
`client/java/opensysml-client/src/main/resources/release-digests.json`, the
copies the npm tarball and the jar package, so `npm install
@openmbee/opensysml@X.Y.Z` and `org.openmbee:opensysml:X.Y.Z` download their own
release on a pin, with the optional sigstore dependencies absent. `publish-crates`
stamps the crate the same way with `--from-manifest dist/SHA256SUMS.txt`, once
the signed manifest exists. Every one of these jobs then fails unless the
packaged table pins all five `sysml-grpc-*` assets for the tag — opening the
wheel and sdist, the packed tarball, the jar and the `.crate` to read the table
they carry — and `build-release` fails before signing unless the wheel's pins
are the digests the manifest lists. The signed manifest is therefore what a
client reaches for only for another release, or when it is older than the
release it is asked for.

Stamping with an explicit `--table` touches that one file in the job's working
copy and nothing else: the shared `client/release-digests.json` is not written,
no copy is synced, and nothing is committed, so the `sync-release-digests.py
--check` step elsewhere in the same pipeline, which reads the committed tree,
is unaffected. `tests/hygiene/release_config_test.go` holds all four publishing
jobs to this order — stamp, package, assert — and the Node and Java suites
assert the table they load pins the tag `$OPENSYSML_EXPECT_PINNED_RELEASE`
names, which only those two jobs set, after stamping.

The committed table is brought up to date by the release itself. After
`publish-github-release`, the `pin-release-digests` job checks out `develop`,
runs `pin_release_checksums.py --from-manifest dist/SHA256SUMS.txt` against the
default table — the shared `client/release-digests.json`, so every client copy
is synced in the same stamp — confirms `sync-release-digests.py --check`, and
opens `chore/pin-vX.Y.Z` against `develop`. A client *built from the next
revision* — a checkout, or a package of a later release asked for this one —
then pins it too rather than reaching for the signed manifest. The job is
idempotent: when `develop` already pins the tag it ends with nothing to open,
the branch is rebuilt from `develop` on every run (a run repeated after a
failure replaces the earlier branch), and an open pull request for the branch
is reused rather than duplicated. A pin already on `develop` that disagrees with
the manifest fails the job, as the script refuses to replace a pin.

The job pushes and opens the pull request with the token
`publish-github-release` publishes the release with — `GITHUB_TOKEN`, else
`GH_TOKEN`, else `CIRCLE_TOKEN`, a project environment variable rather than a
context. Publishing a release needs that token to write this repository's
contents, and pushing the branch needs no more; opening the pull request also
needs it to write pull requests. A classic token's `repo` scope covers both; a
fine-grained token needs `Contents: read and write` and `Pull requests: read
and write` on this repository. A token lacking the latter fails the job at the
pull request with HTTP 403 or 404 after the branch is pushed; grant the scope
and re-run the job, which reuses the branch.

`tests/hygiene` holds the table to this: every release `CHANGELOG.md` records
is pinned for all five `sysml-grpc-*` assets, except the release the tree
itself declares in `_version.py` until its tag exists (its digests are built by
that tag), and the releases listed in the test as unpinnable — `v0.0.4`, which
predates the service binaries, and `v0.0.9`, `v0.1.1` and `v0.1.2`, which
predate signing and have no bundle to verify a manifest with. The changelog,
not git tags, is the list of releases, since a shallow clone has none; so a
release branch that bumps the version fails this test until the previous
release's pin pull request has landed. The committed table covers every signed
release through `v0.9.2`.

## The SonarCloud scan

Not a release step — the `scan` job runs in the `build-test` workflow on every
commit, after `go-static`, `go-coverage-merge`, `go-gates`, `python-test`,
`java-test` and `node-test`. It does not wait on `go-race-test`: a race-run
failure used to hide the scan entirely, and nothing the race run produces
reaches the analysis — but it is documented here with the other CircleCI
credential plumbing.

It waits on the three client jobs because each writes a coverage report the
scan reads: a language whose report is absent has every one of its lines
counted as uncovered, which is what dropped new-code coverage to 54.8% on the
0.4.0 analysis while the suites themselves were passing.

The job references the organization context named exactly `SonarCloud`, which
supplies `SONAR_TOKEN` (the same context `Open-MBEE/flexo-mms-layer1-service`
uses, so no new credential is provisioned). It reads
`sonar-project.properties` at the repository root and four coverage reports
persisted to the workspace — `coverage.txt` from `go-coverage-merge`, which joins
`go-coverage`'s four package shards,
`coverage-python.xml` from `python-test`, `coverage-node.lcov` from
`node-test`, and JaCoCo's `jacoco.xml` per Java module — and it un-shallows the
clone because SonarCloud needs full history for blame and new-code detection.

The Go profile is written with `-coverpkg=./...` so a package is credited for
the code it exercises elsewhere; without it `internal/syntax/ast/dump.go`
measures 21% though the parser's golden tests run 90% of it.

`java-test` also persists each module's `target/classes`, `target/test-classes`
and a `target/dependency` directory it fills with `dependency:copy-dependencies`
(the Maven repository itself is that job's cache, not the workspace). The Java
sensor resolves types from those, and without them it warns about missing
`sonar.java.binaries`/`sonar.java.libraries` and degrades to a syntactic
analysis, so the `scan` job fails if any of the six directories is empty.
`sonar.python.version` names the range `client/python/pyproject.toml` declares,
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

The job runs on a `large` container — 8 GB, the largest class in the plan —
whose memory is split between three processes. `SONAR_SCANNER_JAVA_OPTS:
-Xmx5500m` sizes the forked analysis JVM the sonar-scanner-cli 8 launcher
starts: Sonar's Go sensor parses one directory at a time and holds that
directory's parser output in memory, so a large package
(`internal/exec/runtime`) exhausted the scanner's default heap with
`java.lang.OutOfMemoryError` before it was raised. `SONAR_SCANNER_OPTS:
-Xmx256m` sizes the launcher itself and carries `-D` properties such as the
project version, and `sonar.javascript.node.maxspace=1024` caps the Node
process the JS/TS sensor spawns, whose default 2.2 GB the ~60 TypeScript
files do not need.

A scan that dies with `EXECUTION FAILURE` and exit 3, with no Java exception
in its log, is the container's OOM-killer, not the analysis — an analysis
heap near the container size plus an uncapped Node heap has done this at the
moment the JS/TS sensor starts its Node process. A `when: always` step right
after the scan prints the cgroup memory counters, where an OOM kill shows as
`oom_kill 1` rather than being guessed at.

The scan step itself reproduces what the `sonarsource/sonarcloud` orb did —
download the pinned sonar-scanner-cli 8.0.1.6346 into a cache keyed on the
version, `chmod` the launcher and its JRE — but inline, so the scanner runs
through a one-retry wrapper: a log line matching a transient SonarCloud API
or network failure (HTTP 5xx, JRE-metadata query failure, timeouts, resets)
sleeps 30 seconds and tries once more, while an analysis failure exits with
the scanner's status on the first attempt. Each attempt's log is stored as an
artifact. The orb is no longer used.

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

The Python client in `client/python/` is published to PyPI as
[`opensysml`](https://pypi.org/project/opensysml/) by the `release` workflow — the
same `v<version>` tag that publishes the binaries, and at the same version: `v0.9.0`
publishes `opensysml` 0.9.0. Nothing is uploaded from a laptop, and no other tag
publishes the package. Releases up to 0.5.0 were cut on a tag of their own,
`opensysml-v<version>`, which the workflow no longer matches; the version line
before that carries on from `pysysml` 0.2.0, which was the same client, so no version
number is reused.

### Why the same tag

`opensysml` does not ship the service: it downloads a `sysml-grpc` binary at
runtime for whatever release the caller names (`version=`,
`$OPENSYSML_GRPC_VERSION`, or `latest`), verifying it against the digest it pins
for that release (its copy of `client/release-digests.json`) or, for a
release it pins nothing for, against the digest in the release's signed
`SHA256SUMS.txt` (see [the signed checksum manifest](#the-signed-checksum-manifest)).
That flexibility is what makes an uncoordinated pair hard to test: a package at one
version against a service at another is a combination nobody ran the suite on.
Releasing the two together from one tag means every `opensysml` version has a core
release of the same version, tested with it in the same pipeline, and a caller who
wants exactly that pairing pins one number:

```bash
pip install opensysml==0.9.0
export OPENSYSML_GRPC_VERSION=v0.9.0
```

The wheel and sdist go on the GitHub release too, listed in the signed
`SHA256SUMS.txt`, so the release page holds every deliverable of that version.

The cost is that a client-only fix is a core release (a patch tag, with the
binaries rebuilt from the same source), and that one step of a release is
irreversible: `publish-github-release` runs `ghr -replace`, so re-running a tag's
workflow replaces the GitHub assets, while a PyPI version can be yanked but never
re-uploaded. `publish-pypi` therefore refuses a version the index already has, and
on a re-run of a published tag that job fails by design while the rest of the
workflow succeeds — the package was already published from the same revision, so
nothing is missing. The upload runs last, only after the whole suite has passed on
the tagged revision and the GitHub release is published, so a failure anywhere
else — a rebuild, an expired GitHub token — never leaves a package on PyPI whose
release does not exist.

### The version, in one place

`client/python/opensysml/_version.py` is the only declaration:

- `client/python/pyproject.toml` has `dynamic = ["version"]` and reads
  `opensysml._version.VERSION` (there is no `setup.py` any more —
  `pyproject.toml` declares the build);
- `opensysml.__version__` reports that declaration, which ships beside the module
  and is therefore the version of the code being imported. A wheel's metadata is
  generated from it, so the two agree there; an editable install's dist-info is
  written once, at install time, and a checkout that bumps `VERSION` afterwards
  would otherwise report the version it had when `pip install -e` ran.

`client/python/tests/test_version.py` fails if a second version literal reappears
anywhere under `client/python/`, or if the declaration, the installed metadata and
`__version__` stop agreeing. Where the install is editable, the tests locate the
package through the install's own PEP 610 record (`opensysml/_dist.py`) rather than
the dist-info's directory, which for an editable install is a site-packages path
holding no `opensysml/` at all.

The tag must name the declared version. `client/python/scripts/check_version.py` is run
by `build-python-package` before anything is built, and fails loudly otherwise:

```bash
python client/python/scripts/check_version.py --tag v0.9.0   # prints 0.9.0
```

So setting `VERSION` in `client/python/opensysml/_version.py` to the version being
released is a step of [the release branch](#the-release-branch), beside folding the
changelog; a `v*` tag pushed while the two disagree fails the release before a binary
is built. The tag is SemVer and the declaration is PEP 440 in canonical form, so the
check translates the tag before comparing: `v0.9.0` names `0.9.0`, and a pre-release
tag `v0.9.0-rc1` (or `v0.9.0-rc.1`) names `0.9.0rc1`, which is what `VERSION` must say
(`0.9.0-rc1` is refused, since the build tools would name the files `0.9.0rc1` anyway).
Only `-alpha.N`, `-beta.N` and `-rc.N` are accepted as pre-release suffixes, the ones
with a single PEP 440 meaning; a tag like `v0.9.0-1` is refused rather than read as
the post-release `0.9.0.post1` and sent to PyPI proper.

```bash
python client/python/scripts/check_version.py --tag v0.9.0-rc1   # prints 0.9.0rc1
```

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

### What the jobs do, in order

Two jobs of the `release` workflow, so the distribution is built once and the same
bytes go to the GitHub release and to PyPI.

`build-python-package`, which runs after `build-release-binaries` (whose `dist/grpc`
it attaches) and gates `build-release`:

1. `check_version.py` — the tag must name the declared version.
2. `pin_release_checksums.py --version "$CIRCLE_TAG" --from-binaries dist/grpc
   --table client/python/opensysml/release-digests.json` — hashes the five
   `sysml-grpc-*` binaries the release ships and stamps them, under the tag,
   into the copy of the table the distribution packages (see [Pinned release
   digests](#pinned-release-digests)). The committed tables are untouched.
3. `python -m build` — wheel *and* sdist, into `client/python/dist/`.
4. `twine check --strict` — the metadata a broken listing comes from.
5. Installs the built wheel into a clean virtualenv, imports it, and checks
   `opensysml.__version__` is the version being published.
6. Opens the wheel and the sdist and fails unless each packages a
   `release-digests.json` pinning all five assets for the tag to the digests of
   the binaries in `dist/grpc`, and the installed wheel's `pinned_digest()`
   returns them.
7. Persists `client/python/dist/` to the workspace. `build-release` copies both
   files into `dist/`, lists them in `SHA256SUMS.txt`, fails unless the wheel's
   pins are the manifest's `sysml-grpc-*` lines, signs the manifest, and checks
   the distribution's names carry the tag's version.

`publish-pypi`, which runs last, after the Go suite, the Python client tests,
`build-release` and `publish-github-release` have all passed on the tagged revision:

1. Resolves the version from the tag again and checks the workspace holds the
   wheel and sdist of that version.
2. Requires the token for the index it will use.
3. Refuses to continue if that index already has this version (a re-run of an
   already-published version fails here, deliberately: it cannot be replaced,
   and `--skip-existing` would let a half-intended re-run look successful).
4. `twine check --strict` again, then `twine upload` with
   `TWINE_USERNAME=__token__` and the token from the context.

### Dry run on TestPyPI

A **pre-release version publishes to TestPyPI instead of PyPI** — that is the
whole rule, so the happy path has no extra switch to forget. Since the tag is
the core's, a rehearsal is a core pre-release: it builds and publishes the
binaries to a GitHub release like any other tag, and only the package's
destination changes.

```bash
# 1. Declare a pre-release version, e.g. VERSION = "0.9.0rc1"
$EDITOR client/python/opensysml/_version.py
# 2. Land it, then tag it (the SemVer spelling of the same version)
git tag -a v0.9.0-rc1 -m "v0.9.0-rc1" && git push origin v0.9.0-rc1
```

The job resolves the version, sees a PEP 440 pre-release, requires
`TEST_PYPI_API_TOKEN`, and uploads to `https://test.pypi.org/legacy/`. Verify it
the same way as a real release, pointing pip at TestPyPI but taking the
dependencies from PyPI:

```bash
python -m venv /tmp/opensysml-rc && . /tmp/opensysml-rc/bin/activate
pip install --index-url https://test.pypi.org/simple/ \
            --extra-index-url https://pypi.org/simple/ opensysml==0.9.0rc1
OPENSYSML_GRPC_VERSION=v0.9.0-rc1 python -c "import opensysml; print(opensysml.__version__)"
```

Then set `VERSION` to the final version and tag `v0.9.0`.

Nothing about the pre-release path is required for a normal release; if you skip
it, no TestPyPI token is needed at all.

### Verifying an upload

In a clean virtualenv, from the index — not from the source tree:

```bash
python -m venv /tmp/opensysml-verify && . /tmp/opensysml-verify/bin/activate
pip install opensysml==0.9.0
python -c "import opensysml; print(opensysml.__version__)"    # must print 0.9.0
```

Then check the client end to end against the core release of the same version,
since that is the pairing the release tested:

```bash
export OPENSYSML_GRPC_VERSION=v0.9.0          # the same tag
python -c "import opensysml; print(opensysml.load('examples/state-machine-demo.sysml').diagnostics)"
```

Finally, read the project page: the description, the license, the project URLs
and the Python versions are the metadata `twine check --strict` accepted, not
metadata anyone reviewed.

### If an upload goes wrong

A PyPI version cannot be replaced. Yank it
(PyPI → project → *Manage* → *Releases* → *Yank*, which hides it from resolvers
without breaking a pin that already names it), and cut the next core release —
the package's version is the core's, so the fix is a patch tag, not a new
`VERSION` alone. Deleting a release frees nothing: the version number stays used.

## Releasing jupyter-opensysml-kernel to PyPI

The Jupyter kernel package in `client/jupyter-kernel/` is published to PyPI as
[`jupyter-opensysml-kernel`](https://pypi.org/project/jupyter-opensysml-kernel/) by the
same `release` workflow, at the core's version and in lockstep with `opensysml`:
`client/python/scripts/check_version.py --jupyter-kernel` refuses a tag whose version the
kernel package does not declare, so a release bump stamps both `_version.py` files.

The package is one wheel per released platform, each bundling that platform's
`sysml-jupyter-kernel-<os>-<arch>` asset and installing the kernelspec as shared data so
`pip install` alone registers the kernel, plus an sdist that bundles none and downloads the
asset, verified against the digests in its own
`jupyter_opensysml_kernel/release-digests.json`, when it registers the kernelspec. The
distributions are built the way the Python client's is, from the binaries themselves:

1. `build-release-binaries` cross-compiles `sysml-jupyter-kernel` into `dist/jupyter`
   beside the service binaries, checks the Linux ones are static and that each reports
   the tag.
2. `build-jupyter-kernel-package` stamps the digests of those binaries into the package's
   table (`pin_release_checksums.py --from-binaries dist/jupyter --table
   client/jupyter-kernel/jupyter_opensysml_kernel/release-digests.json`), then runs
   `scripts/build-jupyter-kernel-dist.sh dist/jupyter client/jupyter-kernel/dist`: the
   sdist first, then for each platform the binary is checked against its `.sha256`
   sidecar, staged as `jupyter_opensysml_kernel/bin/sysml-jupyter-kernel[.exe]`, and
   `python -m build --wheel` runs with `JUPYTER_OPENSYSML_KERNEL_PLATFORM=<os>-<arch>`,
   which `client/jupyter-kernel/setup.py` turns into the wheel's platform tag and the
   `share/jupyter/kernels/sysml` data files. The script opens every wheel to check it
   carries its kernel and the kernelspec, and the sdist to check it carries neither. The
   job then installs the Linux x86_64 wheel alone into a clean virtualenv and asserts it
   imports at the tag's version, that `jupyter kernelspec list` finds `sysml` under the
   virtualenv's prefix, and that `python -m jupyter_opensysml_kernel -version` starts the
   bundled kernel and it reports the tag; and asserts that every wheel and the sdist are
   built against the tag and pin all five `sysml-jupyter-kernel-*` assets to the digests
   of the binaries just built.
3. `build-release` lists the kernel binaries, their `.sha256` sidecars and the six kernel
   distributions in `SHA256SUMS.txt`, checks every wheel's pins against the manifest it is
   about to sign, and signs it.
4. `publish-pypi-jupyter-kernel` runs after the GitHub release exists, refuses a version the
   index already has, and uploads the verified distribution with the same `PyPI` context
   as `publish-pypi`. The project needs its own trusted-publisher or token entry on PyPI;
   the token in the context must be allowed to upload to both projects.

`tests/hygiene/release_config_test.go` holds the configuration to this order, and
`pin_release_checksums.py` to the rule that a release carrying any kernel asset carries all
five. Nightly snapshots build and publish the kernel package the same way, as a development
version pinning the night's prerelease.

The conda-forge recipe under `packaging/conda` is rendered from the release's manifest
with `scripts/render-conda-recipe.sh` after the release exists; see
[packaging/conda/README.md](../../packaging/conda/README.md).

## Releasing @openmbee/opensysml to npm

The Node client in `client/node/` is published to npm as `@openmbee/opensysml`
by the `release` workflow's `publish-npm` job, from the same core `v<version>`
tag that publishes the binaries and `opensysml` — at that version. No other tag
publishes it; the `client-node-v*` path never ran and is no longer matched.
The npm packages are published with core releases at the client's version, and
the `npm` context they need is already in place (see
[What the job needs](#what-the-job-needs-1)).

### Seven packages, one tag

`@openmbee/opensysml` carries no binary. The service binary comes from one of five
per-platform packages it names in `optionalDependencies`, which npm installs by
matching their `os`/`cpu` metadata. The optional `@openmbee/opensysml-wasm` peer
package carries the combined WebAssembly module and its matching Go runtime:

| package | os | cpu |
| --- | --- | --- |
| `@openmbee/opensysml-sysml-grpc-linux-x64` | linux | x64 |
| `@openmbee/opensysml-sysml-grpc-linux-arm64` | linux | arm64 |
| `@openmbee/opensysml-sysml-grpc-darwin-x64` | darwin | x64 |
| `@openmbee/opensysml-sysml-grpc-darwin-arm64` | darwin | arm64 |
| `@openmbee/opensysml-sysml-grpc-win32-x64` | win32 | x64 |
| `@openmbee/opensysml-wasm` | — | — |

All seven share the version in `client/node/package.json`. The platform
packages and WASM package are published first, so `@openmbee/opensysml` is
never on the registry naming a version of a package that is not. Where no
platform package matches — a platform with no
release build — the client falls back to `$OPENSYSML_BINARY`, a binary in
`~/.opensysml/bin/`, a release download into that cache, `sysml-grpc` on
`$PATH`, or an explicit external service. That download is the Python client's:
the same shared cache and metadata, the same pinned digests, and the same
signed-manifest verification, refusing a release it can neither pin nor verify.
See `client/node/README.md`.

### Where the binaries come from

The five binaries are `build-release-binaries`' `dist/grpc` output, with the
`.sha256` sidecars it writes beside them — the same bytes as the GitHub
release and the signed `SHA256SUMS.txt`, which `build-release` folds the
sidecars into after checking them — persisted to the workspace the npm job
attaches. `npm run platform-packages` refuses to package a binary
whose bytes disagree with its `.sha256` sidecar, or that has none, so the
packages can only carry what the release built. The WASM package is built from
`dist/wasm` in the same workspace with both assets checked against their
sidecars. npm's `--provenance` is not used: the CLI mints attestations only on
GitHub Actions and GitLab CI/CD.

### Why the core's tag

The client follows the Python client's choice (see
[Why the same tag](#why-the-same-tag)): every npm version then has a core
release of the same version tested with it in the same pipeline, and a caller
pins one number — `npm install @openmbee/opensysml@0.9.1` gets the release's own
binary via the platform package; install `@openmbee/opensysml-wasm` at the same
version for Node's automatic `connectWasm()` package resolution. The cost: a
client-only fix is a core patch release. And since an npm publish is
irreversible, the job runs last and refuses
a version already on the registry, just like `publish-pypi`.

### The version

`client/node/package.json` follows `client/python/opensysml/_version.py` — the
same version, spelled the SemVer way (`0.9.0-rc.1` for `0.9.0rc1`).
`check_version.py --node` in `build-python-package` fails the release before
anything is built when they disagree, and the pytest gate in
`test_check_version.py` runs on every PR that touches either file. The Node
package tests also ensure the optional WASM peer follows the client version. The tag must
spell the SemVer version exactly, `v` aside.

### Pre-releases

A pre-release tag — the same one that sends `opensysml` to TestPyPI — publishes
all seven packages to the `next` dist-tag; `latest` is untouched. Install a
pre-release with `@next` or the exact version.

### What the job needs

Everything below is already in place; it is recorded so it can be re-created.

1. The **`@openmbee` scope** on npm, with the publishing account a member of the
   org with publish rights on its packages. A new package under the scope is
   created by its first `npm publish --access public`.
2. A **granular access token** stored as `NPM_TOKEN` in the CircleCI restricted
   context `npm` (lower-case, matched exactly, restricted to a security group).
   The token was created on npmjs.com → *Access Tokens* → *Generate New Token*
   → *Granular Access Token*, Packages and scopes: Read and write, restricted
   to the `@openmbee` scope, **Bypass 2FA** enabled for non-interactive
   publishing, no IP allowlist. (npm classic/automation tokens were revoked in
   December 2025, and npm has no trusted publishing for CircleCI.)
3. **Rotation is a standing task**: granular write tokens expire after at most
   90 days, so before each release check its expiry and, if it has lapsed or
   will soon, create a replacement the same way and update `NPM_TOKEN` in the
   `npm` context. `publish-npm`'s `npm whoami` step fails before anything is
   published when the token has expired.

### What the job does, in order

`publish-npm` runs after `publish-github-release`, beside `publish-pypi`:

1. Resolves the version: fails if the tag is not `v<version>` matching
   `client/node/package.json`, picks the `latest`/`next` dist-tag from the
   version, and lists the workspace binaries it will package.
2. Stamps the five `sysml-grpc-*` digests of `dist/grpc` for `CIRCLE_TAG` into
   `client/node/release-digests.json` with
   `pin_release_checksums.py --from-binaries dist/grpc --table …`, exactly as
   `build-python-package` does for the wheel. Only that file in the working
   copy changes; see [Pinned release digests](#pinned-release-digests).
3. Requires `NPM_TOKEN` from the `npm` context.
4. Refuses to run if any of the seven packages is already on the registry at this
   version (a publish cannot be repeated).
5. Builds and tests the client against the release's linux binary (`npm ci`,
   build, typecheck, lint, tests), with `$OPENSYSML_EXPECT_PINNED_RELEASE` set
   to the tag so the suite asserts the stamped table pins it.
6. Builds the five platform packages from `dist/grpc` and the WASM package
   from `dist/wasm`, checking every asset against its `.sha256` sidecar.
7. Packs the client into `dist/npm/` with `npm pack` and opens the tarball:
   its `release-digests.json` must pin all five service assets for the tag,
   with the digests of the binaries in `dist/grpc`, or the job fails.
8. Authenticates to npm and runs `npm whoami`, so an expired token fails before
   the first publish.
9. Publishes the WASM and five platform packages, then the client — the
   tarball verified in step 7, not a fresh pack — on the resolved dist-tag.

### If a publish goes wrong

`publish-npm` runs after `publish-github-release` and beside `publish-pypi`, so
a failure there leaves the GitHub release and PyPI in place. Before the first
`npm publish` — a version/tag mismatch, a missing or expired token, an `npm
whoami`, build, test or digest failure — nothing is on npm: fix the cause (for
example rotate the token in the context) and re-run only that job — *Rerun
workflow from failed* — without repeating the rest of the workflow.

After the first publish the version is used: the registry refusal makes a
re-run fail by design, and a half-published set — some platform packages up,
the client not — is not repaired by re-running. `npm deprecate` what went up
(and `npm unpublish` within 72 hours only if nothing depends on it) and cut the
next core patch release. Never remove the refusal to force a re-run through.

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
`client/python/tests/test_legacy_pysysml_placeholder.py` asserts the same contract from
source on every run.

This is expected to happen exactly once. Nothing further should be published
under the old name; a client fix goes to `opensysml`.

## Releasing the Java client to Maven Central

The Java client in `client/java/` is published to Maven Central as
`org.openmbee:opensysml` — with its parent, `org.openmbee:opensysml-parent`
— by the `release` workflow's `publish-maven` job, from the same core
`v<version>` tag that publishes the binaries, `opensysml` and
`@openmbee/opensysml`, at that version. The `opensysml-java-v*` path was never
tagged and is no longer used. **The Java artifacts are not on Maven Central yet.**

### What a maintainer must obtain first

None of these can be provisioned from a checkout. The key and the token live
in the restricted context **`Maven Central`** (Organization Settings →
Contexts — a context reference is matched exactly, so the case has to match),
which holds `CENTRAL_TOKEN_USERNAME`, `CENTRAL_TOKEN_PASSWORD`,
`GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`, set up like the PyPI and npm contexts
(see [what the job needs](#what-the-job-needs)). Contexts restricted to a
security group admit only their members, so whoever pushes the tag must be
allowed to use all of them — `PyPI`, `npm`, `Maven Central` and `crates.io` —
or the job fails as unauthorized before anything runs. The signing key in
place is a freshly generated one with a two-year expiry, so the rotation note
below applies within two years.

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

   The private key and its passphrase are the context's
   `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`. Store the key base64-encoded on one
   line — `gpg --armor --export-secret-keys <KEY_ID> | base64 | tr -d '\n'` —
   since the CircleCI UI drops newlines; the job also accepts the raw armored
   block.
   The job test-signs before anything uploads, so an expired key or a wrong
   passphrase fails before anything reaches Central. A key approaching its
   expiry needs extending (`gpg --quick-set-expire`) and the public key
   re-published, or replacing outright — update `GPG_PRIVATE_KEY` and
   `GPG_PASSPHRASE` to match. Signatures already published stay verifiable.
3. **Portal tokens.** Central portal → *View Account* → *Generate User Token*
   gives a username/password pair for a `<server>` with `<id>central`, the
   context's `CENTRAL_TOKEN_USERNAME`/`CENTRAL_TOKEN_PASSWORD`, written
   into `~/.m2/settings.xml` by the job. A token can be revoked and regenerated
   in the portal and then replaced in the context.

### The version

`client/java/pom.xml` follows `client/python/opensysml/_version.py` — the same
version, spelled the Maven way, which is the SemVer spelling (`0.9.0-rc1` for
`0.9.0rc1`): the parent pom's `<version>`, both modules' `<parent><version>`,
and the client version the editors name (`opensysml.client.version` in
`editors/mdk/pom.xml`, the `opensysml` dependency in
`editors/syson/backend/pom.xml`). `check_version.py --java` in
`build-python-package` fails the release before anything is built when they
disagree, and the pytest gate in `test_check_version.py` — including the test
that every in-repo reference names the pom's version — runs on every PR that
touches either file. The tag must spell the version exactly, `v` aside.

The editors' own versions are in the same lockstep: every manifest
`check_version.py --editors` reads — the VS Code and SysON frontend
package.json files and their locks, the Cameo and SysON parent poms and their
children's `<parent><version>` — carries the SemVer spelling, and the release
fails early when one disagrees.

### Why the core's tag

The client follows the Python and Node clients' choice (see
[Why the same tag](#why-the-same-tag)): every published version has a core
release of the same version tested with it in the same pipeline, and a consumer
pins one number. The old reason for a separate tag — Central is immutable and
`ghr -replace` re-runs the `v*` tag — is answered by the job running last and
refusing a version already on Central, like `publish-pypi` and `publish-npm`.
The cost stays the same too: a client-only fix is a core patch release.

### What the build already produces

`mvn -f client/java/pom.xml install` attaches everything Central validates:

- `opensysml-<version>.jar`, `-sources.jar` and `-javadoc.jar` (the
  `maven-source-plugin` and `maven-javadoc-plugin` executions are in the default
  build, not the release profile, so a missing one fails long before a release);
- POM metadata Central requires: `name`, `description`, `url`, `licenses`,
  `developers`, `scm`;
- `opensysml-conformance`, which sets `maven.deploy.skip` — it is a test harness,
  not a published artifact.

The `release` profile adds what only a release needs: `maven-gpg-plugin` signing
at `verify` (the passphrase comes from `MAVEN_GPG_PASSPHRASE`, never from a
pom property or the settings), and `central-publishing-maven-plugin` with
`autoPublish=true` and `waitUntil=published` — the job publishes the validated
deployment itself and waits until it is published, so a green job means the
version is on Central. The irreversible step is guarded by running last, on a
revision proven green, and by the refusal of a version already published.

The same checks run locally before a release, without uploading:

```bash
make build                                            # the service the tests start
mvn -f client/java/pom.xml clean verify              # tests, javadoc, sources
mvn -f client/java/pom.xml -Prelease verify          # + signatures, no upload
gpg --verify client/java/opensysml-client/target/*.jar.asc   # check one by hand
```

### Pre-releases

Central has no test registry. A pre-release tag — the same one that sends
`opensysml` to TestPyPI and the npm client to `next` — publishes an ordinary,
permanent version that Maven orders before the release: `0.9.0-rc1` resolves
before `0.9.0`. Consumers get it only by naming it.

### What the job does, in order

`publish-maven` runs after `publish-github-release`, beside `publish-pypi` and
`publish-npm`:

1. Attaches the release workspace, so `dist/grpc` holds the binaries
   `build-release-binaries` built, and resolves the version: fails if the tag
   is not `v<version>` matching `client/java/pom.xml`, or the version is a
   `-SNAPSHOT`.
2. Stamps the five `sysml-grpc-*` digests of `dist/grpc` for `CIRCLE_TAG` into
   `client/java/opensysml-client/src/main/resources/release-digests.json` with
   `pin_release_checksums.py --from-binaries dist/grpc --table …`, exactly as
   `build-python-package` does for the wheel. Only that file in the working
   copy changes; see [Pinned release digests](#pinned-release-digests).
3. Requires all four credential environment variables, naming only the missing
   one.
4. Refuses to run if `org.openmbee:opensysml-parent` or `opensysml` is
   already on Central at this version (a publish cannot be repeated).
5. Runs `mvn clean package -pl :opensysml -am -Dtest=ReleaseAssetsTest` with
   `$OPENSYSML_EXPECT_PINNED_RELEASE` set to the tag — `java-test` ran the
   whole suite on this revision; only the test that reads the stamped resource
   runs again, now asserting it pins the tag — and opens the packaged jar: its
   `release-digests.json` must pin all five service assets for the tag, with
   the digests of the binaries in `dist/grpc`, or the job fails.
6. Imports `GPG_PRIVATE_KEY` and test-signs with `GPG_PASSPHRASE`, so an expired
   key or wrong passphrase fails before the upload.
7. Writes `~/.m2/settings.xml` naming the `central` server, reading the portal
   token from the environment so it never lands on disk.
8. Runs `mvn -Prelease deploy -pl :opensysml -am -DskipTests` — `-am` carries
   the parent pom the client's pom names. The jar is rebuilt from the tree
   whose stamped resource step 5 verified. The plugin uploads, Central
   validates, `autoPublish` releases the deployment, and the build waits until
   it is published.

### If a publish goes wrong

`publish-maven` runs after `publish-github-release` and beside `publish-pypi`
and `publish-npm`, so its failure leaves the GitHub release, PyPI and npm in
place. Before the upload — a tag/version mismatch, a missing variable, a
Central availability-check error, an expired key or wrong passphrase, a build
or javadoc failure — nothing is on Central: fix the cause and *Rerun workflow
from failed*, without repeating the rest of the workflow.

A deployment that fails Central validation is not published — `autoPublish`
only publishes a valid one: drop it in the portal if it remains, fix the cause,
and re-run. If the job timed out waiting while Central was still publishing,
check the portal before anything else; the next run's availability check
answers whether the version landed.

Once published the version is immutable — it cannot be replaced or deleted,
and the refusal makes a re-run fail by design. A published mistake needs the
next core patch release.

## Releasing the Rust client to crates.io

The `opensysml` crate is published on crates.io by the `release` workflow's `publish-crates`
job from the core `v<version>` tag, at the version `Cargo.toml` declares, after
the suite and the GitHub release. `opensysml-rust-v*` was never tagged and is no
longer used. The maintainer-run `cargo publish` and the bump-then-tag procedure
it followed are gone.

### What a maintainer must obtain first

A crates.io API token with the publish-new/publish-update scopes — scoped to
the `opensysml` crate alone once it exists — stored as `CARGO_REGISTRY_TOKEN`
in the restricted context **`crates.io`** (Organization Settings → Contexts —
the name is matched exactly, lower-case included), set up like the PyPI, npm
and `Maven Central` contexts (see [what the job needs](#what-the-job-needs)).
Whoever pushes the tag must be allowed to use all of them, as the Java section
above notes. A token can be given an expiry at creation; rotate it before one
lapses — crates.io refuses an expired token at the publish step, and nothing
is published.

### The version

`client/rust/opensysml/Cargo.toml`'s `[package] version` follows
`client/python/opensysml/_version.py`, at the SemVer spelling of it — the same
spelling package.json and the pom use (`0.9.1`; `0.9.0-rc.1` for `0.9.0rc1`).
`check_version.py --rust` in `build-python-package` enforces the lockstep, and
a pytest gate holds it on every commit — including `client/rust/Cargo.lock`,
whose `opensysml` entry must name the same version (the checklist's
`cargo update -p opensysml` keeps it in step). crates.io publishes
the version Cargo.toml declares, so the tag must spell it exactly.

### Why the core's tag

The same reasons as npm and Maven — see
[Why the same tag](#why-the-same-tag): the crate is the same client's surface
in another language, so the tag that proves the suite is the tag that
publishes it. crates.io versions are immutable, which is why the job runs last
and refuses a version the registry already holds, like PyPI, npm and Central.

### What the crate already carries

`cargo package -p opensysml` must succeed cleanly before any publish — it is
what proves the manifest carries the metadata crates.io requires and that the
packaged file list builds on its own, outside this workspace. The manifest
declares `license`, `description`, `repository`, `homepage`, `documentation`,
`keywords`, `categories` and `rust-version = "1.83"`, so a published crate
documents its own minimum supported Rust version. `opensysml-conformance` is a
workspace member and a runner, not a library, and is **not** published: it
reads `conformance/scenarios` from this repository.

Before packaging, the job stamps the crate's embedded `release-digests.json`
with the five service-asset digests for `CIRCLE_TAG` from
`dist/SHA256SUMS.txt`. The release build has already signed and verified this
manifest. A crate published from a release tag can therefore verify and
download the release it was built against by default. The Rust client still
does not verify the manifest's Sigstore signature itself. A crate built from a
Git checkout, or asked for another release, still needs a matching pin or
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

### Pre-releases

crates.io has no test registry, so a pre-release tag (`v0.9.1-rc.1`…) publishes
an ordinary version. Cargo never selects a pre-release for a `0.9` requirement,
so consumers get it only by naming it exactly.

### What the job does, in order

1. Fails on an empty `CIRCLE_TAG`; resolves the crate version with
   `cargo pkgid` and requires the tag to be `v<version>` — nothing was
   published when they disagree.
2. Requires `CARGO_REGISTRY_TOKEN`, naming it only when missing.
3. Refuses the version when crates.io already holds it (a published version
   cannot be replaced, only yanked), and refuses rather than guesses when the
   API cannot be asked.
4. Stamps `CIRCLE_TAG` from `dist/SHA256SUMS.txt`, packages with
   `cargo package -p opensysml --allow-dirty --locked`, and verifies the
   packaged crate embeds all five service digests for that tag.
5. `cargo publish -p opensysml --locked --no-verify --allow-dirty`; cargo reads
   the token from the environment, so nothing is written to disk.

### If a publish goes wrong

`publish-crates` runs after `publish-github-release` and beside
`publish-pypi`, `publish-npm` and `publish-maven`, so its failure leaves those
in place. Before the upload — a tag/version mismatch, a missing token, a
crates.io availability-check error, a package dry-run failure — nothing is on
crates.io: fix the cause and *Rerun workflow from failed*.

Once published the version is immutable — it cannot be replaced or deleted,
and the availability check makes a re-run fail by design. A mistake is yanked
(`cargo yank --version <version>`, which stops new resolutions while existing
lockfiles keep working) and fixed in the next core patch release.
