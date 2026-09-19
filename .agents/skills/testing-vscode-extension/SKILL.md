---
name: testing-vscode-extension
description: How to build, install, drive and record end-to-end GUI tests of the OpenSysML VS Code extension (editors/vscode) and its sysml-lsp language client — server discovery, completion, diagnostics, and the xdotool pitfalls that waste time.
---

# Testing the OpenSysML VS Code extension

## Build & install (always rebuild; a stale vsix looks identical)

Run everything from the repo root (`AGENTS.md` §2: never `cd`):

**First check that a real VS Code desktop exists.** On some boxes `code` on PATH is only the Devin
CLI standalone (`code --version` prints `Devin CLI Standalone`), which refuses
`--install-extension` ("No installation of Devin stable was found"), and the pre-running
`serve-web` instance on `localhost:6789` may render a blank workbench. `sudo` is usually
passwordless, so the fastest fix is installing the real desktop build:

```bash
curl -sL -o /tmp/code.deb "https://update.code.visualstudio.com/latest/linux-deb-x64/stable"
sudo dpkg -i /tmp/code.deb        # takes ~1 min; then /usr/bin/code is the desktop editor
```

Launch it detached or it dies with the shell that started it:
`DISPLAY=:0 setsid nohup /usr/bin/code --no-sandbox --disable-gpu <workspace> >/tmp/code.log 2>&1 </dev/null &`

```bash
make build                # produces bin/sysml-lsp (the server the client discovers)
make vscode-package       # npm ci + typecheck + esbuild + vsce -> editors/vscode/opensysml-sysml.vsix
DISPLAY=:0 code --no-sandbox --disable-gpu --install-extension editors/vscode/opensysml-sysml.vsix --force
DISPLAY=:0 code --no-sandbox --list-extensions --show-versions   # expect open-mbee.opensysml-sysml@<ver>
```

Restart VS Code after installing, otherwise the old extension host keeps running:

```bash
pkill -f "no-sandbox --disable-gpu /home"      # plain `pkill -f /usr/share/code` may miss it
DISPLAY=:0 nohup code --no-sandbox --disable-gpu "$PWD" &   # $PWD = repo root, the workspace to open
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Workspace trust must be granted — Restricted Mode silently disables the extension (no LSP, no outline).
The trust banner appears on the Welcome tab; "Manage" → "Trust" reloads the window.

**Always open the repo *folder*, not a lone `.sysml` file.** `code <file.sysml>` gives the window no
workspace folder, so `resolveServer` skips the `<workspace>/bin/sysml-lsp` fallback, finds nothing on
PATH, and you get an empty "SysML v2" channel plus 0 problems — which looks exactly like a broken
server. Launch with the repo root as the argument, then open the file from the Explorer.

## Server discovery (`editors/vscode/src/extension.ts`)

**`--stdio` crash loop (seen at b3f16e4, pre-existing on `main`).** The client uses
`TransportKind.stdio`, which makes vscode-languageclient append `--stdio` to the server argv, while
`cmd/sysml-lsp/main.go` parses flags with `flag` and rejects unknown ones. The symptom is a toast
`Client SysML v2 Language Server: connection to server is erroring. write EPIPE`, then in the
"SysML v2" output channel `flag provided but not defined: -stdio` / `Usage: sysml-lsp [options]` /
`Server process exited with code 2` and finally
`server crashed 5 times in the last 3 minutes. The server will not be restarted.`
`pgrep -a sysml-lsp` is empty and the Problems panel stays empty (which looks exactly like "the
feature under test produces no diagnostics" — always check `pgrep` before believing that).
Reproduce outside the editor with `./bin/sysml-lsp --stdio </dev/null`.
Workaround for testing (no repo edit needed): a wrapper that drops argv, pointed at from the
**Settings UI** (`Ctrl+,` → search `opensysml.server.path`, User scope) — the setting has an
`onDidChangeConfiguration` handler, so the server restarts on Enter and
`Starting /tmp/lsp-wrap.sh` appears in the channel:

```bash
printf '#!/bin/sh\nexec /home/ubuntu/repos/OpenSysML/bin/sysml-lsp\n' > /tmp/lsp-wrap.sh
chmod +x /tmp/lsp-wrap.sh
```

The proper fix (report it, do not apply it while testing) is either accepting/ignoring `-stdio` in
`cmd/sysml-lsp/main.go` or dropping `transport` from the `ServerOptions` in `extension.ts`.

Order is `opensysml.server.path` → `<workspace>/bin/sysml-lsp` → `sysml-lsp` on PATH. On this box
`sysml-lsp` is normally NOT on PATH, so a green run genuinely exercises the workspace `bin/` fallback.
Proof lives in the **"SysML v2" output channel**: `Starting /home/.../bin/sysml-lsp`.
Cross-check the process with `pgrep -a sysml-lsp` — the pid is the cheapest, most reliable signal for
"did the server restart/crash?" (the fixed double-read-loop bug manifested as repeated restarts and
`missing Content-Length header` lines in that channel).

Setting `opensysml.server.path` to a bogus path is expected to show a warning
("Could not find sysml-lsp ... Syntax highlighting still works"), kill the server (pgrep empty),
empty the Outline view, but keep TextMate colors. Clearing the setting auto-restarts (there is an
`onDidChangeConfiguration` handler), and `SysML: Restart Language Server` restarts it too.
`[Error - hh:mm:ss] Server process exited with code 0.` in the channel is benign shutdown noise from
vscode-languageclient after a clean stop — not a crash.

## Completion expectations (`internal/frontend/lsp/completion.go`)

Trigger characters are `.` and `:`. Inside a body:
- `engine.` → only that type's members with real kinds/details (`power` → `attributeUsage`,
  `start` → `actionUsage`). A member whose nearest declaration is an untyped redefinition (e.g.
  `attribute redefines power = 110.0;` in `part myCar`) shows no `: Type` suffix — that is correct,
  the nearer declaration wins.
- Unresolved path (`zzz.`) → LSP returns nothing. VS Code then shows its own **word-based**
  suggestions (plain `abc` icons, no detail column). Do not mistake those for a keyword dump; the
  real LSP keyword items carry the detail text `keyword`. A good contrast shot: Ctrl+Space on an
  empty line shows LSP items with `keyword` details and `{}` library packages.
- `ScalarValues::` → library members (`Real`, `Boolean`, `Integer`, ... with `attributeDef` detail).

## Semantic tokens (`internal/frontend/lsp/semantictokens.go`, `internal/semantic/highlight`)

The client enables `textDocument/semanticTokens/full` automatically; only `editor.semanticHighlighting.enabled`
gates it (note a workspace `.vscode/settings.json` value overrides the User setting — flip it in the
**Workspace** tab or the toggle looks like a no-op).

The reliable, screenshot-able proof is Command Palette → **Developer: Inspect Editor Tokens and
Colors**: it shows a `semantic token type` / `modifiers` block plus the TextMate scope it *struck
through*. Move the cursor with **Ctrl+G `line:col`** while the inspector is open — the panel covers
the code and blocks clicks, and VS Code's Col is UTF-16 based, so it doubles as the UTF-16 column
check for tokens after astral-plane characters (a 🚗 counts 2).

Toggling semantic highlighting off is only a *sometimes* visible before/after: this TextMate grammar
gives type names the same `#4EC9B0` the semantic `class` gets, so compare a **keyword**
(`package`: semantic `#C586C0` vs TextMate `#FF7B72`), not a type name.

