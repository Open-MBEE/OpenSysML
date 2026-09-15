# Visual modeling in the VS Code extension

How the extension grows from a text-only language client into a surface a model can
be read as a diagram and, later, authored through. This note is the design for
three tiers, each of which is shippable on its own:

| Tier | What a user gets | Authoring surface |
| --- | --- | --- |
| 1 | A diagram panel that re-renders as the file is typed, with click-to-source | text only |
| 2 | Diagram-side actions that add and change elements | text, written by the tool |
| 3 | A drag-and-drop diagram with persisted layout | diagram, text kept in step |

The tiers are ordered by what they require of the Go side: tier 1 needs a rendering
carried over the wire, tier 2 needs the source-rewriting layer widened, tier 3 needs
layout to be written back into the model.

**Status.** All three tiers are built. The panel (`editors/vscode/src/diagram.ts`,
`src/webview/`), the rendering requests (`internal/lsp/render.go`) and the
authoring request (`internal/lsp/modeledit.go` over `internal/core/edit` and
`model.Workspace.ApplyEdit`) are what [docs/reference/lsp.md](../../reference/lsp.md)
and the extension's README describe. Tier 3 is the SVG canvas in
`src/webview/{layout,canvas}.ts` over the `setLayout`, `setRoute` and `setCanvas`
operations of `internal/core/edit/layout.go`, writing the `DiagramLayout`
annotations of [Diagram layout annotations](../../project/diagram-layout-annotations.md);
its section below is the design as built. Re-parenting is `edit.OpMove`
(`internal/core/edit/move.go`), the `move` operation of `applyModelEdit` and the
node menu's **Move to…**, which offers the drawn declarations whose body admits the
node's kind; the drag that would issue it, and the `CustomTextEditorProvider`
registration, are not built, as the known limitations say. The tier 1 and 2
sections are the design as written before the work, kept for the reasoning behind it.

## What exists today

- **`internal/core/view`** renders a view of the semantic model into a `Rendering`:
  nodes, edges, table rows, and the notices for what the kind could not represent.
  Five kinds are produced — `tree`, `interconnection`, `state`, `action`, `table` —
  read from `semantics.Model.ExposedElements`, the model's connectors, and the
  lowered `ActionGraph`/`StateGraph`, never from source text. `Rendering.Write`
  writes it as `text`, `mermaid` or `markdown`.
- **The frontends that use it** are `sysml <model> -render <view> -render-form
  mermaid` and the REPL's `%view`/`%render`. `Session.viewRenderer`
  (`internal/repl/view.go`) is the pattern: build a resolver and a
  `semantics.Model` over the browse index, hand `view.NewRenderer` a `SourceText`
  so verbatim labels read as written.
- **`internal/lsp`** serves completion, hover, diagnostics, document and workspace
  symbols, semantic tokens, definition, references, rename, formatting and code
  actions over one `model.Workspace`. `Server.applyDidChange` folds each keystroke
  into the workspace and republishes diagnostics, debounced for the other open
  documents.
- **`internal/core/edit`** rewrites the source a model was parsed from without
  disturbing its comments or layout: an `Operation` names an element by FQN, only
  the bytes the parse says carry that element's name or value are replaced, and the
  result is re-parsed and re-analyzed before it is handed back — an edit that would
  make the model unreadable is refused. It has two operations, `OpSetValue` and
  `OpRename`, and one caller, `internal/grpc/edit.go`.
- **`editors/vscode`** contributes the two grammars, the language configuration,
  the `opensysml.*` settings, one `SysML: Restart Language Server` command, and a
  `LanguageClient` over `sysml-lsp` found on the setting, in the workspace's `bin/`,
  or on `PATH`. There is no webview, no diagram and no command that writes a model.

So the renderer is the part that would have been hardest, and it is already
spec-anchored and tested. What is missing is a transport, a panel, and — for
authoring — more operations in `edit`.

## Tier 1 — live diagram panel

### The request

A custom LSP request, because the rendering must come from the same analysis the
diagnostics come from. Re-parsing in the extension would drift, and shelling out to
`sysml -render` per keystroke would pay for the standard library each time.

