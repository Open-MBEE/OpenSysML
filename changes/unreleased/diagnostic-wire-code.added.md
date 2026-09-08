- **A wire `Diagnostic` carries its `code`.** The gRPC/Connect `Diagnostic` message gains
  `string code = 4`, the stable identifier the runtime already assigns: `syntax` for a parse
  error, the pass or rule code for a validation finding, `choice-point` and `guard-unevaluable`
  for a run's notes. Every response that carries diagnostics carries it, so a client branches on
  `code` instead of a message prefix; a diagnostic whose producer assigned no code sends it empty.
  A service that populates it advertises the `diagnostic_codes` capability. The Go, Python,
  Node, Rust and Java clients expose it as `Diagnostic.code` and name the capability.
