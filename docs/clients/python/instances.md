# Instances and values

Instantiate a part usage or definition to inspect its feature values. Attribute
and item access return decoded Python values; item lookup is also available
when a feature name is not a Python identifier.

```python
inst = model.instantiate("Demo::Vehicle")

inst.mass
inst["mass"]
inst.features
inst.get("missing", 0)
```

Integer, real, boolean, string and sequence values become `int`, `float`,
`bool`, `str` and `list`. An object-valued feature becomes a nested `Instance`.
Unknown feature names raise `AttributeError` for attribute access or `KeyError`
for item access.

The service expands object graphs to a bounded depth and stops when it reaches
a type already on the current path. An unexpanded child is represented by its
integer instance ID. `get_feature(name)` returns the raw protobuf
`FeatureValue`; `raw_features` exposes the complete raw map.

## Missing and failed values

A feature that holds no value is `opensysml.UNSET`; it is not `None`, which
represents the model's `null`.

```python
inst.mass is opensysml.UNSET
inst.mass is None
```

A feature the service could not evaluate, such as a cyclic derived attribute,
raises `FeatureValueError` on attribute or item access. The `features` mapping
keeps that error at the failing entry so the rest of the object can still be
inspected. `FeatureValueError` is not an `AttributeError`, so `hasattr` does
not hide it.

## Value types

Values in features, expressions, arguments and outputs retain their wire kind;
they are not serialized to strings or untyped dictionaries. Examples include:

| SysML value | Python value |
| --- | --- |
| `Real`, `Rational`, `Integer`, `Boolean`, `String` | `float`, `int`, `bool`, `str` |
| `Complex` | `complex` |
| array values | `opensysml.Array`, with dimensions and row-major elements |
| numeric vectors | `opensysml.Vector` |
| vector and tensor quantities | `VectorQuantity` and `TensorQuantity` |
| collection sets | `SetValue`, unordered and unique |
| measurement references | `MeasurementRef` |
| calc values | `Function` |
| reflective values from `x meta T` | `Metaobject` |
| enum values | `EnumLiteral` |
| open model questions | `Undetermined` |
| an unbounded multiplicity | `opensysml.INFINITY` |

An `EnumLiteral` identifies the literal declaration and may also carry the
scalar value assigned to that literal. `TensorQuantity` is indexed by one
integer per dimension; `Array.nested()` unfolds its row-major elements into
nested lists. `Undetermined` records why the model leaves a value open and
refuses `bool()` so it cannot be confused with `False`.

The service negotiates support for newer value kinds. If a capability is
missing, receiving an unrepresentable value raises `UnsupportedValueError`;
sending one as an argument is refused before the request is sent. An older
service without `feature_values` raises `MissingCapabilityError` when
instantiation is requested.

Actions, state machines and analysis cases also return decoded maps and values;
see [Verification and analysis](verification.md) and the
[Python API reference](../../reference/python-api.md).

## Actions and state machines

Behavior runs are model methods. Arguments and outputs use the same typed
values as feature access. A value the wire format cannot represent is reported
as an `UnsupportedValueError` in its result entry, leaving other entries
available.

```python
model.execute_action("Demo::addFive", inputs={"result": 10})
model.execute_state("Demo::Machine", events=["go"])
```

Time-triggered behavior advances on a simulation clock that starts at zero.
`execute_state` includes `final_time` when supported by the service.

Use `schedule="declared"`, `"reverse"` (the default) or `"seed:<n>"` to pick
one order when a behavior has several valid orders. The `explore` policy
enumerates outcomes instead:

```python
exploration = model.explore_action("Demo::race")
for outcome in exploration:
    print(outcome.outputs, outcome.linearizations, outcome.witness)

exploration.complete
exploration.runs
exploration.budgets_hit
```

`explore_state` and `explore_analysis` provide the corresponding forms for
state machines and analysis cases. Exploration requires the service's
`schedule_explore` capability; execution scheduling requires `schedule`.
