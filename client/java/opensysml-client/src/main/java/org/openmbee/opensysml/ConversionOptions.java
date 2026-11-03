package org.openmbee.opensysml;

import java.util.List;
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
 * @param includeLibraryStubs restore, in {@code "api-json"}, the ownerless library stubs releases
 *     before 0.11.0 wrote; by default library elements stay bare references, listed in {@link
 *     Conversion#libraryReferences()}; needs the {@code convert_library_stubs} capability
 * @param compact for notation converted to {@code "api-json"}, write the compact document ({@code
 *     api-json-compact/1}) in place of the element array; needs the {@code convert_compact}
 *     capability
 * @param omitDerived with {@code compact}, leave out every derived property of the metamodel
 * @param keepDerived with {@code omitDerived}, the derived properties still written, named as the
 *     element array names them ({@code qualifiedName})
 * @param includeCheckDiagnostics report the semantic check's diagnostics in {@link
 *     Conversion#diagnostics()} beside the output, each with its severity; notation sources only;
 *     needs the {@code convert_check_diagnostics} capability
 */
public record ConversionOptions(
    Optional<String> fromFormat,
    boolean tolerateSyntaxErrors,
    Optional<String> idForm,
    boolean includeLibraryStubs,
    boolean compact,
    boolean omitDerived,
    List<String> keepDerived,
    boolean includeCheckDiagnostics) {

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
   * @param includeLibraryStubs whether library stubs are restored in API JSON
   * @param compact whether the compact API JSON document is written
   * @param omitDerived whether derived properties are left out of the compact document
   * @param keepDerived the derived properties still written when they are left out
   * @param includeCheckDiagnostics whether the check's diagnostics are reported
   */
  public ConversionOptions {
    Objects.requireNonNull(fromFormat, "fromFormat");
    Objects.requireNonNull(idForm, "idForm");
    keepDerived = List.copyOf(Objects.requireNonNull(keepDerived, "keepDerived"));
  }

  /**
   * Options reading the source as {@code fromFormat}, with qualified derived ids.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   */
  public ConversionOptions(Optional<String> fromFormat, boolean tolerateSyntaxErrors) {
    this(fromFormat, tolerateSyntaxErrors, Optional.empty(), false);
  }

  /**
   * Options reading the source as {@code fromFormat} and spelling derived ids as {@code idForm}.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   * @param idForm the derived-id form, when named
   */
  public ConversionOptions(
      Optional<String> fromFormat, boolean tolerateSyntaxErrors, Optional<String> idForm) {
    this(fromFormat, tolerateSyntaxErrors, idForm, false);
  }

  /**
   * Options with the element array written, no check diagnostics and library stubs as asked.
   *
   * @param fromFormat the source format, when named
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   * @param idForm the derived-id form, when named
   * @param includeLibraryStubs whether library stubs are restored in API JSON
   */
  public ConversionOptions(
      Optional<String> fromFormat,
      boolean tolerateSyntaxErrors,
      Optional<String> idForm,
      boolean includeLibraryStubs) {
    this(fromFormat, tolerateSyntaxErrors, idForm, includeLibraryStubs, false, false, List.of(), false);
  }

  /**
   * The service's defaults: the format inferred, syntax errors refused, qualified derived ids,
   * library elements as references, the element array and no check diagnostics.
   *
   * @return the default options
   */
  public static ConversionOptions defaults() {
    return new ConversionOptions(Optional.empty(), false, Optional.empty(), false);
  }

  /**
   * The same options reading the source as {@code fromFormat}.
   *
   * @param fromFormat the source format
   * @return options naming it
   */
  public ConversionOptions withFromFormat(String fromFormat) {
    return new ConversionOptions(
        Optional.of(fromFormat),
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, tolerating syntax errors or not.
   *
   * @param tolerateSyntaxErrors whether unreadable notation is still written back
   * @return options with that tolerance
   */
  public ConversionOptions withTolerateSyntaxErrors(boolean tolerateSyntaxErrors) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options spelling derived ids as {@code idForm}.
   *
   * @param idForm {@link #ID_FORM_QUALIFIED} or {@link #ID_FORM_UUID}
   * @return options naming that form
   */
  public ConversionOptions withIdForm(String idForm) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        Optional.of(Objects.requireNonNull(idForm, "idForm")),
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, restoring the ownerless library stubs in API JSON or not.
   *
   * @param includeLibraryStubs whether library stubs are restored
   * @return options with that choice
   */
  public ConversionOptions withIncludeLibraryStubs(boolean includeLibraryStubs) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, writing the compact API JSON document or the element array.
   *
   * @param compact whether the compact document is written
   * @return options with that choice
   */
  public ConversionOptions withCompact(boolean compact) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, leaving derived properties out of the compact document or not.
   *
   * @param omitDerived whether derived properties are left out
   * @return options with that choice
   */
  public ConversionOptions withOmitDerived(boolean omitDerived) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, still writing the named derived properties when the others are left out.
   *
   * @param keepDerived the derived properties kept, named as the element array names them
   * @return options keeping them
   */
  public ConversionOptions withKeepDerived(List<String> keepDerived) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }

  /**
   * The same options, reporting the semantic check's diagnostics beside the output or not.
   *
   * @param includeCheckDiagnostics whether the check's diagnostics are reported
   * @return options with that choice
   */
  public ConversionOptions withIncludeCheckDiagnostics(boolean includeCheckDiagnostics) {
    return new ConversionOptions(
        fromFormat,
        tolerateSyntaxErrors,
        idForm,
        includeLibraryStubs,
        compact,
        omitDerived,
        keepDerived,
        includeCheckDiagnostics);
  }
}
