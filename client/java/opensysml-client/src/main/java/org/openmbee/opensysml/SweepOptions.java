package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * How a sweep's rows run.
 *
 * @param subject FQN of a part definition or usage to instantiate and bind as an analysis case's
 *     subject; absent for a usage binding its own, and refused for a calc, which has none
 * @param arguments values binding the target's {@code in} parameters in declaration order, as
 *     every row binds them
 * @param namedArguments values binding them by name, as every row binds them
 * @param samples rows to draw uniformly from each range instead of stepping through it, in draw
 *     order; 0 steps through the ranges
 * @param seed the seed the draws are taken from: the same seed draws the same table on every
 *     platform; ignored when {@code samples} is 0
 */
public record SweepOptions(
    Optional<String> subject,
    List<Value> arguments,
    Map<String, Value> namedArguments,
    long samples,
    long seed) {

  /**
   * Validates the options, copying their collections.
   *
   * @param subject the subject, when named
   * @param arguments the positional arguments
   * @param namedArguments the named arguments
   * @param samples the rows to draw
   * @param seed the draws' seed
   */
  public SweepOptions {
    Objects.requireNonNull(subject, "subject");
    arguments = List.copyOf(arguments);
    namedArguments = Collections.unmodifiableMap(new LinkedHashMap<>(namedArguments));
  }

  /**
   * No subject, no arguments, ranges stepped through.
   *
   * @return the default options
   */
  public static SweepOptions defaults() {
    return new SweepOptions(Optional.empty(), List.of(), Map.of(), 0, 0);
  }

  /**
   * The same options run on a subject.
   *
   * @param subject FQN of the part definition or usage
   * @return options naming it
   */
  public SweepOptions withSubject(String subject) {
    return new SweepOptions(Optional.of(subject), arguments, namedArguments, samples, seed);
  }

  /**
   * The same options with these positional arguments.
   *
   * @param arguments the arguments, in parameter order
   * @return options carrying them
   */
  public SweepOptions withArguments(List<Value> arguments) {
    return new SweepOptions(subject, arguments, namedArguments, samples, seed);
  }

  /**
   * The same options with these named arguments.
   *
   * @param namedArguments the arguments by parameter name
   * @return options carrying them
   */
  public SweepOptions withNamedArguments(Map<String, Value> namedArguments) {
    return new SweepOptions(subject, arguments, namedArguments, samples, seed);
  }

  /**
   * The same options drawing rows rather than stepping through the ranges.
   *
   * @param samples the rows to draw
   * @return options drawing them
   */
  public SweepOptions withSamples(long samples) {
    return new SweepOptions(subject, arguments, namedArguments, samples, seed);
  }

  /**
   * The same options drawing from another seed.
   *
   * @param seed the seed
   * @return options drawing from it
   */
  public SweepOptions withSeed(long seed) {
    return new SweepOptions(subject, arguments, namedArguments, samples, seed);
  }
}
