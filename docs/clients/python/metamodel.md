# Metamodel classes and JSON

`opensysml.metamodel` holds one generated class per SysML/KerML metaclass
(`Element`, `Namespace`, `Feature`, `PartUsage`, ...), each with one read-only
property per metamodel property. `opensysml.read_json` builds those objects
from a JSON export in pure Python: it needs no `sysml-grpc` service, never
converts to notation and never writes back.

This is not the same as [typed classes](typed-classes.md):
`opensysml.generate` writes classes for the definitions in *your* model
(`Vehicle`, `Engine`) over service-backed instances, while the metamodel
classes describe the language's own elements and are the same for every model.

```python
import opensysml
from opensysml.metamodel import Feature, PartUsage

g = opensysml.read_json("vehicle.full.json")   # sysml-toolkit full-json

for p in g.all(PartUsage):
    print(p.qualifiedName, [t.name for t in p.type], len(p.ownedFeature))
    assert isinstance(p, Feature)              # follows the spec hierarchy
```

The classes follow the metamodel's inheritance, so `graph.all(Feature)` also
returns every `PartUsage`. `graph[id]` looks an element up by `@id`, and
`graph.roots()` returns the namespaces that nothing in the document owns. References are
resolved when read, and many-valued properties return tuples.

## Names

Each property's primary name is `snake_case` (`owned_feature`, `is_abstract`);
the spec's `camelCase` name (`ownedFeature`, `isAbstract`) is an alias for the
same descriptor, so both spellings behave identically and type-check. A name
that would be a Python keyword gets a trailing `_`. `PartUsage.from_json_key("partDefinition")`
and `PartUsage.json_key("part_definition")` convert between the two. The
package ships `py.typed`, so mypy and pyright see every property's type.

## Inputs

`read_json` accepts a file path (a `str` is always a path, never JSON text),
bytes, a single element object, or a list of element objects, with `@id`,
`@type` and `{"@id": ...}` references. It also unwraps SysML v2 API DataVersion
(`identity`/`payload`) and Commit (`change`) envelopes and `sysml:`-prefixed or
IRI types. An `@type` the metamodel does not know is read as its nearest known
ancestor when you pass `supertypes={"MyType": "PartUsage", ...}`, otherwise as
`Element`; `element.json_type` keeps the written name.

## Missing, empty and dangling values

A property the JSON does not carry raises `NotSupplied` (its `.derived` says
whether the property is computed). It never reads as `None` or `()` instead:
only an explicit `null` or `[]` in the JSON means empty. A reference to an `@id`
the document does not contain raises `UnresolvedReference` when read.

OpenSysML's `api-json` export carries the owned properties it writes plus some derived ones
(`ownedFeature`, `owner`, `qualifiedName`, `ownedMember`, ...). It carries no `feature`,
`inheritedFeature` or `definition`, and `type` only on some usages, so those reads raise
`NotSupplied` with `derived=True` until the engine serves derived properties. The export also
writes no `null` and no `[]`: an unset owned property, such as an unnamed element's
`declaredName`, is absent and raises `NotSupplied` too. sysml-toolkit `full-json` carries derived
properties as well. A reference to an element the document does not include, such as a
standard-library element, raises `UnresolvedReference` when read.
Providing engine-computed derived properties, implementing metamodel operations, and loading JSON
into the OpenSysML engine are separate follow-up work.

See the [Python API reference](../../reference/python-api.md#metamodel-classes-and-the-json-reader)
for the full error table.
