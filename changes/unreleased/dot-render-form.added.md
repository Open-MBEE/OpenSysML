- **A view renders as Graphviz DOT with the new `dot` form.** `sysml -render <view> -render-form
  dot` (and `.dot` files under `-render-all -render-form dot`), `%render <view> dot` at the
  prompt, and `"form": "dot"` on the editor's `opensysml/render` request write a tree,
  interconnection, state or action rendering as a `digraph` for the `dot` engine — containment
  as `subgraph "cluster_…"`, the flow direction as `rankdir`, states as rounded boxes with
  `point`/`doublecircle` pseudo-states and trigger/guard/effect labels, and edges styled as the
  Mermaid form styles them — with `// view:`, `// kind:` and `// layout: dot` header comments and
  one `// not represented:` line per notice. Producing DOT needs no Graphviz installation; it is
  written for Graphviz toolchains and for layouts of graphs larger than Mermaid draws. Mermaid
  stays the default machine-readable form. A `sequence` or `table` view has no DOT form and is
  refused the way a wrong form always was.
- **A document's `Diagram` block states the form its source is written in.**
  `DocumentQueries::Diagram` gains an optional `form` attribute, `"mermaid"` (the default) or
  `"dot"`. With `"dot"` the Markdown backend writes a fenced ` ```dot ` block, the HTML backend
  embeds the source in `<pre class="dot">`, and the PDF backend keeps the source under a notice
  that it does not draw DOT, looking for no Graphviz tool; a table is written as a table whatever
  is stated. A form outside the two, or `"dot"` on a kind with no DOT form, is a planning error at
  the declaration.
