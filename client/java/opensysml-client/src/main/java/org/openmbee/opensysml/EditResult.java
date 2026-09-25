package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * The edited notation a batch of edits produced, and what each operation changed.
 *
 * @param content the edited notation of a single-document model; empty for a model of several
 *     documents, whose edited notation is in {@code documents} alone
 * @param applied what each operation changed, grouped by document in the order {@code documents}
 *     lists them and in request order within a document
 * @param documents the edited notation of every document the edits rewrote, the edited document
 *     first, then the others in name order; a document of several the edits left as parsed is not
 *     listed
 * @param diagnostics what the service reported while editing
 */
public record EditResult(
    String content,
    List<AppliedEdit> applied,
    List<EditedDocument> documents,
    List<Diagnostic> diagnostics) {

  /**
   * Creates an edit result, copying its collections.
   *
   * @param content the edited notation of a single-document model, never {@code null}
   * @param applied the applied edits
   * @param documents the edited documents
   * @param diagnostics the diagnostics
   */
  public EditResult {
    Objects.requireNonNull(content, "content");
    applied = List.copyOf(applied);
    documents = List.copyOf(documents);
    diagnostics = List.copyOf(diagnostics);
  }
}
