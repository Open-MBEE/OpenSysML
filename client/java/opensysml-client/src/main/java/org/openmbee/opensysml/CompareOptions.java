package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.OptionalDouble;
import java.util.OptionalLong;

/**
 * How {@link Model#compareResults} runs the configurations.
 *
 * @param actions the configurations to run, by name or id; empty for all
 * @param runs how many runs per configuration; 0 for as many as the tool stored
 * @param seed the seed random draws start from; empty for a fresh one
 * @param draws the draw mode ({@code random}, {@code mean}, …); empty for the service's default
 * @param clockStep the clock step the runs advance by; empty for the service's default
 * @param observe stored observable names mapped onto the features that produce them
 * @param engine the engine to run with; empty for the service's default
 */
public record CompareOptions(
    List<String> actions,
    long runs,
    OptionalLong seed,
    Optional<String> draws,
    OptionalDouble clockStep,
    List<ObservablePair> observe,
    Optional<String> engine) {
  /** Validates the components and makes the lists immutable. */
  public CompareOptions {
    actions = List.copyOf(Objects.requireNonNull(actions, "actions"));
    Objects.requireNonNull(seed, "seed");
    Objects.requireNonNull(draws, "draws");
    Objects.requireNonNull(clockStep, "clockStep");
    observe = List.copyOf(Objects.requireNonNull(observe, "observe"));
    Objects.requireNonNull(engine, "engine");
  }

  /** The service's defaults: every configuration, as many runs as the tool stored. */
  public static CompareOptions defaults() {
    return new CompareOptions(
        List.of(), 0, OptionalLong.empty(), Optional.empty(), OptionalDouble.empty(), List.of(),
        Optional.empty());
  }

  /**
   * A stored observable name mapped onto the feature that produces it.
   *
   * @param stored the name the tool stored the observable under
   * @param feature the feature the runs observe it as
   */
  public record ObservablePair(String stored, String feature) {
    /** Validates the components. */
    public ObservablePair {
      Objects.requireNonNull(stored, "stored");
      Objects.requireNonNull(feature, "feature");
    }
  }
}
