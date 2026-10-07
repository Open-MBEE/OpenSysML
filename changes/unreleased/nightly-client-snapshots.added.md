- **Nightly snapshots of the Python and Node clients.** The nightly workflow now builds
  `opensysml` and `@openmbee/opensysml` (with its five platform packages and the WASM
  package) from the same commit as the binaries and publishes them to PyPI and npm through
  trusted publishing, as development versions a default install never picks up:
  `<next release>.dev<yyyymmdd>` on PyPI, `<next release>-nightly.<yyyymmdd>.g<commit>`
  under the `nightly` dist-tag on npm, both derived from the version the tree declares by
  `client/python/scripts/snapshot_version.py`. Each night is kept for 14 days as the
  prerelease `nightly-<yyyymmdd>-<commit>`, whose `sysml-grpc` the wheel pins by digest and
  installs on its own; `nightly` remains the moving alias of the newest night, and nothing is
  marked latest. See [Nightly snapshots](https://opensysml.opensysml.org/project/nightly/).
