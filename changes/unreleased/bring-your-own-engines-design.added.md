- **A design note for bringing your own engine**
  (`docs/internals/design/bring-your-own-engines.md`). It proposes how a user adds to the analysis
  framework without changing OpenSysML: an engine of their own that answers questions about a
  model, a scheduling policy or sampler of their own inside a built-in engine, or a tool of their
  own, registered from a manifest the environment names and run as a process speaking a JSON-RPC
  message set on standard input, as a WebAssembly module, or as Go over the public
  `client/opensysml` package. Every external witness is checked by the interpreter before it
  counts — a schedule replayed, an assignment evaluated; an external universal claim is
  *observed* only over executions the interpreter replayed and is otherwise *not covered* with
  the claim kept, until the site admits a strength against a referee record earned on a corpus
  under `-engine all`; and nothing a model, a workspace or a remote client can cause an
  executable to run. Nothing is implemented; the note exists to
  be reviewed before code is written.
