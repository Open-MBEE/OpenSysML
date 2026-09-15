# LSP extensions

`sysml-lsp` speaks the Language Server Protocol, plus the methods on this
page. They are not part of the protocol, so a client must ask for them by name,
and the server announces that it serves them by advertising, in the `initialize`
result:

```json
{ "capabilities": { "experimental": {
    "openSysmlRender": true, "openSysmlRenderDocument": true, "openSysmlStdlibContent": true,
    "openSysmlApplyModelEdit": true, "openSysmlDebug": true } } }
```

`openSysmlRender` covers the view-rendering methods, `openSysmlRenderDocument`
the document-rendering ones, `openSysmlStdlibContent` the request that serves
the bundled standard library's text, `openSysmlApplyModelEdit` the request
that turns model operations into text edits, and `openSysmlDebug` the
[`opensysml/debug/*`](#opensysmldebug-requests) requests that run a drawn
behavior and report where it stands.

A client that does not see that capability must not send these methods. That is
how a new client and an older server stay compatible.

None of these methods writes to a document. The rendering methods render what a
document says, with the same renderer [`%view`](repl-commands.md) and `sysml -view`
use; `opensysml/applyModelEdit` computes the edits that would make a document say
something else and hands them back for the client to apply, so the change lands
in the editor's own buffer and undo history.

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
| `form` | `mermaid`, `text`, `markdown`, `dot` or `plantuml`. Omitted writes the machine form of the rendering's kind: `markdown` for a table, `mermaid` for every other kind. `dot` writes Graphviz DOT for a `tree`, `interconnection`, `state` or `action` rendering, without needing Graphviz installed; `plantuml` writes PlantUML in the Pilot visualizer's B&W style for those kinds and a `sequence`, without needing a PlantUML jar. |
| `palette` | Optional. A palette the `dot` and `plantuml` forms fill nodes with by keyword family: `okabe-ito`, `tol-bright`, `tol-muted`, `tol-light`, `brewer-set2`, `brewer-dark2`, `viridis` or `cividis` ([the palettes](../project/view-rendering-forms.md#palettes)). Omitted or empty draws black and white. A `mermaid` artifact notes the palette as not represented; `text` and `markdown` ignore it. |

Omitting `view` renders the view the document declares. If the document declares
several, the request is ambiguous and fails, naming them
(`declares 6 views (KitViews::widgetActions, …); name the one to render`) rather
than picking one. If it declares none, the request fails and points at the pseudo-views.

A `form` the rendering kind cannot be written in (Mermaid for a table, Markdown for a
diagram, DOT for a table or a sequence, PlantUML for a table) is refused, and the reply names
the form the kind does use. A `form` that is not one of the five is refused, and the reply names
all five. A
`palette` that names none of the eight is refused, and the reply names them
(`unknown palette "rainbow"; the palettes are okabe-ito, …, cividis`).

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
  "artifact": "%% KitViews::widgetTree — tree rendering\nflowchart TD\n  n0[\"Kit::Widget<br>«part def»\"]\n  n1[\"cog : Cog<br>«part»\"]\n  n0 --- n1\n  …",
  "nodes": [
    {
      "id": "n0",
      "kind": "part def",
      "name": "Kit::Widget",
      "type": "",
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
      "type": "Cog",
      "detail": "",
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
| `artifact` | What to draw or show: a Mermaid diagram, a Graphviz DOT graph, a PlantUML diagram, the text form, or a Markdown table. |
| `nodes`, `edges` | What the artifact is made of, so a client can map a click on it back to the source. A node's `kind` is the keyword the notation declares it with (`part def`, `state`), its `name` the qualified name of an element the view exposes or the simple name of one nested in it, its `type` the declared type of a typed usage (`Cog` for `part cog : Cog`, empty otherwise), and its `detail` the notes the artifact draws after the name (`initial`, `already shown`); a client never parses the type out of the detail. A node's `parent` is the node containing it, when one does. An edge's `kind` is `connection`, `transition`, `succession` or `flow`. |
| `rows`, `columns` | A table rendering's cells, in place of nodes and edges. |
| `origin` | Where the element was declared, as a document URI, the `range` of the whole declaration and, when the declaration names one, the `selectionRange` of the identifier alone. A client highlights the element whose `range` holds the cursor and navigates to its `selectionRange`, as `textDocument/definition` does. Absent for an element with no locatable declaration: a standard library symbol the index served from its cache, or a step a lowering sequenced without a declaration of its own, carries none rather than a bogus range. |
| `notices` | What the rendering could not represent, as the text form reports it. |
| `fqn` | On a node: the qualified name `opensysml/applyModelEdit` targets the node's declaration by, each name quoted on its own as the notation spells it, so `'x::y'` (one name) and `x::y` (`y` in `x`) are two targets. Absent for a node whose declaration is not in the document — a library element, a step a lowering sequenced — or is reached only through an unnamed one, so a client offers no edit on it but a layout. On an edge: the qualified name of the declaring connection, transition, succession or flow, absent likewise. |
| `notation` | On a node with an `fqn`: the keyword the declaration was written with (`part def`, `port`, `state`), which is the `memberKind` a `move` asks the new owner to admit. |
| `declaration` | On a node or edge the document declares but no qualified name reaches — an unnamed transition, a connection inside an unnamed part — the `range` of that declaration, in place of `fqn`. `opensysml/applyModelEdit`'s `setLayout` and `setRoute` take it as the target of an inline annotation, since a view body has no name to state one about; no other operation reaches such an element. |
| `owners` | On a node with an `fqn`: the namespaces declaring it, nearest first, each as its `fqn` and whether it is a `feature`, whether or not the view draws them. A client writes a connection into the nearest owner two nodes share, or into the document when they share none, and spells each end from there — through a feature by `.`, into any other namespace by `::` (`tank.fuelOut`, `Car::tank.fuelOut`). Absent for a top-level declaration. |
| `palette` | What a diagram of this kind offers to add, in the document's language: `members` are member kinds for `applyModelEdit`'s `addMember`, `connections` are connection kinds for `addConnection`, `typed` are the `members` that may be given a `type`, and `owners` lists, for each member only some bodies declare (`subject`, `actor`, `stakeholder` in a requirement or case, `objective` in a case), the ids of the nodes whose declaration opens such a body — a client offers such a member on those nodes alone, and a member absent from `owners` on every declared node. The same admission decides where a node may be moved: `owners` also lists, under the `notation` of each drawn node that only some bodies declare (`entry action`, `subject`), the nodes that admit it, so a client offers as the new owner of a node the declared nodes, and the document, that its `notation` finds in `owners` — or every one when it is absent. An `interconnection` offers parts, ports, items, attributes and the connection kinds; a `state` diagram states and transitions; an `action` or `sequence` diagram actions, control nodes and successions; a `tree` every kind the language has. Absent for a kind that is not edited from a diagram — a `table`, whose rows name no owner or endpoint to act on, included. |
| `x`, `y`, `width`, `height`, `collapsed` | On a node: where the model places it, from a `DiagramLayout::Layout` annotation, in pixels from the canvas's top-left corner with y increasing downward. Absent for a node the model does not place; `width` and `height` only when the annotation sizes it; `collapsed` only when it says so. |
| `route` | On an edge: the waypoints a `DiagramLayout::Route` annotation gives it, as an array of `{"x", "y"}` in the same coordinates. Absent for an edge with none. |
| `canvas` | The drawing surface the view states with a `DiagramLayout::Canvas` annotation: its `unit` when given, and `width` and `height` together when the annotation sizes it (an explicit `0` is a size). Absent for a view stating none and for every pseudo-view. |
| `version` | The version of the document the rendering was made from, so a client can tell a rendering of the text it is showing from a stale one. |

A node placed by `metadata Layout about cog { x = 120; y = 80; width = 90; height = 40; }`
in the view's body arrives as
`{ "id": "n1", …, "x": 120, "y": 80, "width": 90, "height": 40 }`; the `artifact`
carries the same geometry in the writer's own notation (`%% layout: n1 x=120 y=80 w=90 h=40`
in Mermaid, `at (120, 80) size 90×40` in text). A client that lets the user drag the
node writes the position back with `opensysml/applyModelEdit`'s `setLayout`, so the
rendering it gets next carries the new `x` and `y`. See
[Diagram layout annotations](../project/diagram-layout-annotations.md) for how a
position is resolved when a view and the element itself both state one.

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
lists it. An optional `diagramForm`, `"mermaid"` (the default when omitted) or
`"dot"` or `"plantuml"`, is the form every graph-shaped diagram block of the document is written
in — a ` ```dot ` fence of Graphviz DOT under `"dot"`, a ` ```plantuml ` fence under
`"plantuml"`, as `sysml -render-document -diagram-form dot|plantuml` writes; a table-kind
view is a pipe table whichever form. Any other value fails the request with the typed
error's message naming the three forms, as does `"dot"` on a document holding a `sequence`
diagram, which has no DOT form. If the name resolves to nothing, names an element that is not a
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

## `opensysml/applyModelEdit` (request)

Computes the text edits that make a document say what a list of model
operations asks, and returns them as a `WorkspaceEdit` for the client to apply.
The server never writes: the client applies the edit to its own buffer, so the
change takes part in the editor's undo history, and the server learns of it
through `textDocument/didChange` like any typed change, re-analyzes, and sends
`opensysml/renderChanged`. A diagram redraws from the changed document, never
from the operation it asked for.

```json
{
  "textDocument": { "uri": "file:///tmp/kit.sysml" },
  "version": 7,
  "operations": [
    { "kind": "addMember", "owner": "Kit::Widget", "memberKind": "part", "name": "axle", "type": "Axle" },
    { "kind": "addConnection", "owner": "Kit::Widget", "memberKind": "connection", "from": "cog", "to": "axle" }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `textDocument.uri` | The document to edit. It must be one the session holds. |
| `version` | The version of the document the operations were read from — for a diagram action, the `version` of the rendering it was taken on. The edit is computed against exactly that text; any other version is answered `stale` (below). |
| `operations` | Applied together, in order, as one edit: either every operation is written or none is. |

The operations are the source-preserving edits the service's
[`ApplyEdits`](wire-contract.md) also performs, here with `kind` naming the
operation and its fields beside it:

| `kind` | Fields | Writes |
| --- | --- | --- |
| `setValue` | `target`, `value` | A new value expression for an attribute, replacing the old one's bytes alone. |
| `rename` | `target`, `newName` | A new name at the declaration and at every reference that writes it, in the document and in every other document of the workspace — respelled as the editor's rename (`textDocument/rename`) respells them: a shorthand redefinition is declaration and reference at one span, a use of an alias keeps the alias. Refused as `referenced-elsewhere` while the name is written by a document the server cannot rewrite: a bundled library file, or one the index holds without the workspace holding its source. |
| `addMember` | `owner`, `memberKind`, `name`, `type?`, `multiplicity?`, `value?`, `specializes?` | A member at the end of the owner's body, indented like its neighbors; an owner declared without a body gets one. `memberKind` is a keyword the language declares members with (`part`, `port def`, `state`, `fork`, KerML `feature`); `type` is legal only for a usage, `specializes` only for a definition. A connector definition (`connection def`, `interface def`, `flow def`, KerML `assoc`, `interaction`) is not a member kind: declared by name alone it lacks the ends the analyzer requires, so the request is refused as `illegal-kind`. |
| `addConnection` | `owner`, `memberKind`, `from`, `to`, `name?`, `type?` | A `connection`, `interface`, `allocation`, `binding`, `flow`, `succession` or `transition` (KerML: `connector`, `binding`, `flow`, `succession`) in the owner's body, with `from` and `to` written as they resolve from the owner's scope (`tank.fuelOut`). |
| `delete` | `target`, `cascade?` | The declaration and the trivia that belongs to it — its own line, a line comment after it and the comment block above it. Refused as `delete-referenced` when something still refers to it, in the document or in another of the workspace, unless `cascade` is set, in which case the referring declarations go too — an import of the target, the usage typed by it, whatever refers to those, in whichever workspace document declares them — until nothing left behind refers to anything removed. Refused as `referenced-elsewhere`, cascade or not, while a document the server cannot rewrite (a bundled library file, or one indexed without its source held) refers to anything the delete would remove. |
| `move` | `target`, `owner` | The declaration — with its body, its comments and the trivia a `delete` takes — removed from where it is and written at the end of the owner's body, as an `addMember` would write it, re-indented to its neighbors; an owner declared without a body gets one, and the empty `owner` is the document. Every reference the move would break is respelled to reach the declaration where it now is, by the shortest qualified name that still resolves to it, and an import the move leaves redundant or dangling is dropped or respelled with the rest. The move rewrites its own document only: it is refused as `referenced-elsewhere` while another document of the workspace refers to the target or anything within it. Refused as `owner-inside-target` when the owner is the target or declared within it, as `illegal-kind` when the owner's body does not admit the target's kind (the `palette`'s admission), as `member-name-taken` when the owner already declares the name, and as `move-referenced` when a reference has no spelling that reaches the moved declaration. |
| `setLayout` | `target` or `declaration`, `view?`, `layout?` | The `DiagramLayout::Layout` annotation placing the target: `layout` is `{ "x", "y", "width"?, "height"?, "collapsed"? }` in the coordinates `opensysml/render` reports, `width` and `height` given together or not at all. With a `view`, the annotation is stated about the target in that view's body and places it there alone; without one, it is written inline in the target's own body and places it in every view that states nothing. An annotation already there is rewritten in place, value by value; a target declared without a body gets one; omitting `layout` removes the annotation with the line it stood on (and the body it alone filled). |
| `setRoute` | `target` or `declaration`, `view?`, `route?` | The `DiagramLayout::Route` annotation of a connection, transition, succession or flow, `route` being its waypoints as an array of `{"x", "y"}`; `view` and an omitted or empty `route` mean what they do for `setLayout`. |
| `setCanvas` | `target`, `canvas?` | The `DiagramLayout::Canvas` annotation of the view `target` names: `{ "unit"?, "width"?, "height"? }`, the sizes together or not at all; omitted, the annotation is removed. |

`target` and `owner` are qualified names, as `nodes[].fqn` and `nodes[].owners[].fqn` in a
rendering give them, each name quoted on its own where the notation requires it; the empty
`owner` is the document. A `view` is the rendering's `view`; the `target` of a `setRoute`
is the edge's `fqn`. A `setLayout` or `setRoute` may give `declaration`, the node's or
edge's `declaration` range, in place of `target` — one or the other, not both — for an
element no qualified name reaches; such an annotation goes inline, and is refused as
`not-named` with a `view`, whose body could not name what it is about. No other operation
takes a `declaration`. Every `declaration` of a request is a range of the version the request
names: when an earlier operation of the same request moves or lengthens the declaration, the
later one still reaches it, and is refused as `unknown-target` only when the earlier one
rewrote the declaration itself. The three layout operations write into the document alone — the
target of an inline annotation, or the view whose body states a view-local one, must be
declared in it, though such a view may place an element another document declares —
and are refused as `not-a-view` when `view` or a canvas `target` is not a view, `not-exposed`
when the view does not expose the target, `not-drawn` when no rendering the annotation
applies in draws the target as the node or edge it positions, and `not-annotated` when
there is nothing to clear. The result is one of three shapes:

```json
{ "version": 7,
  "edit": { "documentChanges": [ { "textDocument": { "uri": "file:///tmp/kit.sysml", "version": 7 },
                                    "edits": [ { "range": { "…": "…" }, "newText": "  part axle : Axle;\n  connect cog to axle;\n" } ] } ] } }
```

A rename or delete that reaches into other documents answers one `TextDocumentEdit` per
document it rewrites, the requested document first:

```json
{ "version": 7,
  "edit": { "documentChanges": [
    { "textDocument": { "uri": "file:///tmp/kit.sysml", "version": 7 },
      "edits": [ { "range": { "…": "…" }, "newText": "Cog" } ] },
    { "textDocument": { "uri": "file:///tmp/fleet.sysml", "version": 12 },
      "edits": [ { "range": { "…": "…" }, "newText": "Cog" } ] } ] } }
```

```json
{ "version": 7,
  "refused": [ { "operation": 0, "failure": "result-invalid", "message": "edit introduces 1 error",
                 "diagnostics": [ { "range": { "…": "…" }, "severity": 1, "message": "unresolved name Axle" } ] } ] }
```

```json
{ "version": 8, "stale": true }
```

| Field | Meaning |
| --- | --- |
| `version` | The document version the answer is about. |
| `edit` | A `WorkspaceEdit` with one versioned `TextDocumentEdit` per document the operations rewrite — the requested document first, at the request's `version`; every other document at the version the server holds for it, or `null` for one it read from disk and has no open buffer of. Each document's edits, applied to the version named, produce the text the operations ask for. Every byte outside the edited spans is unchanged: comments, blank lines and indentation survive. The documents are read and the edits computed together, under one lock, so they agree with each other and with the diagnostics the server had published. |
| `refused` | Why nothing was written. `operation` is the index of the operation at fault, or `-1` when the request as a whole was; `failure` is a stable name (`unknown-target`, `invalid-name`, `owner-unknown`, `owner-inside-target`, `illegal-kind`, `member-name-taken`, `rename-referenced`, `delete-referenced`, `move-referenced`, `referenced-elsewhere`, `not-a-view`, `not-exposed`, `not-drawn`, `not-annotated`, `result-invalid`, …); `message` says it in words. `diagnostics` carries the errors the edited text would have had, located in that text; `referring` names the declarations that still refer to a target whose delete was refused, the reference a rename would capture or a move cannot respell, or the declarations of other documents that refer to what a delete or rename would change and the server cannot rewrite, or to what a move would change — a declaration of another document is qualified by that document. `referrers` names the same declarations one by one, each as `{ "name", "uri" }` with the document declaring it, for a client that groups them by file. |
| `stale` | The client's `version` is not the document's; nothing was computed. The operations may name declarations that version no longer has, or namesakes that replaced them, so a client does not resend them at the newer version: it shows the newer text or rendering and lets the action be taken again. |

An edit across documents is all of them or none: a reference the server cannot follow, or
a document the rewrite would leave with an error, refuses the whole request, and no
document's change is answered alone. Only the requested document's version is named in
the request, so only that one can be answered `stale`; the version each other document's
change carries is the one its edits were computed against, and a client applies the edit
only while every document it names is open and still at that version. A document it names
that the client has no buffer of — one the server read from disk, so that its change
carries no version — the client opens first and asks again, so that every document the
answer names is a buffer, versioned and synced to the server. When one has moved on —
typed into while the request was in flight, or opened since the server read it — the
client applies nothing, says which document moved, and lets the action be taken again on
the newer text; a change is never landed on text the server did not see. The VS Code
extension does exactly that, since the language client library applies a
`TextDocumentEdit` without checking its version: it compares the versions and calls
`workspace.applyEdit` in one turn, and VS Code pins each document to the version it holds
at that call, so a document changing before the edit lands makes the call fail rather than
misapply.

An edit is refused, rather than written, whenever the edited document would
parse or analyze with an error the original did not have. The check is the same
one diagnostics come from, so what the panel refuses is exactly what the editor
would have underlined.

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

## `opensysml/debug/*` (requests)

Runs the state machine or action flow a `state` or `action` rendering draws,
with the same executors the REPL's [`%state` and `%action`
debuggers](repl-commands.md) use, and reports where it stands in the IDs of
that rendering: the `id` of each node and the position of each edge in the
`opensysml/render` result. A client that drew the result overlays the execution
on it — fills the active states, places a token on each node one sits at, flashes
the edges just taken — without a mapping of its own.

Every request answers a *snapshot* (below). A session runs in a runtime built
from the workspace as it is when the session starts, so later edits do not move
the behavior underneath it; see `opensysml/debugChanged` for what an edit does.

### `opensysml/debug/start`

```json
{
  "textDocument": { "uri": "file:///tmp/lander.sysml" },
  "view": "LanderViews::descentStates",
  "target": "Lander::Descent",
  "object": "Lander::lander"
}
```

`view` is a view the document declares, or a built-in `#` view, as for
`opensysml/render`; it must render a `state` or `action` kind. `target` is the
qualified name of the state machine or action the rendering draws — a `state def`
or `state` usage for a state rendering, an `action def` or `action` usage for an
action one. `object`, optional, names a part or object definition or usage; when
it is given the object is instantiated first and performs the behavior, so
`send … via` and references to the performer's features resolve the way they do
under `%instantiate`. An object whose type exhibits or performs the target
already runs it once instantiated, and the session debugs that running behavior
rather than starting a second one beside it; a type running the target under
several usages is refused with `InvalidParams` until `target` names the usage,
as `%state` refuses it. Without an object the behavior runs on its own.

The answer is the initial snapshot: the machine in the state its entry transition
selects, or the action's first token on its start node. The rendering it is
reported in and the runtime it runs in come from one reading of the workspace,
so its `version` is the document version both were built from; an edit that
lands while the session is being built is handled as `opensysml/debugChanged`
describes — the snapshot is answered under the new IDs, or the start is refused
with `InvalidParams` when the edit rewrote the behavior. Errors are answered with
`InvalidParams` when the request itself is wrong — a view of another kind, a
target the document does not declare or that is not a behavior the kind draws, an
object that does not exist — and as a plain error when the behavior cannot be
initialized (no entry transition, an initial node the flow lacks).

### `opensysml/debug/step`, `opensysml/debug/continue`, `opensysml/debug/stop`

```json
{ "session": "debug-1" }
```

`step` moves the behavior by one step: for an action, one token move (a token
leaving a node, a fork spawning its branches, a join firing); for a state machine,
the next of a change condition firing, an event being dispatched, or a round of
`do` behaviors, in that order of preference — the same step `%state` takes, so
an event due later is dispatched too, and the clock moves to its instant. A
step that finds nothing to do leaves the machine `suspended` (quiescent) or an
action `waiting` on the clock.

`continue` runs until the behavior completes, waits for something the clock or a
signal must bring, reaches a breakpoint, or spends the runtime's step budget,
which it reports as a failure. The clock does not move: a behavior waiting on
`accept after` stays `waiting` until `advance`, a machine whose only pending
event is due later included.

`stop` ends the session and releases its runtime. The session's ID is then
unknown to the server, and any request naming it is answered with
`InvalidParams`.

### `opensysml/debug/send`

```json
{ "session": "debug-1", "signal": "Lander::Abort", "args": { "reason": "\"fuel\"", "level": "1 + 2" } }
```

Posts a signal to the behavior, as `%send` does. `signal` names a signal
definition by qualified name, or by the name the target's document declares it
under; a name that declares nothing is delivered by name alone and may carry no
arguments. Each argument is a SysML expression evaluated in the scope the target
is declared in, bound to the signal feature it is keyed by. A signal the behavior
accepts nowhere it now stands is refused with `InvalidParams` rather than queued
to be lost; an accepted one is queued, and the next `step` or `continue`
delivers it.

### `opensysml/debug/advance`

```json
{ "session": "debug-1", "time": 5 }
```

Moves the runtime's clock forward by `time`, a finite duration of at least 0 in
the clock's units, dispatching what falls due on the way: timed transitions
fire, `accept after` waits end and the tokens move on. A breakpoint reached on
the way stops the advance short, the clock held at the instant it was reached,
and the rest of the duration is for the next `advance`. A `time` that is
negative, infinite or not a number is refused with `InvalidParams`.

### `opensysml/debug/breakpoints`

```json
{ "session": "debug-1", "nodeIds": ["n4", "n7"] }
```

Replaces the session's breakpoints with the nodes named, by the `id` of the
render result the snapshot's `version` refers to. A node the rendering does not
draw, or that draws nothing that runs — the root, a title — is refused with
`InvalidParams`, and the breakpoints stand as they were. A run (`step`,
`continue`, `advance`) stops when a token arrives at a breakpoint node, a
breakpoint state becomes active, or a transition routes through a breakpoint
pseudostate — as the dispatch completes, so a state left again at the same
instant is paused on too; the snapshot then reports `suspended` with `pausedAt`
naming the node, and the next run resumes past it. Breakpoints are
kept on the runtime node, so they follow a node whose `id` changes when the
rendering is redrawn.

### The snapshot

```json
{
  "protocol": 1,
  "session": "debug-1",
  "kind": "state",
  "view": "LanderViews::descentStates",
  "target": "Lander::Descent",
  "object": "Lander::lander",
  "root": "n0",
  "version": 3,
  "state": "suspended",
  "reason": "paused at breakpoint braking",
  "time": 5,
  "tokens": [],
  "activeStates": ["n2", "n4", "n9"],
  "taken": [ { "index": 3, "from": "n3", "to": "n4" } ],
  "queue": [ { "event": "Touchdown", "at": 12.5 } ],
  "breakpoints": ["n4"],
  "pausedAt": "n4",
  "notes": []
}
```

| Field | Meaning |
|---|---|
| `protocol` | The version of this shape, `1`. It moves when a field a client relies on changes meaning. |
| `session` | The session's ID, to name in later requests. |
| `kind` | `state` or `action`. |
| `view`, `target`, `object` | What was started, `target` and `object` as qualified names. `object` is absent when none performs the behavior. |
| `root` | The `id` of the node drawing the behavior itself. |
| `version` | The document version whose `opensysml/render` result the IDs below belong to, as that result's `version` reports it. |
| `state` | `ready` for a behavior not yet initialized, `running` while there is work at the current instant, `waiting` for a behavior parked on the clock or a signal, `suspended` for one stopped at a breakpoint or a quiescent machine, `completed`, `failed`, or `ended` for a session that is over. |
| `reason` | Why, for the last four: what is waited for, the breakpoint, the runtime's error, or why the session ended. |
| `time` | The runtime clock's current instant. |
| `tokens` | Of an action: each control token, with `id`, the `node` it is at, `placed` (false when it runs somewhere the rendering does not draw — a statement inside a node, a flow deeper than the rendering lowers — and `node` is the innermost drawn node around it), `via` (the edge it arrived by), `awaiting` (the join edges it still waits for), `waiting` (the signal or instant an `accept` parks it on) and `due` (the instant a timed wait ends). Empty for a machine. |
| `activeStates` | Of a machine: the `id` of every active state — each state a token sits in with the composite states and regions enclosing it, outermost first, region by region — so a client fills them all without walking the rendering's nesting. Empty for an action. |
| `taken` | The edges traversed since the previous snapshot, in order: the transitions a machine fired, or the successions tokens moved along. Each is its `index` among the render result's edges, with `from` and `to` for a client keyed by endpoints. |
| `queue` | Events the behavior has yet to take: a machine's queued events with the instant each is due `at`, and messages `pending` on the runtime's bus. |
| `breakpoints` | The breakpoint nodes, by `id`, sorted. |
| `pausedAt` | The breakpoint node the last run stopped at; absent when it stopped for another reason. |
| `notes` | What the runtime noted about the last request's run without changing it — a choice it made among enabled alternatives, a guard it could not evaluate — one trace line each, as the REPL prints them. |
| `results` | Of a completed action: the values its features hold at the end, by name, each formatted as the REPL prints a value. Absent otherwise. |

`tokens`, `activeStates`, `taken`, `queue`, `breakpoints` and `notes` are always
present, empty when there is nothing to report, so a client need not test for
absence.

### `opensysml/debugChanged` (notification, server → client)

Carries a snapshot, sent when a document change moved a session. An edit that
leaves as they were the declarations the run reads — the target's, the
performer's when there is one, and every declaration those name and the named
name in turn: the definitions a machine, its states or its performer specialize
or are typed by, the actions an action invokes, the signal definitions its
triggers accept, the types of the performer's features, the attributes a guard
or an effect names in other packages, and the definition and values a `send`
named — keeps the session running; a redraw of the view gives the nodes new IDs,
so the notification carries the snapshot in the fresh IDs at the new `version`.
An edit that rewrites or removes any of those declarations, that makes the run
read a declaration it did not (a definition declared nearer now shadows the one
a specialization resolved to), that rewrites or removes the declared view — even
one that still draws the target — that makes the view render another kind or
stop drawing the target, or closing the document, ends the session: the snapshot
reports `ended` with the `reason`, and the session's runtime is released. A
library's declarations are not watched, as no edit reaches them. A `send` whose
signal or arguments name a declaration the run had not read checks it the same
way, and answers the `ended` snapshot instead of posting when it was edited
since the session began. A pseudo-view has no declaration to rewrite, so a
session on one ends only for the other reasons. Shutting the server down ends
every session the same way.

The notification is sent after the workspace has taken the change, and is not
debounced: a client keeps the last snapshot it received for each session. Like
`opensysml/renderChanged`, it is best-effort — a client that missed one learns
the current IDs from its next request's answer.

## Trying it by hand

The protocol is JSON-RPC over stdio, so the methods can be driven without an
editor: send `initialize`, then `textDocument/didOpen`, then:

```
→ opensysml/render  { "textDocument": { "uri": "file:///tmp/kit.sysml" }, "view": "#tree" }
← { "view": "", "kind": "tree",
    "stated": "no view declared; rendering /tmp/kit.sysml directly",
    "form": "mermaid",
    "artifact": "%%  — tree rendering (no view declared; …)\nflowchart TD\n  n0[\"Kit::Widget<br>«part def»\"]\n…",
    "nodes": [ … ], "edges": [ … ], "notices": [], "version": 1 }
```

The VS Code extension's diagram panel is the reference client; see
[the editors guide](../guide/08-editors.md#the-diagram-panel).
