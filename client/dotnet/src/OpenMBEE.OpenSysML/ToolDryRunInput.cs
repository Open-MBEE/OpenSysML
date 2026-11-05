using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One value a tool call would send: the tool variable it is sent under, the parameter it came from, and the value as the protocol spells it.</summary>
public sealed record ToolDryRunInput
{
    /// <summary>Creates the record.</summary>
    /// <param name="variable">The tool variable it is sent under.</param>
    /// <param name="parameter">The parameter it came from.</param>
    /// <param name="value">The value as the protocol spells it.</param>
    /// <param name="unit">The unit it is sent in; empty when none.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ToolDryRunInput(string variable, string parameter, string value, string unit)
    {
        ArgumentNullException.ThrowIfNull(variable);
        ArgumentNullException.ThrowIfNull(parameter);
        ArgumentNullException.ThrowIfNull(value);
        ArgumentNullException.ThrowIfNull(unit);
        Variable = variable;
        Parameter = parameter;
        Value = value;
        Unit = unit;
    }

    /// <summary>The tool variable it is sent under.</summary>
    public string Variable { get; }

    /// <summary>The parameter it came from.</summary>
    public string Parameter { get; }

    /// <summary>The value as the protocol spells it.</summary>
    public string Value { get; }

    /// <summary>The unit it is sent in; empty when none.</summary>
    public string Unit { get; }
}
