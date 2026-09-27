# The MATLAB client API

This page covers what the `+opensysml` package exposes. To choose between the clients, see
[client libraries](clients.md); for a task-oriented walkthrough, see
[guide chapter 9](../guide/09-clients.md#from-matlab). The package's own notes are in
[client/matlab/README.md](../../client/matlab/README.md).

The client is a thin JSON-over-HTTP client of the Connect-JSON surface — no protobuf, no
generated code, and it never downloads a binary. It runs on **MATLAB R2019b+** and on **GNU
Octave 7+**. On MATLAB the HTTP transport is `matlab.net.http`; on Octave, which has no such
API, it is a `curl` subprocess. MATLAB's HTTP path is written to the documented API but
exercised by no CI — Octave plus curl is the tested path.

```matlab
addpath('client/matlab')
```

## Connection

```matlab
conn = opensysml.connect();                       % $OPENSYSML_SERVICE, else a private child
conn = opensysml.external('localhost:50051');     % a service someone else runs
conn = opensysml.private('binary', '/path/to/sysml-grpc');
```

`opensysml.private` resolves the binary in the order `$OPENSYSML_GRPC_BINARY`,
`~/.opensysml/bin/sysml-grpc`, `PATH`, and spawns the child with `-port 0 -health-port 0
-report-address -exit-with-parent` through Java's `ProcessBuilder`. An Octave built without
Java cannot spawn a private child — the error says so; start `sysml-grpc` yourself and use
`external` (or `OPENSYSML_SERVICE`). The client holds the child's stdin open for the orphan
guarantee; `conn.close()` closes that pipe and the service exits. `conn.serverInfo()` asks
`GetServerInfo` once and caches it; `conn.hasCapability(name)` reads the negotiated names.

## Models and calls

```matlab
model = opensysml.parseFile(conn, 'model.sysml', 'language', 'sysml', 'strict', true);
model = opensysml.parseSource(conn, 'package P { part def B; }', 'name', 'p.sysml');
model = opensysml.parseSources(conn, {{'a.sysml', src_a}, {'b.sysml', src_b}});

diags = opensysml.diagnostics(model);             % cell of severity/message/file/line/col/code
sym   = opensysml.symbol(model, 'P::B');          % plain decoded record
v     = opensysml.evaluate(model, '2 + 2', 'context', 'P', 'subject', 'P::obj');
inst  = opensysml.instantiate(model, 'P::B');     % struct('id', int64, 'type_symbol_id', ..., 'feature_values', containers.Map)
ans   = opensysml.executeAction(model, 'P::act', 'inputs', struct('x', int64(1)), 'schedule', '');
ans   = opensysml.executeState(model, 'P::sm', 'events', {'go'}, 'schedule', '');
rows  = opensysml.query(model, 'oslc.where=...');
```

Under those is `opensysml.call(conn, 'Method', request)` — one `POST` to
`/sysml.SysMLService/<method>` with a proto3-JSON body — which covers every RPC, and
`opensysml.callRaw(conn, 'Method', requestJsonText)` returns `[status, contentType, bodyText]`
unparsed; `callRaw` is what the conformance runner drives so the scenario's request goes over
the wire exactly as written.

## Values

`opensysml.decodeValue` reads all nineteen `Value` arms; `opensysml.encodeValue` writes request
values. `intValue` arrives as a JSON *string* and accumulates its digits in `int64` — never
through a double — so `-9223372036854775808` survives exactly; `realValue` may be the strings
`"NaN"`, `"Infinity"`, `"-Infinity"`. Wrapper shapes: `struct('instanceRef', int64)`,
`struct('magnitude', ..., 'unit', ..., 'unitTerm', ...)`, `struct('literalId', ...)`,
`struct('calcId', ..., 'self', ...)`, `struct('elementId', 'metaclassId', ...)`,
`struct('unset', true)` and `struct('infinity', true)`; sequences and request lists are **cell
arrays**, because `jsonencode` writes a 1x1 struct array as one object and `{}` is the empty
list.

## Errors

Errors are `MException`s with `opensysml:*` identifiers: `opensysml:transport` for a connection
failure or a non-JSON body where JSON was required, `opensysml:connect` for a non-200 Connect
error body, `opensysml:diagnostics` for a call answered `200` with a non-empty `error` field,
and `opensysml:decode` / `opensysml:encode` / `opensysml:unknownArm` for the `Value` contract.

## Conformance runner

```sh
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); addpath('client/matlab/conformance/private'); run_conformance('--address','localhost:50051')"
make conformance-matlab
```

Flags: `--binary`/`--address`, `--scenarios`/`--fixtures`, `--run` substring, `--report` file
(`-` is stdout), `--allow-skips`, `-v`. `--binary` needs Java for the private child; Octave
without Java uses `--address`. The report is the same JSON every runner writes, and the exit
status is non-zero on any fail or error, or on a skip without `--allow-skips`.
