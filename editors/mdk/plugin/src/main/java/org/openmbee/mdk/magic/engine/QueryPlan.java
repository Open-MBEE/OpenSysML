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

  public Optional<QueryParameter> parameter(String name) {
    return parameters.stream().filter(parameter -> parameter.name().equals(name)).findFirst();
  }
}
