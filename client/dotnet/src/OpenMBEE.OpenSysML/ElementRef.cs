using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An element the model's API element form knows: one of the model, or a standard-library element it references.
/// Its <see cref="Id"/> is accepted back by <see cref="Model.ElementPropertiesAsync"/>, so a client walks the element
/// graph with one vocabulary.
/// </summary>
public sealed record ElementRef
{
    /// <summary>Creates an element reference.</summary>
    /// <param name="id">The element form's <c>@id</c>.</param>
    /// <param name="qualifiedName">The qualified name; empty for an unnamed element, and the library path (<c>owner/Metaclass[i]</c>) of an unnamed standard-library member.</param>
    /// <param name="metaclass">The metaclass (<c>PartDefinition</c>, <c>ConnectionUsage</c>, …); empty when the id is known to nothing.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    internal ElementRef(string id, string qualifiedName, string metaclass)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(qualifiedName);
        ArgumentNullException.ThrowIfNull(metaclass);
        Id = id;
        QualifiedName = qualifiedName;
        Metaclass = metaclass;
    }

    /// <summary>The element form's <c>@id</c>, which <see cref="Model.ElementPropertiesAsync"/> accepts back as the element to read.</summary>
    public string Id { get; }

    /// <summary>The qualified name; empty for an unnamed element, and the library path (<c>owner/Metaclass[i]</c>) of an unnamed standard-library member.</summary>
    public string QualifiedName { get; }

    /// <summary>The metaclass (<c>PartDefinition</c>, <c>ConnectionUsage</c>, …); empty when the id is known to nothing.</summary>
    public string Metaclass { get; }

    /// <summary>The element as it is best named: its qualified name, or its id when it has none.</summary>
    /// <returns>The element's text.</returns>
    public override string ToString() => QualifiedName.Length > 0 ? QualifiedName : Id;
}
