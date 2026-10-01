package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;

/**
 * A SysML v1 model migrated to one of the formats the service writes.
 *
 * <p>Migration is ledgered, not lossless: every element lands in the {@link #report()} as mapped,
 * approximated, unmapped or skipped. The mapping is experimental; {@link #experimentalNotice()}
 * says so in the service's words.
 *
 * @param content the migrated model
 * @param fromFormat the v1 form read, canonically {@code "xmi"}: {@code "uml"} and {@code "mdzip"}
 *     name the same reader
 * @param toFormat the format {@code content} is written in
 * @param experimentalNotice what is experimental about the migration, in the service's own wording
 * @param report what became of every element; never {@code null}, the summary and counts coming
 *     back with every migration
 * @param results the JSON index of the result snapshots the v1 tool stored, as {@code
 *     -migration-results} writes it, when asked for; else empty
 * @param files the image files the model's diagrams embed, by the relative path {@code content}
 *     refers to them with; empty when there are none
 */
public record Migration(
    String content,
    String fromFormat,
    String toFormat,
    String experimentalNotice,
    MigrationReport report,
    String results,
    Map<String, byte[]> files) {

  /**
   * Creates a migration, copying its files.
   *
   * @param content the migrated model, never {@code null}
   * @param fromFormat the form read, never {@code null}
   * @param toFormat the format written, never {@code null}
   * @param experimentalNotice the notice, never {@code null}
   * @param report the report, never {@code null}
   * @param results the results index, never {@code null}
   * @param files the image files, never {@code null}
   */
  public Migration {
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(fromFormat, "fromFormat");
    Objects.requireNonNull(toFormat, "toFormat");
    Objects.requireNonNull(experimentalNotice, "experimentalNotice");
    Objects.requireNonNull(report, "report");
    Objects.requireNonNull(results, "results");
    files = copyOf(files);
  }

  /**
   * The image files, copied: altering a returned array leaves the migration's own as the service
   * wrote it.
   *
   * @return the files by relative path, unmodifiable
   */
  @Override
  public Map<String, byte[]> files() {
    return copyOf(files);
  }

  private static Map<String, byte[]> copyOf(Map<String, byte[]> files) {
    Map<String, byte[]> copied = new LinkedHashMap<>();
    files.forEach((path, data) -> copied.put(path, data.clone()));
    return Collections.unmodifiableMap(copied);
  }
}
