package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * One element of a parsed model, as the service describes it.
 *
 * <p>Several fields summarise metamodel properties that {@code elementProperties} derives in
 * full; each names the property it summarises, and that call is the surface whose answer the
 * specification defines.
 *
 * @param id fully qualified name, the identity the service reports
 * @param name declared name, empty for an anonymous element
 * @param kind declaration keyword kind ({@code "partDef"}, {@code "attributeUsage"}, …): the
 *     written syntax, not the metaclass {@code elementProperties} reports
 * @param metadata summary facts by name: {@code "type"} and {@code "specializes"} are the first
 *     written edge of each ({@code specializations} has them all), {@code "direction"} the
 *     direction written on the feature (the derived {@code Feature::direction} is {@code
 *     elementProperties}' {@code direction}), plus {@code "multiplicity"}, {@code "abstract"}, …
 * @param childIds FQNs of the element's children
 * @param attributes own attributes with their resolved values
 * @param typeFacts static type facts, absent unless the service advertises {@code type_facts}:
 *     the written typing and the derived {@code Feature::type}
 * @param multiplicity declared multiplicity, absent when none is declared
 * @param specializations generalization edges written, in declaration order: what {@code
 *     elementProperties}' {@code ownedSpecialization} reflects, without the implied ones
 * @param withheldLibraryAttributes how many inherited standard-library attributes are left out
 * @param members every member, own then inherited, as a service advertising {@code symbol_members}
 *     enumerates them; empty from one that does not
 * @param withheldLibraryMembers how many members inherited from standard-library content are left
 *     out, until asked for with {@link SymbolOptions#withLibraryMembers}
 * @param annotatingMetadataIds FQNs of the metadata usages annotating the element (the inverse of
 *     KerML {@code AnnotatingElement::annotatedElement}): its own {@code #M} prefixes and body
 *     {@code @M} usages, then the ones naming it in an {@code about} clause elsewhere, which
 *     {@code childIds} and {@code members} leave out; empty from a service that does not advertise
 *     {@code symbol_annotating_metadata}
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
    int withheldLibraryAttributes,
    List<Symbol.Member> members,
    int withheldLibraryMembers,
    List<String> annotatingMetadataIds) {

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
   * @param members every member enumerated, own then inherited
   * @param withheldLibraryMembers count of omitted library members
   * @param annotatingMetadataIds FQNs of the metadata usages annotating the element
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
    members = List.copyOf(members);
    annotatingMetadataIds = List.copyOf(annotatingMetadataIds);
  }

  /**
   * Creates a symbol of a service that names no annotating metadata.
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
   * @param members every member enumerated, own then inherited
   * @param withheldLibraryMembers count of omitted library members
   */
  public Symbol(
      String id,
      String name,
      String kind,
      Map<String, String> metadata,
      List<String> childIds,
      List<Symbol.Attribute> attributes,
      Optional<Symbol.TypeFacts> typeFacts,
      Optional<Symbol.Multiplicity> multiplicity,
      List<Symbol.Specialization> specializations,
      int withheldLibraryAttributes,
      List<Symbol.Member> members,
      int withheldLibraryMembers) {
    this(
        id,
        name,
        kind,
        metadata,
        childIds,
        attributes,
        typeFacts,
        multiplicity,
        specializations,
        withheldLibraryAttributes,
        members,
        withheldLibraryMembers,
        List.of());
  }

  /**
   * Creates a symbol of a service that enumerates no members.
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
  public Symbol(
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
    this(
        id,
        name,
        kind,
        metadata,
        childIds,
        attributes,
        typeFacts,
        multiplicity,
        specializations,
        withheldLibraryAttributes,
        List.of(),
        0);
  }

  /**
   * The members this symbol declares itself: {@link #members()} without the inherited ones.
   *
   * @return the owned members, in declaration order
   */
  public List<Member> ownMembers() {
    return members.stream().filter(member -> !member.inherited()).toList();
  }

  /**
   * The members inherited through specialization and typing, in inheritance order, as KerML
   * {@code Type::member} lists them: a redefining member hides the one it redefines, and a private
   * member of a supertype is not inherited.
   *
   * @return the inherited members, each naming the type it comes from
   */
  public List<Member> inheritedMembers() {
    return members.stream().filter(Member::inherited).toList();
  }

  /**
   * One member of a type, own or inherited.
   *
   * @param id fully qualified name of the member, the id {@link Model#symbol(String)} accepts
   * @param name declared name, empty for an anonymous member
   * @param kind element kind ({@code "partUsage"}, {@code "attributeUsage"}, …)
   * @param inherited whether the member is inherited rather than declared by the symbol
   * @param inheritedFrom FQN of the type that declares an inherited member, absent for an own one
   * @param library whether the member is standard-library content
   */
  public record Member(
      String id,
      String name,
      String kind,
      boolean inherited,
      Optional<String> inheritedFrom,
      boolean library) {
    /**
     * Creates a member.
     *
     * @param id fully qualified name, never {@code null}
     * @param name declared name
     * @param kind element kind
     * @param inherited whether it is inherited
     * @param inheritedFrom the declaring type, when inherited
     * @param library whether it is standard-library content
     */
    public Member {
      Objects.requireNonNull(id, "id");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(inheritedFrom, "inheritedFrom");
    }
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
   * One derived type of a feature.
   *
   * @param id FQN of the type
   * @param kind symbol kind of the type ({@code "partDef"}, {@code "attributeDef"}, …), absent
   *     when the service reports none
   */
  public record TypeRef(String id, Optional<String> kind) {
    /**
     * Creates a type reference.
     *
     * @param id FQN of the type, never {@code null}
     * @param kind symbol kind of the type
     */
    public TypeRef {
      Objects.requireNonNull(id, "id");
      Objects.requireNonNull(kind, "kind");
    }
  }

  /**
   * The static type of a usage, or the classification of a definition. {@code declared},
   * {@code resolvedId} and {@code resolvedKind} are the typing as written ({@code : T}); {@code
   * types} is the derived KerML {@code Feature::type} — the {@code type} property of {@code
   * elementProperties} — which the typing of every feature the usage subsets, references or
   * redefines contributes to.
   *
   * @param declared type name as written, absent when none is declared
   * @param resolvedId FQN of the resolved written type, absent when unresolved
   * @param resolvedKind kind of the resolved written type, absent when unresolved
   * @param primitive library scalar the type reduces to, absent when it is not a scalar
   * @param primitiveSource where {@code primitive} came from, absent when there is none
   * @param quantity whether values of the type carry a measurement unit
   * @param unit unit as written by the default value, absent when it names none
   * @param types the derived types, most specific first with redundant supertypes removed;
   *     empty for a definition or when none derives
   */
  public record TypeFacts(
      Optional<String> declared,
      Optional<String> resolvedId,
      Optional<String> resolvedKind,
      Optional<String> primitive,
      Optional<String> primitiveSource,
      boolean quantity,
      Optional<String> unit,
      List<TypeRef> types) {
    /**
     * Creates type facts, copying the type list.
     *
     * @param declared declared type name
     * @param resolvedId FQN of the resolved type
     * @param resolvedKind kind of the resolved type
     * @param primitive library scalar the type reduces to
     * @param primitiveSource origin of {@code primitive}
     * @param quantity whether values carry a unit
     * @param unit unit as written
     * @param types derived types
     */
    public TypeFacts {
      types = List.copyOf(types);
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
   *     {@code "typing"} or {@code "references"}, reference subsetting: {@code ::>} and {@code
   *     include})
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
