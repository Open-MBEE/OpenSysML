package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * One distinct result an exploration reached: the runs agreeing on it are counted as its
 * linearizations, and one of them is its witness.
 *
 * @param outputs an action's output parameters, or a state machine's final context, by name; for
 *     an analysis case, its outputs and, as {@code "objective <name>"} and {@code "assertion
 *     <name>"} entries, what its verdicts answered
 * @param finalState the state a machine ended in, absent for an action
 * @param statesVisited the trace of states a machine entered, empty for an action
 * @param error why the runs reaching this outcome failed, absent when they completed
 * @param linearizations how many of the orders explored reached this outcome
 * @param witness the choices of one run that reached it, each as {@code "<choice point>: <taken>"}
 * @param diagnostics what the service reported for the witness run
 */
public record Outcome(
    Map<String, Value> outputs,
    Optional<String> finalState,
    List<String> statesVisited,
    Optional<String> error,
    int linearizations,
    List<String> witness,
    List<Diagnostic> diagnostics) {

  /**
   * Creates an outcome, copying its collections.
   *
   * @param outputs the outputs by name
   * @param finalState the final state, when there is one
   * @param statesVisited the trace
   * @param error the failure, when the runs failed
   * @param linearizations the number of orders reaching it
   * @param witness one run's choices
   * @param diagnostics the diagnostics
   */
  public Outcome {
    outputs = Map.copyOf(outputs);
    Objects.requireNonNull(finalState, "finalState");
    statesVisited = List.copyOf(statesVisited);
    Objects.requireNonNull(error, "error");
    witness = List.copyOf(witness);
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * Whether the runs reaching this outcome completed rather than failed.
   *
   * @return {@code true} when no error was reported
   */
  public boolean completed() {
    return error.isEmpty();
  }
}
