package org.openmbee.mdk.magic.engine;

import java.util.List;
import java.util.Optional;

/**
 * The query definitions one subject stands for, by the service's names, and every {@code in}
 * parameter they declare; the bindings a run needs are asked for from this.
 */
public record QueryPlan(List<String> queries, List<QueryParameter> parameters) {
  public QueryPlan {
    queries = List.copyOf(queries);
    parameters = List.copyOf(parameters);
    if (queries.isEmpty()) throw new IllegalArgumentException("a query plan names at least one query");
  }

  /** The parameter a binding may default to the selected element: the sole element-typed one without a value. */
  public Optional<QueryParameter> soleElementParameter() {
    List<QueryParameter> elements = parameters.stream()
        .filter(parameter -> parameter.kind() == QueryParameter.Kind.ELEMENT && parameter.required())
        .toList();
    return elements.size() == 1 ? Optional.of(elements.get(0)) : Optional.empty();
  }

  /** The parameter {@code key} names: its name when one query declares it, else {@code query::name}. */
  public Optional<QueryParameter> parameter(String key) {
    List<QueryParameter> named = parameters.stream()
        .filter(parameter -> parameter.name().equals(key) || qualified(parameter).equals(key))
        .toList();
    if (named.size() > 1 && named.stream().noneMatch(parameter -> qualified(parameter).equals(key))) {
      throw new IllegalArgumentException(key + " is a parameter of several queries; name it as one of "
          + String.join(", ", named.stream().map(QueryPlan::qualified).toList()));
    }
    return named.stream().filter(parameter -> qualified(parameter).equals(key)).findFirst()
        .or(() -> named.stream().findFirst());
  }

  /** How a binding names {@code parameter}: by name alone unless another query declares the name too. */
  public String key(QueryParameter parameter) {
    boolean shared = parameters.stream()
        .anyMatch(other -> other.name().equals(parameter.name()) && !other.query().equals(parameter.query()));
    return shared ? qualified(parameter) : parameter.name();
  }

  private static String qualified(QueryParameter parameter) {
    return parameter.query() + "::" + parameter.name();
  }
}
