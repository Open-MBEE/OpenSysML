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
 */
public record Query(List<String> scope, List<String> select, Optional<Condition> where) {

  /**
   * Creates a query, copying its lists.
   *
   * @param scope the scope
   * @param select the selection
   * @param where the filter, when there is one
   */
  public Query {
    scope = List.copyOf(scope);
    select = List.copyOf(select);
    Objects.requireNonNull(where, "where");
  }

  /**
   * Every element of the model, with every property.
   *
   * @return the unconstrained query
   */
  public static Query all() {
    return new Query(List.of(), List.of(), Optional.empty());
  }

  /**
   * The same query over these elements and what they nest.
   *
   * @param scope FQNs of the elements
   * @return a query scoped to them
   */
  public Query withScope(List<String> scope) {
    return new Query(scope, select, where);
  }

  /**
   * The same query reporting these properties.
   *
   * @param select the property names
   * @return a query selecting them
   */
  public Query withSelect(List<String> select) {
    return new Query(scope, select, where);
  }

  /**
   * The same query filtered by a condition.
   *
   * @param where the filter
   * @return a query every element of which satisfies it
   */
  public Query where(Condition where) {
    return new Query(scope, select, Optional.of(where));
  }
}