```
opensysml/render  (request)
  params: { textDocument: { uri }, view?: string, form?: "mermaid" | "text" | "markdown" }
  result: {
    view: string,            // qualified name, as the notation writes it
    kind: string,            // tree | interconnection | state | action | table
    stated: string,          // how the kind was decided, "" for the default
    form: string,            // the form actually written
    artifact: string,        // the Mermaid / text / Markdown document
    nodes: [ { id, kind, name, detail, origin? } ],
    edges: [ { from, to, label, kind } ],
    notices: [ string ],
    version: number          // the document version the rendering was made from
  }

opensysml/views  (request)
  params: { textDocument: { uri } }
  result: {
    views: [ { name, kind, supported, reason? } ],
    pseudoViews: string[]
  }

opensysml/renderChanged  (notification, server → client)
  params: { textDocument: { uri }, version: number }
```

`opensysml/views` is what fills the panel's view picker. It reports an
unsupported kind (`geometry`, `textual`) with the reason rather than omitting
it, and lists the supported `#<kind>` pseudo-view specs so the client does not
duplicate that vocabulary.

`origin` on a node is `{ uri, range }`. `view.Node` carries no source location
today; tier 1 adds one, as a `source.Span` plus the document it belongs to, set
where the node is built from a symbol. It is additive: the text and Mermaid forms
ignore it, so their goldens do not move.

`opensysml/renderChanged` exists so the server, not the client, decides when a
rendering is stale — it is emitted after the same analysis that publishes
diagnostics, on the same debounce as the cross-document sweep
(`crossDocRefreshWindow`, 200ms). The client answers it with a fresh
`opensysml/render`. Push-the-notification/pull-the-artifact keeps a large diagram
off the wire when the panel is hidden.

Capability: the server advertises `experimental: { openSysmlRender: true }` in
`initialize`, and the client only registers the panel when it sees it, so an old
server and a new extension degrade to today's behavior instead of erroring.

### The Go side

- `view.Node` and `view.Edge` grow an origin. `Rendering` grows a `JSON()`-shaped
  companion in `internal/core/view` — a plain data type in `view`, marshaled by the
  LSP layer, so `view` keeps no protocol knowledge.
- `model.Workspace` grows `RenderView(doc, fqn string) (*view.Rendering, *Document, error)`
  — the document returned is the snapshot the rendering was made from, read under
  the same lock, so the LSP layer takes version, FQNs and ranges from one revision —
  and `Views(doc string) []ViewInfo`, built on `newResolver` exactly as
  `Session.viewRenderer` builds its own, with `SourceText` reading the workspace's
  content for the document. This is where the REPL and the LSP converge: the REPL's
  helper stays, but both go through one workspace-level entry point.
- `internal/lsp` gains `render.go` handling the two requests and emitting the
  notification, wired through the same `changeHandler`/`AsyncHandler` chain. A
  request naming no view renders the single view in the document, and reports the
  ambiguity when there are several.

A document with no `view` declaration is the common case for a model being written,
and a panel that can only draw declared views would be empty most of the time. So
`opensysml/render` accepts, in place of a view name, a `#<kind>` or
`#<kind>:<fqn>` pseudo-view listed by `opensysml/views`: the server renders the
element as if a view exposing it had been declared, through the same renderer,
and reports `stated: "no view declared; rendering <element> directly"`.
Nothing synthetic is added to the model or the index — the exposed set is
passed to the renderer directly.

### The extension side

- `SysML: Open Diagram` opens a `WebviewPanel` beside the editor, one per document,
  retained across tab switches with `retainContextWhenHidden` off and state restored
  through `setState`/`getState`. The command is bound to <kbd>Alt</kbd>+<kbd>D</kbd> and
  <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> (the PlantUML and Markdown-preview
  conventions) with `when` clauses that hold only for a model editor or the panel
  itself, and sits in the editor title bar and the editor and Explorer context menus.
  It resolves its document from the menu's resource, the focused panel (returning
  to the source), the active editor, or the one model editor in view — in that order
  — and is registered whether or not the server draws, so a key or menu always
  answers, with a diagram or with the reason there is none.
