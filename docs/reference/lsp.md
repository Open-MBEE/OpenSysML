# LSP extensions

`sysml-lsp` speaks the Language Server Protocol, plus the methods on this
page. They are not part of the protocol, so a client must ask for them by name,
and the server announces that it serves them by advertising, in the `initialize`
result:

```json
{ "capabilities": { "experimental": {
    "openSysmlRender": true, "openSysmlRenderDocument": true, "openSysmlStdlibContent": true } } }
```

`openSysmlRender` covers the view-rendering methods, `openSysmlRenderDocument`
the document-rendering ones, and `openSysmlStdlibContent` the request that serves
the bundled standard library's text.

A client that does not see that capability must not send these methods. That is
how a new client and an older server stay compatible.

Everything here is read-only: it renders what a document says and never writes
to it. The renderings are the same ones [`%view`](repl-commands.md) and `sysml -view`
produce, from the same renderer.

## Strict conformance (setting)

Whether the server judges a document as conforming SysML v2 (reporting notation
only OpenSysML accepts as an error instead of a warning) is controlled by a boolean
setting, `strictConformance`. It is read from `initialize`'s `initializationOptions`
and from `workspace/didChangeConfiguration`, in any of the three shapes clients
nest settings in:

```json
{ "strictConformance": true }
{ "sysml": { "strictConformance": true } }
{ "sysml.strictConformance": true }
```

