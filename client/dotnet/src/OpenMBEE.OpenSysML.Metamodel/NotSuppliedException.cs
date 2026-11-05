using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// A property was read that the document does not supply for the element: the key is absent, as
/// opposed to present with a null value. Derived properties are the usual case, since an export of
/// owned properties omits them; the reader invents no default in their place.
/// </summary>
public class NotSuppliedException : MetamodelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="property">The property read.</param>
    /// <param name="elementId">The <c>@id</c> of the element it was read from.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public NotSuppliedException(MetaProperty property, string elementId)
        : base(Describe(property, elementId))
    {
        Property = property;
        ElementId = elementId;
    }

    /// <summary>The property read.</summary>
    public MetaProperty Property { get; }

    /// <summary>The <c>@id</c> of the element it was read from.</summary>
    public string ElementId { get; }

    /// <summary>Whether the property is derived, and so absent from an export of owned properties.</summary>
    public bool IsDerived => Property.IsDerived;

    private static string Describe(MetaProperty property, string elementId)
    {
        ArgumentNullException.ThrowIfNull(property);
        ArgumentNullException.ThrowIfNull(elementId);
        var message = $"document carries no \"{property.JsonKey}\" for element {elementId}";
        return property.IsDerived ? message + "; owned-only exports omit derived properties" : message;
    }
}
