package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * How a source is parsed.
 *
 * @param language notation the source is written in
 * @param strictConformance judge the source as conforming SysML v2, making notation no pinned
 *     production admits an error rather than a warning
 * @param baseModelHash for {@link Connection#parseSources(java.util.List, ParseOptions)}, the
 *     {@link Model#hash()} of a model of the same documents the caller holds results of, typically
 *     the previous answer: the new model's {@link Model#affected()} then names the documents whose
 *     results may differ from it; needs the {@code parse_sources_affected} capability, and a
 *     one-document parse refuses it
 */
public record ParseOptions(
    Language language, boolean strictConformance, Optional<String> baseModelHash) {

  /**
   * Validates the options.
   *
   * @param language the notation
   * @param strictConformance whether to judge conformance strictly
   * @param baseModelHash the base model's hash, when the affected documents are wanted
   */
  public ParseOptions {
    Objects.requireNonNull(language, "language");
    Objects.requireNonNull(baseModelHash, "baseModelHash");
    baseModelHash.ifPresent(
        hash -> {
          if (hash.isEmpty()) {
            throw new IllegalArgumentException("baseModelHash must not be empty");
          }
        });
  }

  /**
   * Options for the notation, judged as asked, with no base model.
   *
   * @param language the notation
   * @param strictConformance whether to judge conformance strictly
   */
  public ParseOptions(Language language, boolean strictConformance) {
    this(language, strictConformance, Optional.empty());
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
    return new ParseOptions(language, strictConformance, baseModelHash);
  }

  /**
   * The same options, judging conformance strictly or not.
   *
   * @param strictConformance whether to judge strictly
   * @return options with that judgement
   */
  public ParseOptions withStrictConformance(boolean strictConformance) {
    return new ParseOptions(language, strictConformance, baseModelHash);
  }

  /**
   * The same options, relating the parse to a base model so {@link Model#affected()} names the
   * documents whose results may differ from it.
   *
   * @param baseModelHash the {@link Model#hash()} of the base model
   * @return options naming that base
   */
  public ParseOptions withBaseModelHash(String baseModelHash) {
    return new ParseOptions(
        language, strictConformance, Optional.of(Objects.requireNonNull(baseModelHash, "baseModelHash")));
  }
}
