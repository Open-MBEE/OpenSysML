- Interconnection renderings draw constraint usages and constraint defs as nodes, with their
  parameters — the `in`/`out`/`inout` attributes the constraint def declares and any the usage
  declares itself — as squares on the border labelled outside, the way a part's ports are drawn, so
  a SysML v1 parametric diagram migrated to a `Views::asInterconnectionDiagram` view shows its
  constraint properties with the bindings ending at their parameters instead of dropping each
  constraint with a "has no place in an interconnection rendering" notice and leaving the lines
  dangling. A `DiagramLayout::Layout` about a parameter (`about first::input` for one usage's,
  `about Law::input` for every usage's) places its square on the nearest side of the constraint's
  border, and the `port-symbols` notation element hides the squares with the ports' and attaches
  the bindings to the constraint itself. Mixed renderings draw constraints the same way.
- The SysML v1 migrator writes a connector's `DiagramLayout::Route` with its points run from the
  feature the connector's first written end names to its second's, reversing a line the diagram
  drew from the second end, so a rendering following the Route from `bind a = b`'s `a` follows the
  drawn line instead of collapsing it onto one end.
