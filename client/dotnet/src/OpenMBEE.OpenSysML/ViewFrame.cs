using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The view frame drawn round a graph-shaped rendering (SysML v2 §8.2.3.26): the name compartment's name and view
/// definition, and the lines of the three information compartments.
/// </summary>
public sealed record ViewFrame
{
    /// <summary>Creates a frame, copying its lists.</summary>
    /// <param name="name">The view's name, as the name compartment shows it.</param>
    /// <param name="type">The view definition the view is typed by; empty for an untyped view.</param>
    /// <param name="exposes">The view's expose statements as the notation writes them, drawn top right.</param>
    /// <param name="filters">The view's element filter conditions as written, drawn bottom left.</param>
    /// <param name="annotations">The bodies of the comments and documentation annotating the view, drawn bottom right.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A list holds a null.</exception>
    public ViewFrame(string name, string type, IEnumerable<string> exposes, IEnumerable<string> filters, IEnumerable<string> annotations)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(type);
        Name = name;
        Type = type;
        Exposes = new ValueList<string>(exposes);
        Filters = new ValueList<string>(filters);
        Annotations = new ValueList<string>(annotations);
    }

    /// <summary>The view's name, as the name compartment shows it.</summary>
    public string Name { get; }

    /// <summary>The view definition the view is typed by; empty for an untyped view.</summary>
    public string Type { get; }

    /// <summary>The view's expose statements as the notation writes them, drawn top right.</summary>
    public ValueList<string> Exposes { get; }

    /// <summary>The view's element filter conditions as written, drawn bottom left.</summary>
    public ValueList<string> Filters { get; }

    /// <summary>The bodies of the comments and documentation annotating the view, drawn bottom right.</summary>
    public ValueList<string> Annotations { get; }
}
