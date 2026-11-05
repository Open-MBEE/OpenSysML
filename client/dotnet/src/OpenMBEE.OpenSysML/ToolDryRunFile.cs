using System;

namespace OpenMBEE.OpenSysML;

/// <summary>The input file a tool dry run would write.</summary>
public sealed record ToolDryRunFile
{
    /// <summary>Creates the record.</summary>
    /// <param name="name">The file's name.</param>
    /// <param name="format">Its format: <c>json</c> or <c>csv</c>.</param>
    /// <param name="data">What it would hold.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ToolDryRunFile(string name, string format, string data)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(format);
        ArgumentNullException.ThrowIfNull(data);
        Name = name;
        Format = format;
        Data = data;
    }

    /// <summary>The file's name.</summary>
    public string Name { get; }

    /// <summary>Its format: <c>json</c> or <c>csv</c>.</summary>
    public string Format { get; }

    /// <summary>What it would hold.</summary>
    public string Data { get; }
}
