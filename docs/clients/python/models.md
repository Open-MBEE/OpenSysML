# Models and symbols

`load` and `loads` parse one document into a `Model`. A model includes the
service's diagnostics, its root symbols and the source identity used by later
operations.

```python
import opensysml

model = opensysml.load("model.sysml")
if not model.ok:
    for diagnostic in model.diagnostics:
        print(diagnostic.code, diagnostic.message)
```

`model.ok` is false if any diagnostic is an error; `model.errors` returns only
those diagnostics. `model.raise_for_errors()` raises `ModelError` or returns the
model, and `strict=True` on `load`, `loads` or `Connection.load_from_content`
does the same during parsing. `ModelError.diagnostics` and `.model` preserve
what the service reported and parsed.

Diagnostics carry `severity`, `message`, `code` and a source location. Branch
on `code`, not message text. Common codes include `syntax`,
`feature-value-overriding` and `unresolved`; execution notes can include
`choice-point` and `guard-unevaluable`. If the connected service does not
advertise `diagnostic_codes`, a diagnostic's code is the empty string.

```python
unresolved = [d for d in model.diagnostics if d.code == "unresolved"]
```

## Several source files

`load` and `loads` each parse one document as a model of its own. An import of a
package declared only in a neighbouring file does not resolve, and `load`
refuses a directory. Use `parse_sources` to parse multiple documents together:

```python
from opensysml import SourceDocument, parse_sources

model = parse_sources([
    "models/base.sysml",
    ("chapter.sysml", chapter_text),
    SourceDocument.inline("units.kerml", kerml_text, language="kerml"),
], strict=True)

model["Chapter::Car::engine"]
model.documents
```

Each source can be a path read by the service, a `(name, text)` pair or a
`SourceDocument`. Inline documents need a unique name; their names also identify
diagnostics. `model.root` is the first document's root, `model.roots` contains
one per document, and lookups cover the model as a whole. A service that does
not advertise `parse_sources` raises `MissingCapabilityError`.

Notation conversion with `model.convert("sysml")` is defined for one document;
use a graph format such as `"ttl"` or `"api-json"` for a multi-document model.

## Finding symbols

`Model.find` searches for a short name and returns `None` if it is absent.
Subscript lookup accepts a short or fully qualified name and raises
`SymbolNotFoundError` (also a `KeyError`) when missing. `get` accepts a
fully-qualified name and returns `None` when absent.

```python
vehicle = model["Vehicle"]
vehicle.attributes()
vehicle.parts()
vehicle.get_attr("mass")

model.find("Nope")             # None
model.get("Demo::Vehicle")     # fully-qualified lookup
"Vehicle" in model             # True
```

The service indexes model symbols and the standard library. A qualified lookup
is a single request; short-name lookup queries then fetches the matching symbol.
The standard library is available by qualified name even though it is not part
of the model's source tree.

A `Symbol` exposes its identity, kind, children and resolved type facts:

```python
engine = model.get("Demo::Vehicle::engine")
engine.type_facts
engine.multiplicity
engine.specializations
```

`TypeFacts` describes declared and resolved types; `Multiplicity` records
lower and upper bounds; and `Specialization` identifies typing, redefinition,
subsetting and other relationships. Use the [Python API reference](../../reference/python-api.md)
for the full symbol surface.
