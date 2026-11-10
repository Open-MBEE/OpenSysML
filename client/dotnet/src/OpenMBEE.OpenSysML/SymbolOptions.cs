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
}
