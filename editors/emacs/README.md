# SysML v2 for Emacs (OpenSysML)

Major modes for `.sysml` and `.kerml` files, with highlighting generated from
OpenSysML's lexer, indentation that agrees with the language server's formatter,
an lsp-mode client for `sysml-lsp`, and a buffer showing views of the model.

| File | What it gives you | Needs |
| --- | --- | --- |
| `sysml-mode.el` | `sysml-mode` and `kerml-mode`: highlighting, indentation, comments, imenu | Emacs 29.1+ |
| `sysml-lsp.el` | Diagnostics, hover, go-to-definition, references, symbols, completion, formatting, rename, quick fixes, semantic tokens | [lsp-mode](https://emacs-lsp.github.io/lsp-mode/) 9.0+ and `sysml-lsp` |
| `sysml-diagram.el` | `M-x sysml-diagram-show`: a view of the model, redrawn as you type | `sysml-lsp.el`; [Mermaid CLI](https://github.com/mermaid-js/mermaid-cli) for pictures |

The package is installed from this repository. Like the VS Code extension, it is
deliberately not published to MELPA.

## Install

Build the server once (the modes work without it, highlighting only):

```bash
make build    # from the repo root: writes bin/sysml-lsp
```

Then, with `use-package` (adjust the path to your checkout):

```elisp
(use-package sysml-mode
  :load-path "~/src/OpenSysML/editors/emacs"
  :mode (("\\.sysml\\'" . sysml-mode)
         ("\\.kerml\\'" . kerml-mode)))

(use-package sysml-lsp
  :load-path "~/src/OpenSysML/editors/emacs"
  :after lsp-mode
  :demand t
  :hook ((sysml-mode kerml-mode) . lsp-deferred))

(use-package sysml-diagram
  :load-path "~/src/OpenSysML/editors/emacs"
  :after sysml-lsp
  :bind (:map sysml-base-mode-map ("C-c C-v" . sysml-diagram-show)))
```

If your `use-package` installs packages by default (`use-package-always-ensure`,
or elpaca's `elpaca-use-package-mode`), add `:ensure nil` to each block: they are
loaded from the checkout, not installed.

Without `use-package`:

```elisp
(add-to-list 'load-path "~/src/OpenSysML/editors/emacs")
(require 'sysml-mode)
(with-eval-after-load 'lsp-mode
  (require 'sysml-lsp)
  (require 'sysml-diagram))
(add-hook 'sysml-base-mode-hook #'lsp-deferred)
```

The server is found in this order, as the VS Code extension finds it:

1. `sysml-lsp-server-path`, if set;
2. `bin/sysml-lsp` in the project root (a checkout that ran `make build`);
3. `sysml-lsp` on `exec-path`.

After rebuilding the server, `M-x lsp-workspace-restart` picks the new binary up.

## Code lenses

A server that offers code lenses (`sysml-lsp` 0.10 and later) shows a reference
count on each definition and **Run**/**Evaluate** on each executable action, state,
calc, constraint and requirement, when `lsp-lens-enable` is on. Both are client
commands, carried out here rather than sent back to the server:

- **N references** lists the references through xref (`consult-xref` if you use
  it).
- **Run X** / **Evaluate X** saves the model buffers and runs the matching
  `sysml -action`/`-state`/`-calc`/`-constraint`/`-requirement` check over every
  workspace folder of the session (the file alone when none holds it), in a
  compilation buffer. Nothing runs while a model buffer is left unsaved. `sysml` is looked for beside the server, then
  in the project's `bin/`, then on `exec-path`.

## Highlighting

Every reserved word is highlighted, grouped by what it introduces. Each group has
its own face, inheriting a standard one, so a theme colours it without
configuration and `M-x customize-group RET sysml-faces` recolours one group alone:

| Face | Words |
| --- | --- |
| `sysml-declaration-keyword-face` | `part`, `def`, `action`, `package`, … |
| `sysml-control-keyword-face` | `then`, `accept`, `transition`, `if`, … |
| `sysml-modifier-keyword-face` | `abstract`, `in`, `private`, `ref`, … |
| `sysml-relationship-keyword-face` | `specializes`, `import`, `redefines`, … |
| `sysml-operator-keyword-face` | `and`, `implies`, `istype`, … |
| `sysml-constant-face` | `true`, `false`, `null` |
| `sysml-other-keyword-face` | reserved words outside those groups |
| `sysml-contextual-keyword-face` | words that are syntax in one place only: `chain`, `done`, `var` (KerML), … |
| `sysml-declaration-name-face` | the name after `def`, `package`, `namespace` or `alias` |
| `sysml-type-reference-face` | the type after `:`, `:>`, `:>>` or `::>` |
| `sysml-namespace-face` | `ScalarValues` in `ScalarValues::Real` |
| `sysml-quoted-name-face` | `'left wheel'` |

`//` notes, `//* */` multi-line notes and `/* */` comments are comments; strings
and quoted names are never read as code.

With the server attached, `sysml-lsp` adds its semantic tokens, which colour
names by what they resolve to: a feature's name differently from a definition's.
The server's keyword, comment, string and number tokens are left out, so the
keyword groups above keep their faces. Turn the tokens off with
`(setq sysml-lsp-semantic-tokens nil)`.

## Indentation

`TAB` and `indent-region` indent as the server's formatter does: one
`sysml-indent-offset` (default 4) per open brace, and one more for a line that
continues the statement above it — a line starting with an infix operator, or
following one or an open bracket. The text of a multi-line comment is left as
written. `M-x lsp-format-buffer` asks the server to reformat the whole file with
the same rule.

## Model views

`M-x sysml-diagram-show` shows the model in a buffer beside it. It draws the same
renderings the REPL's `%view` prints and the VS Code diagram panel shows:

- **What it shows.** The one view the document declares, or the document itself
  as a model tree. `C-u M-x sysml-diagram-show`, or `v` in the view buffer, picks
  another: a declared view, or the document as a tree, interconnection diagram,
  state machine, action flow, table or matrix. A view whose rendering is not supported
  (`sequence`, `geometry`, `textual`) is listed with the reason and refused.
- **Navigation.** `RET` or a click on an element visits its declaration, with its
  name selected. Moving point in the model highlights the innermost element
  declared around it. A standard library declaration opens read-only, its text
  fetched from the server (`opensysml/stdlibContent`). An element with no
  locatable declaration is not a link.
- **While typing.** The buffer redraws when the server reports the document
  changed, and only while it is visible. A rendering that fails keeps the last
  good one on screen, dimmed, with the error in the header line.
- **Pictures.** `i` draws the view with the Mermaid CLI (`mmdc`, installed with
  `npm install -g @mermaid-js/mermaid-cli`) and shows the SVG, in a graphical
  frame. A table or matrix is not drawn: it is already shown as one.

| Key | Command |
| --- | --- |
| `RET` | Visit the element's declaration |
| `n` / `p` | Next / previous element |
| `v` | Pick another view |
| `t` | Toggle the server's plain-text rendering |
| `i` | Draw it as a picture with `mmdc` |
| `g` | Render again |
| `q` | Quit the window |

The buffer is read-only: it renders the model, and does not edit it. The command
is refused when the server does not advertise the render methods
([LSP extensions](../../docs/reference/lsp.md)).

## Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `sysml-indent-offset` | `4` | Columns per nesting level, for indentation and formatting. |
| `sysml-lsp-server-path` | `nil` | Absolute path to `sysml-lsp`; `nil` falls back to the project build, then `exec-path`. |
| `sysml-lsp-server-args` | `nil` | Extra server arguments, e.g. `("-strict")`. |
| `sysml-lsp-strict-conformance` | `nil` | Report notation only OpenSysML accepts as errors. Takes effect at once, without a restart. |
| `sysml-lsp-semantic-tokens` | `t` | Add the server's semantic tokens to highlighting. |
| `sysml-diagram-mmdc-program` | `"mmdc"` | The Mermaid CLI pictures are drawn with. |
| `sysml-diagram-display-action` | right side window | How the view buffer is displayed. |

## Keyword generation

`sysml-mode-keywords.el` is generated — do not edit it by hand. Its lists come
from `internal/syntax/source.Keywords()` and `lexer.ContextualWords()`, grouped by
`editors/internal/keywords`, the same grouping the VS Code grammars are generated
from, so the two editors cannot disagree:

```bash
make emacs-keywords    # regenerate
go test ./editors/...  # fails if the committed file is stale
```

## Development

```bash
make emacs-compile   # byte-compile and checkdoc, warnings as errors
make emacs-test      # the ERT suites; the live ones need `make build` first
```

Both run under `emacs -Q`, installing lsp-mode from MELPA into `build/emacs-deps`
the first time, so your own configuration plays no part. The live suites start
`bin/sysml-lsp`; without it they skip, unless `OPENSYSML_REQUIRE_EMACS_LSP` is
set, as CI sets it. The indentation suite indents the formatter's own fixture,
`internal/syntax/format/testdata/continuation.sysml`, and compares it with the
formatter's golden.
