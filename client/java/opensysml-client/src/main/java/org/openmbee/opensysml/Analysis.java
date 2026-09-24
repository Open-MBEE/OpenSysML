package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * What one run of an analysis case produced: what {@link Model#runAnalysis(String)} answers.
 *
 * @param outputs the case's out and return parameters by name, in declaration order; a body
 *     returning into an unnamed result is the output {@code "result"}
 * @param verdicts the case's objectives, in order, then the assertions in its body, of kinds {@link
 *     Verdict#KIND_OBJECTIVE} and {@link Verdict#KIND_ASSERTION}
 * @param verifications the body verdicts of the case and of the verification cases it performs,
 *     empty for an analysis case
 * @param evaluations the applications the run made of the case's own calcs, in the order made: for
 *     a trade study, its evaluation function applied to each alternative in subject order
 * @param instances the objects the run reported: those reachable from the subject, including it,
 *     and those the outputs and evaluations refer to
 * @param diagnostics what the service reported while running
 * @param standing how strongly the run's answer stands
 */
public record Analysis(
    Map<String, Value> outputs,
    List<Verdict> verdicts,
    List<VerificationVerdict> verifications,
    List<CaseEvaluation> evaluations,
    List<Instance> instances,
    List<Diagnostic> diagnostics,
    Standing standing) {

  /**
   * Creates an analysis, copying its collections and keeping the outputs' order.
   *
   * @param outputs the outputs by name
   * @param verdicts the verdicts
   * @param verifications the body verdicts
   * @param evaluations the evaluations
   * @param instances the objects
   * @param diagnostics the diagnostics
   * @param standing the standing, never {@code null}
   */
  public Analysis {
    outputs = Collections.unmodifiableMap(new LinkedHashMap<>(outputs));
    verdicts = List.copyOf(verdicts);
    verifications = List.copyOf(verifications);
    evaluations = List.copyOf(evaluations);
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(standing, "standing");
  }

  /**
   * Whether every objective and assertion was decided and held. A case stating none holds
   * trivially.
   *
   * @return {@code true} when no verdict is undecided or violated
   */
  public boolean holds() {
    return verdicts.stream().allMatch(verdict -> verdict.decided() && verdict.holds());
  }

  /**
   * The case's objective.
   *
   * @return the first objective verdict, absent for a case stating none
   */
  public Optional<Verdict> objective() {
    return verdicts.stream().filter(verdict -> Verdict.KIND_OBJECTIVE.equals(verdict.kind())).findFirst();
  }

  /**
   * The evaluation a trade study selected.
   *
   * @return the selected evaluation, absent when none was
   */
  public Optional<CaseEvaluation> selected() {
    return evaluations.stream().filter(CaseEvaluation::selected).findFirst();
  }

  /**
   * The object a value refers to.
   *
   * @param reference a reference the run reported
   * @return the object, absent when it is not among those reported
   */
  public Optional<Instance> resolve(Value.InstanceReference reference) {
    Objects.requireNonNull(reference, "reference");
    return Instances.find(instances, reference.instanceId());
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
