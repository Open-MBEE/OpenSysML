package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import org.openmbee.opensysml.proto.CompareResultsResponse;

/**
 * What {@link Model#compareResults} answers: each configuration of a SysML v1 tool's results
 * index run again, with the tool's distributions beside the runs'.
 *
 * @param source where the results index came from
 * @param configurations the configurations, in index order
 * @param standing the engine the runs used and how strong its answers are
 * @param diagnostics the diagnostics the comparison raised
 * @param wire the response as the service answered it
 */
public record Comparison(
    String source,
    List<ConfigurationComparison> configurations,
    Standing standing,
    List<Diagnostic> diagnostics,
    CompareResultsResponse wire) {
  /** Validates the components and makes the lists immutable. */
  public Comparison {
    Objects.requireNonNull(source, "source");
    configurations = List.copyOf(Objects.requireNonNull(configurations, "configurations"));
    Objects.requireNonNull(standing, "standing");
    diagnostics = List.copyOf(Objects.requireNonNull(diagnostics, "diagnostics"));
    Objects.requireNonNull(wire, "wire");
  }
}
