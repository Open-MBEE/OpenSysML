- **The `dot` form draws a diagram where the model says.** Positions the `DiagramLayout`
  library states are written as Graphviz geometry: a placed node is pinned at its centre
  (`pos="x,y!"`, `pin=true`, `width`/`height`/`fixedsize=true` when sized), a routed connection,
  flow, transition or succession carries its waypoints as the B-spline that draws the polyline
  exactly, a sized canvas is the graph's `bb`, and a collapsed node is written as a leaf. Pixels
  from the top-left corner become points from the bottom-left (`1 px = 0.75 pt`; a canvas in `pt`
  converts one to one). The `// layout:` header names the command that keeps the geometry —
  `neato -n` for node positions alone, `neato -n2` when an edge is routed too — and what DOT
  cannot carry (a canvas in another unit, a cluster without a size, a partly placed
  rendering) is a `// not represented:` notice. A rendering without geometry writes exactly the
  DOT it wrote before.
