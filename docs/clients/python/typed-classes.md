# Typed classes

`Instance` is dynamic: an editor cannot complete `inst.mass`, and a type
checker cannot reject `inst.mas`. `opensysml.generate` emits a Python class
for each SysML definition so applications can add static checking around the
same runtime instances.

```bash
python -m opensysml.generate model.sysml -o model_types.py
opensysml-generate model.sysml -o model_types.py
```

```python
import opensysml
from model_types import Vehicle

model = opensysml.load("model.sysml")
instance = model.instantiate("Demo::Vehicle")
vehicle: Vehicle = Vehicle.from_instance(instance)

vehicle.mass
vehicle.instance
```

The generated properties carry annotations and delegate reads to the
underlying `Instance`; they are views, not copies. `from_instance` rejects an
instance of an unrelated definition and accepts a specialized definition.
An instance whose type is a usage rather than a definition can still be
wrapped; use `Vehicle.unchecked(instance)` for an explicitly unchecked view.
The package ships a `py.typed` marker, so its own annotations are available to
mypy and pyright too.

## Keeping generated code current

Each generated module records the generator schema and source model hash:

```python
SYSML_GENERATOR_VERSION = "1"
SYSML_MODEL_HASH = "sha256:…"
```

`--check` regenerates in memory and writes nothing:

```bash
python -m opensysml.generate model.sysml -o model_types.py --check
```

It exits nonzero and names the regeneration command when the module is
missing or stale, making it suitable for CI. Generation requires a service
that advertises the `type_facts` capability; otherwise the generator refuses
to emit classes whose feature annotations would all be indistinguishable
`object` values.

The output is deterministic and is one runtime `.py` file, rather than a
separate implementation and stub that can drift. Definitions are emitted in
fully-qualified order with base classes first. The committed golden example
is `client/python/tests/golden/vehicle_types.py`.

## Type mapping

| SysML construct | Generated Python type |
| --- | --- |
| `Real`, `Rational` | `float` |
| `Complex` | `complex` |
| `Integer`, `Natural` | `int` |
| `Boolean` | `bool` |
| `String` | `str` |
| `Array` | `opensysml.Array` with dimensions and row-major elements |
| numeric vectors | `opensysml.Vector` |
| vector quantities | `opensysml.VectorQuantity` |
| tensor quantities | `opensysml.TensorQuantity` with dimensions and quantity components |
| `Collections::Set` | `opensysml.SetValue` |
| measurement references | `opensysml.MeasurementRef` |
| calc values | `opensysml.Function` |
| `x meta T` | `opensysml.Metaobject` |
| unbounded multiplicity or count | `opensysml.INFINITY` |
| an open model question | `opensysml.Undetermined` |
| an object feature holding no value | `opensysml.UNSET` |
| scalar library type (such as `Celsius :> Real`) | its Python scalar type |
| enum usage | `EnumLiteral`, with literal identity and optional scalar value |
| another model definition | its generated class |
| multiplicity `1`, `1..1` or undeclared | `X` |
| `0..1` multiplicity | `X | None` |
| `*`, `0..*` or upper bound greater than 1 | `list[X]` |
| `Number` | `object`, with a comment naming the broad type |
| type outside the model | `object`, with a comment naming its FQN |
| unresolved or absent type | `object`, with a comment naming what was written |
| `specializes`, `subsets` or `redefines` | Python base class |

An unbounded multiplicity uses the `INFINITY` singleton. `None` remains the
model's `null`, distinct from the `UNSET` sentinel for an object feature that
holds nothing. Unsupported or unresolved types are never assigned a guessed
type or `Any`.

## Inheritance and limitations

`specializes`, `subsets` and `redefines` relationships are represented as
Python base classes. A redefinition reuses the original feature name and
overrides its base property; a subset adds a property beside the inherited
one. Python resolves multiple bases in declaration order. Redundant bases are
omitted where another declared base already specializes them. If the model's
hierarchy cannot be linearized, the generator keeps the usable bases and
records the omitted edge rather than emitting an unimportable module.

Only structural usages (`attribute`, `part`, `item`, `occurrence`,
`individual`, `port`, `enum`) become properties. Behavioral and connector
usages such as actions, states, calcs, constraints, requirements, connections,
flows, interfaces, allocations and cases are not instance feature values;
use the model's execution and verification methods for them.

A redefinition reuses the original feature name, so the generated property
overrides its base property and uses the redefinition's type when narrowed.
Python does not check that narrowed annotation against the base property. A
subset under a new name adds a property and inherits the type and multiplicity
it leaves unstated.

In multiple inheritance, bases appear in declaration order and Python's
usual method-resolution order applies. A base already inherited through
another declared base is omitted. If no valid Python linearization exists,
the generator keeps usable bases and records the omitted edge in the
generated class rather than producing a module that fails to import.

Generated classes do not model generic parameters, and an `enum def` becomes
a plain class. Two definitions with the same simple name get path-qualified
class names, such as `A_Thing` and `B_Thing`. A feature named like a generated
class member (`instance`, `from_instance` or `sysml_id`) gets a trailing
underscore on its Python property; its SysML feature name is unchanged.
