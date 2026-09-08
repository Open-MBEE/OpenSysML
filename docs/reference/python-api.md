# The Python client API

This page covers what `opensysml` costs, the typed classes it can generate for a model, and the
modules behind them. For a task-oriented walkthrough, see [guide chapter 9](../guide/09-clients.md).

## Latency

Measured on this repo's benchmark (`clients/python/scripts/bench_latency.py`, 8-core
x86-64 Linux, loopback, 20 part definitions / 808 bytes, 200 iterations, warm
`Connection`):

| operation | p50 | p95 | p99 |
| --- | --- | --- | --- |
| `load` / `load_from_content`, cache miss | 35 ms | 56 ms | 60 ms |
| `load` / `load_from_content`, cache hit | 0.5 ms | 1.0 ms | 1.0 ms |
| `eval("2 + 2")` on a cached model | 0.4 ms | 1.0 ms | 1.2 ms |
| `convert` sysml → sysml | 0.7 ms | 1.2 ms | 2.0 ms |
| `convert` sysml → ttl | 1.1 ms | 1.4 ms | 1.5 ms |
| `convert` ttl → sysml | 1.2 ms | 1.5 ms | 2.7 ms |

Reproduce with `make build-grpc && bin/sysml-grpc` and
`python clients/python/scripts/bench_latency.py --iterations 200`.

The shape matters more than the absolute numbers: a parse costs two orders of
magnitude more than a query on the parsed result, because it loads the standard
library into a fresh symbol index and runs the semantic passes. Everything else
here takes around a millisecond, of which the RPC itself (protobuf encode/decode
plus loopback) is a few hundred microseconds.

### Starting the service

A `Connection` that names no address starts a private `sysml-grpc` child, which
binds the port the kernel gives it and reports the address on stdout. There is no
readiness backoff to tune: the client waits for that one line, and the first
`GetServerInfo` (which it makes anyway, to check the release) doubles as the
readiness check, since the address is reported only once the listener is bound.
The wait for the line is bounded by `opensysml.connection.START_TIMEOUT` (2.5 s),
after which `ConnectionError` is raised quoting the last lines the child logged. A
child that exits instead of reporting closes stdout, and is reported when it exits
rather than at the timeout. Starting and reaching one costs ~7.0 ms p50 / ~9.1 ms p95 on
the machine above, and a second connection in the same interpreter joins that
child for ~0.6 ms. `Connection(auto_start=False)` costs ~0.3 ms and the first RPC
on it ~1 ms; `import opensysml` is ~120 ms, mostly `grpc` (~48 ms) and the
generated protobuf modules.

### Real-time analytics

This is a request/response service over gRPC, not a hard real-time engine. It
gives no deadline guarantee, and nothing in it is scheduled: the runtime's step
and time budgets bound how long an execution may run, which caps the worst case
rather than promising one. Latency tails come from the Go garbage collector, the
scheduler, TCP, and, on a cache miss, a parse whose cost scales with the
document. Treat the p99 above as a soft budget, and measure the p99 for the
relevant model sizes, cache state and concurrency before relying on it.

What that means in practice:

- **Reuse one `Connection`.** Channel setup plus the first parse is tens of
  milliseconds; the module-level functions already share a singleton.
- **Parse once, then query by hash.** Repeated `load` of unchanged content hits
  the service cache and costs ~0.5 ms, but asking the model itself (`model.eval`,
  `model.instantiate`, `model.execute_*`, which pass its hash) skips even that. The cache holds 100 models
  (`-cache-size`) and evicts least-recently-used, so a stream of distinct
  sources will evict a model whose hash is still held.
- **Batch.** One RPC carrying many samples beats one RPC per sample; at ~0.5 ms
  of overhead per call, a per-sample loop tops out in the low thousands of calls
  per second per connection.
- **Keep the hot loop local.** Filtering, thresholding and windowing over a
  telemetry stream belong in NumPy in the calling process. Use the service for the
  coarse-grained steps (resolving a model question, instantiating, running an
  action or state machine, converting a model), not for every sample.
- **Convert off the hot path.** Conversion re-parses its input on every call; it
  is not cached by content the way `load` is.

## Generated typed classes

`Instance` is dynamic, so an editor cannot complete `inst.mass` and a type checker
cannot reject `inst.mas`. `opensysml.generate` emits a Python class per SysML
definition, so both can:

```bash
python -m opensysml.generate internal/repl/testdata/vehicle_package.sysml -o demo_types.py
opensysml-generate model.sysml -o model_types.py     # same thing, as a console script
```

```python
import opensysml
from demo_types import Vehicle

model = opensysml.load("internal/repl/testdata/vehicle_package.sysml")
inst = model.instantiate("Demo::Vehicle")

v: Vehicle = Vehicle.from_instance(inst)   # a typed view over the Instance
v.mass                                      # 1500.0, typed float
v.engine.power                              # 300.0, through the generated Engine
v.instance                                  # the underlying Instance
```

mypy (or pyright) then reports `v.mas` as an unknown attribute and `v.mass + "x"`
as an unsupported operand pair. `opensysml` ships a `py.typed` marker, so its own
annotations are used too.

