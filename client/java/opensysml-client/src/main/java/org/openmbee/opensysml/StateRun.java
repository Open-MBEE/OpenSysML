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
 * @param trace the documented execution records returned when requested
 * @param traceDropped the number of oldest records the service discarded
 * @param diagnostics what the service reported while executing
 */
public record StateRun(
    List<String> statesVisited,
    Map<String, Value> finalContext,
    OptionalDouble finalTime,
    List<DocumentValue.DocumentEvent> trace,
    int traceDropped,
    List<Diagnostic> diagnostics) {

  /**
   * Creates a state run, copying its collections.
   */
  public StateRun {
    statesVisited = List.copyOf(statesVisited);
    finalContext = Map.copyOf(finalContext);
    Objects.requireNonNull(finalTime, "finalTime");
    trace = List.copyOf(trace);
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
