# Wire contract for hand-written JSON clients

What a client that speaks Connect + JSON to `sysml-grpc` **without a generated library** has
to know to decode every answer correctly: MATLAB's `webwrite`, R's `httr2`, Julia's `HTTP.jl`,
C with `libcurl`, or `curl` in a shell script. It picks up where
[service transports](service-transports.md#two-things-a-hand-written-json-client-must-know)
stops. That page explains the four protocols on one port and why a generated protobuf client
is the better choice when one exists; this page assumes JSON is the choice you have, and
states, field by field, what the bytes mean.

Every request and response below was captured from a `sysml-grpc` built from this repository
(`go build -o bin/sysml-grpc ./cmd/sysml-grpc`, started with `-port 50099`, its default
transport) and is pasted as the service wrote it, with one exception: responses too long
for a line are re-indented, and an omitted part is marked `…` and named in the text. Request
bodies that are one of the conformance fixtures under
[`conformance/fixtures/`](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance/fixtures)
name the fixture instead of repeating it.

```console
$ curl -s -X POST http://localhost:50099/sysml.SysMLService/<Method> \
    -H 'Content-Type: application/json' -d '<request>'
```

The Python client's decoding (`clients/python/opensysml/values.py`, `errors.py`) is the
reference for what follows; where this page says a client *must* do something, that is what
the Python client does, stated so that it can be reproduced in a language that has no
client.

## The request line, and what an HTTP client sees before the body

One URL per method: `POST /sysml.SysMLService/<Method>`, where `<Method>` is the RPC's name as
spelled in `api/proto/sysml.proto` (`ParseSources`, `Evaluate`, `Instantiate`, …), with
`Content-Type: application/json`. Three things go wrong before the service reads a body, and
none of them answers a JSON error:

| Request | Status | Body |
|---|---|---|
| Unknown method (`/sysml.SysMLService/NoSuchMethod`) | `404 Not Found` | `404 page not found` (plain text) |
| `GET` instead of `POST` | `405 Method Not Allowed` | empty, with `Allow: POST` |
| No `Content-Type`, or one not served | `415 Unsupported Media Type` | empty, with `Accept-Post: …, application/json, …` |

So a client checks the response's `Content-Type` is `application/json` **before** parsing.
Every answer the service itself produces, success or failure, is JSON.

### Field names and JSON types

Field names are the proto3 JSON mapping of the proto names: `model_hash` on the wire is
`modelHash`, `strict_conformance` is `strictConformance`, `states_visited` is `statesVisited`.
The proto file is the authority; take a name from it and lowerCamelCase it. Types follow the
same mapping, and three of its rules matter here:

1. **`int64` is a JSON string.** `{"intValue":"4"}`, `{"id":"1"}`, `{"instanceId":"3"}`. On
   input the service accepts either spelling — `{"intValue":10}` and `{"intValue":"10"}` produce
   the same call — but it always *writes* a string, so a decoder that reads `intValue` as a
   number is wrong on output even when its requests work.
2. **A field at its default value is omitted.** `holds:false`, `error:""`, `materialized:false`,
   an empty `diagnostics` list, a zero `real` inside `complex` — none of these appear. Absent
   means default, and the decoder must supply it. The exception is a `oneof` arm, which is
   written even at its default (`{"intValue":"0"}`, `{"boolValue":false}`, `{"realValue":0}`,
   `{"stringValue":""}`) because the arm's presence is the information.
3. **Unknown request fields are silently dropped.** A misspelled field is not an error:
   `{"modelHash":"…","expresion":"1 + 1"}` is the same call as one with no expression, and the
   answer is the *empty-expression* answer (below), not a complaint about `expresion`. Check your
   spelling against the proto; the service will not.

```console
$ … /Evaluate -d '{"modelHash":"2af52c50cee63699ece8f9021b6344e4fe9f2fe6eeb0f3f8edd9feaa5443dea2","expresion":"1 + 1"}'
{"error":"expression parse failed","diagnostics":[{"severity":"error","message":"expected an expression","span":{"file":"<expression>","startLine":1,"startCol":1,"endLine":1,"endCol":1}}]}
```

A body that is not valid JSON, by contrast, is refused with a Connect error (see
[Three places a failure can be](#three-places-a-failure-can-be)):

```console
$ … /Evaluate -d '{"modelHash": nope}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"unmarshal message: unmarshal into *proto.EvaluateRequest: proto: syntax error (line 1:15): invalid value nope"}
```

## The session: `ParseSources` and the model hash

There is no session object. A client parses a model once, receives a **model hash**, and
passes that hash to every later call. `ParseSources` takes `documents` (not `sources`), each
with either `content` plus a `name` and an optional `language` (`sysml`, the default, or
`kerml`), or a `filePath` the *service's* process can read (its language follows the file
extension); and an optional `strictConformance` flag.

```console
$ … /ParseSources -d '{"documents":[{"name":"vehicle.sysml","content":"…"}]}'
{"modelHash":"2af52c50cee63699ece8f9021b6344e4fe9f2fe6eeb0f3f8edd9feaa5443dea2","roots":[{"kind":"RootNamespace","childIds":["Demo"]}]}
```

`content` above is `conformance/fixtures/vehicle.sysml` as one JSON string. Several documents
form one model, in which imports between them resolve and a diagnostic names the document it
came from:

```console
$ … /ParseSources -d '{"documents":[{"name":"behavior.sysml","content":"…"},{"name":"verification.sysml","content":"…"}]}'
{"modelHash":"b4e096aa76331818a290956ac449f6391924767796eeea816b3adf103f5cded9","roots":[{"kind":"RootNamespace","childIds":["Test"]},{"kind":"RootNamespace","childIds":["Demo"]}]}
```

The response has three fields a client reads:

- `modelHash` — the handle for every later call. Present even when the model has errors.
- `roots` — one `SymbolInfo` per document, in request order; `childIds` are the fully
  qualified names of its top-level members. Every `id` in this API is a fully qualified name
  (`Demo::Vehicle::mass`), and that is what `symbolId`, `contextSymbolId`, `elementId` and
  friends take.
- `diagnostics` — the parse and validation findings, absent when there are none. Their shape is
  in [Diagnostics](#diagnostics-the-shape-of-a-finding).

A syntax error is **not** a failed call. The status is 200, the model is cached, and the hash
is usable for what did parse, but a client that treats a hash as "the model is good" must
check `diagnostics` for `"severity":"error"` first:

```console
$ … /ParseSources -d '{"documents":[{"name":"syntax_error.sysml","content":"package Test { invalid syntax ((( }\n"}]}'
{"modelHash":"da0e2628154910330555183af59d8f803233352122b69f64da8ff138094f0c50","roots":[{"kind":"RootNamespace","childIds":["Test"]}],"diagnostics":[{"severity":"error","message":"expected a namespace member","span":{"file":"syntax_error.sysml","startLine":1,"startCol":16,"endLine":1,"endCol":23}}]}
```

What *is* refused, with a Connect error, is a request the service cannot make a model from at
all:

```console
$ … /ParseSources -d '{"documents":[]}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"documents must name at least one document"}

$ … /ParseSources -d '{"documents":[{"name":"a.sysml","content":"package A {}"},{"name":"a.sysml","content":"package B {}"}]}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"documents 0 and 1 are both named \"a.sysml\": each document of a model needs its own name"}
```

### What a model hash is

A model hash is the lowercase hex SHA-256 (64 characters) of the request that produced it: the
conformance mode (`default` or `strict`), the number of documents, and each document's name,
language and content, length-delimited (`internal/grpc/service.go`, `parseSources`). It is
**deterministic**: the same documents in the same order with the same flag give the same hash
from any service of the same version, so a client may compute nothing and simply compare
hashes to know whether two models are the same text. It is also *only* a hash of the
text — the same `vehicle.sysml` parsed with `strictConformance:true` is a different model with
a different hash, because the mode is part of the key:

```console
$ … /ParseSources -d '{"documents":[{"name":"vehicle.sysml","content":"…"}],"strictConformance":true}'
{"modelHash":"39e81db405bd2c37d99f7cb6097b4ec6736737090a025a50d8d3f6c500ee955b","roots":[{"kind":"RootNamespace","childIds":["Demo"]}]}
```

`strictConformance` makes OpenSysML's extension notation a parse error rather than an accepted
extension; what that covers is under [Strict conformance](../guide/03-command-line.md#strict-conformance)
in the guide.

### How long a hash is valid

The service keeps parsed models in an in-memory **LRU cache of fixed capacity**
(`internal/grpc/cache.go`), sized by the `-cache-size` flag, **default 100**. There is no
time-to-live: a model stays until it is one of the least recently *used* when the cache is full
and a new model arrives, or until the process exits. Every call that names a hash counts as a
use, so a model in active use is not evicted. Re-parsing a model the cache still holds returns
the same hash and does not re-parse.

The consequence for a client: a hash is a **cache key, not a durable identifier**. Store the
documents, not the hash; recompute by re-parsing whenever the service says the hash is gone.
Here it is happening, against a service started with `-cache-size 1`:

```console
$ … /ParseSources -d '{"documents":[{"name":"a.sysml","content":"package A { attribute x = 1; }"}]}'
{"modelHash":"f90104e75e9172ab29c8648e3529a103e178756d35b4643d03c8c64419eaabae","roots":[{"kind":"RootNamespace","childIds":["A"]}]}

$ … /Evaluate -d '{"modelHash":"f90104e75e9172ab29c8648e3529a103e178756d35b4643d03c8c64419eaabae","expression":"A::x"}'
{"result":{"intValue":"1"}}

$ … /ParseSources -d '{"documents":[{"name":"b.sysml","content":"package B { attribute y = 2; }"}]}'
{"modelHash":"ca693643bf449df4d2904900963596195409a34bbc51f243964952077ab99eb3","roots":[{"kind":"RootNamespace","childIds":["B"]}]}

$ … /Evaluate -d '{"modelHash":"f90104e75e9172ab29c8648e3529a103e178756d35b4643d03c8c64419eaabae","expression":"A::x"}'
HTTP/1.1 404 Not Found
{"code":"not_found","message":"model not found: f90104e75e9172ab29c8648e3529a103e178756d35b4643d03c8c64419eaabae"}

$ … /ParseSources -d '{"documents":[{"name":"a.sysml","content":"package A { attribute x = 1; }"}]}'
{"modelHash":"f90104e75e9172ab29c8648e3529a103e178756d35b4643d03c8c64419eaabae","roots":[{"kind":"RootNamespace","childIds":["A"]}]}
```

A **stale or unknown hash** is therefore always HTTP 404 with `"code":"not_found"`, on every
method that takes one. The message is `model not found: <hash>` everywhere except `ApplyEdits`
and `Convert`, which say `model <hash> is no longer cached: parse it again …`. The Python
client turns the `model not found:` message into `ModelNotFoundError`; the right recovery is to
re-parse and retry, and since the hash is deterministic the retry can reuse the stored hash.
Note that `not_found` is also the status for an unknown *symbol* on some methods
(`RunDocumentQuery`, below) and for a `filePath` the service cannot read — the message says
which (`model not found:`, `symbol not found:`, `file not found:`), and a client that recovers
by re-parsing must read it.

## `Value`: fifteen arms, exactly one present

Every value the engine returns — an expression result, a feature of an instance, an action
output, a state-machine context variable — is a `Value`, which is a proto `oneof` of fifteen
arms. In JSON that is **an object with exactly one key**, and the key is the discriminator.
A decoder therefore does not look for a `kind` field: it looks at which key is present. The
arms, each captured from `Evaluate` against the model at the end of this section:

| Key | JSON type | Captured | Meaning |
|---|---|---|---|
| `intValue` | string | `{"result":{"intValue":"4"}}` | Integer (64-bit); string because `int64` |
| `realValue` | number | `{"result":{"realValue":0.3333333333333333}}` | Real (IEEE-754 double) |
| `boolValue` | boolean | `{"result":{"boolValue":true}}` | Boolean |
| `stringValue` | string | `{"result":{"stringValue":"abc"}}` | String |
| `instanceId` | string | `{"result":{"instanceId":"2"}}` | A reference to a runtime instance, by id |
| `sequence` | object | `{"result":{"sequence":{"elements":[{"stringValue":"nav"},{"stringValue":"sci"}]}}}` | Ordered collection; `elements` are `Value`s |
| `null` | string | `{"result":{"null":""}}` | The SysML `null`, or an unsupported value (non-empty string) |
| `quantity` | object | `{"result":{"quantity":{"realMagnitude":5.4,"unit":"SI::km/SI::h","unitTerm":{…}}}}` | Magnitude with a unit |
| `enumLiteral` | object | `{"result":{"enumLiteral":{"literalId":"Rover::Mode::idle","enumerationId":"Rover::Mode","name":"Mode::idle"}}}` | Enumeration literal |
| `unset` | boolean | `{"result":{"unset":true}}` | A feature that exists and has no value |
| `complex` | object | `{"result":{"complex":{"real":1.5,"imaginary":-2}}}` | Complex number |
| `array` | object | `{"result":{"array":{"dimensions":["2","3"],"elements":[{"intValue":"1"},…,{"intValue":"6"}]}}}` | Multi-dimensional array; `elements` are `Value`s in row-major order |
| `vector` | object | `{"result":{"vector":{"components":[{"realValue":3},{"realValue":4}]}}}` | Numeric vector; each component an `intValue` or `realValue` |
| `vectorQuantity` | object | `{"result":{"vectorQuantity":{"components":[{"realMagnitude":3,"unit":"m","unitTerm":{…}},…]}}}` | Vector of quantities; one `quantity` body per component |
| `measurementRef` | object | `{"result":{"measurementRef":{"unit":"m","unitTerm":{…},"unitId":"SI::metre"}}}` | A measurement reference on its own: a unit, its reduction, and the declaration it names |

The `array`, `vector` and `vectorQuantity` rows were captured against
`conformance/fixtures/structured.sysml` (`S::grid`, `S::v`, `S::d`), `measurementRef` against
`conformance/fixtures/measurement_ref.sysml` (`M::u`); the rest against the model below, with requests of the form
`{"modelHash":"59c4…a654","expression":"<expr>","contextSymbolId":"Rover"}` with `rover.count`,
`1.0 / 3.0`, `rover.armed`, `"abc"`, `rover.wheel`, `rover.tags`, `null`, `rover.speed`,
`Mode::idle`, `rover.serial` and `rover.z`, and the model was:

```sysml
package Rover {
	private import ScalarValues::*;
	private import SI::*;
	private import ComplexFunctions::*;

	enum def Mode { idle; driving; }

	part def Wheel {
		attribute radius : Real = 0.25;
	}

	part def Vehicle {
		attribute count : Integer = 4;
		attribute mass : Real = 12.5;
		attribute armed : Boolean = true;
		attribute callsign : String = "R-1";
		attribute speed : ISQ::SpeedValue = 5.4 [SI::km/SI::h];
		attribute mode : Mode = Mode::driving;
		attribute z : Complex = rect(1.5, -2.0);
		attribute serial : Integer;
		attribute tags : String[2] = ("nav", "sci");
		part wheel : Wheel;
	}

	part rover : Vehicle {
		attribute :>> mass = 20.0;
	}
}
```

### The decoding rule

```text
decode(v):
  if v is absent                → no value was produced (see "result vs unset vs null")
  key := the single key of v
  intValue     → parse the string as a 64-bit integer; never as a double
  realValue    → the number, as a double
  boolValue    → the boolean
  stringValue  → the string
  instanceId   → an opaque reference; parse as 64-bit integer, keep it a reference
  sequence     → map decode over v.sequence.elements (absent elements = empty list)
  null         → if v.null == "" then the language's null, else an error naming v.null
  unset        → the language's "unset" sentinel, distinct from null and from false
  quantity     → see below
  enumLiteral  → identity is literalId; enumerationId is its type; name is for display
  complex      → complex(v.complex.real or 0, v.complex.imaginary or 0)
  array        → shape v.array.dimensions (parse each as int64); elements := map decode over
                 v.array.elements; require len(elements) == product(dimensions), else an error
  vector       → map over v.vector.components: intValue → integer, realValue → double,
                 anything else → an error
  vectorQuantity → map the quantity rule over v.vectorQuantity.components; empty → an error
  measurementRef → unit := v.measurementRef.unit, id := v.measurementRef.unitId;
                 require v.measurementRef.unitTerm when either is present, else an error;
                 require unit or id, else an error; id absent means a composed unit
  anything else → an error: a newer service than this decoder
```

The arms in detail, where a detail exists:

**`intValue`.** A string holding a decimal integer, possibly negative, up to the full `int64`
range. Parse it to an integer type of at least 64 bits. **Do not** convert it to a double:
above 2^53 that loses digits silently, and the engine's Integer is exact —

```console
$ … /Evaluate -d '{"modelHash":"59c471bcbfb8ea2aec1997d62334d7144bcc165e56eb1c79058dcf5ca378a654","expression":"9007199254740993"}'
{"result":{"intValue":"9007199254740993"}}
```

A double would read that as `9007199254740992`. MATLAB's `jsondecode` gives you a `char`
array, which `int64(str2double(...))` corrupts and `sscanf(s, '%ld')` does not; R needs
`bit64::as.integer64`; Julia's `parse(Int64, s)` and C's `strtoll` are exact.

**`realValue`.** A JSON number. A whole double is written without a fraction — `20.0` arrives as
`20` — so a parser that types by spelling (Julia's `JSON.parse`, R's `jsonlite`, MATLAB's
`jsondecode`) gives an integer type; convert to double unconditionally. **NaN and the two
infinities:** the proto3 JSON mapping spells them as the strings `"NaN"`, `"Infinity"` and
`"-Infinity"`, and that is what this service's serializer (`protojson`) would write if it were
ever handed one. In practice it is not: the engine refuses to produce a non-finite Real and
reports it as an in-body error instead —

```console
$ … /Evaluate -d '{"modelHash":"59c4…a654","expression":"1.0 / 0.0","contextSymbolId":"Rover"}'
{"error":"evaluation failed: division by zero"}

$ … /Evaluate -d '{"modelHash":"59c4…a654","expression":"1.0e308 * 10.0","contextSymbolId":"Rover"}'
{"error":"evaluation failed: arithmetic overflow: result is not a finite Real"}
```

A defensive decoder still accepts a string in `realValue` and maps the three spellings to the
language's NaN/±Inf, so that it never fails on a value the mapping allows; it should not
expect to see one.

**`instanceId`.** The id of an instance the call created. It is meaningful **only within the
response that carries it**: instances are built fresh per call and are not addressable
afterwards. `Instantiate` and the `Verify*` calls return an `instances` table in the same
response that an `instanceId` indexes; `Evaluate` does not, so `rover.wheel` above yields
`{"instanceId":"2"}` and nothing to look `2` up in. Ids are small integers assigned in
creation order and restart at 1 for every call; do not persist them, compare them across
calls, or read them as anything but a key into the sibling `instances` list.

**`sequence`.** `elements` is a list of `Value`s, recursively; an empty sequence has no
`elements` key at all (the default-omission rule). Elements may be of different arms:

```console
$ … /Evaluate -d '{"modelHash":"59c4…a654","expression":"(1, 2.5, \"a\")","contextSymbolId":"Rover"}'
{"result":{"sequence":{"elements":[{"intValue":"1"},{"realValue":2.5},{"stringValue":"a"}]}}}
```

**`null`.** The arm is a string. Empty means the SysML `null` value. **Non-empty** means the
engine held a value it has no wire representation for, and the string says what it was — the
Python client raises `UnsupportedValueError(text)` for that case rather than returning `None`,
and a hand-written client should not silently equate the two either.

**`unset`, and the three ways to have no value.** Three different things look like "nothing"
and a client must keep them apart:

| Shape | Meaning |
|---|---|
| `result` key **absent** from the response | The call produced no value: it failed (`error` is present), or the method has no result for this input |
| `{"result":{"unset":true}}` | The call produced a value, and it is *unset*: the feature exists and nothing has been assigned to it |
| `{"result":{"null":""}}` | The call produced the SysML `null` |

Here is a definition with an attribute that has a type and no value, from
`conformance/fixtures/unset.sysml` (`attribute d : Real;` beside `attribute k : Real = 2.0;`):

```console
$ … /Evaluate -d '{"modelHash":"07bfcf7c99b9bc7176279e47f22e7c6deabae0e012fdae8cc94811e9a73b564f","expression":"d","subjectSymbolId":"P::Q"}'
{"result":{"unset":true}}

$ … /Evaluate -d '{"modelHash":"07bfcf7c99b9bc7176279e47f22e7c6deabae0e012fdae8cc94811e9a73b564f","expression":"d + 1.0","subjectSymbolId":"P::Q"}'
{"error":"evaluation failed: type mismatch: operator '+' is not defined for an instance and a Real"}
```

`unset` is written `true` whenever the arm is present; it is never `false`. Reading it as a
boolean and testing it for truth is therefore a bug waiting for the arm to be absent: the
question is "is the `unset` key present", never "is `unset` true".

**`quantity`.** A magnitude with a unit:

```json
{"quantity":{"realMagnitude":5.4,"unit":"SI::km/SI::h","unitTerm":{"scaleNum":5,"scaleDen":18,"factors":[{"unitId":"SI::metre","exponent":1},{"unitId":"SI::second","exponent":-1}]}}}
```

- The magnitude is its own `oneof`: **`intMagnitude`** (a string, same rule as `intValue`) or
  **`realMagnitude`** (a number). Exactly one is present.
- `unit` is the unit expression as written, by fully qualified name of each unit; it is the
  display form and the identity of the unit *as declared*.
- `unitTerm` is the same unit reduced to base units: `factors` are `(unitId, exponent)` pairs
  and `scaleNum/scaleDen` is the exact rational scale to those base units (here 5/18: one
  km/h is 5/18 m/s). A client converting between units multiplies by this ratio; a client
  comparing two quantities for the same dimension compares `factors`. The scale is a
  *ratio*, not a decimal — do not read `scaleNum` alone as "the scale".

**`enumLiteral`.** Three strings: `literalId`, the fully qualified name of the literal and its
**identity**; `enumerationId`, the fully qualified name of its enumeration; and `name`, a
display label. Compare literals by `literalId`. `name` is what the model author wrote at the
reference site relative to a scope (`Mode::idle` here, but `idle` or `Rover::Mode::idle` from
another scope for the same literal), so two equal literals can carry different `name`s and two
literals of different enumerations can carry the same one.

**`complex`.** `real` and `imaginary`, both doubles, **either omitted when zero**:
`rect(0.0, 2.0)` is `{"result":{"complex":{"imaginary":2}}}`. Read each with a default of 0.

**`array`.** The shape and the elements, flattened:

```console
$ … /Evaluate -d '{"modelHash":"42cc…54b0","expression":"S::grid"}'
{"result":{"array":{"dimensions":["2", "3"], "elements":[{"intValue":"1"}, {"intValue":"2"}, {"intValue":"3"}, {"intValue":"4"}, {"intValue":"5"}, {"intValue":"6"}]}}}
```

- `dimensions` is the rank and extents, `int64` strings like `intValue`; a rank-0 array has no
  `dimensions` key (default omission) and exactly one element.
- `elements` is the array flattened in **row-major** order — the last dimension varies fastest,
  so the element at `(i, j)` of a `(2, 3)` array is `elements[i*3 + j]` — and every element is a
  `Value` of any arm, so an array of quantities, or of arrays, nests without a second encoding.
- The element count is the product of the dimensions; a client must check it (and that every
  extent is positive) before indexing, and reject a message that disagrees. The service applies
  the same rule to an array sent to it.

**`vector`.** `components` is a list of `Value`s each of which is an `intValue` or a
`realValue` — nothing else — so an Integer and a Real component stay distinct, as they do in
a `sequence`. A `vector` is not a `sequence`: `VectorOf((3.0, 4.0))` is one value with a
dimension, and the engine's vector functions accept it where a sequence of numbers would be
read element by element. A component of any other arm is an error, on both sides.

**`vectorQuantity`.** `components` is a list of `quantity` bodies — each with its own
magnitude, `unit` and `unitTerm`, exactly as the `quantity` arm carries them:

```console
$ … /Evaluate -d '{"modelHash":"42cc…54b0","expression":"S::d"}'
{"result":{"vectorQuantity":{"components":[{"realMagnitude":3, "unit":"m", "unitTerm":{"scaleNum":1, "scaleDen":1, "factors":[{"unitId":"SI::metre", "exponent":1}]}}, {"realMagnitude":4, "unit":"m", "unitTerm":{"scaleNum":1, "scaleDen":1, "factors":[{"unitId":"SI::metre", "exponent":1}]}}]}}}
```

The unit is carried per component rather than once, so a vector whose components were
composed in different units arrives as it was computed; a client wanting one unit checks that
every component names the same one. An empty `components` list is an error: a vector quantity
has at least one component. A component sent without its `unitTerm` is refused by the rule
under `quantity`.

**`measurementRef`.** A unit on its own — what `SI::m`, `m / s` or a quantity's `.mRef`
evaluate to — with no magnitude:

```console
$ … /Evaluate -d '{"modelHash":"5b0f…40d5","expression":"M::u"}'
{"result":{"measurementRef":{"unit":"m", "unitTerm":{"scaleNum":1, "scaleDen":1, "factors":[{"unitId":"SI::metre", "exponent":1}]}, "unitId":"SI::metre"}}}

$ … /Evaluate -d '{"modelHash":"5b0f…40d5","expression":"M::speed"}'
{"result":{"measurementRef":{"unit":"m/s", "unitTerm":{"scaleNum":1, "scaleDen":1, "factors":[{"unitId":"SI::metre", "exponent":1}, {"unitId":"SI::second", "exponent":-1}]}}}}
```

- `unit` and `unitTerm` are the `quantity` arm's, under the same rule: a `unit` that names one
  is never sent without its reduction, and a client sending one without it is refused.
- `unitId` is the fully qualified name of the unit *declaration* the reference is — the
  canonical name, `SI::metre` for `m` and for the alias `SI::m` — and is the identity a client
  keeps to send the same reference back. It is **absent for a composed unit** (`m / s` above,
  `km / h`): a unit computed from others names no one declaration, and a client must not
  fabricate one. A reference sent with a `unitId` is resolved against the model's own
  declaration, and refused when the id names nothing, names something that is not a unit,
  or its `unitTerm` disagrees with the declaration's own reduction (see `EvaluateCalc`).
- A `measurementRef` is not a `quantity` with magnitude one: `ConvertQuantity(q, ref)` takes
  one, `q * ref` does not.

### What a client must not do

- **Do not compare enum literals by `name`.** Compare `literalId`.
- **Do not read `intValue` (or `intMagnitude`, `id`, `instanceId`) as a double.** Above 2^53 the
  digits are gone and nothing tells you.
- **Do not read `unset` as a boolean.** Its presence is the fact; a missing `result` is a
  different fact (no value), and `{"null":""}` a third (the null value).
- **Do not treat a non-empty `null` string as null.** It names a value that could not be sent.
- **Do not keep an `instanceId` past the response it arrived in**, or use one to index a
  different response's `instances`.
- **Do not default a missing `realValue` to 0 or a missing `boolValue` to false to "make it
  work".** If the key you expected is not the one present, the value is of another kind, and
  the decoder must say so.
- **Do not read an `array` or a `vector` as a `sequence`.** The first has a shape and the
  second a dimension; flattening either into a list loses what the arm exists to carry.
- **Do not index an `array` before checking `len(elements) == product(dimensions)`.**
- **Do not invent a `unitId` for a `measurementRef` that has none.** A composed unit names no
  declaration; send it back as it came, with its `unit` and `unitTerm` only.

## Three places a failure can be

A call can fail at three levels, and a client's classification starts by telling them apart:

1. **The transport refused the call.** HTTP status is not 200 and the body is
   `{"code":"<connect code>","message":"…"}`. Nothing was done. This is a Connect error.
2. **The service ran the call and the model failed.** HTTP 200; the response has a non-empty
   `error` string (and on some methods a `failureReason` and/or `diagnostics`). The result
   fields are absent. This is an in-body failure.
3. **The call succeeded and reports findings.** HTTP 200, result fields present, plus a
   `diagnostics` list (`ParseSources`, and `Evaluate` when the *expression* did not parse).
   Whether an error-severity diagnostic is a failure is the client's decision, and for a parse it
   almost always is.

The rule of thumb from the service's side: a Connect error means **the request itself was not
acceptable** — malformed, naming a model or capability the service does not have, or asking
for two exclusive things — while a 200 with `error` means the request was fine and **the model
did not do what was asked** — a symbol is missing or of the wrong kind, an expression does not
type-check, an action has no start, a value overflowed.

### Diagnostics: the shape of a finding

```json
{"severity":"error","message":"expected a namespace member","span":{"file":"syntax_error.sysml","startLine":1,"startCol":16,"endLine":1,"endCol":23}}
```

`severity` is `"error"`, `"warning"` or `"info"`. `span` locates it: `file` is the document's
`name` from the request (or `<expression>` for an `Evaluate` expression), lines and columns are
**1-based**, `end*` is exclusive, and the whole `span` is omitted when there is no location.
Line and column are `int32`, so unlike `int64` they are JSON *numbers*.

### In-body failures

`error` is text for a person; do not parse it for control flow beyond the prefix. Where a
client needs to branch, the service gives a field for it:

```console
$ … /EvaluateCalc -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::sedan"}'
{"error":"calc invocation failed: not a calc: Demo::sedan is a part usage, not a calc definition or usage","failureReason":"FAILURE_REASON_WRONG_KIND"}
```

`failureReason` is a proto enum, written as its **name**, not its number. The names are in
`api/proto/sysml.proto` (`FailureReason`); `FAILURE_REASON_WRONG_KIND` means the symbol exists
but is not the kind the method operates on, and the Python client raises `WrongKindError` for
it, a subclass of its general `ExecutionError`. A `failureReason` that is absent with an `error`
present is an unspecified execution failure. The other common in-body failures, all HTTP 200:

```text
{"error":"symbol not found: Demo::Nope"}                                    Instantiate
{"error":"state machine not found: Test::NoMachine"}                        ExecuteState
{"error":"action execution failed: initialize action: invalid action flow: no initial node found in action noStart"}
{"error":"evaluation failed: object has no such feature: member nothing not found in instance"}  Evaluate
{"error":"evaluation failed: unresolved reference: sqrt — did you mean RealFunctions::sqrt or QuantityCalculations::sqrt?"}
```

The Python client maps `symbol not found:` to `SymbolNotFoundError` and every other in-body
`error` to `ModelError` (for `Instantiate`) or `ExecutionError` (for the behavior calls),
keeping the text as the message.

### Connect errors: the code table

The body is always `{"code":"…","message":"…"}`; the HTTP status is a function of the code, and
the code is the one to switch on. The full Connect table, and which of its rows this service
actually produces and for what:

| `code` | HTTP | This service answers it for | Client class (Python name) |
|---|---|---|---|
| `invalid_argument` | 400 | Body is not valid JSON for the request type; `documents` empty or with duplicate names; `query` and `oslcQuery` both present; unknown query property; a document query given no binding for a required parameter, or a `queryId` that is not a document query | `InvalidRequestError` — fix the request |
| `failed_precondition` | 400 | The request is well-formed but the model is not in the state the operation needs: an edit on a multi-document model that must name its document; a document query whose own definition is faulty when planned or run; a document whose own definition is faulty when planned | `InvalidRequestError` |
| `out_of_range` | 400 | Not currently produced; reserved by the protocol for a value outside its valid range | `InvalidRequestError` |
| `not_found` | 404 | `model not found: <hash>` (or `model <hash> is no longer cached: …` on `ApplyEdits`/`Convert`) — stale or unknown model hash, on every method that takes one; `symbol not found: <id>` on `RunDocumentQuery` and `RenderDocument`; `file not found: …` for a `filePath` the service could not read | `ModelNotFoundError` / `SymbolNotFoundError` / `ModelFileNotFoundError`, by message prefix — re-parse, fix the name, fix the path |
| `unimplemented` | 501 | A capability the running service was started without (`capability "query" is unavailable`) or a method it does not have | `UnsupportedOperationError` — do not retry |
| `unavailable` | 503 | Not produced by the service itself; a proxy or a shutting-down process answers it | `ConnectionError` — retry with backoff |
| `deadline_exceeded` | 504 | The client's deadline passed | `ServiceTimeoutError` |
| `canceled` | 499 | The client cancelled | `ServiceTimeoutError` |
| `resource_exhausted` | 429 | A document query (`RunDocumentQuery`, or one a `RenderDocument` runs) exhausted its visit, invocation or invocation-depth budget | `ServiceError` — the query, not the request, is at fault |
| `already_exists` | 409 | Not currently produced | `ServiceError` |
| `aborted` | 409 | Not currently produced | `ServiceError` |
| `permission_denied` | 403 | Not currently produced (the service has no authentication) | `ServiceError` |
| `unauthenticated` | 401 | Not currently produced | `ServiceError` |
| `internal` | 500 | A bug in the service: something that should not have failed did (an edit that could not be applied, a document-query library that could not be loaded, a document whose evaluation failed for a reason other than one of its queries) | `ServiceError` — report it |
| `unknown` | 500 | An error the service could not classify | `ServiceError` |
| `data_loss` | 500 | Not currently produced | `ServiceError` |

Examples of the rows this service produces, each captured:

```console
$ … /Query -d '{"modelHash":"2af5…dea2","query":{},"oslcQuery":"sysml:type=PartUsage"}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"query and oslc_query are mutually exclusive"}

$ … /Query -d '{"modelHash":"2af5…dea2","query":{"where":{"primitive":{"property":"colour","operator":"PRIMITIVE_OPERATOR_EQUAL","value":["red"]}}}}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"unknown query property \"colour\"; queryable properties are @id, @type, declaredName, declaredShortName, documentation, isAbstract, multiplicityLower, multiplicityUpper, name, owner, qualifiedName, shortName, type"}

$ … /ApplyEdits -d '{"modelHash":"b4e0…ded9","operations":[{"setValue":{"target":"Demo::sedan::mass","value":"1300.0"}}]}'
HTTP/1.1 400 Bad Request
{"code":"failed_precondition","message":"this operation is defined on one document, and the model has 2: name the document to operate on by parsing it on its own"}

$ … /RunDocumentQuery -d '{"modelHash":"7e6a…a687","queryId":"Observatory::NoSuchQuery"}'
HTTP/1.1 404 Not Found
{"code":"not_found","message":"symbol not found: Observatory::NoSuchQuery"}
```

A `filePath` that does not exist for the service's process:

```console
$ … /ParseSources -d '{"documents":[{"filePath":"/nonexistent/x.sysml"}]}'
HTTP/1.1 404 Not Found
{"code":"not_found","message":"file not found: open /nonexistent/x.sysml: no such file or directory"}
```

And a method whose capability the service does not have — which in practice means a service
older than the method; a service built from this repository advertises all of them, and the
conformance runner withholds one only through a test-only environment variable, which is how
this was captured with `query` withheld:

```text
HTTP/1.1 501 Not Implemented
{"code":"unimplemented","message":"capability \"query\" is unavailable"}
```

Capabilities, and how a client discovers them up front with `GetServerInfo`, are on
[service transports](service-transports.md#capabilities-and-what-an-absent-one-does).

### Classifying, in order

```text
if HTTP status != 200:
    parse {"code","message"}                (Content-Type must be application/json)
    switch code: not_found → by message prefix; unimplemented → give up on the method;
                 unavailable/deadline_exceeded → retry; invalid_argument/failed_precondition →
                 caller's bug; internal/unknown → report
elif body.error is present and non-empty:
    model/execution failure; failureReason (if present) says which kind
elif body.diagnostics has an entry with severity "error":
    the call ran but the input did not parse/validate; decide per call
else:
    success; read the result fields
```

## Behavior calls

Every behavior call takes a `modelHash` and a fully qualified `symbolId` (spelled
`actionSymbolId` / `stateMachineSymbolId` on the two execution calls), builds a fresh runtime,
runs, and returns the result plus any `error`/`diagnostics`. The examples use
`conformance/fixtures/behavior.sysml` and `verification.sysml`, parsed together as model
`b4e096aa76331818a290956ac449f6391924767796eeea816b3adf103f5cded9`, and the `Rover` model above.

### `Instantiate` and the `Instance` shape

```console
$ … /Instantiate -d '{"modelHash":"59c471bcbfb8ea2aec1997d62334d7144bcc165e56eb1c79058dcf5ca378a654","symbolId":"Rover::rover"}'
```

```json
{
  "instance": {
    "id": "1",
    "typeSymbolId": "Rover::rover",
    "featureValues": {
      "armed":    {"featureName": "armed",    "value": {"boolValue": true},   "materialized": true},
      "callsign": {"featureName": "callsign", "value": {"stringValue": "R-1"}, "materialized": true},
      "count":    {"featureName": "count",    "value": {"intValue": "4"},     "materialized": true},
      "mass":     {"featureName": "mass",     "value": {"realValue": 20},     "materialized": true},
      "mode": {
        "featureName": "mode",
        "value": {"enumLiteral": {"literalId": "Rover::Mode::driving", "enumerationId": "Rover::Mode", "name": "Mode::driving"}},
        "materialized": true
      },
      "serial":   {"featureName": "serial",   "value": {"unset": true},       "materialized": true},
      "speed": {
        "featureName": "speed",
        "value": {"quantity": {"realMagnitude": 5.4, "unit": "SI::km/SI::h", "unitTerm": {"scaleNum": 5, "scaleDen": 18, "factors": [{"unitId": "SI::metre", "exponent": 1}, {"unitId": "SI::second", "exponent": -1}]}}},
        "materialized": true
      },
      "tags":     {"featureName": "tags",     "values": [{"stringValue": "nav"}, {"stringValue": "sci"}], "materialized": true},
      "wheel":    {"featureName": "wheel",    "value": {"instanceId": "3"},   "materialized": true},
      "z":        {"featureName": "z",        "value": {"complex": {"real": 1.5, "imaginary": -2}}, "materialized": true}
    }
  },
  "instances": [ … ]
}
```

(Re-indented; `instances` holds two entries, the object above as `"id":"1"` and the `Wheel` as
`"id":"3"` with `radius` `{"realValue":0.25}`.) The shape:

- `instance` is the root; `instances` is **every** instance the call created, root included,
  each with its `id`. An `instanceId` anywhere in the response is a key into this list. Ids
  are strings (`int64`).
- `typeSymbolId` is the fully qualified name of the definition or usage the instance is of.
- `featureValues` is a map from feature name to `FeatureValue`. The map key and `featureName`
  are the same string. Each `FeatureValue` has one of:
  - `value` — a single `Value`, for a feature of multiplicity at most 1;
  - `values` — a list of `Value`s, for a multi-valued feature (`tags`, multiplicity `[2]`).
    Absent when empty. **Which of the two appears is decided by the feature's multiplicity,
    not by how many values it has**; a decoder must accept either;
  - `error` — the feature could not be evaluated; neither `value` nor `values` is present;
  - neither — the feature is single-valued and **not materialized** (the runtime did not
    compute it; see `materialized`).
- `materialized` is `true` when the runtime computed the feature's value for this instance.
  It is omitted when false, and when false there is no `value`. A `Verify*` response shows
  this: constraint and requirement features of the subject are reported unmaterialized.

An error on one feature does not fail the call. `conformance/fixtures/cyclic.sysml` defines
`a = b + 1.0` and `b = a + 1.0`:

```console
$ … /Instantiate -d '{"modelHash":"5a85c5777a29e42321bdd577aa814c74c40914a569b354949b5f55be29a6c15b","symbolId":"Demo::Cyclic"}'
{"instance":{"id":"1","typeSymbolId":"Demo::Cyclic","featureValues":{"a":{"featureName":"a","error":"feature value Cyclic.a: feature value Cyclic.b: cyclic feature value dependency: Cyclic.a"},"b":{"featureName":"b","error":"feature value Cyclic.b: feature value Cyclic.a: cyclic feature value dependency: Cyclic.b"}}},"instances":[…]}
```

(`instances` repeats the root.) The Python client raises `FeatureValueError` when such a feature
is *read*, not when the instance is received; a hand-written client should likewise keep the
error beside the feature.

A symbol the model does not have is an in-body failure:

```console
$ … /Instantiate -d '{"modelHash":"5a85…c15b","symbolId":"Demo::Nope"}'
{"error":"symbol not found: Demo::Nope"}
```

### `ExecuteAction`

`inputs` is a map from parameter name to `Value`; `outputs` is the same shape back. A missing
input keeps its declared default:

```console
$ … /ExecuteAction -d '{"modelHash":"b4e0…ded9","actionSymbolId":"Test::addFive","inputs":{"result":{"intValue":"10"}}}'
{"outputs":{"result":{"intValue":"15"}}}

$ … /ExecuteAction -d '{"modelHash":"b4e0…ded9","actionSymbolId":"Test::addFive"}'
{"outputs":{"result":{"intValue":"5"}}}

$ … /ExecuteAction -d '{"modelHash":"b4e0…ded9","actionSymbolId":"Test::noStart"}'
{"error":"action execution failed: initialize action: invalid action flow: no initial node found in action noStart"}
```

An action with no outputs answers `{}` (captured for `action nop { first start; done;
succession first start then done; }`). The response carries no step trace; ordering-sensitive
behavior is pinned by the engine's golden traces, not exposed on this call.

### `ExecuteState`

`events` is an ordered list of event names to feed the machine after it enters; `statesVisited`
is the trace of state names in the order entered, and `finalContext` the machine's variables
when it stopped, as a name → `Value` map. `conformance/fixtures/behavior.sysml`'s `Test::Machine`
runs to `done` on its own:

```console
$ … /ExecuteState -d '{"modelHash":"b4e0…ded9","stateMachineSymbolId":"Test::Machine"}'
{"statesVisited":["init","Running","done"]}
```

A machine with an attribute and event-triggered transitions (`state Controller { attribute
cycles : Integer = 0; entry; then off; state off; state on; transition first off accept start
do assign cycles := cycles + 1 then on; transition first on accept stop then off; }` in package
`Pump`):

```console
$ … /ExecuteState -d '{"modelHash":"449e7db990943f08c1918829b0c730cc9d9bf415b236a70debe93592d2477b6b","stateMachineSymbolId":"Pump::Controller","events":["start","stop","start"]}'
{"statesVisited":["off","on","off","on"],"finalContext":{"cycles":{"intValue":"2"}}}

$ … /ExecuteState -d '{"modelHash":"b4e0…ded9","stateMachineSymbolId":"Test::NoMachine"}'
{"error":"state machine not found: Test::NoMachine"}
```

`finalContext` is absent when the machine has no variables; `statesVisited` lists a state each
time it is entered, so a state entered twice appears twice.

### `EvaluateCalc`

`arguments` is a positional list of `Value`s matching the calc's `in` parameters in order.
`result` is the calc's value:

```console
$ … /EvaluateCalc -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::add","arguments":[{"intValue":"2"},{"realValue":3.5}]}'
{"result":{"realValue":5.5}}
```

A structured argument goes back the way it came — the same `array`, `vector` or
`vectorQuantity` body the service writes — and a malformed one is an in-body failure naming
the fault, not a value read some other way:

```console
$ … /EvaluateCalc -d '{"modelHash":"42cc…54b0","symbolId":"S::length","arguments":[{"vector":{"components":[{"realValue":3.0},{"realValue":4.0}]}}]}'
{"result":{"realValue":5}}

$ … /EvaluateCalc -d '{"modelHash":"42cc…54b0","symbolId":"S::length","arguments":[{"vector":{"components":[{"realValue":3.0},{"stringValue":"4"}]}}]}'
{"error":"calc argument could not be read: vector component is not a number: component 2", "failureReason":"FAILURE_REASON_EVALUATION"}
```

A `measurementRef` argument goes back the same way, and is read against the model's own unit
declaration when it names one — a named unit without its reduction, or with a reduction that
is not the declaration's, is the same kind of in-body failure:

```console
$ … /EvaluateCalc -d '{"modelHash":"5b0f…40d5","symbolId":"M::toUnit","arguments":[{"quantity":{"intMagnitude":"3","unit":"km","unitTerm":{"scaleNum":1000,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]}}},{"measurementRef":{"unit":"m","unitTerm":{"scaleNum":1,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]},"unitId":"SI::metre"}}]}'
{"result":{"quantity":{"realMagnitude":3000, "unit":"m", "unitTerm":{"scaleNum":1, "scaleDen":1, "factors":[{"unitId":"SI::metre", "exponent":1}]}}}}

$ … /EvaluateCalc -d '{"modelHash":"5b0f…40d5","symbolId":"M::toUnit","arguments":[…,{"measurementRef":{"unit":"m","unitId":"SI::metre"}}]}'
{"error":"calc argument could not be read: unit carries no reduction to base units: m", "failureReason":"FAILURE_REASON_EVALUATION"}

$ … /EvaluateCalc -d '{"modelHash":"5b0f…40d5","symbolId":"M::toUnit","arguments":[…,{"measurementRef":{"unit":"m","unitTerm":{"scaleNum":1000,"scaleDen":1,"factors":[{"unitId":"SI::metre","exponent":1}]},"unitId":"SI::metre"}}]}'
{"error":"calc argument could not be read: unit as written does not reduce to its unit_term: SI::metre reduces to metre, unit_term is 1000·metre", "failureReason":"FAILURE_REASON_EVALUATION"}
```

A service without the `structured_values` capability refuses a structured argument, and one
without `measurement_refs` a `measurementRef` argument, with the `unimplemented` Connect
error instead, naming the capability; check `GetServerInfo` first.

A calc *usage* whose output features are evaluated from its own members (no `arguments`)
answers them as `outputs`, a list of `{"name":…,"value":<Value>}` in declaration order, in
place of `result`; a client reads whichever of the two is present. A symbol that is not a calc
is the `FAILURE_REASON_WRONG_KIND` failure shown under [In-body failures](#in-body-failures);
for an analysis case the message says to use `RunAnalysis`.

### `RunAnalysis`

`symbolId` names an analysis definition or usage. `subjectSymbolId` optionally names a part or
usage to instantiate as the case's `subject`, as `VerifyRequirement` takes one; a usage that
binds its own subject (`subject s = ship;`) needs none, and a definition or unbinding usage
run without one is an in-body failure naming the subject. `arguments` is a positional list of
`Value`s for the case's other `in` parameters in declaration order and `namedArguments` binds
them by name; a parameter left without a value or default is an in-body failure. `outputs`
are the case's `out` and `return` values as `EvaluateCalc` reports a usage's, a returned value
with no name under `result`; `verdicts` is one `Verdict` per `objective` and per
`assert constraint` in the body, in that order, with `kind` `objective` or `assertion`,
`holds` for a satisfied one, `condition` for one that is not, and `error` for one that could
not be decided; `instances` is the subject's object graph when one was
instantiated:

```console
$ … /RunAnalysis -d '{"modelHash":"e43c…9a2a","symbolId":"An::shipCost"}'
{"outputs":[{"name":"total","value":{"realValue":12}}],"verdicts":[{"kind":"objective","elementId":"An::CostAnalysis::affordable","element":"affordable","holds":true}]}

$ … /RunAnalysis -d '{"modelHash":"e43c…9a2a","symbolId":"An::CostAnalysis","subjectSymbolId":"An::barge","namedArguments":{"limit":{"realValue":50.0}}}'
{"outputs":[{"name":"total","value":{"realValue":37}}],"verdicts":[{"kind":"objective","elementId":"An::CostAnalysis::affordable","element":"affordable","holds":true,"instanceId":"1","instanceTypeId":"An::barge"}],"instances":[{"id":"1","typeSymbolId":"An::barge",…}]}

$ … /RunAnalysis -d '{"modelHash":"e43c…9a2a","symbolId":"An::CostAnalysis"}'
{"error":"analysis run failed: analysis An::CostAnalysis: s subject is unbound: bind it (`subject s = <element>`) or run it on an object","failureReason":"FAILURE_REASON_EVALUATION"}

$ … /RunAnalysis -d '{"modelHash":"e43c…9a2a","symbolId":"An::Ship"}'
{"error":"not an analysis case: An::Ship is a part def, not an analysis case definition or usage","failureReason":"FAILURE_REASON_WRONG_KIND"}
```

A step that fails, a body that deadlocks or exhausts its step budget and a case that runs itself are
`FAILURE_REASON_EVALUATION` failures naming the case. Structured and complex arguments are
capability-gated as `EvaluateCalc`'s are.

### `RunSweep`

The same request as `RunAnalysis` — `symbolId` naming an analysis case **or** a calc,
`subjectSymbolId`, `arguments`, `namedArguments` — plus `ranges`, and `samples` with `seed`.
Each `SweepRange` names a `parameter` the target declares and the arguments do not bind, with
`start`, `end` and an optional `step` as `Value`s; `end` is included where the step lands on it,
a range between Integers with no step steps by one, and one between Reals with no step is
refused. The response's `parameters` are the swept parameters in request order and `rows` is one
run each, in lexicographic order over them (the first range varying slowest). A row carries the
`inputs` bound for that run, its `outputs` (a calc's returned value under `result`, as
`EvaluateCalc` reports it), its `verdicts` where the case has an objective, and `elapsedMicros`,
the wall time of that run in microseconds — the one fixed unit, absent for a run that took
under one:

```console
$ … /RunSweep -d '{"modelHash":"a6dc…4849","symbolId":"An::Fall","ranges":[{"parameter":"t","start":{"realValue":0.0},"end":{"realValue":2.0},"step":{"realValue":1.0}}]}'
{"rows":[{"inputs":[{"name":"t","value":{"realValue":0}}],"outputs":[{"name":"result","value":{"realValue":0}}],"elapsedMicros":"31"},
         {"inputs":[{"name":"t","value":{"realValue":1}}],"outputs":[{"name":"result","value":{"realValue":4.905}}]},
         {"inputs":[{"name":"t","value":{"realValue":2}}],"outputs":[{"name":"result","value":{"realValue":19.62}}]}],
 "parameters":["t"]}

$ … /RunSweep -d '{"modelHash":"a6dc…4849","symbolId":"An::CostAnalysis","subjectSymbolId":"An::barge","ranges":[{"parameter":"tax","start":{"realValue":0.0},"end":{"realValue":1.0},"step":{"realValue":0.5}}]}'
{"rows":[…,
         {"inputs":[{"name":"tax","value":{"realValue":1}}],"outputs":[{"name":"total","value":{"realValue":10}}],
          "verdicts":[{"kind":"objective","element":"obj","condition":"total <= 8.0","instanceId":"1","instanceTypeId":"An::barge"}],"elapsedMicros":"16"}],
 "parameters":["tax"]}
```

A run that fails is a row of its own, carrying `error` and `failureReason` in place of its
`outputs`, and the runs after it are still made:

```console
$ … /RunSweep -d '{"modelHash":"ed8c…2ec4","symbolId":"An::Ratio","namedArguments":{"a":{"realValue":4.0}},"ranges":[{"parameter":"b","start":{"intValue":"-1"},"end":{"intValue":"1"}}]}'
{"rows":[{"inputs":[{"name":"b","value":{"intValue":"-1"}}],"outputs":[{"name":"result","value":{"realValue":-4}}],"elapsedMicros":"18"},
         {"inputs":[{"name":"b","value":{"intValue":"0"}}],"elapsedMicros":"6",
          "error":"calc An::Ratio: evaluating the returned expression: division by zero","failureReason":"FAILURE_REASON_EVALUATION"},
         {"inputs":[{"name":"b","value":{"intValue":"1"}}],"outputs":[{"name":"result","value":{"realValue":4}}]}],
 "parameters":["b"]}
```

`samples` draws that many values for each range instead of running every value of it —
uniformly, in draw order, from `math/rand/v2`'s `PCG` seeded from `seed`, which the response
echoes alongside `sampled` — and a sampled range needs no `step`:

```console
$ … /RunSweep -d '{"modelHash":"a6dc…4849","symbolId":"An::Fall","ranges":[{"parameter":"t","start":{"realValue":0.0},"end":{"realValue":10.0}}],"samples":"2","seed":"42"}'
{"rows":[{"inputs":[{"name":"t","value":{"realValue":8.254725069980449}}],"outputs":[{"name":"result","value":{"realValue":334.22908373662705}}],"elapsedMicros":"7"},
         {"inputs":[{"name":"t","value":{"realValue":0.4281995136143024}}],"outputs":[{"name":"result","value":{"realValue":0.8993554090689709}}]}],
 "parameters":["t"],"sampled":true,"seed":"42"}
```

A request the plan cannot be built from answers `error` with no rows at all, so a client
distinguishes a refused plan from a table of failed runs by whether `rows` is present: a symbol
that is neither an analysis case nor a calc is `FAILURE_REASON_WRONG_KIND`, and a missing range,
a step of zero, a step whose sign never reaches `end`, incompatible units, an undeclared
parameter, one the arguments bind, a negative `samples`, and a plan asking for more runs than
`OPENSYSML_MAX_SWEEP_RUNS` allows are `FAILURE_REASON_EVALUATION`. `seed` is a `uint64` with no
unset state on the wire, so a request that draws without naming one draws from seed 0 (where the
CLI's `-samples` requires `-seed` rather than choosing a seed for you):

```console
$ … /RunSweep -d '{"modelHash":"a6dc…4849","symbolId":"An::Ship","ranges":[{"parameter":"t","start":{"intValue":"1"},"end":{"intValue":"3"}}]}'
{"error":"not a calc: An::Ship declares neither an analysis case nor a calc","failureReason":"FAILURE_REASON_WRONG_KIND"}

$ … /RunSweep -d '{"modelHash":"a6dc…4849","symbolId":"An::Fall"}'
{"error":"no sweep range: name a range as <parameter>=<from>..<to>","failureReason":"FAILURE_REASON_EVALUATION"}
```

### `Evaluate`

Not a behavior in the model, but the general-purpose call that every other example here uses:
`expression` is SysML expression text, `contextSymbolId` names the scope names resolve in, and
`subjectSymbolId` optionally names a usage whose features the expression may refer to directly
(the expression is evaluated *on* an instance of it):

```console
$ … /Evaluate -d '{"modelHash":"59c4…a654","expression":"mass * 2.0","subjectSymbolId":"Rover::rover"}'
{"result":{"realValue":40}}
```

The response is `result` **or** `error` (with `diagnostics` when the expression did not parse),
never both.

### `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction` and the `Verdict` shape

`VerifyConstraint` and `VerifyRequirement` take a `symbolId` and an optional `subjectSymbolId`
— the usage to instantiate and evaluate the condition against — and return one `verdict`.
`VerifySatisfaction` takes the element that owns `satisfy` assertions and returns `verdicts`,
one per assertion. All three return the `instances` they built, in the same shape as
`Instantiate`.

```console
$ … /VerifyConstraint -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::Vehicle::massPositive"}'
{"verdict":{"kind":"constraint","elementId":"Demo::Vehicle::massPositive","element":"Demo::Vehicle::massPositive","holds":true}}

$ … /VerifyRequirement -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::Vehicle::lightEnough"}'
{"verdict":{"kind":"requirement","elementId":"Demo::Vehicle::lightEnough","element":"Demo::Vehicle::lightEnough","holds":true}}

$ … /VerifyConstraint -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::Vehicle::massLight","subjectSymbolId":"Demo::sedan"}'
```

```json
{
  "verdict": {
    "kind": "constraint",
    "elementId": "Demo::Vehicle::massLight",
    "element": "Demo::Vehicle::massLight",
    "condition": "mass < 100.0",
    "instanceId": "1",
    "instanceTypeId": "Demo::sedan"
  },
  "instances": [
    {
      "id": "1",
      "typeSymbolId": "Demo::sedan",
      "featureValues": {
        "lightEnough":  {"featureName": "lightEnough"},
        "mass":         {"featureName": "mass", "value": {"realValue": 1200}, "materialized": true},
        "massLight":    {"featureName": "massLight"},
        "massPositive": {"featureName": "massPositive"},
        "tiny":         {"featureName": "tiny"}
      }
    }
  ]
}
```

Reading a `Verdict`:

- `kind` — `"constraint"`, `"requirement"` or `"satisfy"`.
- `holds` — the verdict. **Omitted when false** (the default-omission rule), so the second and
  third examples say `massLight` does *not* hold for `sedan` (1200 is not < 100) by having no
  `holds` key. Read it with a default of `false`, and read `error` first.
- `error` — non-empty when the condition **could not be evaluated**. Then `holds` is
  meaningless (and absent), and the answer is neither true nor false: it is a failure. The
  Python client raises for it rather than returning `False`. `failureReason` accompanies it
  when the reason is classified:

  ```console
  $ … /VerifyConstraint -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::sedan"}'
  {"verdict":{"kind":"constraint","elementId":"Demo::sedan","element":"Demo::sedan","error":"not a constraint: sedan is a part usage, not a constraint definition or usage","failureReason":"FAILURE_REASON_WRONG_KIND"}}
  ```

  So: `holds:true` → holds; no `holds`, no `error` → does not hold; `error` → could not decide.
- `condition` — the condition that evaluated to false, as written, when the runtime can name
  one; omitted when the verdict holds, when the false verdict is not attributed to a single
  condition, and on a failure (the `not a constraint` example above has none).
- `elementId` / `element` — the id of the element checked, and its display form. For a
  `satisfy` assertion, which has no name, `elementId` is absent and `element` is the assertion
  text.
- `instanceId` / `instanceTypeId` — the instance the condition was evaluated on, a key into
  `instances`, and its type. Absent when no subject was instantiated (the first two examples,
  which evaluated against the definition's own defaults).

`VerifySatisfaction` over `Demo::analysis`, which asserts `massLimit` (max 2000) and `massTiny`
(max 10) are satisfied by `sedan`:

```console
$ … /VerifySatisfaction -d '{"modelHash":"b4e0…ded9","symbolId":"Demo::analysis"}'
```

```json
{
  "verdicts": [
    {"kind": "satisfy", "element": "satisfy massLimit by sedan", "holds": true, "instanceId": "1", "instanceTypeId": "Demo::sedan"},
    {"kind": "satisfy", "element": "satisfy massTiny by sedan", "condition": "vehicle.mass <= maxMass", "instanceId": "2", "instanceTypeId": "Demo::sedan"}
  ],
  "instances": [ … ]
}
```

(`instances` holds `"id":"1"` and `"id":"2"`, both `Demo::sedan` with the feature values shown
for the `massLight` example.) Each assertion instantiated its own `sedan`, hence two ids; the
second verdict has no `holds` and no `error`, so it is a real *false*.

## Queries

Two query surfaces exist and answer differently shaped tables. Their semantics — what may be
selected, filtered and bound — are on the Go API page and are not repeated here:
[SysML v2 API & Services `Query`](api.md#sysml-v2-api--services-query) and
[Native document queries and rendering over gRPC](api.md#native-document-queries-and-rendering-over-grpc).
Each is its own capability: `Query` needs `query` (and `oslc_query` when the request uses
`oslcQuery`); `RunDocumentQuery` needs `document_query`; `RenderDocument` needs
`render_document`.

### `Query`

The request carries a structured `query` (`scope`, `select`, `where`) **or** an `oslcQuery`
string in the OSLC syntax of [OSLC Query text](oslc-query.md), never both. `where` is a
`Constraint`, itself a `oneof` of `primitive` and `composite`; the operator is a proto enum and
is written as its **name**; `value` is a list of strings:

```console
$ … /Query -d '{"modelHash":"2af52c50cee63699ece8f9021b6344e4fe9f2fe6eeb0f3f8edd9feaa5443dea2","query":{"select":["name","owner","type"],"where":{"primitive":{"property":"@type","operator":"PRIMITIVE_OPERATOR_EQUAL","value":["PartUsage"]}}}}'
```

```json
{
  "elements": [
    {"id": "Demo::Vehicle::engine", "type": "PartUsage", "properties": {"name": "engine", "owner": "Demo::Vehicle", "type": "Demo::Engine"}},
    {"id": "Demo::sedan",           "type": "PartUsage", "properties": {"name": "sedan",  "owner": "Demo",          "type": "Demo::Vehicle"}}
  ]
}
```

Each element has its `id` (fully qualified name), its `type` (the SysML metaclass name), and
`properties`, a **string → string** map holding exactly the `select`ed properties that the
element has a value for. Everything in `properties` is a string, including numbers such as
`multiplicityLower`; there are no `Value` objects on this call. An element without a selected
property simply lacks the key. No matches is `{}` (captured for `"property":"name"` equal to
`"nobody"`).

### `RunDocumentQuery`

A document query is a `calc def` in the model specializing `DocumentQueries::Query`; the request
names it by `queryId` and supplies `bindings` for its `in` parameters. Each binding is a
`parameter` name and a list of `values`, and each value is a `DocumentValue` — a `oneof` whose
arm says what was bound:

- **`elementId`** binds a model element by fully qualified name. *This is how a parameter
  of type `Element` is bound.*
- **`stringValue`, `intValue` (string), `realValue`, `boolValue`, `infinity`** bind a literal;
  the query treats it as a value, not a name. A string that happens to be a qualified name
  bound as `stringValue` is a string.
- **`quantity`** binds a magnitude with a unit, the same object a `Value` carries (`unit`,
  `unitTerm` and one of `intMagnitude`/`realMagnitude`); it is how a projected
  `attribute :>> mass = 2290000 [kg];` is answered. Bound, it conforms to a parameter typed
  by a quantity value type of the same dimension (`MassValue` for a mass, any for
  `ScalarQuantityValue`); a parameter of another dimension or of a scalar type such as
  `String` refuses it with `invalid_argument`.

Model `7e6a…a687` is `conformance/fixtures/document.sysml`; `HeavySubsystemNames` takes
`root : Element` and `threshold : String`:

```console
$ … /RunDocumentQuery -d '{"modelHash":"7e6a2c9c119fa1b951f0db8fbac36e2b94899144999085b0e27ee124cb57a687","queryId":"Observatory::HeavySubsystemNames","bindings":[{"parameter":"root","values":[{"elementId":"Observatory::telescope"}]},{"parameter":"threshold","values":[{"stringValue":"10"}]}]}'
```

```json
{
  "columns": [{"name": "name"}],
  "rows": [
    {"element": {"elementId": "Observatory::telescope::mount",          "elementType": "PartUsage"}, "cells": [{"values": [{"stringValue": "mount"}]}]},
    {"element": {"elementId": "Observatory::telescope::segmentControl", "elementType": "PartUsage"}, "cells": [{"values": [{"stringValue": "segmentControl"}]}]}
  ]
}
```

The answer is a table: `columns` in order, and `rows` each with `element` (the row's subject as
a `DocumentValue`, here always `elementId` plus `elementType`) and `cells` **positionally
aligned with `columns`**. A cell holds `values`, a list of `DocumentValue`s (several for a
multi-valued property, none for a missing one, in which case `values` is absent). A
`DocumentValue` decodes like a `Value` — one arm present — but its arms are the seven above and
never a nested sequence or enum. `SubsystemTable` projects two columns and shows a
`realValue` cell:

```console
$ … /RunDocumentQuery -d '{"modelHash":"7e6a…a687","queryId":"Observatory::SubsystemTable","bindings":[{"parameter":"root","values":[{"elementId":"Observatory::telescope"}]}]}'
{"columns":[{"name":"name"},{"name":"mass"}],"rows":[{"element":{"elementId":"Observatory::telescope::baffle|shroud *tricky*","elementType":"PartUsage"},"cells":[{"values":[{"stringValue":"baffle|shroud *tricky*"}]},{"values":[{"realValue":1.5}]}]},{"element":{"elementId":"Observatory::telescope::mount","elementType":"PartUsage"},"cells":[{"values":[{"stringValue":"mount"}]},{"values":[{"realValue":15}]}]},{"element":{"elementId":"Observatory::telescope::optics","elementType":"PartUsage"},"cells":[{"values":[{"stringValue":"optics"}]},{"values":[{"realValue":8.5}]}]},{"element":{"elementId":"Observatory::telescope::segmentControl","elementType":"PartUsage"},"cells":[{"values":[{"stringValue":"segmentControl"}]},{"values":[{"realValue":20}]}]}]}
```

The request-side failures are Connect errors, because the request — not the model — is wrong:

```console
$ … /RunDocumentQuery -d '{"modelHash":"7e6a…a687","queryId":"Observatory::SubsystemTable"}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"query Observatory::SubsystemTable requires binding root (declared in document.sysml)"}

$ … /RunDocumentQuery -d '{"modelHash":"7e6a…a687","queryId":"Observatory::telescope"}'
HTTP/1.1 400 Bad Request
{"code":"invalid_argument","message":"Observatory::telescope is not a document query: one is a calc def specializing DocumentQueries::Query"}
```

## Minimal clients: four illustrations

The four snippets below are **illustrations, not shipped code**. They are not in `clients/`, not
tested, and not run by CI; they exist to show how short a correct decoder is in each language
and where its pitfalls lie. A real client for any of these languages is one that passes the
scenarios in `conformance/scenarios/*.json` through its own public API, as every shipped client
does ([Every client runs the same conformance suite](clients.md#every-client-runs-the-same-conformance-suite)).
Each snippet is a POST helper that classifies Connect errors, plus the `Value` decoder from
[The decoding rule](#the-decoding-rule); everything else (the `Instance` table, verdicts,
query rows) is plain JSON once the `Value`s inside it are decoded.

### R (`httr2` + `jsonlite`)

```r
library(httr2); library(jsonlite)
`%||%` <- function(a, b) if (is.null(a)) b else a

sysml_post <- function(method, body, base = "http://localhost:50051") {
  resp <- request(paste0(base, "/sysml.SysMLService/", method)) |>
    req_body_json(body, auto_unbox = TRUE, digits = NA) |>   # wrap lists in I() to keep them arrays
    req_error(is_error = function(r) FALSE) |>
    req_perform()
  out <- resp_body_json(resp, simplifyVector = FALSE)
  if (resp_status(resp) != 200) stop(sprintf("connect %s: %s", out$code, out$message))
  out
}

decode_value <- function(v) {
  if (is.null(v)) return(structure(list(), class = "sysml_no_result"))
  switch(names(v)[[1]],
    intValue    = bit64::as.integer64(v$intValue),          # never as.numeric: exact past 2^53
    realValue   = as.double(v$realValue),                    # "NaN"/"Infinity" strings decode via as.double too
    boolValue   = v$boolValue,
    stringValue = v$stringValue,
    instanceId  = structure(v$instanceId, class = "sysml_instance_ref"),
    sequence    = lapply(v$sequence$elements, decode_value),
    null        = if (nzchar(v[["null"]])) stop("unsupported value: ", v[["null"]]) else NULL,
    unset       = structure(NA, class = "sysml_unset"),
    quantity    = list(magnitude = if (!is.null(v$quantity$intMagnitude))
                                     bit64::as.integer64(v$quantity$intMagnitude)
                                   else as.double(v$quantity$realMagnitude),
                       unit = v$quantity$unit, unit_term = v$quantity$unitTerm),
    enumLiteral = structure(v$enumLiteral$literalId, class = "sysml_enum_literal",
                            enumeration = v$enumLiteral$enumerationId, label = v$enumLiteral$name),
    complex     = complex(real = v$complex$real %||% 0, imaginary = v$complex$imaginary %||% 0),
    stop("unknown Value arm: ", names(v)[[1]]))
}

m <- sysml_post("ParseSources", list(documents = list(list(name = "a.sysml", content = "package A { attribute x = 1; }"))))
decode_value(sysml_post("Evaluate", list(modelHash = m$modelHash, expression = "A::x"))$result)
```

### Julia (`HTTP.jl` + `JSON.jl`)

```julia
using HTTP, JSON

function sysml_post(method, body; base = "http://localhost:50051")
    r = HTTP.post("$base/sysml.SysMLService/$method",
                  ["Content-Type" => "application/json"], JSON.json(body); status_exception = false)
    out = JSON.parse(String(r.body))
    r.status == 200 || error("connect $(out["code"]): $(out["message"])")
    out
end

struct Unset end
struct InstanceRef; id::Int64; end
struct EnumLiteral; literal::String; enumeration::String; label::String; end
struct Quantity; magnitude::Union{Int64,Float64}; unit::String; term::Any; end

asreal(x) = x isa AbstractString ? parse(Float64, x) : Float64(x)   # "NaN"/"Infinity" and 20 → 20.0

function decode_value(v)
    v === nothing && return missing                       # no result at all
    haskey(v, "intValue")    && return parse(Int64, v["intValue"])
    haskey(v, "realValue")   && return asreal(v["realValue"])
    haskey(v, "boolValue")   && return v["boolValue"]::Bool
    haskey(v, "stringValue") && return v["stringValue"]::String
    haskey(v, "instanceId")  && return InstanceRef(parse(Int64, v["instanceId"]))
    haskey(v, "sequence")    && return [decode_value(e) for e in get(v["sequence"], "elements", [])]
    haskey(v, "null")        && return isempty(v["null"]) ? nothing : error("unsupported value: ", v["null"])
    haskey(v, "unset")       && return Unset()
    haskey(v, "quantity")    && (q = v["quantity"]; return Quantity(
        haskey(q, "intMagnitude") ? parse(Int64, q["intMagnitude"]) : asreal(q["realMagnitude"]),
        get(q, "unit", ""), get(q, "unitTerm", nothing)))
    haskey(v, "enumLiteral") && (l = v["enumLiteral"]; return EnumLiteral(l["literalId"], l["enumerationId"], l["name"]))
    haskey(v, "complex")     && (c = v["complex"]; return complex(asreal(get(c, "real", 0.0)), asreal(get(c, "imaginary", 0.0))))
    error("unknown Value arm: ", first(keys(v)))
end

m = sysml_post("ParseSources", Dict("documents" => [Dict("name" => "a.sysml", "content" => "package A { attribute x = 1; }")]))
decode_value(get(sysml_post("Evaluate", Dict("modelHash" => m["modelHash"], "expression" => "A::x")), "result", nothing))
```

### MATLAB (`matlab.net.http` + `jsondecode`)

`webwrite` posts the same body and decodes a 200 the same way, but it raises on any other
status without exposing the `{"code","message"}` body, so the helper uses `matlab.net.http`.

```matlab
function out = sysmlPost(method, body, base)
    if nargin < 3, base = 'http://localhost:50051'; end
    import matlab.net.http.*
    req = RequestMessage('POST', HeaderField('Content-Type', 'application/json'), MessageBody(jsonencode(body)));
    resp = req.send(sprintf('%s/sysml.SysMLService/%s', base, method), HTTPOptions('ConvertResponse', false));
    out = jsondecode(char(resp.Body.Data));       % 404/405/415 answer plain text; this would raise on it
    if resp.StatusCode ~= 200, error('sysml:connect', 'connect %s: %s', out.code, out.message); end
end

function val = decodeValue(v)
    % jsondecode turns {"intValue":"4"} into a struct with field intValue = '4' (char),
    % {"realValue":20} into double 20, and an array of mixed objects into a cell array.
    if isempty(v), val = []; return; end                  % no result (field absent)
    kind = fieldnames(v); kind = kind{1};
    asInt64 = @(s) int64(java.lang.Long.parseLong(s));   % exact; str2double/sscanf go through double
    asReal  = @(x) realOf(x);
    switch kind
        case 'intValue',    val = asInt64(v.intValue);
        case 'realValue',   val = asReal(v.realValue);
        case 'boolValue',   val = logical(v.boolValue);
        case 'stringValue', val = string(v.stringValue);
        case 'instanceId',  val = struct('instanceRef', asInt64(v.instanceId));
        case 'sequence',    el = {};
                            if isfield(v.sequence, 'elements'), el = v.sequence.elements; end
                            if isstruct(el), el = num2cell(el); end    % uniform objects arrive as a struct array
                            val = cellfun(@decodeValue, el, 'UniformOutput', false);
        case 'null',        if ~isempty(v.null), error('sysml:unsupported', '%s', v.null); end; val = missing;
        case 'unset',       val = struct('unset', true);                % test isfield(val,'unset'), never val.unset
        case 'quantity',    q = v.quantity;
                            if isfield(q, 'intMagnitude'), mag = asInt64(q.intMagnitude); else, mag = asReal(q.realMagnitude); end
                            val = struct('magnitude', mag, 'unit', q.unit, 'unitTerm', q.unitTerm);
        case 'enumLiteral', val = struct('literalId', v.enumLiteral.literalId, 'enumerationId', v.enumLiteral.enumerationId, 'name', v.enumLiteral.name);
        case 'complex',     re = 0; im = 0;                             % each omitted when zero
                            if isfield(v.complex, 'real'), re = v.complex.real; end
                            if isfield(v.complex, 'imaginary'), im = v.complex.imaginary; end
                            val = complex(re, im);
        otherwise,          error('sysml:unknownArm', 'unknown Value arm: %s', kind);
    end
end

function x = realOf(x)                        % "NaN", "Infinity", "-Infinity" arrive as char
    if ischar(x) || isstring(x)
        switch char(x), case 'Infinity', x = Inf; case '-Infinity', x = -Inf; otherwise, x = NaN; end
    else
        x = double(x);
    end
end
```

### C (`libcurl` + `cJSON`)

```c
#include <curl/curl.h>
#include <cjson/cJSON.h>
#include <stdlib.h>
#include <string.h>

struct buf { char *p; size_t n; };
static size_t grow(void *d, size_t s, size_t n, void *u) {
    struct buf *b = u; b->p = realloc(b->p, b->n + s * n + 1);
    memcpy(b->p + b->n, d, s * n); b->n += s * n; b->p[b->n] = 0; return s * n;
}

/* Returns the parsed body; *status receives the HTTP status. On != 200 the body is {"code","message"}. */
cJSON *sysml_post(const char *base, const char *method, const char *json, long *status) {
    char url[512]; snprintf(url, sizeof url, "%s/sysml.SysMLService/%s", base, method);
    struct buf b = {0}; CURL *c = curl_easy_init();
    struct curl_slist *h = curl_slist_append(NULL, "Content-Type: application/json");
    curl_easy_setopt(c, CURLOPT_URL, url); curl_easy_setopt(c, CURLOPT_HTTPHEADER, h);
    curl_easy_setopt(c, CURLOPT_POSTFIELDS, json);
    curl_easy_setopt(c, CURLOPT_WRITEFUNCTION, grow); curl_easy_setopt(c, CURLOPT_WRITEDATA, &b);
    if (curl_easy_perform(c) != CURLE_OK) { *status = 0; return NULL; }
    curl_easy_getinfo(c, CURLINFO_RESPONSE_CODE, status);
    curl_slist_free_all(h); curl_easy_cleanup(c);
    cJSON *out = cJSON_Parse(b.p); free(b.p); return out;   /* NULL for the plain-text 404/405/415 bodies */
}

enum kind { NO_RESULT, INT, REAL, BOOL, STR, INSTANCE, SEQ, NUL, QUANTITY, ENUM, UNSET, COMPLEX, UNKNOWN };
struct value { enum kind kind; long long i; double d; const char *s; const cJSON *node; };

/* Discriminate by the single key present. Strings for int64; doubles may be "NaN"/"Infinity". */
struct value decode_value(const cJSON *v) {
    struct value r = { NO_RESULT, 0, 0, NULL, v };
    if (!v) return r;
    const cJSON *a;
    if ((a = cJSON_GetObjectItem(v, "intValue")))    { r.kind = INT;  r.i = strtoll(a->valuestring, NULL, 10); }
    else if ((a = cJSON_GetObjectItem(v, "realValue")))   { r.kind = REAL; r.d = cJSON_IsString(a) ? strtod(a->valuestring, NULL) : a->valuedouble; }
    else if ((a = cJSON_GetObjectItem(v, "boolValue")))   { r.kind = BOOL; r.i = cJSON_IsTrue(a); }
    else if ((a = cJSON_GetObjectItem(v, "stringValue"))) { r.kind = STR;  r.s = a->valuestring; }
    else if ((a = cJSON_GetObjectItem(v, "instanceId")))  { r.kind = INSTANCE; r.i = strtoll(a->valuestring, NULL, 10); }
    else if ((a = cJSON_GetObjectItem(v, "sequence")))    { r.kind = SEQ;  r.node = cJSON_GetObjectItem(a, "elements"); /* recurse over cJSON_ArrayForEach */ }
    else if ((a = cJSON_GetObjectItem(v, "null")))        { r.kind = NUL;  r.s = a->valuestring; /* non-empty: unsupported value, not null */ }
    else if ((a = cJSON_GetObjectItem(v, "unset")))       { r.kind = UNSET; }
    else if ((a = cJSON_GetObjectItem(v, "quantity")))    { r.kind = QUANTITY; r.node = a; /* intMagnitude (string) or realMagnitude; unit; unitTerm */ }
    else if ((a = cJSON_GetObjectItem(v, "enumLiteral"))) { r.kind = ENUM; r.s = cJSON_GetObjectItem(a, "literalId")->valuestring; r.node = a; }
    else if ((a = cJSON_GetObjectItem(v, "complex")))     { r.kind = COMPLEX; r.node = a; /* "real"/"imaginary", each absent when 0 */ }
    else { r.kind = UNKNOWN; r.s = v->child ? v->child->string : ""; /* a newer service than this decoder */ }
    return r;
}
```

A conformance-grade client in any of these languages additionally: reads `error` before
`result` on every response; keeps `instanceId` scoped to its response; classifies Connect codes
per the table above; and re-parses on `model not found:`. The scenarios that pin all of this
are `conformance/scenarios/*.json`; run them through your client's public API, as
`make conformance` does for the shipped ones.

## Further reading

- [Service transports](service-transports.md) — the four protocols on one port, capabilities,
  and why protobuf bodies are the default for a generated client
- [Client libraries](clients.md) — the shipped clients, their lifecycle modes and the
  conformance suite they share
- [Go packages](api.md) — `Query` and document-query semantics this page's examples exercise
- `api/proto/sysml.proto` — the authority for every field name and type on this page
