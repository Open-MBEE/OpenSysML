# 9. From your own program

Everything the REPL and the editors do is available to a program. There are five ways to reach the
engine, and all of them go through the same implementation, so a model reads the same whichever
one you use. The Go API calls the engine in the process that imports it; the Python,
Node/TypeScript, Java and Rust clients call the `sysml-grpc` service, which each of them
starts and stops on its own.

| Language | Package | Reaches the engine by | Reference |
| --- | --- | --- | --- |
| Go | `github.com/Open-MBEE/OpenSysML/client/opensysml` | in process, or Connect to a service | [Go packages](../reference/api.md) |
| Python | `opensysml` | gRPC, to a private child service or a named one | [Python API](../reference/python-api.md) |
| Node/TypeScript | `@opensysml/client` | Connect, from Node or from a browser page | [Node API](../reference/node-api.md) |
| Java | `org.openmbee:opensysml-client` | Connect, over the JDK's own HTTP client | [Java API](../reference/java-api.md) |
| Rust | `opensysml` | Connect, blocking, with no async runtime | [Rust API](../reference/rust-api.md) |

They do not all cover the same ground. Go and Python expose every RPC the service offers. Node,
Java and Rust cover a smaller v1 surface (parse, look up a symbol, evaluate, instantiate), and of
those three only Node has an escape hatch to the rest, through the generated Connect client it
exposes. Only Python and Go are published so far.
[Client libraries](../reference/clients.md) lays out what each covers and how to choose;
[the troubleshooting chapter](10-troubleshooting.md) covers runs that stop short.

## The same task, five ways

One model, one task, in each language: parse a file, evaluate an attribute against an object, look
up a definition, instantiate it. Pick your language in the tabs; the sections after them go into
what only that client has.

```sysml
// vehicle.sysml
package Demo {
  part def Vehicle {
    attribute mass = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1800.0;
  }
}
```

### Install it

=== "Go"

    ```sh
    go get github.com/Open-MBEE/OpenSysML@latest
    ```

    Nothing else: the engine and the SysML standard library are in the module.

=== "Python"

    ```bash
    pip install opensysml
    export OPENSYSML_GRPC_VERSION=latest   # or install sysml-grpc yourself
    ```

=== "Node"

    ```bash
    npm install @opensysml/client          # once the first release is published
    export OPENSYSML_GRPC_VERSION=latest
    ```

=== "Java"

    ```xml
    <dependency>
      <groupId>org.openmbee</groupId>
      <artifactId>opensysml-client</artifactId>
      <version>0.1.0-SNAPSHOT</version>
    </dependency>
    ```

    Not published yet: `make build && mvn -f clients/java/pom.xml install` from a checkout.

=== "Rust"

    ```toml
    [dependencies]
    opensysml = { git = "https://github.com/Open-MBEE/OpenSysML.git", branch = "main" }
    ```

    Not published yet, so take it from git or a path. Rust 1.83 or later.

