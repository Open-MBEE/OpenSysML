package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * One tabular or JSON data file to import, with its content.
 *
 * @param name the file's name; its extension selects the format unless {@code format} does
 * @param content the file's content
 * @param format the format, {@code csv}, {@code tsv}, {@code json} or {@code jsonl}; empty to go by
 *     the name
 * @param map the column-to-feature mapping, as the {@code -import-map} file's content; empty for
 *     the header-named default
 */
public record DataImport(String name, String content, Optional<String> format, Optional<String> map) {
  /** Validates the components. */
  public DataImport {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(format, "format");
    Objects.requireNonNull(map, "map");
  }

  /**
   * An import going by its name's extension, with the header-named mapping.
   *
   * @param name the file's name
   * @param content the file's content
   * @return the import
   */
  public static DataImport of(String name, String content) {
    return new DataImport(name, content, Optional.empty(), Optional.empty());
  }
}
