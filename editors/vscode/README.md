# SysML v2 for VS Code (OpenSysML)

Syntax highlighting and language support for `.sysml` and `.kerml` files, backed by
OpenSysML's `sysml-lsp` server: diagnostics, hover, go-to-definition, document
symbols, typed completion, a live diagram panel, and Markdown rendering of
native document definitions.

This extension is built and side-loaded from this repository. It is deliberately
**not published** to the Visual Studio Marketplace or Open VSX.

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

`SysML: Open Diagram` opens a diagram of the active model beside it, drawn as
Mermaid from the server's rendering and redrawn as the model is typed.

| | |
| --- | --- |
| **What it draws** | The view the document declares, chosen in the picker when it declares several. A document declaring none is drawn directly, as a model tree, interconnection diagram, state diagram, action flow, sequence diagram or element table — a table is written as Markdown rather than drawn, and is shown as that. A view whose rendering is not supported (`geometry`, `textual`) is listed but not drawable, and the reason is written under the diagram. |
| **Navigation** | Click a node to open the declaration it was built from; moving the cursor in the editor highlights the node whose declaration contains it. A node built from a standard library declaration opens the bundled library file, read-only. |
| **While typing** | A rendering that fails mid-keystroke leaves the last good diagram on screen, dimmed, with the error in the status line: the panel never blanks. What a rendering could not represent is listed under it. |
| **Cost** | The panel asks for a diagram only while visible, and only once an editing burst settles. Mermaid is bundled into the extension, and the panel's CSP allows the bundled script alone — nothing is fetched from the network. |

### Editing from the diagram

The panel's **Add…** menu and a node's right-click menu write to the `.sysml` or
`.kerml` file; the diagram itself is never edited. Each action is turned into a
source-preserving edit by the language server and applied to the editor's buffer
like typed text, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it, the file is the only
source of truth, and the diagram redraws from what the file now says.

| Action | What it writes |
| --- | --- |
| **Add…** (palette) | A member — `part`, `port`, `state`, `action`, a `def`, … — into the declaration under the editor's cursor, else the diagram's one root, else a declaration picked from a list; or a connection between two picked nodes. The kinds offered follow the diagram: an interconnection diagram offers parts, ports and connections, a state diagram states and transitions, an action or sequence diagram actions, control nodes and successions, a tree everything the language has. A member kind that takes a type asks for one. A kind only some bodies declare — `subject`, `actor` and `stakeholder` in a requirement or case, `objective` in a case — is offered only while the diagram draws such a declaration, and goes into one of them. |
| **Add …** (node menu) | The member kinds the node's declaration may hold, into it; or a connection, flow, succession, … from the node to one picked from a list. The connection is written in the nearest declaration that contains both ends, with the ends spelled as paths from it (`tank.fuelOut`). |
| **Rename…** | The declaration's name, at the declaration and every reference in the file. |
| **Delete** | The declaration, its own line and the comment block above it. A declaration something still refers to is refused, naming the referents; **Delete all** removes them too. A declaration another file refers to is refused outright, as is renaming one: an edit rewrites one file. |

An edit that would leave the file with an error it did not have — a type that
does not resolve, a name already taken, a connection end out of scope — is
refused, and the message names the diagnostic. Nodes the file does not declare
(library elements, steps a lowering sequenced) offer *Go to declaration* only.

The command exists only when the server advertises
`experimental: { openSysmlRender: true }`, and the editing menus only with
`openSysmlApplyModelEdit`, so an older `sysml-lsp` keeps working without them.
The requests behind the panel — `opensysml/render`, `opensysml/views`,
`opensysml/applyModelEdit` and the `opensysml/renderChanged` notification — are
documented in [docs/reference/lsp.md](../../docs/reference/lsp.md). Layout is
not persisted and nodes are not dragged: that is the design's Tier 3, not yet
built.

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

## Grammar generation

`syntaxes/*.tmLanguage.json` are generated — do not edit them by hand. The
keyword list comes from `internal/core/lexer.Keywords()`, and the contextual
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
npm test            # unit tests for the edit and menu logic, on node's test runner
npm run check-nodes # render Mermaid fixtures and verify source-node matching
```

`esbuild.mjs` builds two bundles: the extension for Node, and the diagram
webview for the browser with Mermaid bundled in. They typecheck against
different libraries — the webview needs the DOM, the extension must not see it —
so `src/webview` has its own `tsconfig.json`.

Press <kbd>F5</kbd> in VS Code with `editors/vscode` open to launch an Extension
Development Host. `examples/demo.sysml` is a highlighting smoke-test file.