- The webview bundles Mermaid locally (no CDN, and a `Content-Security-Policy` with
  a nonce and no `connect-src`), renders the artifact, and re-renders on the
  extension's `postMessage`.
- Clicking a node posts its id back; the extension maps id → origin and calls
  `vscode.window.showTextDocument` with that range selected. The reverse — cursor in
  the editor highlights the node whose origin contains it — comes from
  `onDidChangeTextEditorSelection`.
- A rendering that fails (a parse error mid-keystroke) leaves the last good diagram
  on screen, dimmed, with the error in the panel's status line. Blanking the panel
  on every incomplete keystroke makes it unusable while typing.
- Notices are shown as a collapsible list under the diagram, not dropped, matching
  what the text form does.

### Test contract

- `internal/core/view`: origins are covered by the existing render tests, extended
  to assert that each node's origin spans the declaration it was built from, and
  that the text/Mermaid goldens are unchanged.
- `internal/lsp/render_test.go`: request/response over the in-process server for
  each kind, for a pseudo-view, for a document with no views, for an unsupported
  kind (asserting the reason), and for a stale-version request. Plus a
  didChange → `renderChanged` ordering test.
- `editors/vscode`: `npm run typecheck` and a GUI pass per
  `.agents/skills/testing-vscode-extension/SKILL.md` — open a model, open the
  panel, type, watch it redraw, click a node and land on the declaration.

### What tier 1 is not

It is a viewer. The model is authored in text; the diagram never writes.

## Tier 2 — authoring through diagram actions

The diagram gains a palette and a context menu whose actions are *text edits*: the
`.sysml` file stays the single source of truth, and the diagram is always a
rendering of what the file now says. This is the tier that makes "create models
visually" true without a graphical editor's bookkeeping.

### Widening `internal/core/edit`

As built, the operations below carry a few more fields than sketched here
(`OpAddMember` also takes a multiplicity, a value and specializations;
`OpAddConnection` a type), and `applyModelEdit`'s refusal names a stable
`failure` beside the diagnostic. Three operations are added, in the package's existing style — name the target the
way symbols name it, splice bytes the parse located, re-analyze before returning:

- `OpAddMember{Owner, Kind, Name, Type}` inserts a member into an owner's body:
  `part engine : Engine;` into `part def Vehicle { … }`. The insertion point is the
  end of the owner's body span, indented to the body's own level, and an owner
  declared without a body gets one. The notation is emitted by a small writer in
  `edit`. The whole document is deliberately not passed through
  `internal/core/format`: source-preserving edits keep every byte outside edited
  spans identical, so the writer detects indentation only for its insertion.
- `OpAddConnection{Owner, Kind, From, To, Name}` inserts a `connect a to b;`,
  `flow`, `interface`, `succession` or `transition` into the owner's body, with the
  endpoints written as the names that resolve from that scope.
- `OpDelete{Target}` removes a declaration and the trivia that belongs to it: its
  leading comment block and its own line, never a neighbor's.

Each refuses rather than writes when re-analysis reports an error the original did
not have — an added member whose type does not resolve, a connection whose endpoint
is out of scope, a delete that orphans a reference. The refusal names the
diagnostic, and the panel shows it. `OpSetValue` and `OpRename` already exist and
are reused as-is for editing a value or a name from the diagram.

Deletion is where a "refuse on new errors" rule is least obviously right: deleting a
part that something connects to *should* be reported. The operation therefore
carries `Cascade bool`; without it the delete is refused and names the referents,
with it the referring declarations are deleted in the same operation, and the panel
asks before setting it, listing the referents by file. A reference from another
document of the workspace is followed: a rename respells it there, a cascade delete
removes the declaration making it, recursively, and the `WorkspaceEdit` carries one
`TextDocumentEdit` per document rewritten. `edit.Model.Other` hands the operation the
other documents it may rewrite; a reference from a document it may not — a bundled
library file — is still refused as `referenced-elsewhere`, since the edit cannot
follow it. The documents are snapshotted, rewritten and re-analyzed together under
one workspace lock, so a rewrite that leaves any of them with a new error refuses
the whole operation and nothing is written.

### From the diagram to the file

