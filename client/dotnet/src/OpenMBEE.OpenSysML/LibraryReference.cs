using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A standard-library element an <c>api-json</c> conversion refers to, listed beside the element array rather than
/// written into it as a root.
/// </summary>
public sealed record LibraryReference
{
    /// <summary>Creates a reference.</summary>
    /// <param name="id">The element's id, as the element array's references name it.</param>
    /// <param name="qualifiedName">The element's qualified name in the standard library; empty for an unnamed member.</param>
    /// <param name="type">The element's metaclass, as the API's JSON <c>@type</c> spells it.</param>
    /// <param name="libraryPath">The positional path <c>owner/Metaclass[i]</c> of an unnamed member, which has no qualified name; empty for a named element.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public LibraryReference(string id, string qualifiedName, string type, string libraryPath)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(qualifiedName);
        ArgumentNullException.ThrowIfNull(type);
        ArgumentNullException.ThrowIfNull(libraryPath);
        Id = id;
        QualifiedName = qualifiedName;
        Type = type;
        LibraryPath = libraryPath;
    }

    /// <summary>The element's id, as the element array's references name it.</summary>
    public string Id { get; }

    /// <summary>The element's qualified name in the standard library; empty for an unnamed member, which <see cref="LibraryPath"/> names.</summary>
    public string QualifiedName { get; }

    /// <summary>The element's metaclass, as the API's JSON <c>@type</c> spells it.</summary>
    public string Type { get; }

    /// <summary>The positional path <c>owner/Metaclass[i]</c> of an unnamed member, which has no qualified name; empty for a named element.</summary>
    public string LibraryPath { get; }

    /// <summary>The element as a reader names it: its qualified name, or an unnamed member's library path, and metaclass.</summary>
    /// <returns>The qualified name or library path, then the metaclass in parentheses.</returns>
    public override string ToString() => $"{(QualifiedName.Length > 0 ? QualifiedName : LibraryPath)} ({Type})";
}
