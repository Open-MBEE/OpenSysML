- **D2 is a rendering form.** `-render-form d2`, `%render <view> d2 [palette]`, `"form": "d2"` on
  `opensysml/render` and VS Code's *Export Diagram* write a tree, interconnection, state, action or
  sequence rendering as a [D2](https://d2lang.com) diagram in the Pilot visualizer's B&W look —
  nested containers for a part's parts and its drawn ports, pseudostate glyphs for control nodes,
  D2's `sequence_diagram` for a sequence, the named palettes as fills, `-render-link` templates as
  `link:` attributes — and `-render-all` writes `.d2` files. Documents take `-diagram-form d2` (`%render-document <name> d2`, `diagramForm:
  "d2"`): a ` ```d2 ` fence in Markdown, `<pre class="d2">` in HTML, and in a PDF the figure drawn
  by the `d2` executable that `OPENSYSML_D2` names (or `d2` on `PATH`), kept as source under a
  notice without one. Matrix renderings remain tabular and refuse D2, like tables.
  `scripts/download-doc-pdf-toolchain.sh` provisions a pinned D2.
