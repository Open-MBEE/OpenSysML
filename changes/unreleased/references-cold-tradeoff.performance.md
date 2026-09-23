- **A references query made right after an edit is slower than in 0.8.1, in exchange for
  incremental invalidation.** The resolver now records which documents and names each
  resolution read, so an edit invalidates only what depended on it: a rename or an edit beside
  a large document is several times faster than before. The recording is paid on the first
  query after an edit that walks a long wildcard-import chain, where cold references measure
  about 40% slower on the LSP benchmark; the query itself returns the same locations.
