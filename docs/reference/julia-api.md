# The Julia client API

`OpenSysML` is a Julia 1.10+ client for `sysml-grpc`. It sends Connect-JSON
requests over `HTTP.jl` and decodes responses with `JSON.jl`; there is no
generated protobuf dependency. The package is not yet in the Julia General
registry:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
using OpenSysML
```

## Connecting and binary resolution

```julia
conn = connect(; timeout=30, version=nothing, require_capabilities=[])
conn = external("localhost:50051"; timeout=30, token=nothing)
conn = private(; binary=nothing, timeout=30)
```

`connect` uses `$OPENSYSML_SERVICE` when set and otherwise starts a private
child. `external` connects to a service managed elsewhere; closing it only
disconnects. A service on an address other machines reach requires a bearer
token, which `external` and `connect` send as `Authorization: Bearer` on every
call: the `token` keyword, or `$OPENSYSML_GRPC_TOKEN` (`TOKEN_ENV`) when none
is given; a call without the right one raises `ConnectError` with code
`unauthenticated`. `private` resolves and starts its own child, without
`$OPENSYSML_GRPC_TOKEN` in its environment: a private child needs no token. These constructors
accept a timeout, expected service version, and required capabilities.
`call(conn, method, request)` posts a proto3-JSON request and returns the
decoded JSON value.

Binary lookup follows the package's configured source order:
`$OPENSYSML_BINARY`, `$OPENSYSML_GRPC_BINARY`, the shared cache at
`~/.opensysml/bin/sysml-grpc` (or `.exe`), a release download when requested,
and then `PATH`. Set `$OPENSYSML_GRPC_VERSION=latest` or a release tag to
enable automatic download, or pass `version=`. Without a version request,
download is disabled. `$OPENSYSML_GITHUB_REPO` changes the default
`Open-MBEE/OpenSysML` release repository. A download is bounded to 512 MiB
for the binary and 8 MiB for metadata, has a 15-second network timeout,
verifies the pinned SHA-256 digest, and installs through a temporary file and
atomic rename.

If a release has no digest in the package table,
`UnpinnedReleaseError` refuses it unless
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` is `1`/`true`/`yes` or names the
repository. An allowed download verifies the `.sha256` sidecar and warns that
the release is unpinned. A mismatch raises `ChecksumMismatchError`. The
package does not verify a Sigstore signature for `SHA256SUMS.txt`.

## Capabilities and errors

`server_info(conn)` negotiates and caches `ServerInfo`; `has_capability(conn,
capability)` and `capability in server_info(conn)` query the result.
`require_capability(conn, capability)` raises `MissingCapabilityError` with
an upgrade remedy when a service lacks an operation. If an expected service
version does not match, `StaleServiceError` describes the mismatch.

