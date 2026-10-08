package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The lowered graph of an action or state machine and of every behavior it performs, in the
 * canonical {@code graphs:<version>} JSON form an external analysis engine is sent: what {@link
 * Model#exportGraphs(String)} answers.
 *
 * @param content the form as canonical JSON, ending in one newline
 * @param version the version of the form, the {@code version} field of the JSON
 * @param subject the qualified name of the behavior as resolved
 */
public record Graphs(String content, int version, String subject) {

  /**
   * Creates an exported graph.
   *
   * @param content the JSON, never {@code null}
   * @param version the form's version
   * @param subject the resolved subject, never {@code null}
   */
  public Graphs {
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(subject, "subject");
  }

  @Override
  public String toString() {
    return content;
  }
}
