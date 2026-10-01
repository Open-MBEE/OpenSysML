package org.openmbee.opensysml;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.TreeSet;

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

  private static final Map<String, String> EXTENSIONS =
      Map.of(
          ".sysml", "sysml",
          ".kerml", "sysml",
          ".ttl", "ttl",
          ".turtle", "ttl",
          ".json", "api-json");

  /**
   * The format to write a file as, from its extension.
   *
   * @param path the file
   * @return {@code "sysml"} for {@code .sysml} and {@code .kerml}, {@code "ttl"} for {@code .ttl}
   *     and {@code .turtle}, {@code "api-json"} for {@code .json}
   * @throws IllegalArgumentException if the extension names no format this client writes
   */
  public static String formatOf(Path path) {
    Objects.requireNonNull(path, "path");
    Path name = path.getFileName();
    String file = name == null ? "" : name.toString();
    int dot = file.lastIndexOf('.');
    String extension = dot <= 0 ? "" : file.substring(dot).toLowerCase(Locale.ROOT);
    String format = EXTENSIONS.get(extension);
    if (format == null) {
      throw new IllegalArgumentException(
          "cannot tell the format to write "
              + path
              + " as: expected one of "
              + new TreeSet<>(EXTENSIONS.keySet())
              + ", or name the format explicitly");
    }
    return format;
  }

  /**
   * Writes the converted model to a file, byte-for-byte as the service returned it.
   *
   * @param path the file, created or truncated
   * @return {@code path}, for chaining
   * @throws UncheckedIOException if the file cannot be written
   */
  public Path write(Path path) {
    return writeContent(content, path);
  }

  static Path writeContent(String content, Path path) {
    Objects.requireNonNull(path, "path");
    try {
      return Files.writeString(path, content, StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new UncheckedIOException(e);
    }
  }

  @Override
  public String toString() {
    return content;
  }
}