The four service clients each need a `sysml-grpc` binary to start, and they share the cache they
find one in — `~/.opensysml/bin/sysml-grpc`, then `$PATH`, with a release downloaded into that
cache when `$OPENSYSML_GRPC_VERSION` asks for one. An explicit path comes first, from a variable
that differs by client: `$OPENSYSML_BINARY` for Python and Node, `$OPENSYSML_GRPC_BINARY` for Java
and Rust. [Getting the service binary](#getting-the-service-binary) gives the five ways to provide
one. The Go API needs none: it is the engine.

### Parse, evaluate, look up, instantiate

=== "Go"

    ```go
    client, err := opensysml.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    model, err := client.ParseFile(ctx, "vehicle.sysml")
    mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
    vehicle, err := client.LookupSymbol(ctx, model, "Demo::Vehicle")
    built, err := client.Instantiate(ctx, model, "Demo::Vehicle")

    fmt.Println(mass)                       // 1800
    fmt.Println(vehicle.Kind, vehicle.ID)   // partDef Demo::Vehicle
    fmt.Println(built.Root.TypeSymbolID)    // Demo::Vehicle
    ```

=== "Python"

    ```python
    import opensysml

    model = opensysml.load("vehicle.sysml")

    print(model.eval("mass", subject="Demo::sedan"))    # 1800.0
    vehicle = model.get("Demo::Vehicle")
    print(vehicle.kind, vehicle.id)                     # partDef Demo::Vehicle
    built = model.instantiate("Demo::Vehicle")
    print(built.type_symbol_id, built.mass)             # Demo::Vehicle 1500.0
    ```

=== "Node"

    ```ts
    import { load } from "@opensysml/client";

    await using model = await load("vehicle.sysml");

    await model.eval("mass", { subject: "Demo::sedan" });  // { kind: "real", value: 1800 }
    const vehicle = await model.symbol("Demo::Vehicle");
    vehicle.kind; vehicle.id;                              // partDef, Demo::Vehicle
    const built = await model.instantiate("Demo::Vehicle");
    built.root.typeId;                                     // Demo::Vehicle
    built.get("mass");                                     // { kind: "single", value: … }
    ```

=== "Java"

    ```java
    try (Connection connection = Connection.open()) {
      Model model = connection.load(Path.of("vehicle.sysml"));

      Value mass = model.evalWithSubject("mass", "Demo::sedan");  // RealValue[value=1800.0]
      Symbol vehicle = model.symbol("Demo::Vehicle");             // partDef, Demo::Vehicle
      Instantiation built = model.instantiate("Demo::Vehicle");

      System.out.println(built.root().typeSymbolId());            // Demo::Vehicle
    }
    ```

=== "Rust"

    ```rust
    use opensysml::{load, EvalOptions};

    let model = load("vehicle.sysml")?;

    let subject = EvalOptions { subject: Some("Demo::sedan".into()), ..Default::default() };
    let mass = model.evaluate("mass", &subject)?;   // Real(1800.0)
    let vehicle = model.symbol("Demo::Vehicle")?;   // partDef, Demo::Vehicle
    let built = model.instantiate("Demo::Vehicle")?;

    println!("{}", built.instance.type_symbol_id());  // Demo::Vehicle
    ```

Names differ, answers do not: one engine parses the file and one runtime computes `mass`, so
`1800.0` is the same number arrived at the same way whichever tab you read. Broken source is not a
failed call in any of them — it parses, and the errors arrive as the model's diagnostics.

### When a call goes wrong

Each client keeps two failures apart, because the service does: a call that was **refused** (no such
symbol, a capability missing, a service that would not start) and a call that **succeeded** with an
answer reporting a model failure (an expression that will not evaluate). A false verdict from a
constraint is neither — it is an answer about the model.

=== "Go"

    `*StatusError` for the first (`errors.Is(err, opensysml.CodeNotFound)`), `*FailureError` for the
    second (`errors.Is(err, opensysml.ErrFailure)`).

=== "Python"

    `ServiceError` for the first, with a subclass per status a caller acts on
    (`ModelNotFoundError`, `ServiceTimeoutError`, …); `ExecutionError`, `SymbolNotFoundError` and
    `ModelError` for the second. Everything descends from `OpenSysMLError`, and each also inherits
    the built-in it stands in for, so `except KeyError` catches a missing symbol.

=== "Node"

    `ServiceError` for the first, carrying the Connect `code`; `EvaluationError` and
    `SymbolNotFoundError` for the second. Both are `instanceof OpenSysMLError`.

=== "Java"

    `ServiceException` for the first, `ModelException` for the second, both unchecked and both
    `OpenSysMLException`. `TransportException`, `CapabilityException` and `ServiceStartException`
    name the specific refusals.

=== "Rust"

    One `Error` enum: `Error::Service` for the first, `Error::Model` for the second.

## From Go

A Go program that imports OpenSysML links the parser, the semantic engine and the runtime, so
`opensysml.New()` calls them directly — no port, no child process, no serialization:

```go
client, err := opensysml.New()
if err != nil { ... }
defer client.Close()

model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
inst, err := client.Instantiate(ctx, model, "Demo::Vehicle")
```

```sh
go get github.com/Open-MBEE/OpenSysML@latest
```

Nothing else needs installing: the SysML standard library is embedded in the module and no
operation shells out. Every RPC the service offers is a method (`ParseFiles` for a model made of
several documents, `ExecuteAction` and `ExecuteState`, `VerifyConstraint`, `VerifyRequirement`,
`VerifySatisfaction`, `EvaluateCalc`, `RunAnalysis`, `Query`, `RunDocumentQuery`, `RenderDocument`,
`Convert` and `ApplyEdits`), and queries and edits are built from typed values rather than a string dialect, so
an unsupported operator is a compile error rather than a refused call.

`Dial("host:50051")` is the other constructor, for a shared `sysml-grpc` that someone else runs.
This package never spawns a service of its own, because a private child would only be serving the
same code `New` already calls directly.

A call fails in exactly one of two ways, and the distinction comes from the wire contract: a
refused call is a `*StatusError` (`errors.Is(err, opensysml.CodeNotFound)`), while an answer that
reports a failure is a `*FailureError` (`errors.Is(err, opensysml.ErrFailure)`). Syntax errors are
neither: parsing broken source succeeds, and the errors show up in `Model.Diagnostics`. Likewise, a
false verdict is an answer about the model, not an error. A trade study's `RunAnalysis` reports
each alternative's evaluation in `Analysis.Evaluations` (`Selected()` for the one picked, resolved
to its object by `Analysis.Instance`), and a run that fails part way is an `*AnalysisError` whose
`Partial` keeps what it computed.

[The Go package reference](../reference/api.md) documents the surface type by type, and
[client/opensysml/README.md](https://github.com/Open-MBEE/OpenSysML/blob/main/client/opensysml/README.md)
states its concurrency, ownership and stability promises.

## From Python

`opensysml` is the Python client. Every call goes through the `sysml-grpc` service, which the
client starts and stops automatically, so a script can parse, inspect, execute and convert a model
without running `sysml` as a subprocess. The complete API, the generated typed classes and
the measured latency are documented in [reference/python-api.md](../reference/python-api.md).

### Installation

```bash
pip install opensysml             # from PyPI
pip install -e clients/python/          # or from a checkout, at the repository root
```

The dependencies (`grpcio`, `protobuf>=7.35.1`, `filelock`, `psutil`) are installed with it.
Those projects publish wheels for CPython 3.10 and later only, which is the range
`requires-python` declares.

### Getting the service binary

Every call goes through `sysml-grpc`. The client starts an instance of the service, looking for
the binary in this order: `$OPENSYSML_BINARY`, then
`~/.opensysml/bin/sysml-grpc` (`.exe` on Windows), then a release download into that cache if
one was requested, then `sysml-grpc` on `$PATH`. There are five ways to provide it:

```bash
# 1. Download the release build (verified against the digest pinned in opensysml)
python -c "from opensysml.binary import download_binary; download_binary('latest')"

# 2. Have opensysml.connect() download it on first use
export OPENSYSML_GRPC_VERSION=latest      # or a tag like v0.0.5

# 3. Build from source
make build-grpc && mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/

# 4. Name a build to start, wherever it is
export OPENSYSML_BINARY=$PWD/bin/sysml-grpc

# 5. Install one on $PATH, with a package manager or `go install`
```

If none of these is in place, `connect()` raises `ConnectionError` listing everywhere it looked,
rather than downloading anything you did not ask for. A binary from `$OPENSYSML_BINARY` or `$PATH`
belongs to no release, so it is started as found: not copied into the cache and not verified
against the pinned digests described below. If `$OPENSYSML_BINARY` names something that is not
executable, that is an error, not a reason to look elsewhere. `OPENSYSML_GITHUB_REPO` overrides
the repository releases are fetched from (default `Open-MBEE/OpenSysML`).

A download records its release tag, repository and digest beside the binary
(`~/.opensysml/bin/sysml-grpc.json`). That way a cache left by an earlier release, or by another
repository publishing the same tag, is replaced rather than handed to a client asking for a newer
one. Without this check an older build would answer and the call would fail with a
`MissingCapabilityError` naming a capability the requested release does provide. The check only
runs when a release is requested, so when `OPENSYSML_GRPC_VERSION` is unset a manually installed
binary (option 3) is left alone. If the requested release cannot be downloaded, because there is no
asset for the platform or no network, the cached binary keeps serving and a warning says so rather
than the connection failing.

A download is verified against the SHA-256 digest that `opensysml` pins for that release, not
against the `.sha256` file served beside the binary. That sidecar comes from whoever served
the binary, so it catches corruption but not a republished release. A release this
`opensysml` pins no digest for is refused, with the version named, and a working cached binary is
kept rather than trusting the served checksum. Setting
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for any repository; a fork's releases
do not need this) explicitly accepts same-origin trust for the named repository, with a warning.
The binary is cached in `~/.opensysml/bin`. A service started by the client is a private child of
the interpreter and keeps no state on disk, so `OPENSYSML_STATE_DIR`, which used to relocate the
service records, no longer has any effect.

