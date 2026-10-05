# Editing and saving

A loaded model can be converted back to SysML notation or to RDF Turtle. The
service performs the conversion; the Python client does not maintain a
separate writer.

```python
model = opensysml.load("model.sysml")

notation = model.to_sysml()
turtle = model.to_turtle()
model.save("model.ttl")
model.save("out.sysml")

opensysml.convert("ttl", file_path="model.sysml")
opensysml.convert("sysml", content=turtle.content, from_format="ttl")
```

A `Conversion` carries output text, source and target formats, and whether
the mapping is experimental. `str()` and `len()` operate on its text;
`write(path)` saves it. File formats are inferred from extensions; inline
content must name `from_format`.

Turtle conversion is experimental and raises `ExperimentalFeatureWarning`.
The mapping carries supported model structure and body behavior, and refuses
constructs it cannot write back. SysML notation conversion is stable.
Migration from SysML v1 is also experimental; see the
[migration reference](../../reference/sysml-v1-migration.md) for its supported input
formats and report.

## Editing source

`model.edit()` creates an `Editor` that describes changes to the loaded
source. Changes are expressed as typed operations and applied as one batch:

```python
editor = model.edit()
editor.add_part_def("", "Vehicle")
editor.add_part("Vehicle", "engine", type="Engine")
result = editor.apply()
result.save("model.sysml")
```

Editors can add, rename, delete and move declarations, change values, and
write supported requirement statements and state transitions. Typed helpers
include `add_part_def`, `add_part`, `add_connection`, `add_flow`,
`add_allocation`, `add_satisfy`, `add_transition`, `add_calc_def` and
`add_action_def`. `add_member` provides the general form for supported
member kinds and modifiers. The API reference lists all operation signatures
and their capability requirements.

`set_value(target, expression)` replaces or adds one feature value;
`rename(target, new_name)` rewrites the declaration and its references. A
target may be a fully qualified symbol ID or a `Symbol` returned by a model
lookup. These operations return the editor and can be chained.

```python
editor = model.edit()
editor.set_value("Demo::sedan::mass", "1800.0")
editor.rename("Demo::Vehicle::mass", "curbMass")
result = editor.apply()
```

To add a declaration, use `add_member(owner, kind, name, ...)` or a typed
helper such as `add_part_def`:

```python
empty = opensysml.loads("package Demo {}", strict=True)
result = empty.edit().add_part_def("", "Vehicle").apply()
```

`add_member` accepts a type, multiplicity, value, specialization,
abstractness, redefinition, default, direction, metadata, body expression
and documentation where the grammar permits them. Metadata writes `#M`
prefixes; `doc=` writes a declaration's first `doc /* ... */` member.
Helpers also cover `ref` and `return` parameters, requirements, verification
objectives, state transitions, action-body sequencing, metadata usages and
prefixes, imports, documentation, comments and source-only line notes.

Constraint helpers distinguish a feature value (`value=`) from a constraint
body (`expression=`). Calc helpers accept input pairs, a return type and a
return expression; a return expression requires a type and cannot be combined
with a body `expression=`. Action helpers accept lists of input and output
pairs. `add_perform_action` declares an action perform usage, while
`add_perform` writes a perform statement in an action body.

`delete(target, cascade=False)` removes a declaration transactionally.
`move(target, owner)` moves a declaration and its body, comments and
rewritten references to another namespace in the same document. `add_import`
authors membership or namespace imports, with optional recursive, all and
filter clauses. `add_connection` can author connection-like usages such as
bindings, flows, allocations and transitions, with endpoints resolved in the
owner's scope.

The returned `EditResult` is a `Conversion`: it contains the edited notation,
applied operations and per-document rewritten content. For a multi-document
model, the entire batch is applied atomically and the result lists each
document by its original name.

### Edit guarantees

- Operations locate source through parser spans, not string search, so a
  matching comment or string literal is not changed.
- The service splices bytes around edited spans without reformatting
  untouched source.
- It parses and analyses the result before returning it. An edit that
  introduces a syntax or name-resolution error is refused.
- References are resolved against the model after the whole batch, so one
  operation may refer to a declaration another operation adds.
- Names are allocated in operation order, and insertion anchors must already
  exist in the source model.

Refusals are typed subclasses of `EditError`: `NoEditsError`,
`EditTargetError`, `InvalidEditError`, `OverlappingEditsError` and
`EditResultError`. Errors include diagnostics and, for relevant rename or
delete refusals, the referring elements.

```python
try:
    model.edit().rename("Demo::System::margin", "label").apply()
except opensysml.InvalidEditError as error:
    print(error.failure)
```

An editor is applied once; reload saved source before making another edit.
The model must still be in the service's bounded cache when the edit is
applied. The service needs `apply_edits`; individual operations also require
their advertised authoring capabilities.

Specific operation groups are capability-gated. For example, connection
authoring requires `authoring` and `connection_authoring`; satisfy and
requirement edits require their authoring capabilities; transitions require
`transition_authoring`; action sequencing requires `sequence_authoring`;
imports require `import_authoring`; constraint bodies and state subactions
require their corresponding capabilities. Documentation, comments, metadata
and member modifiers are likewise checked before the edit is sent.

### Limits

Editing changes a model that already exists; it does not construct one from
scratch as mutable Python objects. An operation cannot be applied twice, and
the service refuses to edit an evicted model rather than operating on
different content. A rename is refused if it would capture, shadow or change
the meaning of a name at the declaration or one of its references. References
outside the loaded model cannot be rewritten.

See the [RDF mapping reference](../../reference/rdf-mapping.md) for the
current conversion contract and the [Python API reference](../../reference/python-api.md)
for the complete editing surface.

## Saving and source fidelity

A `Model` writes the exact source held by the service for its model hash, so
editing a file after loading does not change what `model.save()` writes.
`convert(file_path=...)` instead reads the file's current contents. Since the
service keeps parsed models in a bounded cache, a model evicted before saving
raises `ModelNotFoundError`; load it again rather than writing another source.

Notation-to-notation conversion preserves comments and layout because it
re-emits the parsed source. Notation-to-Turtle-to-notation preserves an
equivalent model, not identical bytes: comments are not represented in RDF.
Syntax errors normally raise `ConversionError` with diagnostics.
`tolerate_syntax_errors=True` is available for notation-to-notation conversion
only, where errors are retained in `Conversion.diagnostics`.

## Migrating from SysML v1

A SysML v1 XMI, UML or `.mdzip` input is migrated, not converted. Conversion
is lossless; migration reports every v1 element as mapped, approximated,
unmapped or skipped.

```python
migration = opensysml.migrate(
    "sysml",
    file_path="Vehicle.mdzip",
    report=True,
)
migration.report.summary
migration.report.by_verdict("unmapped")
migration.write("Vehicle.sysml")
```

`migrate` infers the input format from a file extension or takes
`from_format` for inline bytes. It can produce SysML notation or Turtle, and
returns a `MigrationReport`; optional results, layout, image and strictness
settings are documented in the [migration reference](../../reference/sysml-v1-migration.md).
Migration is experimental and raises `ExperimentalFeatureWarning`. `convert`
refuses a v1 source with an `InvalidRequestError` directing callers to
`migrate`.
