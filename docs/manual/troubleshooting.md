# Limitations and Troubleshooting

## How errors surface

Document generation fails loudly and early, never partially. Every mistake is
a **typed error**. Planning problems (structure, query references, bindings)
surface when the document is compiled, as `document-plan-*` diagnostics in
the editor and as source-located errors from the CLI. Execution
problems (a query that cannot run) stop the render with the query error's
message. A document that cannot be rendered exits `2`, and nothing is written.

```console
$ sysml report.sysml -render-document E::R
✓ package E
sysml: document E::R content t query E::Q: query E::Q references unknown property massss
$ echo $?
2
```

## Common mistakes

### Document structure

| Mistake | Error |
|---|---|
| Rendering a name that is not a document | `... is not a document: one is a part def specializing DocumentQueries::Document` |
| Rendering a name that resolves to nothing | `unresolved reference: <name>` |
| No `title` on a document or section | `missing-title` — the title is required, not defaulted |
| A document nested inside a document | `nested-document` |
| A `title`, `caption` or other attribute that is not a literal string | `invalid-attribute` |

### Paragraphs and runs

| Mistake | Error |
|---|---|
| A paragraph with neither text, a query, nor runs | `missing-text` |
| A paragraph with both text and a query, or runs alongside either | `conflicting-text` / `conflicting-runs` |
| A `Span` without `text` | `missing-run-text` |
| A `Span` style other than `plain`/`emphasis`/`strong`/`code` | `invalid-run-style` |
| A `Link` without a `target` URL | `missing-link-target` |
| A `Ref` without a target | `missing-ref-target` |
| A `Ref` target that is not a content block of the same document | `unknown-ref-target` / `invalid-ref-target` |
| A run that is somehow both kinds at once, or carries nested content | `ambiguous-run` |

### Tables, lists and queries

| Mistake | Error |
|---|---|
| A `Table` or `List` without a query | `missing-query` |
| Two queries on one block | `conflicting-query` |
| A query name that resolves to nothing, or not to a document query | `unknown-query` |
| A list style other than `bullet`/`number` | `list style must be "bullet" or "number", got "roman"` |
| A `groupBy` column the query does not project | `unknown-group-column` |
| Binding a parameter the query does not declare | `unknown-parameter` |
| Binding the same parameter twice | `duplicate-binding` |
| Not binding a required parameter | `missing-binding` |
| A binding whose value's type or multiplicity does not fit | `binding-type` / `binding-multiplicity` |

### Diagrams

| Mistake | Error |
|---|---|
| No `source` | `missing-view-source` |
| A plain-element source without a `kind` | `missing-diagram-kind` |
| A `kind` on a declared view (which brings its own) | `conflicting-diagram-kind` |
| A kind other than `tree`/`interconnection`/`state`/`action`/`table`/`sequence` | `unsupported-diagram-kind` |
| A direction other than `TB`/`LR`/`RL`/`BT` | `invalid-direction` |
| A direction on a kind that is not a directed graph (e.g. sequence) | `unsupported-direction` |
| A palette other than `okabe-ito`, `tol-bright`, `tol-muted`, `tol-light`, `brewer-set2`, `brewer-dark2`, `viridis` or `cividis` | `invalid-palette` |
| A palette on a kind with no DOT or PlantUML form (table) | `unsupported-palette` |

### Query execution

| Mistake | Error |
|---|---|
| Filtering, ordering or projecting a property no source element has | `unknown-property` |
| A `WhereType` name that is neither a metamodel type nor resolvable | `unknown-classification` |
| An unknown comparison operator | `invalid-operator` |
| `OrderBy` over incomparable value types, or `missing`/`multiple = "error"` triggered | `invalid-order` |
| An unknown relationship kind or direction | `unknown-relationship` |
| A query invoking a query that does not exist | `unknown-invocation` |
| A query invocation cycle, or exceeding a depth, count or visit budget | `invocation-cycle` / `invocation-depth` / `invocation-budget` / `visit-budget` — the engine terminates rather than hangs |

### HTML

| Mistake | Error |
|---|---|
| `-html-*` flags without `-doc-form html` | flag-conflict error |
| `-html-css` naming a file that cannot be read | the file's own read error |
| `-html-css` with a scheme other than `http`, `https` or a protocol-relative URL | read as a path, so a URL of another scheme is a missing file |
| `-html-css` or `-html-no-default-css` with `-html-fragment` | refused — a fragment has no place for a stylesheet; style the page you embed it in |
| `-html-default-css` beside any flag that loads a model or writes something else | refused — it writes the default stylesheet and nothing else |
| `-doc-title-page`, `-doc-toc` or `-doc-number-sections` with `-doc-form markdown` | flag-conflict error — Markdown has no page shell to put them in |
| `-pdf-engine` with `-doc-form html` | flag-conflict error — HTML needs no external converter |

### PDF

| Mistake | Error |
|---|---|
| `-doc-form pdf` without `-o` | refused — a PDF is a binary artifact |
| The selected converter not installed | `tool-missing`, naming the tool, its `OPENSYSML_*` override variable and the other engines |
| The converter exits non-zero | `tool-failed`, with the tool's own words |
| `-pdf-engine` without `-doc-form pdf` | flag-conflict error |

## Limitations

Each of these is a current fact about the implementation, not a design
position; they are tracked in the project's compliance record.

- **The vocabulary is non-normative.** `DocumentQueries` is an OpenSysML
  extension; other SysML v2 tools will parse models that use it but will not
  render documents from them.
- **Query-generated runs cannot cross-reference.** Column runs restyle
  query-produced text and can link to external URLs, but `Ref`-style
  cross-references to other content blocks apply to statically-authored runs
  only.
- **Cross-document links assume one output directory.** A `Ref` may target
  a content block or the root of another document (see the authoring
  chapter's cross-document pattern), and `-render-documents` writes the
  linked set together. Rendering one document alone still succeeds, but its
  cross-document links point at the target's expected file name and dangle
  until that document is rendered into the same directory.
- **Captions are marked, not inferred.** The Markdown dialect writes a
  `<!-- caption -->` comment line before every table and diagram caption; the
  PDF backend styles only marked lines as captions, and an emphasized line
  without the marker stays an ordinary paragraph. HTML needs no marker, as it
  writes a real `<caption>` or `<figcaption>`. A marker whose next line is
  not a fully emphasized caption is a typed `dangling-caption` error.
- **HTML and PDF are CLI-only.** The REPL, gRPC and LSP surfaces render
  Markdown only.
- **PDF reproducibility is per-toolchain.** Byte-identical output holds for
  one pinned converter toolchain; different converter versions or fonts
  produce different bytes. Prince is recognized but not provisioned by the
  toolchain download script (it is commercial).
- **Only HTML presentation is configurable.** `-html-css` and its companions
  restyle an HTML document, but there is no Mermaid theme option and the PDF
  path's stylesheet is fixed, so for PDF a diagram's caption and direction plus
  the deliverable flags are the whole presentation surface.
- **`-json` does not combine with `-render-document`** — the document IR is
  not reported as JSON.
- **Editor rendering is on demand.** The Render Document command re-renders
  when invoked; there is no live preview that updates as you type (the
  `renderChanged` notification tells a client *when* to re-request).
