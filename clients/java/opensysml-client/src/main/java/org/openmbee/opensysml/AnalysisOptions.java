package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * How an analysis case is run.
 *
 * @param subject FQN of the part definition or usage to instantiate and bind as the case's subject;
 *     absent for a usage binding its own
 * @param arguments values binding the case's {@code in} parameters in declaration order, the subject
 *     excluded
 * @param namedArguments values binding its {@code in} parameters by name
 * @param schedule the policy the actions the case performs resolve their choice points under, as
 *     for {@link ExecutionOptions#schedule()}
 */
public record AnalysisOptions(
    Optional<String> subject,
    List<Value> arguments,
    Map<String, Value> namedArguments,
    Optional<String> schedule) {

  /**
   * Validates the options, copying their collections.
   *
   * @param subject the subject, when named
   * @param arguments the positional arguments
   * @param namedArguments the named arguments
   * @param schedule the schedule, when named
   */
  public AnalysisOptions {
    Objects.requireNonNull(subject, "subject");
    arguments = List.copyOf(arguments);
    namedArguments = Collections.unmodifiableMap(new LinkedHashMap<>(namedArguments));
    Objects.requireNonNull(schedule, "schedule");
  }

  /**
   * No subject, no arguments, the service's default schedule.
   *
   * @return the default options
   */
  public static AnalysisOptions defaults() {
    return new AnalysisOptions(Optional.empty(), List.of(), Map.of(), Optional.empty());
  }

  /**
   * The same options run on a subject.
   *
   * @param subject FQN of the part definition or usage
   * @return options naming it
   */
  public AnalysisOptions withSubject(String subject) {
    return new AnalysisOptions(Optional.of(subject), arguments, namedArguments, schedule);
  }

  /**
   * The same options with these positional arguments.
   *
   * @param arguments the arguments, in parameter order
   * @return options carrying them
   */
  public AnalysisOptions withArguments(List<Value> arguments) {
    return new AnalysisOptions(subject, arguments, namedArguments, schedule);
  }

  /**
   * The same options with these named arguments.
   *
   * @param namedArguments the arguments by parameter name
   * @return options carrying them
   */
  public AnalysisOptions withNamedArguments(Map<String, Value> namedArguments) {
    return new AnalysisOptions(subject, arguments, namedArguments, schedule);
  }

  /**
   * The same options under another schedule.
   *
   * @param schedule the policy
   * @return options naming it
   */
  public AnalysisOptions withSchedule(String schedule) {
    return new AnalysisOptions(subject, arguments, namedArguments, Optional.of(schedule));
  }

  /**
   * Whether the schedule explores every order rather than running one.
   *
   * @return {@code true} for {@code "explore"} and {@code "explore:<options>"}
   */
  public boolean explores() {
    return schedule.filter(ExecutionOptions::explores).isPresent();
  }
}
