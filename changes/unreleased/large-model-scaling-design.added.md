- **A design for scaling to very large models** (`docs/project/large-model-scaling-design.md`).
  Starting from the satellite-network stress test's profiles, it separates the four costs a large
  model pays — per element once, per workspace per edit, per process on one core, and per modeled
  object by construction — and designs one approach against each: a persistent resolver and
  semantic model owned by the workspace and invalidated through a document dependency relation
  rather than cleared on every change; closed documents held as interface records (the facts other
  documents can observe, plus stored diagnostics) that hydrate to a full tree only when opened or
  queried, generalizing the standard library's snapshot and index-record cache; parallel
  per-document analysis over a read-only index; and one definition with many occurrences in the
  runtime, with sparse per-occurrence values. Each names its differential test against the
  unoptimized path, the measurement that decides it, and its place in the sequence. Nothing is
  implemented; the page exists to be reviewed before code is written.
