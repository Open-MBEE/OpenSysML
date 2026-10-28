# Security

How to report a vulnerability and what to expect, which versions get fixes, how to verify a
release before running it, how to harden a served instance, and where advisories are published.
The policy below is [`SECURITY.md`](../../SECURITY.md) at the repository root, embedded when
this site is built, and the site also serves it as
[`/.well-known/security.txt`](https://runtime.opensysml.org/.well-known/security.txt)
([RFC 9116](https://www.rfc-editor.org/rfc/rfc9116)).

<!-- security-policy:begin -->
*The site build replaces this block with [`SECURITY.md`](../../SECURITY.md); read that file
on GitHub for the reporting channel, the response deadlines, coordinated disclosure, the
supported versions and the scope.*
<!-- security-policy:end -->

## Verifying a release

Every stable release publishes one checksum manifest, `SHA256SUMS.txt`, covering every asset of
the release — archives, bare binaries, the Python wheel and sdist, the WebAssembly builds, and
the SBOM of each of them — and signs it with [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) from the
release pipeline's identity (keyless; the certificate names the CircleCI project, not a person),
as `SHA256SUMS.txt.bundle`. A SLSA provenance statement, `provenance.intoto.json`, is signed the
same way. Verify the manifest first; everything else is checked against it. The commands below
were run against [v0.11.0](https://github.com/Open-MBEE/OpenSysML/releases/tag/v0.11.0);
substitute the tag you downloaded.

### The checksum manifest and its signature

```bash
R=https://github.com/Open-MBEE/OpenSysML/releases/download/v0.11.0
curl -fsSLO $R/SHA256SUMS.txt
curl -fsSLO $R/SHA256SUMS.txt.bundle
cosign verify-blob SHA256SUMS.txt --bundle SHA256SUMS.txt.bundle \
  --certificate-oidc-issuer https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62 \
  --certificate-identity-regexp '^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f-]+$'
sha256sum -c SHA256SUMS.txt --ignore-missing
```

`cosign` prints `Verified OK` when the manifest was signed by this project's release pipeline;
`sha256sum` prints one `OK` per downloaded asset it finds in the manifest. The issuer and the
identity pattern are the pipeline's and do not change between releases; a bundle that verifies
under a different identity is not a release of this project. The installer does this check for
you with `--verify-signature` (see [Install](01-install.md)). The `nightly` prereleases are
signed by GitHub Actions instead; their identity is on the
[nightly snapshot page](../project/nightly.md).

### The provenance statement

`provenance.intoto.json` is an in-toto Statement with a
[SLSA v1 provenance](https://slsa.dev/spec/v1.0/provenance) predicate: the tag and commit the
assets were built from, and the digest of every asset. Its signature covers the statement, and
`cosign` checks that the asset you name is one of its subjects:

```bash
curl -fsSLO $R/provenance.intoto.json
curl -fsSLO $R/provenance.intoto.json.bundle
curl -fsSLO $R/opensysml-linux-amd64.tar.gz
cosign verify-blob-attestation opensysml-linux-amd64.tar.gz \
  --bundle provenance.intoto.json.bundle --type slsaprovenance1 \
  --certificate-oidc-issuer https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62 \
  --certificate-identity-regexp '^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f-]+$'
jq -r '.predicate.buildDefinition.buildType,
       (.predicate.buildDefinition.resolvedDependencies[] | "\(.uri) \(.digest.gitCommit)")' \
  provenance.intoto.json
```

For v0.11.0 the build type is `https://github.com/Open-MBEE/OpenSysML/.circleci/build-release/v1`
and the one resolved dependency is `git+https://github.com/Open-MBEE/OpenSysML@refs/tags/v0.11.0`
at commit `ddf4df00037d6ddbfb1fa9d743a4389f75c9e2dc` — the commit the tag points at on GitHub.

### The software bills of materials

Releases after v0.11.0 publish one [SPDX 2.3](https://spdx.github.io/spdx-spec/v2.3/) JSON
software bill of materials per asset, named `<asset>.spdx.json`: the Go modules and toolchain a
binary or archive was built from (the list `go version -m` prints), or the Python distribution a
wheel or sdist carries. Each SBOM is a line of the signed manifest, so once the manifest is
verified the digest check vouches for it like any other asset:

```bash
curl -fsSLO $R/opensysml-linux-amd64.tar.gz.spdx.json
sha256sum -c SHA256SUMS.txt --ignore-missing
jq -r '.packages[] | "\(.name) \(.versionInfo)"' opensysml-linux-amd64.tar.gz.spdx.json
```

How the SBOMs are produced and checked against the binaries is in
[Releasing § The software bills of materials](../project/releasing.md#the-software-bills-of-materials).

### The `.sha256` sidecars

The bare service binaries (`sysml-grpc-*`, `sysml-jupyter-kernel-*`) and the WebAssembly builds
carry a `<asset>.sha256` sidecar for clients that fetch one file. A sidecar is a convenience,
not a trust anchor: it is served from the same place as the asset, so check it against the
signed manifest.

```bash
curl -fsSLO $R/sysml-grpc-linux-amd64.sha256
grep ' sysml-grpc-linux-amd64$' SHA256SUMS.txt | diff - sysml-grpc-linux-amd64.sha256
```

The Python and Node clients, which download `sysml-grpc` for you, use pinned digests stamped
into the package at release time or the signed manifest, never a sidecar alone; a digest
mismatch or an invalid signature is an error, not a fallback to unverified bytes
([Python client § The service](../clients/python/service.md)).

### Python packages

The wheel and sdist uploaded to PyPI are the same files as on the GitHub release, and the
manifest lists them, so a package downloaded from PyPI can be checked against the signed
manifest:

```bash
curl -fsSLO $R/opensysml-0.11.0-py3-none-any.whl
curl -fsSLO $R/opensysml-0.11.0.tar.gz
sha256sum -c SHA256SUMS.txt --ignore-missing
```

Pin the same digest in a requirements file
(`opensysml==0.11.0 --hash=sha256:a4bb6cb481bf008f336bc4dce71b9fd345de971268ebc5874e0a36b84eb5503d`
for the v0.11.0 wheel) and `pip install --require-hashes` refuses anything else. The Jupyter
kernel's platform wheels (`jupyter_opensysml_kernel-*.whl`) are on the release and in the
manifest too.

### npm packages

`@openmbee/opensysml` and its platform packages are published to the npm registry, which records
a subresource integrity digest per tarball that `npm install` checks on every download; a
lockfile pins it. Read the registry's record before pinning:

```bash
npm view @openmbee/opensysml@0.11.0 dist.integrity
```

The platform packages that carry `sysml-grpc` embed the binary from the release's assets.

### Maven Central

`org.openmbee:opensysml` is on Maven Central from 0.10.1
(`pkg:maven/org.openmbee/opensysml@0.11.0`). Central publishes a detached GPG signature
(`.asc`) and checksums beside every artifact, as it requires of every publisher, and Maven and
Gradle check the checksums on download; pin the version in the build file so a resolved
artifact cannot change under it.

### crates.io

Cargo verifies every crate it downloads against the checksum recorded in the registry index, so
`cargo add opensysml@0.11.0` with a committed `Cargo.lock` pins the exact crate. The registry
reports the same checksum:

```bash
curl -fsS -A 'opensysml-docs (curl)' https://crates.io/api/v1/crates/opensysml/0.11.0 \
  | jq -r '.version | .num, .checksum, .yanked'
```

### The container image

`openmbee/opensysml:<version>` is a multi-platform image whose index carries a BuildKit
[provenance attestation](https://docs.docker.com/build/metadata/attestations/slsa-provenance/)
and an SPDX SBOM per platform. Pin the index digest and read the attestations with
`docker buildx imagetools`:

```bash
docker buildx imagetools inspect openmbee/opensysml:0.11.0
docker buildx imagetools inspect openmbee/opensysml:0.11.0 \
  --format '{{ range $k, $v := .Provenance }}{{ $k }}: {{ json $v.SLSA.buildDefinition.externalParameters.request.root.request.args }}{{ "\n" }}{{ end }}'
docker buildx imagetools inspect openmbee/opensysml:0.11.0 \
  --format '{{ range $k, $v := .SBOM }}{{ $k }}: {{ $v.SPDX.spdxVersion }} {{ len $v.SPDX.packages }} packages{{ "\n" }}{{ end }}'
```

The first command prints the index digest (`sha256:e3c7a9dc…` for v0.11.0) and one manifest per
platform plus one attestation manifest each. The provenance names the source the image was built
from — `vcs:source` `https://github.com/Open-MBEE/OpenSysML.git`, `vcs:revision`
`ddf4df00037d6ddbfb1fa9d743a4389f75c9e2dc`, `build-arg:VERSION` `v0.11.0` for v0.11.0 — which
must match the release's provenance above. Run the image by digest
(`openmbee/opensysml@sha256:…`) once you have checked it.

### Windows installer and signed binaries

The MSI and the Windows executables are Authenticode signed through SignPath Foundation once
the project's application is approved, and published as separate `*-signed*` assets with their
digests in `SHA256SUMS-windows-signed.txt`; the unsigned MSI's digest is in
`SHA256SUMS-windows-msi.txt`. Both digest files are produced by the GitHub Actions workflow that
builds the installer, not by the CircleCI pipeline, so they are not covered by the cosign
signature above — check the signature on the file itself (`Get-AuthenticodeSignature` in
PowerShell, or the file's Properties → Digital Signatures) and see the
[code signing policy](https://github.com/Open-MBEE/OpenSysML#code-signing-policy) for who may
approve a signature. macOS binaries are not Developer ID signed or notarized.

## Hardening a served instance

`sysml-grpc` and `sysml-mcp -http` bind loopback unless the address says otherwise. Before an
address other machines can reach:

- **`sysml-grpc` has no authentication.** Nothing in the service tells one caller from
  another: every client that can reach the address can call every RPC the capability list
  advertises, edits and runs included. Keep it on loopback, or reach it only through something
  that does authenticate — a reverse proxy or gateway, a VPN, a network policy — and never from
  an address the public internet routes to.
- **TLS.** `-tls-cert` and `-tls-key` (both or neither) serve every protocol over HTTPS on the
  one port, TLS 1.2 or later; it encrypts the connection and proves the server, not the client.
  [Service transports](../reference/service-transports.md) is the reference, including the
  browser-origin allow-list, which is not authentication either.
- **`sysml-mcp` requires a bearer token** on a non-loopback address — `-token-file`, `-token`
  or `OPENSYSML_MCP_TOKEN` — and refuses to start without one; every request must carry it.
  See [`sysml-mcp` § Transports](../reference/sysml-mcp.md#transports).
- A crafted model, project file, import or query must never execute or disclose anything on
  the host; a case where one does is in scope above, and a denial of service against a served
  instance is too.

## Advisories

Fixes are announced as
[GitHub Security Advisories](https://github.com/Open-MBEE/OpenSysML/security/advisories) on
this repository, with a CVE requested through GitHub, and the fix release's notes link the
advisory. Dependabot alerts and SonarCloud code-scanning results are watched by the code owners
on the deadlines in the policy above. Every release's notes end with the support period in
force and a link back to [Verifying a release](#verifying-a-release).
