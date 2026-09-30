# The Julia client API

This page covers what the `OpenSysML` package exposes. To choose between the clients, see
[client libraries](clients.md); for a task-oriented walkthrough, see
[guide chapter 9](../guide/09-clients.md#from-julia). The package's own notes are in
[client/julia/OpenSysML/README.md](../../client/julia/OpenSysML/README.md).

The client is a thin JSON-over-HTTP client of the Connect-JSON surface — no protobuf, no
generated code, and it never downloads a binary. Julia **1.10** is the declared `[compat]`
floor; the only dependencies are `HTTP.jl` and `JSON.jl` (0.21.x).

Nothing is published to the General registry yet, so a consumer takes it from a path:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
using OpenSysML
```

## Connection

```julia
conn = connect()                      # $OPENSYSML_SERVICE, else a private child
conn = private()                      # a child of this process
conn = external("localhost:50051")    # a service someone else runs
```

`private()` resolves the binary in the order `$OPENSYSML_GRPC_BINARY`,
`~/.opensysml/bin/sysml-grpc`, `PATH`, and starts it with `-port 0 -health-port 0
-report-address -exit-with-parent`, reading the address from the child's first stdout line. The
client holds the write end of the child's stdin and never writes to it, so the child exits at end
of file however this process dies; `close(conn)` closes that pipe and reaps the child. Closing an
`external` connection never stops that service. `server_info(conn)` asks `GetServerInfo` once and
caches it; `has_capability(conn, name)` reads the negotiated capability names.

## Models and calls

```julia
model = parse_file(conn, "model.sysml"; language = "sysml", strict = true)
model = parse_source(conn, "package P { part def B; }"; name = "p.sysml")
model = parse_sources(conn, [("a.sysml", src_a), ("b.sysml", src_b)])

diagnostics(model)                    # Vector{Diagnostic}: severity, message, file, line, col, code
symbol(model, "P::B")                 # the plain decoded record
evaluate(model, "2 + 2"; context = "P", subject = "P::obj")
instantiate(model, "P::B")            # Instance: id, type_symbol_id, feature_values::Dict
execute_action(model, "P::act"; inputs = Dict("x" => 1), schedule = "")
execute_state(model, "P::sm"; events = ["go"], schedule = "")
query(model, "oslc.where=...")
```

Under those is `call(conn, method, request)` — one `POST` to
`/sysml.SysMLService/<method>` with a proto3-JSON body — which covers every RPC the service
offers whether or not a named function wraps it, and `call` is what the conformance runner drives.

## Values

`decode_value` reads all nineteen `Value` arms; `encode_value` writes them for `execute_action`
inputs and any hand-built request. `Int64` arrives as a `JSON` *string* on the wire and is parsed
exactly, never through a double; `realValue` may be the strings `"NaN"`, `"Infinity"`,
`"-Infinity"`. The wrapper types are `InstanceRef`, `Quantity`, `EnumLiteral`, `FunctionRef`,
`Metaobject`, `Undetermined`, and the sentinels `Unset` and `Infinity`; sequences become
`Vector`s, sets `Set`s, arrays and tensor quantities carry their `dimensions`.

## Errors

Three exception types: `ConnectError(code, message, http_status)` for a non-200 Connect body,
`TransportError` for a connection failure or a non-JSON body where JSON was required — the
`Content-Type` is checked before the body is parsed, so a proxy's 404 page reports as transport —
and `DiagnosticError` for a call that answered `200` with a non-empty `error` field, carrying the
decoded diagnostics.

## Conformance runner

```bash
julia --project=client/julia/OpenSysML client/julia/OpenSysML/conformance/run.jl --binary bin/sysml-grpc
make conformance-julia
```

Flags: `--binary`/`--address`, `--scenarios`/`--fixtures`, `--run` substring, `--report` file
(`-` is stdout), `--allow-skips`, `-v`. The report is the same JSON every runner writes, and the
exit status is non-zero on any fail or error, or on a skip without `--allow-skips`.
