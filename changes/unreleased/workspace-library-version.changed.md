- **A workspace copy of a library file rooted at the library's packages is the library, as it
  is for the RDF mapping.** The language server treated such a copy as the user's file — derived
  ids, a minting action on every declaration — where `sysml -convert` recognised the same bytes
  as the bundled file. The workspace now applies the one recognition
  (`identity.Catalog.DocumentRootedAt`, moved out of `internal/core/export`): a document whose
  every root is a top-level package of one bundled library file, stating that package's
  normative id or declared as the library declares it, stands in for the bundled file. Its
  declarations are what the library's names resolve to, its elements keep their normative ids
  (hover states `(normative, KerML)`) and get no minting action, and it is not warned for its
  `standard library` keyword. Editing a root so it no longer qualifies, or closing a version
  whose on-disk text is the user's, puts the bundled file back. The library identity the
  runtime names library types by (`symbols.Index.LibraryIdentity`) digests each library
  document's tier and text, no longer its name, so an unchanged version standing in leaves
  it — and the objects carried across a re-analysis — as they were.
- **A file opened under a bundled library's own name no longer removes that library from the
  workspace when it is closed.** Closing it put nothing back, so every later document was checked
  against a library missing that file; the standard-library expression gate
  (`TestExprTypeCheckNoStdlibFalsePositives`) passed on an incomplete library for that reason.
  Over the whole library it now reports nine dimension defects in the published `SI.sysml` and
  `USCustomaryUnits.sysml`, pinned as an exact set and recorded in `docs/project/omg-issues.md`
  ("Defects in the vendored quantity libraries"); the library bytes are unchanged.
