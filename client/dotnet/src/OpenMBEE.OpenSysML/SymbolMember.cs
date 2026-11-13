using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One member of a symbol, own or inherited, as a service advertising <see cref="Capabilities.SymbolMembers"/>
/// enumerates it in <see cref="Symbol.Members"/>, with where it comes from.
/// </summary>
public sealed record SymbolMember
{
    /// <summary>Creates a member.</summary>
    /// <param name="id">The member's qualified name, the id <see cref="Model.SymbolAsync"/> takes; a member declared without a name has none that resolves.</param>
    /// <param name="name">The declared name; empty for an unnamed member such as <c>bind a = b;</c>.</param>
    /// <param name="kind">The member's symbol kind (<c>partUsage</c>, <c>bindingUsage</c>, …).</param>
    /// <param name="isInherited">Whether the member is inherited rather than declared by the symbol.</param>
    /// <param name="inheritedFrom">The qualified name of the type declaring an inherited member; empty for an own member.</param>
    /// <param name="isLibrary">Whether the member is declared in standard-library content.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    internal SymbolMember(string id, string name, string kind, bool isInherited, string inheritedFrom, bool isLibrary)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(inheritedFrom);
        Id = id;
        Name = name;
        Kind = kind;
        IsInherited = isInherited;
        InheritedFrom = inheritedFrom;
        IsLibrary = isLibrary;
    }

    /// <summary>The member's qualified name, the id <see cref="Model.SymbolAsync"/> takes; a member declared without a name has none that resolves.</summary>
    public string Id { get; }

    /// <summary>The declared name; empty for an unnamed member such as <c>bind a = b;</c>.</summary>
    public string Name { get; }

    /// <summary>The member's symbol kind (<c>partUsage</c>, <c>bindingUsage</c>, …).</summary>
    public string Kind { get; }

    /// <summary>Whether the member is inherited rather than declared by the symbol.</summary>
    public bool IsInherited { get; }

    /// <summary>The qualified name of the type declaring an inherited member; empty for an own member.</summary>
    public string InheritedFrom { get; }

    /// <summary>Whether the member is declared in standard-library content.</summary>
    public bool IsLibrary { get; }

    /// <summary>The member, as <c>Demo::Base::p (partUsage)</c>.</summary>
    /// <returns>The member's text.</returns>
    public override string ToString() => $"{(Id.Length == 0 ? Name : Id)} ({Kind})";
}
