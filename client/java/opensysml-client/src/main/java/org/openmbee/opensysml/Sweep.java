package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * Every run of one sweep, in the order the runs were made: what {@link Model#runSweep(String,
 * java.util.List)} answers.
 *
 * <p>A swept table runs lexicographically over its parameters in the order their ranges were
 * given; a sampled one runs in draw order, and echoes the seed it was drawn from so the table can
 * be reproduced.
 *
 * @param rows one row per run
 * @param parameters the swept parameters, in the order their ranges were given, which is the order
 *     each row's inputs are in
 * @param sampled whether the rows were drawn rather than stepped through
 * @param seed the seed the rows were drawn from; 0 for a swept table
 * @param instances the subjects the runs were about and the objects reachable from them; empty
 *     when no run bound a subject
 * @param diagnostics what the service reported while running
 * @param standing the engine that ran the table, the strength of its evidence and the bounds it
 *     ran under; unreported when the service predates {@code engines}
 */
public record Sweep(
    List<SweepRow> rows,
    List<String> parameters,
    boolean sampled,
    long seed,
    List<Instance> instances,
    List<Diagnostic> diagnostics,
    Standing standing) {

  /**
   * Creates a sweep table, copying its collections.
   *
   * @param rows the runs
   * @param parameters the swept parameters
   * @param sampled whether the rows were drawn
   * @param seed the draws' seed
   * @param instances the objects the runs reported
   * @param diagnostics the diagnostics
   * @param standing the standing, never {@code null}
   */
  public Sweep {
    rows = List.copyOf(rows);
    parameters = List.copyOf(parameters);
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(standing, "standing");
  }

  /**
   * Whether every run succeeded and every verdict of them holds.
   *
   * @return {@code true} when no row failed and no verdict is undecided or violated
   */
  public boolean holds() {
    return !rows.isEmpty() && rows.stream().allMatch(SweepRow::holds);
  }

  /**
   * The runs that failed.
   *
   * @return the failed rows
   */
  public List<SweepRow> failures() {
    return rows.stream().filter(SweepRow::failed).toList();
  }

  /**
   * The object a value refers to.
   *
   * @param reference a reference a run reported
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
