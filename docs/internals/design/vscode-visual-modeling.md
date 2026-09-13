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
a place to keep layout that is not the model.

**Status.** Tiers 1 and 2 are built: the panel (`editors/vscode/src/diagram.ts`,
`src/webview/`), the rendering requests (`internal/lsp/render.go`) and the
authoring request (`internal/lsp/modeledit.go` over `internal/core/edit` and
`model.Workspace.ApplyEdit`) are what [docs/reference/lsp.md](../../reference/lsp.md)
and the extension's README describe. Of tier 3, the move re-parenting needs is
built: `edit.OpMove` (`internal/core/edit/move.go`), the `move` operation of
`applyModelEdit`, the service's `MoveEdit`, and the node menu's **Move to…**, which
offers the drawn declarations whose body admits the node's kind; the drag that
will issue it, and the layout sidecar, are not. The `DiagramLayout` annotations
that have since landed give tier 3 a place in the model for layout, which changes
its "layout is not model data" premise below. The rest of this note is the design
as written before the work, kept for the reasoning behind it.

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
  through `setState`/`getState`.
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
language client library applies edits without checking that, so the panel does, and
an edit naming a document that moved on meanwhile is not applied at all.

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

### Layout is not model data

A node's position is not in the model and must never be written to the `.sysml`
file — that would put tool state into a spec-conformant document and make two
authors' files differ by nothing but pixels. Layout lives beside the model, in
`<model>.sysml.layout.json`, an explicitly tool-defined sidecar:

```
{ "schema": 1, "view": "Views::vehicleView", "kind": "interconnection",
  "nodes": { "Vehicle::engine": { "x": 120, "y": 40, "w": 160, "h": 80 } },
  "edges": { "Vehicle::c1": { "waypoints": [ [200,80], [260,120] ] } } }
```

Keys are FQNs, not the rendering's node ids: an id is assigned per render
(`nodeIDs.take`) and is not stable across edits, while an FQN is as stable as the
model. An element with no entry is auto-placed by the layout engine and its
position written back on first drag; an entry whose element is gone is dropped on
save. The sidecar is optional, and committing it is the user's choice — a model
opened without one still draws, just auto-laid-out.

### Direct manipulation

Dragging a node changes layout only, so it touches the sidecar and not the model.
Every action that changes the model goes through tier 2's operations — drawing an
edge between two nodes is `OpAddConnection`, dropping a palette item is
`OpAddMember`, deleting a node is `OpDelete`, renaming in place is `OpRename`. So
tier 3 adds no new way to write a model; it adds gestures over tier 2's vocabulary.

Two things genuinely new:

- **Re-parenting** (dragging a part into a different definition) is a move: delete
  from one body and add to another, as one operation so a failure leaves neither
  half applied. `edit` gains `OpMove{Target, NewOwner}` for it, rather than the
  client issuing two operations it cannot make atomic.
- **Batching.** A drag that creates several elements at once must be one
  `WorkspaceEdit` so it is one undo step. `applyModelEdit` already takes a list;
  tier 3 requires that the list is applied all-or-nothing, which is how `edit`
  composes operations already.

### Rendering surface

Mermaid is a fine read-only renderer and a poor editing surface — it lays out the
graph itself and exposes no handles. Tier 3 replaces the webview's renderer with an
SVG canvas driven by the rendering's nodes and edges plus the sidecar's geometry,
keeping Mermaid as the export path (`SysML: Export Diagram`) so the docs pipeline
and tier 1's panel keep working. The panel is registered as a
`CustomTextEditorProvider` for `.sysml`, so a user can open a model *as* a diagram
and VS Code handles dirty state, undo and save.

### Test contract

- Sidecar: schema round-trip, unknown-element pruning, missing-file defaulting,
  and a golden auto-layout for a fixture model.
- `edit`: `OpMove` goldens, and an all-or-nothing test where the second operation of
  a batch is refused and the source is unchanged.
- GUI: draw a connection between two dragged nodes, save, reopen, check the layout
  and the model both came back.

## Known limitations, stated rather than hidden

- The `geometry` view kind is not rendered by `internal/core/view` and no tier here
  adds it; the panel reports it as unsupported.
- Multi-document models render per document. A view exposing elements from another
  open file draws them, but the panel is anchored to one document's URI, and a
  cross-document layout sidecar is out of scope. Rename and delete follow references
  into the workspace's other documents; a reference from a bundled library file, or
  from a document the index holds without the workspace holding its source, still
  refuses the edit.
- Only the requesting document's version travels in the request, so only it can be
  answered `stale` by the server; another document that changed between the server
  computing the edit and the client applying it is caught by the client comparing
  the versions the edit carries, not by a server refusal.
- Tier 3's layout sidecar is tool-defined. SysML v2 §10.2 leaves how a view is drawn
  to the tool, so nothing here claims to be a normative diagram interchange, and no
  attempt is made to read or write another tool's layout.
- The palette writes the notation OpenSysML's writer emits, which is
  spec-conformant but not necessarily byte-identical to what a user would have
  typed. `format` makes it consistent with the file; it does not make it a
  particular author's style.
