# OpenSysML for Julia

`OpenSysML` is a Julia 1.10+ client for the `sysml-grpc` service. It uses
Connect-JSON over `HTTP.jl`, with `JSON.jl` for the wire format. Install it
from a checkout:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
using OpenSysML
```

```julia
conn = connect()
model = parse_source(conn,
    "package Demo { part def Car { attribute mass : Integer = 5; } }")
car = instantiate(model, "Demo::Car")
close(conn)
```

## Connections and capabilities

`connect(; timeout=30, version=nothing, require_capabilities=[])` connects to
`$OPENSYSML_SERVICE` when set, otherwise it starts a private service child.
`external(address; ...)` connects to a service owned by another process, and
`private(; binary=nothing, ...)` starts one directly. `close(conn)` stops a
private child; it does not stop an external service. `call(conn, method,
request)` is the JSON-level escape hatch for any RPC.

`server_info(conn)` returns the service version and its advertised capability
set. `has_capability(conn, name)`, `name in server_info(conn)`, and
`require_capability(conn, name)` support capability-aware code. Connection and
capability failures include `ConnectError`, `TransportError`,
`MissingCapabilityError`, and `StaleServiceError`.

Binary resolution checks `$OPENSYSML_BINARY`, then
`$OPENSYSML_GRPC_BINARY`, the executable at `~/.opensysml/bin/sysml-grpc`
(`sysml-grpc.exe` on Windows), a requested release download, and then `PATH`.
Without a requested version, a missing binary is an error rather than an
implicit download. `$OPENSYSML_GRPC_VERSION=latest` or a release tag enables
download and can also be supplied as `version=` to `private` or `connect`.
`$OPENSYSML_GITHUB_REPO` overrides the release repository (default
`Open-MBEE/OpenSysML`). Downloads are size-bounded, checked against the
package's SHA-256 release pins, installed atomically, and made executable.
An unpinned release is refused unless
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` names the repository or is `1`; then the
release's `.sha256` sidecar is used.

## Models, values, and RPCs

```julia
model = parse_file(conn, "model.sysml"; language="sysml")
model = parse_source(conn, source; name="inline.sysml")
model = parse_sources(conn, [("a.sysml", source_a), ("b.sysml", source_b)])
model = parse_source(conn, source; strict=true, strict_conformance=true)

diagnostics(model)
symbol(model, "Demo::Car")
find(model, "Car")
get(model, "Demo::Car", nothing)
evaluate(model, "2 + 2")
instantiate(model, "Demo::Car")
execute_action(model, "Demo::Action"; inputs=Dict("x" => 1))
execute_state(model, "Demo::Machine"; events=["start"])
```

`strict=true` raises `ModelError` when parsing reports error-severity
diagnostics, matching the Python client. The separate wire option is
`strict_conformance=true`; it asks the service for strict conformance checks.
`parse_sources` accepts an ordered sequence of paths, `(name, content)` pairs,
or `SourceDocument`s.

`decode_value` and `encode_value` handle all service `Value` arms, including
exact `Int64`, quantities, units, enum literals, instances, collections,
arrays, vectors, tensors, functions, metaobjects, infinity and undetermined
values. `Quantity`, `Unit`, `UnitFactor`, `MeasurementRef`, `VectorQuantity`,
and `TensorQuantity` are Julia types. Use `in_unit(q, unit)` to inspect a
magnitude in a unit and `to_unit(q, unit)` to convert it.

The typed wrappers cover verification (`verify_constraint`,
`verify_requirement`, `verify_satisfaction`, `satisfied`, and
`validate_instance`), calculations (`calc`), analyses (`run_analysis`),
parameter sweeps (`run_sweep`), and schedule exploration. `list_engines`
lists available engines. Sweep ranges accept one `Pair`, an ordered vector of
pairs, or a dictionary with at most one entry.

Query helpers include `build_query`, `query`, `build_document_bindings`,
`run_document_query`, and `render_document`. Conversion is available through
`convert_file`, `convert_source`, `convert_model`, `to_sysml`, `to_turtle`,
`to_api_json`, and `save`. RDF/Turtle and API-JSON conversions emit an
experimental-feature warning. A SysML v1 model (`.mdzip`, `.xmi`, `.uml`) is
migrated, not converted: `convert_file` refuses it, and `migrate_file(conn,
path, to_format; report=true)` or `migrate_source(conn, bytes, to_format;
from_format="mdzip")` answer a `Migration` whose `report` gives every
element's verdict (`by_verdict(migration.report, "unmapped")`) and which
`save(migration, path)` writes with its image files beside the model.

## Authoring

```julia
result = edit(model) do editor
    set_value(editor, "Demo::Car::mass", "6")
    add_attribute(editor, "Demo::Car", "wheelCount"; type="Integer", value="4")
end
result.content
result.applied
result.documents
```

`edit(model)` also returns an `Editor` for non-do-block use. `apply_edits`
accepts that editor, a `Model` and raw operations, or a `Connection`, model
hash, and raw operations. `Editor` methods cover set-value, rename, move,
delete, declarations, imports, metadata, comments, documentation,
requirements, transitions, connections, and action bodies. `Body` builds
nested action-body statements. Every authoring operation is checked against
the service capability it needs before `ApplyEdits` is sent; multi-document
edits require `edit_documents`.

The result types are `EditResult`, `AppliedEdit`, `EditedDocument`, and
`Referrer`. Edit failures are typed under `EditError`; each concrete error is
a direct subtype. In particular, `IllegalMemberKindError` is not a subtype
of `InvalidEditError`.

## Typed generation

`generate_source(model, source_text)` returns a Julia module with a wrapper
for each model definition and typed feature accessor functions. Multiplicity
determines whether accessors return one value, `Union{Nothing,T}`, or
`Vector{T}`. The generated module records the generator version and a
normalized SHA-256 hash of its source text.

The command-line entry point accepts a source file, `-o`/`--output`,
`--host`, `--port`, and `--check`:

```sh
julia --project=client/julia/OpenSysML \
  -e 'using OpenSysML; OpenSysML.generate_main(ARGS)' -- model.sysml -o model_types.jl
```

## Not provided

- Sigstore signature verification of `SHA256SUMS.txt`; pinned release digests
  are checked, and explicitly allowed unpinned downloads use the served
  `.sha256` sidecar.
- The Python FMI reference runner, which relies on FMPy.
- A process-wide shared/refcounted private child or fork-disown behavior;
  Julia has no fork model, and each `private()` connection owns its child.

The [Julia API reference](../../../docs/reference/julia-api.md) has the complete
surface and the [conformance runner](conformance/run.jl) exercises the shared
service scenarios.
