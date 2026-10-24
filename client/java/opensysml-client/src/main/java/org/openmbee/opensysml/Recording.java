package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * What {@link Model#recordRun} did: the run, sweep or sample it made, the records it generated,
 * and the model they now belong to.
 *
 * @param run the single run, when the case ran once
 * @param sweep the sweep, when ranges were given
 * @param sample the Monte Carlo sample, when runs were asked for
 * @param packageName the package the records went into
 * @param definition the per-case record definition
 * @param records the records made, qualified, in order
 * @param source the model's notation with the records in it
 * @param content the record notation alone
 * @param documents the document the records went into, or the records as a document of their own
 * @param skipped rows or runs that failed and were not recorded
 * @param reasons why runs were not recorded
 * @param diagnostics notes raised
 * @param modelHash the recorded model, which the service holds
 */
public record Recording(
    Optional<Analysis> run,
    Optional<Sweep> sweep,
    Optional<Runs> sample,
    String packageName,
    String definition,
    List<String> records,
    String source,
    String content,
    List<EditedDocument> documents,
    long skipped,
    List<String> reasons,
    List<Diagnostic> diagnostics,
    String modelHash) {
  /** Validates and defensively copies the collections. */
  public Recording {
    Objects.requireNonNull(run, "run");
    Objects.requireNonNull(sweep, "sweep");
    Objects.requireNonNull(sample, "sample");
    Objects.requireNonNull(packageName, "packageName");
    Objects.requireNonNull(definition, "definition");
    records = List.copyOf(records);
    Objects.requireNonNull(source, "source");
    Objects.requireNonNull(content, "content");
    documents = List.copyOf(documents);
    reasons = List.copyOf(reasons);
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(modelHash, "modelHash");
  }
}