Releases up to v0.0.4 contain only the `sysml` and `sysml-lsp` archives. `sysml-grpc` binaries are
published from the next release onward; until then, build the service from source
(option 3).

### A first script

```python
import opensysml

model = opensysml.load("model.sysml")
for d in model.diagnostics:
    print(d)

print(model.eval("1 + 2 * 3"))
```

Evaluation is a method on the model, like every other operation, so a script never has to carry
the model hash back to the connection. `model.eval(expr, context_symbol_id=…)` resolves the
expression's names in that element's scope.

### Requiring a usable model

A model with syntax errors still parses to a `Model`, because the service reports what it
could read along with the diagnostics. A script that does not look at those diagnostics is
therefore querying a model with declarations missing. To require a valid model instead:

```python
model = opensysml.load("model.sysml")
model.ok                       # False when any diagnostic is an error
model.errors                   # only the error-severity diagnostics
model.raise_for_errors()       # raises ModelError, or returns the model

model = opensysml.load("model.sysml", strict=True)   # raises instead of returning
```

`strict=True` is available on `opensysml.load`, `Connection.load` and
`Connection.load_from_content`. The `ModelError` it raises carries the errors as
`.diagnostics` and the model itself as `.model`, so a caller that wants to report
them does not have to load twice:

```python
try:
    model = opensysml.load("model.sysml", strict=True)
except opensysml.ModelError as exc:
    for d in exc.diagnostics:
        print(d)
    partial = exc.model            # what the service did parse
```