`from_instance` rejects an instance of another definition, naming both types,
rather than failing later at attribute access. An instance of a definition that
specializes the expected one is accepted, since its generated class derives from
the expected class. An instance whose type no generated class describes is
accepted: instantiating a *usage* reports the usage's own FQN (`Demo::myCar`, not
`Demo::SportsCar`), which the client cannot relate to a definition, so rejecting
it would break the ordinary way to obtain an instance. `Vehicle.unchecked(inst)`
is the explicit escape hatch for a deliberately unchecked view.

### Keeping a generated module honest

A generated module records what it was generated from, so a stale one can be
detected up front rather than discovered at attribute access (or never, when a
removed feature still type-checks):

```python
SYSML_GENERATOR_VERSION = "1"   # emission schema of this generator
SYSML_MODEL_HASH = "sha256:…"   # hash of the model source it was generated from
```

`--check` regenerates in memory and compares, writing nothing:

```bash
python -m opensysml.generate model.sysml -o model_types.py --check   # exits 1 if stale
```

It exits non-zero when the module is missing or would change, naming the command
that regenerates it, which makes it usable as a CI or pre-commit gate.

Generation requires a service that reports the `type_facts` capability, which it
asks for over `GetServerInfo`. A service too old to answer that RPC, or one that
answers without the capability, does not populate `SymbolInfo.type_info`, and
generating against it would type every feature as `object`, indistinguishable from
a feature that is genuinely untyped. Generation therefore fails, naming the service
in use, where it came from, and how to replace it, rather than emitting a silently
useless module.

The generator emits a **runtime `.py`**, not a `.py` + `.pyi` pair: each feature is
a property that carries the annotation and performs the delegation, so the types
and the code that implements them cannot drift apart, and there is one artifact to
commit. Output is deterministic (definitions ordered by fully-qualified name, base
classes first, and nothing environment-dependent written), so it can be committed
and diffed; `clients/python/tests/golden/vehicle_types.py` is exactly that.

Generated classes are views, not copies: attribute access goes to the underlying
feature value on every read, and Tier 1 behaviour is preserved. A feature value that
failed to evaluate raises `FeatureValueError`; one holding a value of a different type than the
model declared raises `TypeMismatchError` rather than returning a wrongly typed value. A feature holding
no value (Tier 1 `UNSET`) reads as `None` for a `0..1` property and as the empty list for a
collection property.

### SysML → Python mapping

