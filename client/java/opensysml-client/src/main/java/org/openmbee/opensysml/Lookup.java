package org.openmbee.opensysml;

import org.openmbee.opensysml.proto.LookupKind;

/**
 * What the one string of a lookup names. A qualified name and an id can be spelled alike (a
 * package declared {@code Col__V__c_pend0}, the id generated for an unnamed connector end; a
 * member {@code '@3'} beside the unnamed member {@code Owner::@3}), so a lookup states which.
 */
public enum Lookup {
  /**
   * The legacy form, one string for either kind: a declared name first, then an id, so a
   * declaration shadows the element an id of its spelling was reported for.
   */
  EITHER(LookupKind.LOOKUP_KIND_UNSPECIFIED),
  /** A qualified name, resolved from the root namespace; never read as an id. */
  QUALIFIED_NAME(LookupKind.LOOKUP_KIND_QUALIFIED_NAME),
  /** An id the service reported for an element, whatever its visibility; never read as a name. */
  ELEMENT_ID(LookupKind.LOOKUP_KIND_ELEMENT_ID);

  private final LookupKind kind;

  Lookup(LookupKind kind) {
    this.kind = kind;
  }

  /**
   * The kind as a request states it.
   *
   * @return the wire value
   */
  public LookupKind kind() {
    return kind;
  }

  /**
   * The lookup a request's kind states.
   *
   * @param kind the wire value
   * @return the lookup, {@link #EITHER} for none or an unknown one
   */
  public static Lookup of(LookupKind kind) {
    for (Lookup lookup : values()) {
      if (lookup.kind == kind) {
        return lookup;
      }
    }
    return EITHER;
  }
}
