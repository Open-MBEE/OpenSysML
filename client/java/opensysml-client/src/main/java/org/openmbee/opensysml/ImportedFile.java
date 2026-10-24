package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * What one data import assigned.
 *
 * @param name the import's name, as the request gave it
 * @param values how many values the import set
 * @param elements how many elements the import touched
 * @param assignments the values set, in order
 */
public record ImportedFile(String name, long values, long elements, List<ImportedValue> assignments) {
  /** Validates the components and makes the list immutable. */
  public ImportedFile {
    Objects.requireNonNull(name, "name");
    assignments = List.copyOf(Objects.requireNonNull(assignments, "assignments"));
  }
}
