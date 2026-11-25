# Non-keeping source parses

This record describes the transient-parse surface end to end: the gap it fills, the state of the
code before the change, the decisions behind it, the public names it adds, and the tests and
gates that make those names contracts. It is written for a reader who has only this tree.

## Why the capability is missing

`ParseSources` answers a model and inserts it into the service's parsed-model cache. The cache is
a least-recently-used set: `Cache.Get` promotes the model it returns and `Cache.Add` evicts the
least recently used entry at capacity. The production service runs with three entries.

That is the right default for a caller opening a model it will keep querying. It is the wrong
default for a caller that parses a *derived* model only to read an answer from it. A consumer
that computes an evidence verdict over a union of the project's documents, then reads that
verdict again after editing another project, pays for the union's insertion each time. Whether
the caller touches its own entry first only chooses which model becomes the victim: the union
still occupies a slot, and the next read finds the open model displaced and has to repair the
missing entry with another parse.

What the caller needs is a parse that answers a model and drops it: no insertion, and no
promotion of an entry it did not ask to keep.

## Baseline: 4d67c7a98f / v0.12.1

At the pinned baseline and at the v0.12.1 release:

- `api/proto/sysml.proto` declares `ParseSourcesRequest` with exactly three fields: `documents`,
  `strict_conformance` and `base_model_hash`. There is no field describing what the service does
  with the parsed model, and `ParseSourcesResponse` has no flag saying whether it kept one.
- `internal/frontend/grpc/service.go` has
  `parseModelKeeping(ctx, inputs, mode, incremental, keep bool)`, introduced on 2026-10-17, but
  `ParseSources` always calls the `parseModel` wrapper with `keep=true`. The `keep=false` branch
  is used only for internal staging — `ImportData`, `RecordRun` and the export preparation path —
  where a model is built in order to derive something from it and then dropped.
- The service advertises capabilities through `GetServerInfo`; there is no capability for a
  non-keeping parse, so a client cannot ask for one conditionally or tell an older service from
  one that supports it.
