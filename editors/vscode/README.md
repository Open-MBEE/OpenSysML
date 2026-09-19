# SysML v2 for VS Code (OpenSysML)

Syntax highlighting and language support for `.sysml` and `.kerml` files, backed by
OpenSysML's `sysml-lsp` server: diagnostics, hover, go-to-definition, document
symbols, typed completion, a live diagram panel, and Markdown rendering of
native document definitions.

This extension is side-loaded. It is deliberately **not published** to the Visual
Studio Marketplace or Open VSX.

## Install from the nightly snapshot

Every night the newest green `develop` commit is packaged as `opensysml-sysml.vsix`
and attached to the [`nightly`](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly)
prerelease, beside the `sysml-lsp` it was built with (see
[docs/project/nightly.md](../../docs/project/nightly.md)):

```bash
curl -fsSLO https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Its version is `<manifest version>-nightly-<yyyymmdd>-<commit>`, so a later night
installs over an earlier one as an update.

## Build and side-load

```bash
make build          # from the repo root: builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package     # typecheck + bundle + opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Then open any `.sysml` file. The extension finds the server in this order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a repo checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If none exist, highlighting still works and a warning explains how to build the
server. `SysML: Restart Language Server` restarts it after a rebuild.

## The standard library

The server bundles the standard library, so a definition, reference or hover can
land in a file that is not on disk. The extension serves those as read-only
`sysml-stdlib:` documents, fetched from the server with its
`opensysml/stdlibContent` request: <kbd>Ctrl</kbd>+click on `ScalarValues::Integer`
opens the bundled `ScalarValues.kerml` on the declaring line, and hover,
go-to-definition, the outline and semantic highlighting work inside it, so
navigation continues from one library file into the next. The editor cannot be
edited, and the server refuses a change to such a document. An older `sysml-lsp`
that does not advertise the request reports its library locations as before, and
the Output channel says so.

## The diagram panel

A diagram of a model opens beside it as soon as a `.sysml` or `.kerml` file is
shown, drawn as SVG from the server's rendering and redrawn as the model is
typed. The diagram opens quietly: focus stays in the text, and the diagrams of
every file share one editor group, so switching between model files adds a tab
there instead of another column. Nothing opens for a file that is not on disk
— an untitled buffer, a `git:` revision, a diff — or for one only peeked at from
a hover. A file declaring several drawable views opens on its own only when the
cursor sits in one of them or one was chosen for it before; otherwise nothing
opens and nothing asks — `SysML: Open Diagram` does the asking.

Close a file's last diagram and it stays closed for that file, across switches to other
files and across a reload of the window, until you ask for it again. Closing
the file's editor leaves its diagram where it is; renaming the file carries its
open diagram — or the memory of a closed one — along, deleting the file forgets it. To have no
diagram open on its own, turn `opensysml.diagram.autoOpen` off; `SysML: Open
Diagram` then works as before.

`SysML: Open Diagram` brings a diagram back — and lets it open on its own again
— and is a keystroke or a click away from any `.sysml` or `.kerml` file:

| From | How |
| --- | --- |
| **The keyboard** | <kbd>Alt</kbd>+<kbd>D</kbd> (<kbd>Option</kbd>+<kbd>D</kbd> on macOS), the shortcut diagram-preview extensions such as PlantUML use; or <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> (<kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> on macOS), VS Code's own shortcut for a Markdown preview. Both work with the cursor in a model file's text and nowhere else — not in the Find widget, the terminal or another file — so they shadow no other shortcut. Pressed with the diagram panel focused, either key returns to the source, whether or not the language server is running. Rebind them under *Preferences: Open Keyboard Shortcuts* by searching for `opensysml.openDiagram`; `opensysml.exportDiagram` has no default key and takes one the same way. |
| **The editor** | The preview button at the right of the title bar, or *Open Diagram* in the right-click menu; the title bar's `…` menu and the right-click menu also offer *Export Diagram*. |
| **The Explorer** | *Open Diagram* in a `.sysml` or `.kerml` file's right-click menu, which opens the file and its diagram side by side. |
| **The Command Palette** | `SysML: Open Diagram`. With no model file focused, the one model file in view is drawn; with several in view, the command asks which to focus. |

