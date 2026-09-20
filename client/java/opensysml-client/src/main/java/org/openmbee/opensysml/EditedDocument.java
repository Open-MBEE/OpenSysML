package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The edited notation of one document of the model.
 *
 * @param name the document's name as the parse named it: the file path of a loaded file, or the
 *     name inline content was given
 * @param content the edited notation, byte-identical to the source outside the edited spans
 */
public record EditedDocument(String name, String content) {

  /**
   * Creates an edited document.
   *
   * @param name the document's name, never {@code null}
   * @param content the edited notation, never {@code null}
   */
  public EditedDocument {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(content, "content");
  }
}
