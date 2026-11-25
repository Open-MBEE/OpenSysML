# api-json projection for `Convert`

## The missing capability and why a downstream consumer needs it

The api-json element form is the representation a consumer reads when it needs the *meaning* of a
SysML v2 model — declarations, their metaclasses, their members and the annotations carrying
facts about them — rather than its notation. A whole-corpus conversion writes every element the
model, its imports and the standard library make reachable. On a facet gate that has to judge only
the declarations carrying a particular annotation, that means reading, transferring and parsing a
reply far larger than the part it will use: the recorded whole export on the gate's corpus is 54
files → 108,309 elements → 185 MB → 127 s, most of it in standard-library files the gate never
judges. A caller that knows which declarations and annotations it will read has no way to say so:
it must take the whole array and filter locally.

What was missing is a **projection on the conversion request**: a way to name the declarations and
annotations the caller will read, by qualified name, by metaclass, or by annotation type, and get
back only those elements — with every element that *is* written byte-identical to the whole-corpus
answer, so the consumer's judgements cannot change with the narrowing. It must also avoid
materializing the full element array on the server, or the memory cost that motivated reading less
is paid anyway.

## Exact state at the baseline

At baseline `4d67c7a98f` (`nightly-20261118-4d67c7a98f`, `v0.12.1-2379-g4d67c7a98f`, and the
`v0.12.1` release before it):

- `ConvertRequest` offered a source `oneof`, `from_format`, `to_format`,
  `tolerate_syntax_errors`, `id_form`, `documents`, `compact`, `omit_derived`, `keep_derived`,
  `include_check_diagnostics`, `include_library_stubs` and `inherited_members`. There was no
  element-level selection: `documents` selected whole documents, not declarations, and there was
  no way to name an annotation type.
- `Convert` answered one `content` string. The api-json writer built the whole element array in
  memory (`apiJSONElements`), so the 185 MB reply had to exist as objects before it was written.
