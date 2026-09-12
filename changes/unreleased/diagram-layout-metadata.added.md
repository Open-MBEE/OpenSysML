- **A model can say where a view draws its elements, and every rendering carries it.** The
  bundled `DiagramLayout` library declares `Layout` (`x`, `y`, optional `width`, `height`,
  `collapsed`), `Route` (an edge's waypoints, flattened `x0, y0, x1, y1, …`) and `Canvas` (a view's
  `unit`, `width`, `height`), in pixels from the top-left corner. A `metadata Layout about <element>
  { … }` stated in a view's body places the element in that view; an `@Layout { … }` inside the
  element's own body is what every view that does not place it falls back to. The geometry rides
  the rendering tree (`Node.Geometry`, `Edge.Route`, `Rendering.Canvas`) in the tree,
  interconnection, state and action renderings; Mermaid keeps it as `%% layout:`, `%% route:` and
  `%% canvas:` comments after the header, the text form appends `at (x, y)`, `size w×h`, `collapsed`
  and `via (x, y) …`, and `opensysml/render` adds optional `x`, `y`, `width`, `height`, `collapsed`,
  `route` and `canvas` fields. A model without layout annotations renders byte-for-byte as before.
- **Validation of layout annotations.** `-validate` and the REPL warn about a `Layout` or `Route`
  on an element the rendering draws no node or edge for, and about a second position for one
  element in one view (the first applies); a `Route` with an odd number of values, a non-constant
  binding, a `Canvas` binding only one of `width` and `height`, and a `Canvas` stated outside the
  body of the view it sizes are errors.
