using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One binding a tool call would expect back.</summary>
public sealed record ToolDryRunOutput
{
    /// <summary>Creates the record.</summary>
    /// <param name="variable">The tool variable it is read from.</param>
    /// <param name="parameter">The parameter it binds.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ToolDryRunOutput(string variable, string parameter)
    {
        ArgumentNullException.ThrowIfNull(variable);
        ArgumentNullException.ThrowIfNull(parameter);
        Variable = variable;
        Parameter = parameter;
    }

    /// <summary>The tool variable it is read from.</summary>
    public string Variable { get; }

    /// <summary>The parameter it binds.</summary>
    public string Parameter { get; }
}
