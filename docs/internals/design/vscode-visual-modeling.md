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
`src/webview/`), the rendering requests (`internal/frontend/lsp/render.go`) and the
authoring request (`internal/frontend/lsp/modeledit.go` over `internal/check/edit` and
`model.Workspace.ApplyEdit`) are what [docs/reference/lsp.md](../../reference/lsp.md)
and the extension's README describe. Tier 3 is the SVG canvas in
`src/webview/{layout,canvas}.ts` over the `setLayout`, `setRoute` and `setCanvas`
operations of `internal/check/edit/layout.go`, writing the `DiagramLayout`
annotations of [Diagram layout annotations](../../project/diagram-layout-annotations.md);
its section below is the design as built. Re-parenting is `edit.OpMove`
(`internal/check/edit/move.go`), the `move` operation of `applyModelEdit` and the
node menu's **Move to…**, which offers the drawn declarations whose body admits the
node's kind, and the <kbd>Shift</kbd>-drop of a node on another, which issues it for
the node under the pointer; the `CustomTextEditorProvider` registration is not
built, as the known limitations say. The tier 1 and 2
sections are the design as written before the work, kept for the reasoning behind it.

## What exists today

- **`internal/ir/view`** renders a view of the semantic model into a `Rendering`:
  nodes, edges, table rows, and the notices for what the kind could not represent.
  Five kinds are produced — `tree`, `interconnection`, `state`, `action`, `table` —
  read from `semantics.Model.ExposedElements`, the model's connectors, and the
  lowered `ActionGraph`/`StateGraph`, never from source text. `Rendering.Write`
  writes it as `text`, `mermaid` or `markdown`.
- **The frontends that use it** are `sysml <model> -render <view> -render-form
  mermaid` and the REPL's `%view`/`%render`. `Session.viewRenderer`
  (`internal/frontend/repl/view.go`) is the pattern: build a resolver and a
  `semantics.Model` over the browse index, hand `view.NewRenderer` a `SourceText`
  so verbatim labels read as written.
- **`internal/frontend/lsp`** serves completion, hover, diagnostics, document and workspace
  symbols, semantic tokens, definition, references, rename, formatting and code
  actions over one `model.Workspace`. `Server.applyDidChange` folds each keystroke
  into the workspace and republishes diagnostics, debounced for the other open
  documents.
- **`internal/check/edit`** rewrites the source a model was parsed from without
  disturbing its comments or layout: an `Operation` names an element by FQN, only
  the bytes the parse says carry that element's name or value are replaced, and the
  result is re-parsed and re-analyzed before it is handed back — an edit that would
  make the model unreadable is refused. It has two operations, `OpSetValue` and
  `OpRename`, and one caller, `internal/frontend/grpc/edit.go`.
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
The cross-document contract is advertised by both sides as
`openSysmlCrossDocumentLayout`: the server names another document's nodes only
to a client that pins them by `declaredIn` and `digest`, and the client reads a
server that never sent `declaredHere` as declaring every node it named, so an
older extension drags no unpinned name and an older server loses no menu.

### The Go side

- `view.Node` and `view.Edge` grow an origin. `Rendering` grows a `JSON()`-shaped
  companion in `internal/ir/view` — a plain data type in `view`, marshaled by the
  LSP layer, so `view` keeps no protocol knowledge.
- `model.Workspace` grows `RenderView(doc, fqn string) (*view.Rendering, *Document, error)`
  — the document returned is the snapshot the rendering was made from, read under
  the same lock, so the LSP layer takes version, FQNs and ranges from one revision —
  and `Views(doc string) []ViewInfo`, built on `newResolver` exactly as
  `Session.viewRenderer` builds its own, with `SourceText` reading the workspace's
  content for the document. This is where the REPL and the LSP converge: the REPL's
  helper stays, but both go through one workspace-level entry point.
- `internal/frontend/lsp` gains `render.go` handling the two requests and emitting the
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

- `SysML: Open Diagram` opens a `WebviewPanel` beside the editor, one per document
  and view, retained across tab switches with `retainContextWhenHidden` off and
  state (`{uri, view}`) restored through `setState`/`getState`, so a reload brings
  every panel back on its view. The command is bound to <kbd>Alt</kbd>+<kbd>D</kbd> and
  <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> (the PlantUML and Markdown-preview
  conventions) with `when` clauses that hold only for a model editor or the panel
  itself, and sits in the editor title bar and the editor and Explorer context menus.
  It resolves its document from the menu's resource, the focused panel (returning
  to the source), the active editor, or the one model editor in view — in that order
  — and is registered whether or not the server draws, so a key or menu always
  answers, with a diagram or with the reason there is none.
