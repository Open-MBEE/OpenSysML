using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// A document is not an element graph: not JSON, not an element object, an array of them or an envelope
/// holding them, or holding an element without a string <c>@id</c> or <c>@type</c>, two elements with one
/// <c>@id</c>, invalid UTF-8, an undecodable Unicode property name or element identity, an object with a
/// key twice, or an <c>@type</c> whose supertypes form a cycle.
/// </summary>
public class MalformedDocumentException : MetamodelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">What is wrong with the document.</param>
    public MalformedDocumentException(string message)
        : base(message)
    {
    }

    /// <summary>Creates the exception with a cause.</summary>
    /// <param name="message">What is wrong with the document.</param>
    /// <param name="innerException">The underlying failure, such as the JSON parser's.</param>
    public MalformedDocumentException(string message, Exception? innerException)
        : base(message, innerException)
    {
    }
}
