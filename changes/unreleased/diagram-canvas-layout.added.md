- **The VS Code diagram panel is an editable canvas, and a dragged node's position is written
  into the model.** The panel draws its own SVG from the server's rendering: a node the model
  places with a `DiagramLayout::Layout` annotation is drawn exactly there, at the size it
  states, every other node takes a deterministic slot in a grid under its owner, and an edge
  follows its `DiagramLayout::Route` waypoints. Dragging a node writes `metadata Layout about
  … { x = …; y = …; }` — into the view's body when a view is drawn, into the element's own
  when the document is — as one source-preserving edit of the file when the pointer is
  released, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> puts it back; an annotation already stated is
  updated in place, and the children the model places move with their owner. Dragging the
  handle on an edge bends it through a `Route` waypoint, dragging a waypoint moves it, and
  double-clicking one removes it. The palette, node menu, click-to-source and cursor
  highlight work on the canvas as before; a sequence diagram is drawn as lifelines and a
  table as Markdown, neither dragged. `SysML: Export Diagram` saves the Mermaid (or a table's
  Markdown) the server writes, positions included, to a file. Behind it, `opensysml/applyModelEdit`
  gains the `setLayout`, `setRoute` and `setCanvas` operations, which set, update or clear the
  three `DiagramLayout` annotations and refuse a target outside the document, a view that is
  none (`not-a-view`), a view-local placement of an element the view does not expose
  (`not-exposed`), an element no rendering draws (`not-drawn`) and a clearing with nothing to
  clear (`not-annotated`); a model without annotations keeps exactly its bytes. Not built:
  moving a node into another owner, placing an element in another document's view, and the
  `geometry` view kind.
