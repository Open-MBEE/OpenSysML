- **A library copy whose root package states only a short name is read as the library.** The
  notation-side library check looked the root up by its long name alone, so a copy opening with
  `standard library package <Occurrences> {` fell through to user-document analysis and its
  elements took derived ids. The check now uses the name the symbol table registers the package
  under — the long name, else the short name — as the graph-side check already did.
