using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One assignment an import made: the element, the feature set and the literal it is set to, and where in the data it came from.</summary>
public sealed record ImportedValue
{
    /// <summary>Creates an assignment.</summary>
    /// <param name="element">The qualified name of the element whose feature is set.</param>
    /// <param name="feature">The feature set.</param>
    /// <param name="value">The literal it is set to; empty for a redefinition made only to reach a nested inherited feature.</param>
    /// <param name="redefines">The qualified name of the inherited feature redefined to set it; empty when the element declares the feature.</param>
    /// <param name="where">Where in the data the value came from.</param>
    /// <param name="description">The line <c>sysml -import</c> reports the assignment as.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ImportedValue(string element, string feature, string value, string redefines, string where, string description)
    {
        ArgumentNullException.ThrowIfNull(element);
        ArgumentNullException.ThrowIfNull(feature);
        ArgumentNullException.ThrowIfNull(value);
        ArgumentNullException.ThrowIfNull(redefines);
        ArgumentNullException.ThrowIfNull(where);
        ArgumentNullException.ThrowIfNull(description);
        Element = element;
        Feature = feature;
        Value = value;
        Redefines = redefines;
        Where = where;
        Description = description;
    }

    /// <summary>The qualified name of the element whose feature is set.</summary>
    public string Element { get; }

    /// <summary>The feature set.</summary>
    public string Feature { get; }

    /// <summary>The literal it is set to, as notation (<c>7</c>, <c>"fast"</c>); empty for a redefinition made only to reach a nested inherited feature, which sets no value.</summary>
    public string Value { get; }

    /// <summary>The qualified name of the inherited feature redefined to set it; empty when the element declares the feature itself.</summary>
    public string Redefines { get; }

    /// <summary>Where in the data the value came from, as <c>fleet.csv line 2, column count</c>.</summary>
    public string Where { get; }

    /// <summary>The line <c>sysml -import</c> reports the assignment as, as <c>V::car::count = 9</c>.</summary>
    public string Description { get; }

    /// <summary>The assignment as <c>sysml -import</c> reports it.</summary>
    /// <returns><see cref="Description"/>.</returns>
    public override string ToString() => Description;
}