Deltas (`semanticTokens/full/delta`) are deliberately unimplemented — the server answers -32601;
verify that over stdio JSON-RPC, not from the GUI.

## Quick-fix code actions (`internal/frontend/lsp/codeaction.go`, `internal/semantic/resolve/fixes.go`)

Cursor on the diagnostic + **Ctrl+.** (`ctrl+period` via xdotool works). Copilot always injects its
own `Fix`/`Explain` entries, so "no server fix offered" looks like a menu with *only* those two —
not the "No code actions available" message. Expected titles/edits:
- near miss: `Change 'Wheeel' to 'Wheel'` (preferred)
- resolvable elsewhere: `Change 'Integer' to 'ScalarValues::Integer'` **and** `Import 'ScalarValues::*'`,
  the latter inserted as its own line before the first member, matching that member's indentation
- missing `;`: `Insert ';'` — the diagnostic sits on the *next* token (often the closing `}` a line
  below), but the edit must land at the end of the previous statement
- ambiguous (`expected '{' or ';' after declaration`): no fix, by design

A fast way to learn exact titles/ranges before driving the GUI is a small stdio JSON-RPC probe
script against `bin/sysml-lsp` (initialize → didOpen → semanticTokens/full → codeAction).

## Lifecycle / process-leak testing (`cmd/sysml-lsp/main.go`, `internal/frontend/lsp/lifecycle.go`)

- The client always appends `--stdio` (`vscode-languageclient/lib/node/main.js`: `TransportKind.stdio`
  → `args.push('--stdio')`), so the server binary must accept that flag or the client crash-loops.
- `pgrep -af sysml-lsp` run from a shell whose own command line contains the string `sysml-lsp`
  matches that bash process and gives a false positive. Put the check in a tiny script
  (`/tmp/lspcheck.sh`) and call it, so the output is only real servers.
- Expected argv while a window is open: exactly one `<repo>/bin/sysml-lsp --stdio`. After **File →
  Close Window** it must disappear within a few seconds; a surviving process is the leak bug.
- Cheap, high-signal stdio probe for exit statuses (no GUI): initialize → `shutdown` → any request
  (expect error `-32600`) → `exit` ⇒ process status 0; initialize → `exit` with no shutdown ⇒ status 1.
  A process still alive after 10 s is the old bug.
- A convincing **negative control** without touching the branch: `git worktree add /tmp/wt-main
  origin/main`, `go build -o /tmp/sysml-lsp-main ./cmd/sysml-lsp` in it, copy over `bin/sysml-lsp`,
  reopen the folder — the channel then shows `flag provided but not defined: -stdio`,
  `Server process exited with code 2` and `crashed 5 times in the last 3 minutes`. Restore the fixed
  binary and run **SysML: Restart Language Server** to recover in the same window (no relaunch needed).

## GUI driving pitfalls (cost real time here)

- `ctrl+shift+u` via xdotool types a literal `u` into the editor instead of opening Output. Use the
  **View → Output** menu, or the Output tab in the bottom panel.
- The **"SysML v2" channel is often missing from the Output panel's channel dropdown**. Reliable route:
  Command Palette → **"Output: Show Output Channels..."** → type `SysML` → Enter.
- `shift+alt+a` (Toggle Block Comment) does not reach VS Code; use Command Palette
  "Toggle Block Comment" instead. `ctrl+slash` works fine.
- Typing long lines with `type` while the suggest widget is open silently swallows characters
  (`Engine` → `ngine`). Press `Escape` between chunks, or accept that stress-test text is garbled.
- The bottom panel is read-only: click into the editor area before typing, and close the panel
  (X at its top-right) before triggering completion so the popup has room.
- Undo a stray edit with Command Palette **"File: Revert File"** — it is far more reliable than
  counting Ctrl+Z presses, and leaves the git tree clean.

## Multi-file / workspace-indexing testing (`internal/frontend/lsp/files.go`, `sync.go`)

The cleanest fixture is a **throwaway folder outside the repo** (e.g. `/home/ubuntu/ws-multifile`)
holding only a couple of tiny models, so the Problems count is entirely about the feature:

```
lib.sysml   package Lib { part def Widget; }
main.sysml  package Main { import Lib::*; part w : Widget; }
```

