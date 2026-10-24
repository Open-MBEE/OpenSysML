package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * How {@link Model#recordRun} runs a case and where it writes the records, as {@code sysml
 * -record-run} is told.
 *
 * <p>With {@code ranges} the case is swept and every row recorded; with {@code runs} a Monte Carlo
 * case is sampled and every run recorded with the conclusion; otherwise the case runs once.
 *
 * @param subject qualified name of the subject the case is run on
 * @param arguments the case's parameters, positionally
 * @param namedArguments the case's parameters, by name
 * @param into qualified name of the package the records go into; absent for {@code Records}
 * @param ranges the swept parameters
 * @param runs how many Monte Carlo runs to make; zero for none
 * @param seed the seed the Monte Carlo draws start from
 */
public record RecordOptions(
    Optional<String> subject,
    List<Value> arguments,
    Map<String, Value> namedArguments,
    Optional<String> into,
    List<SweepRange> ranges,
    long runs,
    Optional<Long> seed) {
  /** Validates and defensively copies the collections. */
  public RecordOptions {
    Objects.requireNonNull(subject, "subject");
    arguments = List.copyOf(arguments);
    namedArguments = Collections.unmodifiableMap(new LinkedHashMap<>(namedArguments));
    Objects.requireNonNull(into, "into");
    ranges = List.copyOf(ranges);
    Objects.requireNonNull(seed, "seed");
  }

  /**
   * One run, no subject, into {@code Records}.
   *
   * @return the defaults
   */
  public static RecordOptions defaults() {
    return new RecordOptions(
        Optional.empty(), List.of(), Map.of(), Optional.empty(), List.of(), 0, Optional.empty());
  }

  /**
   * The same run on a subject.
   *
   * @param subject qualified name of the subject
   * @return the options
   */
  public RecordOptions withSubject(String subject) {
    return new RecordOptions(
        Optional.of(subject), arguments, namedArguments, into, ranges, runs, seed);
  }

  /**
   * The same with the case's parameters bound positionally.
   *
   * @param arguments the values
   * @return the options
   */
  public RecordOptions withArguments(List<Value> arguments) {
    return new RecordOptions(subject, arguments, namedArguments, into, ranges, runs, seed);
  }

  /**
   * The same with the case's parameters bound by name.
   *
   * @param namedArguments name to value
   * @return the options
   */
  public RecordOptions withNamedArguments(Map<String, Value> namedArguments) {
    return new RecordOptions(subject, arguments, namedArguments, into, ranges, runs, seed);
  }

  /**
   * The same recorded into a package.
   *
   * @param into qualified name of the package
   * @return the options
   */
  public RecordOptions into(String into) {
    return new RecordOptions(
        subject, arguments, namedArguments, Optional.of(into), ranges, runs, seed);
  }

  /**
   * The same sweeping the case.
   *
   * @param ranges the swept parameters
   * @return the options
   */
  public RecordOptions withRanges(List<SweepRange> ranges) {
    return new RecordOptions(subject, arguments, namedArguments, into, ranges, runs, seed);
  }

  /**
   * The same sampling a Monte Carlo case.
   *
   * @param runs how many runs
   * @param seed the seed the draws start from
   * @return the options
   */
  public RecordOptions withRuns(long runs, long seed) {
    return new RecordOptions(
        subject, arguments, namedArguments, into, ranges, runs, Optional.of(seed));
  }
}
