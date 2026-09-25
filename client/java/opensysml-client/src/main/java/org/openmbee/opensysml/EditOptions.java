package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * How a batch of edits is applied.
 *
 * @param acceptDocuments whether the response's {@code documents} are read; a model of several
 *     documents is edited only when set — unset, such a model is refused as a failed precondition.
 *     {@code true} by default: this client reads {@code documents}
 * @param document the document whose declarations the operations target, named as the parse named
 *     it; absent names the model's first document, which is the only one of a single-document
 *     model
 */
public record EditOptions(boolean acceptDocuments, Optional<String> document) {

  /**
   * Validates the options.
   *
   * @param acceptDocuments whether the response's documents are read
   * @param document the document targeted, when named
   */
  public EditOptions {
    Objects.requireNonNull(document, "document");
  }

  /**
   * Documents accepted, the model's first document edited.
   *
   * @return the default options
   */
  public static EditOptions defaults() {
    return new EditOptions(true, Optional.empty());
  }

  /**
   * The same options, reading the response's documents or not.
   *
   * @param acceptDocuments whether documents are read
   * @return options with that reading
   */
  public EditOptions withAcceptDocuments(boolean acceptDocuments) {
    return new EditOptions(acceptDocuments, document);
  }

  /**
   * The same options targeting one document's declarations.
   *
   * @param document the document's name, as the parse gave it
   * @return options naming it
   */
  public EditOptions withDocument(String document) {
    return new EditOptions(acceptDocuments, Optional.of(document));
  }
}
