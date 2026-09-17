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
   model, or about the objects a session has instantiated from it: bound to
   one, the same operations read the values it holds now and `Verdicts`
   checks its constraints and requirements.
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
| `Project` | Turn elements into rows of named, typed columns |
| `Objects` | The objects the session holds that are of a type, each under its path — the one operation that reads objects rather than elements; every other operation accepts an object where it accepts an element and reads what the object holds ([Objects the session holds](query-cookbook.md#objects-the-session-holds)) |
| `Verdicts` | One row per assertion about each source row's object — the held object, or the element's declared one — and the objects it holds: the constraint, requirement, `satisfy` or verification case checked, with `path`, `kind`, `verdict` (`holds`, `violated`, `undecided`), `condition`, `reason` and `verification` for the filters and projections to read ([Which constraints and requirements hold](query-cookbook.md#which-constraints-and-requirements-hold)) |

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

## Which query is which

OpenSysML answers four kinds of question that are each called a query, and
this manual is about one of them. They differ in what they read and what they
return:

| Surface | Reads | Returns | Cannot see |
|---|---|---|---|
| **Document query** — `calc def` specializing `DocumentQueries::Query`, run by `-run-query`, `%run-query`, `RunDocumentQuery` or a document block | The model's elements, and, bound to or collecting one, the objects a session holds — the values they hold now, and through `Verdicts` whether their constraints, requirements and `satisfy` assertions hold | Ordered rows of typed cells — elements, objects, verdicts and values — that tables, lists and paragraphs render | Objects no session holds; a run's trace |
| **API `Query`** — the SysML v2 API & Services query over a loaded model, `model.query(...)` in Python, and the OSLC text `-query` takes ([reference](../reference/api.md#sysml-v2-api--services-query)) | The model's elements alone, by the standard's closed set of properties | Elements as `@id`, `@type` and their properties, in declaration order | Traversal, specialization, computed values, objects, verdicts — the standard's query model is deliberately weak, which is what makes other tools able to send it |
| **`Evaluate`** — `-eval`, `%eval`, `%eval in`, the `Evaluate` RPC | One expression in one scope: a declaration's namespace, reading what the model states, or one object, reading what it holds after its behaviors ran | One value | Anything the expression does not name; it discovers no assertion and walks no object graph, so a check is one expression at a time |
| **`solve`** — `%solve`, `%check`, `%explain`, the `solve` engine | A constraint, requirement or `satisfy` assertion and what is already fixed — the values an object holds, or the ones the model declares | Values that would satisfy it (one witness), or `unsat` with the conditions that conflict | What *does* hold: satisfiability is not evaluation, which `%constraint`, `%satisfy`, `%validate` and `Verdicts` do |

So "list every requirement this rover violates, with the part it is about" is
a document query over `Verdicts`; "which parts have mass over 10 kg" is a
document query over elements or a `WhereFeature` over held objects; "give me
every `PartUsage` for MATLAB" is the API `Query`; "what is `rover.battery.charge`
now" is `%eval in`; and "is there any charge at which `powerMargin` holds" is
`%solve`. The rest of this manual covers the first row.

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
