# Reading installed packages

Status: §1 is shipped; §3 (a plain `lib/` beside `.project.json`, `@<version>` in a usage IRI) is
in progress. Nothing in this document is a package manager: OpenSysML has none and will not grow one.

**Division of labour.** OpenSysML (`sysml`, `sysml-lsp`, the REPL, the kernel, the MCP server) is
an analyser: it *reads* what a package manager installs and never fetches. Whichever package
manager a modeler uses — sysand today, any other tool that installs KerML §10.3 project-interchange
packages tomorrow — fetches, verifies, installs and locks. Supporting a package manager in
OpenSysML means one well-known install directory in `wellKnownLibraryDirs` plus whatever IRI scheme
its usages carry, read-only; nothing in the loader, LSP or validator reaches the network.

## 0. Goals and non-goals

Goals

- A modeler installs a dependency with their package manager, and the project it names is
  resolvable by `sysml`, `sysml-lsp`, the REPL, the kernel and the MCP server — which need no flag
  and no network to find it.
- Everything is the OMG format first: KerML §10.3 `.project.json`, `.meta.json`, `.kpar`. No
  tool-specific file is required to consume a package, and OpenSysML requires none to read one.
- Portable: any tool that reads the OMG format can consume the same installed tree with nothing
  OpenSysML-specific — no extension library, no registration, no bespoke metadata file.
- Tool-agnostic: any `.project.json` usage IRI that a package manager (sysand, a future OMG
  registry, a plain git remote or URL) answers must keep working once the project is on disk.
- OpenSysML stays a reader, so the analyser carries no opinion about where packages come from; it
  reads every package manager's installs alike.

Non-goals

- A client inside OpenSysML: no `sysml -add/-install/-lock`, no auto-download from `-validate` or the
  LSP, no lock file or package index code in this repository, ever.
- A lax mode: a directory or archive without `.project.json` is not a project and is an error,
  never a synthesized project.
- Replacing sysand for its users, or recommending one package manager over another.
- Binary/engine packaging (that is `bring-your-own-engines.md`).

## 1. What exists today (read side, shipped)

`internal/workspace/project/` — about 2300 lines of non-test Go, all spec-driven, no sysand code:

| Capability | Where | Notes |
|---|---|---|
| Parse `.project.json` (Table 12) | `interchange.go` `parseInterchange` | `name`/`version` mandatory; `publisher` read (sysand extension, optional); unknown keys kept |
| Parse/verify `.meta.json`, read `.kpar` in memory | `archive.go` `ParseArchive`, `ParseMeta`, `verify` | every checksum of a known algorithm verified before any member is read; path traversal, symlink escape and size bounds enforced |
| Write/unpack archives deterministically | `bundle.go`, CLI `-bundle`/`-unbundle` | byte-identical output; `SOURCE_DATE_EPOCH` honoured |
| Find installed projects | `discovery.go` `Discovery.Around` | nearest enclosing project + usage closure under well-known dirs (`.sysand/lib`) and `-library-root`/`OPENSYSML_LIBRARY_ROOTS`/LSP `libraryRoots` |
| Match usage → installed project | `interchange.go` `Uses`, `resourceSegments`, `slug` | last two IRI path segments = publisher/name, lower-case slugs; exact `versionConstraint` or latest (semver order) |
| OMG std-lib usages | `discovery.go` | `https://www.omg.org/spec/{KerML,SysML}/20250201/*.kpar` answered by the bundled library |
| Import-driven dependency search | `deps.go` | unresolved root namespace → project files → installed projects |
| Fixtures | `tests/kpar/testdata/{omg,sysand}` | the 10 OMG release archives; one sysand-laid-out project with a dir and a `.kpar` dependency |

What does not exist here, by design: anything that touches the network, writes `.project.json`,
resolves a version range, records a lock, publishes, or lists what is available — that is the
package manager's job, whichever one the modeler uses.

## 2. What sysand does (for interop, measured from its docs 2026-10)

- IRIs: `pkg:sysand/<publisher>/<name>[@version]`; `publisher` is a sysand extension field in `.project.json`.
- Install layout: `<project>/.sysand/lib/<publisher>-<name>_<version>/` or `.kpar`; std libs are never installed.
- Lock: `sysand-lock.toml` (identifiers list + sources list per project, checksums). OpenSysML
  never reads it; the installed tree is the interface.
