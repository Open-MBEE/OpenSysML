package org.openmbee.opensysml;

import java.util.Locale;
import java.util.Objects;
import java.util.Optional;

/**
 * A parse or semantic finding the service reported about a model.
 *
 * @param severity how serious the finding is
 * @param message the finding, as the service words it
 * @param code stable identifier of what was found, independent of the message wording: a
 *     validation code, {@code "syntax"}, or {@code "choice-point"} / {@code "guard-unevaluable"}
 *     on a run; empty when the service assigned none
 * @param span where in the source it is, absent when the service located none
 */
public record Diagnostic(Severity severity, String message, String code, Optional<Span> span) {

  /**
   * Creates a diagnostic.
   *
   * @param severity the severity, never {@code null}
   * @param message the message, never {@code null}
   * @param code the code, empty rather than {@code null} when there is none
   * @param span the source location, absent when unlocated
   */
  public Diagnostic {
    Objects.requireNonNull(severity, "severity");
    Objects.requireNonNull(message, "message");
    Objects.requireNonNull(code, "code");
    Objects.requireNonNull(span, "span");
  }

  /** How serious a diagnostic is. */
  public enum Severity {
    /** The model is wrong: parsing or validation failed. */
    ERROR,
    /** The model is questionable but was accepted. */
    WARNING,
    /** Informational only. */
    INFO,
    /** A severity this release of the client does not know. */
    UNKNOWN;

    /**
     * The severity a service reported, by its wire name.
     *
     * @param wireName the severity as the service spells it ({@code "error"})
     * @return the matching severity, or {@link #UNKNOWN} for anything else
     */
    public static Severity fromWireName(String wireName) {
      return switch (wireName) {
        case "error" -> ERROR;
        case "warning" -> WARNING;
        case "info" -> INFO;
        default -> UNKNOWN;
      };
    }

    /**
     * The name the service spells this severity with.
     *
     * @return the wire name; {@code "unknown"} for a severity this release does not know
     */
    public String wireName() {
      return name().toLowerCase(Locale.ROOT);
    }
  }

  /**
   * A range of a source file.
   *
   * @param file path of the file, as the service reports it
   * @param startLine first line, 1-based
   * @param startColumn first column, 1-based
   * @param endLine last line, 1-based
   * @param endColumn last column, 1-based
   */
  public record Span(String file, int startLine, int startColumn, int endLine, int endColumn) {
    /**
     * Creates a span.
     *
     * @param file the file, never {@code null}
     * @param startLine first line
     * @param startColumn first column
     * @param endLine last line
     * @param endColumn last column
     */
    public Span {
      Objects.requireNonNull(file, "file");
    }
  }
}
