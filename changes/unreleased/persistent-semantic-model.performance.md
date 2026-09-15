- **A workspace keeps its semantic model between edits and invalidates it per document.**
  `model.Workspace` owns one `resolve.Resolver` and one `semantics.Model` for its lifetime and
  hands them to every analysis it runs; the resolver keeps a frame per document owning what was
  memoized while that document was analyzed and records which documents it read (a namespace it
  imports that another contributes to, a namespace both contribute to, a symbol of another that a
  resolution returned). Replacing a document drops its frame and, transitively, its dependents' —
  their memo entries, cached diagnostics and reverse references — and nothing else, where every
  edit used to clear the whole workspace. The OOSEM, MOSA and identity-metadata audits and the
  coherent-quantity ranking gather each document's facts once into the workspace and judge each
  analyzed document over the union, where they gathered every document once per document
  analyzed. `TestIncrementalEqualsFresh` replays scripted and random edit sequences over the
  fixtures and the OMG corpora and compares diagnostics, resolutions and references with a fresh
  workspace after every step. On the satellite-network stress test, editing a two-line file beside
  512 satellites goes from 861 ms and 327 MiB per edit to 8.7 ms and 2.0 MiB; editing the library
  every file of the split network imports costs one analysis of the model (8.8 s to 5.3 s at 512
  satellites), and loading the 1 600-satellite network split into 34 files through one workspace
  goes from 126 s to 18 s. A loaded workspace holds about twice the heap (254 MiB to 478 MiB at
  512 satellites), the memo tables that were allocated and discarded on every analysis, and a
  thousand edits grow it by 4.5%. A one-shot `sysml -validate` pays the dependency recording it
  never uses: about a sixth more wall time (1.9 s to 2.2 s at 200 satellites) and 4% more
  allocation. Figures and the machine they were taken on are in `docs/internals/performance.md`
  and `docs/project/satellite-network-stress-test.md`.
