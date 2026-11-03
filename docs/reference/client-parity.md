# Client feature parity

The six remote clients wrap every RPC of `sysml-grpc` with a typed API. This page records,
per client, the request options and cross-cutting features that distinguish a complete
wrapper from a thin one, and links the test that proves each cell. The Go package
[`client/opensysml`](api.md) is the reference wrapper. The Jupyter kernel is a consumer of the
Python client and is not a separate row.

Legend: **typed** — a typed wrapper with a test; **raw** — reachable only through the raw
response (`wire()`, the decoded dictionary); **—** — absent, listed under *Open gaps*.

## RPC coverage

Every client exposes all 31 RPCs (`GetServerInfo`, `ParseFile`, `ParseSources`, `GetSymbol`,
`GetDiagnostics`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `Convert`,
`Migrate`, `ApplyEdits`, `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`,
`ValidateInstance`, `EvaluateCalc`, `RunAnalysis`, `CheckBehavior`, `RunSweep`, `RunRuns`,
`SelfCheck`, `RecordRun`, `ListEngines`, `Query`, `RunDocumentQuery`, `RenderDocument`,
`RenderView`, `ExportGraphs`, `ImportData`, `CompareResults`) as typed calls, and each runs
the shared [conformance suite](clients.md#every-client-runs-the-same-conformance-suite)
over them. The service has no streaming RPC. Capability preflight (`GetServerInfo`
capabilities checked before a call) is typed in every client.

## Request options

| Option | Python | Node | Java | Rust | Julia | MATLAB |
| --- | --- | --- | --- | --- | --- | --- |
| `Convert.id_form` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `Convert.include_library_stubs` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `ConvertResponse.library_references` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `Convert.documents` | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | typed | typed |
| `Convert.compact`, `omit_derived`, `keep_derived` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert-options.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `Convert.include_check_diagnostics` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert-options.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `Query.oslc_query` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_query.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/query.test.ts) | typed | typed | typed | typed |
| `Query.order_by` (capability gated) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_query.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/query.test.ts) | typed | typed | typed | typed |
| `ParseSources.base_model_hash` / `affected` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_parse_sources.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/sources.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `RunSweep.samples` | typed | typed | typed | typed | typed | typed |
| `RenderView.compartments` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_document.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/document.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `RenderView.show`, `depth`, `ports` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_document.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/document.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `ImportData.import_as` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_import_compare.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/importCompare.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) |
| `RenderDocument.form` (markdown, html, asciidoc) | typed | typed | typed | typed | typed | typed |

## Cross-cutting features

| Feature | Python | Node | Java | Rust | Julia | MATLAB |
| --- | --- | --- | --- | --- | --- | --- |
| Bearer token (`OPENSYSML_GRPC_TOKEN`, explicit) | typed | typed | typed | typed | typed | typed |
| TLS to a remote service | typed | typed | typed | typed | typed | typed |
| Transport: Connect protobuf / JSON | typed | typed | typed | typed | typed | typed |
| In-process WebAssembly engine | — | typed (`connectWasm`) | — | — | — | — |
| Deadlines / timeouts | typed | typed | typed | typed | typed | typed |
| Typed `UNAUTHENTICATED` error | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/errors.test.ts) | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_errors_capabilities.m) |
| Typed `NOT_FOUND` split (model / file / symbol) | typed | typed | typed | typed | typed | typed |
| Typed `INVALID_ARGUMENT`, `DEADLINE_EXCEEDED`, `UNAVAILABLE`, `UNIMPLEMENTED` | typed | typed | typed | typed | typed | typed |
| Missing-capability error carrying the capability | typed | typed | typed | typed | typed | typed |
| Stale-service (version mismatch) error | typed | typed | typed | typed | typed | typed |
| Private child service lifecycle | typed | typed | typed | typed | typed | typed |
| Streaming / observe a run | — (no streaming RPC exists) | — | — | — | — | — |

