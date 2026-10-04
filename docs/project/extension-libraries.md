# Extension libraries — pin, sync and engine contract

The non-normative OpenSysML extension libraries (`OpenSysMLMathFunctions`,
`DocumentQueries`, `IdentityMetadata`, `DiagramLayout`, `OOSEM`, `MOSA`,
`StateSpaceIntegration`, `Stochastic`, `RandomFunctions`, `Simulation`,
`AnalysisRecords`, `MigrationMetadata`, `StateMachines` and `SysMLValidation`)
are maintained upstream at
[Open-MBEE/OpenSysML-Extensions-Library](https://github.com/Open-MBEE/OpenSysML-Extensions-Library),
with their history imported from this repository. Other tools get them from
that repository or from the `.kpar` project archive each upstream release
attaches.

OpenSysML consumes them as a vendored copy at
`internal/workspace/libs/stdlib/OpenSysML Libraries/`:

- `scripts/extension-libraries-pin.sh` records the repository and the commit
  the copy was taken from (`EXTENSIONS_REPO`, `EXTENSIONS_REF`,
  `EXTENSIONS_COMMIT`, each environment-overridable).
- `scripts/sync-extension-libraries.sh` rewrites the copy from the pin.
  `--check` reports drift without writing (run by `make
  extension-libraries-check`, which CI runs on every pull request); `--ref
  REF` syncs a branch or tag instead of the pin; `--source DIR` syncs a local
  upstream checkout. An upstream holding no libraries is refused, and a file
  the upstream does not carry — `README.md` aside, which the vendored copy
  owns — is an error, so edits are never made here: they go upstream, the pin
  is bumped, and the sync re-runs.
- `engine-contract.json`, synced beside the libraries, is the machine-read
  contract: the qualified names the engine binds — the functions the runtime
  registry implements, the metadata and features it reads, the document
  elements the doc and query plans compile, and the names the SysML v1
  migrator writes. `internal/exec/runtime/engine_contract_test.go` holds the
  registry to the manifest's `function` entries;
  `tests/hygiene/engine_contract_test.go` derives every extension-qualified
  name the code binds and requires it to be a manifest entry. Breaking the
  contract upstream requires a `contract` bump, a coordinated OpenSysML pull
  request that re-pins, and a MAJOR upstream release.
- `.github/workflows/extension-libraries-upstream.yml` is the advisory
  nightly: it syncs the upstream `main` (or a dispatch-given ref), checks the
  library snapshot and runs the whole suite, so an upstream change that
  breaks OpenSysML surfaces within a day. It is not a required check.