The error hierarchy includes `OpenSysMLError`, `ConnectError`,
`TransportError`, `ModelError`, `DiagnosticError`, `ExecutionError`,
`ConversionError`, `MigrationError`, `MissingCapabilityError`, `StaleServiceError`,
`SymbolNotFoundError`, `TypeMismatchError`, and `FeatureValueError`.
`ConnectError` is abstract; concrete Connect failures include
`InvalidRequestError`, `ModelNotFoundError`, `ModelFileNotFoundError`,
`AuthenticationError` (the service refused the call's bearer token),
`ServiceTimeoutError`, `UnsupportedOperationError`, and
`ServiceUnavailableError`.

Each concrete edit failure is a direct subtype of abstract `EditError`.
`IllegalMemberKindError` is a flat subtype of `EditError`, not an
`InvalidEditError`. Errors preserve failure names, diagnostics, referring
elements, and referrers.

## Parsing and models

```julia
model = parse_file(conn, "model.sysml"; language="sysml", strict=false,
                   strict_conformance=false)
model = parse_source(conn, source; name="inline.sysml", language="",
                     strict=false, strict_conformance=false)
model = parse_sources(conn, [("a.sysml", source_a), ("b.sysml", source_b)];
                      strict=false, strict_conformance=false)

diagnostics(model)
errors(model)
symbol(model, "Demo::Car")
get_symbol(model, "Demo::Car")
find(model, "Car")
get(model, "Demo::Car", nothing)
```

`strict=true` raises `ModelError` if parse diagnostics include an error. The
wire-level strict conformance flag is named `strict_conformance`; it is
separate from `strict`. `parse_sources` preserves input order and accepts
`SourceDocument`s, file paths, or `(name, content)` pairs.

Model operations include `evaluate(model, expression; context, subject)`,
`instantiate(model, symbol_id)`, `execute_action(model, symbol_id; inputs,
schedule)`, and `execute_state(model, symbol_id; events, schedule, trace)`.
`execute_action` returns an `ActionOutputs` dictionary of output parameters;
performer attributes are available separately as `performer`. `execute_state`
returns a `StateRun` with `states_visited`, `final_context`, `final_time`,
`trace::Vector{DocumentEvent}`, and `trace_dropped`. Requesting a trace checks
the `state_trace` capability locally and cannot be combined with exploration.
A failed traced run keeps its existing `ExecutionFailure` or `WrongKindError`
classification, with partial records on `trace` and the discarded count on
`trace_dropped`.
`Instance`, `InstanceRef`, `TypeFacts`, `SymbolFacts`, `SymbolInfo`,
`Multiplicity`, and `Diagnostic` carry decoded model and service results.

## Values and units

`decode_value` decodes the service's value oneof and `encode_value` creates
request values. The mapping retains exact `Int64` values and `BigInt` ones
beyond `Int64`, decodes an exact Rational no `Float64` holds (`rationalValue`, a quantity's
`rationalMagnitude`) as `Rational{BigInt}`, sends every `Rational` (`1//4` included) as
`rationalValue` to a service with the `rational_values` capability and, to one without it, one a
`Float64` holds as that `realValue` while refusing any other (a `Float64` sent is always a Real), decodes sequences and sets recursively, and exposes
structured values through `Quantity`, `EnumLiteral`, `ArrayValue`,
`VectorValue`, `VectorQuantity`, `TensorQuantity`, `MeasurementRef`,
`FunctionRef`, `Metaobject`, `Undetermined`, `Unset`, and `Infinity`.

`Unit` and `UnitFactor` represent units and factors. `in_unit(quantity,
unit)` reads a magnitude in another compatible unit; `to_unit(quantity,
unit)` converts it. `same_value(left, right)` compares values using the
client's unit semantics. `unit`, `describe`, `exponents`, `magnitudes`,
`reduction`, and `commensurable` are module functions, not exported names.

## Verification, execution, analyses, and sweeps

```julia
verify_constraint(model, "Demo::constraint"; question=nothing, engine=nothing,
                  arguments=Any[], named_arguments=Dict())
verify_requirement(model, "Demo::requirement"; question=nothing, engine=nothing,
                   arguments=Any[], named_arguments=Dict())
verify_satisfaction(model, "Demo::satisfy"; question=nothing, engine=nothing)
satisfied(model, "Demo::requirement"; question=nothing, engine=nothing)
validate_instance(model, "Demo::car"; engine=nothing)
calc(model, "Demo::sum"; inputs=Dict("x" => 2), engine=nothing)
run_analysis(model, "Demo::analysis"; inputs=Dict(), engine=nothing, tool_dry_run=false)
run_sweep(model, "Demo::analysis", ["Demo::x" => [1, 2]];
          inputs=Dict(), engine=nothing)
list_engines(conn; probe=false)
execute_action(model, "Demo::action"; inputs=Dict(), schedule="", performer=nothing,
               advance=nothing, seed=nothing, draws=nothing, clock_step=0.0)
execute_state(model, "Demo::machine"; events=String[], schedule="", performer=nothing,
              trace=false, advance=nothing, seed=nothing, draws=nothing, clock_step=0.0)
refresh_diagnostics(model; disable_lint=(), enable_lint=())
explore_analysis(model, "Demo::analysis"; engine=nothing)
explore_action(model, "Demo::action"; inputs=Dict(), engine=nothing)
explore_state(model, "Demo::machine"; events=String[], engine=nothing)
```

Engine, question, and schedule options are checked against their respective
service capabilities before the RPC. A `Verdict` lists in `notes` what a solver's
answer rests on, each a `VerdictNote` (`kind` `"assumed_one"` or `"uncovered"`,
`subject`, `text`), empty from a service without `verdict_notes`. `execute_action` and `execute_state` take
the run options of the `sysml` command: `advance` (simulated seconds to run
for), `seed` (the model's own draws), `draws` (`"random"`, `"min"`, `"max"` or
`"average"`) and `clock_step`; a negative or unknown one is an `ArgumentError`
before anything is sent. `run_analysis(...; tool_dry_run=true)` runs the case
only to its first external tool call and answers what the tool would be given
in the result's `tool_dry_run` (a `ToolDryRun`: `action`, `tool`, `version`,
`executable`, `args`, `env`, `inputs`, `outputs`, `lines`, ...), as
`sysml -tool-dry-run` does. `list_engines(conn; probe=true)` starts each
external engine and checks its handshake before listing it.
`refresh_diagnostics(model; disable_lint, enable_lint)` asks the service for
the model's diagnostics again under a lint selection, as
`sysml -disable-lint`/`-enable-lint` select them; `model.diagnostics` stays
what the parse answered. Sweep ranges may be one `Pair`, an
ordered `AbstractVector` of pairs, or an `AbstractDict` with no more than one
entry. Multi-entry dictionaries are rejected because iteration order is not a
stable parameter order.

## Queries, documents, and conversion

