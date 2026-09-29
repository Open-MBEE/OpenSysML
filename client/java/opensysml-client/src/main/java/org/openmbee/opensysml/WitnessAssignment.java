package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * One feature's value in the assignment witnessing a {@link Verdict}: reported for a violated
 * {@code holds} question and a satisfiable {@code satisfiable} question, as the evaluator
 * replayed it.
 *
 * @param feature the qualified feature name, chain steps appended with {@code '.'}
 * @param value the value the evaluator replayed for the feature, absent when the service reported
 *     none
 * @param unit the base units the magnitude is expressed in; empty for a value that has none
 * @param exact the solver's exact value as text
 */
public record WitnessAssignment(String feature, Optional<Value> value, String unit, String exact) {

  /**
   * Creates a witness assignment.
   *
   * @param feature the feature, never {@code null}
   * @param value its value, never {@code null}
   * @param unit its base units, never {@code null}
   * @param exact the exact spelling, never {@code null}
   */
  public WitnessAssignment {
    Objects.requireNonNull(feature, "feature");
    Objects.requireNonNull(value, "value");
    Objects.requireNonNull(unit, "unit");
    Objects.requireNonNull(exact, "exact");
  }
}
