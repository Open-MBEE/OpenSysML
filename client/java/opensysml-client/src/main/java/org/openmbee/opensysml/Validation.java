package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * Every assertion about one object and the objects it holds, checked against their values: what
 * {@link Model#validateInstance(String)} answers.
 *
 * @param summary the object as a whole, of kind {@link Verdict#KIND_OBJECT}: holds when every
 *     assertion holds and every held object was reached; undecided when some assertion was, or
 *     nesting was left unreached
 * @param verdicts one per assertion, each about the object its {@link Verdict#instanceId()} names,
 *     reached along its {@link Verdict#instancePath()}
 * @param verifications the body verdicts of the verification cases verifying each requirement a
 *     verdict is about, each naming its requirement
 * @param instances every object reached, the validated one first
 * @param diagnostics what the service reported while validating
 * @param bounded whether nesting deeper than the validation descends, or past its budget, was left
 *     unvalidated, in which case the summary decides nothing
 */
public record Validation(
    Verdict summary,
    List<Verdict> verdicts,
    List<VerificationVerdict> verifications,
    List<Instance> instances,
    List<Diagnostic> diagnostics,
    boolean bounded) {

  /**
   * Creates a validation, copying its collections.
   *
   * @param summary the object's verdict, never {@code null}
   * @param verdicts the assertion verdicts
   * @param verifications the body verdicts
   * @param instances the objects
   * @param diagnostics the diagnostics
   * @param bounded whether nesting was left unvalidated
   */
  public Validation {
    Objects.requireNonNull(summary, "summary");
    verdicts = List.copyOf(verdicts);
    verifications = List.copyOf(verifications);
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * Whether the object as a whole was decided and holds.
   *
   * @return {@code true} for a decided summary that holds
   */
  public boolean holds() {
    return summary.decided() && summary.holds();
  }

  /**
   * The validated object.
   *
   * @return the object, absent when the service reported none
   */
  public Optional<Instance> root() {
    return instances.isEmpty() ? Optional.empty() : Optional.of(instances.get(0));
  }

  /**
   * The object of an id.
   *
   * @param instanceId the id the service gave the object
   * @return the object, absent when it is not among those reported
   */
  public Optional<Instance> instance(long instanceId) {
    return Instances.find(instances, instanceId);
  }
}
