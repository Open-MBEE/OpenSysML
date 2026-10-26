# Package management over git

A proposal: nothing below §1 is implemented yet.

## 0. Goals and non-goals

Goals

- A modeler adds a usage to `.project.json`, runs one command, and the project it names is
  installed, locked and resolvable by `sysml`, `sysml-lsp`, the REPL, the kernel and the MCP server.
- Everything is the OMG format first: KerML 1.0 §10.3 `.project.json`, `.meta.json`, `.kpar`.
  No provider-specific file is required to consume a package.
- Git is the storage and transport: a package is a git repository (or a path in one) whose tagged
  commit *is* the project directory; the index is a git repository too.
- Portable: any tool that reads the OMG format can consume a package with nothing OpenSysML-specific —
  no extension library, no registration, no bespoke metadata file. Publishing = tagging a git repository.
- The index is optional and derived. Its machine form is plain JSON; its SysML form uses the standard
  library only, and the invariants are checked by `sysml` itself — the "SysML-powered" part is our
  gate and search surface, never a consumer requirement.
- Provider-agnostic: any `.project.json` usage IRI that another tool (sysand, a future OMG
  registry, a plain URL) answers must keep working; our index must be consumable by those tools.

Non-goals (this phase)

- A lax mode: a repository without `.project.json` at the tag is not a package and is an error,
  never a synthesized project.

- A hosted account/token service. The git host is the account system.
- Replacing sysand for its users; interoperating with it is in scope, re-implementing its CLI is not.
- Binary/engine packaging (that is `bring-your-own-engines.md`).

## 1. What exists today (read side, shipped)

`internal/workspace/project/` — about 2300 lines of non-test Go, all spec-driven, no sysand code:

| Capability | Where | Notes |
|---|---|---|
| Parse `.project.json` (Table 12) | `interchange.go` `parseInterchange` | `name`/`version` mandatory; `publisher` read (sysand extension, optional); unknown keys kept |
| Parse/verify `.meta.json`, read `.kpar` in memory | `archive.go` `ParseArchive`, `ParseMeta`, `verify` | every checksum of a known algorithm verified before any member is read |
| Write/unpack archives deterministically | `bundle.go`, CLI `-bundle`/`-unbundle` | byte-identical output; `SOURCE_DATE_EPOCH` honoured |
| Find installed projects | `discovery.go` `Discovery.Around` | nearest enclosing project + usage closure under well-known dirs (`.sysand/lib`) and `-library-root`/`OPENSYSML_LIBRARY_ROOTS`/LSP `libraryRoots` |
| Match usage → installed project | `interchange.go` `Uses`, `resourceSegments`, `slug` | last two IRI path segments = publisher/name, lower-case slugs; exact `versionConstraint` or latest (semver order) |
| OMG std-lib usages | `discovery.go` | `https://www.omg.org/spec/{KerML,SysML}/20250201/*.kpar` answered by the bundled library |
| Import-driven dependency search | `deps.go` | unresolved root namespace → project files → installed projects |
| Fixtures | `tests/kpar/testdata/{omg,sysand}` | the 10 OMG release archives; one sysand-laid-out project with a dir and a `.kpar` dependency |

What does not exist: anything that touches the network, writes `.project.json`, resolves a
version range, records a lock, publishes, or lists what is available. The Extensions Library
repo has no `.project.json` and no release with a `.kpar` (the docs say releases attach one —
there is no release yet), so our own libraries are not installable as a package today.

## 2. What sysand does (for interop, measured from its docs 2026-10)

- IRIs: `pkg:sysand/<publisher>/<name>[@version]`; `publisher` is a sysand extension field in `.project.json`.
- Install layout: `<project>/.sysand/lib/<publisher>-<name>_<version>/` or `.kpar`; std libs are never installed.
- Lock: `sysand-lock.toml` (identifiers list + sources list per project, checksums).
- Config: `sysand.toml` (`[[index]] url`, `[[project]] identifiers/sources` overrides: `src_path`, `editable`, `kpar_path`, `remote_kpar`, `remote_src`).
- Index: static files, `<index>/<publisher>/<name>/versions.json` (+ per-version data); URL templates for GitLab/Gitea raw APIs; `sysand index init|add|yank|remove` builds one from `.kpar`s; sysand.com is the default index with accounts, orgs, tokens, trusted publishing.
- Licence: `MIT OR Apache-2.0`, Rust. We import none of it and need not.

Interop contract we should honour: resolve `pkg:sysand/...` IRIs (already matched by slug), keep
reading `.sysand/lib`, and be able to *emit* the static index layout so a sysand user can
`--index https://.../our-index` and install our packages.

## 3. Proposed design

### 3.1 Package = a git repository at a tag (nothing else)