| SysML | Python |
| --- | --- |
| `Real`, `Rational` | `float` |
| `Complex` | `complex`, one number rather than two floats |
| `Array` | `opensysml.Array`: `dimensions` and row-major `elements`, `nested()` unfolds them |
| `CartesianVectorValue` and the other numeric vectors | `opensysml.Vector`: a tuple of `int`/`float` components, kept apart |
| `VectorQuantityValue` | `opensysml.VectorQuantity`: a tuple of `Quantity`, one unit per component |
| `MeasurementUnit` and the other measurement references (`SI::m`, `m / s`, a quantity's `mRef`) | `opensysml.MeasurementRef`: the `Unit` with its reduction, and `unit_id` naming the declaration a named unit is (`SI::metre`), empty for a composed unit |
| a calc definition, calc usage or `in calc` parameter read as a value | `opensysml.Function`: `calc_id` naming the calc, `self_id` the object it was read off (0 for none) |
| `Integer`, `Natural` | `int` |
| `Boolean` | `bool` |
| `String` | `str` |
| usage typed by a definition that reduces to a library scalar (`attribute def Celsius :> Real`) | that scalar (`float`) |
| usage typed by an `enum def` | `EnumLiteral`, the identity of the literal held |
| usage typed by any other definition in the model | that definition's generated class |
| multiplicity `1`, `1..1`, or undeclared | `X` |
| multiplicity `0..1` | `X \| None` |
| `*`, `0..*`, `n..m` with upper > 1 | `list[X]` |
| `Number` | `object`, with a comment naming the type |
| a type resolved outside the model (e.g. a library type) | `object`, with a comment naming its FQN |
| an unresolved or absent type | `object`, with a comment naming what was written |
| `specializes`, `subsets` or `redefines` a definition in the model | Python base class |

The fallback is always `object` and always says why in the property's docstring;
no feature is given a type the model does not support, and `Any` is never used to
dodge one.

### Known limitations

- **Behavioral and connector usages.** Only structural usages (attribute, part,
  item, occurrence, individual, port, enum) become properties. Action, state,
  calc, constraint, requirement, connection, flow, interface, allocation and case
  usages are not instance feature values and are skipped.
- **Redefinition narrowing.** A redefinition reuses the redefined feature's name,
  so its property overrides the base class's, and takes over the type and
  multiplicity it does not restate; a redefinition that *narrows* the type is
  emitted with its own declared type, which Python does not check against the
  base property.
- **Multiple inheritance.** Emitted in declaration order, a target named twice
  appearing once, and a base another declared base already specializes is left
  implicit (`Hybrid :> Vehicle, Electric` with `Electric :> Vehicle` emits
  `class Hybrid(Electric)`). A hierarchy that linearizes no way at all keeps the
  bases it can and names the left-out edge in a comment, rather than emitting a
  module that fails to import.
- **Generics and enumerations.** No generic parameters. An `enumDef` becomes a plain
  class rather than a Python `Enum`; a usage typed by it is `EnumLiteral`, which
  carries the literal's declaration identity but does not enumerate its siblings.
- **Name collisions.** Two definitions with the same simple name both get
  path-qualified class names (`A_Thing`, `B_Thing`). A feature named like a member
  `TypedObject` provides (`instance`, `from_instance`, `sysml_id`) gets a trailing
  underscore (`instance_`); the SysML feature name it reads is unchanged.

`opensysml.connect(host, port, auto_start=True)` returns a caller-owned
`Connection`; the module-level functions share a lazily created singleton
connection instead. Naming a host and port, writing `host:port` as the host, or setting
`$OPENSYSML_SERVICE=host:port` opts in to a service this client does not
manage; with none of them, the connection reaches a private child of this
interpreter. A `host:port` address written as the host is read as one
(`connect("localhost:50123")` reaches port 50123), and a port named twice with
two different values raises `ValueError` naming the disagreement rather
than timing out against an address nobody asked for. The helpers taking
`host`/`port` (`load`, `evaluate`, `convert`, `instantiate`) read it the same way.

`opensysml` never stops a service it did not start, and never uses one it was not
pointed at. A private child is reference-counted within the interpreter that
started it (one child per release requirement, shared by that interpreter's
connections, stopped when the last is closed or the interpreter exits) and
dies with that interpreter however it dies, because it exits at end of file on a
stdin pipe only that process holds. Nothing about it is written to disk, so there
is no record to authenticate, no pid outside its own `Popen` to signal, and no
stale state for a later run to clean up. A connection to a caller-managed service
takes no reference and leaves it running, whatever it does.

A caller-managed service is checked in the same way as the cached binary: it is asked what
it is with `GetServerInfo`, and a release other than the one asked for raises
`StaleServiceError` naming the mismatch and the remedy, instead of serving an old
build whose first newer call fails as a `MissingCapabilityError`.
`connect(version=…, require_capabilities=[…])` asks explicitly;
`OPENSYSML_GRPC_VERSION` asks for a release for the binary cache and the running
service alike, and with neither set whatever answers is accepted. Such a service
is never stopped or replaced to satisfy the check — the remedy is to stop it,
point the client elsewhere, or accept what is running — and the check stays
lazy: a caller-managed service that is not yet listening is checked once it answers. A
private child cannot be a mismatch to begin with, since children are held per
requirement, so a connection asking for another release starts its own rather
than joining one. A service that only lacks a required *capability* is reported as
`MissingCapabilityError`: capabilities come with a release, so the class raised
does not depend on who started the service.

The client checks the advertised list before it makes a capability-gated call, so
that error usually arrives without a round trip. When a call does reach a service
that lacks the capability, the service refuses it with `UNIMPLEMENTED` naming the
capability, and the client raises the same `MissingCapabilityError` with the gRPC
error kept as its `__cause__` — so the class raised does not depend on which
side detected the problem either. A capability that only describes how a response is
populated is not a refusal: the answer omits the fields it names, as documented
per call.

## Development

```bash
pytest clients/python/tests/                    # unit tests
pytest -m integration clients/python/tests/     # needs a running sysml-grpc

# Tests needing a service skip without one; CI sets this so an absent service
# fails the run instead of quietly passing.
OPENSYSML_REQUIRE_SERVICE=1 pytest clients/python/tests/

# Regenerate the committed golden generated file (needs a running sysml-grpc)
python -m opensysml.generate internal/repl/testdata/vehicle_package.sysml \
    -o clients/python/tests/golden/vehicle_types.py

# Regenerate protobuf bindings (from the repository root)
pip install grpcio-tools
make python-proto
```

## Modules

- `binary.py` — locates, downloads and checksum-verifies `sysml-grpc`
- `connection.py` — gRPC channel, service lifecycle, ownership of services it started
- `model.py` — a parsed model: root symbol and diagnostics
- `symbol.py` — lazy symbol proxy, fetches children on demand
- `instance.py` — instantiated object and its feature values
- `conversion.py` — a written model, its formats, extension inference, and the
  `ExperimentalFeatureWarning` an RDF conversion raises
- `query.py` — the standard's Query payload, translated and its answers
- `document.py` — native document queries: typed bindings, typed rows, and
  `model.render_document`'s Markdown
- `verdict.py` — a verification's answer and what a calculation computed
- `exploration.py` — every outcome a run under `explore` reached, each with its
  linearization count and a witness, and whether the search completed
- `errors.py` — the exception hierarchy and the gRPC status translation
- `capabilities.py` — what the connected service reports it supports
- `typefacts.py` — a symbol's static type, multiplicity and supertypes
- `typed.py` — base class and feature-value decoders the generated classes are built on
- `generate.py` — emits typed classes from a parsed model
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
