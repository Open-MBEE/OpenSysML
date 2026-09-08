# Service transports and encodings

This page describes what `sysml-grpc` serves on its port, which body encoding a client should
choose, and the flags that control it. It is written for someone about to write a client. The
measurements and the reasoning behind the choices are in
[the transport evaluation](../internals/design/transport-evaluation.md).

## One port, four ways in

`sysml-grpc` serves, by default, all of these on `-port` (50051):

| Protocol | Content type | Who speaks it |
|----------|--------------|---------------|
| gRPC | `application/grpc` | `grpc-go`, `grpc-java`, `tonic`, `grpcio` (the Python client), `grpcurl` |
| gRPC-Web | `application/grpc-web`, `application/grpc-web+json` | browser clients using `fetch` |
| Connect protocol | `application/proto`, `application/json` | `connect-go`, `@connectrpc/connect-web`, and any HTTP client at all |
| gRPC server reflection | — | `grpcurl`, `grpcui` |

Plus `GET /health`, described below. A Connect unary call is an ordinary POST to
`/sysml.SysMLService/<Method>` whose whole body is the request message, so `curl` is a
first-class client and no generated code is needed to reach the service.

One implementation sits behind all of them: the same fifteen RPCs of
[`api/proto/sysml.proto`](https://github.com/Open-MBEE/OpenSysML/blob/main/api/proto/sysml.proto),
the same semantics, the same status codes. An existing gRPC client (including a generated
`grpc-go` stub, `grpcurl` and the `opensysml` Python client) reaches the default server
unchanged.

`-transport grpc` serves the `grpc-go` server alone, as releases before this one did: no
Connect, no gRPC-Web, no `curl`, health on `-health-port` only. It is an escape hatch, not the
recommended path. `-transport stdio` is a prototype, described below.

`grpc-go` itself is confined to that escape hatch, to tests, to the conformance runner's real
`grpc-go` client, and to committed generated code. The service logic, the public Go API and the
stdio prototype construct errors with `connect-go`, whose codes have the same numbers as the
canonical gRPC status codes, so every transport answers with the same code and message.
`scripts/check-grpc-imports.sh` fails CI if production code imports `grpc-go` again.

The clients this repository ships are described on [client libraries](clients.md): the Python
client speaks gRPC, the Node, Java and Rust clients speak Connect with protobuf bodies, and the
public Go API answers in process without a transport at all.

## Capabilities, and what an absent one does

`GetServerInfo` reports a version string and a list of capability **names**. Negotiate on the
names: a version string tells a client what release answered, not what that release can do for
the call it is about to make.

An absent capability behaves in one of two ways, and which one applies is part of each
capability's definition rather than something a client has to guess:

| The capability describes | A request that needs it | What a client should do |
|---|---|---|
| what the service can be *asked*: `strict_conformance`, `inline_language`, `parse_sources`, `evaluate_subject`, `verification`, `convert`, `apply_edits`, `authoring`, `query`, `oslc_query`, `document_query`, `render_document` | is **refused** with `UNIMPLEMENTED`, naming the capability | check the advertised list first, and report the missing capability locally rather than spending a round trip |
| how a response is *populated*: `type_facts`, `symbol_attributes`, `feature_values`, `enum_values`, `unset_value`, `complex_values`, `structured_values`, `measurement_refs`, `function_values`, `verification_verdicts`, `case_evaluations`, `infinity_value`, `diagnostic_codes` | is answered with those fields **omitted** | check before reading the fields; an omitted field is not an error |

`complex_values`, `structured_values`, `measurement_refs`, `function_values` and `infinity_value` sit in both
rows: a complex — or an array, vector or vector quantity, a bare measurement reference, a calc held as a
value, or the unbounded value `*` — in a response is
reported as an `unsupported` null without it, and one in an action input or calc argument is
refused with `UNIMPLEMENTED` rather than read as another value — a service that predates the
arm would read it as an unknown field, so every client checks the list before sending one
(each covers its value nested inside a sequence or array as well as one at the top level;
`measurement_refs` is its own capability rather than part of `structured_values`, so a client
built against the three structured arms keeps reading a bare reference as the unsupported null
it read before). In the other direction, a client built
before an arm existed parses a newer service's answer as an unknown field — a `Value` with no
kind set — and no client this repository ships reads that as a plain null or crashes: Python
raises `UnsupportedValueError`, Rust returns `Error::Decode`, Go reads an `unsupported` null
naming the case, Java reports a top-level result as `Optional.empty()` and refuses one nested
in a sequence, feature or array with `TransportException`, and Node reports the `absent`
kind, which `encodeValue` refuses to send back (`MalformedValueError`) and whose vector
components are refused when read.

So a client cannot treat "the call succeeded" as "the field was computed", and cannot treat
"no refusal" as "the capability is there". Every client this repository ships checks the list
before making a capability-gated call, and maps the service's refusal onto the same error it
raises itself: `MissingCapabilityError` in Python, `CapabilityException` in Java,
`CodeUnimplemented` in Go. The conformance suite runs against the default service and against
a test-only configuration with capabilities withheld, so both columns of that table are
exercised, not just described.

## Choose protobuf, not JSON

**Protobuf is the body encoding for every client shipped or documented by this project. JSON is
the debugging affordance.** This is not a style preference; it is the strongest measurement in the
evaluation:

| 468 KB `Query` response | p50 | p95 | p99 |
|---|---|---|---|
| protobuf body | 6.34 ms | 9.18 ms | 9.88 ms |
| JSON body | 37.88 ms | 41.82 ms | 44.99 ms |

Six times slower, reproducible across runs. The cause is **not** payload size: the same
answer is 467,971 bytes as protobuf and 513,339 as JSON, only 9.7% more. The cost is `protojson`
encode plus `json_format` decode CPU time, so a faster link does not help and the cost falls on
both ends. Small answers show no measurable difference between the encodings, which is why the
warning is about large ones.

The server also says so at runtime: a JSON-encoded response whose message exceeds 256 KiB logs
a warning naming the procedure and the size. The check uses `proto.Size` for the threshold, so
it costs no second encoding; the number logged is the protobuf size of the answer, a few percent
under what the JSON body will weigh. There is no cheaper JSON path to switch to: `connect-go`
marshals such a response once, not twice, so there is no double marshal to remove, and
`protojson` has no streaming encoder to substitute. The available mitigation is the choice of
encoding, and that belongs to the client.

A browser client written by hand against `application/json` has no protobuf option for its
*body*. `@connectrpc/connect-web`, however, speaks `application/proto` in the browser, and
should be generated rather than hand-written for exactly this reason. `Query` over the whole
model is the call where this decides whether a UI feels responsive.

## Two things a hand-written JSON client must know

```console
$ curl -X POST http://localhost:50051/sysml.SysMLService/ParseFile \
    -H "Content-Type: application/json" \
    -d '{"content":"package Demo { part def Rover { attribute mass = 12.5; } }"}'
{"modelHash":"6245ef48…e78d","root":{"kind":"RootNamespace","childIds":["Demo"]}}

$ curl -X POST http://localhost:50051/sysml.SysMLService/Evaluate \
    -H "Content-Type: application/json" \
    -d '{"expression":"1 + 2 * 3","modelHash":"6245ef48…e78d"}'
{"result":{"intValue":"7"}}
```

1. **An `int64` is a JSON string.** `"7"`, not `7` — the proto3 JSON mapping, not a quirk of
   this service. `intValue`, `modelHash` lengths, step counts and every other 64-bit field
   read this way.
2. **Errors are an HTTP status plus a body**, not gRPC trailers:
   `{"code":"not_found","message":"…"}`. The code names are the Connect protocol's spelling of
   the same gRPC codes a gRPC client sees, so `not_found` here and `NOT_FOUND` there are one
   status. Every client in every protocol must map them identically; the conformance suite
   asserts that it does.

Those two are the minimum. The rest of what a hand-written JSON client has to decode — every
arm of `Value`, the three ways to have no value, the diagnostics and verdict shapes, the code
table, and how long a `modelHash` stays valid — is on
[the wire contract](wire-contract.md), with every example captured from a running service.

## A browser client

Three prerequisites, in the order you will run into them:

**CORS.** `-cors-allowed-origins` takes a comma-separated list of exact origins
(`https://studio.example.org,http://localhost:5173`) and is off when empty. A `*` entry is
**refused at startup**: a service that answers every origin is not a default worth having.
The allowed set drives the preflight response, and the response exposes gRPC-Web's trailer
headers (`Grpc-Status`, `Grpc-Message`, `Grpc-Status-Details-Bin`); a gRPC-Web client whose
trailers are not exposed fails in a way that looks like a server bug. CORS is a browser-side
control, not authentication: a non-browser client is unaffected by the list, and this
service still has no authentication of any kind.

**TLS.** `-tls-cert` and `-tls-key` (both or neither) serve everything above over HTTPS on the
same port, negotiating `h2` and `http/1.1`, minimum TLS 1.2. A browser on an `https://` page
cannot post to `http://`, so this is a prerequisite rather than a hardening step. Without the
flags the port is cleartext with `h2c`, which is what a gRPC client needs against a port that
offers no TLS, and which is appropriate only inside a trusted network or behind a proxy that
terminates TLS. Bidirectional streaming, if it is ever added, would additionally require
HTTP/2 end to end, which in a browser means TLS.

**The `grpc-web-text` gap.** `connect-go` v1.20 implements
`application/grpc-web` and `application/grpc-web+json` but not the base64 `grpc-web-text`
variant; posting that content type answers `415`, and a test pins that. `grpc-web-text` exists
for clients that cannot read a binary response body (the old `XMLHttpRequest` paths). Any
client using `fetch`, which is what `@connectrpc/connect-web` and `grpc-web`'s fetch transport
do, never asks for it. **This is fine for a `fetch`-based browser client and only for
one**: a client that needs `grpc-web-text` needs a proxy in front of this service.

## Health

`GET /health` answers on the main port and reports the build:

```console
$ curl -s http://localhost:50051/health
{"service":"sysml-grpc","status":"ok","version":"0.2.1"}
```

A separate HTTP health port existed because the gRPC-only server could not serve a plain GET.
It no longer has to, so `-health-port` is **deprecated**:

| | Behavior |
|---|---|
| Today, default (`-health-port 8081`) | `/health` answers on both the main port and 8081; the second listener logs a deprecation warning |
| Today, `-health-port 0` | no second listener; `/health` answers on the main port |
| A future release | the default becomes `0`; the flag stays accepted for a release after that |
| `-transport grpc` | unchanged — 8081 is the only health surface, and no warning is logged |

Poll the main port. Nothing in this repository polls 8081: the Python client's readiness probe
is a `GetDiagnostics` call over gRPC, not an HTTP GET, so it is unaffected by every row of that
table.

## `-transport stdio`, and why not to build on it

`-transport stdio` serves one client over stdin/stdout with `Content-Length` framing. It is
**not the default, and not a supported client transport**: no client this project publishes
(not the Python client, not a generated stub) speaks it, and none will. It exists because a
supervisor that already spawns the binary as a child process could reach the service without a
port, which is the question [the evaluation](../internals/design/transport-evaluation.md) asked;
its answer was to serve clients over a port. Choosing stdio also gives up everything else on this
page: no reflection, no `/health`, no CORS, no TLS, one client per process. Write a client against
the default port instead. The prototype is kept behind the flag and tested (`internal/stdiorpc`
covers the protocol and `cmd/sysml-grpc` covers the binary answering a framed call) so that it
cannot rot unnoticed while it is still in the tree.

## Every protocol is tested, not merely served

A second protocol surface that no test exercises will rot. The conformance suite
([`conformance/`](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance)) runs its whole
scenario list once per protocol — gRPC, Connect with a protobuf body, Connect with a JSON body
— against one service, asserting identical results *and* identical status codes:

```console
$ make conformance                                    # all three protocols
$ go run ./cmd/conformance -protocols connect-json    # one of them
$ go run ./cmd/conformance -transport grpc -protocols grpc
```

The JSON-specific edge cases above (`int64` as a string, the error shape) are exactly what that
parameterization covers, in the encoding a browser client will use.
