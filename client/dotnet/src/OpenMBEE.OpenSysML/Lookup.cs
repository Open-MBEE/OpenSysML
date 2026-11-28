namespace OpenMBEE.OpenSysML;

/// <summary>
/// What the one string of a lookup names. A qualified name and an id can be spelled alike (a package declared
/// <c>Col__V__c_pend0</c>, the id generated for an unnamed connector end; a member <c>'@3'</c> beside the unnamed
/// member <c>Owner::@3</c>), so a lookup states which. A service without <see cref="Capabilities.SymbolLookupKind"/>
/// reads the string as either, whatever is stated.
/// </summary>
public enum Lookup
{
    /// <summary>The legacy form, one string for either kind: a declared name first, then an id.</summary>
    Either,

    /// <summary>A qualified name, resolved from the root namespace through visible memberships; never read as an id.</summary>
    QualifiedName,

    /// <summary>An id the service reported for an element (<see cref="Symbol.Id"/>, a child id, a member's), whatever its visibility; never read as a name.</summary>
    ElementId,
}