`strict_conformance=True`, available on the same three calls, answers a different question:
whether the source is conforming SysML v2. OpenSysML's own notation, such as `defer` and the
pseudostates, is then reported as an error rather than a warning
([3. Strict conformance](03-command-line.md#strict-conformance)). The two settings are
independent: `strict` decides whether errors raise, and `strict_conformance` decides what
counts as an error.

```python
model = opensysml.load("model.sysml", strict_conformance=True)
model.ok                       # False if the model uses OpenSysML notation
```

A service that predates the field raises `MissingCapabilityError` rather than silently answering
the default question. Support is advertised as `strict_conformance` in
`Connection.server_info().capabilities`.

### Inspecting symbols

`Model.find` takes a **short** name and searches the symbol tree, returning `None` when there is
no such symbol. `model["Vehicle"]` is the raising counterpart: it names the missing symbol, whereas
chaining off `find`'s `None` would fail one call later with `AttributeError: 'NoneType' object
has no attribute 'attributes'`.

```python
vehicle = model["Vehicle"]        # SymbolNotFoundError if absent; also a KeyError
vehicle.attributes()              # [Symbol(id='Demo::Vehicle::mass', kind='attributeUsage')]
vehicle.parts()                   # [Symbol(id='Demo::Vehicle::engine', kind='partUsage')]
vehicle.get_attr("mass")          # Symbol, or None if there is no such attribute

model.find("Nope")                # None — for asking whether a symbol exists
"Vehicle" in model                # True
model["Vehcile"]                  # SymbolNotFoundError: ... did you mean 'Vehicle'?
```

The subscript accepts a short name or a fully-qualified name. `model.get("Demo::Vehicle")` looks a
symbol up by fully-qualified name only and returns `None` if it finds nothing.

A symbol also carries its static type facts, as the service resolved them:

```python
engine = model.get("Demo::Vehicle::engine")
engine.type_facts        # TypeFacts(declared='Engine', resolved_id='Demo::Engine', ...)
engine.multiplicity      # Multiplicity(lower='0', upper='1'), or None if undeclared
engine.specializations   # [Specialization(kind='typing', target_id='Demo::Engine', ...)]
```

### Instances

Feature values come back as Python values, and a feature value that holds an object comes back as
a nested `Instance`:

```python
inst = model.instantiate("Demo::Vehicle")

inst.mass                 # 1500.0
inst["mass"]              # 1500.0
inst.engine               # Instance(id=2, type='Demo::Engine', features=1)
inst.engine.power         # 300.0
inst.features             # {'mass': 1500.0, 'engine': Instance(...)}
inst.get("missing", 0)    # 0
```

Integers, reals, booleans, strings and sequences map to `int`, `float`, `bool`, `str` and `list`.
Unknown names raise `AttributeError` for attribute access or `KeyError` for item access, so
`hasattr`, `copy` and `pickle` behave as expected.

A feature value that holds nothing, such as a feature declared `attribute d : Real;` with no
value, reads as `opensysml.UNSET`, which `%features` and `-instantiate` show as
`<unset>`. It is falsy and is not `None`, which still stands for the model's `null`:

```python
inst.d is opensysml.UNSET   # True
inst.d is None            # False
```

The service expands the object graph to depth 8 and stops at a type already present on the path,
so a part that contains its own kind terminates. A child that was not expanded comes back as its
bare integer id rather than an `Instance`.

The raw protobuf is still reachable: `get_feature(name)` returns the `FeatureValue` message, and
`raw_features` is the complete map.

```python
inst.get_feature("mass").materialized         # True
inst.get_feature("engine").value.instance_id  # 2
```

A feature value the service could not evaluate, such as a cyclic derived attribute, is never
reported as `None`. Attribute and item access raise `FeatureValueError`, while `features` carries
the `FeatureValueError` as that entry's value so the rest of the instance stays inspectable.

```python
cyclic.a               # raises FeatureValueError: feature value 'a': ... cyclic feature value dependency
cyclic.features["a"]   # FeatureValueError(...)
```

`FeatureValueError` is not an `AttributeError`, so `hasattr` on such a feature propagates it
rather than returning `False`. Use `features` to inspect an instance whose feature values may have
failed to evaluate.

`eval` returns a single value, so a result the wire format cannot represent raises
`UnsupportedValueError` rather than being reported per entry.

Instances are capability-negotiated the same way as conversion: a service that does not report
the `feature_values` capability (every release before 0.1.0) raises `MissingCapabilityError`
naming the required upgrade, rather than returning an object whose values all appear to be
missing.

Actions and state machines run on the model too:

```python
model.execute_action("Demo::addFive", inputs={"result": 10})   # {'result': 15}
model.execute_state("Demo::Machine", events=["go"])            # {'states_visited': [...], ...}
```

`execute_action` and `execute_state` treat their result maps the same way:
a value the wire format cannot represent is reported as an
`UnsupportedValueError` in that entry, leaving the other entries intact.

Every call about a loaded model is a `Model` method. The module-level
`opensysml.instantiate`, `opensysml.evaluate` and `opensysml.convert` are still available for
working directly from a file (`opensysml.instantiate("Demo::Vehicle",
file_path="model.sysml")`) or from a hash obtained elsewhere (`model_hash=…`).

### Verifying constraints, requirements, satisfaction, calculations and analyses

The checks the REPL performs with `%constraint`, `%requirement`, `%satisfy`, `%calc` and `%analysis` are also
available as RPCs, so a script can find out whether a model satisfies its requirements.
These calls use the same runtime evaluation the REPL does, not a second implementation.

```python
model = opensysml.load("lander.sysml", strict=True)

for verdict in model.verify_satisfaction():        # every assert satisfy … by …
    print(verdict)
# ✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)
# ✗ satisfy touchdown by fastLander fails (on Landing::fastLander ID: 2): condition
#   evaluated to false: lander.verticalSpeed <= maxVerticalSpeed

model.satisfied()                                  # False — one assertion fails
model.verify_satisfaction("Landing::analysisContext")   # only what that element asserts
```

Constraints and requirements are named directly, optionally with a subject to instantiate, so that
the verdict is about that object's values rather than the declared defaults:

```python
model.verify_constraint("Demo::Vehicle::massOK", subject="Demo::sedan")
model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::sedan")
```

A `Verdict` is truthy when the condition holds, and reports the reason when it does not:

```python
verdict = model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::truck")

bool(verdict)          # False
verdict.holds          # False
verdict.condition      # 'mass < 2000.0' — the condition that evaluated to false
verdict.element        # 'Demo::Vehicle::lightEnough', or the assertion as written
verdict.kind           # 'constraint', 'requirement' or 'satisfy'
verdict.instance_id    # the object the verdict is about, 0 for declared defaults
verdict.instances      # the objects the call built, as Instances
verdict.diagnostics    # diagnostics the service reported for the run
print(verdict.explain())
```

**A false verdict is a result, not an exception.** A condition that evaluated to false is the
answer to the question you asked, so it is returned. Only a failure to evaluate at all, such as an
unbound feature or an exhausted step budget, is a malfunction, and such a failure is not
reported as a failing verdict. It appears as `verdict.error` with `verdict.evaluated` set to
`False`, and `verdict.raise_for_error()` turns it into an `ExecutionError` for scripts that must
not mistake it for a failed requirement:

```python
for verdict in model.verify_satisfaction():
    verdict.raise_for_error()      # nothing raised for a verdict of false
    if not verdict:
        print(verdict.explain())
```

Where the model states verification cases for a requirement, the verdict their bodies produced is
reported beside the requirement's own — `verdict.holds` still says what the requirement engine
decided, and a failing verification body does not change it:

```python
verdict = model.verify_requirement("Landing::touchdown")
verdict.holds                        # True — the requirement is satisfied
[(v.case_id, v.kind) for v in verdict.verifications]
# [('Landing::checkSlow', 'pass'), ('Landing::checkFast', 'fail')]
bool(verdict.verifications[0])       # True — a VerificationVerdict is truthy when it passed
print(verdict.verifications[1].explain())
```

A `kind` of `"pass"` or `"fail"` is what the library's own `VerificationCases::PassIf` calculation
computed over the run of the case body, `"inconclusive"` a body that produced no verdict value and
`"error"` one whose run could not be carried out, with `detail` carrying the reason. `subcase` is
true for a case another case performed as a step. `run_analysis` accepts a verification case and
reports the same list, and `verify_satisfaction`'s verdicts carry it too. A service that does not
advertise `verification_verdicts` (`opensysml.CAPABILITY_VERIFICATION_VERDICTS`) reports none.

A request that cannot be answered at all, such as one naming an unknown symbol or a subject that
cannot be instantiated, raises `ExecutionError` from the call itself rather than returning a
verdict. Narrowing to an element that makes no satisfaction assertion is not such a request: it
returns no verdicts, and `satisfied()` is then vacuously `True`.

**Naming an element of the wrong kind is an invalid request, not a verdict.** Asking whether a
part def holds as a constraint raises `WrongKindError`, a subclass of `ExecutionError`, from
`verify_constraint`, `verify_requirement`, `verify_satisfaction` and `calc`, exactly as naming a
nonexistent element does. A caller reading `.holds` is therefore never told the model does
not hold when the real problem is that a part def was named:

```python
model.verify_constraint("Demo::Wheel")
# opensysml.errors.WrongKindError: not a constraint: Demo::Wheel is a part def,
# not a constraint definition or usage
```

The kind is read from a typed `failure_reason` reported by the service, never from the message
text.

`verify_satisfaction` evaluates many assertions in one call and reports a single object graph for
all of them, so `verdict.instances` holds every object the call built. Pick out the object a
verdict is about using its `instance_id`:

```python
subject = next(i for i in verdict.instances if i.id == verdict.instance_id)
```

Calculations are invoked with positional arguments. A calc *usage* named without arguments is
evaluated from its own members and reports every output feature it computes (SysML 7.17):

```python
model.calc("Demo::add", arguments=[2.5, 4.0]).value    # 6.5
model.calc("Demo::c").outputs                          # {'a': 6, 'b': 10}
```

An analysis case is run the way `%analysis` runs one: a usage that binds its `subject` needs
nothing more, a definition (or a usage binding none) takes the subject to instantiate, and
`arguments` and `named_arguments` bind its other `in` parameters. The result carries the case's
`out` and `return` values, one verdict per `objective` and per `assert constraint` in the body,
and the subject's objects; it is truthy when every verdict was decided and holds:

```python
run = model.run_analysis("An::shipCost")
run.outputs                                            # {'total': 12.0}
run.verdicts[0].holds                                  # True — objective affordable

run = model.run_analysis("An::CostAnalysis", subject="An::barge",
                         named_arguments={"limit": 50.0})
run.outputs["total"], bool(run)                        # (37.0, True)

run = model.run_analysis("An::CostAnalysis", subject="An::barge")
bool(run), run.verdicts[0].condition                   # (False, 'total <= limit')
```

An objective that could not be decided — a feature it reads has no value — is a verdict with
`error` set and `evaluated` false, as a verification's is, and `raise_for_error()` turns it into
an `ExecutionError`. A definition run with no subject, an `in` parameter left without a value, a
step that fails and a case that runs itself raise `ExecutionError` from the call; naming a symbol
that is not an analysis raises `WrongKindError`, and so does asking `calc` to run an analysis.

A trade study (`TradeStudies::TradeStudy`) runs the same way and reports in addition each
evaluation the run made of its `evaluationFunction` — one per alternative the subject lists, in
subject order, as a `CaseEvaluation` carrying the alternative it scored (an `Instance`), the
result or the error, and whether it was the one selected (`selected`) or scored the same without
being (`tied`); `run.selected` is the selected ones. A service that does not advertise
`case_evaluations` (`opensysml.CAPABILITY_CASE_EVALUATIONS`) reports none:

```python
run = model.run_analysis("Trade::lightest")
run.outputs["selectedAlternative"].type_symbol_id          # 'Trade::b'
[(e.arguments[0].type_symbol_id, e.result, e.selected, e.tied) for e in run.evaluations]
# [('Trade::a', 30.0, False, False), ('Trade::b', 10.0, True, False), ('Trade::c', 10.0, False, True)]
run.selected[0].arguments[0].id == run.outputs["selectedAlternative"].id   # True
```

An `evaluationFunction` that fails for one alternative fails the run as any failing step does,
with `AnalysisRunError` — an `ExecutionError` whose `result` keeps what the run computed before
it failed: the evaluations made, the earlier ones with their results and the failing one with its
`error`, and the objective undecided. Swept, that run is a row keeping the same beside its own
`error`.

A parameter can be swept rather than fixed: `run_sweep` runs the analysis case or calc once per
value of each range and returns a `SweepTable`, whose rows carry the inputs bound for that run,
its outputs and verdicts, and the seconds it took. Ranges are `{parameter: (from, to)}` or
`{parameter: (from, to, step)}`; several of them run their cartesian product, the first varying
slowest, and `samples` with `seed` draws that many values per range instead:

```python
table = model.run_sweep("An::CostAnalysis", {"limit": (10.0, 50.0, 20.0)}, subject="An::barge")
[row.inputs["limit"] for row in table]                 # [10.0, 30.0, 50.0]
table[0].outputs["total"], table[0].verdicts["affordable"]

drawn = model.run_sweep("An::CostAnalysis", {"limit": (10.0, 50.0)}, subject="An::barge",
                        samples=8, seed=42)
drawn.sampled, drawn.seed, len(drawn)                  # (True, 42, 8)
```

A run that fails is a row of its own — falsy, with `error` set — rather than an exception, and
the runs after it are still made, so `table.failures` collects them; `raise_for_error()` on a row
turns one into an `ExecutionError`. A plan the service refuses raises `ExecutionError` instead,
naming what is wrong with it: a step of zero, a step whose sign never reaches its end, a Real
range with no step, incompatible units, a parameter the target declares none of, a case's
subject, one the arguments already bind, a distribution asked for by name, and a plan asking for
more runs than the service's budget allows. A row's verdicts resolve against `table.instances`,
which carries the objects the runs were about, so a failing row's subject can be read. A row of
a trade study carries that run's `evaluations` and `selected` as `run_analysis`'s result does,
so a table shows where the pick changes as the parameter moves.

Verification is capability-negotiated the same way as conversion: against a service that does
not report the `verification` capability, these calls raise `MissingCapabilityError` naming the
required upgrade rather than failing on an unimplemented method.

### Errors

Every failure a caller can act on is an `OpenSysMLError`. The service's gRPC status codes are
translated at the client boundary, so a script never needs to `import grpc` and switch on status
codes. The original `grpc.RpcError` is still available as `__cause__` for its debug string.

```
OpenSysMLError
├── ConnectionError            service unreachable or would not start (UNAVAILABLE)
│   ├── StaleServiceError      another release is already listening on that address
│   └── ChecksumMismatchError  a download contradicts the digest pinned for it
│       └── UnpinnedReleaseError  this opensysml pins no digest for that release
├── ServiceError               any other status the service failed a call with
│   ├── ModelNotFoundError     the model hash is no longer in the service cache
│   ├── ModelFileNotFoundError the service could not read the path (also FileNotFoundError)
│   ├── InvalidRequestError    request rejected as malformed (also ValueError)
│   ├── ServiceTimeoutError    deadline exceeded or cancelled (also TimeoutError)
│   └── UnsupportedOperationError  the service does not implement the call
├── ExecutionError             eval/instantiate/execute/verify failed (also RuntimeError)
│   └── WrongKindError         the call named an element of another kind than it asks about
├── ModelError                 strict load of a model with error diagnostics
├── SymbolNotFoundError        model["Nope"] (also KeyError)
├── FeatureValueError          a feature value could not be evaluated
├── ConversionError            the model could not be written in that format
├── UnsupportedValueError      a value the wire format cannot represent
├── TypeMismatchError          a feature value contradicts its generated view
├── InstanceTypeError          a typed view was given an instance of another type
└── MissingCapabilityError     the connected service does not report the capability
```

`ServiceError.code` is the underlying `grpc.StatusCode`. A status this client does not recognize
is still delivered as a `ServiceError`, so nothing escapes the hierarchy.

The two most common failures are both `NOT_FOUND` on the wire but need different fixes, so the
client tells them apart from what the service reports:

```python
opensysml.load("/tmp/nope.sysml")     # ModelFileNotFoundError: file not found: …
model.to_turtle()                   # ModelNotFoundError if the model was evicted
```

`ExecutionError` inherits from the built-in `RuntimeError`, so `except RuntimeError:` catches it.
A traceback reading `opensysml.errors.RuntimeError` used to suggest that but did not deliver it.
The old name is kept as a deprecated alias of `ExecutionError` (it is the same class, so
existing `except opensysml.errors.RuntimeError` clauses still work) and emits a
`DeprecationWarning` when accessed. Inheriting from the built-in, rather than only renaming, also
fixes existing code that never caught the old class, and the alias is left out of `__all__` so a
star-import no longer shadows the built-in.

#### Names that no longer shadow a built-in

Both names the package used to bind over a Python built-in were renamed before 0.2.0 was
published, so no deprecation cycle is needed:

| Old name | Use instead |
| --- | --- |
| `opensysml.eval` | `opensysml.evaluate` |
| `opensysml.RuntimeError`, `opensysml.errors.RuntimeError` | `opensysml.ExecutionError` |

Each old name still resolves to the same object (`opensysml.eval` *is* `opensysml.evaluate`
and `opensysml.RuntimeError` *is* `ExecutionError`, so existing snippets and `except` clauses
still work) and emits a `DeprecationWarning` when accessed. Neither appears in `__all__`, so
`from opensysml import *` no longer binds over `eval` or `RuntimeError`. The `Model.eval` and
`Connection.eval` *methods* keep their name, because an attribute of an object shadows nothing.

### Writing a model back out

A loaded model can be written back to SysML notation or RDF Turtle. The service does the
conversion with the same code as `sysml -convert`, so the client adds no second
implementation of the mapping.

The Turtle direction is [experimental](../reference/rdf-mapping.md#status-experimental): it
carries model structure and the behavior written in model bodies, refuses any construct it cannot
write back, and its vocabulary may change without a compatibility path. Any conversion that uses it
issues an `ExperimentalFeatureWarning` and sets `Conversion.experimental`. The notation direction
is stable and issues no warning.

```python
model = opensysml.load("model.sysml")

model.to_sysml()                 # Conversion: SysML notation
model.to_turtle()                # Conversion: RDF Turtle
model.save("model.ttl")          # writes Turtle; format taken from the extension
model.save("out.sysml")

opensysml.convert("ttl", file_path="model.sysml")            # without loading first
opensysml.convert("sysml", content=turtle, from_format="ttl")  # Turtle back to notation
```

A `Conversion` holds the output text, the formats converted between, and whether the mapping
used is `experimental`, with `experimental_notice` explaining why. `str()` and `len()` return the
text, and `write(path)` saves it. Formats are named `sysml`, `kerml`, `text`, `ttl`, `turtle` or
`rdf`. A file path's format is inferred from its extension; inline `content` has no extension, so
it needs `from_format`.

A `Model` writes out the source the service parsed, identified by `model.hash`, so editing the file
between `load` and `save` does not change what is written: the model saved is the model you
inspected. `convert(file_path=…)` is the alternative, and reads the file as it is now. The
parsed source lives in the service's bounded cache, so a model evicted since it was loaded
raises `ModelNotFoundError` rather than writing different content; load it again.

What each direction preserves:

- **Notation → notation** re-emits the model from its source, so comments and layout are
  preserved. It preserves source rather than printing an AST in general, so a model the client
  built element by element cannot be printed this way.
- **Notation → Turtle → notation** gives back an equivalent model rather than identical bytes.
  Comments do not survive the graph, because RDF has no place to record them. See
  [the RDF mapping](../reference/rdf-mapping.md) for what a graph must carry to support the
  return trip.
- Syntax errors normally fail the conversion and come back as a `ConversionError` carrying
  `diagnostics`. `tolerate_syntax_errors=True` writes notation anyway and reports the errors
  as `Conversion.diagnostics`. It applies to the notation → notation direction only, because every
  other direction builds a graph in which an unparsed declaration would be silently dropped.

Conversion is capability-negotiated: against a service that does not report the `convert`
capability, these calls raise `MissingCapabilityError` naming the required upgrade rather than
failing on an unimplemented method. For a service that does not report the RDF mapping's status,
the status is worked out from the formats it reports, so an RDF conversion warns either way.
Suppress the warning with `warnings.simplefilter("ignore",
opensysml.ExperimentalFeatureWarning)`; no stable feature uses that warning class.

### Changing a model and writing it back

A loaded model can be edited in place (setting the value of a feature, renaming a declaration)
and written back with its comments, blank lines and indentation intact. You describe the edit
rather than typing it out: the client sends operations naming elements by the same ids a read
reports, and the service applies them to the source it parsed.

```python
model = opensysml.load("spacecraft.sysml")

edit = model.edit()
edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
edit.rename("Demo::sc::margin", "massMargin")

result = edit.apply()             # re-parsed and validated service-side
result.save("spacecraft.sysml")   # every byte outside an edited span unchanged
```

`model.edit()` returns an `Editor`. `set_value(target, value)` replaces an existing `= <expr>` on
a feature, or adds one before the closing `;` when the feature has none; `value` is SysML
notation for a single expression, such as `"1050.0[SI::kg]"`, `'"flight-2"'`, `"true"` or
`"unitMass * count"`. `rename(target, new_name)` rewrites a declaration's name token and the
references to it. A target is a symbol id (its fully-qualified name, as reported by `Symbol.id`)
or a `Symbol` itself, so an element you found with a read can be edited by passing it back. Both
calls return the editor, so operations can be chained, and `len(edit)` counts them.

The same editor can add declarations:

```python
model = opensysml.loads("package Demo {}", strict=True)
result = model.edit().add_part_def("", "Vehicle").apply()
```

`add_member(owner, kind, name, type=None, multiplicity=None, value=None, specializes=None)`
accepts notation strings for the declaration. Typed `add_*` helpers cover the common SysML and
KerML kinds, and `delete(target, cascade=False)` removes declarations transactionally.

`apply()` sends the operations in a single call and returns an `EditResult`, which *is* a
`Conversion`: `str(result)` is the edited notation, and `result.save(path)` and
`result.write(path)` write it. `result.applied` lists the changes as
`AppliedEdit(operation_index, target, offset, length, old_text, new_text)` in source order, where
`length == 0` marks a value added to a feature that had none before.

How editing works:

- **Spans, not text search.** The value expression and the name token are located through
  the parsed model's own spans, so a comment or string that happens to contain the same text is
  never touched.
- **Bytes are spliced and nothing is reformatted.** Edits are applied right-to-left by offset in a
  single pass, and every byte outside an edited span is identical to the source the service
  parsed. Nothing is re-indented or re-wrapped.
- **The result is read back before it is returned.** The edited source is re-lexed, re-parsed and
  re-analysed. An edit that would introduce a syntax or name-resolution error is refused, with the
  diagnostics explaining why, and nothing is returned, so the service never produces a file its
  parser cannot read. Errors the model already had are not blamed on the edit; only
  errors the edit introduces cause a refusal.

Every refusal is a typed error, never a silent no-op:

| Situation | Error |
| --- | --- |
| No operation was added to the editor | `NoEditsError` |
| No such element, an ambiguous name, or an element that cannot carry a value or a name | `EditTargetError` |
| A new value that does not parse as one expression, or a new name that is not an identifier or already means something where the element is declared | `InvalidEditError` |
| A rename that would capture or shadow another name, at the declaration or at any reference | `InvalidEditError` |
| Two operations that would edit overlapping bytes | `OverlappingEditsError` |
| The edited model does not read back cleanly — a value naming something that does not resolve, say | `EditResultError` |

All of these are subclasses of `EditError`, which carries `failure` (the kind of refusal),
`diagnostics`, and `referring_elements` for a refused rename or a refused non-cascade delete. An
`EditResultError`'s diagnostics have spans in the edited text. `referring_elements` names
each namespace a reference is made from, telling you where to look rather than which
expression is at fault.

```python
try:
    model.edit().rename("Demo::SC::margin", "label").apply()
except opensysml.InvalidEditError as refused:
    print(refused.failure)   # 'EDIT_FAILURE_INVALID_NAME'
```

A rename rewrites the declaration's name token and every reference to it in the model's
source (the matching segment of a qualified name, an alias target, an import), so the model
keeps its original meaning.

These limitations are intentional:

- **A rename is refused where it would change what a name means.** A new name that already means
  something where the element is declared (as a sibling, or reached through an enclosing
  namespace, an import or a supertype), or that already means something at one of the references
  being rewritten, is refused. Such a rename would either be ambiguous or shadow an existing
  declaration, and either way unrelated expressions would start resolving to the renamed element.
  References from another file are not rewritten, because an edit sees only the source of the
  model it was given.
- **A model cannot be built from scratch in Python.** Declarations are added to and deleted from a
  model that is already loaded; there is no way to author one from nothing, and no object facade.
  A declaration is described by the notation arguments of an `add_*` call rather than by a mutable
  Python object.
- An editor is applied **once**. It describes an edit of the model it was created from, so
  applying it twice raises `RuntimeError`. Load the saved file and edit that instead.
- The parsed source lives in the service's bounded cache, so a model evicted since it was loaded
  raises `ModelNotFoundError` rather than editing different content; load it again.

Editing is capability-negotiated the same way as conversion: a service that does not report the
`apply_edits` capability raises `MissingCapabilityError` naming the required upgrade before any
call is made.

### Querying a model using the standard query model

`model.query(...)` runs the query defined by the **SysML v2 API & Services** standard, using
`scope`, `select` and `where`, so a payload written for that API works verbatim. This is the form
the API Cookbook notebooks and MATLAB System Composer's `executeQuery` send:

```python
model = opensysml.load("model.sysml")

model.query({"@type": "Query", "where": {
    "@type": "PrimitiveConstraint",
    "operator": "=", "property": "@type", "value": ["PartUsage"]}})
# [Demo::vehicle (PartUsage), Demo::vehicle::wheels (PartUsage)]

# The same query in keyword form, narrowed and projected
model.query(
    scope=["Demo::vehicle"],
    select=["name", "qualifiedName"],
    where={"operator": "=", "property": "@type", "value": ["PartUsage"]},
)
```

Each result is a `QueryElement` with `id` (the element's qualified name), `type` (its
metamodel type, for example `PartUsage`) and `properties`, the selected properties the element
has. A property an element does not have is absent rather than empty. An unnamed element, such as
a `doc` note or an anonymous usage, is not returned at all, because it has no qualified name to
identify or scope it by. `as_dict()` returns the result using the standard's JSON
names. `scope` accepts qualified names or the standard's `{"@id": …}` references and covers
each named element together with everything nested inside it; an empty scope covers the whole
loaded model.

A payload the standard does not describe, such as one with an unknown operator or a constraint
without a property, raises `QueryError` before anything is sent. A property the service does not
provide raises `InvalidRequestError` naming the properties that do exist, rather than returning no
results, and an evicted model raises `ModelNotFoundError`. As with every other call, gRPC status
codes stop at the client boundary. The query is capability-negotiated the same way as
conversion: a service that does not report the `query` capability raises `MissingCapabilityError`.

The standard's query model has **no graph traversal and no transitive closure**: "everything
under this part" is expressed as a `scope`, while "everything specializing this definition" cannot
be expressed at all. It is an interoperability surface, not OpenSysML's expressive query
facility; [the API reference](../reference/api.md) states exactly what is supported.

### Native document queries and rendered documents

`model.run_document_query(...)` runs a *document query* (a `calc def` specializing
`DocumentQueries::Query`) and `model.render_document(...)` renders a *document* (a `part def`
specializing `DocumentQueries::Document`), the way the REPL's `%run-query` and `%render-document`
do:

```python
model = opensysml.load("report.sysml")

result = model.run_document_query("Observatory::SubsystemTable")
result.columns                       # ("name", "mass")
for row in result.rows:
    row.element.id                   # "Observatory::telescope::optics"
    row.element.type                 # "PartUsage"
    row.cells                        # (("optics",), (8.5,))

# A parameterized query takes typed bindings; a list binds a multi-valued parameter
model.run_document_query("Observatory::HeavierThan", bindings={"threshold": 10.0})

markdown = model.render_document("Observatory::SubsystemReport")
```

A binding value is an element (`opensysml.ElementRef("Demo::optics")`), a `str`, an `int`, a
`float`, a `bool`, or a list of these; anything else raises `DocumentQueryError` before anything
is sent. Cell values come back with those Python types, an element as `ElementRef` and an
unbounded multiplicity as `opensysml.INFINITY`. `render_document` takes no bindings, because a
document binds its queries' parameters in the model; it returns the Markdown text, identical to
what `sysml -render-document` writes.

An unknown query or document raises `SymbolNotFoundError`, a bad binding raises
`InvalidRequestError` naming the parameter, and both calls are capability-negotiated
(`document_query` and `render_document`) the same way as everything above.

## From Node or a browser

```ts
import { loads } from "@opensysml/client";

await using model = await loads(`package Demo {
  part def Wheel { attribute radius : ScalarValues::Real = 0.3; }
  part def Car { part wheels : Wheel[4]; attribute mass : ScalarValues::Real = 1500.0; }
}`);

const value = await model.eval("2 + 2");
const car = await model.symbol("Demo::Car");
const tree = await model.instantiate("Demo::Car");
tree.get("wheels");
```

`@opensysml/client` is not published yet, so build it from a checkout: `npm install && npm run build`
in `clients/node`. `loads` and `load` are the one-shot forms; `connect()` keeps a connection (and so
a service and its parse cache) open across several models. Both a connection and a model are
async-disposable, so `await using` closes them, and `close()` is the explicit form. Values arrive as
discriminated unions to switch on (`value.kind === "quantity"`), integers as `bigint` so an `int64`
is never rounded, and `unset` (a feature the model never gives a value) is distinct from `absent`
(a field the answer did not carry).

The same package runs in a browser, from a second entry point that spawns nothing:

```ts
import { connect } from "@opensysml/client/browser";

await using connection = await connect({ address: "https://sysml.example.com" });
```

That needs a service that allows the page's exact origin (`sysml-grpc -cors-allowed-origins
https://app.example.com`, never `*`), and TLS if an HTTPS page is to reach it at all.

The ergonomic layer covers `GetServerInfo`, `ParseFile`, `GetSymbol`, `Evaluate` and `Instantiate`;
`connection.rpc` is the generated Connect client and reaches everything else.
[The Node API reference](../reference/node-api.md) documents the exports, the errors and the binary
resolution.

## From Java

```java
try (Connection connection = Connection.open()) {      // starts a private sysml-grpc
  Model model = connection.load(Path.of("model.sysml"));

  Value sum = model.eval("1 + 2 * 3");                 // Value.IntegerValue[value=7]
  Value mass = model.evalWithSubject("mass", "Demo::sedan");

  Symbol vehicle = model.symbol("Demo::Vehicle");      // findSymbol returns Optional
  Instantiation built = model.instantiate("Demo::Vehicle");
}
```

The client is meant to live inside a JVM host application it does not own (an Eclipse-based tool,
a Cameo plugin, a web service), so it is built for JDK 17 and its only compile-scope dependency is
`protobuf-java`. The transport is `java.net.http.HttpClient` speaking Connect, which keeps gRPC's
Netty out of a host that has its own. Nothing is published yet; `make build` followed by
`mvn -f clients/java/pom.xml install` puts it in your local repository.

Everything returned is immutable, and no protobuf message appears in the public API: `Value` is a
sealed interface over records, so its variants are closed and enumerable, and `Symbol`, `Diagnostic`,
`Instance` and `Instantiation` are records with copied collections. Everything thrown is unchecked
and descends from `OpenSysMLException`, with `ServiceException` (the call was refused) kept separate
from `ModelException` (the call succeeded and the answer reports a model failure).

One private service is started per classloader, so an Eclipse plugin and a web application in one JVM
each own one and share nothing, while every connection made through one copy of the client shares a
child and therefore its parse cache. Call `Connection.stopSharedServices()` from a plugin's `stop()`
or a `ServletContextListener`, since unloading a classloader does not by itself stop the child.

[The Java API reference](../reference/java-api.md) documents the surface, the exceptions and the
options.

## From Rust

```rust
use opensysml::{loads, Value};

let model = loads("package Demo { part def Car { attribute mass : ScalarValues::Real = 1500.0; } }")?;

let value = model.eval("2 + 2")?;                    // Value::Integer(4)
let car = model.symbol("Demo::Car")?;
let built = model.instantiate("Demo::Car")?;
```

The crate is blocking and pulls in no async runtime: every one of the service's RPCs is unary and the
usual consumer talks to a local child that answers in milliseconds, so a private `tokio::Runtime`
inside a library would cost every consumer something for little gain. Calling it from inside a
runtime is fine, and a test pins that. It is not on crates.io yet, so take it from a path or from
git; the minimum supported Rust version is 1.83.

`load`/`loads` connect and parse in one call; `Connection::private()`, `Connection::external(host,
port)` and `Connection::connect()` (which honours `$OPENSYSML_SERVICE`) are the explicit forms.
`Drop` releases a private child deterministically, and the stdin pipe the client holds is what
makes cleanup survive a `SIGKILL`. `Error` is one enum over every failure, keeping `Error::Service`
(refused) apart from `Error::Model` (answered, and the answer reports a model failure), and every
domain type exposes `wire()` for any field the typed surface does not carry yet.

[The Rust API reference](../reference/rust-api.md) documents the API, the error variants and the one
gap in its release verification.

---

Next: [10. Troubleshooting](10-troubleshooting.md).
