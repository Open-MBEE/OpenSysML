using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// The base of every failure the metamodel types report. It is not an <c>OpenSysMLException</c>:
/// the metamodel assembly stands alone, without the client library.
/// </summary>
public class MetamodelException : Exception
{
    /// <summary>Creates an exception.</summary>
    public MetamodelException()
    {
    }

    /// <summary>Creates an exception.</summary>
    /// <param name="message">What failed.</param>
    public MetamodelException(string message)
        : base(message)
    {
    }

    /// <summary>Creates an exception with a cause.</summary>
    /// <param name="message">What failed.</param>
    /// <param name="innerException">The underlying failure.</param>
    public MetamodelException(string message, Exception? innerException)
        : base(message, innerException)
    {
    }
}
