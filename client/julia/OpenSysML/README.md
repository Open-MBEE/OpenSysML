# OpenSysML Julia client

`OpenSysML` is a thin Julia client for the `sysml-grpc` service. It speaks
Connect with JSON bodies — `POST /sysml.SysMLService/<Method>` with
`Content-Type: application/json` — over HTTP.jl and JSON.jl. There is no
protobuf, no generated code and no binary download: everything on the wire is
decoded by the rules in
[`docs/reference/wire-contract.md`](../../../docs/reference/wire-contract.md).
It is not published to the Julia general registry; use it from this checkout:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
```

Requires **Julia 1.10** or later.

## Usage

```julia
using OpenSysML

conn = connect()                         # $OPENSYSML_SERVICE, else a private child
model = parse_source(conn, "package Demo { part def Car { attribute mass : Integer = 5; } }")

evaluate(model, "Demo::Car::mass")       # 5
inst = instantiate(model, "Demo::Car")   # inst.feature_values["mass"] == 5
close(conn)
```

### Connections

- `connect()` dials `$OPENSYSML_SERVICE` (`host:port`, or a full `http(s)://`
  URL) when set, and otherwise starts a private child.
- `external(address)` connects to a service somebody else runs; `close` does
  not stop it.
- `private(; binary=nothing)` starts `sysml-grpc -port 0 -health-port 0
  -report-address -exit-with-parent`, reads the address from the child's first
  stdout line, and holds the child's stdin pipe open for the connection's
  lifetime — that is the orphan guarantee, and it holds however this process
  dies. `close` closes stdin and waits, killing the child if it does not exit.
- Binary resolution, in order, never a download: `$OPENSYSML_GRPC_BINARY`,
  `~/.opensysml/bin/sysml-grpc` (`sysml-grpc.exe` on Windows), then `PATH`.

`call(conn, method, request)` posts the proto3-JSON request and returns the
decoded response (`Dict`, `Vector`, scalars — `JSON.parse` output). It is the
public escape hatch to every RPC the typed helpers do not wrap. A non-200
status whose body is `{"code","message"}` raises `ConnectError(code, message,
http_status)`; a connection failure, or an answer that is not JSON (the
`Content-Type` is checked before parsing, per the wire contract), raises
`TransportError`. The request timeout defaults to 30 s and is the `timeout`
keyword on every constructor.

`server_info(conn)` returns `(version, capabilities)`, asked once and cached;
`has_capability(conn, name)` checks one name.

### Operations

```julia
model  = parse_file(conn, "model.sysml"; language="sysml", strict=false)
model  = parse_source(conn, content; name="inline.sysml", language="")
model  = parse_sources(conn, [("a.sysml", a), ("b.sysml", b)]; strict=false)

diagnostics(model)                       # Vector{Diagnostic}
symbol(model, "Demo::Car")               # the GetSymbol answer as a Dict
evaluate(model, "mass"; context=nothing, subject=nothing)   # decoded Value
inst   = instantiate(model, "Demo::Car") # Instance(id, type_symbol_id, feature_values)
execute_action(model, "Test::addFive"; inputs=Dict("result" => 10), schedule="")
execute_state(model, "Test::Machine"; events=["start"], schedule="")
query(model, "oslc.where=rdf:type=\"PartUsage\"")
```

`evaluate` raises `DiagnosticError(message, diagnostics)` when the answer
carries a non-empty `error`; the same applies to every model-level helper. The
`execute_*` and `query` answers are the wire records with every `Value` inside
them decoded.

### Values

`decode_value` implements the wire contract's decoding rule for all nineteen
`Value` arms:

| Wire arm | Julia |
|---|---|
| `intValue` | `Int64` (exact, never a double) |
| `realValue` | `Float64` (`"NaN"`, `"Infinity"`, `"-Infinity"` accepted) |
| `boolValue` | `Bool` |
| `stringValue` | `String` |
| `instanceId` | `InstanceRef` |
| `sequence` | `Vector` of decoded values |
| `null` | `nothing` (`""`), else an error naming the unsupported value |
| `unset` | `Unset()` |
| `quantity` | `Quantity(magnitude, unit, unit_term)` |
| `enumLiteral` | `EnumLiteral(literal_id, enumeration_id, name, value)` |
| `complex` | `Complex` |
| `array` | `(dimensions, elements)` named tuple, product-checked |
| `vector` | `(components,)` — `Int64`/`Float64` only |
| `vectorQuantity` | `(components,)` — one `Quantity` each |
| `measurementRef` | `(unit, unit_id, unit_term)` — `unit_id` `nothing` for a composed unit |
| `infinity` | `Infinity()` (only `true` carries the arm) |
| `function` | `FunctionRef(calc_id, self)` |
| `set` | `Set` (a repeated member is an error) |
| `tensorQuantity` | `(dimensions, components)`, product-checked |
| `metaobject` | `Metaobject(element_id, metaclass_id)` |
| `undetermined` | `Undetermined(reason, lower, upper)` |

`encode_value` writes the request side: `Int64`→`intValue` (as a string),
`AbstractFloat`→`realValue`, `Bool`, `AbstractString`, `nothing`→`{"null":""}`,
`Quantity`, `Complex`, `InstanceRef`, `EnumLiteral`, vectors and sets→`sequence`
/`set`, and `FunctionRef` (a function read off an object is refused, as the
contract requires). `Unset`, `Undetermined` and `Infinity` are refused on
input.

### Errors

| Exception | When |
|---|---|
| `ConnectError(code, message, http_status)` | the service refused the call; `code` is the Connect code (`not_found`, `invalid_argument`, …) |
| `TransportError(message)` | the call never reached a decodable answer: connection failure, timeout, or a non-JSON answer (404/405/415 plain-text bodies) |
| `DiagnosticError(message, diagnostics)` | the call ran and the answer reports a model failure (`error` non-empty), with any diagnostics it carried |

## Conformance runner

`conformance/run.jl` drives every scenario in `conformance/scenarios/*.json`
through `call` — the whole RPC surface, no subset skips — and writes the same
report shape the other clients' runners write:

```bash
julia --project=client/julia/OpenSysML client/julia/OpenSysML/conformance/run.jl \
    --binary bin/sysml-grpc --report bin/conformance-report-julia.json
```

Flags: `--binary PATH` (start a private child; also `$OPENSYSML_GRPC_BINARY`)
or `--address host:port` (an existing service), `--scenarios DIR`,
`--fixtures DIR` (both defaulting to the repo's `conformance/`), `--run
SUBSTRING`, `--report FILE` (`-` for stdout), `--allow-skips`, `-v`. A scenario
skipped for a missing capability fails the run unless `--allow-skips` is given.
`make conformance-julia` at the repository root builds the service and runs it.

## Tests

```bash
julia --project=client/julia/OpenSysML -e 'using Pkg; Pkg.test()'
```

The `decode_value`/`encode_value` and runner-comparison tests need no service.
The live tests start a private child from `$OPENSYSML_GRPC_BINARY` or
`bin/sysml-grpc` and skip with a message when neither exists.

## What the client does not do

No protobuf bodies (JSON only — the measurable difference is
`docs/internals/design/transport-evaluation.md`'s), no binary download, and no
typed wrappers past the operations above: `ApplyEdits`, `Convert`, the
verification calls, `EvaluateCalc`, `RunAnalysis`, `RunSweep`, `RunDocumentQuery`
and `RenderDocument` are all reachable through `call` with the request spelled
as in the wire contract, answered as the plain record the wire gives.
