using System;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// A property was read whose reference the document does not resolve: an <c>@id</c> the document holds no
/// element for, such as a standard-library element an export refers to without including it, or an
/// <c>@ref</c>, which names an element rather than identifying one. The read of the document succeeds; only
/// reading the property fails, every time it is read.
/// </summary>
public class UnresolvedReferenceException : MetamodelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="property">The property read.</param>
    /// <param name="elementId">The <c>@id</c> of the element it was read from.</param>
    /// <param name="targetId">The <c>@id</c> the reference names; null when it names an <c>@ref</c>.</param>
    /// <param name="targetRef">The <c>@ref</c> the reference names; null when it names an <c>@id</c>.</param>
    /// <exception cref="ArgumentNullException"><paramref name="property"/> or <paramref name="elementId"/> is null.</exception>
    /// <exception cref="ArgumentException">Not exactly one of <paramref name="targetId"/> and <paramref name="targetRef"/> is given.</exception>
    public UnresolvedReferenceException(MetaProperty property, string elementId, string? targetId, string? targetRef)
        : base(Describe(property, elementId, targetId, targetRef))
    {
        Property = property;
        ElementId = elementId;
        TargetId = targetId;
        TargetRef = targetRef;
    }

    /// <summary>The property read.</summary>
    public MetaProperty Property { get; }

    /// <summary>The <c>@id</c> of the element it was read from.</summary>
    public string ElementId { get; }

    /// <summary>The <c>@id</c> the document holds no element for; null when the reference is an <c>@ref</c>.</summary>
    public string? TargetId { get; }

    /// <summary>The <c>@ref</c> the reference names; null when it is an <c>@id</c>.</summary>
    public string? TargetRef { get; }

    private static string Describe(MetaProperty property, string elementId, string? targetId, string? targetRef)
    {
        ArgumentNullException.ThrowIfNull(property);
        ArgumentNullException.ThrowIfNull(elementId);
        if ((targetId is null) == (targetRef is null))
        {
            throw new ArgumentException("an unresolved reference names exactly one @id or @ref", nameof(targetId));
        }

        var target = targetId is not null ? $"@id \"{targetId}\"" : $"@ref \"{targetRef}\"";
        return $"\"{property.JsonKey}\" of element {elementId} refers to unresolved {target}";
    }
}
