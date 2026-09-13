- **A rendering quotes a name holding `::` whole.** A view drew a declaration whose unrestricted
  name holds the qualified-name separator by splitting the joined qualified name at `::`: `port
  'fuel::out'` was labeled `'out'` and a top-level `part 'x::y'` was written `x::y`, as if it were
  `y` in `x`. The tree, interconnection, sequence and table renderings, the REPL's `%render` and
  `%view`, the `opensysml/render` LSP result and the VS Code diagram now spell each segment from the
  declaration's owner chain, so `'x::y'` and `'fuel::out'` are one name each, and a connection
  added from the diagram between such ports names the port that is there.
