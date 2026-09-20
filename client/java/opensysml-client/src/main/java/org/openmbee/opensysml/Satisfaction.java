package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * The verdict of each {@code satisfy} assertion evaluated, in declaration order: what {@link
 * Model#verifySatisfaction()} answers. A model stating none answers with no verdicts.
 *
 * @param verdicts one verdict per assertion
 * @param verifications the body verdicts of every requirement the verdicts are about, each naming
 *     its requirement, which the verdicts carry as their {@link Verdict#requirementId()}
 * @param instances the objects the verdicts are about, each reported once
 * @param diagnostics what the service reported while verifying
 */
public record Satisfaction(
    List<Verdict> verdicts,
    List<VerificationVerdict> verifications,
    List<Instance> instances,
    List<Diagnostic> diagnostics) {

  /**
   * Creates a satisfaction result, copying its collections.
   *
   * @param verdicts the verdicts
   * @param verifications the body verdicts
   * @param instances the objects
   * @param diagnostics the diagnostics
   */
  public Satisfaction {
    verdicts = List.copyOf(verdicts);
    verifications = List.copyOf(verifications);
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * Whether every assertion was decided and held. A model stating none holds trivially.
   *
   * @return {@code true} when no verdict is undecided or violated
   */
  public boolean holds() {
    return verdicts.stream().allMatch(verdict -> verdict.decided() && verdict.holds());
  }

  /**
   * The assertions the model answered false about.
   *
   * @return the violated verdicts, in declaration order
   */
  public List<Verdict> violated() {
    return verdicts.stream().filter(Verdict::violated).toList();
  }

  /**
   * The assertions that could not be evaluated.
   *
   * @return the undecided verdicts, in declaration order
   */
  public List<Verdict> undecided() {
    return verdicts.stream().filter(verdict -> !verdict.decided()).toList();
  }

  /**
   * The body verdicts reported for one requirement.
   *
   * @param requirementId FQN of the requirement
   * @return its verification cases' verdicts, in the order reported
   */
  public List<VerificationVerdict> verificationsOf(String requirementId) {
    Objects.requireNonNull(requirementId, "requirementId");
    return verifications.stream()
        .filter(verdict -> verdict.requirementId().filter(requirementId::equals).isPresent())
        .toList();
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
