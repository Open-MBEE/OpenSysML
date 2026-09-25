package org.openmbee.opensysml;

import java.util.Objects;

/**
 * A rendered document: the Markdown {@link Model#renderDocument(String)} answers, byte-for-byte
 * what the service's command line writes.
 *
 * @param markdown the rendered document
 */
public record RenderedDocument(String markdown) {

  /**
   * Creates a rendered document.
   *
   * @param markdown the Markdown, never {@code null}
   */
  public RenderedDocument {
    Objects.requireNonNull(markdown, "markdown");
  }
}
