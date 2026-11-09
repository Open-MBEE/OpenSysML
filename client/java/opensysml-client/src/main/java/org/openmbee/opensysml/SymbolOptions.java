package org.openmbee.opensysml;

/**
 * What a symbol lookup asks the service for beyond the symbol itself.
 *
 * @param includeLibraryMembers list the members inherited from standard-library content too,
 *     which the service otherwise leaves out and counts in {@link Symbol#withheldLibraryMembers()}
 */
public record SymbolOptions(boolean includeLibraryMembers) {
  /**
   * The service's default: standard-library members counted, not listed.
   *
   * @return the default options
   */
  public static SymbolOptions defaults() {
    return new SymbolOptions(false);
  }

  /**
   * The same lookup, listing or counting the inherited standard-library members.
   *
   * @param include whether to list them
   * @return options with the choice
   */
  public SymbolOptions withLibraryMembers(boolean include) {
    return new SymbolOptions(include);
  }
}
