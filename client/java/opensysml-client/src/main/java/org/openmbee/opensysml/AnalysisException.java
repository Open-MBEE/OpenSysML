package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * An analysis case that could not run to its end but left something to inspect: the outputs and
 * evaluations made before an alternative failed, an objective the failure left undecided.
 *
 * <p>A request refused before the run, or a failure leaving nothing to report, is a plain {@link
 * ModelException} whose {@link #failureReason()} says why.
 */
public class AnalysisException extends ModelException {

  private static final long serialVersionUID = 1L;

  private final transient Analysis partial;

  /**
   * Creates an analysis exception.
   *
   * @param message the failure, as the service worded it
   * @param failureReason what kind of failure it is
   * @param diagnostics diagnostics the answer carried
   * @param partial what the run left
   */
  public AnalysisException(
      String message, FailureReason failureReason, List<Diagnostic> diagnostics, Analysis partial) {
    super(message, failureReason, diagnostics);
    this.partial = Objects.requireNonNull(partial, "partial");
  }

  /**
   * What the run left: the outputs and evaluations made, the objects they name, each verdict
   * undecided.
   *
   * @return the partial result; absent after Java serialization, which does not carry it
   */
  public Optional<Analysis> partial() {
    return Optional.ofNullable(partial);
  }
}
