# Nightly snapshots

[![Nightly snapshot](https://github.com/Open-MBEE/OpenSysML/actions/workflows/nightly.yml/badge.svg?branch=develop)](https://github.com/Open-MBEE/OpenSysML/actions/workflows/nightly.yml)

Every night the newest green commit on `develop` is built into the same binaries,
WebAssembly assets and Python distribution a release ships, plus the npm packages, and
published as the prerelease **`nightly-<yyyymmdd>-<commit>`**, kept for 14 days, with
**[`nightly`](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly)** as the moving
alias of the newest night. The Python and Node clients built from the same commit go to PyPI
and npm as development versions. It is a development build: what a change looks like the day
it lands, before the next stable release
([latest](https://github.com/Open-MBEE/OpenSysML/releases/latest), installed as described in
the [install guide](../guide/01-install.md)) carries it.

| | Where |
|---|---|
| The newest snapshot | [github.com/Open-MBEE/OpenSysML/releases/tag/nightly](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly) — assets, the commit it was built from, and a line per unreleased changelog entry it contains |
| The last 14 nights | the [releases page](https://github.com/Open-MBEE/OpenSysML/releases?q=nightly-&expanded=true), one `nightly-<yyyymmdd>-<commit>` prerelease each |
| The Python client | [PyPI](https://pypi.org/project/opensysml/#history), as `opensysml` `<next>.dev<yyyymmdd>` versions; `pip install opensysml` ignores them |
| The Node client | [npm](https://www.npmjs.com/package/@openmbee/opensysml?activeTab=versions), as `<next>-nightly.<yyyymmdd>.g<commit>` versions under the `nightly` dist-tag; `latest` is the stable release |
| Whether last night's run passed | the badge above, and the [workflow runs](https://github.com/Open-MBEE/OpenSysML/actions/workflows/nightly.yml) |
| What has landed since the last release | [`develop`'s changelog fragments](https://github.com/Open-MBEE/OpenSysML/tree/develop/changes/unreleased), or the compare link in the release notes |

## What a snapshot is

- **Built from the newest green `develop` commit.** The workflow walks `develop`'s own
  commits (its first-parent history, one per merged pull request) from the head and takes the
  first whose CircleCI `build-test` workflow — the Go suite, the
  corpus gates and the client tests — passed. A red head is skipped, so a snapshot can be a
  few commits behind `develop`; the release notes name the commit and link the
  compare against the last stable tag. The walk ends at the commit the current snapshot was
  built from, so the snapshot never moves backwards, and a green commit that predates
  `scripts/build-release-artifacts.sh` or `client/python/scripts/snapshot_version.py` is
  skipped, since the workflow cannot build it.
  Snapshots built from commits predating the WASM build do not include WebAssembly assets.
- **Kept for 14 days, under two names.** Each night publishes its own prerelease,
  `nightly-<yyyymmdd>-<commit>`, and recreates the alias `nightly` at the same commit with
  the same assets. A link to `releases/tag/nightly` always names the newest night; a link to
  an asset of a particular night is stable under that night's own tag until the workflow
  deletes it, tag and all, 14 days after it was published. The client packages pin the
  night they were built from by its own tag, so they keep working for those 14 days
  however many nights follow. A night on which no commit newer than the snapshot is green
  publishes nothing and the previous snapshot stands; the night the alias stands at is
  never deleted, however old.
- **Never the latest release.** Every snapshot is a prerelease and is not marked latest, so
  `releases/latest`, `go install …@latest`, the Homebrew tap and the Windows installer all
  keep following the stable `v*` line. On PyPI the snapshot is a development version, which
  `pip install opensysml` skips unless asked for it by exact version or with `--pre`; on npm
  it is published under the `nightly` dist-tag, and `latest` stays where the release left
  it. Maven Central and crates.io get no snapshot at all. Nothing on the stable release
  path changes because a snapshot exists.
- **Identified by its version.** Every binary reports `nightly-<yyyymmdd>-<commit>` from
  `--version`, with the commit and build time on the following lines, and the release title
  and tag carry the same string; the VS Code extension shows it after its own version. The
  clients report the development version of the release the snapshot leads to —
  `0.9.3.dev20261006` from `opensysml.__version__`, `0.9.3-nightly.20261006.gabc1234` from
  the npm manifest — whose date and commit name the night. Quote it when reporting a
  problem.

## What it contains

The assets are the ones a stable release ships, laid out the same way (see
[the release procedure's asset list](releasing.md#what-circleci-publishes)):

- `opensysml-<os>-<arch>.tar.gz` (`.zip` on Windows) — `sysml` and `sysml-lsp` under their
  plain names, with their manual pages, for linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64 and windows/amd64;
- `sysml-<os>-<arch>.tar.gz` and `sysml-lsp-<os>-<arch>.tar.gz` — each binary on its own;
- `sysml-grpc-<os>-<arch>` with a `.sha256` sidecar — the gRPC service, raw;
- `wasm/sysml-wasm.wasm` and `wasm/wasm_exec.js` with `.sha256` sidecars — the
  combined WebAssembly module and matching Go runtime;
- `opensysml-<version>-py3-none-any.whl` and `opensysml-<version>.tar.gz` — the Python
  client, the same files published to PyPI, with that night's `sysml-grpc` digests pinned
  inside;
- `SHA256SUMS.txt` over all of the above and its cosign bundle `SHA256SUMS.txt.bundle`.

The nightly checksum manifest is cosign-signed; nightly assets do not have SLSA
provenance, just like the other assets in the snapshot.

And one a stable release does not ship:

- `opensysml-sysml.vsix` — the [VS Code extension](../guide/08-editors.md#vs-code) packaged
  from the same commit (as `make vscode-package` does). The extension is side-loaded rather than
  published to a marketplace, so the snapshot is where a build of it is picked up. Its
  version is the extension manifest's with the snapshot version appended as the pre-release
  part — `0.9.0-nightly-<yyyymmdd>-<commit>` — so VS Code tells one night's build from the
  next, installs a later night over an earlier one without `--force`, and ranks any stable
  `0.9.0` above them all. It is in `SHA256SUMS.txt` with the rest.

The npm packages are not release assets: the seven tarballs go to the registry, where npm
verifies them against the registry's integrity hashes and the provenance the workflow signs.

Not in a snapshot: the Windows installer, the Authenticode-signed Windows binaries, the
Homebrew formula, and the Maven and crates.io client packages. Those belong to the
stable release.

## Installing one

The archives install like a release's: unpack `opensysml-<os>-<arch>` and put `sysml` and
`sysml-lsp` on your `PATH`. On macOS, Gatekeeper treats a snapshot exactly as it treats a
direct release download — fetch it with `curl`, not a browser, and see
[macOS: Gatekeeper](../guide/01-install.md#macos-gatekeeper).

```bash
curl -fsSLO https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-linux-amd64.tar.gz
curl -fsSLO https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/SHA256SUMS.txt
sha256sum -c --ignore-missing SHA256SUMS.txt
tar xzf opensysml-linux-amd64.tar.gz
./sysml --version
```

To hold on to a particular night, use its own tag in place of `nightly` in those URLs — the
version the binary prints is the tag.

The [install script](../guide/01-install.md#with-the-install-script) does the same for the
platform it runs on, into a directory of your choosing:

```bash
curl -fsSL https://opensysml.org/install.sh | sh -s -- --version nightly --bin-dir ~/opensysml-nightly
```
```powershell
irm https://opensysml.org/install.ps1 | iex   # after: $env:OPENSYSML_VERSION = 'nightly'
```

Keep a snapshot beside your installed release rather than over it: the version string tells
the two apart, and the release is the one to go back to when the snapshot breaks.

The extension installs from its `.vsix` and finds the snapshot's `sysml-lsp` on your `PATH`
or at the path `opensysml.server.path` names (a checkout's `bin/sysml-lsp` is found on its
own, see [the editors guide](../guide/08-editors.md#vs-code)):

```bash
curl -fsSLO https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-sysml.vsix
sha256sum -c --ignore-missing SHA256SUMS.txt
code --install-extension opensysml-sysml.vsix
```

VS Code installs a later night over an earlier one as an update. To go back to an earlier
night, or from a snapshot to a stable build of the extension whose version is lower, add
`--force`.

### The client packages

The clients are versioned as development builds of the release `develop` is heading for:
the release segment of the version the tree declares
(`client/python/opensysml/_version.py`, matched by `client/node/package.json`), bumped to
the next patch once that release is tagged, with the night appended. With `0.9.2` released
and declared, the night of 2026-10-06 built from commit `abc1234` is:

| | Version | Install |
|---|---|---|
| PyPI | `0.9.3.dev20261006` | `pip install opensysml==0.9.3.dev20261006` |
| npm | `0.9.3-nightly.20261006.gabc1234` | `npm install @openmbee/opensysml@nightly`, or that exact version |
| GitHub | `nightly-20261006-abc1234` | the release both packages were built with |

Both package versions rank below `0.9.3` and any `0.9.3` release candidate, so the next
release supersedes every snapshot before it. `pip install opensysml` and
`pip install --upgrade opensysml` never select a development version — only an exact pin,
or `--pre`, does — and `npm install @openmbee/opensysml` installs `latest`, which no
snapshot moves. A PyPI version is immutable and the date is the version, so a day's first
snapshot of the Python client is the one PyPI keeps if the workflow is run again that day;
the npm version carries the commit, so a rerun publishes a new one under `nightly`.

Each package starts the `sysml-grpc` of its own night. The npm platform packages carry the
binaries, as they do for a release. The wheel pins the digests of that night's five
`sysml-grpc` assets under its `nightly-<yyyymmdd>-<commit>` tag, the same way a release's
wheel pins its release, so `opensysml.connect()` downloads and verifies the matching
service with no environment variables set — the pin, not the signed manifest, is what
admits it, since the nightly manifest is signed by the workflow's GitHub identity rather
than the CircleCI identity the client trusts for a release. That download stops working
when the night's release is deleted 14 days on; by then a newer snapshot exists, and a
`sysml-grpc` already cached stays in use. To pair an older snapshot of the client with
another service, put a `sysml-grpc` on your `PATH` or point `OPENSYSML_BINARY` at it, as
the [service guide](../clients/python/service.md) describes.

Snapshots of the clients are for trying what has landed, not for depending on: the next
night's build may change behavior without a changelog entry, and a published snapshot is
never republished or fixed in place. Pin the exact version in a lockfile only for as long
as you mean to.

## Verifying one

`SHA256SUMS.txt` is signed keylessly with cosign by the workflow that built the snapshot, so
a download can be checked back to that run. The identity is the workflow file on `develop`,
not the CircleCI identity a stable release is signed with:

```bash
cosign verify-blob SHA256SUMS.txt --bundle SHA256SUMS.txt.bundle \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity https://github.com/Open-MBEE/OpenSysML/.github/workflows/nightly.yml@refs/heads/develop
sha256sum -c --ignore-missing SHA256SUMS.txt
```

The wheel and sdist on PyPI are the files in that manifest, so a downloaded wheel checks the
same way. The npm packages carry provenance attestations signed from the same workflow run,
which `npm audit signatures` verifies.

## What to expect

A snapshot passed the same suite a release does before it was built, but it has not been
through the [pre-tag gate](releasing.md#before-tagging):
no performance record, no recount of the figures a release quotes, no adjudication of what
moved. Behavior can change between nights without a changelog entry until the fragment for it
lands, and a snapshot may carry a defect the next night's build fixes. Report one as an issue naming
the `nightly-…` version it printed; a snapshot never becomes a release, so nothing is
re-published under its name.

## How it is produced

[`.github/workflows/nightly.yml`](../../.github/workflows/nightly.yml)
runs at 03:23 UTC and on demand (`workflow_dispatch`, with a `force` input that republishes
the same commit — for instance after the workflow itself changed). It picks the commit as
described above and, in one job, derives the versions with the commit's own
[`client/python/scripts/snapshot_version.py`](../../client/python/scripts/snapshot_version.py)
and stamps them into the checkout's `_version.py` and `package.json` (nothing is
committed); builds the assets with
[`scripts/build-release-artifacts.sh`](../../scripts/build-release-artifacts.sh)
— the same targets, platforms, layout and version check as the CircleCI `build-release`
job; packages the VS Code extension with its own `npm run package` stamped with the
snapshot version (not through the Makefile, which the older selected commit may lack the
knob for) and checks the `.vsix` carries it; pins the digests of the `sysml-grpc` binaries
it just built under the night's tag in the wheel's table with
`pin_release_checksums.py --from-binaries`, builds the wheel and sdist, and checks the
installed wheel reports the snapshot version, names the night's release as the one it was
built against and pins all five digests, as the release pipeline checks a release's wheel;
builds the platform and WASM packages from the same binaries and packs all seven npm
tarballs; signs the manifest, which now lists the wheel, with its own GitHub OIDC
identity; and publishes with the repository's own `GITHUB_TOKEN`: the per-night release
first, then the alias recreated at the same commit, then the deletion of per-night releases
published more than 14 days ago. Two further jobs, each in the `nightly` GitHub
environment, publish the wheel and sdist to PyPI with
[`pypa/gh-action-pypi-publish`](https://github.com/pypa/gh-action-pypi-publish) and the
seven tarballs to npm under the `nightly` dist-tag — platform and WASM packages first, the
client last, as the release does — and check afterwards that `latest` still names the
stable release. Both skip a version the registry already has rather than fail, since a
version can never be replaced. The release notes are generated: the commit, the count since
the last `v*` tag, the install lines for the packages, the verification commands, and one
line per unreleased changelog entry (`python3 scripts/changelog.py summary`) — the lead
sentence of each `changes/unreleased/` fragment as it stands at that commit.

There is no secret to configure. The releases are made with the repository's `GITHUB_TOKEN`,
and both registries accept the workflow's short-lived OIDC token through trusted publishing,
registered once by a maintainer:

- **PyPI** — in the [`opensysml` project's publishing
  settings](https://pypi.org/manage/project/opensysml/settings/publishing/), add a GitHub
  Actions publisher with owner `Open-MBEE`, repository `OpenSysML`, workflow
  `nightly.yml` and environment `nightly`. The stable release keeps publishing from
  CircleCI with its token; the two coexist.
- **npm** — for each of `@openmbee/opensysml`, the five `@openmbee/opensysml-sysml-grpc-*`
  packages and `@openmbee/opensysml-wasm`, open the package's settings on npmjs.com,
  choose GitHub Actions under *Trusted Publisher*, and enter organization `Open-MBEE`,
  repository `OpenSysML`, workflow filename `nightly.yml` and environment `nightly`. A
  package configured for trusted publishing still accepts the release's token publish from
  CircleCI.
- **GitHub** — the `nightly` environment is created on the workflow's first run; nothing
  need be added to it. Protection rules on it (required reviewers, a wait timer) gate the
  two registry jobs without affecting the GitHub release.

Until a publisher is registered, its job fails with the registry's authentication error
and nothing else changes: the release is already published by then, and the other registry's
job runs on its own.

The `nightly` and `nightly-<yyyymmdd>-<commit>` tags are the only tags that are not versions.
The Makefile's default `VERSION` describes a checkout against `v*` tags only, so a fetched
snapshot tag does not turn a local build's version into `nightly-…`, and neither the CircleCI
`release` workflow (tags `v*`) nor the Windows signing workflow reacts to one.
