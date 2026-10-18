package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One observable's distribution in a tool's results beside the runs' distribution of it.
 *
 * @param stored the name the tool stored the observable under
 * @param feature the feature the runs observe it as
 * @param unit the unit the values are in, {@code ""} when none
 * @param tool the tool's distribution; empty when the tool stored no values
 * @param pooled whether the tool's distribution pools summaries rather than raw values
 * @param unstored whether the tool stored nothing for the observable
 * @param run the runs' distribution; empty when no run produced the feature
 * @param produced how many runs produced the feature
 * @param compared whether both distributions exist and were compared
 * @param difference the relative differences, when compared
 * @param declared the analysis case's declared statistics, when it declares any
 * @param notes what the comparison noted
 */
public record ObservableComparison(
    String stored,
    String feature,
    String unit,
    Optional<ComparedDistribution> tool,
    boolean pooled,
    boolean unstored,
    Optional<ComparedDistribution> run,
    long produced,
    boolean compared,
    Optional<StatisticDifference> difference,
    Optional<CaseStatistics> declared,
    List<String> notes) {
  /** Validates the components and makes the list immutable. */
  public ObservableComparison {
    Objects.requireNonNull(stored, "stored");
    Objects.requireNonNull(feature, "feature");
    Objects.requireNonNull(unit, "unit");
    Objects.requireNonNull(tool, "tool");
    Objects.requireNonNull(run, "run");
    Objects.requireNonNull(difference, "difference");
    Objects.requireNonNull(declared, "declared");
    notes = List.copyOf(Objects.requireNonNull(notes, "notes"));
  }
}
