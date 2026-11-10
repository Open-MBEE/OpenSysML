using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One derived type of a feature, as <see cref="TypeFacts.Types"/> lists them.</summary>
public sealed record TypeRef
{
    /// <summary>Creates a type reference.</summary>
    /// <param name="id">The type's qualified name.</param>
    /// <param name="kind">The type's symbol kind (<c>partDef</c>, <c>attributeDef</c>, …); empty when the service reported none.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public TypeRef(string id, string kind)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(kind);
        Id = id;
        Kind = kind;
    }

    /// <summary>The type's qualified name.</summary>
    public string Id { get; }

    /// <summary>The type's symbol kind (<c>partDef</c>, <c>attributeDef</c>, …); empty when the service reported none.</summary>
    public string Kind { get; }

    /// <summary>The type, as its qualified name.</summary>
    /// <returns>The type's text.</returns>
    public override string ToString() => Id;
}
