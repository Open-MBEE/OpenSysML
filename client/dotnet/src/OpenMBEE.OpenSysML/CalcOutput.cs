using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One named value a calculation, an analysis case or a sweep row reports, in the order the service reported it.</summary>
public sealed record CalcOutput
{
    /// <summary>Creates an output.</summary>
    /// <param name="name">The output feature or parameter's name; <c>result</c> for a value returned into an unnamed result.</param>
    /// <param name="value">Its value.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public CalcOutput(string name, Value value)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(value);
        Name = name;
        Value = value;
    }

    /// <summary>The output feature or parameter's name; <c>result</c> for a value returned into an unnamed result.</summary>
    public string Name { get; }

    /// <summary>Its value.</summary>
    public Value Value { get; }

    /// <summary>The output, as <c>total = 12.0</c>.</summary>
    /// <returns>The output's text.</returns>
    public override string ToString() => $"{Name} = {Value}";
}
