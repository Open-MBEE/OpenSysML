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

  private static void requireTarget(String target) {
    Objects.requireNonNull(target, "target");
  }

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
      requireTarget(target);
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
      requireTarget(target);
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
   * @param doc documentation body text, written as the first body member {@code doc /* ... *}{@code /};
   *     empty for none
   */
  record AddMember(
      String owner,
      String kind,
      String name,
      Optional<String> type,
      Optional<String> multiplicity,
      Optional<String> value,
      List<String> specializes,
      boolean isAbstract,
      List<String> redefines,
      boolean isDefault,
      String direction,
      Optional<String> bodyExpression,
      String doc)
      implements Edit {

    public AddMember(
        String owner,
        String kind,
        String name,
        Optional<String> type,
        Optional<String> multiplicity,
        Optional<String> value,
        List<String> specializes,
        boolean isAbstract,
        List<String> redefines,
        boolean isDefault,
        String direction) {
      this(owner, kind, name, type, multiplicity, value, specializes, isAbstract, redefines,
          isDefault, direction, Optional.empty(), "");
    }

    public AddMember(
        String owner,
        String kind,
        String name,
        Optional<String> type,
        Optional<String> multiplicity,
        Optional<String> value,
        List<String> specializes) {
      this(owner, kind, name, type, multiplicity, value, specializes,
          false, List.of(), false, "", Optional.empty(), "");
    }

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
      redefines = List.copyOf(redefines);
      Objects.requireNonNull(direction, "direction");
      Objects.requireNonNull(bodyExpression, "bodyExpression");
      Objects.requireNonNull(doc, "doc");
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
          owner, kind, name, Optional.empty(), Optional.empty(), Optional.empty(), List.of(),
          false, List.of(), false, "", Optional.empty(), "");
    }

    /**
     * The same member declared with a type.
     *
     * @param type the type target, as notation
     * @return the edit carrying it
     */
    public AddMember withType(String type) {
      return new AddMember(
          owner, kind, name, Optional.of(type), multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /**
     * The same member declared with a multiplicity.
     *
     * @param multiplicity the multiplicity, including brackets
     * @return the edit carrying it
     */
    public AddMember withMultiplicity(String multiplicity) {
      return new AddMember(
          owner, kind, name, type, Optional.of(multiplicity), value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /**
     * The same member declared with a value.
     *
     * @param value the value expression, as notation
     * @return the edit carrying it
     */
    public AddMember withValue(String value) {
      return new AddMember(
          owner, kind, name, type, multiplicity, Optional.of(value), specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /**
     * The same member declared with specialization targets.
     *
     * @param specializes the targets, as qualified names
     * @return the edit carrying them
     */
    public AddMember withSpecializes(List<String> specializes) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /** The same member declared abstract. */
    public AddMember withAbstract(boolean isAbstract) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /** The same member declared with redefinition targets. */
    public AddMember withRedefines(List<String> redefines) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /** The same member's value declared with the {@code default} keyword. */
    public AddMember withDefault(boolean isDefault) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /** The same member stating an expression in its body. */
    public AddMember withBodyExpression(String bodyExpression) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, Optional.of(bodyExpression), doc);
    }

    /** The same member declared with a usage direction. */
    public AddMember withDirection(String direction) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }

    /** The same member documented with body text, its first body member. */
    public AddMember withDoc(String doc) {
      return new AddMember(
          owner, kind, name, type, multiplicity, value, specializes,
          isAbstract, redefines, isDefault, direction, bodyExpression, doc);
    }
  }

  /**
   * Adds {@code doc /* ... *}{@code /} as the first body member of an existing declaration, opening
   * a body for one ended by {@code ;}.
   *
   * @param target the documented declaration, as {@link Symbol#id()} names it
   * @param body the documentation text, read back exactly as {@code Documentation::body}; it may
   *     not contain the comment close or a carriage return
   * @param name the documentation's declared name, when written
   * @param locale the documentation's locale, when written
   * @param replace rewrite the one documentation the target owns instead of refusing
   */
  record AddDocumentation(
      String target, String body, Optional<String> name, Optional<String> locale, boolean replace)
      implements Edit {

    public AddDocumentation {
      requireTarget(target);
      Objects.requireNonNull(body, "body");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(locale, "locale");
    }

    public static AddDocumentation of(String target, String body) {
      return new AddDocumentation(target, body, Optional.empty(), Optional.empty(), false);
    }

    public AddDocumentation withName(String name) {
      return new AddDocumentation(target, body, Optional.of(name), locale, replace);
    }

    public AddDocumentation withLocale(String locale) {
      return new AddDocumentation(target, body, name, Optional.of(locale), replace);
    }

    public AddDocumentation withReplace(boolean replace) {
      return new AddDocumentation(target, body, name, locale, replace);
    }
  }

  /**
   * Inserts {@code comment [name] [about a, b] [locale "..."] /* ... *}{@code /} where a new member
   * of the owner goes, opening a body for one ended by {@code ;}.
   *
   * @param owner the namespace receiving the comment; empty is the document root
   * @param body the comment text, read back exactly as {@code Comment::body}; it may not contain
   *     the comment close or a carriage return
   * @param name the comment's declared name, when written
   * @param about the annotated elements, by qualified name
   * @param locale the comment's locale, when written
   */
  record AddComment(
      String owner, String body, Optional<String> name, List<String> about, Optional<String> locale)
      implements Edit {

    public AddComment {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(body, "body");
      Objects.requireNonNull(name, "name");
      about = List.copyOf(about);
      Objects.requireNonNull(locale, "locale");
    }

    public static AddComment of(String owner, String body) {
      return new AddComment(owner, body, Optional.empty(), List.of(), Optional.empty());
    }

    public AddComment withName(String name) {
      return new AddComment(owner, body, Optional.of(name), about, locale);
    }

    public AddComment withAbout(List<String> about) {
      return new AddComment(owner, body, name, about, locale);
    }

    public AddComment withLocale(String locale) {
      return new AddComment(owner, body, name, about, Optional.of(locale));
    }
  }

  /**
   * Writes the line note {@code // text} on its own line above a declaration. A note is lexical
   * trivia, not a model element.
   *
   * @param target the declaration the note precedes, as {@link Symbol#id()} names it
   * @param text the one line of note text; it may not contain a line break
   */
  record AddNote(String target, String text) implements Edit {

    public AddNote {
      requireTarget(target);
      Objects.requireNonNull(text, "text");
      if (text.indexOf('\n') >= 0 || text.indexOf('\r') >= 0) {
        throw new IllegalArgumentException("a note is one line: its text may not contain a line break");
      }
    }
  }

  /** Inserts a satisfy usage into a body that admits behavior usages. */
  record AddSatisfy(
      String owner,
      String requirement,
      Optional<String> satisfyingFeature,
      boolean asserted,
      boolean negated)
      implements Edit {

    public AddSatisfy {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(requirement, "requirement");
      Objects.requireNonNull(satisfyingFeature, "satisfyingFeature");
    }

    public static AddSatisfy of(String owner, String requirement) {
      return new AddSatisfy(owner, requirement, Optional.empty(), false, false);
    }

    public AddSatisfy withSatisfyingFeature(String feature) {
      return new AddSatisfy(owner, requirement, Optional.of(feature), asserted, negated);
    }

    public AddSatisfy withAsserted(boolean asserted) {
      return new AddSatisfy(owner, requirement, satisfyingFeature, asserted, negated);
    }

    public AddSatisfy withNegated(boolean negated) {
      return new AddSatisfy(owner, requirement, satisfyingFeature, asserted, negated);
    }
  }

  /** Inserts a require or assume constraint into a requirement-like body. */
  record AddRequirementConstraint(
      String owner, String kind, String expression, Optional<String> name)
      implements Edit {

    public AddRequirementConstraint {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(expression, "expression");
      Objects.requireNonNull(name, "name");
    }

    public static AddRequirementConstraint of(String owner, String kind, String expression) {
      return new AddRequirementConstraint(owner, kind, expression, Optional.empty());
    }

    public AddRequirementConstraint withName(String name) {
      return new AddRequirementConstraint(owner, kind, expression, Optional.of(name));
    }
  }

  /** Inserts a transition or entry transition into a state body. */
  record AddTransition(
      String owner,
      Optional<String> name,
      Optional<String> source,
      String target,
      Optional<String> trigger,
      Optional<String> guard,
      Optional<String> effect,
      boolean initial)
      implements Edit {

    public AddTransition {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(source, "source");
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(trigger, "trigger");
      Objects.requireNonNull(guard, "guard");
      Objects.requireNonNull(effect, "effect");
    }

    public static AddTransition of(String owner, String source, String target) {
      return new AddTransition(
          owner, Optional.empty(), Optional.of(source), target,
          Optional.empty(), Optional.empty(), Optional.empty(), false);
    }

    public static AddTransition entry(String owner, String target) {
      return new AddTransition(
          owner, Optional.empty(), Optional.empty(), target,
          Optional.empty(), Optional.empty(), Optional.empty(), true);
    }

    public AddTransition withName(String name) {
      return new AddTransition(owner, Optional.of(name), source, target, trigger, guard, effect, initial);
    }

    public AddTransition withTrigger(String trigger) {
      return new AddTransition(owner, name, source, target, Optional.of(trigger), guard, effect, initial);
    }

    public AddTransition withGuard(String guard) {
      return new AddTransition(owner, name, source, target, trigger, Optional.of(guard), effect, initial);
    }

    public AddTransition withEffect(String effect) {
      return new AddTransition(owner, name, source, target, trigger, guard, Optional.of(effect), initial);
    }
  }

  /**
   * Inserts an import declaration into a namespace body or the document root.
   *
   * @param target the imported qualified name, optionally {@code $::}-rooted,
   *     suffixed {@code ::*} for a namespace import
   */
  record AddImport(
      String owner,
      String visibility,
      String target,
      boolean recursive,
      boolean all,
      List<String> filters)
      implements Edit {

    public AddImport {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(visibility, "visibility");
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(filters, "filters");
      filters = List.copyOf(filters);
    }

    public static AddImport of(String owner, String target) {
      return new AddImport(owner, "", target, false, false, List.of());
    }

    public AddImport withVisibility(String visibility) {
      return new AddImport(owner, visibility, target, recursive, all, filters);
    }

    public AddImport withRecursive() {
      return new AddImport(owner, visibility, target, true, all, filters);
    }

    public AddImport withAll() {
      return new AddImport(owner, visibility, target, recursive, true, filters);
    }

    public AddImport withFilters(List<String> filters) {
      return new AddImport(owner, visibility, target, recursive, all, filters);
    }
  }

  /**
   * Inserts a connection-like usage between two feature references.
   *
   * @param owner FQN of the namespace to receive the usage; empty for the document root
   * @param kind the written connection kind, such as {@code "allocation"} or {@code "flow"}
   * @param from the first feature reference, written as notation
   * @param to the second feature reference, written as notation
   * @param name the declared identifier, when named
   * @param type a typing target, when written
   */
  record AddConnection(
      String owner,
      String kind,
      String from,
      String to,
      Optional<String> name,
      Optional<String> type)
      implements Edit {

    /**
     * Creates the edit.
     *
     * @param owner receiving namespace, never {@code null}
     * @param kind connection kind, never {@code null}
     * @param from first feature reference, never {@code null}
     * @param to second feature reference, never {@code null}
     * @param name optional declared identifier
     * @param type optional typing target
     */
    public AddConnection {
      Objects.requireNonNull(owner, "owner");
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(from, "from");
      Objects.requireNonNull(to, "to");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(type, "type");
    }

    /**
     * Creates an unnamed, untyped connection.
     *
     * @param owner receiving namespace, empty for the document root
     * @param kind connection kind
     * @param from first feature reference
     * @param to second feature reference
     * @return the edit
     */
    public static AddConnection of(String owner, String kind, String from, String to) {
      return new AddConnection(
          owner, kind, from, to, Optional.empty(), Optional.empty());
    }

    /**
     * The same connection with a declared name.
     *
     * @param name the identifier
     * @return the edit carrying it
     */
    public AddConnection withName(String name) {
      return new AddConnection(owner, kind, from, to, Optional.of(name), type);
    }

    /**
     * The same connection with a typing target.
     *
     * @param type the type target, as notation
     * @return the edit carrying it
     */
    public AddConnection withType(String type) {
      return new AddConnection(owner, kind, from, to, name, Optional.of(type));
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
      requireTarget(target);
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
      requireTarget(target);
      Objects.requireNonNull(owner, "owner");
    }
  }
}
