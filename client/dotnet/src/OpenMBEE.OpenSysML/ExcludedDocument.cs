using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A document <see cref="Model.ElementPropertiesAsync"/> read the model without: one the parser could not read
/// whole, one the element form cannot write, or one declaring an element an earlier document declares.
/// </summary>
public sealed record ExcludedDocument
{
    internal ExcludedDocument(string document, string error)
    {
        ArgumentNullException.ThrowIfNull(document);
        ArgumentNullException.ThrowIfNull(error);
        Document = document;
        Error = error;
    }

    /// <summary>The document's name, as the parse gave it.</summary>
    public string Document { get; }

    /// <summary>Why the document is left out.</summary>
    public string Error { get; }
}
