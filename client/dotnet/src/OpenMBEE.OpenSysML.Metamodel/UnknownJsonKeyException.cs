using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// A JSON key was looked up that no property of the metaclass has. What a document writes under such a key
/// is in <see cref="IMetamodelElement.Extensions"/> when it is read with <see cref="ReadOptions.KeepExtensions"/>.
/// </summary>
public class UnknownJsonKeyException : MetamodelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="metaclass">The metaclass the key was looked up in.</param>
    /// <param name="jsonKey">The key.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public UnknownJsonKeyException(MetaclassInfo metaclass, string jsonKey)
        : base(Describe(metaclass, jsonKey))
    {
        Metaclass = metaclass;
        JsonKey = jsonKey;
    }

    /// <summary>The metaclass the key was looked up in.</summary>
    public MetaclassInfo Metaclass { get; }

    /// <summary>The key.</summary>
    public string JsonKey { get; }

    private static string Describe(MetaclassInfo metaclass, string jsonKey)
    {
        ArgumentNullException.ThrowIfNull(metaclass);
        ArgumentNullException.ThrowIfNull(jsonKey);
        return $"{metaclass.Name} has no property with JSON key \"{jsonKey}\"";
    }
}
