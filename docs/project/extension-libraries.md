# Extension libraries — pin, sync and engine contract

The non-normative OpenSysML extension libraries (`OpenSysMLMathFunctions`,
`DocumentQueries`, `IdentityMetadata`, `DiagramLayout`, `OOSEM`, `MOSA`,
`StateSpaceIntegration`, `Stochastic`, `RandomFunctions`, `Simulation`,
`AnalysisRecords`, `MigrationMetadata`, `OpenSysMLRenderings`, `StateActivity`,
`StateMachines` and `SysMLValidation`)
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
  upstream checkout. An upstream holding no libraries, no
  `engine-contract.json` or no project manifests is refused, and a file
  the upstream does not carry — `README.md` aside, which each vendored
  directory owns — is an error, so edits are never made here: they go
  upstream, the pin is bumped, and the sync re-runs.
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
  library snapshot and runs the whole suite in both Go modules (`./...` and
  `tools`), so an upstream change that
  breaks OpenSysML surfaces within a day. It is not a required check.

## The published project

Upstream is a KerML 10.3 project interchange. Its root holds the two manifests:

- `.project.json` (KerML 1.0 Table 12): `name` "OpenSysML Extensions Library",
  `publisher` "Open-MBEE", `version` equal to the release tag, `license`,
  `website`, and as `usage` the nine OMG 20250201 standard-library archives the
  units depend on.
- `.meta.json` (Table 13): `index` from each root namespace to its unit at
  `libraries/<Unit>.sysml|.kerml`, `metamodel`, `created`, and a SHA-256
  `checksum` per unit and for `LICENSE`.

The tagged tree is the project; the archive is derived from it.
`scripts/build-kpar.py` upstream writes `opensysml-extensions-library-<version>.kpar`
holding exactly the two manifests and the files `.meta.json` records, at their
repository paths (`libraries/…`, `LICENSE`), and a release attaches it to the
`v<version>` tag.

A consumer declares the library in its own `.project.json` by the repository's
IRI with the version after `@` — the ECMA-427 package URL a package manager
writes, or its `pkg:git/` alias, read alike:

```json
{"usage": [{"resource": "pkg:generic/OpenSysML-Extensions-Library?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FOpen-MBEE%2FOpenSysML-Extensions-Library%401.0.0"}]}
{"usage": [{"resource": "pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library@1.0.0"}]}
```

Whatever installs it puts the tagged tree, or the archive, under `lib/` or
`.sysand/lib` beside that `.project.json` — conventionally
`lib/open-mbee-opensysml-extensions-library_1.0.0/` or
`lib/open-mbee-opensysml-extensions-library_1.0.0.kpar` — and OpenSysML reads
it from there with no flag: the usage is matched to the installed project by
the slugs of the IRI's last two path segments against the manifest's
`publisher` and `name`, which is why the project is named so that its slug is
the repository's. An archive's checksums are verified on reading; a changed
unit refuses the archive with a warning on its path. OpenSysML also bundles the
same sixteen units, so an import resolves either way — the files a load lists
(`-validate` on the project, `:load` in the REPL) tell which copy answered.
`tests/kpar/extensions_test.go` lays this out end to end, in both forms, from
the vendored copy.

### The vendored copy is the published project

`scripts/sync-extension-libraries.sh` vendors the manifests and the `LICENSE`
they record into `internal/workspace/libs/extensions-project/`, beside the
units it writes flat into `stdlib/OpenSysML Libraries/`, and
`internal/workspace/libs/extensions_project_test.go` holds the two to each
other: every vendored unit's SHA-256 equals the checksum `.meta.json` records
for `libraries/<Unit>`, every recorded unit is vendored, and each unit declares
exactly the root namespaces the `index` names for it. The identity comes from
the pin — a manifest edited by hand is drift `--check` reports — so the bundled
copy is provably the project at the pinned commit, not a copy that resembles
it.

The manifests do not live beside the units because the units are vendored flat
(the loader keys them as `OpenSysML Libraries/<Unit>`, not `libraries/<Unit>`)
and because a `.project.json` there would make the standard-library directory
read as an installed project when a developer opens a vendored unit from disk;
the Go embed of `stdlib` skips dotfiles, so the loader itself would not have
noticed. A directory holding only the manifests is never a load input.