- Which view a panel opens on is decided client-side, in `views.ts`, as pure
  functions over the `opensysml/views` listing (`chooseView`): the view the
  document implies (its sole drawable view, `#tree` when it declares none); else
  the drawable view whose declaration `range` holds the editor's cursor; else the
  view last chosen for that document, kept in `workspaceState` under
  `opensysml.diagram.chosenViews` keyed by document URI and forgotten when the
  document no longer declares it; else a quick pick of the drawable views (label
  the name, detail the kind), then **All views**, then the pseudo-views. A view
  the server cannot draw is left out of the quick pick — its items cannot be
  disabled — and the panel's own picker lists it disabled with the reason.
  Cancelling opens nothing. The server never receives an
  empty `view` for a multi-view document: the client always names one. Servers
  whose listing carries no `range` skip the cursor step.
- `DiagramPanels` keys panels by `panelKey(uri, view)`; `renderChanged` and the
  cursor highlight go to every panel of the document, and panels are titled
  `Diagram: <file> — <view>` while the document has more than one. Opening the
  view a panel already draws reveals it; a different view opens another panel
  beside the source. The in-panel picker retargets its panel to the chosen view,
  and re-keys it — unless another panel already draws that view, which is
  revealed instead, so a document never has two panels of one view. Export uses
  the document's one panel's view, and asks when there are none or several.
