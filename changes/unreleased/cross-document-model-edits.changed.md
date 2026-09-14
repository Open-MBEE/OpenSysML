- **A rename or delete from the diagram panel follows references into the other documents of
  the workspace.** `opensysml/applyModelEdit` answers a `WorkspaceEdit` with one versioned
  `TextDocumentEdit` per document it rewrites, the requested document first: a rename respells
  every reference that writes the name wherever the workspace declares it, the way the editor's
  rename does; a cascade delete removes the referring declarations in whichever documents make
  them, recursively; a delete without cascade is refused as `delete-referenced` naming the
  referrers, each with its document, in `referring` and the new `referrers`. Every document is
  snapshotted, rewritten and re-analyzed together under one lock, so a new error in any of them
  refuses the whole request. The `referenced-elsewhere` refusal now applies only to a reference
  the edit cannot follow: one from a bundled library file, or from a document the index holds
  without the workspace holding its source. The VS Code extension applies the edit as one
  `WorkspaceEdit`, so one <kbd>Ctrl</kbd>+<kbd>Z</kbd> reverts every file, lists the referrers
  of a refused delete by file, opens a file the server read from disk before editing it so the
  edit lands on a versioned buffer, and leaves an edit unapplied when another document it names
  was typed into while it was computed. An edit within one document is unchanged.