- No upstream issue or pull request proposes exposing the option. The related parse-cache work,
  [issue #2344](https://github.com/Open-MBEE/OpenSysML/issues/2344) and its analysis-record
  cache, persists diagnostics across processes and does not change parse insertion.

## What the change implements

`ParseSourcesRequest.keep` is a presence-bearing proto3 optional Boolean. When it is absent, or
present and true, the request is the old parse and the response is byte-identical to before. When
it is false, and the service advertises `parse_sources_keep`, the service builds the same
`model_hash`, roots, diagnostics and affected set, and then drops the model from the in-memory
parsed-model LRU: it neither adds an entry there nor looks up an entry already under that hash.
A caller can therefore read a derived union, use the answer, and leave its own cached models
exactly as they were. Whether the trees come from parsing or from disk parse records is a
separate matter: with a record cache configured, a transient parse may restore trees from disk
and write parse and analysis records, without changing the LRU contract above.

A request that states `keep` to a service without the capability is refused with `UNIMPLEMENTED`,
naming `parse_sources_keep`. A service that predates the field would silently keep the model,
which is the outcome the caller was trying to avoid, so the refusal is deliberate rather than a
fallback. A model of one document is always kept: `ParseFile` has no cache option, and every
client that accepts parse options refuses `keep` on its one-document calls.

## Design decisions and alternatives rejected

- **An optional field, not a plain Boolean.** A plain proto3 `bool keep = 4` reads false when it
  is omitted, which would silently turn every existing caller into a transient one and break the
  default. Presence lets omission mean "as before" and lets a client ask explicitly for either
  behavior.
- **`keep` rather than an inverted flag.** A `transient` flag would have kept the wire default
  but would have named the opposite of the service's existing `keep` parameter. The field names
  the one concept already in `parseModelKeeping`, and clients spell it the same way.
- **No parsed-model LRU read on a transient parse.** Looking up the hash of the model being
  parsed would promote an entry that belongs to another caller. A transient parse computes its
  own hash from its inputs and returns its own model, so it touches neither the LRU's contents
  nor its recency. Disk parse records are not that LRU: with a record cache configured, the
  transient parse may read them and write parse and analysis records. The caller's
  `base_model_hash`, when given, is still read from the LRU to compute `affected`; that is the
  model the caller explicitly named.
- **Lineage and records are unchanged.** The incremental workspace and the on-disk record store
  are separate from the parsed-model LRU and evict no entry from it, so keeping their use leaves
  the internal staging callers' behavior intact. A record cache can restore trees from disk in a
  fresh process, making zero parses for a transient union, while validation passes still run to
  prime the incremental `affected` answer.
- **Capability gated on presence.** An explicit `keep=true` is refused by an older service too,
  even though its behavior would have matched, so that a client never relies on an answer an old
  build only appears to support.
- **No new RPC and no response field.** The answer already carries everything the caller needs;
  only the cache side effect changes. There is nothing to version in a response or in a file
  format, and the change is additive at the wire level.

## Public surface

| Layer | Name |
| --- | --- |
| RPC | `ParseSources(ParseSourcesRequest)`, new field `optional bool keep = 4` (JSON name `keep`) |
| Capability | `parse_sources_keep`, advertised by `GetServerInfo` |
| Go service | `CapabilityParseSourcesKeep`; `Service.ParseSources`; `Service.parseModelKeeping(..., keep bool)` skips the parsed-model LRU lookup and insertion when `keep` is false |
| Go client | `Client.ParseDocuments(..., WithKeep(bool))`; `CapabilityParseSourcesKeep`; `WithKeep` on `ParseFile`/`ParseSource` is an `INVALID_ARGUMENT` |
| Python client | `Connection.parse_sources(..., keep=True)` and the module-level `opensysml.parse_sources(..., keep=True)`; `CAPABILITY_PARSE_SOURCES_KEEP` |
| Node client | `ParseSourcesOptions.keep?: boolean`; `CAPABILITY_PARSE_SOURCES_KEEP` |
| Java client | `ParseOptions.withKeep(boolean)` and `ParseOptions.keep()`; `Capabilities.PARSE_SOURCES_KEEP` |
| Rust client | `SourcesOptions::keep: Option<bool>`; `CAPABILITY_PARSE_SOURCES_KEEP` |
| Julia client | `parse_sources(conn, documents; keep=true)`; `CAPABILITY_PARSE_SOURCES_KEEP` |
| MATLAB client | `parseSources(conn, documents, 'keep', false)`; `capabilities().parseSourcesKeep` |
| MCP `parse` tool | input `keep: false`; `CapabilityTools` maps `parse_sources_keep` to `parse` |

No new file format or format version is introduced. The generated Go, Python, TypeScript and Rust
stubs are regenerated from the schema; the Java classes are generated from the same schema at
Maven build time, and the Rust conformance descriptor is rebuilt. Wire compatibility is checked
by the repository's existing field-number and breaking-change gates.

## How the acceptance criteria are locked

| Criterion | Test |
| --- | --- |
| Two-model fixture, a cache of three, a transient union read twice with a read of the other model in between, and the open model still usable | `internal/frontend/grpc/nonkeeping_test.go` → `TestParseSourcesKeepFalseLeavesTheOpenModelCached` |
| The transient model is not retained, and an inserted parse would have evicted one of the three (so the no-eviction check is not vacuous) | the same test: it counts `srv.cache.items`, checks the union's hash is absent, and then parses the same union with `keep=true` and observes an eviction |
| The transient answer is identical to a kept answer | `nonkeeping_test.go` → `TestParseSourcesKeepFalseAnswersWhatAKeptParseAnswers` |
| A service without the capability refuses a stated field; an ordinary parse is unaffected | `nonkeeping_test.go` → `TestParseSourcesKeepNeedsTheCapability` and `internal/frontend/grpc/capability_test.go` |
| A transient parse stopped at any cancellation point leaves nothing cached | `nonkeeping_test.go` → `TestParseSourcesKeepFalseCancelledParseKeepsNothing` |
| A counting client sends the field only when asked | `client/opensysml/keep_internal_test.go` → `TestWithKeepReachesTheWire` counts the `ParseSourcesRequest`s and the single `ServerInfo` preflight; `TestWithKeepOnOneDocumentParseIsRefused` checks the one-document refusal |
| Every client binds the capability and sends the option | `tests/hygiene/client_capabilities_test.go` against `docs/reference/client-parity.md`; `client/python/tests/test_parse_sources.py`, `client/node/test/sources.test.ts`, Java `PublicTypesTest`, `client/rust/opensysml/tests/compact_affected.rs`, Julia `test/extended_client.jl`, MATLAB `test_errors_capabilities.m` |
| The MCP `parse` tool serves the capability and drops the model it answers | `internal/frontend/mcp/runtimetools/runtimetools_test.go` → `TestEveryCapabilityIsServedByATool`; `parse_fallback_test.go` → `TestParseKeepFalseAnswersWithoutKeepingTheModel` and the `a keep option` refusal case |

## Verification

Results recorded on this branch with the pinned toolchain versions in the repository:

- `gofmt -l .` — no output.
- `go build ./...` — pass.
- `go vet ./...` — pass.
- `make proto-lint` — pass; `make proto-breaking` against `origin/develop` — no breaking schema changes.
- Added-comment gate: the two-line awk over the diff prints nothing for both `*.go` and `*.proto`.
- Targeted: `go test ./internal/frontend/grpc -run 'TestParseSourcesKeep|TestCapability'` — pass;
  `go test ./client/opensysml -run 'TestWithKeep|TestCallsAreNotSentWithoutTheirCapability|TestCapabilityConstantsCoverTheService|TestABaseModel'` — pass;
  `go test ./tests/hygiene -count=1` — pass (295.978s).
- Python: `pytest tests/test_parse_sources.py tests/test_wire_compat.py -q` — 59 passed, 10 skipped
  (the skips are service-backed cases without a running `sysml-grpc`).
- Node: `node --test build/test/sources.test.js` — 16 passed, 0 failed.
- Java (JDK 21): `mvn -DskipITs test` — 415 tests, 0 failures, 0 errors, 1 skipped.
- Rust: `cargo test` with `OPENSYSML_GRPC_BINARY` naming a binary built from this branch — all
  targets pass (163, 8, 27, 4, 4, 1, 1, 1, 1, 6 and 25 tests in the individual binaries;
  0 failures).
- Julia: `julia --project=. test/runtests.jl` with the same branch binary — the new
  `parse_sources sends keep only when asked and requires its capability` test passes 3/3, as does
  the 33-case canned-service suite. One pre-existing live-service case exceeds the client's 30 s
  budget on a `ParseFile` in this environment (53 s on the first run, 155 s on a second run made
  while other builds were competing for CPU); it does not exercise the new option.

- Full serial `go test ./...` (`GOMAXPROCS=2 go test -p 1 -count=1 ./...`) — not green in this
  environment. Four packages failed: `internal/frontend/mcp/runtimetools`, where this change had
  advertised a capability no tool served; `internal/translate/export` and `tests/corpus`, where
  the OMG training, pilot, spec-example and PSSM fixtures are absent (the run prints explicit
  `GATE NOT RUN` banners); and `internal/ir/view`, where the installed Graphviz warns "no value
  for width of non-ASCII character 194" on its dot input. Every other package passed, including
  `internal/frontend/grpc`, `client/opensysml`, `tests/hygiene` and all `cmd/*` packages. The
  missing corpora mean those gates prove nothing about the corpora.
- After fixing the MCP gap: `go test ./internal/frontend/mcp/runtimetools -count=1` — pass, and
  `go test ./tests/hygiene -run 'MCP|Capability|Compliance' -count=1` — pass. No heavy run was
  repeated for the three environmental failures; they do not touch this surface.

## Known limitations and follow-ups

- `keep=false` saves the parsed-model LRU slot, not necessarily the parse. With a record cache
  configured, a transient parse can restore trees from disk records and write parse and analysis
  records, so a fresh process can answer a transient union with zero parses; validation passes
  still run to prime incremental `affected`. Without such a cache the documents are parsed afresh.
- The incremental workspace and the record store are still touched by a transient parse; they
  evict no entry from the parsed-model LRU, so the caller's cached models are safe. A caller that
  needs *no* server-side state at all would need a broader definition of transient.
- The `keep` input on the MCP `parse` tool is new. The separately published agent skills that
  describe this tool live outside this repository and need to repeat the input in the release
  that carries it.
- Explicit `keep=true` requires the capability. A caller that only wants the default should omit
  the field.
- The downloaded OMG corpora and the Graphviz-dependent view tests are not available in every
  environment; a full-tree run without them reports `GATE NOT RUN` banners and Graphviz warnings
  rather than proving those gates.
- No upstream issue or pull request has been filed for the capability; when one is, this record's
  baseline section is the evidence to reference.
