package org.openmbee.opensysml;

import java.nio.file.Path;
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
 * @param severalDocuments whether the edited model has several documents, so {@code content} is
 *     empty and {@link #save(Path)} refuses
 */
public record EditResult(
    String content,
    List<AppliedEdit> applied,
    List<EditedDocument> documents,
    List<Diagnostic> diagnostics,
    boolean severalDocuments) {

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

  /**
   * Creates an edit result whose model's document count is not known, inferring it from the
   * response: several documents when {@code content} is empty while rewritten documents carry text.
   *
   * @param content the edited notation of a single-document model, never {@code null}
   * @param applied the applied edits
   * @param documents the edited documents
   * @param diagnostics the diagnostics
   */
  public EditResult(
      String content,
      List<AppliedEdit> applied,
      List<EditedDocument> documents,
      List<Diagnostic> diagnostics) {
    this(
        content,
        applied,
        documents,
        diagnostics,
        content.isEmpty()
            && (documents.size() > 1 || documents.stream().anyMatch(d -> !d.content().isEmpty())));
  }

  /**
   * Writes the edited notation of a single-document model to a file.
   *
   * @param path the file, created or truncated
   * @return {@code path}, for chaining
   * @throws IllegalStateException if the edit rewrote a model of several documents, whose text is
   *     in {@link #documents()} alone
   * @throws java.io.UncheckedIOException if the file cannot be written
   */
  public Path save(Path path) {
    if (severalDocuments) {
      throw new IllegalStateException(
          "the edit rewrote a model of several documents; write each of documents() instead");
    }
    return Conversion.writeContent(content, path);
  }

  @Override
  public String toString() {
    return content;
  }
}
