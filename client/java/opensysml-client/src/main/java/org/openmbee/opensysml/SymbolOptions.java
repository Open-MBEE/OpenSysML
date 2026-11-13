package org.openmbee.opensysml;

/**
 * What a symbol lookup asks the service for beyond the symbol itself.
 *
 * @param includeLibraryMembers list the members inherited from standard-library content too,
 *     which the service otherwise leaves out and counts in {@link Symbol#withheldLibraryMembers()}
 * @param byReportedId take the id as one the service reported (a symbol's id, a child's, a
 *     member's, a specialization target's) and answer with that element whatever its visibility,
 *     an alias being itself, instead of resolving it as a qualified name from the root namespace
 */
public record SymbolOptions(boolean includeLibraryMembers, boolean byReportedId) {
  /**
   * Options resolving a qualified name.
   *
   * @param includeLibraryMembers whether to list the inherited standard-library members
   */
  public SymbolOptions(boolean includeLibraryMembers) {
    this(includeLibraryMembers, false);
  }

  /**
   * The service's default: standard-library members counted, not listed, the id resolved as a
   * qualified name.
   *
   * @return the default options
   */
  public static SymbolOptions defaults() {
    return new SymbolOptions(false, false);
  }

  /**
   * The same lookup, listing or counting the inherited standard-library members.
   *
   * @param include whether to list them
   * @return options with the choice
   */
  public SymbolOptions withLibraryMembers(boolean include) {
    return new SymbolOptions(include, byReportedId);
  }

  /**
   * The same lookup, by an id the service reported or by qualified name.
   *
   * @param reported whether the id is one the service reported
   * @return options with the choice
   */
  public SymbolOptions byReportedId(boolean reported) {
    return new SymbolOptions(includeLibraryMembers, reported);
  }
}
