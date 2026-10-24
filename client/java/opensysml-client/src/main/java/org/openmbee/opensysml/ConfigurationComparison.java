package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.OptionalLong;

/**
 * One configuration of a tool's results index, run again and compared.
 *
 * @param id the configuration's identifier in the index
 * @param name the configuration's name
 * @param behavior the qualified name of the behavior the runs performed
 * @param target the configuration's target, as the index records it
 * @param resultLocation where the tool stored the configuration's results
 * @param status {@code "compared"}, {@code "unresolved"} or {@code "failed"}
 * @param unresolved why the configuration could not be resolved to a behavior, {@code ""} otherwise
 * @param storedRuns how many runs the tool stored
 * @param storedSnapshots how many snapshots the tool stored
 * @param runs how many runs were requested
 * @param completed how many runs completed
 * @param draws the draw mode the runs used
 * @param seed the seed the runs drew with, when random
 * @param clockStep the clock step the runs advanced by
 * @param rows one per run, with its outputs
 * @param instances the instances the runs produced
 * @param observables the observables compared
 * @param errors what failed, per run
 * @param notes what the comparison noted
 * @param lines the comparison as the REPL prints it
 */
public record ConfigurationComparison(
    String id,
    String name,
    String behavior,
    String target,
    String resultLocation,
    String status,
    String unresolved,
    long storedRuns,
    long storedSnapshots,
    long runs,
    long completed,
    String draws,
    OptionalLong seed,
    double clockStep,
    List<SweepRow> rows,
    List<Instance> instances,
    List<ObservableComparison> observables,
    List<String> errors,
    List<String> notes,
    List<String> lines) {
  /** Validates the components and makes the lists immutable. */
  public ConfigurationComparison {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(behavior, "behavior");
    Objects.requireNonNull(target, "target");
    Objects.requireNonNull(resultLocation, "resultLocation");
    Objects.requireNonNull(status, "status");
    Objects.requireNonNull(unresolved, "unresolved");
    Objects.requireNonNull(draws, "draws");
    Objects.requireNonNull(seed, "seed");
    rows = List.copyOf(Objects.requireNonNull(rows, "rows"));
    instances = List.copyOf(Objects.requireNonNull(instances, "instances"));
    observables = List.copyOf(Objects.requireNonNull(observables, "observables"));
    errors = List.copyOf(Objects.requireNonNull(errors, "errors"));
    notes = List.copyOf(Objects.requireNonNull(notes, "notes"));
    lines = List.copyOf(Objects.requireNonNull(lines, "lines"));
  }
}
