- **`ApplyEdits` edits a model of several documents as one atomic batch.** A model parsed by
  `ParseSources` was refused with `FAILED_PRECONDITION`; its operations are now applied through
  the same cross-document path the LSP uses: a rename or cascade delete follows its references
  into the model's other documents, every document touched is re-parsed and re-analysed together,
  and either all of them are answered or none is. `ApplyEditsResponse.documents` (new field 7)
  lists every document the batch rewrote — one entry for a model of one document — as
  `EditedDocument{name, content}`, `name` being the name the parse request gave it, so a new
  client has one code path for both shapes; `content` (field 1) keeps the edited notation of a
  model of exactly one document and is empty for a model of several, even when only one changed.
  Each `AppliedEdit` names its `document` (new field 7), and a refusal — which still carries no
  content — names each referrer with its document in `referrers` (new field 8, `Referrer{name,
  document}`) beside the textual `referring_elements`. `ApplyEditsRequest.document` (new field 3)
  selects a document other than the model's first for the operations to target; a name that is
  not one of the model's is `INVALID_ARGUMENT`. `EDIT_FAILURE_REFERENCED_ELSEWHERE` is appended
  for a rename, delete or move referred to from a document the edit cannot rewrite — a move
  respells references in its own document only, so one referred to from another document of the
  model is refused this way. No existing field changed number, type or meaning, so a generated
  client of the previous schema decodes every answer. `Convert` from a model handle still
  requires a model of one document. The Go client answers `EditResult.Documents`,
  `AppliedEdit.Document` and `EditError.Referrers`, and `ApplyDocumentEdits` names the document
  to edit; the Python client answers `EditResult.documents`, `AppliedEdit.document`,
  `EditError.referrers` and raises `ReferencedElsewhereError`; the Node, Java and Rust clients
  carry the regenerated messages. The conformance suite parses a model of several documents by
  naming `fixtures` rather than one `fixture`, and gains scenarios for a rename and a cascade
  delete crossing documents, an edit answering only the document it touched, and a refusal
  naming a referrer in another document.