- A workspace outside the repo has no `bin/sysml-lsp`, so point `opensysml.server.path` at
  `/home/ubuntu/repos/OpenSysML/bin/sysml-lsp` (User `settings.json` or the Settings UI) — the
  setting takes precedence and restarts the server on change.
- Writing `~/.config/Code/User/settings.json` with `"security.workspace.trust.enabled": false` and
  `"workbench.startupEditor": "none"` avoids the trust banner and the Welcome tab entirely.
- The **Problems panel (`ctrl+shift+m`) plus the status-bar error count** is the high-signal oracle
  for indexing tests: "unresolved reference: Lib/Widget" appearing/disappearing is the whole test.
- A convincing negative control is a one-line switch of `opensysml.server.path` to a binary built
  from `origin/main` (`git worktree add /tmp/wt-main origin/main && go build -C /tmp/wt-main -o
  /tmp/sysml-lsp-main ./cmd/sysml-lsp` — build *in the worktree*, or you rebuild the branch);
  the pre-indexing server shows the unresolved references on the same file.
  Switching the setting back auto-restarts — no window reload needed. Remember `git worktree remove`.
- Watcher tests (create/change/delete a `.sysml` outside the editor) are driven from the shell with
  `printf > file` / `rm`; VS Code's `**/*.{sysml,kerml}` watcher forwards them and the Problems panel
  updates within ~1-2 s. A deleted file whose tab is still open keeps the buffer authoritative — the
  diagnostics only change when that tab is closed.
- Verify the server was not silently restarted between steps: `pgrep -af sysml-lsp` pid must be
  unchanged (put it in `/tmp/lspcheck.sh` per the note above).

### More GUI driving pitfalls found here
- `shift+F12` (Find All References) does not reach VS Code through xdotool — use Command Palette
  **"References: Find All References"**. `F12` (go to definition) does work.
- Closing an editor tab: **middle-click the tab**. Clicking the tab's little `x` needs a hover first
  and the coordinates shift whenever the sidebar collapses; `ctrl+w` is risky (can close the window).
- `key` actions take ONE combo: `"shift+Down shift+Down"` errors with `unknown key`; send two actions.
- Opening a file by name with `ctrl+p` → type `lib.sysml` → Enter is far more reliable than clicking
  the Explorer tree, especially after the sidebar has been toggled.
- **Do not use "completion after `Qualifier::`" as the completion oracle.** As of 0b239642 typing
  `attribute a : ScalarValues::` and pressing `ctrl+space` yields "No suggestions", and with a
  trailing `::` the file is also a syntax error (`expected a name after '::'`), which suppresses
  semantic completion. Use a *plain* name prefix instead: in a syntactically valid file, `Wh` +
  `ctrl+space` returns LSP items with a type detail (e.g. `Wheel  partDef`) — the detail column is
  what distinguishes real LSP items from VS Code's word-based (`abc` icon) suggestions.
- A stray `u` from `ctrl+shift+u` is easiest to remove by selecting the whole buffer (`ctrl+a`) and
  retyping the fixture; `ctrl+z` after an LSP-driven edit sometimes only reverts part of it.
- After rebuilding `bin/sysml-lsp`, run Command Palette **"Developer: Reload Window"** so the client
  respawns against the new binary; killing the server process alone can leave the old one in use.

## Diagram panel / `opensysml/render` testing (`editors/vscode/src/diagram.ts`, `src/webview/diagram.ts`)

- Fixtures: `examples/views-demo.sysml` declares 7 views (3 drawable trees, 3 `not drawable`);
  `examples/state-machine-demo.sysml` declares none and therefore exercises the `#tree` pseudo-view
  fallback. `views-demo.sysml` has ~13 pre-existing Problems on the branch — do not use the Problems
  count as the pass/fail oracle for rendering.
- Learn every expected value *before* driving the GUI with a stdio JSON-RPC probe against
  `bin/sysml-lsp`: `initialize` (check `capabilities.experimental.openSysmlRender`), `textDocument/didOpen`,
  `opensysml/views`, `opensysml/render` (`{textDocument, view, form:"mermaid"}`), then `didChange` to see
  the `opensysml/renderChanged {textDocument, version}` push. That gives you node ids, node counts,
  origin line numbers and notice counts to assert against pixels.
- With a declared-view document the panel opens with **no view selected** and the status line asks you to
  name one — that is the implemented initial state, not a failure. Pick a view in the picker.
- **Tables are not Mermaid.** As of 54118526 the client requests *no* `form`, so the server returns
  `form=markdown` for a table kind and the webview shows the artifact text in a `<pre>` (header row
  `| Element | Kind | Type | Declared in |`). If a client build ever hardcodes `form:"mermaid"` again,
  `#table` fails with `a table rendering is not written as mermaid; ask for text or markdown`.
  A **declared** `render asElementTable` view (`LanderViews::partsTable`) does render — a probe from a
  workspace holding only `views-demo.sysml` reports it `supported:true`, `form=markdown`. It only lists
  `supported:false` when the workspace *is this repository*, because the parser fixture
  `tests/parser/testdata/parse/view_expose.sysml` declares a `package Views` that shadows the
  standard library's, so `render asElementTable` no longer resolves to a standard rendering. Test the
  diagram panel from a scratch folder, not the repo root, or expect that shadowing.
- "Never blank" needs the webview-state cache. Hiding the panel (switching the other tab group to a
  different tab) tears the webview down; on re-show it reconstructs from `vscode.getState().last`. Test
  it explicitly, and test it with a view whose render *errors* — that is the case that blanked before
  the cache existed.
