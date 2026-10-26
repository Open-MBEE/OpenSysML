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
- A release is a tag `v<semver>` (bare `<semver>` accepted) whose tree is the project directory
  (git/hg: an annotated tag; svn: a copied directory under `tags/`, by the usual convention);
  `version` in `.project.json` must equal the tag or the install fails. A `.kpar` release
  asset is optional and derived; when present its SHA-256 is what the lock and index record.
- Sub-directory packages (monorepo) are allowed: `pkg:<vcs>/<host>/<path>@<version>#<subpath>` (purl subpath).
- **No `.project.json` at the tag → error** ("not a project interchange: <repo>@<tag> has no .project.json").
  No lax mode, no synthesized project.
- Identity: `pkg:<vcs>/<host>/<path>[@<version>][#<subpath>]` with `<vcs>` one of `git`, `svn`,
  `hg` — the repository itself, no provider vocabulary
  (`pkg:git/github.com/Open-MBEE/OpenSysML-Extensions-Library@1.0.0`); a plain clone URL with
  `@<version>` (`https://host/path.git@1.0.0`, `ssh://git@host/path@1.0.0`) is accepted and
  normalized to it. Any host that speaks one of the three is a package host: GitHub, GitLab, Gitea,
  a bare server, an svn or hg server, a local path. `pkg:github/...`-style provider types are
  refused, and so is `pkg:cvs/...` (per-file tags, no tree revision: not supportable). Matching against installed projects already works (`resourceSegments` keeps the last
  two path segments → `org`/`repo` slugs), so a usage written this way resolves today once the
  project is on disk — but `resourceSegments` discards the `@<version>` and `pickInstalled` honours
  only `versionConstraint`, so today a versioned IRI alone selects the latest installed release;
  Phase 1 makes `@<version>` an exact constraint when the usage carries none, and the lock entry
  wins over either. `pkg:sysand/<publisher>/<name>` and
  `https://www.omg.org/spec/...kpar` keep resolving as they do now.
- Resolution needs **no index** and nothing but the user's VCS executable: tags come from
  `git ls-remote --tags` / `hg identify -r <tag>` / `svn ls <url>/tags` (`v<version>`, then
  `<version>`), the tree from `git clone --depth 1 --branch <tag>` (or `git archive --remote`
  where the server allows it) / `hg clone -r <tag>` + `hg archive` / `svn export <url>/tags/<tag>`,
  and the revision (commit sha, changeset id, svn last-changed revision) is what the lock records
  next to one VCS-independent tree digest. A host's archive-by-tag URL (GitHub
  `/archive/refs/tags/<tag>.tar.gz`, GitLab `/-/archive/<tag>/`) is only an internal fast path keyed
  by hostname, falling back to the VCS; it never appears in an identity or in the lock. Any tool can do the same with no OpenSysML involvement — that is the
  portability contract.

### 3.2 Index = optional, derived, two renderings of the same facts

The index exists for discovery (search, "who provides namespace X"), for mirror/yank/trust facts a
repo cannot state about itself, and to serve sysand clients. Nothing requires it to install.

- Source of truth stays the packages' own `.project.json` at their tags. The index repository
  (proposed `Open-MBEE/opensysml-index`) holds one record per release, *generated* from them by
  `syspi index add pkg:git/host/org/repo@1.2.0` (fetch, verify, record revision + checksums + usages).
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
        :>> version = "1.0.0";  :>> tag = "v1.0.0";  :>> vcs = "git";  :>> revision = "5134a14d…";
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
  PR on release; the gate runs `syspi index check` (validate + resolve the closure) and `sysml -self-check-package`.
- Later option: the same model pushed to Flexo MMS through the existing SysML v2 API sync, so a live
  server-side index is the same model, not a second schema.

### 3.3 Client: SysPI, a separate tool (OpenSysML stays read-only)

The client is **SysPI — the SysML Package Installer** (`Open-MBEE/SysPI`, binary `syspi`), a
second interoperable package manager beside sysand, not a mode of `sysml`. OpenSysML only *reads*
what a package manager installs (§1); it never touches the network, writes `.project.json`, or
records a lock. The whole contract between the two tools is the on-disk layout: `lib/` beside
`.project.json` and `@<version>` in a usage IRI as an exact constraint. Any reader that honours the
OMG format gets the same benefit, and the two implementations check each other — sysand and
OpenSysML already stand in that relation.

| Command | Does |
|---|---|
| `syspi init [-publisher P -name N]` | writes `.project.json` (+ `.meta.json`) |
| `syspi add <iri>[@constraint]` | appends a usage, resolves, installs, locks |
| `syspi install` / `sync` | resolves `.project.json` + lock, installs missing/changed projects |
| `syspi lock` | resolves to exact versions/revisions/checksums, writes the lock |
| `syspi bundle <dir>` / `publish` | builds the `.kpar` (`sysml -bundle` does the same, byte-identical), checks tag == version, uploads the asset, opens the index PR |
| `syspi index emit|check|add|yank` | emits the static JSON index for a set of packages; maintains an index checkout; `check` = validate + closure |

