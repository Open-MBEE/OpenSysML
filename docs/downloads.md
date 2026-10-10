---
description: Download OpenSysML release binaries, nightly builds and client packages.
---

# Downloads

Stable links below follow the latest release. The [install guide](guide/01-install.md)
covers checksum and signature verification, Gatekeeper on macOS, and the optional SMT solver.

## Install script

One command on every platform: it picks the bundle below for this machine, verifies it
against `SHA256SUMS.txt` and installs `sysml` and `sysml-lsp`. The
[install guide](guide/01-install.md#with-the-install-script) lists its options — a pinned
release or the nightly, `sysml-grpc` as well, another directory.

```bash
curl -fsSL https://opensysml.org/install.sh | sh        # Linux and macOS
```
```powershell
irm https://opensysml.org/install.ps1 | iex             # Windows
```

## Stable release

| Platform | Bundle | Note |
|---|---|---|
| Linux x64 | [opensysml-linux-amd64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-amd64.tar.gz) | Contains `sysml` and `sysml-lsp`. |
| Linux arm64 | [opensysml-linux-arm64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-arm64.tar.gz) | Contains `sysml` and `sysml-lsp`. |
| macOS Apple Silicon | [opensysml-darwin-arm64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-darwin-arm64.tar.gz) | Contains `sysml` and `sysml-lsp`; Homebrew also installs Z3. |
| macOS Intel | [opensysml-darwin-amd64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-darwin-amd64.tar.gz) | Contains `sysml` and `sysml-lsp`; Homebrew also installs Z3. |
| macOS · Homebrew | `brew install Open-MBEE/tap/opensysml` | Recommended; installs the SMT solver too. |
| Windows x64 | [MSI installer on the latest release page](https://github.com/Open-MBEE/OpenSysML/releases/latest) · [portable ZIP](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-windows-amd64.zip) | The MSI bundles Z3 and adds `sysml` to `PATH`; the portable ZIP contains `sysml` and `sysml-lsp`. |

## Individual tools

Each archive contains only the named tool. The `sysml-grpc` service and the
`sysml-jupyter-kernel` Jupyter kernel are separate downloads; most
[client libraries](reference/clients.md) fetch the service themselves, and
`pip install jupyter-opensysml-kernel` bundles the kernel for the platform (see the
[Jupyter chapter](guide/12-jupyter.md)). Both are raw binaries, each with a sidecar SHA-256 file.

| Tool | Linux x64 | Linux arm64 | macOS Intel | macOS Apple Silicon | Windows x64 |
|---|---|---|---|---|---|
| `sysml` | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-linux-amd64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-linux-arm64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-darwin-amd64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-darwin-arm64.tar.gz) | [ZIP](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-windows-amd64.zip) |
| `sysml-lsp` | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-lsp-linux-amd64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-lsp-linux-arm64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-lsp-darwin-amd64.tar.gz) | [tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-lsp-darwin-arm64.tar.gz) | [ZIP](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-lsp-windows-amd64.zip) |
| `sysml-grpc` | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-linux-amd64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-linux-amd64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-linux-arm64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-linux-arm64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-darwin-amd64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-darwin-amd64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-darwin-arm64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-darwin-arm64.sha256) | [EXE](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-windows-amd64.exe) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-grpc-windows-amd64.exe.sha256) |
| `sysml-jupyter-kernel` | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-linux-amd64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-linux-amd64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-linux-arm64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-linux-arm64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-darwin-amd64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-darwin-amd64.sha256) | [binary](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-darwin-arm64) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-darwin-arm64.sha256) | [EXE](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-windows-amd64.exe) · [SHA-256](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/sysml-jupyter-kernel-windows-amd64.exe.sha256) |

## Verify

| File | Purpose |
|---|---|
| [SHA256SUMS.txt](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/SHA256SUMS.txt) | Digests for release assets. |
| [SHA256SUMS.txt.bundle](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/SHA256SUMS.txt.bundle) | Signed checksum bundle. |
| [provenance.intoto.json](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/provenance.intoto.json) | Build provenance. |
| [provenance.intoto.json.bundle](https://github.com/Open-MBEE/OpenSysML/releases/latest/download/provenance.intoto.json.bundle) | Signed provenance bundle. |
| Windows installer | Unsigned MSI digest: `SHA256SUMS-windows-msi.txt`; signed MSI digest: `SHA256SUMS-windows-signed.txt`. Both are on the release page. |

After downloading release assets into the same directory, verify their checksums with:

```bash
sha256sum -c --ignore-missing SHA256SUMS.txt
```

For signature verification with cosign, see the [install guide](guide/01-install.md).

## Package managers

- **Python:** [`opensysml` on PyPI](https://pypi.org/project/opensysml/) — `pip install opensysml`.
- **Node:** [`@openmbee/opensysml` on npm](https://www.npmjs.com/package/@openmbee/opensysml) —
  `npm install @openmbee/opensysml`; its per-platform packages carry `sysml-grpc`.
- **Rust:** [`opensysml` on crates.io](https://crates.io/crates/opensysml) —
  `opensysml = "0.9"`.
- **Go:** `go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest`, or use the
  [Go client](reference/api.md).
- **Java:** not on Maven Central; build from a checkout with
  `make build && mvn -f client/java/pom.xml install` ([Java client](reference/java-api.md)).
- **Julia:** not in the General registry; develop from a checkout with
  `Pkg.develop(path="client/julia/OpenSysML")` ([Julia client](reference/julia-api.md)).
- **MATLAB:** source files; add `client/matlab` to the MATLAB path
  ([MATLAB client](reference/matlab-api.md)).
- **VS Code:** the extension is not on a marketplace. Download the
  [nightly VSIX](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-sysml.vsix)
  and run `code --install-extension opensysml-sysml.vsix`, or build and side-load it per the
  [editors guide](guide/08-editors.md).

See [all releases](https://github.com/Open-MBEE/OpenSysML/releases).

## Nightly

Nightly bundles rebuild `develop`. They contain `sysml` and `sysml-lsp` but have no MSI or provenance.
The install script installs one with `--version nightly` (`-Version nightly` on Windows).

| Platform | Bundle |
|---|---|
| Linux x64 | [opensysml-linux-amd64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-linux-amd64.tar.gz) |
| Linux arm64 | [opensysml-linux-arm64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-linux-arm64.tar.gz) |
| macOS Apple Silicon | [opensysml-darwin-arm64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-darwin-arm64.tar.gz) |
| macOS Intel | [opensysml-darwin-amd64.tar.gz](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-darwin-amd64.tar.gz) |
| Windows x64 | [opensysml-windows-amd64.zip](https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-windows-amd64.zip) |

See the [nightly snapshot guide](project/nightly.md) and [all releases](https://github.com/Open-MBEE/OpenSysML/releases).