- Config: `sysand.toml` (`[[index]] url`, `[[project]] identifiers/sources` overrides: `src_path`, `editable`, `kpar_path`, `remote_kpar`, `remote_src`).
- Index: static files, `<index>/<publisher>/<name>/versions.json` (+ per-version data); `sysand index init|add|yank|remove` builds one from `.kpar`s; sysand.com is the default index.
- Licence: `MIT OR Apache-2.0`, Rust. We import none of it and need not.

Interop contract we honour: resolve `pkg:sysand/...` IRIs (matched by slug) and keep reading
`.sysand/lib`.

## 3. What OpenSysML reads (the whole contract)

A package manager and OpenSysML meet only on disk, at the consuming project's `.project.json`:

- **Well-known install directories.** `lib/` beside `.project.json` joins `.sysand/lib` in
  `wellKnownLibraryDirs`; each holds installed projects as `<publisher>-<name>_<version>/`
  directories or `.kpar` files, found by their own `.project.json`, never by directory name. A project
  walking its own files skips its library directories, so `-validate proj/` does not load every
  installed version. `-library-root`/`OPENSYSML_LIBRARY_ROOTS` and `.kpar` files on the command line
  remain the explicit alternatives.
- **Usage IRIs.** `resource` is an IRI (Table 12); OpenSysML matches it to an installed project by
  the last two path segments (publisher/name slugs). This covers `pkg:sysand/<publisher>/<name>`,
  the Package-URL git type `pkg:git/<host>/<path>[@<version>][#<subpath>]` (purl is ECMA-427; a git
  remote is the identity, no provider vocabulary — `pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library@1.0.0`)
  and a plain URL alike, with `https://www.omg.org/spec/...kpar` answered by the bundled standard
  library. A purl `#<subpath>` fragment names a project directory within the repository the path
  names (`pkg:git/host/acme/widgets#packages/gears`): the installed project matched is the fragment's
  last segment under the repository's owner (`acme`/`gears`), read through its own installation alone.
- **Versions.** `versionConstraint` is honoured as today (exact, else latest installed by semver
  order). `@<version>` in the IRI is an exact constraint when the usage carries no
  `versionConstraint` (before this, `resourceSegments` discarded it and the latest installed release
  won). Which version lands under `lib/` is the package manager's decision at install time, made
  from its own lock; OpenSysML never reads a lock file and matches the usage against whatever is
  installed.
- **Verification.** `.meta.json` checksums, archive path traversal, symlink escape and size bounds
  are checked on the read side regardless of who wrote the tree.
- **Nothing else.** No fetch, no lock reading, no index, no `add`/`install` command, no network from
  `-validate`, the LSP, the REPL or the kernel. Editor affordances stay read-only: an unresolved root
  namespace is a diagnostic that names the project files and well-known directories searched.

Adding support for a further package manager is the same two items — its install directory in
`wellKnownLibraryDirs` and, if it has one, its IRI scheme in `resourceSegments` — plus a
`tests/kpar/testdata` fixture laid out the way that tool installs.

## 4. What publishing a package takes (for our own libraries)

The Extensions Library (`Open-MBEE/OpenSysML-Extensions-Library`) is published as a project any
package manager can install: a root `.project.json` (`publisher` Open-MBEE, `version` equal to the
release tag) and `.meta.json`, `build-kpar.py` deriving the `.kpar` archive from them, and a
release job asserting tag == version. Nothing OpenSysML-specific is required to consume it, and
`sysml -validate` resolves it from `lib/` or `.sysand/lib` with no flag once installed.

## 5. Decisions

Settled: OpenSysML is not a package manager and never fetches; it reads what package managers
install and adds support for further ones as they appear (install directory + IRI scheme,
read-only), never a downloader, lock or index of its own.

Settled: `lib/` beside `.project.json` is read as a well-known install directory next to
`.sysand/lib`; `@<version>` in a usage IRI is an exact constraint; no lax mode.

Settled: usage IRIs are read as IRIs — `pkg:sysand/...`, the ECMA-427 `pkg:git/...` type and plain
URLs are matched the same way; OpenSysML defines no IRI scheme of its own.
