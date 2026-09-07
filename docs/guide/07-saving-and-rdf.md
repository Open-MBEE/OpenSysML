# 7. Saving, and converting to RDF

A model can be written in two forms, SysML v2 notation (`.sysml`, `.kerml`) and RDF in
Turtle (`.ttl`), and converted between them from the REPL, the command line or over gRPC. JSON
is not involved anywhere in this path, not even as an intermediate form.

The vocabulary each triple uses, and the constructs the mapping does not cover, are documented
in [reference/rdf-mapping.md](../reference/rdf-mapping.md).

> **RDF conversion is experimental.** It covers model structure and the behavior written in model
> bodies, refuses any construct it cannot write back, and its vocabulary may change without a
> compatibility path. Every run that converts RDF prints a note saying so. Saving to `.sysml` or
> `.kerml` is stable and exact. See
> [reference/rdf-mapping.md § Status](../reference/rdf-mapping.md#status-experimental).

## Saving a session

`%save` writes the session out, choosing the format from the file extension: `.sysml` for
notation and `.ttl` for RDF Turtle.

```bash
sysml> package Demo { private import ScalarValues::*; part def Vehicle { attribute mass : Real = 1500.0; } }
✓ package Demo

sysml> %save my_model.sysml
saved 102 bytes of sysml to my_model.sysml

sysml> %save my_model.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
saved 3702 bytes of ttl to my_model.ttl
```

A leading `~` is expanded. An existing file is replaced (and the replacement is reported), and the
write is atomic, so an interrupted save leaves the previous file intact. An existing file keeps
its permissions, and a symlink is written through rather than replaced.

A session that does not fully parse is still saved as notation. The file contains the text you
typed, re-indented, so the syntax errors are reported as warnings and no work is left stranded in
the REPL.

```bash
sysml> package Demo {
  ...>   part def Vehicle;
  ...>   ???
  ...> }
3:3: error: expected a namespace member
  ???
  ^~
warning: <session>: 1 syntax error(s):
  3:3: expected a namespace member
warning: the file is saved as typed; fix these and save again
sysml> %save my_model.sysml
warning: <session>: 1 syntax error(s):
  3:3: expected a namespace member
warning: the file is saved as typed; fix these and save again
saved 47 bytes of sysml to my_model.sysml
```

Saving to `.ttl` still refuses such a session, because a graph built from a tree the parser
only partly recovered would silently drop declarations. `sysml -convert` behaves the same way,
since in that case the source already exists on disk.

The same conversion is available without starting the REPL:

```bash
$ sysml my_model.sysml -convert ttl -o my_model.ttl   # notation to RDF
$ sysml my_model.ttl -convert sysml -o back.sysml      # RDF to notation
$ sysml my_model.sysml -convert ttl                    # to stdout
```

The model is given the same way as in every other use of the command, and `-convert` names the
target format.

## Converting from the command line

```bash
sysml model.sysml -convert ttl -o model.ttl     # notation to RDF
sysml model.ttl -convert sysml -o model.sysml   # RDF to notation
sysml model.sysml -convert ttl                  # to stdout
```

The model is a positional argument, as in every other mode of the command, and `-convert` names
the target format. Flags can go before or after the model.

The input format is inferred from the file extension. If the extension is missing or
unrecognized, say what it is with `-from`:

```bash
sysml input.txt -convert ttl -from sysml
```

`-convert` and `-from` accept `sysml`, `kerml`, `text`, `ttl`, `turtle` and `rdf`; `-from` also
accepts `xmi` (or `mdzip`) for a SysML v1 model exported from Cameo/MagicDraw, which is migrated
to v2 on the way in — see [SysML v1 migration](../reference/sysml-v1-migration.md). The output path
plays no part in choosing the format, so a destination without an extension, such as `-o /dev/null`
or a FIFO, needs no extra flags.

Converting to the same format rewrites the input: notation is reformatted, and
Turtle is normalized (prefixes sorted, predicates grouped by subject).

### Exit status

The command exits non-zero and writes nothing for any input it cannot convert
faithfully, whether that is a syntax error in the notation, malformed Turtle, or an RDF construct
outside the mapping. It never writes a partial model.

## Converting over gRPC and from Python

The same conversion is available as the service method `Convert`, which `GetServerInfo` reports as
the `convert` capability. It accepts either a `file_path` that the service opens or `content`
passed inline, takes the same format names as `-from` and `-to`, and returns the written text
with its formats, or an `error` together with the diagnostics that explain it.
`tolerate_syntax_errors` writes notation despite syntax errors; it is rejected for any direction
that builds a graph, where an unparsed declaration would be silently dropped.

A response whose conversion used the RDF mapping sets `experimental` and `experimental_notice`,
whether it succeeded or refused, so a client can learn the status from the response rather than
from this page. The `opensysml` client raises this as an `ExperimentalFeatureWarning`, which
`warnings.simplefilter` can suppress:

```python
import warnings
from opensysml import ExperimentalFeatureWarning

warnings.simplefilter("ignore", ExperimentalFeatureWarning)
```

From Python:

```python
model = opensysml.load("model.sysml")
model.save("model.ttl")                          # SysML notation to RDF
opensysml.convert("sysml", file_path="model.ttl")  # and back
```

The client API is documented in [reference/python-api.md](../reference/python-api.md), and
[chapter 9](09-clients.md#from-python) shows how to use it.

## Round-tripping

`notation → RDF → notation` gives back an equivalent model, and
`notation → RDF → notation → RDF` gives back the *same graph*. This is the property the test
suite checks over the fixtures in `internal/core/export/testdata/convert/`.

The notation that comes out of a round trip is the source itself when the graph still carries it:
every element written to `.ttl` carries its lines as `sysx:sourceText`, comments and blank lines
included, exactly as the file spells them — tabs, odd indentation, CRLF and all — and converting
back returns them, so the file comes back byte for byte. The graph stays authoritative: an
element whose triples were edited after the export — a flag set, a value changed, a member
removed — is written back from its structure in canonical notation, and only that element's
lines change. A graph without source text, from another tool or with the text stripped, is
written entirely from its structure: a reference may then be written relative to a different
scope, and a clause written `:>` comes back as `specializes`. Both forms parse to the same model,
which the second conversion to RDF confirms. The rules are in
[reference/rdf-mapping.md](../reference/rdf-mapping.md#source-text).

**Saving to `.sysml` writes the source directly.** It writes the session's own source through
the formatter rather than re-printing the graph, so comments, notes and spacing are preserved
whether or not the model was edited. Only the `.ttl` direction uses the mapping. Syntax is
checked in every direction, and notation the parser cannot read is rejected, so a save never
silently reformats a model that does not parse.

## A worked example

[`examples/rdf-interop-demo.sysml`](../../examples/rdf-interop-demo.sysml) is the reference model
for this chapter: a rover and its ground link, declared with packages, definitions, usages, ports,
a connection, multiplicity, values and documentation. The mapping covers all of these, so the
model converts in both directions:

```bash
$ sysml examples/rdf-interop-demo.sysml -convert ttl -o /tmp/rover.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover.ttl (ttl, 40883 bytes)
$ sysml /tmp/rover.ttl -convert sysml -o /tmp/rover-back.sysml
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover-back.sysml (sysml, 1014 bytes)
```

Converting the returned notation again produces a byte-identical graph, which is the round-trip
property described above. The `//` header comment comes back with the source text the graph
carries and is lost once that text is stripped, as explained in
[reference/rdf-mapping.md](../reference/rdf-mapping.md); the package's `doc` and `comment` are
declarations and survive either way.

[`examples/semantic-layer/demo.sysml`](../../examples/semantic-layer/demo.sysml)
and [`examples/repl-behavioral-demo.sysml`](../../examples/repl-behavioral-demo.sysml)
also convert, as do all of the `parser_features_demo_*.kerml` files. The
behavior written in a body converts too: states, regions, substates, action nodes, assignments,
transitions and the result expression a calculation ends in all have a mapping. Conversion is
refused for constructs the notation could not be rebuilt from, such as a name shared by two
members of one namespace. A refusal names the construct where conversion stopped and says why
the graph could not carry it:

```bash
$ sysml examples/parser_features_demo_action_semantics.sysml -convert ttl -o /tmp/action-semantics.ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/action-semantics.ttl (ttl, 84121 bytes)
0
```

For example, two members of one namespace sharing a name are refused, since the name is what
identifies an element in the graph:

```sysml
package P {
  part def D {
    attribute x : ScalarValues::Real;
    attribute x : ScalarValues::Integer;
  }
}
```

```bash
$ sysml dup.sysml -convert ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
sysml: cannot convert the duplicate declaration of "x" at dup.sysml:4:5: a name identifies an element in the graph, so two members of one namespace cannot share it
2
```

---

Next: [8. Editors](08-editors.md).