```
opensysml/applyModelEdit  (request)
  params: { textDocument: { uri }, version: number, operations: [ Operation ] }
  result: { edit: WorkspaceEdit } | { refused: [ { operation: number, diagnostic } ] }
```

The server translates the operations into `edit` operations, runs them against the
workspace's current content, and returns the byte diff as a `WorkspaceEdit` — it
does not write the file. The client applies it with
`vscode.workspace.applyEdit`, which puts the change in VS Code's undo stack, so a
diagram action is undone with `ctrl+z` like anything typed — across every document
it touched, since it is one edit. The request names the
`version` of the rendering the action was taken on, not the buffer's: its targets
are names the user saw there, and a later version may spell the same names for
other declarations. A `version` that no longer matches is rejected, and the panel
redraws and asks the user to repeat the action on what is now shown. Each other
document's `TextDocumentEdit` carries the version the server computed it against; the
language client library applies edits without checking that, so the panel does. A
document the edit names that no buffer holds is opened first and the edit asked for
again, so every document it lands on is a versioned buffer; the versions are compared
and `applyEdit` called in one turn, and VS Code pins each document to the version it
holds at that call, so an edit naming a document that moved on meanwhile is not
applied at all.

The diagram never mutates itself. It applies the edit, the edit re-triggers
analysis, analysis emits `renderChanged`, and the panel redraws from the model. One
direction of truth, so a diagram that disagrees with the file is not
representable.

### The palette

Which actions are offered is decided by the rendering's kind, not hardcoded:
`interconnection` offers parts, ports and connections; `state` offers states,
transitions, entry/exit; `action` offers action nodes, successions, forks and
joins; `tree` offers a member of any definition kind. The names come from the same
tables `view` already keys its kinds by, so a kind added later does not need the
palette rewritten.

### Test contract

- `internal/core/edit`: per operation, a golden pair (source in, source out) proving
  comments, blank lines and indentation survive; a refusal test per new-error class;
  a cascade-delete test; an idempotence test through `format`.
- `internal/lsp`: `applyModelEdit` returning a `WorkspaceEdit` whose application
  reproduces the golden output, a stale-version rejection, and a refusal shape.
- GUI: add a part and a connection from the palette, check the file, `ctrl+z`, check
  the file again.

## Tier 3 — a drag-and-drop graphical editor

Tiers 1 and 2 avoid the two problems a real graphical editor has: layout, and
edits whose intent is not a text operation. Tier 3 takes them on.

### Layout is model data

A node's position is in the model: the bundled `DiagramLayout` library declares
`Layout`, `Route` and `Canvas` metadata, and a view places an element by stating
`metadata Layout about engine { x = 120; y = 80; }` in its body, or an element
places itself in every view that says nothing by carrying the annotation inline.
That is the design of
[Diagram layout annotations](../../project/diagram-layout-annotations.md), and it
replaced an earlier plan for a `<model>.sysml.layout.json` sidecar: the sidecar
kept pixels out of a conformant document, but at the price of a second file that
drifts from the model, that a rename orphans, and that no other tool reads. The
annotations are ordinary metadata, so they travel with the model, rename with the
element, and version with it.

The consequences for the editor:

- Positions are keyed by declaration, so the canvas writes to a node's `fqn` and an
  edge's `fqn`, never to a rendering's node id, which is assigned per render.
- An element the model does not place is laid out by the client and its position
  written on first drag, so a model with no annotation draws and its bytes are
  untouched until a user drags something.
- The unit is the canvas's pixel, y downward, as `opensysml/render` reports it, so
  a position written by one client draws the same in another.

### The write-back

`internal/core/edit` gained `SetLayout`, `SetRoute` and `SetCanvas`
(`layout.go`), each a source-preserving splice: an annotation already there has
its values rewritten in place, one added goes where the writer puts it — the view's
body for a view-local one, the element's own body for an inline one, opening a
bodyless declaration as `AddMember` does — and a cleared one is removed with its
line and, when it was the whole body, with the body. The operations refuse a target
outside the document (`unknown-target`), a view that is none (`not-a-view`), a
view-local placement of an element the view does not expose (`not-exposed`), an
element no rendering draws as the node or edge the annotation positions
(`not-drawn`), and a clearing with nothing to clear (`not-annotated`).
`opensysml/applyModelEdit` exposes them as `setLayout`, `setRoute` and
`setCanvas`; the three rewrite the requesting document alone, so their
`WorkspaceEdit` carries one `TextDocumentEdit`.