A payload that does not mention it leaves the mode alone, and a value that is not
a boolean is ignored rather than read as either answer. Changing it republishes
the diagnostics of every open document, so the editor never keeps the other
mode's verdict. A client that cannot send settings can start the server with
`-strict` instead. Both correspond to the CLI's `-strict` and the REPL's `%strict`;
[the guide](../guide/03-command-line.md#strict-conformance) explains what the mode
changes.

## `opensysml/render` (request)

Renders one view of a document.

```json
{
  "textDocument": { "uri": "file:///tmp/kit.sysml" },
  "view": "KitViews::widgetTree",
  "form": "mermaid"
}
```

| Field | Meaning |
| --- | --- |
| `textDocument.uri` | The document to render. It must be one the session holds — an open document, or a workspace file the server read. |
| `view` | The qualified name of a view the document declares, a pseudo-view (below), or omitted. |
| `form` | `mermaid`, `text`, `markdown` or `dot`. Omitted writes the machine form of the rendering's kind: `markdown` for a table, `mermaid` for every other kind. `dot` writes Graphviz DOT for a `tree`, `interconnection`, `state` or `action` rendering, without needing Graphviz installed. |

Omitting `view` renders the view the document declares. If the document declares
several, the request is ambiguous and fails, naming them
(`declares 6 views (KitViews::widgetActions, …); name the one to render`) rather
than picking one. If it declares none, the request fails and points at the pseudo-views.

A `form` the rendering kind cannot be written in (Mermaid for a table, Markdown for a
diagram, DOT for a table or a sequence) is refused, and the reply names the form the kind
does use. A `form` that is not one of the four is refused, and the reply names all four.

**Pseudo-views.** A document that is still being written usually declares no `view`,
so a rendering can be requested as if one had been declared:

| `view` | Renders |
| --- | --- |
| `#tree` | Everything the document declares, as a tree |
| `#interconnection` | …as an interconnection diagram |
| `#state` | …as a state diagram |
| `#action` | …as an action flow |
| `#sequence` | …as a message sequence |
| `#table` | …as an element table |
| `#state:Kit::WidgetStates` | One element the document declares, here as a state diagram |

A pseudo-view adds nothing to the model and nothing to the symbol index: the
exposed set is passed to the renderer directly, and the result says so in
`stated`, as `no view declared; rendering Kit::WidgetStates directly`.

The result, for `{"view": "KitViews::widgetTree"}` over a document declaring
`part def Widget { part cog : Cog; part gear : Cog; connect cog to gear; }`:

```json
{
  "view": "KitViews::widgetTree",
  "kind": "tree",
  "stated": "",
  "form": "mermaid",
  "artifact": "%% KitViews::widgetTree — tree rendering\nflowchart TD\n  n0[\"part def Kit::Widget\"]\n  n1[\"part cog (Cog)\"]\n  n0 --- n1\n  …",
  "nodes": [
    {
      "id": "n0",
      "kind": "part def",
      "name": "Kit::Widget",
      "detail": "",
      "origin": {
        "uri": "file:///tmp/kit.sysml",
        "range": { "start": { "line": 1, "character": 1 }, "end": { "line": 6, "character": 1 } },
        "selectionRange": { "start": { "line": 1, "character": 10 }, "end": { "line": 1, "character": 16 } }
      }
    },
    {
      "id": "n1",
      "kind": "part",
      "name": "cog",
      "detail": "Cog",
      "parent": "n0",
      "origin": { "uri": "file:///tmp/kit.sysml", "range": { "…": "…" } }
    }
  ],
  "edges": [{ "from": "n0", "to": "n1", "label": "", "kind": "connection" }],
  "notices": [],
  "version": 7
}
```

| Field | Meaning |
| --- | --- |
| `view` | The view rendered, by qualified name; empty for a pseudo-view. |
| `kind` | `tree`, `interconnection`, `state`, `action`, `sequence` or `table`. |
| `stated` | How the kind was decided — the rendering the view names, the standard view definition it specializes, or that no view was declared. Empty when the view took the default. |
| `artifact` | What to draw or show: a Mermaid diagram, a Graphviz DOT graph, the text form, or a Markdown table. |
| `nodes`, `edges` | What the artifact is made of, so a client can map a click on it back to the source. A node's `parent` is the node containing it, when one does. An edge's `kind` is `connection`, `transition`, `succession` or `flow`. |
| `rows`, `columns` | A table rendering's cells, in place of nodes and edges. |
| `origin` | Where the element was declared, as a document URI, the `range` of the whole declaration and, when the declaration names one, the `selectionRange` of the identifier alone. A client highlights the element whose `range` holds the cursor and navigates to its `selectionRange`, as `textDocument/definition` does. Absent for an element with no locatable declaration: a standard library symbol the index served from its cache, or a step a lowering sequenced without a declaration of its own, carries none rather than a bogus range. |
| `notices` | What the rendering could not represent, as the text form reports it. |
| `version` | The version of the document the rendering was made from, so a client can tell a rendering of the text it is showing from a stale one. |

A view that asks for a rendering this implementation does not produce (`geometry`,
`textual`) fails with the reason, e.g.
`KitViews::widgetGeometry: geometry rendering (view def GeometryView) is not supported`.

## `opensysml/views` (request)

Lists the views a document declares, which is what fills a diagram panel's view
picker.

```json
{ "textDocument": { "uri": "file:///tmp/kit.sysml" } }
```

```json
{
  "views": [
    { "name": "KitViews::widgetParts", "kind": "interconnection", "supported": true },
    { "name": "KitViews::widgetSequence", "kind": "sequence", "supported": true },
    {
      "name": "KitViews::widgetGeometry",
      "kind": "geometry",
      "supported": false,
      "reason": "KitViews::widgetGeometry: geometry rendering (view def GeometryView) is not supported"
    }
  ],
  "pseudoViews": ["#action", "#interconnection", "#sequence", "#state", "#table", "#tree"]
}
```

Views are listed in qualified-name order. An unsupported one stays in the listing,
with `supported: false` and the reason, so a client can say why it cannot be
drawn instead of hiding it. `pseudoViews` lists the supported `#<kind>` specs
in sorted order; a client can use it to offer pseudo-views without duplicating
the server's list of supported kinds.

## `opensysml/documents` (request)

Lists the document definitions the workspace holds (the `part def`s
specializing `DocumentQueries::Document`), which is what fills a Render
Document command's picker. It takes no parameters.

```json
{
  "documents": [
    { "name": "Observatory::MassReport", "uri": "file:///tmp/observatory.sysml" }
  ]
}
```

Documents are listed in qualified-name order; `uri` is the file that declares each
one. Standard-library documents are not listed: the listing covers what the
workspace's own files declare.

## `opensysml/renderDocument` (request)

Renders one document definition to Markdown: the document is compiled to a plan,
its queries are executed against the workspace model, and the result is written
the way the REPL's `%render-document` and `sysml -render-document` write it. It is
the same pipeline, run against the same workspace the diagnostics are computed from.

```json
{ "name": "Observatory::MassReport" }
```

```json
{ "name": "Observatory::MassReport", "markdown": "# Telescope Mass Report\n…" }
```

`name` is the qualified name of a document definition, as `opensysml/documents`
lists it. If the name resolves to nothing, names an element that is not a
document, or names a document whose planning or query execution fails, the
request fails with the typed error's message (for example `Observatory::Subsystem
is not a document: one is a part def specializing DocumentQueries::Document`)
rather than crashing or answering with partial output.

## `opensysml/stdlibContent` (request)

The server bundles the standard library, so a definition, reference, hover or
rendering origin may land in a library file no client has on disk. Such a
location is reported under the `sysml-stdlib` scheme, whose path is the file's
path within the library, percent-encoded:

```json
{ "uri": "sysml-stdlib:///Kernel%20Libraries/Kernel%20Data%20Type%20Library/ScalarValues.kerml",
  "range": { "start": { "line": 19, "character": 1 }, "end": { "line": 20, "character": 1 } } }
```

That is where `ScalarValues.kerml` declares `datatype Integer specializes Rational;`.

Positions are in UTF-16 code units of the bundled text, as they are for workspace
files. This request serves that text, so a client can show the location:

```json
{ "uri": "sysml-stdlib:///Kernel%20Libraries/Kernel%20Data%20Type%20Library/ScalarValues.kerml" }
```

```json
{ "text": "standard library package ScalarValues {\n\tdoc\n\t/*\n…" }
```

A URI of another scheme, or one naming no library file, fails with an
invalid-params error. A client that opens the document may then send it the
ordinary requests — hover, definition, references, document symbols, semantic
tokens — against the `sysml-stdlib:` URI; the server answers from the bundled
text, so navigation continues from one library file into another. References
list the workspace's uses of a library element and, with the declaration asked
for, its library declaration; uses inside the library itself are not enumerated.
`textDocument/didOpen` and `didClose` for such a URI are accepted and change
nothing: the text a client sends is ignored in favour of the bundled one, and
closing removes nothing from the library. `textDocument/didChange` is refused
with an invalid-request error — reported to the user through `window/showMessage`
as well — and is never applied: the library is read-only.

The VS Code extension registers a content provider for the scheme that calls this
request, so <kbd>Ctrl</kbd>+click on a library name opens the file in a read-only
editor. Another client needs the same: a provider for `sysml-stdlib` documents
that fetches their text with `opensysml/stdlibContent`.

## `opensysml/renderChanged` (notification, server → client)

```json
{ "textDocument": { "uri": "file:///tmp/kit.sysml" }, "version": 8 }
```

Sent after the analysis that publishes the document's diagnostics, so a client
sees the diagnostics of a version before the notification for it. It is
debounced on the same window the cross-document diagnostics sweep uses, so a burst
of keystrokes costs one notification rather than one per keystroke.

It carries no rendering: the client responds with a fresh `opensysml/render` if it
is showing the document, and does nothing if it is not. This keeps a large
diagram off the wire for a panel nobody is looking at.

## Trying it by hand

The protocol is JSON-RPC over stdio, so the methods can be driven without an
editor: send `initialize`, then `textDocument/didOpen`, then:

```
→ opensysml/render  { "textDocument": { "uri": "file:///tmp/kit.sysml" }, "view": "#tree" }
← { "view": "", "kind": "tree",
    "stated": "no view declared; rendering /tmp/kit.sysml directly",
    "form": "mermaid",
    "artifact": "%%  — tree rendering (no view declared; …)\nflowchart TD\n  n0[\"part def Kit::Widget\"]\n…",
    "nodes": [ … ], "edges": [ … ], "notices": [], "version": 1 }
```

The VS Code extension's diagram panel is the reference client; see
[the editors guide](../guide/08-editors.md#the-diagram-panel).
