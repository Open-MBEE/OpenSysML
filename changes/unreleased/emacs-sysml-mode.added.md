- **An Emacs package** (`editors/emacs`): `sysml-mode` and `kerml-mode` highlight every
  reserved word by the group it belongs to, each with its own customizable face, along with
  declared names, type references, qualified names, quoted names and all three comment forms;
  indentation follows the formatter — one level per brace and one more for a line continuing
  the statement above — checked against the formatter's own golden fixture. An lsp-mode
  client for `sysml-lsp` finds the server as the VS Code extension does, sends
  `sysml.strictConformance`, and layers the server's semantic tokens over the keyword faces
  without flattening them. `M-x sysml-diagram-show` shows a view of the model as a buffer: a
  declared view or a pseudo-view, links to each element's declaration, the element at point
  highlighted, redrawn when the server reports a change, and the last good view kept when a
  render fails; with the Mermaid CLI installed it also draws a picture. The keyword lists are
  generated from the lexer by `make emacs-keywords`, grouped by the new
  `editors/internal/keywords` package the VS Code and JupyterLab grammars are generated from
  too, and `go test ./editors/...` fails when they are stale.
- **`library` no longer counts as introducing a declaration name** in the generated editor
  grammars: in `standard library package Lib`, it is `Lib` that is highlighted as the name,
  not `package`.
