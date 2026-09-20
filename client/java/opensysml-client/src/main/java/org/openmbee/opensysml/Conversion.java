package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A model written out in one of the formats the service writes.
 *
 * @param content the converted model
 * @param fromFormat the format the source was read as; reported even when it was inferred, so a
 *     caller learns what the inference decided
 * @param toFormat the format {@code content} is written in
 * @param diagnostics syntax errors the service tolerated under {@code tolerateSyntaxErrors};
 *     empty otherwise, a conversion that failed being a {@link ModelException}
 * @param experimental whether the conversion went through the RDF mapping, which is experimental;
 *     a notation conversion is stable
 * @param experimentalNotice what is experimental about it, in the service's own wording; empty
 *     when {@code experimental} is false
 */
public record Conversion(
    String content,
    String fromFormat,
    String toFormat,
    List<Diagnostic> diagnostics,
    boolean experimental,
    String experimentalNotice) {

  /**
   * Creates a conversion, copying its diagnostics.
   *
   * @param content the converted model, never {@code null}
   * @param fromFormat the format read
   * @param toFormat the format written
   * @param diagnostics the diagnostics
   * @param experimental whether the conversion is experimental
   * @param experimentalNotice the notice, never {@code null}
   */
  public Conversion {
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(fromFormat, "fromFormat");
    Objects.requireNonNull(toFormat, "toFormat");
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(experimentalNotice, "experimentalNotice");
  }
}
