# Downloads

Everything ships from [GitHub Releases](https://github.com/Open-MBEE/OpenSysML/releases)
— `sysml` (CLI and REPL), `sysml-lsp` (the language server) and `sysml-grpc` (the
service) in every archive. Each release publishes `SHA256SUMS.txt` with a digest per
asset; the [install guide](guide/01-install.md) covers verification, Gatekeeper on
macOS, and the optional SMT solver.

## Stable release

| Platform | Get it |
|---|---|
| Linux x64 / arm64 | `opensysml-linux-amd64.tar.gz` / `opensysml-linux-arm64.tar.gz` |
| macOS (recommended) | `brew install Open-MBEE/tap/opensysml` — installs the SMT solver too |
| macOS direct | `opensysml-darwin-arm64.tar.gz` (Apple Silicon) / `opensysml-darwin-amd64.tar.gz` (Intel) |
| Windows | `opensysml-<x.y.z>-windows-amd64.msi` installer (bundles Z3, adds `sysml` to `PATH`), or `opensysml-windows-amd64.zip` for a portable unpack |

## From package managers and toolchains

- **Python:** `pip install opensysml` ([PyPI](https://pypi.org/project/opensysml/)) —
  published on every release at the core's version.
- **Go:** `go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest`, or import
  `client/opensysml` for the [Go client](reference/api.md).
- **VS Code:** the extension is not on any marketplace — grab the prebuilt
  `opensysml-sysml.vsix` from the [nightly release](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly)
  and run `code --install-extension opensysml-sysml.vsix`, or build it from
  `editors/vscode` and side-load it, per the [editors guide](guide/08-editors.md).
- **Node, Java, Rust, Julia, MATLAB:** the [client libraries](reference/clients.md)
  exist but are not yet published to npm, Maven, crates.io or a Julia registry — pin a
  release tag or build from source.

## Nightly

The [nightly snapshot](project/nightly.md) rebuilds `develop` every night as a
prerelease, for trying what landed since the last release.
