# Client feature parity

The seven remote clients wrap every RPC of `sysml-grpc` with a typed API. This page records,
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
over them. The 32nd, `InstancePaths` (every root-relative path to one object, capability
`instance_paths`), is typed in the Go, Python, Node, Java and .NET clients — Python as
`Instance.paths()` beside `Instance.owner`, `.feature` and `.path`, Node as
`Model.instancePaths()` beside `Instance.reachedBy`, `.ownerId` and `.path`, Java as
`Model.instancePaths()` beside `Instance.reachedBy()`, `.owner()` and `.path()`, .NET as
`Model.InstancePathsAsync()` and `InstancePathsAtAsync()` beside `Instance.ReachedBy`, `.Path` and
`Instantiation.FindOwner()`, each reading the `reached_by` of an instantiated `Instance` — and the
other clients carry `reached_by`
through their generated `Instance` message. The service has no streaming RPC. Capability preflight (`GetServerInfo`
capabilities checked before a call) is typed in every client.

## Capabilities

The service announces what it can do as capability names (`GetServerInfo.capabilities`), and
a client binds a capability by declaring its name in its capabilities module and gating or
decoding by it: `symbol_members` is bound where `Symbol.members` is typed with its provenance
and `include_library_members` can be asked for, `element_properties` where
`GetElementProperties` is a typed call, `instance_paths` where `InstancePaths` is one and
`reached_by` is read. The table below is the checklist of every capability the service
announces against every client, in the service's order. **Not bound** means the client declares
no such name: it neither gates on the capability nor decodes what it populates, and a program
that needs it reads the raw response or uses another client.

`TestClientCapabilityChecklist` in `tests/hygiene` holds this table to the code in both
directions: every capability `sysml-grpc` announces must have a row, a row must name a
capability the service still announces, and each cell must agree with the client's
capabilities module (`client/python/opensysml/capabilities.py`, `client/opensysml/types.go`,
`client/node/src/core/capabilities.ts`, `Capabilities.java`,
`client/rust/opensysml/src/capabilities.rs`, `client/julia/OpenSysML/src/capabilities.jl`,
`client/matlab/+opensysml/capabilities.m`). A capability added to the service without a row,
or bound in a client without its cell saying so, fails the suite, so the table cannot drift
from the clients and a gap is a stated gap rather than a silent one.

