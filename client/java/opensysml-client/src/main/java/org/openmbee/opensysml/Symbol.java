package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * One element of a parsed model, as the service describes it.
 *
 * @param id fully qualified name, the identity the service reports
 * @param name declared name, empty for an anonymous element
 * @param kind element kind ({@code "PartDefinition"}, {@code "AttributeUsage"}, …)
 * @param metadata further facts by name, as the service reports them
 * @param childIds FQNs of the element's children
 * @param attributes own attributes with their resolved values
 * @param typeFacts static type facts, absent unless the service advertises {@code type_facts}
 * @param multiplicity declared multiplicity, absent when none is declared
 * @param specializations generalization edges declared, in declaration order
 * @param withheldLibraryAttributes how many inherited standard-library attributes are left out
 */
public record Symbol(
    String id,
    String name,
    String kind,
    Map<String, String> metadata,
    List<String> childIds,
    List<Symbol.Attribute> attributes,
    Optional<Symbol.TypeFacts> typeFacts,
    Optional<Symbol.Multiplicity> multiplicity,
    List<Symbol.Specialization> specializations,
    int withheldLibraryAttributes) {

  /**
   * Creates a symbol, copying every collection.
   *
   * @param id fully qualified name
   * @param name declared name
   * @param kind element kind
   * @param metadata further facts by name
   * @param childIds FQNs of the children
   * @param attributes own attributes
   * @param typeFacts static type facts, when reported
   * @param multiplicity declared multiplicity, when declared
   * @param specializations generalization edges
   * @param withheldLibraryAttributes count of omitted library attributes
   */
  public Symbol {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(kind, "kind");
    metadata = Map.copyOf(metadata);
    childIds = List.copyOf(childIds);
    attributes = List.copyOf(attributes);
    specializations = List.copyOf(specializations);
    Objects.requireNonNull(typeFacts, "typeFacts");
    Objects.requireNonNull(multiplicity, "multiplicity");
  }

  /**
   * An attribute of an element, with the value the service resolved for it.
   *
   * @param name attribute name
   * @param type type as written, empty when none is declared
   * @param value resolved value, absent when the service resolved none
   * @param unit unit as written, absent when the value carries none
   */
  public record Attribute(String name, String type, Optional<Value> value, Optional<String> unit) {
    /**
     * Creates an attribute.
     *
     * @param name attribute name, never {@code null}
     * @param type declared type
     * @param value resolved value, when resolved
     * @param unit unit as written, when written
     */
    public Attribute {
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(type, "type");
      Objects.requireNonNull(value, "value");
      Objects.requireNonNull(unit, "unit");
    }
  }

  /**
   * The static type of a usage, or the classification of a definition.
   *
   * @param declared type name as written, absent when none is declared
   * @param resolvedId FQN of the resolved type, absent when unresolved
   * @param resolvedKind kind of the resolved type, absent when unresolved
   * @param primitive library scalar the type reduces to, absent when it is not a scalar
   * @param primitiveSource where {@code primitive} came from, absent when there is none
   * @param quantity whether values of the type carry a measurement unit
   * @param unit unit as written by the default value, absent when it names none
   */
  public record TypeFacts(
      Optional<String> declared,
      Optional<String> resolvedId,
      Optional<String> resolvedKind,
      Optional<String> primitive,
      Optional<String> primitiveSource,
      boolean quantity,
      Optional<String> unit) {
    /**
     * Creates type facts.
     *
     * @param declared declared type name
     * @param resolvedId FQN of the resolved type
     * @param resolvedKind kind of the resolved type
     * @param primitive library scalar the type reduces to
     * @param primitiveSource origin of {@code primitive}
     * @param quantity whether values carry a unit
     * @param unit unit as written
     */
    public TypeFacts {
      Objects.requireNonNull(declared, "declared");
      Objects.requireNonNull(resolvedId, "resolvedId");
      Objects.requireNonNull(resolvedKind, "resolvedKind");
      Objects.requireNonNull(primitive, "primitive");
      Objects.requireNonNull(primitiveSource, "primitiveSource");
      Objects.requireNonNull(unit, "unit");
    }
  }

  /**
   * A declared multiplicity range. A bound the service cannot evaluate statically is absent.
   *
   * @param lower lower bound ({@code "0"}, {@code "1"}, {@code "*"}), absent when not static
   * @param upper upper bound, absent when not static
   */
  public record Multiplicity(Optional<String> lower, Optional<String> upper) {
    /**
     * Creates a multiplicity.
     *
     * @param lower lower bound
     * @param upper upper bound
     */
    public Multiplicity {
      Objects.requireNonNull(lower, "lower");
      Objects.requireNonNull(upper, "upper");
    }
  }

  /**
   * One generalization edge an element declares.
   *
   * @param kind relationship kind ({@code "specializes"}, {@code "subsets"}, {@code "redefines"},
   *     {@code "typing"})
   * @param declared the target as written in the source
   * @param targetId FQN of the resolved target, absent when the name does not resolve
   * @param targetKind kind of the resolved target, absent when unresolved
   */
  public record Specialization(
      String kind, String declared, Optional<String> targetId, Optional<String> targetKind) {
    /**
     * Creates a specialization.
     *
     * @param kind relationship kind, never {@code null}
     * @param declared target as written, never {@code null}
     * @param targetId FQN of the resolved target
     * @param targetKind kind of the resolved target
     */
    public Specialization {
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(declared, "declared");
      Objects.requireNonNull(targetId, "targetId");
      Objects.requireNonNull(targetKind, "targetKind");
    }
  }
}