- Upstream had just landed a streamed writer and the document filter: issue
  [#2239](https://github.com/Open-MBEE/OpenSysML/issues/2239), closed by PR
  [#2318](https://github.com/Open-MBEE/OpenSysML/pull/2318) (2026-11-16). That work writes one
  element at a time, byte-identically, and adds `ConvertRequest.documents`, plus compact and
  omit-derived options. It did **not** add declaration or annotation projection, and it did not
  add a streaming RPC. Follow-ups #2322, #2353 and #2331 touched the same surface.
- The service advertised `convert`, `convert_documents`, `convert_compact`,
  `convert_library_references`, `convert_library_stubs`, `convert_inherited_members` and
  `convert_check_diagnostics`; there was no capability for element selection.

A later check of the repository (200 first-parent merges past the baseline) confirmed the gap was
still open: the streamed writer and the document filter had landed, but neither a
declaration/annotation projection nor a streaming RPC existed.

## What this change implements

`ConvertRequest.projection` carries a `ConvertProjection` with three `repeated string` selectors:

- `qualified_names` — elements whose written `qualifiedName` is named;
- `metaclasses` — elements whose written `@type` is named;
- `annotation_types` — annotations whose annotating element's metaclass or metadata-definition
  name is named, together with the annotation element, the annotating element and the declaration
  it annotates.

The selectors are unioned. An empty selector contributes nothing; all three empty write the whole
model, exactly as a request with no projection does. A metadata usage typed by `@Facet` is named
by `Facet` or by the definition's qualified name, and a comment, documentation or textual
representation is named by its metaclass. A name no element of the model carries is an
`INVALID_ARGUMENT`, so a misspelled selector cannot quietly return an empty array. A metaclass the
metamodel declares is known even when this model holds no element of it; that is a valid, empty
projection.

The projection index is built once over the settled graph the writer reads, using the writer's own
metaclass normalization and metamodel-derived `qualifiedName`. It indexes the `@type` and
`qualifiedName` the reply actually carries, never the raw `rdf:type` or a stale stated name, then
resolves the selectors into the set of subjects to write. The writer's own indexes, collection
reconciliation and encoding still run over the whole graph; only the loop that visits subjects is
narrowed. This is what makes every emitted element byte-identical to an unprojected conversion —
including its `@id`, its derived properties and every reference — while elements left out become
bare `@id` references. The standard array remains streamed one element at a time; the projection
index holds names and term sets, never element objects. The compact document is supported by the
same path.

With a `documents` filter the written graph holds bare references to declarations of the other
documents: a metadata definition that types a written annotation has no name triples there.
Selectors resolve against the full model's settled graph instead, and the selected subjects are
intersected with the document-filtered graph when the writer visits them. A selector therefore
still names a definition in an excluded document, while the reply writes only elements of the
documents the caller asked for.

Projection composes with `documents`, `idForm`, `compact`, `includeLibraryStubs` and
`inheritedMembers`. It applies to `api-json` only; any other target is `INVALID_ARGUMENT`.

## Design decisions and alternatives rejected

**Projection fields on `ConvertRequest` rather than a new RPC.** A new streaming RPC would require
a second service method, a second response type and a second implementation in every transport and
client for a benefit the existing writer already provides: the element array is now streamed, so
the large reply is never held whole on the server. Adding selectors to the request keeps one
conversion path, one capability check and one response shape, and lets projection combine with the
document, compact and id-form options that already live on the same request. No streaming RPC was
added.

**Filtering subjects in the writer rather than rebuilding a smaller graph.** Removing triples for
unselected elements from the graph before writing would change the metamodel evaluation behind
derived properties, collection ordering and reference resolution, so emitted elements would no
longer be byte-identical. Filtering at the visit loop keeps every shared index intact and makes
byte-identity a structural property of the writer, not a convention. It also needs only one pass of
lightweight indexing over the subjects.

**Matching names from the settled graph rather than re-deriving them.** The writer computes some
properties (notably `qualifiedName`) through the metamodel evaluator. Matching against the same
settled graph the writer encodes avoids a second, possibly divergent derivation: the name a
selector matches is the name the emitted element carries.

**Three selectors unioned, not intersected.** A caller reading facet-carrying declarations may
name the annotation type and also a few declarations by name; intersection would force it to
enumerate every selector for every element. Union makes each selector an independent way to add
elements. Unknown-name refusals are per selector, so a bad name still fails loudly.

**The annotation element and its ends are written with the declaration.** A facet's value lives in
the annotating element and its binding in the `Annotation` relationship. Emitting only the
declaration would give the consumer a bare `@id` it cannot read; emitting the whole annotation
subtree keeps the reply self-contained while still shrinking it.

**Rejected: a generic JSON-path/field projection.** It would couple the wire contract to the
encoder's key names and to derivation order, and could emit partial element objects whose shape is
not the element form's. The three named selectors cover the declarations-and-annotations use case
without exposing writer internals.

## Public surface

### RPC and message

- `ConvertRequest.projection = 15`, of type `ConvertProjection`.
- `ConvertProjection.qualified_names = 1`, `metaclasses = 2`, `annotation_types = 3`.
- Capability `convert_projection`; a service without it answers `UNIMPLEMENTED`, and a request
  that uses projection against any other target answers `INVALID_ARGUMENT`.
- No new RPC and no change to `ConvertResponse`: the reply's `content`, `from_format`,
  `to_format`, diagnostics and library-reference table keep their names and meaning.

No file format or version number changes. The standard element array keeps its shape; the compact
document stays `api-json-compact/1`. Projection narrows `elements`/`elementCount` and the id table
only. Generated stubs were regenerated for Go (`api/proto/sysml.pb.go`), Python, TypeScript
(`client/node/src/generated`), Rust (`client/rust/opensysml/src/proto`) and the Rust conformance
descriptor; the Java client generates its messages at Maven build time.

### Client methods and options

| Surface | Added |
|---|---|
| Go (`client/opensysml`) | `Projection` type and `WithProjection(Projection)` option on `Convert`, `ConvertFile`, `ConvertSource`; `CapabilityConvertProjection` |
| Python (`client/python`) | `projection=Projection(...)` on `Connection.convert`, `Model.convert` and module-level `convert`; `Projection` and `CAPABILITY_CONVERT_PROJECTION` exported |
| Node (`client/node`) | `Projection` type and `projection` field on `ConvertOptions`; `CAPABILITY_CONVERT_PROJECTION` |
| Java (`client/java`) | `Projection` record and `ConversionOptions.withProjection(Projection)`; `Capabilities.CONVERT_PROJECTION` |
| Rust (`client/rust`) | `Projection` struct and `ConvertOptions.projection`; `CAPABILITY_CONVERT_PROJECTION` |
| Julia (`client/julia`) | `Projection` and `projection=Projection(...)` on the convert functions; `CAPABILITY_CONVERT_PROJECTION` |
| MATLAB (`client/matlab`) | `projection` name-value option on `convert` and `Model.convert`; `convertProjection` capability |
| MCP (`internal/frontend/mcp`) | `projection` object on the `convert` tool's input, mapped to the service capability |
| gRPC / Connect | the `ConvertRequest.projection` field, carried by the regenerated stubs; `ConnectAdapter.Convert` delegates to the same service method |
| WASM / engine / core / syntax | no `Convert` RPC is exposed on these surfaces; the regenerated TypeScript stubs carry the field for browser clients that use the wire type |

Every client gates the option on the advertised capability and copies the selector slices into the
wire message, so a caller changing its `Projection` afterwards cannot change the request.

## Acceptance criteria and the tests that lock them

| Criterion | Test |
|---|---|
| A request for only facet-carrying declarations returns a correspondingly smaller reply | `internal/frontend/grpc/convert_projection_test.go:TestConvertProjectionByAnnotationType` and `:TestConvertProjectionByQualifiedName` compare the reply with the whole corpus; `internal/translate/export/api_json_projection_test.go:TestAPIJSONProjectionByAnnotationType` asserts fewer elements than the whole |
| Emitted elements are byte-identical to the whole-corpus run | `assertProjectedElementsMatchWhole` in `convert_projection_test.go` and `assertProjectedAreByteIdentical` in `api_json_projection_test.go` compare each emitted element's raw JSON bytes against the unprojected element with the same `@id`; both run for all three selectors and for a several-document model |
| Projection by qualified name | `TestConvertProjectionByQualifiedName`, `TestAPIJSONProjectionByQualifiedName`, `:TestConvertProjectionOfSeveralDocuments`, plus `TestAPIJSONProjectionMatchesDerivedQualifiedName` and `TestConvertProjectionMatchesDerivedQualifiedNameFromAPIJSON` for a name the writer derives rather than the graph stating |
| Projection by metaclass | `TestConvertProjectionByMetaclass`, `TestAPIJSONProjectionByMetaclass`, `TestConvertProjectionWithDocumentsAndCompact`, plus `TestAPIJSONProjectionMatchesWrittenMetaclass` and `TestConvertProjectionMatchesWrittenMetaclassFromTTL` for a legacy type name normalized on the wire |
| Projection by annotation type | `TestConvertProjectionByAnnotationType`, `TestAPIJSONProjectionByAnnotationType`, `TestAPIJSONProjectionByAnnotationMetaclass`, `TestAPIJSONProjectionByAnnotationDeclaredShortName` and `TestAPIJSONProjectionByAnnotationQuotedName` for a declared short name and a quoted name with an embedded `::` |
| Annotation type with a document filter | `TestConvertProjectionAnnotationTypeAcrossDocuments` selects by a definition in an excluded document and asserts that only elements of the requested document are written |
| Refusal of unknown names | `TestConvertProjectionUnknownNamesAreInvalidArgument` (one case per selector, asserting `INVALID_ARGUMENT`) and `TestAPIJSONProjectionUnknownNames` (asserting the typed writer error and the selector it names) |
| The full element array is not materialized | The standard writer is `export.WriteAPIJSONTo`, which visits one element at a time; `TestAPIJSONProjectionEmptyWritesWhole` and `TestConvertProjectionEmptyIsTheWholeCorpus` pin that an empty projection takes the unchanged streaming path, and `TestConvertProjectionWithDocumentsAndCompact` pins that narrowing reaches the compact form |
| All surfaces and clients are updated | `client/opensysml/parity_options_internal_test.go` (`TestConvertOptionsAreNotRequestedWithoutTheirCapability`, `TestConvertOptionsReachTheWire`), `client/python/tests/test_conversion.py` (`test_convert_sends_the_projection_asked_for`, `test_projection_requires_its_capability`), `client/node/test/convert-options.test.ts` (the two projection tests), `client/rust/opensysml/src/conversion.rs` (`requests_carry_the_projection_asked_for`), `internal/frontend/mcp/runtimetools/conversion_test.go:TestConvertPassesAProjection` and `info.go` capability mapping, `tests/hygiene/client_capability_exports_test.go:TestClientCapabilityExports` (Node and Julia public capability exports) |
| Capability is advertised and gated | `TestConvertProjectionCapabilityIsAdvertised`, `TestConvertProjectionRefusals` (withheld capability and non-api-json target) |
| Changelog and reference docs | `changes/unreleased/api-json-convert-projection.added.md`, `docs/reference/wire-contract.md` § Conversion, `docs/reference/client-parity.md` § Capabilities, `docs/reference/service-transports.md`, `docs/project/spec-compliance/08-grpc-service-layer.md` |

## Verification

Exact results from the branch head in the isolated worktree:

- `gofmt -l .` — empty output.
- `go build ./...` — passed (no output).
- `go vet ./...` — passed (no output).
- `go test ./internal/translate/export -run TestAPIJSONProjection -count=1` — `ok`, 2.942 s (11 tests, including the two writer regressions for the written `@type` and derived `qualifiedName`).
- `go test ./internal/frontend/grpc -run TestConvertProjection -count=1` — `ok`, 8.532 s (11 tests, including the TTL and api-json source regressions).
- `go test ./internal/frontend/mcp/runtimetools -run TestConvertPassesAProjection -count=1` — `ok`, 9.162 s.
- `go test ./tests/hygiene -run 'TestClientCapabilityExports|TestClientCapabilityChecklist|TestComplianceTablesNameLiveGoIdentifiers|TestCIInstallsTheGrpcioToolsThatGeneratedThePythonStubs' -count=1` — `ok`, 10.481 s.
- `go test -p 1 -count=1 ./tests/hygiene` — `ok`, 32.987 s.
- `python3 -m pytest client/python/tests/test_conversion.py -k 'projection or compact_options' -q` — 7 passed, 52 deselected.
- `npm run build && npm run build:tools && node --test build/test/convert-options.test.js` — 8 tests, 8 pass, 0 fail.
- `mvn -pl opensysml-client -am compile` in `client/java` with a JDK 21 `JAVA_HOME` — passed.
- `python3 scripts/changelog.py check` and `python3 scripts/check-doc-ids.py` — pass.
- Comment gates (`//`, `#`, `%`) over the merge-base diff — zero added comment lines beyond two lines.
- `make proto-breaking` — no breaking schema changes; `make docs-counts` — current (the pinned
  validator jar and transformation model were absent, so their measurement comparisons skipped).
- `python3 scripts/spec_compliance.py check` — 57 sources well-formed.
- `go run -C tools ./cmd/pilot-apijson-diff -check -roots testdata` — reproduces the committed
  fragments (42 files, 1967 matched elements); the all-roots run was killed by the host's memory
  limit here, so the full differential remains unverified in this environment.
- `go test -C tools -count=1 ./referee/apijson` — `ok`, 11.167 s.
- `go test ./... -count=1` — 123 packages passed and 18 failed in this environment. Thirteen
  failures were 10-minute per-package timeouts while several unrelated builds were running at a
  load average above 85 (`cmd/sysml`, `internal/check/passes`, `internal/exec/runtime`,
  `internal/exec/smt`, `internal/frontend/grpc`, `internal/semantic/semantics`,
  `internal/workspace/model`, `tests/codegen`, `tests/export`, `tests/grpc`, `tests/kpar`,
  `tests/migrate`, `tests/wasm`); these packages' projections-relevant tests are covered by the
  targeted runs above. `internal/translate/export` failed one pre-existing test that opens the
  absent training corpus (`examples/sysml-v2-training/32. Requirements/Requirement Groups.sysml`),
  `tests/corpus` reported its absent optional corpora, `internal/ir/view` failed Graphviz DOT tests
  against the installed `dot`, and `internal/exec/solve` failed a differential test. `tests/hygiene`
  had failed a generator-version pin because the local `grpcio-tools` is newer than the pinned
  version, plus a missing capability row in `docs/reference/client-parity.md` and file paths in the
  compliance citation that did not match tracked files; the stub's version header was restored to
  the pinned value, the capability row and the citation paths were corrected, and
  `go test ./tests/hygiene -run 'TestCIInstallsTheGrpcioToolsThatGeneratedThePythonStubs|TestClientCapabilityChecklist|TestComplianceTablesNameLiveGoIdentifiers'`
  passes (`ok`, 19.324 s). No require variable was set for this run, so absent corpora did not
  become a gate.

The Rust crate, Julia package and MATLAB toolbox could not be built here: no `cargo`, `julia` or
MATLAB/Octave is installed. Their changes are option/field wiring to the same regenerated field,
and the Rust request-shape test is committed for a machine with the toolchain.

## Known limitations and follow-ups

- Projection narrows the **written** set only. References from written elements to elements left
  out remain `@id` links, as library elements already do; a caller that wants the referenced
  elements must ask for them explicitly. This is deliberate — it is what keeps emitted elements
  byte-identical.
- There is no streaming RPC. The reply is still one `content` string. The server writes the
  standard api-json array one element at a time, so the 185 MB object array is no longer
  materialized, but the bytes of the reply are still held for the response. A chunked RPC would be
  a separate change with its own transport, client and conformance work.
- Annotation-type selection names an annotating element's metaclass or the qualified name (or
  short name) of the metadata definition that types it. It does not match an annotation by the
  *value* of one of its properties; that would need a value predicate the element form does not
  expose.
- A short metadata-definition name selects every definition with that short name in the model. Use
  the qualified name where two packages define annotations with the same short name.
- The compact form's id table is built from the projected elements and the references they carry,
  so its handle indices differ from a whole-corpus compact document. The element *shape* and the
  decoded values are unchanged; raw compact bytes are not expected to match an unprojected compact
  reply, only the standard elements are byte-identical.
- No CLI flag was added. The command line keeps converting whole documents; consumers that need
  projection use a client library or the RPC. A CLI spelling can follow without changing the wire
  contract.
- No new OMG corpus measurement was taken: the recorded 185 MB / 127 s figure predates this change
  and describes the whole-corpus export, not a projected one. Re-measure the facet gate against a
  projected request before treating the figure as superseded.
