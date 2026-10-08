package org.openmbee.opensysml;

import java.nio.file.Path;
import java.util.Objects;
import java.util.Optional;

/**
 * How a SysML v1 model is migrated: the {@code sysml -migrate} command's companion flags.
 *
 * @param fromFormat the v1 form to read the source as ({@code "xmi"}, {@code "uml"} or {@code
 *     "mdzip"}); absent infers it from a file's extension — inline content has none, so it must say
 * @param report whether every element's verdict and the report text {@code -migration-report}
 *     writes are asked for, not just the summary and counts
 * @param results whether the JSON index of the result snapshots the v1 tool stored is asked for,
 *     as {@code -migration-results} writes it
 * @param layoutFile an MTIP export whose diagram layouts the migrated views are laid out from, as
 *     {@code -layout} names it
 * @param layoutContent the MTIP export carried inline instead; at most one of the two
 * @param imageBaseUrl the URL the migrated model refers to its image files under, instead of the
 *     relative {@code images/} paths; empty for the relative paths
 * @param strict write only standard notation, leaving an element whose only v2 form is an
 *     OpenSysML extension unmapped, as {@code -strict}
 */
public record MigrationOptions(
    Optional<String> fromFormat,
    boolean report,
    boolean results,
    Optional<Path> layoutFile,
    Optional<String> layoutContent,
    String imageBaseUrl,
    boolean strict) {

  /**
   * Validates the options.
   *
   * @param fromFormat the v1 form, when named
   * @param report whether the full report is asked for
   * @param results whether the results index is asked for
   * @param layoutFile the MTIP export, when a file
   * @param layoutContent the MTIP export, when inline
   * @param imageBaseUrl the image base URL, never {@code null}
   * @param strict whether only standard notation is written
   * @throws IllegalArgumentException if both a layout file and layout content are given
   */
  public MigrationOptions {
    Objects.requireNonNull(fromFormat, "fromFormat");
    Objects.requireNonNull(layoutFile, "layoutFile");
    Objects.requireNonNull(layoutContent, "layoutContent");
    Objects.requireNonNull(imageBaseUrl, "imageBaseUrl");
    if (layoutFile.isPresent() && layoutContent.isPresent()) {
      throw new IllegalArgumentException("a layout is a file or inline content, not both");
    }
  }

  /**
   * The v1 form inferred, the summary only, no layout, relative image paths, not strict.
   *
   * @return the default options
   */
  public static MigrationOptions defaults() {
    return new MigrationOptions(
        Optional.empty(), false, false, Optional.empty(), Optional.empty(), "", false);
  }

  /**
   * The same options reading the source as one v1 form.
   *
   * @param fromFormat {@code "xmi"}, {@code "uml"} or {@code "mdzip"}
   * @return options naming it
   */
  public MigrationOptions withFromFormat(String fromFormat) {
    return new MigrationOptions(
        Optional.of(fromFormat), report, results, layoutFile, layoutContent, imageBaseUrl, strict);
  }

  /**
   * The same options asking for every element's verdict and the report text.
   *
   * @param report whether the full report is asked for
   * @return options asking, or not
   */
  public MigrationOptions withReport(boolean report) {
    return new MigrationOptions(
        fromFormat, report, results, layoutFile, layoutContent, imageBaseUrl, strict);
  }

  /**
   * The same options asking for the index of the v1 tool's stored results.
   *
   * @param results whether the results index is asked for
   * @return options asking, or not
   */
  public MigrationOptions withResults(boolean results) {
    return new MigrationOptions(
        fromFormat, report, results, layoutFile, layoutContent, imageBaseUrl, strict);
  }

  /**
   * The same options laying the migrated views out from an MTIP export the service reads.
   *
   * @param layoutFile the MTIP export
   * @return options naming it, and carrying no inline layout
   */
  public MigrationOptions withLayoutFile(Path layoutFile) {
    return new MigrationOptions(
        fromFormat, report, results, Optional.of(layoutFile), Optional.empty(), imageBaseUrl, strict);
  }

  /**
   * The same options laying the migrated views out from an MTIP export carried inline.
   *
   * @param layoutContent the MTIP export's text
   * @return options carrying it, and naming no layout file
   */
  public MigrationOptions withLayoutContent(String layoutContent) {
    return new MigrationOptions(
        fromFormat, report, results, Optional.empty(), Optional.of(layoutContent), imageBaseUrl, strict);
  }

  /**
   * The same options referring to image files under a URL.
   *
   * @param imageBaseUrl the URL, or empty for the relative {@code images/} paths
   * @return options with that base
   */
  public MigrationOptions withImageBaseUrl(String imageBaseUrl) {
    return new MigrationOptions(
        fromFormat, report, results, layoutFile, layoutContent, imageBaseUrl, strict);
  }

  /**
   * The same options writing only standard notation, or not.
   *
   * @param strict whether an element whose only v2 form is an extension is left unmapped
   * @return options with that strictness
   */
  public MigrationOptions withStrict(boolean strict) {
    return new MigrationOptions(
        fromFormat, report, results, layoutFile, layoutContent, imageBaseUrl, strict);
  }
}
