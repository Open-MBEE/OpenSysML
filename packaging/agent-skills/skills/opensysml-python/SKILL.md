---
name: opensysml-python
description: Parse, query, instantiate, execute and verify SysML v2 models from Python with the `opensysml` client (pip install opensysml), which drives the OpenSysML sysml-grpc service — requiring a clean model, evaluating expressions, reading object features, running actions and calcs, checking requirements, handling its error types. Use when writing Python that reads or runs SysML v2 models.
---

# Using OpenSysML from Python

`opensysml` talks to the `sysml-grpc` service and starts a private one for you, so a script never runs
`sysml` as a subprocess. Reference: `docs/guide/09-clients.md` (From Python) and
`docs/reference/python-api.md` in the OpenSysML repository.

## Install and get the service binary

```bash
pip install opensysml       # CPython 3.10+
```

The client looks for `sysml-grpc` at `$OPENSYSML_BINARY`, then `~/.opensysml/bin/sysml-grpc`, then on
`PATH`. If none is there, download the release build, which is checked against a pinned digest:

```bash
python -c "from opensysml.binary import download_binary; download_binary('latest')"
```

Without a binary, `connect()` raises `opensysml.ConnectionError` listing where it looked.

## Load, and require a clean model

```python
import opensysml

model = opensysml.load("model.sysml", strict=True)    # raises ModelError on any error diagnostic
```

A model with syntax errors still loads without `strict=True`; check before trusting it:

```python
model = opensysml.load("model.sysml")
if not model.ok:
    for d in model.errors:            # Diagnostic: severity, message, code, location
        print(d)
```

Branch on `d.code` (`"syntax"`, `"unresolved"`, ...), never on message text. `ModelError` carries
`.diagnostics` and the partial `.model`. `strict_conformance=True` additionally makes OpenSysML-only
notation an error. `load` reads one document, so imports across files do not resolve; for a model
spread over several files use `opensysml.parse_sources(["base.sysml", ("extra.sysml", text)], strict=True)`.

## Evaluate, instantiate, run

```python
model.eval("MyModel::Vehicle::mass")                  # Python value: int, float, bool, str, list
inst = model.instantiate("MyModel::car")
inst.mass, inst["mass"], inst.features               # nested objects come back as Instance
outputs = model.execute_action("MyModel::calibrate", inputs={"x": 21})   # dict of out values
result = model.calc("MyModel::Margin", arguments=[20.0, 100.0])          # result.value
```

Three distinct "no value" results, none of them an error: `opensysml.Undetermined` (the model leaves
the value open; `bool()` raises `TypeError`), `opensysml.UNSET` (an object holds nothing for the
feature; falsy, not `None`), and `None` (the model's `null`). Never coerce them to 0 or `False`.

## Verify

```python
verdict = model.verify_requirement("MyModel::Vehicle::lightEnough", subject="MyModel::truck")
bool(verdict), verdict.condition, verdict.explain()
for v in model.verify_satisfaction():     # every `assert satisfy R by x;`
    v.raise_for_error()                    # raises only if it could not be evaluated
    if not v:
        print(v.explain())
```

A false verdict is a returned result, not an exception. `verify_constraint` takes the same `subject`.

## Errors

Every actionable failure is an `opensysml.OpenSysMLError`; gRPC codes are already translated, so do
not `import grpc`. Most useful subclasses: `ModelError` (strict load), `ModelFileNotFoundError` (bad
path), `ExecutionError` (eval/instantiate/execute/verify failed; also a `RuntimeError`),
`SymbolNotFoundError` (`model["Nope"]`; also `KeyError`), `FeatureValueError`, `ConnectionError`,
`MissingCapabilityError` (the service is older than the call needs).

## Pitfalls

- Reuse one `Connection` (`opensysml.connect()`) or the module-level functions; a parse costs tens of
  milliseconds, a query on a loaded model about one.
- Use qualified names (`Package::Def::feature`) everywhere a symbol is named.
- Write models with the `sysml-v2-modeling` skill and validate them with `sysml -validate` when the
  CLI is available; its diagnostics match `model.diagnostics`.
