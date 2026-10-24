package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;
import java.util.OptionalDouble;
import java.util.OptionalLong;

/**
 * How a behavior is run.
 *
 * @param schedule the policy the run resolves its choice points under, as {@code sysml -schedule}
 *     spells it: {@code "declared"}, {@code "reverse"} (the service's default) or {@code
 *     "seed:<n>"} for one run; {@code "explore"} or {@code "explore:runs=<n>,depth=<d>"} for an
 *     exploration. Absent leaves the choice to the call.
 * @param performer the object the behavior runs on, as {@code sysml -action "<action> <object>"}
 *     names it: a part definition or usage to make an object of, or a path from one into its parts
 *     ({@code "Mission::mission.vehicle"}), made for the run. Absent runs outside any object.
 * @param trace whether a state run returns its documented execution trace
 * @param advance simulated time, in seconds, to run for ({@code sysml -advance}): the behavior is
 *     started and its clock moved by this much; an action not completed by then fails with the
 *     instant it waits for. Absent runs to completion.
 * @param seed the seed the model's own draws are taken from ({@code sysml -seed}); absent leaves
 *     them to the schedule
 * @param draws how RandomFunctions draws resolve ({@code sysml -draws}): {@code "random"} (the
 *     default), {@code "min"}, {@code "max"} or {@code "average"}
 * @param clockStep the step, in seconds, the run's clock ticks by ({@code sysml -clock-step}); 0
 *     is a continuous clock
 */
public record ExecutionOptions(
    Optional<String> schedule,
    Optional<String> performer,
    boolean trace,
    OptionalDouble advance,
    OptionalLong seed,
    Optional<String> draws,
    double clockStep) {

  private static final String EXPLORE = "explore";

  /**
   * Validates the options.
   *
   * @param schedule the schedule, when named
   * @param performer the performer, when named
   */
  public ExecutionOptions {
    Objects.requireNonNull(schedule, "schedule");
    Objects.requireNonNull(performer, "performer");
    Objects.requireNonNull(advance, "advance");
    Objects.requireNonNull(seed, "seed");
    Objects.requireNonNull(draws, "draws");
  }

  /**
   * The service's default schedule, outside any object.
   *
   * @return the default options
   */
  public static ExecutionOptions defaults() {
    return new ExecutionOptions(
        Optional.empty(),
        Optional.empty(),
        false,
        OptionalDouble.empty(),
        OptionalLong.empty(),
        Optional.empty(),
        0);
  }

  /**
   * The same options under another schedule.
   *
   * @param schedule the policy
   * @return options naming it
   */
  public ExecutionOptions withSchedule(String schedule) {
    return new ExecutionOptions(
        Optional.of(schedule), performer, trace, advance, seed, draws, clockStep);
  }

  /**
   * The same options, performed by an object.
   *
   * @param performer the object, as a declaration or a path into one
   * @return options naming it
   */
  public ExecutionOptions withPerformer(String performer) {
    return new ExecutionOptions(
        schedule, Optional.of(performer), trace, advance, seed, draws, clockStep);
  }

  /** The same options with a state-run trace requested. */
  public ExecutionOptions withTrace() {
    return new ExecutionOptions(schedule, performer, true, advance, seed, draws, clockStep);
  }

  /**
   * The same options run for a duration of simulated time rather than to completion.
   *
   * @param seconds the duration, as {@code sysml -advance} takes it
   * @return options running for it
   */
  public ExecutionOptions withAdvance(double seconds) {
    return new ExecutionOptions(
        schedule, performer, trace, OptionalDouble.of(seconds), seed, draws, clockStep);
  }

  /**
   * The same options with the model's own draws seeded.
   *
   * @param seed the seed, as {@code sysml -seed} takes it
   * @return options drawing from it
   */
  public ExecutionOptions withSeed(long seed) {
    return new ExecutionOptions(
        schedule, performer, trace, advance, OptionalLong.of(seed), draws, clockStep);
  }

  /**
   * The same options resolving RandomFunctions draws under a policy.
   *
   * @param draws {@code "random"}, {@code "min"}, {@code "max"} or {@code "average"}
   * @return options under it
   */
  public ExecutionOptions withDraws(String draws) {
    return new ExecutionOptions(
        schedule, performer, trace, advance, seed, Optional.of(draws), clockStep);
  }

  /**
   * The same options with the run's clock ticking by a step.
   *
   * @param seconds the step; 0 is a continuous clock
   * @return options ticking by it
   */
  public ExecutionOptions withClockStep(double seconds) {
    return new ExecutionOptions(schedule, performer, trace, advance, seed, draws, seconds);
  }

  /**
   * Whether the schedule explores every order rather than running one.
   *
   * @return {@code true} for {@code "explore"} and {@code "explore:<options>"}
   */
  public boolean explores() {
    return schedule.filter(ExecutionOptions::explores).isPresent();
  }

  static boolean explores(String schedule) {
    return schedule.equals(EXPLORE) || schedule.startsWith(EXPLORE + ":");
  }
}
