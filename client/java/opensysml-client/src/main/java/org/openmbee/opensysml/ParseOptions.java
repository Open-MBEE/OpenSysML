package org.openmbee.opensysml;

import java.util.Objects;

/**
 * How a source is parsed.
 *
 * @param language notation the source is written in
 * @param strictConformance judge the source as conforming SysML v2, making notation no pinned
 *     production admits an error rather than a warning
 */
public record ParseOptions(Language language, boolean strictConformance) {

  /**
   * Validates the options.
   *
   * @param language the notation
   * @param strictConformance whether to judge conformance strictly
   */
  public ParseOptions {
    Objects.requireNonNull(language, "language");
  }

  /**
   * SysML notation, judged as the service judges it by default.
   *
   * @return the default options
   */
  public static ParseOptions defaults() {
    return new ParseOptions(Language.SYSML, false);
  }

  /**
   * The same options for another notation.
   *
   * @param language the notation
   * @return options naming that notation
   */
  public ParseOptions withLanguage(Language language) {
    return new ParseOptions(language, strictConformance);
  }

  /**
   * The same options, judging conformance strictly or not.
   *
   * @param strictConformance whether to judge strictly
   * @return options with that judgement
   */
  public ParseOptions withStrictConformance(boolean strictConformance) {
    return new ParseOptions(language, strictConformance);
  }
}
