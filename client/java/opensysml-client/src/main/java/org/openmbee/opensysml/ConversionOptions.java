package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * How a conversion reads its source.
 *
 * @param fromFormat the format to read the source as ({@code "sysml"}, {@code "kerml"}, {@code
 *     "text"}, {@code "ttl"}, {@code "turtle"} or {@code "rdf"}); absent infers it from a file's
 *     extension and reads a model's parsed source as notation — inline content has neither, so it
 *     must say
 * @param tolerateSyntaxErrors write notation back out even when the parser could not read all of
 *     it, reporting its syntax errors as the conversion's diagnostics; notation to notation only
 */
public record ConversionOptions(Optional<String> fromFormat, boolean tolerateSyntaxErrors) {

  /**
   * Validates the options.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   */
  public ConversionOptions {
    Objects.requireNonNull(fromFormat, "fromFormat");
  }

  /**
   * The format inferred, and no tolerance of syntax errors.
   *
   * @return the default options
   */
  public static ConversionOptions defaults() {
    return new ConversionOptions(Optional.empty(), false);
  }

  /**
   * The same options reading the source as one format.
   *
   * @param fromFormat the format to read
   * @return options naming it
   */
  public ConversionOptions withFromFormat(String fromFormat) {
    return new ConversionOptions(Optional.of(fromFormat), tolerateSyntaxErrors);
  }

  /**
   * The same options, tolerating syntax errors or not.
   *
   * @param tolerateSyntaxErrors whether to write unreadable notation back
   * @return options with that tolerance
   */
  public ConversionOptions withTolerateSyntaxErrors(boolean tolerateSyntaxErrors) {
    return new ConversionOptions(fromFormat, tolerateSyntaxErrors);
  }
}
