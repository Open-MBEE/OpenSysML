# Persistent content-addressed parse cache

## The capability that was missing

The gRPC service could reuse a parsed model inside one process, but a new process always paid
for the whole parse again. A downstream consumer that opens the same documents in several
processes — an editor spawning a helper, a build that restarts between stages, a service that
shares one model across a pool — had no way to ask a cold process to answer unchanged bytes
without re-parsing them. The analysis it needed was already persisted; the trees and the symbol
index it analyses over were not.

This record describes the disk-backed parse cache that closes that gap, and the warm/restore
behaviour of the parsing RPCs that exposes it.

## State before the change

At baseline `4d67c7a98f` (released as v0.12.1) the relevant code was:

- `internal/frontend/grpc/lineage.go` kept the four most recently used document sets in an
  incremental workspace. From the second parse of a set it re-parsed only changed documents and
  re-analysed only those and the documents that read them. That reuse was per process: a new
  service had an empty lineage.
- `internal/workspace/libs/cache.go` held derived library facts on disk under `$XDG_CACHE_HOME`,
  keyed by content and build. `internal/workspace/libs/modelrecord.go` persisted a bounded
  analysis/diagnostics record per model, so a new process could skip validation passes. It did
  not persist the trees the analysis was over.
- `internal/semantic/symbols/snapshot.go` and `internal/syntax/ast/astcodec` serialised the
  bundled standard library into an embedded snapshot at build time. That path applied to the
  one frozen, base-less library index; it had no facility for the service's per-request overlay
  indices.
