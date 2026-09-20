package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One declaration referring to the target of a refused rename, delete or move.
 *
 * @param name the declaration as the notation names it: the qualified name of a named one, or the
 *     heading of an anonymous one within its namespace
 * @param document the document declaring it, named as the parse named it
 */
public record Referrer(String name, String document) {

  /**
   * Creates a referrer.
   *
   * @param name the declaration, never {@code null}
   * @param document the document, never {@code null}
   */
  public Referrer {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(document, "document");
  }
}
