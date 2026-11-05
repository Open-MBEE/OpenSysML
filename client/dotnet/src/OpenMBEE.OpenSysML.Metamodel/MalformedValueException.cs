using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// A property was read whose value the document supplies does not fit it: the wrong JSON kind, a literal the
/// enumeration lacks, a number out of range, an undecodable Unicode string, a null in an array, or a reference to an element outside the
/// property's metaclass. The read of the document succeeds; only reading the property fails, every time it is
/// read. With <see cref="ReadOptions.CheckRanges"/> off an out-of-range reference fails only the property's
/// typed read, and <see cref="IMetamodelElement.GetValue"/> returns it.
/// </summary>
public class MalformedValueException : MetamodelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="property">The property read.</param>
    /// <param name="elementId">The <c>@id</c> of the element it was read from.</param>
    /// <param name="reason">What is wrong with the value.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public MalformedValueException(MetaProperty property, string elementId, string reason)
        : base(Describe(property, elementId, reason))
    {
        Property = property;
        ElementId = elementId;
    }

    /// <summary>The property read.</summary>
    public MetaProperty Property { get; }

    /// <summary>The <c>@id</c> of the element it was read from.</summary>
    public string ElementId { get; }

    private static string Describe(MetaProperty property, string elementId, string reason)
    {
        ArgumentNullException.ThrowIfNull(property);
        ArgumentNullException.ThrowIfNull(elementId);
        ArgumentNullException.ThrowIfNull(reason);
        return $"\"{property.JsonKey}\" of element {elementId}: {reason}";
    }
}
