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
- **A document renders its diagrams as DOT on request.** `sysml -render-document … -diagram-form
  dot` (and `-render-documents`), `%render-document <name> dot` at the prompt, and
  `"diagramForm": "dot"` on `opensysml/renderDocument` write every graph-shaped `Diagram` block
  of the document as a fenced ` ```dot ` block in Markdown and as `<pre class="dot">` in HTML;
  the PDF backend keeps the source under a notice that it does not draw DOT, looking for no
  Graphviz tool. Mermaid stays the default, and a table-kind block is a table either way. The
  form is a choice of the render, not of the model: a `Diagram` block states what is drawn, and
  no attribute names the notation. An unknown form, or `dot` on a document holding a `sequence`
  diagram, is a typed error.
- **The DOT form draws a view where the model places it.** A rendering's `DiagramLayout` geometry
  is written as Graphviz reads it: a positioned node is pinned at the centre of its box
  (`pos="x,y!"`, `pin=true`, one pixel to one point with y measured up from the canvas's bottom
  edge), a stated size is `width`/`height` in inches with `fixedsize=true` and an unstated one is
  fitted to the label so the box's corner stays put, a collapsed node keeps `comment="collapsed"`,
  a positioned cluster states its `bb` and pins its anchor at the centre, a `Route` is the edge's
  `pos` spline through its waypoints (a route of one waypoint is noticed, not drawn), and a sized
  `Canvas` is the graph's `size` and a
  `// canvas:` header line. The `// layout:` header names the engine that honours the file:
  `neato -n2` when every node is placed and every edge routed, `neato -n` when every node is
  placed, `neato` when some are, `dot` when none — so `neato -n -Tsvg view.dot` draws the view
  as laid out; a route an engine short of `neato -n2` redraws is noticed. A model without layout annotations
  writes the same DOT as before, and no Graphviz binary is run to produce it.
