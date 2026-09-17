# Introduction and Concepts

## Why generate documents from the model

A systems engineering report — a mass rollup, a requirements traceability
matrix, an interface listing — restates what the model already knows. Writing
it by hand means it is wrong the day after the model changes. OpenSysML's
document-generation engine instead treats the report itself as part of the
model: *queries* say what to collect, a *document definition* says how to
arrange it, and rendering is a compilation step. Regenerating the document is
rerunning one command.

## The pipeline

```
model → queries → document plan → document tree → Markdown or HTML → (PDF)
```

1. **The model** is ordinary SysML v2: parts, attributes, requirements,
   connections, views. Nothing about it is document-specific.
2. **Queries** are `calc def`s specializing `DocumentQueries::Query`. Each one
   composes library operations — collect owned elements or descendants,
   filter by type, name, metadata or attribute value, traverse relationships,
   order, project columns — into a reusable, parameterized question about the
   model.
3. **A document definition** is a `part def` specializing
   `DocumentQueries::Document`. Its nested parts are the document's content in
   declaration order: sections, paragraphs, tables, lists and diagrams. Blocks
   that carry data name a query and bind its parameters.
4. The engine compiles the definition into an immutable **document plan**,
   validating structure, query references and bindings up front — a mistake is
   a typed error at planning time, not a half-rendered artifact.
5. Evaluating the plan runs every query and produces an immutable,
   backend-neutral **document tree**: the fully-resolved title, sections,
   text runs, table rows, list items and diagram renderings.
6. A backend writes the tree out. The Markdown backend writes deterministic
   CommonMark; the [HTML backend](outputs.md#html) writes semantic HTML whose
   `sysml-` classes and `data-` attributes keep each node's model facts, so a
   stylesheet can address them. The PDF path converts the Markdown with an
   external engine ([WeasyPrint, pandoc or Prince](outputs.md#pdf)).

## The vocabulary

Everything the engine understands is declared in one bundled library package,
`DocumentQueries`. It is a non-normative OpenSysML extension — the types are
ordinary SysML v2 declarations, so a model using them still parses everywhere,
but only OpenSysML gives them document semantics.

**Query operations** (each a `calc def` taking and returning ordered element
sequences):

| Operation | What it does |
|---|---|
| `OwnedElements` | The direct children of each source element |
| `Descendants` | Children transitively, to a depth bound |
| `Ancestors` | Owners transitively, to a depth bound |
| `RelatedElements` | Elements reachable over one named relationship kind — specialization, subsetting, redefinition, typing, connection, allocation, satisfaction or verification — outgoing or incoming, to a depth bound |
| `WhereType` | Keep elements of a metamodel type |
| `WhereMetadata` | Keep elements annotated with a metadata definition |
| `WhereName` | Keep elements whose name passes a comparison |
| `WhereFeature` | Keep elements whose attribute value passes a comparison |
| `OrderBy` | Sort by a property, with explicit missing- and multiple-value policies |
| `Project` | Turn elements into rows of named, typed columns — declared properties, computed `Column` expressions and `RelatedColumn` cells holding the elements a relationship reaches from each row, their count or whether there are any |
| `Objects` | The objects the session holds that are of a type, each under its path — the one operation that reads objects rather than elements; every other operation accepts an object where it accepts an element and reads what the object holds ([Objects the session holds](query-cookbook.md#objects-the-session-holds)) |

**Document content blocks** (each a `part def` nested inside a document or
section):

| Block | What it renders |
|---|---|
| `Section` | A titled heading with nested content |
| `Paragraph` | Static text, inline runs, or one query's values |
| `Span`, `Link`, `Ref` | Inline runs inside a paragraph: styled text, a URL link, a cross-reference to another block |
| `Table` | A query's rows as a table, optionally grouped by a column |
| `List` | A query's values as a bullet or numbered list |
| `Diagram` | A view or element drawn by the view engine, as a Mermaid diagram or table |

## What "deterministic" means here

The same model renders to byte-identical Markdown or HTML every time: queries
preserve model declaration order unless an `OrderBy` says otherwise, ordering policies
for missing and duplicate keys are explicit parameters rather than accidents,
and the renderer escapes content so model text can never corrupt document
structure. PDF output adds an external converter to the loop; its guarantees
are narrower and spelled out in [Outputs](outputs.md#determinism).

## Where to go next

[Getting started](getting-started.md) builds the smallest working document.
If you already have the shape in mind, the
[query cookbook](query-cookbook.md) and
[document authoring](authoring.md) chapters are reference-style and can be
read in any order.