Without a running server, or with a `sysml-lsp` too old to draw, every one of
these says so instead of doing nothing; a diagram that would have opened on its
own just waits for the server.

| | |
| --- | --- |
| **What it draws** | The view the document declares. A document declaring several drawable views opens on the one whose declaration holds the editor's cursor, else the one last chosen for that document in this workspace, else the one picked from a list — the drawable views by name and kind, **All views** to open each in its own panel, and the pseudo-views last; views the server cannot draw are left out of that list (the panel's own picker still shows them, disabled, with the reason), and cancelling opens nothing. A document declaring none is drawn directly, as a model tree, interconnection diagram, state diagram, action flow, sequence diagram or element table — a table is written as Markdown rather than drawn, and is shown as that. A view whose rendering is not supported (`geometry`, `textual`) is listed but not drawable, and the reason is written under the diagram. |
| **Several panels** | A document may have one panel per view open at once; they are titled `Diagram: <file> — <view>` while there are several, each redraws when the model changes, and each highlights the cursor's node. Open Diagram reveals the panel already showing the chosen view, or opens another beside the source for a different one. Picking a view in a panel's picker retargets that panel — unless another panel already draws it, which is revealed instead. Panels come back with their views when the window reloads. |
| **Where things go** | A node the model places — a `DiagramLayout::Layout` annotation in the view's body or the element's own — is drawn exactly there, at the size it states; every other node takes a slot in a grid under its owner, in the order rendered, so the same model draws the same way every time. An edge follows the waypoints its `DiagramLayout::Route` gives it, else runs straight. |
| **Style** | The panel's **Style** list, or the `opensysml.diagram.style` setting, picks the look of every diagram. `theme` (the default) follows the VS Code colour theme. `pilot` is the pilot visualizer's Standard B&W, the look the DOT and PlantUML forms are written in: white canvas, black sans-serif text, thin dark borders, square definitions and rounded usages, a heavier border on a package and a dashed one on a region, bold names over a small italic `«kind»`, thick arrowless connections and dashed flows, filled black pseudo-states. The eight palettes (`okabe-ito`, `tol-bright`, `tol-muted`, `tol-light`, `brewer-set2`, `brewer-dark2`, `viridis`, `cividis`, [described here](../../docs/project/view-rendering-forms.md#palettes)) are that look filled by keyword family — parts one colour, ports another, a usage a lighter tint of its definition's — in the very colours a DOT or PlantUML export of the view takes, since the server names them; text stays black. Changing the list keeps the choice in your settings and redraws every open diagram. A `sysml-lsp` too old to name colours draws a palette as `pilot` and says so under the diagram. |
| **Navigation** | Click a node to open the declaration it was built from; moving the cursor in the editor highlights the node whose declaration contains it. A node built from a standard library declaration opens the bundled library file, read-only. |
| **While typing** | A rendering that fails mid-keystroke leaves the last good diagram on screen, dimmed, with the error in the status line: the panel never blanks. What a rendering could not represent is listed under it. |
| **Cost** | The panel asks for a diagram only while visible, and only once an editing burst settles. The panel draws its own SVG, and its CSP allows the bundled script alone — nothing is fetched from the network. |
| **Export** | `SysML: Export Diagram` saves the diagram in a form picked from a list — Mermaid (`.mmd`), with the model's positions as `%% layout:` comments; Graphviz DOT (`.dot`), with the positions as `pos` attributes and a `// layout:` header naming the engine that keeps them; PlantUML (`.puml`) in the Pilot visualizer's style; Markdown (`.md`) for a table; or the text form (`.txt`) — for the view the document's panel shows; with no panel or several, the document's one drawable view, its model tree when it declares none, or the view picked from a list when it declares several. The list is the one the server advertises (`openSysmlRenderForms`), the pick goes to the server as the request's `form`, and the save dialog opens on that form's extension and filter; a form the drawn kind has no grammar for is refused by the server, and the message names the form the kind uses. |

### Editing from the diagram

The panel's **Add…** menu, a node's right-click menu and dragging on the canvas
write to the `.sysml` or `.kerml` file; the diagram itself is never edited. Each
action is turned into a source-preserving edit by the language server and applied
to the editor's buffers like typed text, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it,
the files are the only source of truth, and the diagram redraws from what the file
now says. A rename or delete that reaches into other files of the workspace changes
them in the same step: one edit, one undo.

| Action | What it writes |
| --- | --- |
| **Add…** (palette) | A member — `part`, `port`, `state`, `action`, a `def`, … — into the declaration under the editor's cursor, else the diagram's one root, else a declaration picked from a list; or a connection between two picked nodes. The kinds offered follow the diagram: an interconnection diagram offers parts, ports and connections, a state diagram states and transitions, an action or sequence diagram actions, control nodes and successions, a tree everything the language has. A member kind that takes a type asks for one. A kind only some bodies declare — `subject`, `actor` and `stakeholder` in a requirement or case, `objective` in a case — is offered only while the diagram draws such a declaration, and goes into one of them. |
| **Add …** (node menu) | The member kinds the node's declaration may hold, into it; or a connection, flow, succession, … from the node to one picked from a list. The connection is written in the nearest declaration that contains both ends, with the ends spelled as paths from it (`tank.fuelOut`). |
| **Rename…** | The declaration's name, at the declaration and every reference that writes it, in this file and in every other file of the workspace. |
| **Move to…** | The declaration — its body, the comment block above it and its own lines — out of its owner and into one picked from a list: the drawn declarations whose body may hold its kind, or the document's top level. References to it and to what it declares are respelled so they still resolve, an import the move leaves dangling or redundant is rewritten or removed, and the destination gets a body if it had none. Moving into itself, into a declaration it holds, or beside a declaration of the same name is refused, as is a declaration another file refers to: a move rewrites one file. |
| **Delete** | The declaration, its own line and the comment block above it. A declaration something still refers to is refused, naming the referents by file; **Delete all** removes them too, in whichever files declare them. |
| **Drag a node** | A `metadata Layout about … { x = …; y = …; }` annotation — in the view's body when a view is drawn, inline in the node's declaration when the document is drawn directly — written when the pointer is released: one edit per drag, whatever the distance, so one <kbd>Ctrl</kbd>+<kbd>Z</kbd> puts the node back. An annotation already there is updated in place, keeping its size and collapsed state; the children the model places move with their owner, and those it does not follow it on their own. |
| **Drag an edge** | A `metadata Route` annotation: drag the handle at the middle of a segment to bend the edge there, drag a waypoint to move it, double-click one to remove it (removing the last removes the annotation). |
| <kbd>Shift</kbd>+**drop a node on another** | What **Move to…** writes for the node under the pointer, plus the `Layout` of where it was dropped, as one edit and one <kbd>Ctrl</kbd>+<kbd>Z</kbd>. While <kbd>Shift</kbd> is held, the node under the dragged one is outlined when its body may hold the dragged declaration and the status line says what releasing does; a node that cannot hold it is not outlined, the pointer says so, and releasing there puts the dragged node back where it was, with the reason in the status line. A drop with <kbd>Shift</kbd> up, or on empty canvas, is a plain drag. What the server refuses — a name already taken in the destination, a declaration another file refers to — is shown in the status line and the node goes back. |

A reference from a file the server cannot rewrite — a bundled library file — refuses
the rename or delete outright. An edit across files is applied only while every file
it names is still as the server saw it; a file typed into meanwhile leaves the edit
unapplied, with a message naming the file, and the action can be taken again.

A drag writes to the diagram kinds that read the annotations back — tree,
interconnection, state and action — and only for nodes and edges the file
declares; a sequence diagram's lifelines and a table are not dragged. Positions
are pixels from the canvas's top-left corner, y downward, as
[the layout annotations](../../docs/project/diagram-layout-annotations.md) define
them. Not built: placing an element in another file's view, and the `geometry`
view kind.

An edit that would leave the file with an error it did not have — a type that
does not resolve, a name already taken, a connection end out of scope — is
refused, and the message names the diagnostic. Nodes the file does not declare
(library elements, steps a lowering sequenced) offer *Go to declaration* only.

The command exists only when the server advertises
`experimental: { openSysmlRender: true }`, and the editing menus only with
`openSysmlApplyModelEdit`, so an older `sysml-lsp` keeps working without them.
Dragging a node another file declares needs `openSysmlCrossDocumentLayout` on
both sides; without it the panel places, and the server names, what the
rendered file declares alone.
The requests behind the panel — `opensysml/render`, `opensysml/views`,
`opensysml/applyModelEdit` and the `opensysml/renderChanged` notification — are
documented in [docs/reference/lsp.md](../../docs/reference/lsp.md).

## Rendering documents

`SysML: Render Document` renders a native document definition — a `part def`
specializing `DocumentQueries::Document` — to Markdown: a quick pick lists the
documents the workspace declares, and the rendering opens beside the editor as
a Markdown preview. It is the pipeline behind the REPL's `%render-document`,
run against the model as currently typed. An error — a query that fails to
plan, a binding that resolves to nothing — is shown as a message rather than a
crash.

The command exists only when the server advertises
`experimental: { openSysmlRenderDocument: true }`. The requests behind it —
`opensysml/documents` and `opensysml/renderDocument` — are documented in
[docs/reference/lsp.md](../../docs/reference/lsp.md).

## Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `opensysml.server.path` | `""` | Absolute path to `sysml-lsp`; empty falls back to the workspace build, then `PATH`. |
| `opensysml.server.args` | `[]` | Extra server arguments. |
| `opensysml.server.enabled` | `true` | Set to `false` for highlighting without a server. |
| `opensysml.trace.server` | `"off"` | Trace LSP traffic in the "SysML v2" output channel. |
| `opensysml.diagram.autoOpen` | `true` | Open a model file's diagram beside it when the file is shown. Set to `false` to open diagrams only with `SysML: Open Diagram`. |
| `opensysml.diagram.style` | `"theme"` | The look every diagram is drawn in: `theme` for the VS Code colour theme, `pilot` for the pilot visualizer's black and white, or one of the eight palettes for that look filled by keyword family. The panel's **Style** list sets the same value. |

## Grammar generation

`syntaxes/*.tmLanguage.json` are generated — do not edit them by hand. The
keyword list comes from `internal/syntax/source.Keywords()`, and the contextual
words the parser reads as syntax without the lexer reserving them (`point`,
`initial`, `var` in `.kerml`, …) from `lexer.ContextualWords()`, so highlighting
cannot drift from either. Generation fails if a word is in both lists, and the
two languages differ where the grammars do — `var` is KerML notation only:

```bash
make vscode-grammar    # regenerate
go test ./editors/...  # fails if the committed grammars are stale
```

## Development

```bash
npm run watch       # rebuild dist/extension.js and dist/webview.js on change
npm run typecheck   # tsc --noEmit, extension and webview
npm test            # unit tests for the edit, menu, layout and canvas logic, on node's test runner
```

`esbuild.mjs` builds two bundles: the extension for Node, and the diagram
webview for the browser, which draws its SVG itself. They typecheck against
different libraries — the webview needs the DOM, the extension must not see it —
so `src/webview` has its own `tsconfig.json`.

Press <kbd>F5</kbd> in VS Code with `editors/vscode` open to launch an Extension
Development Host. `examples/demo.sysml` is a highlighting smoke-test file.
