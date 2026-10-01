package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * How a conversion reads its source.
 *
 * @param fromFormat the format to read the source as ({@code "sysml"}, {@code "kerml"}, {@code
 *     "text"}, {@code "ttl"}, {@code "turtle"}, {@code "rdf"}, {@code "api-json"} or {@code
 *     "json"}); absent infers it from a file's
 *     extension and reads a model's parsed source as notation — inline content has neither, so it
 *     must say
 * @param tolerateSyntaxErrors write notation back out even when the parser could not read all of
 *     it, reporting its syntax errors as the conversion's diagnostics; notation to notation only
 * @param idForm how derived element ids are spelled when notation is written as a graph ({@code
 *     "ttl"} or {@code "api-json"}), as {@code sysml -id} does: {@code "qualified"} or {@code
 *     "uuid"}; absent is qualified, and the service refuses it for any other direction
 */
public record ConversionOptions(
    Optional<String> fromFormat, boolean tolerateSyntaxErrors, Optional<String> idForm) {

  /** Qualified-name derived ids, the default. */
  public static final String ID_FORM_QUALIFIED = "qualified";

  /** Name-based uuids under each root package, the library convention. */
  public static final String ID_FORM_UUID = "uuid";

  /**
   * Validates the options.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   * @param idForm the derived-id form, when named
   */
  public ConversionOptions {
    Objects.requireNonNull(fromFormat, "fromFormat");
    Objects.requireNonNull(idForm, "idForm");
  }

  /**
   * Options naming no id form.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   */
  public ConversionOptions(Optional<String> fromFormat, boolean tolerateSyntaxErrors) {
    this(fromFormat, tolerateSyntaxErrors, Optional.empty());
  }

  /**
   * The format inferred, and no tolerance of syntax errors.
   *
   * @return the default options
   */
  public static ConversionOptions defaults() {
    return new ConversionOptions(Optional.empty(), false, Optional.empty());
  }

  /**
   * The same options reading the source as one format.
   *
   * @param fromFormat the format to read
   * @return options naming it
   */
  public ConversionOptions withFromFormat(String fromFormat) {
    return new ConversionOptions(Optional.of(fromFormat), tolerateSyntaxErrors, idForm);
  }

  /**
   * The same options, tolerating syntax errors or not.
   *
   * @param tolerateSyntaxErrors whether to write unreadable notation back
   * @return options with that tolerance
   */
  public ConversionOptions withTolerateSyntaxErrors(boolean tolerateSyntaxErrors) {
    return new ConversionOptions(fromFormat, tolerateSyntaxErrors, idForm);
  }

  /**
   * The same options, spelling derived element ids in one form.
   *
   * @param idForm {@link #ID_FORM_QUALIFIED} or {@link #ID_FORM_UUID}
   * @return options naming it
   */
  public ConversionOptions withIdForm(String idForm) {
    return new ConversionOptions(
        fromFormat, tolerateSyntaxErrors, Optional.of(Objects.requireNonNull(idForm, "idForm")));
  }
}
