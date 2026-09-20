package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One source-preserving change to a model, applied by {@link Model#applyEdits(List)}.
 *
 * <p>Every target names its element by the id a read reports ({@link Symbol#id()}), and the whole
 * batch applies or none does: a refusal is an {@link EditException} naming why.
 */
public sealed interface Edit {

  /**
   * Sets the value of a feature that already exists, replacing the expression of its {@code =
   * <expr>} or adding one before the declaration's {@code ;}.
   *
   * @param target the feature, as {@link Symbol#id()} names it ({@code "Demo::sc::unitMass"})
   * @param value the new value in notation ({@code "1050.0[SI::kg]"}, {@code "\"m1\""}, {@code
   *     "true"}, {@code "mass * 2"}); it must parse as an expression and resolve in the feature's
   *     own scope
   */
  record SetValue(String target, String value) implements Edit {

    /**
     * Creates the edit.
     *
     * @param target the feature, never {@code null}
     * @param value the new value, never {@code null}
     */
    public SetValue {
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(value, "value");
    }
  }

  /**
   * Rewrites the name token of a declaration and every reference to it in the model's documents. A
   * rename that reaches a reference in a document the edit cannot rewrite is refused, naming the
   * referring elements.
   *
   * @param target the element to rename, as {@link Symbol#id()} names it
   * @param newName the new declared name; it must lex as an identifier
   */
  record Rename(String target, String newName) implements Edit {

    /**
     * Creates the edit.
     *
     * @param target the element, never {@code null}
     * @param newName the new name, never {@code null}
     */
    public Rename {
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(newName, "newName");
    }
  }

  /**
   * Inserts a declaration into a namespace or the document root.
   *
   * @param owner FQN of the namespace to receive the declaration; empty for the document root
   * @param kind the written declaration kind, such as {@code "part def"} or {@code "class"}
   * @param name the declared identifier
   * @param type a type target for a usage, written as notation
   * @param multiplicity a multiplicity, including brackets, such as {@code "[0..*]"}
   * @param value a value expression, written as notation
   * @param specializes specialization targets for a definition
   */
  record AddMember(
      String owner,
      String kind,
      String name,
      Optional<String> type,
      Optional<String> multiplicity,
      Optional<String> value,
      List<String> specializes)
      implements Edit {

    /**
     * Creates the edit, copying the specializations.
     *
     * @param owner the receiving namespace, never {@code null}
     * @param kind the declaration kind, never {@code null}
     * @param name the identifier, never {@code null}
     * @param type the type target, when written
     * @param multiplicity the multiplicity, when written
     * @param value the value expression, when written
     * @param specializes the specialization targets
     */
    public AddMember {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(type, "type");
      Objects.requireNonNull(multiplicity, "multiplicity");
      Objects.requireNonNull(value, "value");
      specializes = List.copyOf(specializes);
    }

    /**
     * A member declaration with none of its optional parts.
     *
     * @param owner the receiving namespace, empty for the document root
     * @param kind the declaration kind
     * @param name the identifier
     * @return the edit
     */
    public static AddMember of(String owner, String kind, String name) {
      return new AddMember(
          owner, kind, name, Optional.empty(), Optional.empty(), Optional.empty(), List.of());
    }

    /**
     * The same member declared with a type.
     *
     * @param type the type target, as notation
     * @return the edit carrying it
     */
    public AddMember withType(String type) {
      return new AddMember(
          owner, kind, name, Optional.of(type), multiplicity, value, specializes);
    }

    /**
     * The same member declared with a multiplicity.
     *
     * @param multiplicity the multiplicity, including brackets
     * @return the edit carrying it
     */
    public AddMember withMultiplicity(String multiplicity) {
      return new AddMember(
          owner, kind, name, type, Optional.of(multiplicity), value, specializes);
    }

    /**
     * The same member declared with a value.
     *
     * @param value the value expression, as notation
     * @return the edit carrying it
     */
    public AddMember withValue(String value) {
      return new AddMember(owner, kind, name, type, multiplicity, Optional.of(value), specializes);
    }

    /**
     * The same member declared with specialization targets.
     *
     * @param specializes the targets, as qualified names
     * @return the edit carrying them
     */
    public AddMember withSpecializes(List<String> specializes) {
      return new AddMember(owner, kind, name, type, multiplicity, value, specializes);
    }
  }

  /**
   * Removes a declaration and its owned trivia.
   *
   * @param target the declaration to remove, as {@link Symbol#id()} names it
   * @param cascade whether to also remove the declarations that refer to the target
   */
  record Delete(String target, boolean cascade) implements Edit {

    /**
     * Creates the edit.
     *
     * @param target the declaration, never {@code null}
     * @param cascade whether referring declarations go too
     */
    public Delete {
      Objects.requireNonNull(target, "target");
    }
  }

  /**
   * Re-parents a declaration: the span {@link Delete} would remove is written where {@link
   * AddMember} would insert it, and the references the move breaks are respelled so the model
   * stays valid. A move whose references cannot be respelled is refused, naming them.
   *
   * @param target the declaration to move, as {@link Symbol#id()} names it
   * @param owner FQN of the namespace to receive it; empty for the document root
   */
  record Move(String target, String owner) implements Edit {

    /**
     * Creates the edit.
     *
     * @param target the declaration, never {@code null}
     * @param owner the receiving namespace, never {@code null}
     */
    public Move {
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(owner, "owner");
    }
  }
}
