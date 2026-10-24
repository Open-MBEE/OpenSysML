package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The signed relative differences between the runs' statistics and the tool's, as the REPL
 * tables them ({@code "+2.0%"}, {@code "-"} when the tool's value is zero).
 *
 * @param min the difference of the smallest values
 * @param mean the difference of the means
 * @param p50 the difference of the medians
 * @param p90 the difference of the 90th percentiles
 * @param max the difference of the largest values
 */
public record StatisticDifference(String min, String mean, String p50, String p90, String max) {
  /** Validates the components. */
  public StatisticDifference {
    Objects.requireNonNull(min, "min");
    Objects.requireNonNull(mean, "mean");
    Objects.requireNonNull(p50, "p50");
    Objects.requireNonNull(p90, "p90");
    Objects.requireNonNull(max, "max");
  }
}
