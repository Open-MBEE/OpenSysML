# OpenSysML MATLAB client

A thin client for the `sysml-grpc` service: JSON over HTTP against the
Connect-JSON surface — no protobuf, no generated code, no binary download —
runnable in **MATLAB R2019b+** and in **GNU Octave 7+**.

## Requirements

- MATLAB R2019b or newer, or GNU Octave 7.0 or newer.
- A `sysml-grpc` binary or a running service. The client never downloads
  one; see `opensysml.private` below.
- On MATLAB, HTTP goes through `matlab.net.http`; on Octave — which has no
  such API — it goes through a `curl` subprocess. MATLAB's HTTP path is
  written to the documented API but exercised by no CI, so Octave + curl is
  the tested path.
- `opensysml.private` spawns the service through Java's
  `ProcessBuilder`. An Octave built without Java (the snap and the CI
  build) cannot spawn a private child — start `sysml-grpc` yourself and
  use `opensysml.external` (or set `OPENSYSML_SERVICE`).

## Install

Put `client/matlab/` on the path:

```matlab
addpath('client/matlab')
```

Then `opensysml.connect()` resolves the service:

```matlab
conn = opensysml.connect();                       % $OPENSYSML_SERVICE, else a private child
conn = opensysml.external('localhost:50051');     % a service you started; close() leaves it running
conn = opensysml.private('binary', '/path/to/sysml-grpc'); % spawn; close() stops it
```

`opensysml.private` resolves the binary in this order:
`$OPENSYSML_GRPC_BINARY`, `~/.opensysml/bin/sysml-grpc`, then `PATH`. The
child is started with `-port 0 -health-port 0 -report-address
-exit-with-parent`; its first stdout line is the address, and the
connection keeps the child's stdin open for the orphan guarantee.

## The call surface

```matlab
model = opensysml.parseFile(conn, 'model.sysml', 'language', 'sysml', 'strict', true);
model = opensysml.parseSource(conn, 'package P { part def B; }', 'name', 'p.sysml');
model = opensysml.parseSources(conn, {{'a.sysml', src_a}, {'b.sysml', src_b}});

diags = opensysml.diagnostics(model);             % cell of severity/message/file/line/col/code
sym   = opensysml.symbol(model, 'P::B');          % plain decoded record
v     = opensysml.evaluate(model, '2 + 2', 'context', 'P');
inst  = opensysml.instantiate(model, 'P::B');     % struct('id', int64, 'type_symbol_id', ..., 'feature_values', containers.Map)
ans   = opensysml.executeAction(model, 'P::act', 'inputs', struct('x', int64(1)), 'schedule', '');
ans   = opensysml.executeState(model, 'P::sm', 'events', {'go'}, 'schedule', '');
rows  = opensysml.query(model, 'oslc.where=...');
```

Lower-level calls:

```matlab
answer = opensysml.call(conn, 'GetServerInfo', struct());   % decoded JSON; raises on error
[status, contentType, body] = opensysml.callRaw(conn, 'GetServerInfo', '{}'); % raw wire
```

## Values

`opensysml.decodeValue` reads all nineteen `Value` arms; `opensysml.encodeValue`
writes request values.

| Wire arm | MATLAB/Octave value |
| --- | --- |
| `intValue` | `int64` (exact 64-bit; digits accumulate in `int64`, never through a double — `-9223372036854775808` survives) |
| `realValue` | `double` (`"NaN"`, `"Infinity"`, `"-Infinity"` strings) |
| `boolValue` | `logical` |
| `stringValue` | `char` |
| `instanceId` | `struct('instanceRef', int64)` |
| `sequence` | cell array |
| `null` | `[]` (a non-empty `null` payload is an error) |
| `unset` | `struct('unset', true)` — cannot be sent |
| `quantity` / `vectorQuantity` / `tensorQuantity` | `struct('magnitude', ..., 'unit', ..., 'unitTerm', ...)` / `struct('components', {…})` |
| `enumLiteral` | `struct('literalId', ..., 'enumerationId', ..., 'name', ..., 'value', ...)` |
| `complex` | `complex(re, im)` |
| `array` | `struct('dimensions', int64 vector, 'elements', cell)` |
| `vector` | `struct('components', {numeric})` |
| `measurementRef` | `struct('unit', ..., 'unitId', ..., 'unitTerm', ...)` |
| `infinity` | `struct('infinity', true)` |
| `function` | `struct('calcId', ..., 'self', instanceRef-or-[])` — a `self` cannot be sent back |
| `set` | `struct('set', {elements})` |
| `metaobject` | `struct('elementId', ..., 'metaclassId', ...)` |
| `undetermined` | `struct('reason', ..., 'lower', ..., 'upper', ...)` — cannot be sent |

JSON pitfalls the client is written around: `jsonencode` writes a 1x1
struct array as an object, so request lists are cell arrays (`{}` is the
empty list); `jsondecode` hands back struct arrays, cells, numeric vectors
and `[]` — `decodeValue` accepts all of them.

## Errors

Errors are `MException`s with `opensysml:*` identifiers:

| Identifier | Raised when |
| --- | --- |
| `opensysml:transport` | connection refused, timeout, non-JSON body where JSON was required |
| `opensysml:connect` | non-200 Connect error body (`{"code","message"}`) — HTTP status in the message |
| `opensysml:diagnostics` | a model-level operation answered with a non-empty `error` field |
| `opensysml:decode` | a `Value` arm violated its contract |
| `opensysml:encode` | a value cannot be written to the wire |
| `opensysml:unknownArm` | a `Value` arm this client doesn't know |

`Content-Type` is checked before the body is read, so the HTML or plain
text a proxy emits for a 404/405/415 reports as `opensysml:transport`, not
a decode error.

## Conformance runner

`conformance/run_conformance.m` drives every scenario of
`conformance/scenarios/` against a service through `callRaw` — the same
report JSON the Rust runner writes.

```sh
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); run_conformance('--binary','bin/sysml-grpc')"
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); run_conformance('--address','localhost:50051','--report','-')"
```

Flags: `--binary`/`--address`, `--scenarios`/`--fixtures` (default the
repo's `conformance/`), `--run SUBSTRING`, `--report FILE` (`-` is stdout),
`--allow-skips`, `-v`. Exit status is non-zero on any fail or error, or on
a skip without `--allow-skips`.

## Tests

```sh
cd client/matlab
octave --no-gui --eval "run_tests"      # from tests/, or: octave --eval "run('tests/run_tests.m')"
```

The decode/encode/rule groups run anywhere; the live group reads
`$OPENSYSML_SERVICE` and skips when unset.

## Scope

Thin means thin: no model-building API, no streaming calls, no
authentication hooks. `parse*`/`evaluate`/`instantiate`/`execute*`/`query`
plus `call`/`callRaw` cover the whole RPC surface.