- The panel is open by default. A model file shown in an editor gets its diagram
  without being asked — on activation, on every change of active editor, and, for
  a file made active while the server was still starting, when the client attaches.
  The decision is one pure function (`src/autoopen.ts`, `shouldAutoOpen`) over the
  `opensysml.diagram.autoOpen` setting, whether a drawing server is attached,
  whether the document already has a panel of any view (a panel restored by the
  serializer counts, so a reload does not double-open), whether the user dismissed it, and
  what the editor is: only a `file:` document in a model language, sitting in an
  editor group, whose active tab is a plain text tab. That rules out untitled
  buffers, `git:` revisions, diff tabs and the peek editor of a hover. An
  automatic open is silent — no "server not running" or "too old" warning, those
  belong to the explicit command — and never takes focus; when a document declares
  several views it runs the same choice as the command up to the quick pick — the
  implied view, the one under the cursor, the remembered one — and opens nothing
  rather than ask. The eligibility is checked again once the listing returns, so a
  panel opened or an editor switched meanwhile is not doubled; a listing that
  fails opens nothing (the command falls back to the server's own choice), and
  ranges listed before an edit are not matched against the cursor after it.
- Placement: one panel per document and view, all in one editor group. The first
  diagram opens `Beside` its source; every later one, automatic or explicit, opens
  in the group an existing diagram already occupies (`diagramColumn`), so switching
  between model files or views adds a tab to the diagram column rather than a
  column to the layout. One panel per view (not one reused panel) keeps each
  panel's view choice and its serialized state, and lets two diagrams be compared
  by dragging one out; one group keeps the layout calm. An explicit Open Diagram
  on a view that already has a panel reveals it in that group.
- Dismissal is per document, whatever the number of its panels: closing the
  document's last panel records its document URI under
  `workspaceState` (`opensysml.diagram.dismissed`), and the document is not
  auto-opened again — across editor switches and across window reloads — until an
  explicit Open Diagram, which clears the entry and opens the panel. Only the user's
  close counts: `DiagramPanel` tracks a `Lifecycle` that the extension marks before
  it disposes a panel itself (a panel replaced by `adopt`, or every panel on
  deactivation), so the `onDidDispose` that follows is not a dismissal; a server
  restart disposes no panel at all. Closing the source editor leaves the panel
  open, as it does today — a diagram is a document of its own, with a way back to
  the source. A rename (`workspace.onDidRenameFiles`) carries a dismissal to the
  new URI and moves an open panel with it — the panel is recreated under the new
  URI in the same group with the same view, each of the document's panels in
  turn, an extension-caused replacement, so it is not a dismissal; a delete clears the dismissal, so a file recreated under the
  same name starts fresh. VS Code reports a folder rename or delete as the folder
  alone, so both apply to every document below it (`renamedUri`). `Dismissals`
  keeps the list in memory and writes it to `workspaceState` in order, so the
  un-awaited mutations of a multi-file rename or delete cannot overwrite one
  another, and a failed write does not hold up the next. The view chosen for a
  document (`ChosenViews`, `opensysml.diagram.chosenViews`) follows the same
  rename and delete, with the same ordered writes.
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

- `internal/ir/view`: origins are covered by the existing render tests, extended
  to assert that each node's origin spans the declaration it was built from, and
  that the text/Mermaid goldens are unchanged.
- `internal/frontend/lsp/render_test.go`: request/response over the in-process server for
  each kind, for a pseudo-view, for a document with no views, for an unsupported
  kind (asserting the reason), and for a stale-version request. Plus a
  didChange → `renderChanged` ordering test.
- `editors/vscode/src/views.test.ts`: the view choice (cursor in a declaration,
  the remembered view and its staleness, the fallbacks, "All views" expansion),
  the panel keying as pure functions, and the chosen-view store's
  remember/clear/rename semantics.
- `editors/vscode/src/autoopen.test.ts`: `shouldAutoOpen` over every input, the
  dismissal store's record/clear/rename semantics, and the `Lifecycle` distinction
  between a disposal the extension asked for and a tab the user closed;
  `manifest.test.ts` pins the setting and its default.
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

### Widening `internal/check/edit`

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
  `internal/syntax/format`: source-preserving edits keep every byte outside edited
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
redraws and asks the user to repeat the action on what is now shown. The version
is the server's guard; the panel has one of its own, since node ids are local to a
drawing and a drawing can be replaced without the document changing — another
view is picked, or a document it imports is edited. The panel numbers every
drawing it posts to the webview, every message the webview sends back that names
a node — an action, a placement, a click that reveals a declaration — carries the
number of the drawing its ids came from, and one whose number is not the current
drawing's is refused before its ids are resolved, with the same message. An action
that prompts — a rename's input box, a move's destination pick — is checked again
once the prompt closes, since the drawing can be replaced while it is open.
The webview takes a drawing's number only once it has drawn it: a drawing that
fails to draw leaves the last one up, dimmed, and the last one's number with it, so
an action taken on what is still shown is refused rather than resolved against the
rendering the panel holds. A restored panel draws its saved rendering as drawing
zero until the server draws again, and zero is never current, so an action taken on
it is refused the same way.
Each other document's `TextDocumentEdit` carries the version the server computed it
against; the language client library applies edits without checking that, so the
panel does. A document the edit names that no buffer holds is opened first and the
edit asked for again, so every document it lands on is a versioned buffer; the
versions are compared and `applyEdit` called in one turn, and VS Code pins each
document to the version it holds at that call, so an edit naming a document that
moved on meanwhile is not applied at all.

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

- `internal/check/edit`: per operation, a golden pair (source in, source out) proving
  comments, blank lines and indentation survive; a refusal test per new-error class;
  a cascade-delete test; an idempotence test through `format`.
- `internal/frontend/lsp`: `applyModelEdit` returning a `WorkspaceEdit` whose application
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

`internal/check/edit` gained `SetLayout`, `SetRoute` and `SetCanvas`
(`layout.go`), each a source-preserving splice: an annotation already there has
its values rewritten in place, one added goes where the writer puts it — the view's
body for a view-local one, the element's own body for an inline one, opening a
bodyless declaration as `AddMember` does — and a cleared one is removed with its
line and, when it was the whole body, with the body. The document written is the
one declaring what holds the annotation, wherever the request came from: the view's
document for a view-local `Layout` or `Route` and for a `Canvas`, the element's own
document for an inline one. A target is resolved through the workspace index, in
the document `declaredIn` names — the rendering's origin, whose text the `digest`
it reported fingerprints — at the text it was rendered from: a declaration range is
read there, and a qualified name must be declared there, so a document since
changed is answered stale rather than read where the range now falls or the name
now reaches, and a namesake another document declares is refused rather than
placed. The rendering itself is converted from one read of the workspace — every
origin, range and digest of every document it draws from, taken under the one lock
— so that what a client hands back names the text the rendering was made from. The
edit is then pinned to the very snapshot the target was read in: the workspace
refuses it, under the one lock, should it have replaced that document in between.
The splices go through the same
per-document routing, atomic validation and `Result.Others` that rename and
delete use, so one request may write several documents and refuses as a whole
when the result is invalid in any of them. The operations refuse a target no
document of the workspace declares (`unknown-target`), a view that is none
(`not-a-view`), a view-local placement of an element the view does not expose
(`not-exposed`), an element no rendering draws as the node or edge the annotation
positions (`not-drawn`), a clearing with nothing to clear (`not-annotated`), and a
destination it may not rewrite — a bundled library file, or a document the index
holds without the workspace holding its source — naming the file
(`referenced-elsewhere`). `opensysml/applyModelEdit` exposes them as `setLayout`,
`setRoute` and `setCanvas`, answering one versioned `TextDocumentEdit` per
document changed, as it does for rename and delete; the requesting document is
among them only when it changed.

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
issued from the node menu's **Move to…** and by dropping a node on another with
<kbd>Shift</kbd> held.

The modifier is what tells the two drags apart. A plain drag is a layout drag
whatever it is released over: a node is routinely dragged across its neighbours'
boxes, and inside its owner's, on the way to a position, and a canvas whose nodes
are placed close together would offer a re-parent on most releases if the pointer's
position alone decided. So a release never moves a declaration unless
<kbd>Shift</kbd> is down at that moment, and a plain drag posts the same `place`
message it always did. The gesture is told on the status line as soon as a node
that some drawn node admits is picked up ("Hold Shift and release over a node to
move … into it"), so it is found without reading the manual, and confirmed while
it is held. While <kbd>Shift</kbd> is down the canvas is not laid out again around
the node's new place — an owner's box growing out to keep the node, and its
neighbours shuffling aside, would carry the target away from under the pointer —
but stays as the model laid it out, with the dragged subtree floating over it
(`liftNode` in `src/webview/canvas.ts`). The edges at the subtree float with it: one
between two of its nodes moves whole, waypoints and label included, as the `setRoute`
a release writes will move it; one crossing the subtree's border keeps its waypoints,
which stay the model's, and is re-anchored on its lifted end (`liftedEdges` in
`src/webview/layout.ts`). The node under the pointer — the innermost,
latest-drawn box of that layout holding the point, with the dragged subtree passed
over (`nodeUnder` in `src/webview/layout.ts`) — is judged by the same `moveDestinations` filter the
**Move to…** menu is built from (`src/edits.ts`: the body admits the node's
`notation`, the target is neither the node, nor its current owner, nor anything
inside it), and outlined when it admits the dragged node. A node that does not
admit it is not outlined; the cursor turns to *not-allowed* and the status line
says why ("already declared in …", "declared inside …", "a subject cannot be
declared in …"). Releasing there cancels the drag rather than falling back to a
layout drag: the node goes back where it was and the reason stays on the status
line, because a release the user meant as a move should not quietly write a
position instead. Releasing with <kbd>Shift</kbd> over empty canvas is a plain drag.

A drop is one `reparent` message and one `applyModelEdit` request, pinned to the
version the canvas was drawn from like every other gesture: the `setLayout` (and
`setRoute`) operations the same drag would have written, followed by the `move`
(`reparentOperations` in `src/edits.ts`). The layout is written first, on the
names the rendering draws, and the move respells the annotation's `about` name
along with every other reference to the moved declaration, so the node redraws
under its new owner at the place it was released — the owner's box grows around
it if need be — and one <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes both. The extension
refuses the drop itself when the rendering it holds no longer offers the target,
and every refusal the server answers — a name clash in the destination, a cycle, a
declaration another file refers to — is posted back to the panel as a `revert`:
the canvas redraws the model's layout, the status line carries the server's
message, and no source changed. A drop begun on a drawing the panel has since
replaced is refused by the drawing's number, as for any edit, so its ids are never
resolved against nodes they did not name. When the edit applies, nothing of the
gesture survives: the document-change path re-renders and the node is wherever the
model now declares it.

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

The canvas has two looks, chosen by `opensysml.diagram.style` and the panel's
**Style** list (`src/style.ts`): `theme`, which takes its colours from the VS Code
theme, and the pilot visualizer's Standard B&W that the DOT and PlantUML forms
follow (`docs/project/view-rendering-forms.md#style`), as CSS on the `pilot` class
— white canvas, black text, 0.5 px `#181818` borders, square definitions and
rounded usages by a class the node's kind gives its box, heavier packages, dashed
regions, bold names over an italic keyword, 3 px arrowless connections, dashed
flows, filled pseudo-states. A palette is that look plus the `fill` and `border`
the server puts on each node when the render request names one; the canvas sets
them as custom properties on the node's shape and computes no colour itself, so
the panel, DOT and PlantUML of one view agree hex for hex and the contrast rule
lives in one place. The server advertises `openSysmlRenderPalette`; without it the
panel asks for no palette, draws `pilot`, and says why under the diagram.

### Test contract

- `edit`: goldens for a new annotation in a view body and inline, an update in
  place, a clearing that removes the body it filled, a route and a canvas, and each
  typed refusal; the unannotated case stays byte-identical; a view in another
  document, an element in another document placed inline, a `Canvas` on a view
  elsewhere, a clearing elsewhere, several documents in one request, the refusals
  for an unheld and a library document, and the atomic refusal when the second
  document's result is invalid (`internal/check/edit/layout_test.go`).
- LSP: a render → `setLayout` → apply → re-render round trip that sees the new
  `x` and `y`, one versioned `TextDocumentEdit` on the document, and a refusal shape
  (`internal/frontend/lsp/modeledit_test.go`); a view drawing another document's parts, whose
  drag writes the view's document alone, a route by declaration range that writes the
  other document at the version the server holds, a direct rendering that writes the
  other document and not its own, a disk-only document written at no version, the
  library refusal, and the atomic refusal when the other document would become
  invalid (`internal/frontend/lsp/modeledit_cross_document_test.go`).
- Extension: a node another document declares is placed by its qualified name, a
  declaration range travels with the document it is one of, and the edit is applied
  only while every document it names is open at the version it carries
  (`src/edits.test.ts`).
- Webview: the automatic layout pinned for a fixture, the model's geometry kept
  exactly, one placement per gesture, and the SVG's node groups, handles and
  arrowheads (`src/webview/layout.test.ts`, `canvas.test.ts`); the node under a
  point, innermost and latest-drawn, with the dragged subtree and hidden nodes
  passed over (`layout.test.ts`); a drop admitted for exactly the targets
  **Move to…** lists, each refusal's reason, and the hint a pick-up shows
  (`drop.test.ts`).
- Extension: the drop's batch — placements first, then the move, nothing when the
  target is not offered or a placement is undeclared — the request it becomes,
  pinned to the rendering's version, and the refusal of a drop from a replaced
  drawing whose ids now name other declarations (`src/edits.test.ts`).
- GUI: drag a node, check the file gained the annotation, <kbd>Ctrl</kbd>+<kbd>Z</kbd>,
  check it is gone, redraw and check the position held. <kbd>Shift</kbd>-drop a part
  on another definition, check the declaration moved in the file and the diagram
  redrew it under the new owner, <kbd>Ctrl</kbd>+<kbd>Z</kbd>, check both are back;
  <kbd>Shift</kbd>-drop on a node that does not admit it and check the file is
  untouched.

## Known limitations, stated rather than hidden

- The `geometry` view kind is not rendered by `internal/ir/view` and no tier here
  adds it; the panel reports it as unsupported.
- Multi-document models render per document. A view exposing elements from another
  file draws them and places them in its own body; a document drawn directly places
  what it draws from another file inline, in that file; a view declared in another
  file is placed in that file. Rename, delete and the layout operations follow
  their targets into the workspace's other documents; a reference from, or an
  annotation into, a bundled library file or a document the index holds without
  the workspace holding its source still refuses the edit, naming the file.
- A node drawn from a bundled library, or from no workspace document, carries no
  `fqn` and no `declaration`, so the panel does not offer to drag it. A node another
  workspace document declares carries its `fqn` but not `declaredHere`, so the panel
  drags it and offers it nothing else: a rename, delete, move or member added is
  written by the document declaring the node, from a panel of that document. Both
  hold only between a client and a server advertising `openSysmlCrossDocumentLayout`;
  across a version gap each side falls back to naming, or reading, the requested
  document's declarations alone.
- Only the requesting document's version travels in the request, so only it can be
  answered `stale` by the server; another document that changed between the server
  computing the edit and the client applying it is caught by the client comparing
  the versions the edit carries, not by a server refusal.
- The layout annotations are this project's library. SysML v2 §10.2 leaves how a
  view is drawn to the tool, so nothing here claims to be a normative diagram
  interchange, and no attempt is made to read or write another tool's layout.
- A drop re-parents within the requesting document only, as **Move to…** does: a
  node drawn from another file is neither dragged nor a drop target, and a
  declaration another file refers to is refused by the server.
- The palette writes the notation OpenSysML's writer emits, which is
  spec-conformant but not necessarily byte-identical to what a user would have
  typed. `format` makes it consistent with the file; it does not make it a
  particular author's style.
