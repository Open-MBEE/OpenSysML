using System;

namespace OpenMBEE.OpenSysML;

/// <summary>A feature drawn on a node's border, which an edge may end at.</summary>
public sealed record ViewPort
{
    /// <summary>Creates a port.</summary>
    /// <param name="id">The port's id within the rendering.</param>
    /// <param name="name">The port's name.</param>
    /// <param name="type">The port's declared type, as the notation writes it; empty when untyped.</param>
    /// <param name="direction">The port's direction, as the service spells it.</param>
    /// <param name="inherited">Whether the node's owner has the port from its type rather than declares it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ViewPort(string id, string name, string type, string direction, bool inherited)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(type);
        ArgumentNullException.ThrowIfNull(direction);
        Id = id;
        Name = name;
        Type = type;
        Direction = direction;
        Inherited = inherited;
    }

    /// <summary>The port's id within the rendering, which <see cref="ViewEdge.FromPort"/> and <see cref="ViewEdge.ToPort"/> name.</summary>
    public string Id { get; }

    /// <summary>The port's name.</summary>
    public string Name { get; }

    /// <summary>The port's declared type, as the notation writes it; empty when untyped.</summary>
    public string Type { get; }

    /// <summary>The port's direction (<c>in</c>, <c>out</c>, <c>inout</c>), as the service spells it; empty for none.</summary>
    public string Direction { get; }

    /// <summary>
    /// Whether the node's owner has the port from its type rather than declares it, which the notation marks with
    /// <c>^</c> before the name. <see cref="Name"/> stays the bare name.
    /// </summary>
    public bool Inherited { get; }
}
