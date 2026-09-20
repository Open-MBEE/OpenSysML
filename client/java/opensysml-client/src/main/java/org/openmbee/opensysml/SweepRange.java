package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * One parameter's sweep range: the values it binds, one per row of the table {@link
 * Model#runSweep(String, java.util.List)} answers.
 *
 * @param parameter the input parameter the range binds, which the target must declare and the
 *     sweep's own arguments must not bind
 * @param start the endpoint the range's rows start at; required
 * @param end the endpoint they run to, inclusive where the step lands on it; required
 * @param step what the range advances by; absent steps by one over whole-number endpoints, and is
 *     refused for a sampled table, which draws instead
 */
public record SweepRange(String parameter, Value start, Value end, Optional<Value> step) {

  /**
   * Creates a range.
   *
   * @param parameter the parameter's name, never {@code null}
   * @param start the start endpoint, never {@code null}
   * @param end the end endpoint, never {@code null}
   * @param step the step, when the range states one
   */
  public SweepRange {
    Objects.requireNonNull(parameter, "parameter");
    Objects.requireNonNull(start, "start");
    Objects.requireNonNull(end, "end");
    Objects.requireNonNull(step, "step");
  }

  /**
   * A range stating no step.
   *
   * @param parameter the parameter's name
   * @param start the start endpoint
   * @param end the end endpoint
   * @return the range
   */
  public static SweepRange of(String parameter, Value start, Value end) {
    return new SweepRange(parameter, start, end, Optional.empty());
  }

  /**
   * The same range advancing by a step.
   *
   * @param step the step
   * @return the range carrying it
   */
  public SweepRange withStep(Value step) {
    return new SweepRange(parameter, start, end, Optional.of(step));
  }
}
