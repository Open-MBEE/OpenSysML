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
| `Convert.id_form` | typed [^py-conv] | typed [^node-conv] | typed [^java-conv] | typed [^rust-conv] | typed [^julia-conv] | typed [^matlab-conv] |
| `Convert.include_library_stubs` | typed [^py-conv] | typed [^node-conv] | typed [^java-conv] | typed [^rust-conv] | typed [^julia-conv] | typed [^matlab-conv] |
| `ConvertResponse.library_references` | typed [^py-conv] | typed [^node-conv] | typed [^java-conv] | typed [^rust-conv] | typed [^julia-conv] | typed [^matlab-conv] |
| `Convert.documents` | typed | typed | typed | typed [^rust-conv] | typed | typed |
| `Convert.compact`, `omit_derived`, `keep_derived` | — | typed [^node-conv] | — | — | — | — |
| `Convert.include_check_diagnostics` | — | typed [^node-conv] | — | — | — | — |
| `Query.oslc_query` | typed [^py-query] | typed [^node-query] | typed | typed | typed | typed |
| `Query.order_by` (capability gated) | typed [^py-query] | typed [^node-query] | typed | typed | typed | typed |
| `ParseSources.base_model_hash` / `affected` | — | — | — | — | — | — |
| `RunSweep.samples` | typed | typed | typed | typed | typed | typed |
| `RenderView.compartments` | typed [^py-doc] | typed [^node-doc] | typed [^java-view] | typed [^rust-view] | typed [^julia-live] | typed [^matlab-live] |
| `RenderView.show`, `depth`, `ports` | typed [^py-doc] | typed [^node-doc] | typed [^java-view] | typed [^rust-view] | typed [^julia-live] | typed [^matlab-live] |
| `ImportData.import_as` | typed [^py-import] | typed [^node-import] | typed [^java-import] | typed [^rust-import] | typed [^julia-live] | typed [^matlab-live] |
| `RenderDocument.form` (markdown, html, asciidoc) | typed | typed | typed | typed | typed | typed |

## Cross-cutting features

| Feature | Python | Node | Java | Rust | Julia | MATLAB |
| --- | --- | --- | --- | --- | --- | --- |
| Bearer token (`OPENSYSML_GRPC_TOKEN`, explicit) | typed | typed | typed | typed | typed | typed |
| TLS to a remote service | typed | typed | typed | typed | typed | typed |
| Transport: Connect protobuf / JSON | typed | typed | typed | typed | typed | typed |
| In-process WebAssembly engine | — | typed (`connectWasm`) | — | — | — | — |
| Deadlines / timeouts | typed | typed | typed | typed | typed | typed |
| Typed `UNAUTHENTICATED` error | typed | typed [^node-err] | typed | typed | typed [^julia-conv] | typed [^matlab-err] |
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
| `Convert.compact`, `omit_derived`, `keep_derived`, `include_check_diagnostics` as typed options | Python, Java, Rust, Julia, MATLAB | about one day across the five clients: options, capability gates (`convert_compact`, `convert_check_diagnostics`), per-diagnostic severity on the result, tests |
| `ParseSources.base_model_hash` and the `affected` list (incremental re-parse) | all six clients | about one day: option plus a typed `affected` result and tests against the `parse_sources_affected` capability |
| `Convert.id_form`, `include_library_stubs`, `RunSweep.samples`, `ImportData.import_as` in the Go package | `client/opensysml` | about half a day |
| In-process engine without a service | Python, Java, Rust, Julia, MATLAB | large; Node's `connectWasm` is the only one, and the others would need a wasm host each |

[^py-conv]: `client/python/tests/test_conversion.py`
[^py-query]: `client/python/tests/test_query.py`
[^py-doc]: `client/python/tests/test_document.py`
[^py-import]: `client/python/tests/test_import_compare.py`
[^node-conv]: `client/node/test/convert.test.ts`
[^node-query]: `client/node/test/query.test.ts`
[^node-doc]: `client/node/test/document.test.ts`
[^node-import]: `client/node/test/importCompare.test.ts`
[^node-err]: `client/node/test/errors.test.ts`
[^java-conv]: `client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java` (`apiJsonNamesLibraryReferencesSeparatelyAndOptsIntoLegacyStubs`, `convertSpellsDerivedIdsInTheFormAskedFor`) and `EditorTest.java`
[^java-view]: `ApiIntegrationTest.java` (`renderViewFoldsMembersIntoCompartmentRowsOnRequest`)
[^java-import]: `ApiIntegrationTest.java` (`importDataNamesWhatEachRowIsMadeIntoAndRefusesAnUnknownForm`)
[^rust-conv]: `client/rust/opensysml/tests/surface.rs` (`api_json_names_library_references_separately_and_opts_into_legacy_stubs`, `a_model_of_several_documents_converts_only_the_named_ones`) and `src/conversion.rs` unit tests
[^rust-view]: `client/rust/opensysml/tests/surface.rs` (render view tests)
[^rust-import]: `client/rust/opensysml/tests/surface.rs` (`import_data_sets_the_values_and_answers_the_edited_model`)
[^julia-conv]: `client/julia/OpenSysML/test/extended_client.jl` (offline request-shape and error-mapping testsets)
[^julia-live]: `client/julia/OpenSysML/test/extended_client.jl` (live section against `sysml-grpc`)
[^matlab-conv]: `client/matlab/tests/test_surface_live.m` and `test_surface_offline.m`
[^matlab-live]: `client/matlab/tests/test_surface_live.m`
[^matlab-err]: `client/matlab/tests/test_errors_capabilities.m`
