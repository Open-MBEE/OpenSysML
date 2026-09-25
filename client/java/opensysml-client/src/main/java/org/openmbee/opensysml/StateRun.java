package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.OptionalDouble;

/**
 * What one execution of a state machine produced: what {@link Model#executeState(String, List)}
 * answers.
 *
 * @param statesVisited the trace of states entered, in order
 * @param finalContext the machine's context when execution stopped, by feature name
 * @param finalTime the run's simulation clock when it ended, in seconds from the 0 it started at;
 *     absent from a service without the {@code final_time} capability
 * @param diagnostics what the service reported while executing
 */
public record StateRun(
    List<String> statesVisited,
    Map<String, Value> finalContext,
    OptionalDouble finalTime,
    List<Diagnostic> diagnostics) {

  /**
   * Creates a state run, copying its collections.
   *
   * @param statesVisited the trace
   * @param finalContext the context by feature name
   * @param finalTime the final time, when reported
   * @param diagnostics the diagnostics
   */
  public StateRun {
    statesVisited = List.copyOf(statesVisited);
    finalContext = Map.copyOf(finalContext);
    Objects.requireNonNull(finalTime, "finalTime");
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * The state the machine ended in.
   *
   * @return the last state visited, absent when none was entered
   */
  public Optional<String> finalState() {
    return statesVisited.isEmpty()
        ? Optional.empty()
        : Optional.of(statesVisited.get(statesVisited.size() - 1));
  }
}