- A package root holds `.project.json` (KerML §10.3 Table 12; `publisher` as the optional extension
  field sysand also uses), the model files, `LICENSES/`, README. `.meta.json` is generated at build
  (`sysml -bundle` does it) — committed or not, the installer regenerates and verifies.
- A release is an annotated tag `v<semver>` (bare `<semver>` accepted) whose tree is the project
  directory; `version` in `.project.json` must equal the tag or the install fails. A `.kpar` release
  asset is optional and derived; when present its SHA-256 is what the lock and index record.
- Sub-directory packages (monorepo) are allowed: `pkg:git/<host>/<path>@<version>#<subpath>` (purl subpath).
- **No `.project.json` at the tag → error** ("not a project interchange: <repo>@<tag> has no .project.json").
  No lax mode, no synthesized project.
- Identity: `pkg:git/<host>/<path>[@<version>][#<subpath>]` — the git remote itself, no provider
  vocabulary (`pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library@1.0.0`); a plain clone URL
  with `@<version>` (`https://host/path.git@1.0.0`, `ssh://git@host/path@1.0.0`) is accepted and
  normalized to it. Any host that speaks git is a package host: GitHub, GitLab, Gitea, a bare server,
  a local path. Matching against installed projects already works (`resourceSegments` keeps the last
  two path segments → `org`/`repo` slugs), so a usage written this way resolves today once the
  project is on disk — but `resourceSegments` discards the `@<version>` and `pickInstalled` honours
  only `versionConstraint`, so today a versioned IRI alone selects the latest installed release;
  Phase 1 makes `@<version>` an exact constraint when the usage carries none, and the lock entry
  wins over either. `pkg:sysand/<publisher>/<name>` and
  `https://www.omg.org/spec/...kpar` keep resolving as they do now.
- Resolution needs **no index** and nothing but git: tags come from `git ls-remote --tags`
  (`v<version>`, then `<version>`), the tree from `git clone --depth 1 --branch <tag>` (or
  `git archive --remote` where the server allows it), and the commit id is what the lock records.
  A host's archive-by-tag URL (GitHub `/archive/refs/tags/<tag>.tar.gz`, GitLab `/-/archive/<tag>/`)
  is only an internal fast path keyed by hostname, falling back to git; it never appears in an
  identity or in the lock. Any tool can do the same with no OpenSysML involvement — that is the
  portability contract.

### 3.2 Index = optional, derived, two renderings of the same facts

The index exists for discovery (search, "who provides namespace X"), for mirror/yank/trust facts a
repo cannot state about itself, and to serve sysand clients. Nothing requires it to install.

- Source of truth stays the packages' own `.project.json` at their tags. The index repository
  (proposed `Open-MBEE/opensysml-index`) holds one record per release, *generated* from them by
  `sysml -index add pkg:git/host/org/repo@1.2.0` (fetch, verify, record commit + checksums + usages).
- **Machine rendering:** `<publisher>/<name>/versions.json` in sysand's static-index layout plus a
  flat `index.json` — plain JSON, served as static files (e.g. GitHub Pages) or read raw from the repo. A sysand user
  points `--index` at it; a shell script can grep it.
- **Model rendering:** `index/<org>/<repo>.sysml`, standard library only — no extension library,
  no metadata definitions of ours to vendor:

```sysml
package 'Open-MBEE/OpenSysML-Extensions-Library' {
    private import ScalarValues::*;
    item 'v1.0.0' : Index::Release {
        :>> iri = "pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library";
        :>> version = "1.0.0";  :>> tag = "v1.0.0";  :>> commit = "5134a14d…";
        :>> sha256 = "…";  :>> license = "Apache-2.0";
        :>> uses = ("https://www.omg.org/spec/KerML/20250201/Semantic-Library.kpar", …);
    }
}
```

  `Index::Release` is one `item def` with `attribute` usages typed by `ScalarValues` and a handful of
  `constraint` usages, defined in `index/Index.sysml` **inside the index repo** as ordinary model text —
  any conformant SysML v2 parser reads the whole index with the OMG standard library alone. The
  constraints (semver well-formed and monotonic per package, `version == tag`, every `uses` IRI
  resolvable within the index or the OMG set, SPDX licence expression, checksum present) are what
  `sysml -self-check-package Index index/` evaluates in the index repo's CI (bare `-validate` only
  checks that the model analyses cleanly; `-self-check-package` applies the named package's
  `constraint def`s to every classified element and fails on a false or erroring one); `DocumentQueries` views render the
  catalogue page; `-query`/MCP answer searches. Both renderings are produced from the same records by
  the same command, and CI fails if they disagree.
- Writes to the index are PRs (human-gated) or a `publish` workflow in the package repo that opens the
  PR on release; the gate runs `-index check` (validate + resolve the closure).
- Later option: the same model pushed to Flexo MMS through the existing SysML v2 API sync, so a live
  server-side index is the same model, not a second schema.

### 3.3 Client, in `sysml` (no new binary)

