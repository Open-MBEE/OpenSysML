- Expanding a model's wildcard imports of large library packages (`import ISQ::*`,
  `import SI::*`) is about 30% faster and allocates about a quarter less: the symbol
  index keeps its re-export and hidden marks and its per-segment name table as small
  sorted slices rather than one map per name, reuses a re-export claim's writable
  record instead of looking it up again, passes an unfiltered import's inherited routes
  on without copying them, and no longer notes a parent namespace's change twice per
  re-exported member. Semantics are unchanged; the embedded standard-library snapshot
  is regenerated for the new table layout. `BenchmarkExpandModelImports` in
  `internal/core/libs` measures the cost.
