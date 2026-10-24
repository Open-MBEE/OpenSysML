package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * What {@link Model#runRuns} runs repeatedly and how, as {@code sysml -runs} is told.
 *
 * <p>Name either an action, run with {@code inputs} on an optional {@code performer}, or a Monte
 * Carlo analysis case, run on a {@code subject} with {@code arguments}. Every run draws at random
 * unless the draws are fixed as {@code min}, {@code max} or {@code average}, so a {@code seed} is
 * required for random draws.
 *
 * @param action qualified name of the action definition or usage, or absent
 * @param analysis qualified name of the Monte Carlo analysis case, or absent
 * @param runs how many runs to make, positive
 * @param seed the seed every random draw starts from
 * @param draws {@code min}, {@code max} or {@code average} to fix the draws instead
 * @param clockStep the clock step of a timed action; zero for the default
 * @param observe the action's features to table after every run
 * @param inputs the action's input parameters, bound first
 * @param performer qualified name of the part that performs the action
 * @param subject qualified name of the subject the case is run on
 * @param arguments the case's parameters, positionally
 * @param namedArguments the case's parameters, by name
 */
public record RunsOptions(
    Optional<String> action,
    Optional<String> analysis,
    long runs,
    Optional<Long> seed,
    Optional<String> draws,
    double clockStep,
    List<String> observe,
    Map<String, Value> inputs,
    Optional<String> performer,
    Optional<String> subject,
    List<Value> arguments,
    Map<String, Value> namedArguments) {
  /** Validates and defensively copies the collections. */
  public RunsOptions {
    Objects.requireNonNull(action, "action");
    Objects.requireNonNull(analysis, "analysis");
    Objects.requireNonNull(seed, "seed");
    Objects.requireNonNull(draws, "draws");
    Objects.requireNonNull(performer, "performer");
    Objects.requireNonNull(subject, "subject");
    observe = List.copyOf(observe);
    inputs = Collections.unmodifiableMap(new LinkedHashMap<>(inputs));
    arguments = List.copyOf(arguments);
    namedArguments = Collections.unmodifiableMap(new LinkedHashMap<>(namedArguments));
  }

  /**
   * Runs an action.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @param runs how many runs to make
   * @return the options, random draws unseeded
   */
  public static RunsOptions action(String actionSymbolId, long runs) {
    Objects.requireNonNull(actionSymbolId, "actionSymbolId");
    return new RunsOptions(
        Optional.of(actionSymbolId),
        Optional.empty(),
        runs,
        Optional.empty(),
        Optional.empty(),
        0,
        List.of(),
        Map.of(),
        Optional.empty(),
        Optional.empty(),
        List.of(),
        Map.of());
  }

  /**
   * Runs a Monte Carlo analysis case.
   *
   * @param analysisSymbolId qualified name of the analysis case
   * @param runs how many runs to make
   * @return the options, random draws unseeded
   */
  public static RunsOptions analysis(String analysisSymbolId, long runs) {
    Objects.requireNonNull(analysisSymbolId, "analysisSymbolId");
    return new RunsOptions(
        Optional.empty(),
        Optional.of(analysisSymbolId),
        runs,
        Optional.empty(),
        Optional.empty(),
        0,
        List.of(),
        Map.of(),
        Optional.empty(),
        Optional.empty(),
        List.of(),
        Map.of());
  }

  /**
   * The same with the random draws seeded.
   *
   * @param seed the seed
   * @return the options
   */
  public RunsOptions withSeed(long seed) {
    return new RunsOptions(
        action, analysis, runs, Optional.of(seed), draws, clockStep, observe, inputs, performer,
        subject, arguments, namedArguments);
  }

  /**
   * The same with the draws fixed.
   *
   * @param draws {@code min}, {@code max} or {@code average}
   * @return the options
   */
  public RunsOptions withDraws(String draws) {
    return new RunsOptions(
        action, analysis, runs, seed, Optional.of(draws), clockStep, observe, inputs, performer,
        subject, arguments, namedArguments);
  }

  /**
   * The same with a clock step.
   *
   * @param clockStep the step
   * @return the options
   */
  public RunsOptions withClockStep(double clockStep) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, performer, subject,
        arguments, namedArguments);
  }

  /**
   * The same observing features after every run.
   *
   * @param observe the features
   * @return the options
   */
  public RunsOptions withObserve(String... observe) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, List.of(observe), inputs, performer,
        subject, arguments, namedArguments);
  }

  /**
   * The same with the action's inputs bound.
   *
   * @param inputs name to value
   * @return the options
   */
  public RunsOptions withInputs(Map<String, Value> inputs) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, performer, subject,
        arguments, namedArguments);
  }

  /**
   * The same performed by a part.
   *
   * @param performer qualified name of the part
   * @return the options
   */
  public RunsOptions withPerformer(String performer) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, Optional.of(performer),
        subject, arguments, namedArguments);
  }

  /**
   * The same run on a subject.
   *
   * @param subject qualified name of the subject
   * @return the options
   */
  public RunsOptions withSubject(String subject) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, performer,
        Optional.of(subject), arguments, namedArguments);
  }

  /**
   * The same with the case's parameters bound positionally.
   *
   * @param arguments the values
   * @return the options
   */
  public RunsOptions withArguments(List<Value> arguments) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, performer, subject,
        arguments, namedArguments);
  }

  /**
   * The same with the case's parameters bound by name.
   *
   * @param namedArguments name to value
   * @return the options
   */
  public RunsOptions withNamedArguments(Map<String, Value> namedArguments) {
    return new RunsOptions(
        action, analysis, runs, seed, draws, clockStep, observe, inputs, performer, subject,
        arguments, namedArguments);
  }
}
