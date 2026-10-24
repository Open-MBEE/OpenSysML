package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One feature value a data import set.
 *
 * @param element the qualified name of the element the row addressed
 * @param feature the feature the value was set on
 * @param value the value, as SysML notation
 * @param redefines the inherited feature the assignment redefines, {@code ""} for a declared one
 * @param where the document and position the assignment was written at
 * @param description the assignment as the REPL reports it
 */
public record ImportedValue(
    String element, String feature, String value, String redefines, String where, String description) {
  /** Validates the components. */
  public ImportedValue {
    Objects.requireNonNull(element, "element");
    Objects.requireNonNull(feature, "feature");
    Objects.requireNonNull(value, "value");
    Objects.requireNonNull(redefines, "redefines");
    Objects.requireNonNull(where, "where");
    Objects.requireNonNull(description, "description");
  }
}
