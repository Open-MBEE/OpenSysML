package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * One literal of an enumeration definition, identified by its declaration rather than by a number
 * or a name.
 *
 * @param literalId FQN of the literal's declaration ({@code "D::Color::red"}), its identity
 * @param enumerationId FQN of the enumeration definition declaring it ({@code "D::Color"})
 * @param name the literal as a reader writes it ({@code "Color::red"})
 * @param value the scalar the literal equals when its enumeration specializes a scalar type
 *     ({@code 3} for {@code high = 3} in {@code enum def Level :> Integer}), else empty
 */
public record EnumLiteral(
    String literalId, String enumerationId, String name, Optional<Value> value) {

  /**
   * Creates an enumeration literal.
   *
   * @param literalId FQN of the literal's declaration, never {@code null}
   * @param enumerationId FQN of the declaring enumeration, never {@code null}
   * @param name the literal as written, never {@code null}
   * @param value the scalar the literal equals, or empty; never {@code null}
   */
  public EnumLiteral {
    Objects.requireNonNull(literalId, "literalId");
    Objects.requireNonNull(enumerationId, "enumerationId");
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(value, "value");
  }

  /**
   * Creates a literal that is only its identity.
   *
   * @param literalId FQN of the literal's declaration, never {@code null}
   * @param enumerationId FQN of the declaring enumeration, never {@code null}
   * @param name the literal as written, never {@code null}
   */
  public EnumLiteral(String literalId, String enumerationId, String name) {
    this(literalId, enumerationId, name, Optional.empty());
  }
}
