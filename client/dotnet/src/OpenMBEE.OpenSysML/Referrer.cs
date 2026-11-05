using System;

namespace OpenMBEE.OpenSysML;

/// <summary>A declaration referring to the target of a refused rename, delete or move, with the document declaring it.</summary>
public sealed record Referrer
{
    /// <summary>Creates a referrer.</summary>
    /// <param name="name">The declaration as the notation names it: the qualified name of a named one, or the heading of an anonymous one within its namespace.</param>
    /// <param name="document">The document declaring it, named as the parse named it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public Referrer(string name, string document)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(document);
        Name = name;
        Document = document;
    }

    /// <summary>The declaration as the notation names it: the qualified name of a named one, or the heading of an anonymous one within its namespace.</summary>
    public string Name { get; }

    /// <summary>The document declaring it, named as the parse named it.</summary>
    public string Document { get; }
}
