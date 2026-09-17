# Document Generation Manual

OpenSysML generates documents from SysML v2 models: queries written in the
model collect and shape elements, a document definition written in the model
arranges them into sections, paragraphs, tables, lists and diagrams, and the
engine renders the result as Markdown, HTML or PDF. The whole document —
content and structure alike — lives in the model, so the report regenerates
whenever the model changes.

This manual is the guide to that pipeline. Every SysML snippet in it parses
and renders with the current `sysml` binary, and every rendered output shown
was produced by it.

1. [Introduction and concepts](introduction.md) — queries, documents, content
   blocks, and the model → queries → document → Markdown/HTML/PDF pipeline
2. [Getting started](getting-started.md) — the smallest working document, end
   to end
3. [Query cookbook](query-cookbook.md) — recipes for collecting, filtering,
   sorting, traversing and projecting model elements
4. [Which query is which](query-kinds.md) — document queries beside the API
   `Query`, `Evaluate`, `all T` and `solve`: what each reads, returns and
   cannot see, and where the runtime's state and event queries sit
5. [Document authoring](authoring.md) — sections, paragraphs, inline runs,
   links, cross-references, tables (including grouped tables), lists and
   diagrams
6. [Outputs](outputs.md) — Markdown, semantic HTML and its stylesheets, the
   PDF engines and their options, and what determinism is guaranteed
7. [Interfaces](interfaces.md) — CLI flags, REPL commands, the gRPC and Python
   APIs, and VS Code/LSP authoring support
8. [A complete worked example](worked-example.md) — a telescope mass report
   with its full source and full rendered output
9. [Limitations and troubleshooting](troubleshooting.md) — the typed error
   catalog and the current limitations

The document-query vocabulary is a **non-normative OpenSysML extension** — it
is not part of the OMG SysML v2 or KerML standard. Models that use it remain
standard SysML v2: the vocabulary is ordinary `calc def`s and `part def`s from
the bundled `DocumentQueries` library package, and using it does not alter the
language's semantics.