| Command | Does |
|---|---|
| `sysml -project-init [-publisher P -name N]` | writes `.project.json` (+ `.meta.json`) |
| `sysml -add <iri>[@constraint]` | appends a usage, resolves, installs, locks |
| `sysml -install` / `-sync` | resolves `.project.json` + lock, installs missing/changed projects |
| `sysml -lock` | resolves to exact versions/commits/checksums, writes the lock |
| `sysml -bundle` (exists) / `-publish` | builds the `.kpar`, checks tag == version, uploads the asset, opens the index PR |
| `sysml -index check|add|yank` | maintains an index checkout; `check` = validate + closure |

- Resolution order per usage: lock entry → direct source (`pkg:git/...` or a git URL: tags via
  `ls-remote`) → configured indexes (`-index`, `OPENSYSML_INDEX`; ours by default, sysand static indexes
  readable as-is) for `pkg:sysand/...` and for search. Version constraints: exact today; add
  caret/tilde/ranges with the semver comparator already in `interchange.go`.
- Fetch and verify: `git` is the protocol (shell out — honours the user's credential helpers, no new
  dependency); a host archive over HTTPS is tried first where the hostname is known to serve one, with
  `git` as the fallback. Verify the tree against the recorded commit / kpar SHA-256
  and the `.meta.json` checksums before anything is read (the archive reader already refuses on a
  checksum mismatch).
- Install layout: `lib/<publisher>-<name>_<version>/` (or `.kpar`) beside `.project.json` — a neutral
  name, nothing tool-specific since the reader goes by `.project.json`. Phase 1 adds `lib` to
  `wellKnownLibraryDirs` (today it lists only `.sysand/lib`) so both are read and a sysand-installed
  project keeps working unchanged.
- Lock: no OMG format exists. `.project.lock.json` beside `.project.json` (same JSON tooling as
  `.meta.json`, one entry per usage: resolved IRI, version, source URL, commit, SHA-256); read
  `sysand-lock.toml` as an input when present for interop, never write it.
- Editors: unresolved root namespace → LSP code action "add usage pkg:git/…" backed by the index
  search; `%project` in the REPL/kernel shows the closure.
- Trust: the index PR gate plus, later, sigstore attestation of the `.kpar` from the package repo's CI
  identity — the `client/python/opensysml/signing.py` cosign-keyless pattern already used for our
  release manifests, applied per package and verified on install.

### 3.4 What is and is not OpenSysML-specific

| | Needed to publish | Needed to consume |
|---|---|---|
| `.project.json` at a tag (OMG) | yes | yes |
| `.kpar` asset | no (optional) | no |
| Index entry | no | no (search/yank/trust only) |
| `Index.sysml` definitions | no | no (read with the OMG stdlib if you want the model view) |
| `.project.lock.json` | no | no (OpenSysML's own reproducibility record) |

## 4. Phasing

Phase 1 — installable packages from git remotes, no index
1. Extensions Library: add `.project.json` (`publisher` Open-MBEE), tag `v1.0.0`; release job attaches
   the `.kpar` built by the existing `scripts/build-kpar.py` (the docs already promise it).
2. `sysml -project-init`, `-add`, `-install`, `-lock` for direct sources (`pkg:git/...`,
   git URL, remote `.kpar`, local path/editable) with `.project.lock.json`; tag == version check;
   "no .project.json" error.
3. Version-range constraints in `satisfies`.
Gate: install the Extensions Library, ode4hera Requirements Management and MBSE4U SYSMOD from their
git repos (SYSMOD's repo already carries `.project.json`/`.meta.json` at root) into a fresh
project and validate what `tests/kpar` validates today.

Phase 2 — the index
4. `opensysml-index` repo: `Index.sysml` definitions, records for our libraries, CI (`sysml -validate`,
   closure check, generated `versions.json`/`index.json`, Pages).
5. `sysml -index check|add|yank`; index-backed resolution and search; `-publish` opening the index PR.
6. LSP "add usage" code action; `%project` in REPL/kernel.

Phase 3 — ecosystem
7. Mirror records for the public sysand.com packages (read-only, pointing at their sources) so one
   index answers both; measure parser/validator fallout over every package.
8. Attestation (cosign keyless from package CI) verified on install.
9. Flexo-served index via the SysML v2 API sync.

## 5. Decisions

Settled: `pkg:git/<host>/<path>@<version>` identity (git is the protocol, no provider list); no extension library (index definitions live in
the index repo, stdlib-only); no lax mode.

Settled: install dir `lib/` beside `.project.json` (`.sysand/lib` still read); lock `.project.lock.json`;
fetch = `git`, with a host archive over HTTPS as an internal fast path.

Open:
1. Index repo name/location, and PR-gated only vs also CI-published from package releases.
2. Whether Phase 3 mirrors sysand.com records at all.
