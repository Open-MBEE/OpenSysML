package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * The statistics an analysis case declares of an observable.
 *
 * @param analysis the analysis case's qualified name
 * @param rows the statistics, in declaration order
 * @param notes what kept a statistic from being compared
 */
public record CaseStatistics(String analysis, List<CaseStatistic> rows, List<String> notes) {
  /** Validates the components and makes the lists immutable. */
  public CaseStatistics {
    Objects.requireNonNull(analysis, "analysis");
    rows = List.copyOf(Objects.requireNonNull(rows, "rows"));
    notes = List.copyOf(Objects.requireNonNull(notes, "notes"));
  }
}
