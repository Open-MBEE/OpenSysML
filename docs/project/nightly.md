# Nightly snapshots

[![Nightly snapshot](https://github.com/Open-MBEE/OpenSysML/actions/workflows/nightly.yml/badge.svg?branch=develop)](https://github.com/Open-MBEE/OpenSysML/actions/workflows/nightly.yml)

Every night the newest green commit on `develop` is built into the same binaries a
release ships and published as the prerelease **[`nightly`](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly)**.
It is a development build: what a change looks like the day it lands, before the next
stable release ([latest](https://github.com/Open-MBEE/OpenSysML/releases/latest), installed as
described in the [install guide](../guide/01-install.md)) carries it.

| | Where |
|---|---|
| The snapshot | [github.com/Open-MBEE/OpenSysML/releases/tag/nightly](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly) — assets, the commit it was built from, and a line per unreleased changelog entry it contains |
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
  `scripts/build-release-artifacts.sh` is skipped, since the workflow cannot build it.
- **Replaced, not accumulated.** There is one snapshot. Each night the previous release is
  deleted, the `nightly` tag moved, and a new release published with only that night's
  assets. A link to `releases/tag/nightly` is stable; a link to an asset of a particular night
  is not. A night on which no commit newer than the snapshot is green publishes nothing and
  the previous snapshot stands.
- **Never the latest release.** The snapshot is a prerelease and is not marked latest, so
  `releases/latest`, `go install …@latest`, the Homebrew tap, PyPI, npm and the Windows
  installer all keep following the stable `v*` line. Nothing on the stable release path
  changes because a snapshot exists.
- **Identified by its version.** Every binary reports `nightly-<yyyymmdd>-<commit>` from
  `--version`, with the commit and build time on the following lines, and the release title
  carries the same string; the VS Code extension shows it after its own version. Quote it
  when reporting a problem.

## What it contains

The assets are the ones a stable release ships, laid out the same way (see
[the release procedure's asset list](releasing.md#what-circleci-publishes)):

- `opensysml-<os>-<arch>.tar.gz` (`.zip` on Windows) — `sysml` and `sysml-lsp` under their
  plain names, with their manual pages, for linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64 and windows/amd64;
- `sysml-<os>-<arch>.tar.gz` and `sysml-lsp-<os>-<arch>.tar.gz` — each binary on its own;
- `sysml-grpc-<os>-<arch>` with a `.sha256` sidecar — the gRPC service, raw;
- `SHA256SUMS.txt` over all of the above and its cosign bundle `SHA256SUMS.txt.bundle`.

And one a stable release does not ship:

- `opensysml-sysml.vsix` — the [VS Code extension](../guide/08-editors.md#vs-code) packaged
  from the same commit (as `make vscode-package` does). The extension is side-loaded rather than
  published to a marketplace, so the snapshot is where a build of it is picked up. Its
  version is the extension manifest's with the snapshot version appended as the pre-release
  part — `0.1.0-nightly-<yyyymmdd>-<commit>` — so VS Code tells one night's build from the
  next, installs a later night over an earlier one without `--force`, and ranks any stable
  `0.1.0` above them all. It is in `SHA256SUMS.txt` with the rest.

Not in a snapshot: the Windows installer, the Authenticode-signed Windows binaries, the
Homebrew formula, and the PyPI, npm, Maven and crates.io client packages. Those belong to the
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

The `opensysml` Python client does not download a snapshot on its own. It accepts a
`sysml-grpc` only when its digest is pinned in the client or the release's checksum
manifest was signed by the CircleCI release pipeline, and a snapshot is neither. To run it
against a snapshot, put the snapshot's `sysml-grpc` on your `PATH` (or point
`OPENSYSML_BINARY` at it); see the
[client's README](../../client/python/README.md).

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
described above, builds the assets with
[`scripts/build-release-artifacts.sh`](../../scripts/build-release-artifacts.sh)
— the same targets, platforms, layout and version check as the CircleCI `build-release`
job — packages the VS Code extension with its own `npm run package` stamped with the
snapshot version (not through the Makefile, which the older selected commit may lack the
knob for) and checks the `.vsix` carries it, signs the manifest with its own GitHub OIDC
identity, and publishes with the
repository's own `GITHUB_TOKEN`. There is no secret to configure. The release notes
are generated: the commit, the count since the last `v*` tag, the verification commands,
and one line per unreleased changelog entry (`python3 scripts/changelog.py summary`) —
the lead sentence of each `changes/unreleased/` fragment as it stands at that commit.

The `nightly` tag is the only tag that is not a version. The Makefile's default `VERSION`
describes a checkout against `v*` tags only, so a fetched `nightly` tag does not turn a
local build's version into `nightly-…`, and neither the CircleCI `release` workflow (tags
`v*`) nor the Windows signing workflow reacts to it.
