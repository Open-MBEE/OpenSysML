package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * What {@link Model#evaluateCalc(String, List)} computed.
 *
 * <p>A calc definition, or a usage invoked with arguments, computes one {@link #result()}. A calc
 * usage evaluated from its own members computes its {@link #outputs()} by name instead, in
 * declaration order; a body returning into an unnamed result is the output {@code "result"}.
 *
 * @param result the value computed, absent when the calc answers through its outputs
 * @param outputs the calc's out and return parameters by name, empty when it answers with a result
 * @param diagnostics what the service reported while computing
 * @param standing how strongly the answer stands
 */
public record Calculation(
    Optional<Value> result,
    Map<String, Value> outputs,
    List<Diagnostic> diagnostics,
    Standing standing) {

  /**
   * Creates a calculation, copying its collections and keeping the outputs' order.
   *
   * @param result the result, when there is one
   * @param outputs the outputs by name
   * @param diagnostics the diagnostics
   * @param standing the standing, never {@code null}
   */
  public Calculation {
    Objects.requireNonNull(result, "result");
    outputs = Collections.unmodifiableMap(new LinkedHashMap<>(outputs));
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(standing, "standing");
  }

  /**
   * The one value the calc computed: its result, or its single output.
   *
   * @return the value, absent when the calc computed none or several outputs
   */
  public Optional<Value> value() {
    if (result.isPresent()) {
      return result;
    }
    return outputs.size() == 1 ? Optional.of(outputs.values().iterator().next()) : Optional.empty();
  }
}
