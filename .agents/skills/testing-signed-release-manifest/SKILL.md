---
name: testing-signed-release-manifest
description: How to end-to-end test the signed release checksum manifest (client/python/opensysml/signing.py, the signing branch of binary.py, and the cosign steps in .circleci/config.yml) on Linux without tagging a release — offline fixture bundles, cosign cross-checks, adversarial trust-model probes, and the CI-config static review.
---

# Testing the signed release manifest

The release pipeline signs `dist/SHA256SUMS.txt` with cosign keyless (CircleCI OIDC) and
publishes `SHA256SUMS.txt.bundle` beside it; the Python client verifies that bundle against a
pinned CircleCI identity and takes the asset digest from the verified manifest whenever
`PINNED_SHA256` has no pin. **The CI half only runs on a `v*` tag of Open-MBEE/OpenSysML, so it
cannot be exercised for real** — treat it as a static review and say so in the report.

## Environment

- Use `~/pv/bin/python`. The default `python3` has an incompatible protobuf and will fail on
  import of the gRPC stubs.
- `sigstore` must be present in that venv (`~/pv/bin/pip install 'sigstore>=4.5.0,<5'`). The
  repo blueprint's `maintenance` block does **not** install it today, so a fresh box may need
  it manually; without it `signing.py` deliberately raises `UnsignedReleaseError` rather than
  failing to import, so tests "pass by refusing" and look misleadingly green.
- cosign is usually at `/home/ubuntu/go/bin/cosign` (v3+ needed: `--new-bundle-format`,
  `--trusted-root`). There is normally **no `circleci` CLI**, so `circleci config validate` is
  not available — parse `.circleci/config.yml` with `python3 -c "import yaml..."` (system
  python3 has PyYAML; `~/pv` does not) and `shellcheck` the extracted `run.command` blocks.

## Baseline: the Python suite is not clean out of the box

Before blaming a branch, reproduce in a clean worktree: `git worktree add /tmp/osml-main origin/main`.
Known pre-existing noise, all caused by a missing service binary at `~/.opensysml/bin/sysml-grpc`:
13 errors in `test_model_surface_integration.py` (`ConnectionError: Binary not found at ...`)
plus `test_generate_golden.py::test_generated_class_reads_an_enum_typed_slot`.
Fix by `make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/` (the blueprint's maintenance
step does this; it may not have run on your box). After that, expect only
`test_stale_service.py::test_a_refused_connection_releases_what_it_took` to fail — that one
fails identically on `origin/main` (a fake service binary exits 1 instead of serving) and is
unrelated to release signing.

## Proving the signing tests are offline

`unshare -rn ~/pv/bin/python -m pytest client/python/tests/test_signing.py client/python/tests/test_binary.py -q`
Sanity-check the namespace first by attempting `urlopen('https://example.com')` inside it and
seeing `URLError`. Equal pass count networked and isolated is the evidence.

## Adversarial probes worth writing (drop-in temp test file, delete afterwards)

Reuse the `release`/`signer` fixture pattern in `client/python/tests/test_signing.py`: monkeypatch
`urllib.request.urlopen` to serve a dict of asset name → bytes, `opensysml.binary.get_binary_path`
to a tmp path, `opensysml.binary.release_asset_name`, and set `opensysml.binary.PINNED_SHA256 = {}`.
The recorded signer must use `trusted_root=tests/fixtures/signed_release/trusted_root.json`
(the fixture root of trust stands in for production Sigstore) and be injected via
`monkeypatch.setitem(SIGNED_MANIFEST_SIGNERS, 'Open-MBEE/OpenSysML', recorded)`.

Probes that actually distinguish working from broken:
1. Valid bundle + real manifest, but serve a **different binary** with a matching `.sha256`
   sidecar → must raise `ChecksumMismatchError` naming the manifest's digest, install nothing
   (assert neither the binary path nor `<path>.tmp` exists). A broken client trusts the sidecar.
2. `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=1` plus the `other-identity` / `expired` bundle → must
   raise `ManifestSignatureError` with **zero warnings recorded**
   (`warnings.catch_warnings(record=True)`). Same-origin trust must not be offered when a
   signature was read and failed.
3. Cache semantics by error class: `ManifestSignatureError` (subclass of `ChecksumMismatchError`,
   *not* of `UnpinnedReleaseError`) must propagate out of `ensure_binary` leaving the cached
   file untouched; a **missing** bundle (404) yields `UnsignedReleaseError` (subclass of
   `UnpinnedReleaseError`), which keeps the cache and warns `Keeping the cached sysml-grpc`.
   Make the cache stale first (write a file + `write_metadata('v0.0.5', ...)`) so
   `stale_cache_reason` is non-None, otherwise `ensure_binary` returns early and the branch
   under test is never reached — a silent false pass.

## Independent cosign cross-check of the fixtures

From `client/python/tests/fixtures/signed_release/`:

```
cosign verify-blob SHA256SUMS.txt --bundle SHA256SUMS.txt.bundle \
  --trusted-root trusted_root.json \
  --certificate-oidc-issuer "<issuer from identity.json>" \
  --certificate-identity-regexp "^https://circleci\.com/api/v2/projects/<project id>/pipeline-definitions/[0-9a-f-]+$"
```

Expect `Verified OK`. Then the negative controls: `.other-identity.bundle` fails with an
identity-regex mismatch, `.expired.bundle` with `integrated time outside certificate validity`,
and the good bundle against a **tampered copy** of the manifest fails with
`invalid signature when validating ASN.1 encoded signature`. Skipping the tamper case leaves
open the possibility that the bundle is not bound to the manifest bytes.

## CI-config static review

- The identity regexp is written `\\.` inside a YAML block scalar and a double-quoted shell
  string; bash reduces it to `\.`, so cosign gets the intended pattern. Verify by
  `bash -c 'printf "%s\n" "...\\..."'` rather than eyeballing it.
- The CI regexp and the client's `_DEFINITION_ID` in `signing.py` both require the UUID shape
  `[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}`. Check they still agree: a looser CI pattern would
  pass a subject the client rejects, so the self-check would not catch it.
- Confirm `mv dist/SHA256SUMS.txt.bundle dist/release/` precedes `ghr -replace ... dist/release/`
  in `publish-github-release`, and that the sign/verify steps sit after the manifest-hashing
  step in `build-release`.
- `shellcheck` on the two new steps reports only SC2155 (`export PATH="$(go env GOPATH)/bin:$PATH"`),
  which is pre-existing style, not a defect.

## Devin Secrets Needed

None. Do not tag, push, or release anything in Open-MBEE/OpenSysML — the signing path is
tag-gated by design and must be reviewed statically instead.
