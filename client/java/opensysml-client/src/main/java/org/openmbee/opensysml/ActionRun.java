package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.OptionalDouble;

/**
 * What one execution of an action produced: what {@link Model#executeAction(String)} answers.
 *
 * @param outputs the action's output parameters by name, empty for an action producing none
 * @param finalTime the run's simulation clock when it ended, in seconds from the 0 it started at;
 *     absent from a service without the {@code final_time} capability
 * @param diagnostics what the service reported while executing
 */
public record ActionRun(
    Map<String, Value> outputs, OptionalDouble finalTime, List<Diagnostic> diagnostics) {

  /**
   * Creates an action run, copying its collections.
   *
   * @param outputs the outputs by name
   * @param finalTime the final time, when reported
   * @param diagnostics the diagnostics
   */
  public ActionRun {
    outputs = Map.copyOf(outputs);
    Objects.requireNonNull(finalTime, "finalTime");
    diagnostics = List.copyOf(diagnostics);
  }
}
