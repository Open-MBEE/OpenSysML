package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

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
 */
public record ExecutionOptions(Optional<String> schedule, Optional<String> performer) {

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
  }

  /**
   * The service's default schedule, outside any object.
   *
   * @return the default options
   */
  public static ExecutionOptions defaults() {
    return new ExecutionOptions(Optional.empty(), Optional.empty());
  }

  /**
   * The same options under another schedule.
   *
   * @param schedule the policy
   * @return options naming it
   */
  public ExecutionOptions withSchedule(String schedule) {
    return new ExecutionOptions(Optional.of(schedule), performer);
  }

  /**
   * The same options, performed by an object.
   *
   * @param performer the object, as a declaration or a path into one
   * @return options naming it
   */
  public ExecutionOptions withPerformer(String performer) {
    return new ExecutionOptions(schedule, Optional.of(performer));
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
