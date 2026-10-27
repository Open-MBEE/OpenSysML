package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A selection of a model's elements, as the SysML v2 API and Services Query resource makes one: the
 * elements considered, the properties reported for each and the filter every one must satisfy.
 *
 * @param scope FQNs of the elements considered, each together with everything nested inside it;
 *     empty considers the whole model
 * @param select the properties to report ({@code "name"}, {@code "owner"}, {@code
 *     "qualifiedName"}, ...); empty reports all of them
 * @param where the filter every considered element must satisfy, absent for none
 * @param orderBy the properties the selected elements are sorted by, in order of precedence, each
 *     ascending; elements equal under every one keep declaration order, as all of them do when it
 *     is empty
 */
public record Query(
    List<String> scope, List<String> select, Optional<Condition> where, List<String> orderBy) {

  /**
   * Creates a query, copying its lists.
   *
   * @param scope the scope
   * @param select the selection
   * @param where the filter, when there is one
   * @param orderBy the sort properties
   */
  public Query {
    scope = List.copyOf(scope);
    select = List.copyOf(select);
    Objects.requireNonNull(where, "where");
    orderBy = List.copyOf(orderBy);
  }

  /**
   * Creates a query in declaration order.
   *
   * @param scope the scope
   * @param select the selection
   * @param where the filter, when there is one
   */
  public Query(List<String> scope, List<String> select, Optional<Condition> where) {
    this(scope, select, where, List.of());
  }

  /**
   * Every element of the model, with every property.
   *
   * @return the unconstrained query
   */
  public static Query all() {
    return new Query(List.of(), List.of(), Optional.empty(), List.of());
  }

  /**
   * The same query over these elements and what they nest.
   *
   * @param scope FQNs of the elements
   * @return a query scoped to them
   */
  public Query withScope(List<String> scope) {
    return new Query(scope, select, where, orderBy);
  }

  /**
   * The same query reporting these properties.
   *
   * @param select the property names
   * @return a query selecting them
   */
  public Query withSelect(List<String> select) {
    return new Query(scope, select, where, orderBy);
  }

  /**
   * The same query filtered by a condition.
   *
   * @param where the filter
   * @return a query every element of which satisfies it
   */
  public Query where(Condition where) {
    return new Query(scope, select, Optional.of(where), orderBy);
  }

  /**
   * The same query sorted by these properties, in order of precedence.
   *
   * @param orderBy the property names
   * @return a query whose elements are sorted by them
   */
  public Query orderBy(List<String> orderBy) {
    return new Query(scope, select, where, orderBy);
  }
}
