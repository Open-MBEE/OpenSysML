using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>One metamodel property of an element, as <see cref="Model.ElementPropertiesAsync"/> answers it.</summary>
public sealed record ElementProperty
{
    /// <summary>Creates a property, copying its values.</summary>
    /// <param name="name">The specification name (<c>connectorEnd</c>).</param>
    /// <param name="definingClass">The metaclass declaring the property (<c>Connector</c>); empty when the element's metaclass declares no such property.</param>
    /// <param name="isMany">Whether the property holds a collection; a single-valued property has at most one value.</param>
    /// <param name="isDerived">Whether the specification derives the property rather than stating it.</param>
    /// <param name="values">The values, in the collection's order; none for an empty or null property.</param>
    /// <param name="unsupported">Why there are no values when the property cannot be answered; empty for a property that is answered.</param>
    /// <exception cref="ArgumentNullException">A string or <paramref name="values"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="values"/> holds a null.</exception>
    public ElementProperty(string name, string definingClass, bool isMany, bool isDerived, IEnumerable<ElementPropertyValue> values, string unsupported)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(definingClass);
        ArgumentNullException.ThrowIfNull(unsupported);
        Name = name;
        DefiningClass = definingClass;
        IsMany = isMany;
        IsDerived = isDerived;
        Values = new ValueList<ElementPropertyValue>(values);
        Unsupported = unsupported;
    }

    /// <summary>The specification name (<c>connectorEnd</c>).</summary>
    public string Name { get; }

    /// <summary>The metaclass declaring the property (<c>Connector</c>); empty when the element's metaclass declares no such property.</summary>
    public string DefiningClass { get; }

    /// <summary>Whether the property holds a collection; a single-valued property has at most one value.</summary>
    public bool IsMany { get; }

    /// <summary>Whether the specification derives the property rather than stating it.</summary>
    public bool IsDerived { get; }

    /// <summary>The values, in the collection's order; none for an empty or null property.</summary>
    public ValueList<ElementPropertyValue> Values { get; }

    /// <summary>
    /// Why there are no values when the property cannot be answered: the element's metaclass declares no such
    /// property, or OpenSysML does not derive it yet, and why. Empty for a property that is answered, with or without values.
    /// </summary>
    public string Unsupported { get; }

    /// <summary>Whether the property was answered, with or without values.</summary>
    public bool IsSupported => Unsupported.Length == 0;

    /// <summary>The one value of a single-valued property; null when it has none.</summary>
    public ElementPropertyValue? Value => Values.IsEmpty ? null : Values[0];

    /// <summary>The elements among the values, in order.</summary>
    public ValueList<ElementRef> Elements => new(Values.OfType<ElementPropertyValue.ElementValue>().Select(value => value.Element));

    /// <summary>The property, as <c>connectorEnd = [Demo::Pair::u1, Demo::Pair::u2]</c>, or why it has no values.</summary>
    /// <returns>The property's text.</returns>
    public override string ToString() =>
        IsSupported ? $"{Name} = [{string.Join(", ", Values)}]" : $"{Name}: {Unsupported}";
}
