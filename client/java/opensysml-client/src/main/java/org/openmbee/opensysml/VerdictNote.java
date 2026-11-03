package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One assumption or omission behind a {@link Verdict}, worded as every surface reports it.
 *
 * @param kind {@link #KIND_ASSUMED_ONE} or {@link #KIND_UNCOVERED}
 * @param subject the feature the note is about, as the report names it
 * @param text the note as a sentence
 */
public record VerdictNote(String kind, String subject, String text) {

  /** A feature read as holding one value or object while only the default {@code 0..*} governs it. */
  public static final String KIND_ASSUMED_ONE = "assumed_one";

  /** An object-holding feature whose own assertions the question does not state. */
  public static final String KIND_UNCOVERED = "uncovered";

  /**
   * Validates the components.
   *
   * @param kind the kind, never {@code null}
   * @param subject the subject, never {@code null}
   * @param text the text, never {@code null}
   */
  public VerdictNote {
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(subject, "subject");
    Objects.requireNonNull(text, "text");
  }
}