Error typing: every client distinguishes the statuses the Python client's `errors.py`
distinguishes (`NOT_FOUND` by subject, `INVALID_ARGUMENT`/`FAILED_PRECONDITION`/`OUT_OF_RANGE`,
`DEADLINE_EXCEEDED`/`CANCELLED`, `UNAVAILABLE`, `UNIMPLEMENTED`, `UNAUTHENTICATED`) and
carries structured details (diagnostics, symbol suggestions, capability names).

## Open gaps

| Gap | Where | Estimate |
| --- | --- | --- |
| `RunSweep.samples` in the Go package | `client/opensysml` | about one day: the package has no typed `RunSweep` wrapper at all, so the option means a typed sweep request, result rows and their evaluations, and tests |
| In-process engine without a service | Python, Java, Rust, Julia, MATLAB | large; Node's `connectWasm` is the only one, and the others would need a wasm host each |

## Proving tests

Each linked cell above points at one of these files; the test names are the ones to search for.

- `py-conv`: [`client/python/tests/test_conversion.py`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py)
- `py-query`: [`client/python/tests/test_query.py`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_query.py)
- `py-doc`: [`client/python/tests/test_document.py`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_document.py)
- `py-import`: [`client/python/tests/test_import_compare.py`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_import_compare.py)
- `node-conv`: [`client/node/test/convert.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts)
- `node-query`: [`client/node/test/query.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/query.test.ts)
- `node-doc`: [`client/node/test/document.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/document.test.ts)
- `node-import`: [`client/node/test/importCompare.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/importCompare.test.ts)
- `node-err`: [`client/node/test/errors.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/errors.test.ts)
- `py-parse`: [`client/python/tests/test_parse_sources.py`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_parse_sources.py) (`base_model_hash` and `Model.affected`)
- `node-conv-options`: [`client/node/test/convert-options.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert-options.test.ts) (`compact`, `omitDerived`, `keepDerived`, `includeCheckDiagnostics`)
- `node-sources`: [`client/node/test/sources.test.ts`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/sources.test.ts) (`baseModelHash` and `model.affected`)
- `java-compact`: [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) (`compactApiJsonTrimsDerivedPropertiesAndKeepsTheNamedOnes`, `checkDiagnosticsReportTheSemanticCheckBesideTheConversion`, `aBaseModelNamesTheDocumentsAnEditReaches`) and [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/PublicTypesTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/PublicTypesTest.java) (capability refusals)
- `rust-compact`: [`client/rust/opensysml/tests/compact_affected.rs`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs)
- `go-options`: [`client/opensysml/parity_options_test.go`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/opensysml/parity_options_test.go) (`WithIDForm`, `WithLibraryStubs`, `WithBaseModel`, `ImportAs`)
- `java-conv`: [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) (`apiJsonNamesLibraryReferencesSeparatelyAndOptsIntoLegacyStubs`, `convertSpellsDerivedIdsInTheFormAskedFor`) and [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/EditorTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/EditorTest.java)
- `java-view`: [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) (`renderViewFoldsMembersIntoCompartmentRowsOnRequest`)
- `java-import`: [`client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) (`importDataNamesWhatEachRowIsMadeIntoAndRefusesAnUnknownForm`)
- `rust-conv`: [`client/rust/opensysml/tests/surface.rs`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) (`api_json_names_library_references_separately_and_opts_into_legacy_stubs`, `a_model_of_several_documents_converts_only_the_named_ones`) and [`client/rust/opensysml/src/conversion.rs`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/src/conversion.rs) unit tests
- `rust-view`: [`client/rust/opensysml/tests/surface.rs`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) (render view tests)
- `rust-import`: [`client/rust/opensysml/tests/surface.rs`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) (`import_data_sets_the_values_and_answers_the_edited_model`)
- `julia-conv`: [`client/julia/OpenSysML/test/extended_client.jl`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) (offline request-shape and error-mapping testsets)
- `julia-live`: [`client/julia/OpenSysML/test/extended_client.jl`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) (live section against `sysml-grpc`)
- `matlab-conv`: [`client/matlab/tests/test_surface_live.m`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) and `test_surface_offline.m`
- `matlab-live`: [`client/matlab/tests/test_surface_live.m`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m)
- `matlab-err`: [`client/matlab/tests/test_errors_capabilities.m`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_errors_capabilities.m)
