# Interfaces

The same pipeline is reachable from the command line, the REPL, the gRPC
service with its Python client, and the LSP server behind the VS Code
extension. Whichever door you use, a document renders identically.

## CLI

```console
$ sysml model.sysml -render-document Observatory::MassReport            # Markdown to stdout
$ sysml model.sysml -render-document Observatory::MassReport -o report.md
$ sysml model.sysml -render-document Observatory::MassReport \
    -doc-form html -html-css theme.css -o report.html
$ sysml model.sysml -render-documents site -doc-form html
$ sysml model.sysml -render-document Observatory::MassReport \
    -doc-form pdf -pdf-engine weasyprint -doc-title-page -doc-toc \
    -doc-number-sections -o report.pdf
$ sysml model.sysml -run-query "Observatory::SubsystemTable root=Observatory::telescope"
$ sysml cookbook.sysml -instantiate Cookbook::telescope -run-query "Cookbook::Violated root=telescope"
$ sysml model.sysml -instantiate Observatory::telescope -render-document Observatory::MassReport -o report.md
```

| Flag | Meaning |
|---|---|
| `-render-document <name>` | Render the named document definition |
| `-doc-form <form>` | `markdown` (default), `html` or `pdf` |
| `-pdf-engine <engine>` | `weasyprint` (default), `pandoc` or `prince` |
| `-doc-title-page` | Separate title page (HTML or PDF) |
| `-doc-toc` | Table of contents (HTML or PDF) |
| `-doc-number-sections` | Hierarchical section numbers (HTML or PDF) |
| `-html-css <file\|url>` | Style the HTML with this sheet, after the default one (repeatable) |
| `-html-no-default-css` | Leave the default stylesheet out |
| `-html-default-css` | Write the default stylesheet and exit |
| `-html-fragment` | The document element alone, to embed in your own page |
| `-run-query "<name> [<p>=<expr> ...]"` | Run one document query directly |
| `-instantiate <name>` | Create an object first, so the query or document reads what it holds |
| `-o <file>` | Output file; required for PDF |

`-run-query` bindings are space-separated `parameter=expression` pairs after
the query's qualified name. A name expression binds the object `-instantiate`
created under it while the run holds one, and the element it refers to
otherwise; `#1` and `telescope.primaryMirror` bind a held object by id and by path;
quoted strings and numeric literals bind values. A document binds its own
parameters in the model, so `-instantiate` beside `-render-document` is enough
for its queries to read the object's current values and to check it through
`Verdicts` ([Objects the session holds](query-cookbook.md#objects-the-session-holds),
[Which constraints and requirements hold](query-cookbook.md#which-constraints-and-requirements-hold)).
The exit code is non-zero on any planning, binding or execution error. Full
details are in the [CLI reference](../reference/cli.md).

## REPL

Inside `sysml`'s interactive session, the same two operations are commands:

```
%run-query Observatory::SubsystemTable root=Observatory::telescope
%render-document Observatory::MassReport
```

The bindings follow the CLI's rule over the objects the session holds: after
`%instantiate telescope`, `root=telescope` binds the object rather than the
usage, `#1` and `telescope.primaryMirror` bind one by id and by path, and a
query over `Verdicts` prints one `<assertion> on <path>: <verdict>` line per
row — the same sweep `%validate telescope` prints, as a query:

```
%instantiate Cookbook::telescope
%run-query Cookbook::Violated root=telescope
```

`%render-document` prints Markdown; PDF output is CLI-only. See the
[REPL command reference](../reference/repl-commands.md).

## gRPC and Python

The `sysml-lsp -grpc` service exposes two document RPCs, advertised as the
`document_query` and `render_document` capabilities:

- **`RunDocumentQuery`** — run a named document query with typed bindings;
  the reply carries the projected columns and typed cell values in the
  engine's deterministic order.
- **`RenderDocument`** — render a named document definition; the reply
  carries the Markdown.

Both run over the objects the service holds for the model: `Instantiate`
creates one and keeps it for as long as the model stays cached, the
counterpart of `%instantiate` and `-instantiate`, so a binding may name it by
id or by path and a document renders its current values.

The Python client wraps both on its model handle:

```python
import opensysml
from opensysml.document import ElementRef, ObjectRef

model = opensysml.load("observatory.sysml")

result = model.run_document_query(
    "Observatory::SubsystemTable",
    bindings={"root": ElementRef("Observatory::telescope")},
)
print(result.columns)          # ('name', 'mass')
for row in result:             # DocumentRow: row.element, cells by column index
    print(row[0], row[1])

markdown = model.render_document("Observatory::MassReport")

cookbook = opensysml.load("cookbook.sysml")
cookbook.instantiate("Cookbook::telescope")
checks = cookbook.run_document_query(
    "Cookbook::Violated",
    bindings={"root": ObjectRef(path="Cookbook::telescope")},   # or ObjectRef(id=1)
)
for row in checks:             # row.element is the assertion, row.verdict its verdict
    print(row.verdict)         # assert constraint lightweight on Cookbook::telescope.primaryMirror: violated
```

Bindings accept an `ElementRef` (a model element by qualified name), an
`ObjectRef` (an object `instantiate` built, by `id`, by `path` —
`"Cookbook::telescope"`, `"#1"`, `"Cookbook::telescope.primaryMirror"` — or
both), `str`, `int`,
`float`, `bool`, or a sequence of those. Query results decode the service's
typed values back into Python values, with unbounded multiplicity as
`opensysml.document.INFINITY`, an object as an `ObjectRef` carrying `id`,
`path` and the usage it stands for (`row.object` for a row over an object),
and a `Verdicts` row's verdict as `row.verdict`, a `DocumentVerdict` carrying
`assertion`, `kind`, `path`, `status` (`holds`, `violated`, `undecided`),
`condition`, `reason` and `verification`. A verdict is answered, never bound.
Errors are typed exceptions (`InvalidRequestError`, `SymbolNotFoundError`,
`MissingCapabilityError`, `ModelNotFoundError`). See the
[API reference](../reference/api.md).

## VS Code and LSP

The LSP server gives editors document-generation support on top of the usual
diagnostics:

- **`opensysml/documents`** lists the document definitions the workspace
  declares (qualified name plus declaring file), which fills the extension's
  Render Document picker.
- **`opensysml/renderDocument`** renders one of them to Markdown through the
  same pipeline the CLI uses, against the same workspace model the
  diagnostics come from.
- **`opensysml/renderChanged`** notifies the client, debounced, when edits
  invalidate a rendering, so an open preview can refresh itself. Rendering
  is on demand — the server does not re-render while you type.

Authoring a document also benefits from the general language support:
planning mistakes (a missing title, an unknown query, a bad binding) surface
as diagnostics in the editor with the same messages the CLI prints. See the
[LSP reference](../reference/lsp.md).