| Capability | Python | Go | Node | Java | Rust | Julia | MATLAB |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `type_facts` | bound | bound | bound | bound | bound | bound | bound |
| `convert` | bound | bound | bound | bound | bound | bound | bound |
| `verification` | bound | bound | bound | bound | bound | bound | bound |
| `query` | bound | bound | bound | bound | bound | bound | bound |
| `oslc_query` | bound | bound | bound | bound | bound | bound | not bound |
| `enum_values` | bound | bound | bound | bound | bound | bound | bound |
| `evaluate_subject` | bound | bound | bound | bound | bound | bound | bound |
| `symbol_attributes` | bound | bound | bound | bound | bound | bound | bound |
| `unset_value` | bound | bound | bound | bound | bound | bound | bound |
| `feature_values` | bound | bound | bound | bound | bound | bound | bound |
| `apply_edits` | bound | bound | bound | bound | bound | bound | bound |
| `authoring` | bound | bound | bound | bound | bound | bound | bound |
| `inline_language` | bound | bound | bound | bound | bound | bound | bound |
| `strict_conformance` | bound | bound | bound | bound | bound | bound | bound |
| `document_query` | bound | bound | bound | bound | bound | bound | bound |
| `render_document` | bound | bound | bound | bound | bound | bound | bound |
| `parse_sources` | bound | bound | bound | bound | bound | bound | bound |
| `complex_values` | bound | bound | bound | bound | bound | bound | bound |
| `structured_values` | bound | bound | bound | bound | bound | bound | bound |
| `measurement_refs` | bound | bound | bound | bound | bound | bound | bound |
| `function_values` | bound | bound | bound | bound | bound | bound | bound |
| `set_values` | bound | bound | bound | bound | bound | bound | bound |
| `tensor_values` | bound | bound | bound | bound | bound | bound | bound |
| `verification_verdicts` | bound | bound | bound | bound | bound | bound | bound |
| `infinity_value` | bound | not bound | bound | bound | bound | bound | bound |
| `diagnostic_codes` | bound | bound | bound | bound | bound | bound | bound |
| `schedule` | bound | bound | bound | bound | bound | bound | bound |
| `case_evaluations` | bound | bound | bound | bound | bound | bound | bound |
| `schedule_explore` | bound | bound | bound | bound | bound | bound | bound |
| `final_time` | bound | bound | bound | bound | bound | bound | bound |
| `engines` | bound | bound | bound | bound | bound | bound | bound |
| `metaobject_values` | bound | bound | bound | bound | bound | bound | bound |
| `symbol_members` | bound | bound | bound | bound | bound | not bound | not bound |
| `undetermined_value` | bound | bound | bound | bound | bound | bound | bound |
| `engines_external` | not bound | not bound | not bound | not bound | bound | not bound | not bound |
| `edit_documents` | bound | bound | bound | bound | bound | bound | bound |
| `performer` | bound | bound | bound | bound | bound | bound | bound |
| `render_document_html` | bound | not bound | bound | bound | bound | bound | bound |
| `render_document_asciidoc` | bound | not bound | bound | bound | bound | bound | bound |
| `connection_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `satisfy_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `requirement_constraint_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `member_modifiers` | bound | bound | bound | bound | bound | bound | bound |
| `transition_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `verification_questions` | bound | bound | bound | bound | bound | bound | bound |
| `verification_arguments` | bound | bound | bound | bound | bound | bound | not bound |
| `query_order_by` | bound | not bound | bound | bound | bound | bound | bound |
| `verification_objective_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `runs` | bound | bound | bound | bound | bound | bound | bound |
| `self_check` | bound | bound | bound | bound | bound | bound | bound |
| `record_run` | bound | bound | bound | bound | bound | bound | bound |
| `metadata_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `metadata_prefix_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `sequence_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `implicit_parameters` | bound | bound | bound | bound | bound | bound | bound |
| `constraint_body_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `state_action_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `import_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `documentation_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `comment_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `action_body_statement_authoring` | bound | bound | bound | bound | bound | bound | bound |
| `migrate` | bound | bound | bound | bound | bound | bound | not bound |
| `big_int_values` | bound | bound | bound | bound | bound | bound | bound |
| `rational_values` | bound | bound | bound | bound | bound | bound | bound |
| `state_trace` | bound | bound | bound | bound | bound | bound | bound |
| `action_trace` | bound | bound | bound | bound | bound | bound | bound |
| `render_view` | bound | bound | bound | bound | bound | bound | bound |
| `export_graphs` | bound | bound | bound | bound | bound | bound | bound |
| `import_data` | bound | bound | bound | bound | bound | bound | bound |
| `compare_results` | bound | bound | bound | bound | bound | bound | bound |
| `instance_paths` | bound | bound | bound | bound | not bound | not bound | not bound |
| `check` | bound | bound | bound | bound | bound | bound | bound |
| `convert_documents` | bound | bound | bound | bound | bound | bound | not bound |
| `convert_compact` | bound | bound | bound | bound | bound | bound | bound |
| `convert_library_references` | bound | bound | bound | bound | bound | bound | not bound |
| `convert_library_stubs` | bound | bound | bound | bound | bound | bound | bound |
| `convert_check_diagnostics` | bound | bound | bound | bound | bound | bound | bound |
| `parse_sources_affected` | bound | bound | bound | bound | bound | bound | bound |
| `render_document_file_name` | bound | bound | bound | bound | bound | bound | not bound |
| `order_dependence` | bound | not bound | bound | not bound | not bound | not bound | not bound |
| `verdict_notes` | bound | not bound | bound | bound | bound | bound | bound |
| `element_properties` | bound | not bound | bound | bound | bound | not bound | not bound |


## Request options

| Option | Python | Node | Java | Rust | Julia | MATLAB | .NET |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `Convert.id_form` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `Convert.include_library_stubs` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `ConvertResponse.library_references` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `Convert.documents` | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `Convert.compact`, `omit_derived`, `keep_derived` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert-options.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `Convert.include_check_diagnostics` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_conversion.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/convert-options.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelConversionTests.cs) |
| `Query.oslc_query` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_query.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/query.test.ts) | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelQueryDocsTests.cs) |
| `Query.order_by` (capability gated) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_query.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/query.test.ts) | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelQueryDocsTests.cs) |
| `ParseSources.base_model_hash` / `affected` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_parse_sources.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/sources.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/compact_affected.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelParseTests.cs) |
| `RunSweep.samples` | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelAnalysisTests.cs) |
| `RenderView.compartments` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_document.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/document.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelQueryDocsTests.cs) |
| `RenderView.show`, `depth`, `ports` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_document.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/document.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelQueryDocsTests.cs) |
| `ImportData.import_as` | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/tests/test_import_compare.py) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/importCompare.test.ts) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/opensysml-client/src/test/java/org/openmbee/opensysml/ApiIntegrationTest.java) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/opensysml/tests/surface.rs) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_surface_live.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelGraphDataTests.cs) |
| `RenderDocument.form` (markdown, html, asciidoc) | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelQueryDocsTests.cs) |

## Cross-cutting features

| Feature | Python | Node | Java | Rust | Julia | MATLAB | .NET |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Bearer token (`OPENSYSML_GRPC_TOKEN`, explicit) | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/BearerTokenTests.cs) |
| TLS to a remote service | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/NetworkFailureTests.cs) |
| Transport: Connect protobuf / JSON | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ConnectTransportTests.cs) |
| In-process WebAssembly engine | — | typed (`connectWasm`) | — | — | — | — | — |
| Deadlines / timeouts | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/NetworkFailureTests.cs) |
| Typed `UNAUTHENTICATED` error | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/test/errors.test.ts) | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/test/extended_client.jl) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/tests/test_errors_capabilities.m) | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/BearerTokenTests.cs) |
| Typed `NOT_FOUND` split (model / file / symbol) | typed | typed | typed | typed | typed | typed | typed |
| Typed `INVALID_ARGUMENT`, `DEADLINE_EXCEEDED`, `UNAVAILABLE`, `UNIMPLEMENTED` | typed | typed | typed | typed | typed | typed | [by status code](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/TransportContractTests.cs) |
| Missing-capability error carrying the capability | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ModelParseTests.cs) |
| Stale-service (version mismatch) error | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/ConnectionOpenTests.cs) |
| Private child service lifecycle | typed | typed | typed | typed | typed | typed | [typed](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/PrivateConnectionTests.cs) |
| Streaming / observe a run | — (no streaming RPC exists) | — | — | — | — | — | — |

Error typing: every client distinguishes the statuses the Python client's `errors.py`
distinguishes (`NOT_FOUND` by subject, `INVALID_ARGUMENT`/`FAILED_PRECONDITION`/`OUT_OF_RANGE`,
`DEADLINE_EXCEEDED`/`CANCELLED`, `UNAVAILABLE`, `UNIMPLEMENTED`, `UNAUTHENTICATED`) and
carries structured details (diagnostics, symbol suggestions, capability names). The .NET client
gives `NOT_FOUND` by subject, `UNAUTHENTICATED` and a missing capability exceptions of their own;
any other status is a `ServiceException`, or a `TransportException` for a deadline or a network
failure, carrying its `StatusCode`, rather than an exception type per status.

## Open gaps

| Gap | Where | Estimate |
| --- | --- | --- |
| `RunSweep.samples` in the Go package | `client/opensysml` | about one day: the package has no typed `RunSweep` wrapper at all, so the option means a typed sweep request, result rows and their evaluations, and tests |
| In-process engine without a service | Python, Java, Rust, Julia, MATLAB, .NET | large; Node's `connectWasm` is the only one, and the others would need a wasm host each |

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
- `dotnet`: [`client/dotnet/test/OpenMBEE.OpenSysML.Tests/`](https://github.com/Open-MBEE/OpenSysML/blob/main/client/dotnet/test/OpenMBEE.OpenSysML.Tests/) (`ModelConversionTests`, `ModelQueryDocsTests`, `ModelParseTests`, `ModelAnalysisTests` and `ModelGraphDataTests` send each option over every wire to a fake service; `BearerTokenTests`, `NetworkFailureTests`, `ConnectTransportTests`, `TransportContractTests`, `ConnectionOpenTests` and `PrivateConnectionTests` cover the cross-cutting rows)
