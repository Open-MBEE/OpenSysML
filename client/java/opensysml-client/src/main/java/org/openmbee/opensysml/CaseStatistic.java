package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One statistic an analysis case declares of an observable, beside what the tool stored.
 *
 * @param statistic the statistic's name ({@code mean}, {@code p90}, …)
 * @param of the feature the statistic is of
 * @param tool the tool's value, as text
 * @param run the runs' value, as text
 * @param difference the signed relative difference, as text
 */
public record CaseStatistic(String statistic, String of, String tool, String run, String difference) {
  /** Validates the components. */
  public CaseStatistic {
    Objects.requireNonNull(statistic, "statistic");
    Objects.requireNonNull(of, "of");
    Objects.requireNonNull(tool, "tool");
    Objects.requireNonNull(run, "run");
    Objects.requireNonNull(difference, "difference");
  }
}