- Upstream issue [#2344](https://github.com/Open-MBEE/OpenSysML/issues/2344) recorded that the
  parse was still cold in every process. Merged PR
  [#2395](https://github.com/Open-MBEE/OpenSysML/pull/2395) persisted a bounded analysis record
  and explicitly did not persist parse trees.

The parsing RPCs had no request field or capability for warming or restoring a content hash.
`ParseSourcesRequest` carried `documents`, `strict_conformance` and `base_model_hash`; the
response carried `model_hash`, roots and diagnostics. The hash was already deterministic over
the documents' names, languages, contents, warnings and conformance mode, so it could serve as a
base-model handle; nothing on disk let a cold process use it to avoid a parse.

## What the change implements

The service now keeps two kinds of record in the same user cache directory:

- one **parse record** per document, keyed by the document's name, language, kind, content and
  build. It holds that document's syntax tree as an `astcodec` stream, the root's node id, the
  parse errors and warnings, and the file-reading warnings. Parse records live in the `parses/`
  subdirectory and are evicted least-recently-used under a 64 MiB bound.
- the existing **analysis record** per document set, which holds the diagnostics the validation
  passes reported.

On a parse request the service first checks its in-process cache, then asks the record cache for
the exact bytes. If every document has a parse record, the whole model is restored: trees are
decoded, the symbol index is rebuilt from those trees by the same overlay path a fresh parse
uses, and the diagnostics come from the analysis record. No document is parsed, and no pass
runs again: the record also holds what each analysis read of the index, so the incremental
workspace can prime its stamps from the recorded answers and a later edit still reports only
the documents it changed. A set with a syntax error is answered from its parse diagnostics
alone: it primes no workspace and carries no stamps, so a later edit reports every document
as affected.
If one document's bytes changed, only its record is missing: the other trees are decoded and just
the edited document is parsed. An edit through an active workspace takes the same record-aware
path, so a fresh process restores its tree too. A damaged or missing record is always a benign
miss — the service parses and records again.

The record cache is consulted after the in-process lineage so a cold set restored from disk still
primes the incremental workspace used by the next edit. Building that workspace installs the
trees the parse cache already holds, so it does not re-parse the unchanged documents.

## Design decisions and alternatives rejected

- **The tree is the source of truth; the index is derived.** Restoring rebuilds the symbol index
  from decoded trees rather than serialising it. `symbols.WriteSnapshot` requires a frozen index
  with no base, while each service model is an overlay over the shared library index; writing a
  second, parallel overlay serialiser would duplicate the index format and could drift from the
  builder. Rebuilding from trees is what the layering already guarantees, and it does not parse.
- **Per-document keys, not only a whole-set key.** A key over the whole document set would make a
  one-byte edit re-parse every document. Keying each document by its own content lets the
  unchanged ones be decoded and satisfies the single-document-reparse requirement.
- **No new RPC or wire field here.** `ParseSources` and `ParseFile` already accept the bytes,
  return a content-derived model hash and accept `base_model_hash`; making those calls the
  warm/restore surface keeps every client and capability list unchanged. A dedicated "warm this
  hash" RPC was rejected as wire surface with no extra expressive power: the service decides from
  the bytes it already has. A separate surface adds `ParseSourcesRequest.keep` for a different
  purpose: a transient parse neither promotes nor inserts into the in-memory parsed-model LRU.
  It still reads a kept entry for the same bytes, so an undo back to a cached state compares
  against that answer's own analysis stamps. That field does not control restore. With a record
  cache configured, a `keep=false` parse can still restore trees from disk and write parse and
  analysis records, because record I/O is independent of the in-memory LRU and leaves its
  eviction contract intact. A restore still runs validation passes over the restored trees, not
  parses, to prime incremental `affected`, but only when every document parsed clean: a set with
  a syntax error publishes no semantic diagnostics, so it carries no stamps for a later edit.
- **Parse records in their own directory.** Analysis-record readers list the cache root for
  `*.idx`; mixing the two kinds would make them count parse records as analyses and would let a
  partial parse look like a recorded report. A `parses/` subdirectory keeps the kinds and their
  independent bounds separate.
- **Restore after lineage, not instead of it.** A restored model alone has no per-document
  analysis stamps, so a later edit could only report every document as affected. Letting the
  lineage see the set first keeps its incremental contract, and installing restored trees into
  the workspace keeps that first look parse-free.
- **Build identity in every key.** Both record kinds include the build id, so a new binary never
  decodes a tree or diagnostics produced by code that may lay them out differently. The parse
  format version is part of the suffix and is bumped whenever the record or the AST stream
  changes shape.
- **Bounds chosen from measured trees.** Across the bundled examples the densest encoded tree
  used 14.6 bytes per node and the largest used 3,905 nodes; a bounds probe recorded those
  figures over every example. The decoder therefore admits one node per 8 stream bytes (with a
  1,024-node floor for short trees) and caps the absolute table at 262,144 nodes, about 67 times
  the largest real tree. Both bounds together keep a hostile record from reserving gigabytes
  while every measured model restores; a record over 64 MiB is refused before it is read. A probe
  with an 8 MiB stream declaring 76 million nodes allocated about 613 bytes per refused decode.
- **Opt-in and bounded.** A service without a record cache still parses everything; the cached
  path answers identically. Neither record kind is unbounded, and nothing is written for a
  request whose context has already been cancelled before its analysis.

## Public surface

No RPC, protobuf message, capability flag or client method was added. The observable surface is:

- `ParseSources` and `ParseFile` are the warm/restore calls. Their request and response messages
  are unchanged. The `model_hash` they return is usable as `base_model_hash` by a later process.
- `WithRecordCache` on the service constructor now also enables tree restore and recording,
  in addition to the analysis record it already enabled. The default binary
  (`cmd/sysml-grpc`) already configures it; `-no-record-cache` disables it.
- Two internal helpers were added so the workspace can accept a tree parsed elsewhere: the
  unexported `workspace/model.newParsedDocument` builds the document and its scope tree from a
  tree the caller already has, and the exported `Workspace.OpenParsed` installs it as an open
  buffer without parsing. `Workspace.RestoreAnalyzed` installs recorded diagnostics and reads
  without analyzing, and `Resolver.RestoreReads` gives the restored answer a frame whose
  invalidation on a later edit is the one the original passes recorded.
- `docs/reference/wire-contract.md`, "Restoring a model another process parsed", documents the
  client-visible behaviour.
- On-disk format: `<user cache>/sysml-ls/libs/parses/<hex sha256>-b<build id>-p1.idx`, a gob
  record of name, language, kind, the `astcodec` tree stream and root id, parse errors, parse
  warnings, file warnings and a SHA-256 digest of all of those. `parseFormatVersion` is `1`; a
  change to the record or the AST stream bumps the `-p` suffix. Analysis records are `-m2` at
  the cache root: each document carries its diagnostics and the reads its analysis made, so a
  restored process primes `affected` without running the passes. A record whose digest fails
  is deleted and answered by a fresh parse.

## Acceptance criteria and the tests that lock them

| Criterion | Test |
|---|---|
| Process A parses a two-document set, edits it through its active workspace; process B asks for the final bytes and makes zero parse calls | `internal/frontend/grpc/parsecache_test.go:TestPersistentParseCacheRestoresAcrossProcessesWithNoParses` — two service instances share only the cache directory; the second counts every call to the parser seam and asserts none, and compares the answer with a fresh parse |
| A restored set with a syntax error hides no sibling diagnostics | `parsecache_test.go:TestPersistentParseCacheRestoredSyntaxErrorDoesNotHideSiblingDiagnostics` — after the syntax error is fixed, the sibling whose unresolved type was never published is reported as affected with its diagnostic |
| A transient undo back to a cached state keeps its stamps and cache order | `internal/frontend/grpc/keep_parity_test.go:TestParseModelKeepingTransientUndoParity` — the lineage advances between the base parse and a same-content request, which reuses the kept answer without promoting or inserting an entry |
| A one-byte edit re-parses exactly one document | `TestPersistentParseCacheReparsesOnlyTheEditedDocument` — changes one byte of one document and asserts the counted calls are exactly that document |
| A whole-set restore answers identically to a fresh parse | the same tests and `requireSameAnswers`, which compare the parse response, the diagnostics and the api-json conversion |
| A real mid-size model restores without a parse | `parsecache_test.go:TestPersistentParseCacheRestoresARealModel` — restores `examples/self-model/pipeline.sysml` through the counting seam; it fails with the old density bound |
| Document order and re-reading do not change the answer | `TestPersistentParseCacheRestoresDocumentsInAnyOrder`, `TestPersistentParseCacheRestoresParseFile` |
| Damaged, missing or implausible records are benign misses and are re-recorded | `TestPersistentParseCacheRecoversFromADamagedRecord`, `TestPersistentParseCacheRefusesAnImplausibleTree`, `TestPersistentParseCacheBitFlipIsABenignMiss`, and `libs/parserecord_test.go:TestParseRecordMissesAreBenign`, `TestParseRecordBitFlipsAreTypedMisses` — every single-bit flip of a record is checked; a changed payload is a typed miss, never a wrong answer or invalid UTF-8 |
| Restoring a large real-model set does not re-analyze it | `BenchmarkPersistentParseCacheRestoreRealModel` — 60 documents built from `examples/self-model/pipeline.sysml`; the restored sub-benchmark reports about 0.6 s against about 10 s cold |
| The cache is keyed by content, name, language, kind, build and format | `libs/parserecord_test.go:TestParseKeyIsContentAddressed`, `TestParseAndModelRecordsCoexist` |
| A service without a record cache is unaffected | `TestPersistentParseCacheIsOptIn` |
| Incremental `affected` semantics survive a warm record cache | `client/opensysml/parity_options_test.go:TestParseDocumentsRelatesAModelToItsBase` — against a shared cache, only the edited document is reported as affected |
| Analysis records are still not half-written or cross-served | the pre-existing `modelrecord_test.go` and `cancellation_test.go` gates, which still pass |
| Cold and restored cost is measurable | `BenchmarkPersistentParseCache` reports both paths over the same two documents |

## Verification

Run in the isolated development worktree on this branch:

- `gofmt -l .` — no output.
- `go build ./...` — passed.
- `go vet ./...` — passed.
- `GOMAXPROCS=2 go test -p 2 -count=1 -timeout 20m` over `internal/frontend/grpc`,
  `internal/workspace/libs`, `internal/workspace/model`, `internal/syntax/ast/astcodec` and
  `client/opensysml` — all five passed in full (186 s, 23 s, 72 s, 0.01 s, 99 s), including
  the cross-process restore, one-byte edit, hostile record, crashed temp file, affected-base,
  cancellation and client parity cases.
- A targeted run of `TestCancelledParseRecordsNoAnalysis`, `TestParseSourcesStopsAtEveryCheck`,
  `TestPersistentParseCacheKeepsAffectedAcrossProcesses` and
  `TestPersistentParseCacheRestoresAcrossProcessesWithNoParses` after the final hardening —
  passed.
- `GOMAXPROCS=2 go test -p 2 -count=1 -bench BenchmarkPersistentParseCacheRestoreRealModel
  -benchtime=1x ./internal/frontend/grpc`: cold 10.47 s, restored 0.57 s over the 60-document
  set. Before recording the analysis reads, the same restored path took 7.6–7.9 s because
  every document's passes ran again to prime `affected`.
- `GOMAXPROCS=2 go test -p 2 -count=1 ./tests/hygiene` — passed (20 s).
- `python3 scripts/check-doc-ids.py`, `python3 scripts/changelog.py check`, and
  `python3 scripts/spec_compliance.py check` — passed. The comment gate printed nothing and
  `gofmt -l .` was empty.
- An earlier clean `GOMAXPROCS=2 go test -p 1 -count=1 -timeout 30m ./...` on the pre-hardening
  head passed 137 packages; four failed for reasons independent of the change: `internal/ir/view`
  (the installed `dot`/`neato` warns on non-ASCII label widths), `internal/translate/export`
  (`examples/sysml-v2-training` is absent), `tests/corpus` (downloaded corpora are absent), and
  `tests/hygiene` (its citation index reads tracked files, and the new Go files were untracked
  then). The whole-tree suite was **not re-run to completion after the hardening fixes**: the
  post-fix bounded run was stopped at 70 packages with zero failures so the branch could be
  finalized. The targeted packages covering every changed function did pass in full after the
  fixes; the reviewer's own gates are the remaining whole-tree coverage.
- `make docs-check` — the link checker reports one pre-existing broken link in
  `docs/project/third-party-notices.md` to a landing asset generated by `make
  docs-landing-assets`; no link in this page or the wire-contract page is reported.

Optional corpora gated by `OPENSYSML_REQUIRE_*` variables were not downloaded; those tests
skip or fail as absent-input gates in a plain run. No converter toolchain was exercised by this
change. Verification ran under a two-thread CPU cap with Go's temp directories on the home
filesystem; no heavy verification commands ran concurrently.

## Known limitations and follow-ups

- The cache is local to the machine and directory configured by `$XDG_CACHE_HOME`; it is not
  shared between hosts or containers.
- The symbol index is rebuilt from restored trees. That is not a parse and costs much less than
  one, but it is not free; a future change could serialise the overlay index if profiling shows
  rebuilding dominates the restored path.
- A record is never served across a build change: its key includes the build id. A rolling
  deployment therefore re-parses once per version.
- Parse records are bounded by size, not by age. The analysis records at the cache root keep
  their existing idle-age pruning; parse records are evicted only when the 64 MiB bound is
  passed.
- No new capability advertises the cache to clients. Callers discover its effect through the
  unchanged `ParseSources` contract; adding a negotiation flag would be a separate wire change.
- The wall-time budget named by the downstream consumer is a property of its own cold-tree
  route. The upstream gate is the deterministic zero-parse assertion plus the benchmark above,
  not a timing threshold in a test.

## Evidence

- Upstream issue [#2344](https://github.com/Open-MBEE/OpenSysML/issues/2344) — cold parse in
  every process.
- Merged PR [#2395](https://github.com/Open-MBEE/OpenSysML/pull/2395) — bounded analysis record
  cache, explicitly without parse trees.
