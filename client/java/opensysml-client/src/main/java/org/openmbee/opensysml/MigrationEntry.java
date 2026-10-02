package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One SysML v1 element's verdict in a migration.
 *
 * @param id the element's {@code xmi:id}
 * @param kind its v1 metaclass, with its applied stereotypes
 * @param name its qualified name in the v1 model
 * @param target the v2 element it was written as; empty when it was not written
 * @param verdict {@code "mapped"}, {@code "approximated"}, {@code "unmapped"} or {@code "skipped"}
 * @param note why the verdict is what it is, in the migrator's words; empty when nothing needs
 *     saying
 */
public record MigrationEntry(
    String id, String kind, String name, String target, String verdict, String note) {

  /**
   * Creates an entry.
   *
   * @param id the element's id, never {@code null}
   * @param kind its kind, never {@code null}
   * @param name its name, never {@code null}
   * @param target its v2 form, never {@code null}
   * @param verdict its verdict, never {@code null}
   * @param note the note, never {@code null}
   */
  public MigrationEntry {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(target, "target");
    Objects.requireNonNull(verdict, "verdict");
    Objects.requireNonNull(note, "note");
  }
}