`build_query(; scope, select, where, order_by)` constructs a standard query
(all seven `PrimitiveConstraint` operators; `order_by` sorts the answer by each
named property in turn, ascending); `query(model, payload; scope, select, where,
order_by)` runs it. Document APIs are
`build_document_bindings`, `run_document_query(model, query; bindings,
options)`, `render_document(model, query; bindings, format)`, and
`render_view(model, name; ports="minimal", show=String[], depth=0, compartments="")`, returning
typed query and view data. `ports="full"` includes all declared ports;
`depth=n` draws only `n` levels of nested elements below each exposed element
(`0` draws every level the view allows); `show=["edge-labels"]` draws
those [optional notation elements](../project/view-rendering-forms.md#optional-notation-elements)
besides the ones the view's rendering shows (`"all"` for every one), and the
`RenderedView`'s `shown` lists the elements drawn. `compartments` says how a
tree rendering folds owned members into compartment rows (`"positioned"`, the
default, `"most"` or `"ports"`); a folded `RenderNode` names its `compartment`.

Conversion is available through `convert_file(conn, path, to_format; ...)`,
`convert_source(conn, content, to_format; from_format, ...)`,
`convert_model(conn, model_hash, to_format; ...)`, and
`convert_model(model, to_format; ...)`, each taking `id_form`
(`ID_FORM_QUALIFIED`, the default, or `ID_FORM_UUID`) for the derived ids of a
graph notation and `include_library_stubs`, which restores in api-json the
ownerless library stubs releases before 0.11.0 wrote (needs the service's
`convert_library_stubs` capability); the `Conversion` lists the
`library_references` (`LibraryReference`) an api-json conversion names beside
its elements. `to_sysml`, `to_turtle`, `to_api_json`, and `save` are model
helpers. Turtle and API-JSON formats are experimental and emit `@warn`. A SysML v1 model — `from_format` `xmi`, `uml`
or `mdzip`, or a path with that extension — is refused by every `convert_*`
with an `ArgumentError`: it is migrated, not converted.

Migration is `migrate_file(conn, path, to_format; from_format, report,
results, layout_path, layout_content, image_base_url, strict)` and
`migrate_source(conn, bytes, to_format; from_format, ...)`, answering a
`Migration` with `content`, `from_format`, `to_format`, a `MigrationReport`
(`summary` and the `mapped`, `approximated`, `unmapped` and `skipped` counts
always; every element's `MigrationEntry` in `entries` and the report `text`
with `report=true`; `by_verdict(report, verdict)` selects them), `results`,
the image `files` by relative path, `source_path` and the
`experimental_notice` it `@warn`s. `save(migration, path)` writes the model
and its images beside it, refusing to overwrite the v1 model or to write an
image outside the model's directory. A migration the service cannot read
raises `MigrationError`.

## Authoring

```julia
result = edit(model) do editor
    rename(editor, "Demo::OldName", "NewName")
    add_attribute(editor, "Demo::Car", "wheelCount";
                  type="Integer", value="4")
end

editor = edit(model)
set_value(editor, "Demo::Car::mass", "6")
result = apply_edits(editor)
```

`edit(model)` returns an `Editor`; `edit(model) do editor ... end` applies the
collected operations. `apply_edits(model, operations)` and
`apply_edits(conn, model_hash, operations; document="", accept_documents=true,
multi_document=false)` accept raw operations. Editor methods cover
`set_value`, `rename`, `move`, `delete`, declaration and member helpers,
connections, requirements, transitions, metadata, documentation, imports,
comments, and nested action statements. `Body` composes nested action
statements including `if`, `while`, `loop`, `for`, `accept`, `send`,
assignment, and termination. Capability preflights follow the operation and
its options; multi-document authoring requires `edit_documents`.

Results are `EditResult`, `AppliedEdit`, `EditedDocument`, and `Referrer`.
`EditResult` carries edited text, formats, diagnostics, experimental metadata,
applied source ranges, and changed documents.

## Generated typed wrappers

```julia
source_text = read("model.sysml", String)
model = parse_file(conn, "model.sysml")
generated = generate_source(model, source_text)
include_string(@__MODULE__, generated)
instance = instantiate(model, "Demo::Car")
car = OpenSysML.from_instance(GeneratedModel.Car, instance)
GeneratedModel.feature_Car_mass(car)
```

`generate_source(model, source_text)::String` emits one wrapper struct for
each definition, with typed functions for its features. It maps primitive,
quantity, enum, and generated object types and respects multiplicity:
required single values return `T`, optional values return
`Union{Nothing,T}`, and collections return `Vector{T}`. The module records a
generator version and a SHA-256 model-source stamp. Julia-safe identifiers
are deterministic and collisions are disambiguated.

Run generation from the command line:

```sh
julia --project=client/julia/OpenSysML \
  -e 'using OpenSysML; OpenSysML.generate_main(ARGS)' -- model.sysml -o model_types.jl
```

`generate_main` also accepts `--host`, `--port`, and `--check`.

## Not provided

- Sigstore verification of `SHA256SUMS.txt`; the package checks pinned release
  digests and supports explicitly allowed sidecar validation for unpinned
  releases.
- The Python FMI reference runner, which depends on FMPy.
- Process-wide shared/refcounted private-child and fork-disown behavior;
  Julia has no fork, and each `private()` connection owns its child.

The package's [README](../../client/julia/OpenSysML/README.md) includes
installation and examples. See the [wire contract](wire-contract.md) for the
Connect-JSON shapes and the [client guide](../clients/julia.md)
for a walkthrough.
