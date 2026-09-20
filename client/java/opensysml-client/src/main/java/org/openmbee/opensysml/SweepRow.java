package org.openmbee.opensysml;

import java.time.Duration;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * One run of a sweep: what it bound, what it produced, and how long it took. A run that failed is
 * a row like any other, carrying {@link #error()} in place of outputs, so one failing run does not
 * lose the rest of the table.
 *
 * @param inputs the swept parameters as this run bound them, by name in range order
 * @param outputs what the run produced, by name; a calc's returned value is named {@code "result"}
 * @param verdicts the objective and assertion verdicts of an analysis case; empty for a calc
 * @param evaluations each application this run made of one of the case's calcs as a value — a
 *     trade study's evaluation of each alternative, in subject order; a failed run keeps the ones
 *     it made. Empty for a calc, or for a service without the {@code case_evaluations} capability
 * @param elapsed the wall time of this run
 * @param error why this run failed; empty when it did not
 * @param failureReason what kind of failure {@code error} reports
 */
public record SweepRow(
    Map<String, Value> inputs,
    Map<String, Value> outputs,
    List<Verdict> verdicts,
    List<CaseEvaluation> evaluations,
    Duration elapsed,
    String error,
    FailureReason failureReason) {

  /**
   * Creates a sweep row, copying its collections.
   *
   * @param inputs the bound parameters by name
   * @param outputs the outputs by name
   * @param verdicts the verdicts
   * @param evaluations the case's evaluations
   * @param elapsed the run's wall time, never {@code null}
   * @param error the failure, empty rather than {@code null} when the run did not fail
   * @param failureReason the failure's kind, never {@code null}
   */
  public SweepRow {
    inputs = Collections.unmodifiableMap(new LinkedHashMap<>(inputs));
    outputs = Collections.unmodifiableMap(new LinkedHashMap<>(outputs));
    verdicts = List.copyOf(verdicts);
    evaluations = List.copyOf(evaluations);
    Objects.requireNonNull(elapsed, "elapsed");
    Objects.requireNonNull(error, "error");
    Objects.requireNonNull(failureReason, "failureReason");
  }

  /**
   * Whether this run failed rather than producing outputs.
   *
   * @return {@code true} when the run carries an error
   */
  public boolean failed() {
    return !error.isEmpty();
  }

  /**
   * Whether the run succeeded and every verdict of it holds.
   *
   * @return {@code true} when the run did not fail and no verdict is undecided or violated
   */
  public boolean holds() {
    return !failed() && verdicts.stream().allMatch(verdict -> verdict.decided() && verdict.holds());
  }
}
