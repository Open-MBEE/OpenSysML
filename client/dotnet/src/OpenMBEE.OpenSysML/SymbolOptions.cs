namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a symbol lookup (<see cref="Model.SymbolAsync"/>, <see cref="Model.FindSymbolAsync"/>) asks the service for
/// beyond the symbol itself.
/// </summary>
public sealed class SymbolOptions
{
    /// <summary>The service's default: the members inherited from standard-library content counted, not listed.</summary>
    public static SymbolOptions Default { get; } = new();

    /// <summary>
    /// Whether <see cref="Symbol.Members"/> lists the members inherited from standard-library content too, each marked
    /// <see cref="SymbolMember.IsLibrary"/>, instead of counting them in <see cref="Symbol.WithheldLibraryMembers"/>. It
    /// needs <see cref="Capabilities.SymbolMembers"/>, without which a service would ignore it.
    /// </summary>
    public bool IncludeLibraryMembers { get; init; }

    /// <summary>
    /// What the string names: a qualified name, an element id the service reported, or either (the legacy form, a declared
    /// name first). A service without <see cref="Capabilities.SymbolLookupKind"/> is asked in the legacy form, a qualified
    /// name resolved as <see cref="ResolveName"/> resolves one.
    /// </summary>
    public Lookup Lookup { get; init; }

    /// <summary>
    /// With <see cref="Lookup.Either"/>: resolve the string as a qualified name from the root namespace (KerML 8.2.3.5),
    /// through visible memberships and to an alias's element, rather than as an id the service reported. A service without
    /// <see cref="Capabilities.SymbolNameResolution"/> is asked by reported id.
    /// </summary>
    public bool ResolveName { get; init; }
}
