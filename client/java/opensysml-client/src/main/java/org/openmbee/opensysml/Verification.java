package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One constraint's or requirement's verdict, with the objects it is about: what {@link
 * Model#verifyConstraint(String)} and {@link Model#verifyRequirement(String)} answer.
 *
 * @param verdict the answer
 * @param verifications the body verdicts of the verification cases verifying this requirement,
 *     empty for a constraint and for a service reporting none
 * @param instances the objects reachable from the verdict's subject, including it, so its feature
 *     values need no further call
 * @param diagnostics what the service reported while verifying
 */
public record Verification(
    Verdict verdict,
    List<VerificationVerdict> verifications,
    List<Instance> instances,
    List<Diagnostic> diagnostics) {

  /**
   * Creates a verification, copying its collections.
   *
   * @param verdict the answer, never {@code null}
   * @param verifications the body verdicts
   * @param instances the objects
   * @param diagnostics the diagnostics
   */
  public Verification {
    Objects.requireNonNull(verdict, "verdict");
    verifications = List.copyOf(verifications);
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * Whether the model answered that the condition holds.
   *
   * @return {@code true} for a decided verdict that holds
   */
  public boolean holds() {
    return verdict.decided() && verdict.holds();
  }

  /**
   * The object the verdict is about.
   *
   * @return the object, absent when the verdict is about declared values alone
   */
  public Optional<Instance> subject() {
    return verdict.instanceId().flatMap(id -> Instances.find(instances, id));
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
