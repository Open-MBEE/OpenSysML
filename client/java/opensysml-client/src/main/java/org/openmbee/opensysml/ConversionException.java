package org.openmbee.opensysml;

import java.util.List;

/**
 * The service could not write a model in the format asked for: notation it could not parse,
 * unless syntax errors were tolerated, or a model the target format cannot carry.
 */
public class ConversionException extends ModelException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a conversion exception.
   *
   * @param message the failure, as the service worded it
   * @param diagnostics the parse diagnostics behind it, when the source did not parse
   */
  public ConversionException(String message, List<Diagnostic> diagnostics) {
    super(message, diagnostics);
  }
}