- Resolution order per usage: lock entry → direct source (`pkg:git/...`, `pkg:svn/...`,
  `pkg:hg/...` or a clone URL: tags listed from the repository) → configured indexes (ours by default, sysand static indexes readable as-is) for
  `pkg:sysand/...` and for search. Version constraints: exact, caret, tilde and comparison ranges
  (semver); `@<version>` in the IRI with no `versionConstraint` is an exact constraint; a lock entry
  wins over both. OpenSysML's reader needs only exact-or-latest plus the `@<version>` pick.
- Fetch and verify: the VCS is the protocol — shell out to the user's `git`, `svn` or `hg` (honours
  their credential helpers and SSH config, no new dependency) behind one source interface per VCS;
  a host archive over HTTPS is tried first where the hostname is known to serve one, with the VCS as
  the fallback. Verify the tree against the recorded tree digest / kpar SHA-256 (the revision alone
  is not content-addressed for svn)
  and the `.meta.json` checksums before anything is read (OpenSysML's archive reader already refuses
  on a checksum mismatch, so a tampered install fails on both sides); path traversal, symlink escape
  and size bounds are enforced on every archive and clone.
- Install layout: `lib/<publisher>-<name>_<version>/` (or `.kpar`) beside `.project.json` — a neutral
  name, nothing tool-specific since the reader goes by `.project.json`. Phase 1 adds `lib` to
  OpenSysML's `wellKnownLibraryDirs` (today it lists only `.sysand/lib`) so both are read and a
  sysand-installed project keeps working unchanged; `syspi` reads an existing `.sysand/lib/` too and
  never writes there.
- Lock: no OMG format exists. `.project.lock.json` beside `.project.json` (one entry per resolved
  usage: canonical resource, version, source URL, tag, `vcs`, `revision`, `.kpar`/tree SHA-256 —
  the tree digest is one deterministic hash over sorted relative paths and contents, so the same
  release served from git, svn or hg locks to the same `sha256` — plus the transitive closure;
  deterministic bytes so a fresh clone reproduces it byte for byte); read
  `sysand-lock.toml` as an input when present for interop, never write it. OpenSysML does not read
  the lock: it finds what is installed under `lib/`.
- Editors: unresolved root namespace → LSP code action "add usage pkg:git/…" backed by the index
  search and running `syspi add` when the binary is on `PATH`; `%project` in the REPL/kernel shows
  the closure. This is the only place OpenSysML calls the client, and it is optional.
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
| `.project.lock.json` | no | no (SysPI's reproducibility record; OpenSysML never reads it) |

## 4. Phasing

Phase 1 — installable packages from git remotes, no index
1. Extensions Library: add `.project.json` (`publisher` Open-MBEE), tag `v1.0.0`; release job attaches
   the `.kpar` built by the existing `scripts/build-kpar.py` (the docs already promise it).
2. OpenSysML (read side only): `lib/` in the well-known install dirs; `@<version>` in a usage IRI
   as an exact constraint when the usage carries no `versionConstraint`.
3. SysPI: `syspi init`, `add`, `install`/`sync`, `lock`, `bundle`, `index emit` for direct sources
   (`pkg:git/...`, `pkg:svn/...`, `pkg:hg/...`, clone URL, remote `.kpar`, local path/editable)
   with `.project.lock.json`;
   tag == version check; "no .project.json" error; caret/tilde/range constraints.
Gate: `syspi` installs the Extensions Library, ode4hera Requirements Management and MBSE4U SYSMOD
from their git repos (SYSMOD's repo already carries `.project.json`/`.meta.json` at root) into a
fresh project, and `sysml -validate` on that project resolves every usage from `lib/` with no
OpenSysML flags — plus what `tests/kpar` validates today.

Phase 2 — the index
4. `opensysml-index` repo: `Index.sysml` definitions, records for our libraries, CI (`sysml -validate`,
   closure check, generated `versions.json`/`index.json`, Pages).
5. `syspi index check|add|yank`; index-backed resolution and search; `syspi publish` opening the
   index PR.
6. LSP "add usage" code action; `%project` in REPL/kernel.

Phase 3 — ecosystem
7. Mirror records for the public sysand.com packages (read-only, pointing at their sources) so one
   index answers both; measure parser/validator fallout over every package.
8. Attestation (cosign keyless from package CI) verified on install.
9. Flexo-served index via the SysML v2 API sync.

## 5. Decisions

Settled: `pkg:<vcs>/<host>/<path>@<version>` identity with `git`, `svn` and `hg` all in Phase 1 (the
VCS is the protocol, no provider list; `cvs` refused); no extension library (index definitions live in
the index repo, stdlib-only); no lax mode.

Settled: install dir `lib/` beside `.project.json` (`.sysand/lib` still read); lock `.project.lock.json`;
fetch = the user's `git`/`svn`/`hg` executable behind one source interface, with a host archive over
HTTPS as an internal fast path; lock entries carry `vcs` + `revision` + a VCS-independent tree digest.

Settled: the client is a separate project, SysPI (`Open-MBEE/SysPI`, binary `syspi`, Go, one static
binary), not a mode of `sysml`. OpenSysML stays the read side; the two share the on-disk contract
(`lib/`, `@<version>`) and the OMG fixtures, no code. An earlier draft of this section put the
client in `sysml` under `-project-init`/`-add`/`-install`/`-lock`; that is withdrawn.

Open:
1. Index repo name/location, and PR-gated only vs also CI-published from package releases.
2. Whether Phase 3 mirrors sysand.com records at all.
