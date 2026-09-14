- **A design note on surface parity** (`docs/internals/design/api-surface-parity.md`). It
  inventories what the REPL, the CLI, the editor, the public Go package and the wire each expose,
  sorts every difference as shared already, missing and worth adding, interactive, protocol-bound
  or local-only, and stages the work: one assembly per operation that all four surfaces call; the
  stateless operations the wire lacks (satisfiability, model-checker options, inline replay, view
  rendering, library search, codegen and the XMI migration report through `Convert`); a session
  API for the action and state debuggers designed apart from the stateless calls; and a `repl`
  protocol in the conformance runner so the agreement is tested rather than claimed. The binaries
  keep calling the engine in-process; nothing is implemented, the note exists to be reviewed
  before code is written.