### Direct manipulation

Dragging a node writes its `Layout`; dragging an edge's handle writes its `Route`.
Every action that changes the model otherwise goes through tier 2's operations —
the palette is `AddMember`, the node menu's connections `AddConnection`, its
`Delete` and `Rename` the same. So tier 3 adds gestures whose outcome is an
annotation, over tier 2's vocabulary for everything else.

A gesture is one edit. The canvas previews a drag by redrawing itself over the
last rendering with the moved geometry laid over it, and posts one `place` message
when the pointer is released; the extension turns it into one `applyModelEdit`
request on the version the rendering was drawn from, so one drag is one undo step
and a stale rendering is redrawn rather than written to. Moving a node carries the
descendants the model places, and the routes between two nodes of the moved
subtree, in the same request; unplaced descendants follow their owner on their own.

Re-parenting is a move — delete from one body and add to another as one
operation, `edit.OpMove{Target, NewOwner}`, so the two halves cannot come apart —
issued from the node menu's **Move to…**; dragging a part into a different
definition does not issue it.

### Rendering surface

Mermaid is a fine read-only renderer and a poor editing surface — it lays out the
graph itself and exposes no handles. The panel draws its own SVG
(`src/webview/canvas.ts`) from a layout computed in the webview
(`src/webview/layout.ts`): a node the model places goes exactly there, at the size
it states; every other node takes a slot in a near-square grid under its owner, in
rendering order, so the layout is a pure function of the rendering. A sequence is
lifelines in a row with its messages down them. Mermaid remains the server's
machine form — the `artifact` of a rendering, the REPL's and the document
pipeline's diagram output — and nothing there changed. The panel is a
`WebviewPanel` beside the text editor, not a `CustomTextEditorProvider`: a model
is edited as text with the diagram in step, and the editor's dirty state, undo and
save are the text document's.

### Test contract

- `edit`: goldens for a new annotation in a view body and inline, an update in
  place, a clearing that removes the body it filled, a route and a canvas, and each
  typed refusal; the unannotated case stays byte-identical
  (`internal/core/edit/layout_test.go`).
- LSP: a render → `setLayout` → apply → re-render round trip that sees the new
  `x` and `y`, one versioned `TextDocumentEdit` on the document, and a refusal shape
  (`internal/lsp/modeledit_test.go`).
- Webview: the automatic layout pinned for a fixture, the model's geometry kept
  exactly, one placement per gesture, and the SVG's node groups, handles and
  arrowheads (`src/webview/layout.test.ts`, `canvas.test.ts`).
- GUI: drag a node, check the file gained the annotation, <kbd>Ctrl</kbd>+<kbd>Z</kbd>,
  check it is gone, redraw and check the position held.

## Known limitations, stated rather than hidden

- The `geometry` view kind is not rendered by `internal/core/view` and no tier here
  adds it; the panel reports it as unsupported.
- Multi-document models render per document. A view exposing elements from another
  open file draws them, and dragging one writes into the view's body in the
  panel's document; a document drawn directly places only what it declares, and
  writing an annotation into another document is out of scope. Rename and delete
  follow references into the workspace's other documents; a reference from a
  bundled library file, or from a document the index holds without the workspace
  holding its source, still refuses the edit.
- Only the requesting document's version travels in the request, so only it can be
  answered `stale` by the server; another document that changed between the server
  computing the edit and the client applying it is caught by the client comparing
  the versions the edit carries, not by a server refusal.
- The layout annotations are this project's library. SysML v2 §10.2 leaves how a
  view is drawn to the tool, so nothing here claims to be a normative diagram
  interchange, and no attempt is made to read or write another tool's layout.
- Re-parenting by drag is not built; a part is moved from the node menu's
  **Move to…** or by editing the text.
- The palette writes the notation OpenSysML's writer emits, which is
  spec-conformant but not necessarily byte-identical to what a user would have
  typed. `format` makes it consistent with the file; it does not make it a
  particular author's style.