- **The panel draws its own SVG** (`editors/vscode/src/webview/{layout,canvas}.ts`), not Mermaid: each
  node is a `g[data-opensysml-id="<serverNodeId>"]`, class `movable` when a drag can write a
  `DiagramLayout::Layout` for it, each edge a `g[data-edge="<index>"]`, its route handles
  `circle[data-point]` (waypoint) and `circle.segment[data-segment]` (bend here). Diagnose
  click-to-source / highlight (not with the GUI) in Command Palette → **Developer: Open Webview
  Developer Tools**, `active-frame (index.html)` context:
  `document.querySelectorAll('#diagram svg g[data-opensysml-id]').length`,
  `document.querySelectorAll('#diagram .movable').length`, and after moving the cursor
  `document.querySelector('#diagram .opensysml-selected')?.dataset.opensysmlId`.
- **Drags are pointer-captured on `#diagram`**, so a `dblclick` there names the container, never the
  handle under it — waypoint removal is two quick clicks paired by the webview itself (`< 400 ms`,
  no movement between press and release). Deliver it as one double-click action (the computer
  tool's double-click, or xdotool `click --repeat 2 1`); two clicks from separate exec calls are
  too far apart and read as two single clicks.
- A dragged node writes the annotation on **pointer release** only — one edit per drag, so one
  Ctrl+Z (focus in the text editor) undoes the whole move. When a declared view is drawn the
  `metadata Layout about <element> { x = …; y = …; }` lands in the view body; when a pseudo-view
  (`#tree`, `#interconnection`, …) is drawn it lands inline in the element's own body as
  `@DiagramLayout::Layout { x = …; y = …; }`. In this repository's root workspace the parser
  fixture's `package Views` shadows the standard library, so declared views are `(not drawable)`
  and only the inline placement can be exercised — use a scratch workspace for the view-body case.
- **Judging "the node is outlined" needs the right fixture.** `LanderViews::overview` draws 14 boxes
  scaled down to ~30 px wide, where a 3 px stroke is unreadable in a screenshot. Use a small view —
  `LanderViews::safetyView` renders 4 large boxes — so the focus-border outline is unmistakable.
  For an objective check independent of eyeballing, crop the same diagram rectangle out of two
  full-resolution screenshots and run `compare -metric AE a.png b.png null:` — `0` means nothing
  changed on screen at all. (Screenshots land at 1600x1200 while the tool's coordinate space is
  1024x768, so multiply crop coordinates by 1.5625.)
- **Cursor-to-node ranges run declaration-start → next-declaration-start**, so a cursor in the *leading
  indentation* of a declaration line resolves to the **previous** element, and column 1 of the first
  declaration line matches nothing. Drive this test with `Ctrl+G line:col` (e.g. `112:20`) or by
  clicking on the declaration text, never `Ctrl+G <line>` alone, or you will report a false failure.
- Live-redraw proof: type a new member (e.g. `part booster : Tank;`) inside a `part def` that the view
  exposes and screenshot before/after — a new box must appear with no manual refresh. Undo with
  Command Palette **File: Revert File** to keep the git tree clean.
- Parse-error proof: the dimmed (`.stale`) diagram with a red status line appears only when
  `opensysml/render` itself fails. A missing trailing `}` or a missing package `{` is recovered by
  the parser and the recovered model renders brightly, so a syntax diagnostic in Problems is not
  the oracle — pick a break the renderer refuses (e.g. misspell the `render as…` kind of the
  declared view being drawn) and confirm the red status line names the error, then compare box
  brightness before/after.

## TextMate grammar changes (`editors/vscode/syntaxes/*.tmLanguage.json`, `tools/gengrammar`)

- A *real* before/after for a grammar rule needs no branch switching and no repackaging: install the
  vsix once, then swap the single JSON in the **installed** extension and reload the window.
  ```bash
  git show origin/main:editors/vscode/syntaxes/sysml.tmLanguage.json > /tmp/sysml-main.tmLanguage.json
  EXT=~/.vscode/extensions/open-mbee.opensysml-sysml-0.1.0/syntaxes/sysml.tmLanguage.json
  cp /tmp/sysml-main.tmLanguage.json "$EXT"                                    # BEFORE
  cp editors/vscode/syntaxes/sysml.tmLanguage.json "$EXT"                      # AFTER (restore!)
  ```
  Command Palette → **Developer: Reload Window** picks the new grammar up; nothing else is needed.
  Always restore the branch copy when done — the installed file is not under git, so `git status`
  will not remind you.
- **Semantic tokens beat TextMate for declared names**, so a word that a new TextMate rule colours
  may still render plain where the LSP marks it as a usage/variable. Example: with the F9
  `keyword.other.contextual` rule, `defer Alarm;` / `region r { }` / `entry point p;` colour as
  keywords, but `attribute point : ScalarValues::Real;` keeps the plain identifier colour. Pick the
  screenshot line accordingly, and prove the scope with **Developer: Inspect Editor Tokens and
  Colors** (`region` → `keyword.other.contextual.source.sysml`, foreground `#FF7B72`).

### Completion oracle for keyword-ish items

Type a real prefix (`poi`, `def`, `var`) then `ctrl+space` and read the **detail column** in a `zoom`:
`keyword` = reserved, `contextual keyword` = the F9 list, no detail + `abc` icon = VS Code word-based
noise. Language gating is visible this way too (`var` only in `.kerml`).
Beware the server **dedupes by label**: in a document that already declares `attribute point`, the
contextual item `point` disappears (a document symbol wins). Use a fixture *without* those names when
asserting the contextual list, and a separate fixture with them for the "still ordinary names" test.
Confirm the whole expected list cheaply first with a stdio JSON-RPC completion probe against
`bin/sysml-lsp`, then prove it in the GUI.

## Name resolution / alias / rename testing (`internal/semantic/resolve`, `internal/frontend/lsp/rename.go`)

- **Never name a fixture package after a standard-library package.** `internal/workspace/libs/stdlib`
  ships `Domain Libraries/Geometry/ShapeItems.sysml`, which itself declares
  `alias Box for RectangularCuboid`. A fixture `package ShapeItems { ... alias Box for Cube; }`
  therefore collides: `%explain ShapeItems::Box` reports `is ambiguous`, and a broken
  `ShapeItems::Box` reference can still resolve (to the stdlib alias), silently masking failures.
  Use a unique package name (`Shapes`, `Demo`) and re-run any assertion first taken with a colliding
  name. Grep before choosing a name:
  `grep -rn "\balias Box\b" internal/workspace/libs/`.
- **LSP rename/references do NOT go through `internal/check/edit/rename.go`.** `Server.Rename` uses
  `Workspace.ResolveReferenceNameSegmentsInDoc` (the name a segment *wrote*, so an alias use belongs
  to the alias) and `References` unions that with `ResolveReferenceSegmentsInDoc`, comparing with
  `symbols.SameElement`. A resolver change to segment identity therefore changes rename/references
  even when `go test ./internal/check/edit` is green: test both in the editor *and* with a probe.
- Cheap oracle before driving the GUI: a stdio JSON-RPC probe that sends `textDocument/rename`
  (with `newName`) and `textDocument/references` for both the alias declaration and the target
  declaration, printing `(line, char, newText)` per edit. Run the same probe against a
  `origin/main`-built server; a swapped set of edit positions is the whole finding.
- A **regression control inside one window**: `Ctrl+,` → `opensysml.server.path` → point at
  `/tmp/old-sysml-lsp` (main build), redo the rename, then point it back. Editing
  `~/.config/Code/User/settings.json` from the shell plus `Developer: Reload Window` is the most
  reliable way to force the swap; confirm it with `pgrep -af sysml-lsp` (put it in a script so the
  grep does not match itself).
- **Expected alias difference vs main once references union both identities:** references on a target
  (`Cube`) include alias-written uses (`part p : Box`, `Shapes::Box`) — e.g. 6 hits where a
  main-built server reports 4 — while references on the alias list only occurrences of its own
  written name. Main also still lands F12 on the `alias ... for ...` line instead of the target
  declaration. Both are intended changes, not regressions; verify the direction before reporting.
- Right after a window/server start, `F2` can answer **"The element can't be renamed / no renameable
  name at this position"** even on a valid name. It is usually a not-yet-indexed document, not a
  failure: `Escape`, wait a few seconds, click inside the identifier again and retry before
  concluding anything.
- `bin/sysml-lsp` accepts `--stdio`, so the `/tmp/lsp-wrap.sh` workaround above is not needed — check with `timeout 5 ./bin/sysml-lsp --stdio </dev/null; echo $?` (0 = fine).
- Judging a rename: the oracle is the **Problems panel after the rename**. A rename that leaves the
  model with new `unresolved reference: X` errors is a failure even if the declaration was renamed.
- Completion after a trailing `Qualifier::` *does* work for member lookup (`Shapes::Box::` →
  `length/width/height` with `attributeUsage` details) even though the file is momentarily a syntax
  error; keep the fixture otherwise valid and `Escape` + revert the line afterwards.

### Overload / ambiguous-call navigation (`internal/frontend/lsp/definition.go`, `hover.go`, `references.go`, `rename.go`)

- A compact fixture: two packages each declaring `calc def pick { in x : Integer; ... }`, one of them
  also `calc def pick { in x : String; ... }`, and a `package Use` importing both with
  `attribute a : String = pick(2);` / `attribute b : String = pick("s");`. Expect one error
  `call of pick is ambiguous between OvA::pick, OvB::pick` on `pick(2)` and nothing on `pick("s")`.
  The two `Duplicate of other owned member name` **warnings** on the same-named `calc def`s are
  pre-existing (identical on a main build) — do not report them.
- `F12` on the tied call opens a **peek "Definitions (2)"** listing the overloads in declaration
  order; on a selected call it jumps straight to the winning overload. Hover on the tied call shows a
  fence `calc def OvA::pick` / `calc def OvB::pick` plus `Ambiguous call: the arguments fit each of
  these overloads equally.`; on a selected call it is just `calc def pick` (no FQN — that is the
  ordinary hover path, not a bug). Both hovers stack under the diagnostic popup, so read the lower
  part of the tooltip.
- References on the *declaration* of an overload the call does not select must NOT list the tied
  call; rename from the tied call is refused with a tooltip
  `cannot rename "pick": the call is ambiguous between several overloads` (no rename box).
- The main-build negative control differs on all four surfaces (F12 = 1 location, plain hover,
  rename succeeds editing the first overload, OvA::pick references include the call), so the
  Settings-UI `opensysml.server.path` swap described above is a strong before/after here.
- `ctrl+shift+m` (Problems), `ctrl+g` (Go to Line `line:col`), `F2`, `F12`, `ctrl+comma` all reach
  VS Code via xdotool; `F1` opens the Command Palette (use it for "References: Find All References").

## Metadata annotation body testing (`internal/frontend/lsp/metadata.go`, `internal/workspace/model/metadata.go`)

For `@Anno { x = ...; }` bodies (KerML 7.4.7 implicit redefinition), a compact fixture is
`metadata def Base { attribute inherited; }` / `metadata def Anno :> Base { attribute own : ScalarValues::Integer; }`
annotated on an `item p { attribute outer; }`, plus `item q { @Missing { ghost = 1; } }` as the
degradation case.

- Hover on a body declaration name reads `enumUsage own redefines Anno::own : ScalarValues::Integer` —
  the kind text is `enumUsage` (not `attributeUsage`); don't report that as a bug, it is what the
  server has always shown for these declarations. The `redefines <FQN> : <type>` clause is the new part.
- `unresolved reference: Missing` on the `@Missing` metaclass name is a **pre-existing diagnostic**
  (identical on a main-built server) and does not contradict "quiet degradation" — degradation means
  no diagnostics on the *body declarations* (`ghost`), plain hover without a `redefines` clause, and
  F12 answering "No definition found" without crashing the server. Prove pre-existence with the
  `origin/main` worktree build (see the negative-control recipe above) before flagging any diagnostic.
- Completion inside the body is position-sensitive: at a declaration position the list is *exactly*
  the metadata definition's features (2 items, `attributeUsage` details); after `=` on the same line
  it is the enclosing scope chain (sibling attributes, items, all library packages — hundreds).
  Trigger the declaration-position case on a *fresh empty line* inside the body (End, Enter,
  Ctrl+Space); typing `own = ` first flips it to the value case.
- Confirm every expected string first with a stdio JSON-RPC probe (hover/definition/completion/
  documentSymbol/semanticTokens) against `bin/sysml-lsp` so GUI deviations are real findings.
- Body names carry semantic token type `enumMember` with modifiers `declaration readonly`
  (Inspect Editor Tokens and Colors).

### Sequence diagrams and the pseudo-view picker

- Ready-made sequence fixtures live in `internal/ir/view/testdata/`: `sequence.sysml`
  (`SequenceViews::pubSubView`, 3 participants `part producer/server/consumer`) and
  `sequence-vehicle.sysml` (`VehicleSequenceViews::startVehicleView`, 2 participants
  `part driver (Driver)` / `part vehicle (Vehicle)`). Copy them into a scratch workspace; both
  auto-select in the panel because they declare exactly one view.
- A sequence diagram is drawn as lifelines (a head box per participant, a dashed line under it,
  messages as arrows between lines). The head boxes are the participants' `g[data-opensysml-id]`
  and answer clicks and highlight; nothing in a sequence diagram is `movable` — a drag on a lifeline
  is a click, and no `Layout` is written. `npm test` (`src/webview/canvas.test.ts`) draws the same
  shapes under jsdom and is the cheapest pre-GUI check.
- The picker's pseudo-view entries come from the server's `opensysml/views` → `pseudoViews`
  (`internal/frontend/lsp/render.go`, `view.PseudoViewSpecs()`), labelled by
  `PSEUDO_VIEW_LABELS` in `editors/vscode/src/diagram.ts` — e.g. `#sequence` →
  `Message sequence (no view declared)`. A pre-#624 server omits the field and the client falls back
  to a 5-entry historical list, which makes a **server build from before the change the perfect
  negative control** for "is the picker really server-driven?".
- An **unsupported** view (`geometry`) is rendered as a *disabled* `<option>` with text suffix
  `(not drawable)`, and its `reason` is also written under the diagram in a `1 view not drawable`
  collapsible (`#undrawable`) — expand it to assert the reason text on screen, rather than hovering
  the option's `title`. A geometry-view fixture is `internal/ir/view/testdata/errors.sysml`
  (`ErrorViews::geometryView`); `examples/views-demo.sysml` no longer declares any unsupported view
  (all 7 of its views are `supported:true` when opened from a scratch folder).
- To exercise the **pluralised** summary (`N views not drawable`) no committed fixture has two
  unsupported views, so author one: a `view x : GeometryView { expose P::Widget; }` plus a
  `view y { expose P::Widget; render asTextualNotation; }` gives one `geometry` and one `textual`,
  and add a third drawable `: GeneralView` view so the panel has something to draw beside the list.
  Note the section is filled from `fillPicker`, so it survives selecting a drawable view — assert
  both the no-view-selected and the rendered states.
  The `textual` reason ends in a backticked shell command; `textContent` shows the backticks
  literally on screen, so do not read them as markdown rendering failure.
- The `#state` pseudo-view on `examples/state-machine-demo.sysml` renders
  `the rendering is empty: nothing the view exposes is shown by a state rendering` — this is
  **pre-existing** (identical on a main-built server, the file's root element is a `part def`). Use the
  declared `LanderViews::descentStates` view in `views-demo.sysml` for state-diagram click-to-source.

### More GUI pitfalls

- **Never `pkill -f "no-sandbox --disable-gpu"` from the exec tool**: the pattern matches the
  wrapping `bash -c` command line, so the shell kills itself and the relaunch in the same command
  never runs (symptom: no window, `/tmp/code.log` does not exist). Kill by a pattern that cannot
  match your own command, or launch into a fresh shell.
- Opening the panel repeatedly for different files piles up editor groups until each is ~150 px wide.
  Close spare groups, and if the diagram and its editor end up in one group use Command Palette
  **"View: Move Editor into Previous/Next Group"** to get them side by side.

### The webview's `message` origin guard (`src/webview/diagram.ts`)

The handler drops events whose `event.origin !== window.origin`. Measured on VS Code desktop 1.134.0
(Linux, `--no-sandbox --disable-gpu`), every extension message satisfies it: the inner frame's
`window.origin` is `vscode-webview://<uuid>` (its `location.href` is
`vscode-webview://<uuid>/index.html?id=…&parentOrigin=vscode-file%3A%2F%2Fvscode-app`) and `views`,
`render` and `highlight` all arrive with exactly that origin — no `"null"`/srcdoc origin. If a future
VS Code version changes the webview frame plumbing, the guard would silently swallow everything;
the symptoms are an **empty View dropdown and an empty status line** in a panel that otherwise loads.
Measure it, don't guess, in Command Palette → **Developer: Open Webview Developer Tools** with the
console context set to `active-frame (index.html)`:

```js
window.__seen=[];
window.addEventListener('message',e=>window.__seen.push(
  {eventOrigin:e.origin,windowOrigin:window.origin,type:e.data&&e.data.type,
   passesGuard:e.origin===window.origin}));
// then change the View picker in the panel, and:
console.log(JSON.stringify(window.__seen,null,1))
```

Pitfalls: the devtools window is a *separate* X window with an empty title — re-running
"Open Webview Developer Tools" toggles/raises it unpredictably, so bring it back with
`wmctrl -l` + `wmctrl -ia <id>` instead. The console context and any listener you installed survive
raising the window, but not a `Developer: Reload Window`.

Cheap message-flow oracles that need no devtools (all three strings are produced only inside the
guarded handler):
- the picker filling with 13 entries for `examples/views-demo.sysml` (7 declared + 6 pseudo-views);
- the status line `<path>: declares 7 views (…); name the one to render` — that text comes from
  `internal/workspace/model/render.go`, i.e. a server error relayed as a `{type:"error"}` message;
- opened from a *scratch* folder that file has **0 Problems** (the ~13 problems in the skill above are
  the repo-root `package Views` shadowing), so use a deliberate error such as
  `port broken : NoSuchPort;` (expect `unresolved reference: NoSuchPort`) as the LSP smoke oracle
  rather than a non-zero problem count.

## Hover presentation testing (`internal/frontend/lsp/hover.go`)

VS Code advertises `hover.contentFormat: ["markdown", ...]`, so the GUI always exercises the
Markdown branch (fenced ```sysml block + prose). The plain-text branch is only reachable from a
probe that advertises `["plaintext"]` — test it there, not in the editor.

- A compact fixture that covers signature text, multi-comment prose and hard breaks in one file:
  ```
  package Demo {
      /* A wheel carries the load. */
      /* Second comment about the wheel. */
      part def Wheel;
      doc /*
       * First line.
       * Second line.
       */
      part def Vehicle { part w : Wheel; }
      part v : Vehicle;
  }
  ```
  Expected on a server with `Symbol.Notation()` + per-comment stripping: `part def Wheel` /
  `part def Vehicle` / `part w` in the fence (a pre-#676 server writes `partDef`/`partUsage`),
  two comments as two paragraphs, and `First line.` / `Second line.` on separate rendered lines.
- **Markdown hides delimiter bugs.** Leaked `*/` `/*` between two joined comments renders as
  `... load. //Second comment ...` in the popup, not as literal `/*` — read the rendered text
  carefully (or diff against a probe) rather than looking only for asterisks.
- The fence colouring itself is a free oracle: `part def Wheel` gets keyword colours, while an
  invalid signature like `partDef Wheel` renders as one plain identifier.
- Hover popups are sticky: `mouse_move` to an empty area, wait ~2 s, then move onto the target, or
  you will screenshot the previous symbol's popup and think the hover is wrong.
- Completion `detail` comes from the same `Notation()` (`internal/frontend/lsp/completion.go`), so
  `Wheel → part def` / `w → part : Wheel` in the detail column is the cheap second surface.
  Note the completion **documentation** panel still shows the raw comment text with `/*` `*/`
  (`symbolDocumentation` does no stripping) — that is unrelated to a hover fix, do not report it as
  a regression without checking a main build.
- The best negative control is one Settings-UI edit, no window reload: `Ctrl+,` → search
  `opensysml.server.path` → point at a binary built from `origin/main` in a worktree, Enter, confirm
  the swap with `pgrep -af sysml-lsp`, re-hover, then set it back. **Pitfall:** reopening Settings
  lands on "Commonly Used" with an empty search box, so a blind `triple_click` at the old field
  position edits `editor.fontSize` instead — always retype the search query first.

## Recording tips

Record the VS Code window maximized (wmctrl above). Verify visual claims by `zoom`ing the status bar
(language indicator "SysML v2"/"KerML", problem counts) and the completion popup — the popup's detail
column is too small to read in a 1024x768 full screenshot.

## Document-query authoring / `opensysml/renderDocument` (`internal/frontend/lsp/document.go`, `editors/vscode/src/document.ts`)

- The fixture in `internal/frontend/lsp/document_test.go` (`package Observatory` with `DocumentQueries::*`,
  `KerML::Root::Element`, `Subsystems`/`SubsystemTable :> Query`, `MassReport :> Document`) works
  verbatim in a scratch workspace with 0 Problems — copy it and every expected value (documents list
  `Observatory::MassReport`, markdown `# Telescope Mass Report` + `| optics | 8.5 |`, error
  `... is not a document: ...`) is confirmable first with a stdio JSON-RPC probe
  (`opensysml/documents` params `{}`, `opensysml/renderDocument` params `{"name": ...}`; capability
  `capabilities.experimental.openSysmlRenderDocument === true`).
- The command is Command Palette → **SysML: Render Document**; it opens an untitled Markdown doc
  beside plus its preview. On a broken document the error is a bottom-right toast, truncated —
  hover the toast to read the full message in a tooltip before judging it.
- Good live-diagnostic oracle: `Ctrl+Shift+K` on the `attribute redefines title = ...;` line ⇒
  Problems shows `document-plan(document-plan-missing-title)` at error severity spanning the
  `part def MassReport` line; File: Revert File clears it. Deleting the binding name `root` while
  testing completion also raises a `document-plan` missing-binding diagnostic live — expected, not
  a mess-up.
- Completion checks: double-click the type name after `calc rows : `, Delete, `Ctrl+Space`, type a
  prefix (`Subsy`) — expect only the query defs (`Subsystems`, `SubsystemTable`), not the part def
  `Subsystem`. In binding-name position (delete `root` before `= telescope`) the list is exactly
  one item `root` with detail `attribute : Element`.

## References / rename latency testing on a large workspace (`internal/workspace/model/refindex.go`)

- The training corpus `examples/sysml-v2-training` (100 files, fetch with
  `./scripts/download-training-examples.sh`) is a ready-made large workspace. Open that *folder*; it has
  no `bin/`, so set `opensysml.server.path` to `<repo>/bin/sysml-lsp` in User settings first.
  A good cross-file fixture: `part def Vehicle` in "02. Part Definitions/Part Definition Example.sysml"
  is used by `Vehicle_1 :> Vehicle` in both "28. Individuals/Individuals and Roles-1.sysml" and
  "…Individuals and Snapshots Example.sysml" (3 locations). "01. Packages/Package Example.sysml" has
  `alias Car for Automobile` with no uses — add `part c : Car;` (unsaved) to exercise alias identity.
- **On-screen timing evidence without instrumentation:** set `"opensysml.trace.server": "messages"`
  (needs `Developer: Reload Window`) and open the "SysML v2" Output channel. vscode-languageclient then
  logs `Received response 'textDocument/references - (N)' in X ms.` for every request, which is far
  more convincing than eyeballing. Expected on that corpus: index build ~50–60 ms on the first query
  after any edit, ~1–2 ms warm; a main-built server (pre reverse index) takes ~1.4 s *every* time.
- Do not type into the Output panel's filter box to isolate lines — it hides most of the trace and
  only shows some later lines; clear it and read the tail (`ctrl+End` inside the panel) instead.
- Rename preview is **Ctrl+Enter** in the rename box (the box's own hint says so); Shift+Enter does
  nothing. The Refactor Preview panel lists edits grouped by file, so "3 edits in 3 files" is readable.
- Find All References on a name that has become **unresolved** (you renamed its declaration in another
  unsaved buffer) does not return "no results": `referenceTarget` falls back to the enclosing
  declaration (e.g. `Vehicle_1`) and lists *its* references. That is pre-existing behaviour, not an
  index-staleness bug — the staleness oracle is "the old declaration file is absent from the list",
  plus re-querying after making the use current (`:> Vehicle2`) returns decl + use.
- Switching `opensysml.server.path` in the Settings UI restarts the server in place (request ids in the
  trace reset to 1, `pgrep` pid changes) — no window reload needed for a main-vs-branch comparison.
- `make vscode-package` runs `npm ci` first and takes several minutes; when backgrounded, wait for
  the ` DONE  Packaged: opensysml-sysml.vsix` line before `ls editors/vscode/*.vsix`, or you will
  conclude the build failed while it is still installing node modules.

## Diagram authoring (`opensysml/applyModelEdit`)

- Use a small interconnection fixture with **explicit private imports** and expose the owner,
  not only its children:
  ```sysml
  package Vehicle {
      private import StandardViewDefinitions::*;
      private import Views::*;
      port def FuelPort;
      part def Tank { port fuelOut : FuelPort; }
      part def Engine { port fuelIn : FuelPort; }
      part def Car {
          part tank : Tank;
          part engine : Engine;
      }
      view carView : GeneralView {
          expose Car;
          render asInterconnectionDiagram;
      }
  }
  ```
  Without the imports the view names are unresolved; imports without visibility produce
  diagnostics. `expose Car::*` renders tank/engine as separate roots, leaving no shared
  rendered owner for a connection. `expose Car` retains the Car node and its children.
- Keep the Problems panel visible: this fixture should start and remain at zero. An
  empty Problems panel alone is not sufficient; confirm the diagram and live server too.
- Palette **Add part…** uses the source cursor owner. Put the cursor on the Car declaration
  after its indentation, then verify the prompt title says `Add part to Vehicle::Car`
  before typing a name. Part prompts for a type; connection prompts allow an empty name.
- The custom context menu labels are **Connection from here…**, **Rename…**, **Delete…**.
  Connection target quick-pick also matches type details: typing `engine` can match both
  `engine : Engine` and `battery : Engine`, so explicitly choose the intended label.
- All Delete actions first show a native confirmation dialog. Referenced deletion then
  shows a second dialog with **Delete all**. Unreferenced deletion should not show the
  second cascade dialog. Canceling the first dialog does not exercise server refusal.
- Focus the source editor before Ctrl+Z / Ctrl+Y. One undo should remove one diagram
  operation (e.g. connection), the next the preceding operation (e.g. added part); each
  diagram update should arrive automatically, without Refresh.
- For duplicate refusal, use the current name after any rename. Expect a bottom-right
  toast such as `Vehicle::Car already declares "motor"` and `Model edit refused:` in
  **Output: Show Output Channels… → SysML v2**. The displayed message need not include
  the protocol's failure-code spelling.
- Table rendering is non-SVG, but may still receive a full member/connection palette.
  Check the current palette table in `internal/frontend/lsp/render.go` rather than
  assuming non-SVG means authoring is hidden.

## Edit-latency and semantic-token comparisons

- Use the same scratch workspace and isolated keystrokes for both server builds.
  Keep ambiguous wildcard imports at document level as well as inside a package;
  include a resolved root wildcard import so there are actual root re-exports.
  Start from the current index invalidation reproducer rather than assuming a
  nested-only import triggers document-root invalidation. Small fixtures may
  still not reproduce the reported multi-second stall; report this limitation.
- Measure `didChange` to `publishDiagnostics` with a transparent framed stdio
  relay or timestamped client trace, keeping payloads unchanged. Distinguish
  server transport latency from visible screen repaint and editor debounce.
  For publications without versions, isolate changes with settled intervals;
  do not attribute a queued older publication to the newest edit.
- A relay must forward/handle EOF and termination so changing the server setting
  does not leave orphan servers. Check the actual child executable, not merely
  the setting value. Separate logs for each server/workload.
- Disable word-based suggestions to prove newly declared names come from the
  LSP, but leave ordinary quick suggestions enabled when checking popup behavior.
  Type a prefix with the list open to prove it does not block editing.
- Inspect the reference with **Developer: Inspect Editor Tokens and Colors**
  after renaming and shifting source lines. Check exact identifier length and
  semantic token type; matching syntax colour alone does not prove a semantic
  token arrived. A finite GUI sequence cannot exclude every scheduling race.
- Show the normal hover before typing with it open. A disappearing hover and
  inserted text prove input was not blocked; the diagnostic squiggle and Problems
  entry are not modal notifications.

## Devin Secrets Needed

None.
