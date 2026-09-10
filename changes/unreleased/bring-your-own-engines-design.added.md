- **A design note for bringing your own engine**
  (`docs/internals/design/bring-your-own-engines.md`). It proposes how a user adds to the analysis
  framework without changing OpenSysML: an engine of their own that answers questions about a
  model, a scheduling policy or sampler of their own inside a built-in engine, or a tool of their
  own, registered from a manifest the environment names and run as a process speaking a JSON-RPC
  message set on standard input, as a WebAssembly module, or as Go over the public
  `client/opensysml` package. Every external witness replays through the interpreter before it
  counts; an external universal claim is printed *observed* until the site admits a higher
  strength against a referee record earned on a corpus under `-engine all`; and nothing a model or
  a workspace contains can cause an executable to run. Nothing is implemented; the note exists to
  be reviewed before code is written.
