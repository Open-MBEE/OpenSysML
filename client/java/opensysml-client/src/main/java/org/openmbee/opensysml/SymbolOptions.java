package org.openmbee.opensysml;

import java.util.Objects;

/**
 * What a symbol lookup asks the service for beyond the symbol itself.
 *
 * @param includeLibraryMembers list the members inherited from standard-library content too,
 *     which the service otherwise leaves out and counts in {@link Symbol#withheldLibraryMembers()}
 * @param byReportedId with {@link Lookup#EITHER} only: take the string as one the service
 *     reported (a symbol's id, a child's, a member's) rather than resolving it from the root
 *     namespace; a declared name still comes first
 * @param lookup what the string names; {@link Lookup#EITHER} is the legacy form
 */
public record SymbolOptions(boolean includeLibraryMembers, boolean byReportedId, Lookup lookup) {
  /**
   * Options of the legacy form, one string for either kind.
   *
   * @param includeLibraryMembers whether to list the inherited standard-library members
   * @param byReportedId whether the string is one the service reported
   */
  public SymbolOptions(boolean includeLibraryMembers, boolean byReportedId) {
    this(includeLibraryMembers, byReportedId, Lookup.EITHER);
  }

  /**
   * Options resolving a qualified name.
   *
   * @param includeLibraryMembers whether to list the inherited standard-library members
   */
  public SymbolOptions(boolean includeLibraryMembers) {
    this(includeLibraryMembers, false, Lookup.EITHER);
  }

  /**
   * The service's default: standard-library members counted, not listed, the id resolved as a
   * qualified name.
   *
   * @return the default options
   */
  public static SymbolOptions defaults() {
    return new SymbolOptions(false, false, Lookup.EITHER);
  }

  /**
   * The same lookup, listing or counting the inherited standard-library members.
   *
   * @param include whether to list them
   * @return options with the choice
   */
  public SymbolOptions withLibraryMembers(boolean include) {
    return new SymbolOptions(include, byReportedId, lookup);
  }

  /**
   * The same lookup, by an id the service reported or by qualified name.
   *
   * @param reported whether the id is one the service reported
   * @return options with the choice
   */
  public SymbolOptions byReportedId(boolean reported) {
    return new SymbolOptions(includeLibraryMembers, reported, lookup);
  }

  /**
   * The same lookup, stating what the string names.
   *
   * @param kind a qualified name, an element id, or either
   * @return options with the choice
   */
  public SymbolOptions lookup(Lookup kind) {
    return new SymbolOptions(
        includeLibraryMembers, byReportedId, Objects.requireNonNull(kind, "kind"));
  }
}
