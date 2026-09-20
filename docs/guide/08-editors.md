# 8. Editors

`sysml-lsp` implements the Language Server Protocol over standard input and output, so any editor
with a generic LSP client can use it. The VS Code extension in
[editors/vscode](../../editors/vscode) also adds `.sysml` and `.kerml` syntax
highlighting.

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

As on the command line, the server reports OpenSysML's own notation extensions as warnings. An
editor that can send settings turns on strict conformance with the boolean
`sysml.strictConformance` ([LSP extensions](../reference/lsp.md#strict-conformance-setting)),
after which the diagnostics for every open document are republished as errors. With this
extension, start the server in strict mode instead:

```json
{
  "opensysml.server.args": ["-strict"]
}
```

### The diagram panel

<kbd>Alt</kbd>+<kbd>D</kbd> (<kbd>Option</kbd>+<kbd>D</kbd> on macOS) in a `.sysml` or `.kerml` file —
or the preview button in its title bar, *Open Diagram* in its right-click menu or the Explorer's,
or `SysML: Open Diagram` from the command palette — shows a diagram of the model beside the
editor; the same key pressed in the diagram returns to the source. The panel draws the same renderings the
REPL's `%view` command prints, as an SVG canvas of its own, and redraws as you edit the model.

- **Content.** The panel draws a view the document declares (chosen from a dropdown when there
  are several) or, as is usual for a model under development, the document itself, rendered as a
  tree, an interconnection diagram, a state diagram, an action flow, a sequence diagram or a
  table. A view whose rendering is unsupported (`geometry`, `textual`) stays in the picker and
  explains why it cannot be drawn.
- **Navigation.** Clicking a node jumps to the declaration it was built from, and moving the
  cursor in the editor highlights the node that contains it. A node built from a standard
  library declaration opens the library file it was declared in, read-only (see
  [the standard library in the editor](#the-standard-library-in-the-editor)).
- **Behavior while editing.** A keystroke that leaves the model unparseable dims the last valid
  diagram and reports the error in the status line beneath it; the panel is never blanked.
  Anything the rendering could not represent is listed below the diagram.
- **Resource use.** The panel requests a diagram only while it is visible, and only after a
  burst of editing has settled. The panel draws its own SVG, so nothing is fetched from the
  network.

The model is the only thing edited: the panel's **Add…** menu, a node's right-click menu and
a drag on the canvas each become a source-preserving edit of the `.sysml` file, applied to the
editor's buffer like typed text, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it and the diagram
redraws from what the file now says.

Where a diagram's boxes go is the model's decision when it states one: a view whose body places
its elements with the bundled `DiagramLayout` library (`metadata Layout about engine { x = 120;
y = 80; }`, and `Route` for an edge's waypoints) is drawn exactly so, and a node the model does
not place takes a slot in a grid under its owner. Dragging a node writes that annotation — into
the view's body when a view is drawn, into the element's own when the document is drawn
directly — as one edit when the pointer is released; dragging the handle on an edge bends it
through a `Route` waypoint. The geometry is on every node and edge the server sends (`x`, `y`,
`width`, `height`, `route`) and in the Mermaid the REPL and the document pipeline write as
`%% layout:` comments, so other clients can honor it; see
[Diagram layout annotations](../project/diagram-layout-annotations.md). A drag applies to the
tree, interconnection, state and action diagrams, which read the annotations back.

#### Exporting a diagram

`SysML: Export Diagram` (the title bar's `…` menu and the right-click menu offer it too) saves
the drawn view in a form you pick from a list: Mermaid (`.mmd`) with the model's positions as
`%% layout:` comments, Graphviz DOT (`.dot`) with the positions as `pos` attributes and a
`// layout:` header naming the engine that keeps them, PlantUML (`.puml`) in the Pilot
visualizer's style, Markdown (`.md`) for a table, or the text form (`.txt`). The list is the
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

**Not implemented:** semantic token deltas (`semanticTokens/full/delta`; the server keeps no
previous result to diff against, so clients re-request the full set), signature help, code lens
and inlay hints. A client that requests one of these gets a
method-not-found response rather than a partial result. Quick fixes are offered only where the
repair is unambiguous: a syntax error that could be fixed with either a body or a semicolon gets
none.

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

Next: [9. From your own program](09-clients.md).
