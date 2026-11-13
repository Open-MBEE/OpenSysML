using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What one <see cref="Model.ElementPropertiesAsync"/> read answered: the element, and its metamodel properties by
/// their specification names (<c>inheritedFeature</c>, <c>connectorEnd</c>, …) as the model's API element form spells
/// them, in the order asked for, or the element form's order when none was named.
/// </summary>
public sealed record ElementProperties
{
    /// <summary>Creates a read, copying its properties.</summary>
    /// <param name="element">The element read, whose <see cref="ElementRef.Id"/> names it again.</param>
    /// <param name="properties">One entry per property, answered or not.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="properties"/> holds a null.</exception>
    internal ElementProperties(ElementRef element, IEnumerable<ElementProperty> properties)
    {
        ArgumentNullException.ThrowIfNull(element);
        Element = element;
        Properties = new ValueList<ElementProperty>(properties);
    }

    /// <summary>The element read, whose <see cref="ElementRef.Id"/> names it again.</summary>
    public ElementRef Element { get; }

    /// <summary>One entry per property, answered or not, in the order asked for, or the element form's order when none was named.</summary>
    public ValueList<ElementProperty> Properties { get; }

    /// <summary>The entry for one property.</summary>
    /// <param name="name">The specification name.</param>
    /// <returns>The first entry of that name; null when the read did not ask for it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    public ElementProperty? Find(string name)
    {
        ArgumentNullException.ThrowIfNull(name);
        foreach (var property in Properties)
        {
            if (property.Name == name)
            {
                return property;
            }
        }

        return null;
    }

    /// <summary>The read, as <c>Demo::Pair (PartDefinition): inheritedFeature, isAbstract</c>.</summary>
    /// <returns>The read's text.</returns>
    public override string ToString()
    {
        var element = Element.Metaclass.Length > 0 ? $"{Element} ({Element.Metaclass})" : Element.ToString();
        return Properties.IsEmpty ? element : $"{element}: {string.Join(", ", Properties.Select(property => property.Name))}";
    }
}
