- **A diagram drawn from an older `sysml-lsp` no longer labels a typed usage `undefined`.** A
  server predating the node `type` field in `opensysml/render` sends no such field, and the
  VS Code panel wrote the missing value into the label as `engine : undefined`. The extension
  now fills in what an older server omits — a node's `type`, `name` and `detail`, an edge's
  `label`, absent lists — before drawing, so such a usage reads `engine`, «part», `Engine`, as
  that server's own client drew it, and an unnamed element leads with its kind alone.
