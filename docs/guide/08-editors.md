# 8. Editors

`sysml-lsp` implements the Language Server Protocol over standard input and output, so any editor
with a generic LSP client can use it. The VS Code extension in
[editors/vscode](../../editors/vscode) also adds `.sysml` and `.kerml` syntax
highlighting; editors built on Tree-sitter get theirs from
[tree-sitter-sysml](https://github.com/Open-MBEE/tree-sitter-sysml), see
[Helix, Neovim and Zed](#helix-neovim-and-zed).

## VS Code

The VS Code extension in [editors/vscode](../../editors/vscode) provides
syntax highlighting for `.sysml` and `.kerml` and an LSP client that launches
`sysml-lsp`. It is not published to any marketplace, so you side-load it: either
the `opensysml-sysml.vsix` the [nightly snapshot](../project/nightly.md) attaches,
packaged from `develop` every night,

```bash
curl -fsSLO https://github.com/Open-MBEE/OpenSysML/releases/download/nightly/opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

or one you build yourself from a checkout:

```bash
make build                                    # builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package                               # -> opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Opening any `.sysml` file highlights it immediately, and the extension starts the first server it
finds, looking in this order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If no server is found, highlighting still works and a warning explains how to build one. To
point the extension at a specific build, use `.vscode/settings.json`:

```json
{
  "opensysml.server.path": "/absolute/path/to/bin/sysml-lsp",
  "opensysml.trace.server": "messages"
}
```

### Strict conformance in the editor

As on the command line, the server reports OpenSysML's own notation extensions, and a namespace
whose memberships are indistinguishable, as warnings. An
editor that can send settings turns on strict conformance with the boolean
`sysml.strictConformance` ([LSP extensions](../reference/lsp.md#strict-conformance-setting)),
after which the diagnostics for every open document are republished as errors. With this
extension, start the server in strict mode instead:

```json
{
  "opensysml.server.args": ["-strict"]
}
```

### Working inside a package-manager project

A workspace folder that is a project — a directory holding a `.project.json`, as a SysML v2
package manager writes it — has the installed projects its usages name indexed along with
its own files, so an `import` of a package an installed project declares resolves in the
editor without opening its file ([From the command line](03-command-line.md#working-inside-a-package-manager-project)
describes what is found). The projects are looked for under the well-known library
directories of the project (`.sysand/lib` for a `sysand` project) and under any directory
named by `OPENSYSML_LIBRARY_ROOTS`, by `sysml-lsp -library-root` or by the `libraryRoots`
list setting ([LSP extensions](../reference/lsp.md#library-roots-setting)); a relative
directory in the setting is taken from the workspace folder. With this extension, name
another provider's directory as a server argument:

```json
{
  "opensysml.server.args": ["-library-root", "/path/to/store"]
}
```

### The diagram panel

<kbd>Alt</kbd>+<kbd>D</kbd> (<kbd>Option</kbd>+<kbd>D</kbd> on macOS) in a `.sysml` or `.kerml` file —
or the preview button in its title bar, *Open Diagram* in its right-click menu or the Explorer's,
or `SysML: Open Diagram` from the command palette — shows a diagram of the model beside the
editor; the same key pressed in the diagram returns to the source. The panel draws the same renderings the
REPL's `%render` command prints, as an SVG canvas of its own, and redraws as you edit the model.

- **Content.** The panel draws a view the document declares (chosen from a dropdown when there
  are several) or, as is usual for a model under development, the document itself, rendered as a
  tree, an interconnection diagram, a state diagram, an action flow, case and mixed diagrams,
  a sequence diagram or a table. A view whose rendering is unsupported (`geometry`, `textual`) stays in the picker and
  explains why it cannot be drawn.
- **Navigation.** Clicking a node jumps to the declaration it was built from, and moving the
  cursor in the editor highlights the node that contains it. A node built from a standard
  library declaration opens the library file it was declared in, read-only (see
  [the standard library in the editor](#the-standard-library-in-the-editor)).
- **Viewing.** <kbd>Ctrl</kbd>+wheel (a trackpad pinch) zooms about the pointer, a plain wheel
  scrolls, and dragging on empty canvas pans; the toolbar's **−**, **+**, **Fit** and **1:1**
  buttons and the bare <kbd>+</kbd>, <kbd>-</kbd> and <kbd>0</kbd> keys do the same. Each panel
  keeps its zoom and position, so a re-opened one comes back where it was. Clicking a node also
  selects it: its relationships and the nodes at their other ends stay bright and the rest dims,
  until <kbd>Esc</kbd> or a click on empty canvas. The toolbar's **Relations** list names the
  relationship kinds the diagram draws, with a count of each (and, while a node is selected, how
  many are at it), and hides or shows each kind for that panel alone; the setting
  `opensysml.diagram.hideRelations` lists the kinds every panel starts with hidden. None of this
  touches the model or the exports.
- **Behavior while editing.** A keystroke that leaves the model unparseable dims the last valid
  diagram and reports the error in the status line beneath it; the panel is never blanked.
  Anything the rendering could not represent is listed below the diagram. A rendering with
  nothing to draw says why (the view exposes nothing, its filter rejects every element it
  exposes, the document has an error) in the status line and in place of the drawing, and the
  same summary is written to the SysML v2 Output channel.
- **Related elements.** *Show related elements…* on a node's right-click menu lists what relates
  to it, grouped by relationship kind with a count for each — what it specializes and what
  specializes it, the definitions typing it and the usages it types, its owner and owned
  members, the elements connections, flows and bindings join it to, the actions its successions
  reach — and the elements you check are drawn in the panel with the
  edges between them and what is already drawn, laid out like any node the model does not place.
  A node drawn this way has a dashed border; *Hide from diagram* leaves it out again (the entry is
  greyed on a node the view itself draws, which only an edit to the view can remove). The list is
  this panel's alone: nothing is written to the `.sysml` file, switching to another view starts
  it afresh, and an element the diagram's kind draws nothing for is reported beneath the diagram.
- **Resource use.** The panel requests a diagram only while it is visible, and only after a
  burst of editing has settled. The panel draws its own SVG, so nothing is fetched from the
  network.

The model is the only thing edited, and new elements are written in the text: the panel
creates nothing. A node's right-click menu (**Go to declaration**, **Rename…**, **Move to…**,
**Delete…**) and a drag on the canvas each become a source-preserving edit of the `.sysml`
file, applied to the editor's buffer like typed text, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it
and the diagram redraws from what the file now says.

Where a diagram's boxes go is the model's decision when it states one: a view whose body places
its elements with the bundled `DiagramLayout` library (`metadata Layout about engine { x = 120;
y = 80; }`, and `Route` for an edge's waypoints) is drawn exactly so, and a node the model does
not place is laid out in layers under its owner by the ELK layered algorithm, its edges
running orthogonally around the boxes (a rendering of more than 600 nodes falls back to a
square grid). The toolbar's **Auto layout** button lays the whole diagram out that way for the
panel alone, the model's `Layout` and `Route` annotations set aside until the next rendering or
**Reset to model**; nothing is written unless you then drag a node, which writes that node's
`Layout` as a drag always does. Dragging a node writes that annotation — into
the view's body when a view is drawn, into the element's own when the document is drawn
directly — as one edit when the pointer is released; dragging the handle on an edge bends it
through a `Route` waypoint. An edge at a node the model places, which ELK's route no longer
fits, is routed at right angles around the other boxes — out of every container except the
ones its ends lie inside — by the panel's bundled libavoid router once the node is dropped,
and drawn straight until the router has loaded, while it is dragged, or when the router finds
no route; that route is the panel's own — it may overlap an ELK-routed edge, which the router
does not see — and written to the model only once one of its waypoints is dragged.
The geometry is on every node and edge the server sends (`x`, `y`,
`width`, `height`, `route`) and in the Mermaid the REPL and the document pipeline write as
`%% layout:` comments, so other clients can honor it; see
[Diagram layout annotations](../project/diagram-layout-annotations.md). A drag applies to the
tree, interconnection, state and action diagrams, which read the annotations back.

The `opensysml.diagram.routing` setting decides whose routes the edges follow. Under `model`,
the default, a stated `Route` is drawn through its waypoints exactly as above, so a diagonal
migrated from another tool stays a diagonal. Under `router`, every edge ELK did not route —
those with a stated `Route` included — is routed at right angles around the boxes by the
panel's router; the stated routes stay in the model (`DiagramLayout` is layout metadata, not
part of the language, so nothing about the model's meaning changes) but are not drawn, and the
waypoint handles are hidden, since a route the panel draws is not one to bend by hand. A node
drag still carries the stated routes between the nodes it moves. Changing the setting re-lays
the open diagrams out at once. The setting is the panel's alone: `sysml -render-lines`, the
Mermaid and DOT exports and the document pipeline keep the stated routes.

#### The diagram in its own window

`SysML: Move Diagram to New Window` (the panel's title bar `…` menu and a model file's offer
it too, and the command palette) moves the diagram into a floating window of VS Code's own,
which can be dragged to another screen. Run from the editor, the command leaves the cursor
where it was; the moved panel keeps its drawing, its chosen view and its style. The diagrams
share one group, and the group goes with the window: a second diagram opened afterwards joins
the floating window beside the first. Set `opensysml.diagram.openIn` to `newWindow` to have
the first diagram opened in a session move out on its own; later ones join it as before.

#### Exporting a diagram

`SysML: Export Diagram` (the title bar's `…` menu and the right-click menu offer it too) saves
the drawn view in a form you pick from a list: Mermaid (`.mmd`) with the model's positions as
`%% layout:` comments, Graphviz DOT (`.dot`) with the positions as `pos` attributes and a
`// layout:` header naming the engine that keeps them, PlantUML (`.puml`) in the Pilot
visualizer's style, D2 (`.d2`) in the same look, Markdown (`.md`), comma-separated (`.csv`) or tab-separated (`.tsv`) values
for a table, or the text form (`.txt`). The list is the
one the connected server advertises, so it matches what that server writes; the pick is sent
as the request's `form`, the server writes that form, and the save dialog opens on the matching
extension and filter. A form the drawn kind has no grammar for — DOT for a sequence, Mermaid for
a table — is refused by the server and the message names the form that kind uses.

Drawing and exporting need a connected server that provides the render methods
([LSP extensions](../reference/lsp.md)); without one, or with an older `sysml-lsp`, the
commands say so. Returning from the diagram to its source needs no server.

### The standard library in the editor

The server bundles the standard library, so a name like `ScalarValues::Integer` resolves
without a copy of the library on disk — and go-to-definition, find-references (with the
declaration included), hover and the diagram panel can all land in it. Such a location is
reported under the `sysml-stdlib:` scheme, whose path is the file's path within the library:

```
sysml-stdlib:///Kernel%20Libraries/Kernel%20Data%20Type%20Library/ScalarValues.kerml
```

In VS Code, <kbd>Ctrl</kbd>+click on a library name opens that file in a read-only editor
showing the bundled text, positioned on the declaring line. The library file is a document like
any other: hover, go-to-definition, the outline and semantic highlighting work inside it, so
navigation continues from one library file to the next (from `Occurrences.kerml` to `Base.kerml`,
say). The editor is read-only and the server refuses an edit to it — the library is what the
server validates the model against, and changing it would change what every diagnostic means.

A workspace that holds the library twice — a `sysml.library` copy under the folder beside the
bundled one, or an `OPENSYSML_LIBRARY_PATH` tree with two copies — gets the
[`library-declared-twice`](../reference/diagnostics.md#library-declared-twice) warning once, on
the first file of the copy references do not resolve to, naming both locations and the one in
use. A copy of one library file beside the bundled library stands in for the bundled file and is
not a duplication; two such copies are, as is a user `library package` declared by two files. The
same warning reaches `sysml -validate`, the prompt, gRPC/Connect validation and the MCP server.

Other clients get the locations too, but need to know how to show the file behind the URI: the
server serves its text through the `opensysml/stdlibContent` request, and a client registers a
content provider for the `sysml-stdlib` scheme that calls it — see
[LSP extensions](../reference/lsp.md#opensysmlstdlibcontent-request) for the request and the
capability that announces it.

After rebuilding the binary, run `SysML: Restart Language Server` from the command palette.
`editors/vscode/README.md` documents every setting, the grammar generator (keywords are taken
from `internal/syntax/source.Keywords()`, so they cannot drift) and the <kbd>F5</kbd>
extension-debugging loop.

Other editors can launch `bin/sysml-lsp` over standard input and output through their own generic
LSP client; only the syntax highlighting is specific to VS Code.

### Running an element from the editor

Above each executable `action`, `state`, `calc`, `constraint` and `requirement` declared
outside another behavior the server offers a *Run* or *Evaluate* code lens naming it, and above
each definition a reference count that opens the references peek. Clicking *Run* or *Evaluate*
saves every unsaved model file and runs the matching `sysml` check over the workspace folders —
`sysml -action Demo::Charge <folder>`, `-state`, `-calc`, `-constraint` or `-requirement` — as
a task, so the result appears in the task terminal and a name another file declares without an
import resolves as it does in the editor. A file outside every folder runs alone. The `sysml`
binary is looked for beside the `sysml-lsp` in use,
then in an open workspace's `bin/`, then on `PATH`. Inside a call's argument list the
parameter list of what it calls appears as you type, and inlay hints show the type a feature
declared without one is inferred to have and the constant a value evaluates to from the model
alone (<kbd>Ctrl</kbd>+<kbd>Alt</kbd> toggles them when `editor.inlayHints.enabled` is
`offUnlessPressed`).

## Helix, Neovim and Zed

[tree-sitter-sysml](https://github.com/Open-MBEE/tree-sitter-sysml) ships Tree-sitter grammars
for both notations — `sysml/` for `.sysml` files and `kerml/` for `.kerml` files, generated from one
shared definition so that a word reserved in one notation stays an ordinary name in the other — with
the query files editors expect under `queries/<lang>/`: highlights, injections (comments and
`rep language "…"` bodies), locals, tags, folds, indents and textobjects. The grammars follow the
OMG textual notation as encoded in the pilot implementation's Xtext at the release OpenSysML pins,
and every model under `examples/` and in the bundled standard library parses with them. They give
an editor highlighting, folding, indentation and textobjects without a running server;
`sysml-lsp` adds diagnostics, completion and navigation on top, declared the same way as any other
language server.

Helix takes both from `languages.toml`; `rev` is a commit on the grammar repository's `develop`
branch:

```toml
[language-server.sysml-lsp]
command = "sysml-lsp"
args = ["--stdio"]

[[language]]
name = "sysml"
scope = "source.sysml"
injection-regex = "sysml"
file-types = ["sysml"]
comment-tokens = "//"
block-comment-tokens = { start = "/*", end = "*/" }
indent = { tab-width = 4, unit = "    " }
language-servers = ["sysml-lsp"]

[[grammar]]
name = "sysml"
source = { git = "https://github.com/Open-MBEE/tree-sitter-sysml", rev = "<commit>", subpath = "sysml" }
```

The `kerml` language and grammar are declared the same way with `subpath = "kerml"`. Then
`hx --grammar fetch && hx --grammar build`, and copy `queries/sysml/` and `queries/kerml/` from the
grammar repository into `~/.config/helix/runtime/queries/`.

Neovim registers the parsers with nvim-treesitter (`location = "sysml"` or `"kerml"`,
`files = { "src/parser.c" }`, `branch = "develop"`) and the server with nvim-lspconfig
(`cmd = { "sysml-lsp", "--stdio" }` for the `sysml` and `kerml` filetypes, which
`vim.filetype.add` maps from the two extensions). A Zed extension points its
`grammars.<name>.repository` at the repository with `path = "sysml"` or `"kerml"` and reuses the
query files. The grammar repository's README carries the complete snippets for each editor and
is the reference when they change.

## OpenCode

[OpenCode](https://opencode.ai) feeds language-server diagnostics back to its coding agent, but
it knows only the servers compiled into it — `gopls` starts for a `.go` file because the
registry inside OpenCode has an entry for it — so `sysml-lsp` has to be declared. The
checkout's [`opencode.json`](../../opencode.json) does that for anyone who opens this repository:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "lsp": {
    "sysml": {
      "command": ["sysml-lsp", "--stdio"],
      "extensions": [".sysml", ".kerml"]
    }
  }
}
```

The same `lsp` block in `~/.config/opencode/opencode.json` enables the server for every project;
see [LSP servers](https://opencode.ai/docs/lsp) in the OpenCode documentation. Either way
`sysml-lsp` must be on `PATH` — `go install github.com/Open-MBEE/OpenSysML/cmd/sysml-lsp@latest`
or the release tarball installs it there, and a checkout that ran `make build` can put its own
build first with `PATH="$PWD/bin:$PATH" opencode`. Two details of the configuration matter:
`lsp` as an object keeps OpenCode's built-in servers enabled alongside this one (omitting the
key disables them all), and OpenCode starts a declared server with the project directory as its
workspace root, so the whole checkout is the server's workspace.

The same file's `mcp` block gives OpenCode's agent the toolchain as tools, through
[`sysml-mcp`](#coding-agents-and-sysml-mcp):

```json
{
  "mcp": {
    "sysml": {
      "type": "local",
      "command": ["sysml-mcp"],
      "enabled": true
    }
  }
}
```

**What the server indexes.** Every `.sysml` and `.kerml` file under the editor's workspace
folders is indexed at start-up, so a name declared in a file you never opened still resolves,
and an open buffer's text stands in for its file on disk. A file opened from outside every
workspace folder — a lone file, or one in another checkout — has its own directory indexed the
same way, so its imports of sibling files resolve rather than being reported unresolved.

**Capabilities advertised at `initialize`**, recorded from a live session with `bin/sysml-lsp`:

- ✅ Document synchronization, incremental (`textDocumentSync.change: 2`)
- ✅ Diagnostics (syntax + semantic errors, published on open and on change)
- ✅ Hover (symbol info, type, multiplicity)
- ✅ Go-to-definition (cross-document navigation, into the bundled standard library as
  `sysml-stdlib:` documents)
- ✅ Find references (workspace-wide search)
- ✅ Completion (trigger characters `:` and `.`; typed kinds and details, `v.` offers members of `v`'s type, `Pkg::` offers that namespace's members, library names included)
- ✅ Document symbols (outline view)
- ✅ Workspace symbols (global search)
- ✅ Document formatting (`textDocument/formatting`; the reply is one small edit per changed
  region, never a rewrite of the whole file, so the editor keeps its undo history, cursor,
  selection and folds)
- ✅ Range formatting (`textDocument/rangeFormatting`; formats the selected lines only, widening a
  partial-line selection to whole lines. Indentation is computed from the whole file's structure,
  so a selection deep in a nested block is indented consistently with its surroundings)
- ✅ Rename, with prepare (`textDocument/prepareRename`, `textDocument/rename`)
- ✅ Semantic tokens, full document and range (`textDocument/semanticTokens/full`,
  `textDocument/semanticTokens/range`; legend advertised at `initialize`,
  keywords/comments/literals from the token stream, names classified from the
  symbol table and the resolver, with the `declaration`, `definition`, `readonly`
  and `abstract` modifiers)
- ✅ Renderings, as the custom `opensysml/render` and `opensysml/views` requests
  and the `opensysml/renderChanged` notification, announced as
  `experimental: { openSysmlRender: true }` — what the diagram panel is built on,
  documented in [LSP extensions](../reference/lsp.md)
- ✅ Standard library documents, as the custom `opensysml/stdlibContent` request,
  announced as `experimental: { openSysmlStdlibContent: true }` — what opens a
  `sysml-stdlib:` location, documented in [LSP extensions](../reference/lsp.md)
- ✅ Code actions (`textDocument/codeAction`): quick fixes for the spelling of an
  unresolved name, importing the namespace that declares it, inserting a missing
  semicolon the parser located exactly; and the `refactor.rewrite` actions on a
  declaration's header that annotate it with a minted element id (an
  `IdentityMetadata::ElementId`, UUID v4, inline in its body or standalone at
  the end of the file) and bind an unbound root namespace to a project
  (`IdentityMetadata::ProjectRef` with a placeholder `projectId` to fill in), see
  [element identity](../project/element-identity-annotations.md). A named
  standard-library element already carries the normative id the specification
  fixes for it — hover states it as `(normative, KerML)` or `(normative, SysML)` —
  so the minting action is not offered on one. A copy of a library file in the
  workspace, rooted at the library's own packages, is that library file: it stands
  in for the bundled one, its elements keep their normative ids and get no minting
  action, until an edit to a root (its name, its `library` keyword, a foreign id)
  makes it the user's file again; see
  [normative library identity](../reference/rdf-mapping.md#normative-library-identity)

- ✅ Signature help (`textDocument/signatureHelp`, triggered by `(` and `,`): inside the
  argument list of a calculation, function or action call, one signature per declaration the
  call may invoke — the overload its arguments select, else those they leave tied — written
  `Fall(h : Real, [g : Real]) : Real`, a bracketed parameter being one with a default. The
  parameter the cursor's argument binds is active, by position for a positional argument
  (the operand of `x->f(…)` counting as the first) and by name for a named one
- ✅ Code lens (`textDocument/codeLens`, resolved on demand with `codeLens/resolve`): a
  reference count on each definition, which the editor's references peek opens
  (`editor.action.showReferences`, whose URI, position and locations the VS Code extension
  converts from their protocol form), and a *Run* (action, state) or *Evaluate* (calc,
  constraint, requirement) command, titled with the element's name, on each executable
  declaration that is not a step or substate of another behavior. The command is
  `opensysml.runElement` with the document URI, the element's qualified name and its kind;
  the VS Code extension implements it, and another client registers a command of that name
- ✅ Inlay hints (`textDocument/inlayHint`): after the name of a feature declared without a
  type, the type it is inferred to have — that of its value, else what the features it
  redefines or subsets give it — when it is more specific than `DataValue`, the result the
  `DataFunctions` operators declare for arithmetic; after a feature's non-literal value, the constant it
  evaluates to from the model alone (`attribute twice = base * 2;` shows `= 20.0` when `base`
  is a package-level `10.0`). A value that reads an instance's feature gets no hint, since
  the model alone does not fix it. Only the declarations in the requested range are read, so
  the hints cost nothing until an editor asks for them

**Not implemented:** semantic token deltas (`semanticTokens/full/delta`; the server keeps no
previous result to diff against, so clients re-request the full set). A client that requests
one gets a method-not-found response rather than a partial result. Quick fixes are offered
only where the repair is unambiguous: a syntax error that could be fixed with either a body or
a semicolon gets none.

**Testing the server:** the protocol is JSON-RPC over standard input and output, so you can send
requests by hand. The following exchange, run against `bin/sysml-lsp`, formats a badly indented
file and renames a definition:

```
→ textDocument/formatting  (file: "package P {\npart def Wheel {\nattribute diameter = 16.0;\n}\npart w : Wheel;\n}\n")
← [{"range": {"start": {"line": 1, "character": 0}, "end": {"line": 1, "character": 0}}, "newText": "    "},
   {"range": {"start": {"line": 2, "character": 0}, "end": {"line": 2, "character": 0}}, "newText": "        "},
   {"range": {"start": {"line": 3, "character": 0}, "end": {"line": 3, "character": 0}}, "newText": "    "},
   {"range": {"start": {"line": 4, "character": 0}, "end": {"line": 4, "character": 0}}, "newText": "    "}]

→ textDocument/rangeFormatting  (range: line 2, character 3 to line 2, character 9)
← [{"range": {"start": {"line": 2, "character": 0}, "end": {"line": 2, "character": 0}}, "newText": "        "}]

→ textDocument/rename      (position: line 1, character 10; newName: "Tyre")
← {"changes": {"file:///tmp/lsp-demo.sysml": [
      {"range": {"start": {"line": 1, "character": 9}, "end": {"line": 1, "character": 14}}, "newText": "Tyre"},
      {"range": {"start": {"line": 4, "character": 9}, "end": {"line": 4, "character": 14}}, "newText": "Tyre"}]}}
```

Each formatting edit touches only the characters that change; lines the formatter would leave
alone get no edit at all, and the range request answers only the edits on the selected lines. The
rename edits the declaration and the `part w : Wheel` reference together, because it works from
name resolution rather than textual replacement. A rename rewrites only the name under the
cursor — an element's long name or its `<short>` name — where it is declared and wherever a
reference spells it, so `part a : O;` still resolves and is left as written when `part def <O> Old;`
is renamed to `Fresh`, and renaming from `<O>` rewrites that reference and not the `Old` ones.
A rename that would change what a name means is refused, and the editor shows the server's
error naming the element the new name would mean: the new name — long or short — already means
something where the element is declared (a sibling's long or short name, or an outer, imported or
inherited name it would shadow), or a reference the rename rewrites, in any open document, would
afterwards read another element (`part w : Tyre;` in a scope that declares its own `Tyre`,
`A::x` renamed to `y` when `A::y` exists, `A` in `A::x` renamed to `B` when a `B` is visible
there — even one with no `x`, which would leave the reference unresolved — `d.x` renamed to `y`
when `d`'s type declares its own `y`, a name that would reach several members at once and so
leave the reference ambiguous, or a name an alias — even an alias for the element itself —
already means there). A name taken only in an unrelated scope is no conflict, nor is renaming a
name to itself.

To check the installation in an editor, open a file containing
`part Wheel { attribute diameter = 16.0; }` and hover over `Wheel`.

---

Next: [client libraries](../clients.md).

## Coding agents and `sysml-mcp`

A language server tells an agent what is wrong with a file; `sysml-mcp` lets the agent *ask*: it
is a [Model Context Protocol](https://modelcontextprotocol.io) server that an agent host —
OpenCode, Claude Code, Cursor, Codex, VS Code's Copilot, Gemini CLI, Devin — starts in the
project directory and calls as tools. Three groups are served: the language server's workspace
(`workspace_diagnostics`, `hover`, `definition`, `references`, `rename`, `format`, `render_view`, ... over
the documents under the root), the runtime (`parse`, `evaluate`, `instantiate`, `execute_action`,
`verify_requirement`, `run_analysis`, `convert`, `migrate`, ...) and REPL sessions
(`session_open`, `session_submit`, `session_run`, the debugger, the solver commands, ...) whose
model, objects and runs persist across the agent's calls. An agent asked to fix a model runs
`workspace_diagnostics`, edits, runs them again; asked whether a requirement holds, it instantiates the
part and calls `verify_requirement` instead of reasoning about the arithmetic.

The server speaks the protocol over stdin and stdout, so it is never run at a prompt; the host
starts it. With nothing installed but Node, the command is `npx -y @openmbee/sysml-mcp`, which
installs the release binary for the platform from npm on first use (`npx -y @openmbee/opensysml-mcp`
is the same package under the name the other OpenSysML npm packages share):

```bash
claude mcp add --transport stdio sysml -- npx -y @openmbee/sysml-mcp -root .
codex mcp add sysml -- npx -y @openmbee/sysml-mcp -root .
```

An installed binary writes the configuration block for any host itself, with
`sysml-mcp -print-config <host>`:

```bash
sysml-mcp -print-config                 # opencode, claude-code, cursor, codex, vscode, gemini, devin
sysml-mcp -print-config claude-code > .mcp.json
sysml-mcp -print-config codex >> ~/.codex/config.toml
sysml-mcp -print-config vscode -print-config-npx   # the npx form
```

The block names the binary by its absolute path, by the command `-print-config-command`
gives, or `npx -y @openmbee/sysml-mcp` with `-print-config-npx`; `-tools lsp,runtime` or
`-root models` go into its `args`. For remote agents, or several agents over one project,
`sysml-mcp -http 8765` serves one long-lived server over the Streamable HTTP transport at
`http://localhost:8765/mcp` (a port alone binds loopback and needs no token; any other address
is served only with a bearer token, `-token-file mcp.token`, that every client sends), and
`sysml-mcp -print-config <host> -http 8765` writes the block that connects to it, the token's
`Authorization` header included when the server has one; the clients
then share the workspace and the `session_*` names. The
[MCP server reference](../reference/sysml-mcp.md) has every host's block, the transports, the full tool
catalogue with what each tool does, and the conventions the tools share: 1-based line and
column positions, paths relative to the root, and nothing written to disk without
`"apply": true`.
