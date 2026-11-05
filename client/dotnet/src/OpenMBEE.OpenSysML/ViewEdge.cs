using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>One edge of a view rendering, joining two of its nodes and possibly their ports.</summary>
public sealed record ViewEdge
{
    /// <summary>Creates an edge.</summary>
    /// <param name="from">The <see cref="ViewNode.Id"/> the edge leaves.</param>
    /// <param name="to">The <see cref="ViewNode.Id"/> the edge reaches.</param>
    /// <param name="fromPort">The <see cref="ViewPort.Id"/> it leaves from; empty for the node itself.</param>
    /// <param name="toPort">The <see cref="ViewPort.Id"/> it reaches; empty for the node itself.</param>
    /// <param name="label">The text the edge carries; empty for none.</param>
    /// <param name="name">The edge's own name; empty for one anonymous.</param>
    /// <param name="kind">What the edge is (<c>connection</c>, <c>transition</c>, <c>succession</c>).</param>
    /// <param name="origin">Where it is declared; null when the service located it nowhere.</param>
    /// <param name="route">The waypoints it follows, in order; copied.</param>
    /// <param name="style">How a style draws it; null when no style colours it.</param>
    /// <exception cref="ArgumentNullException">An argument other than the origin or style is null.</exception>
    /// <exception cref="ArgumentException">A waypoint is null.</exception>
    public ViewEdge(
        string from,
        string to,
        string fromPort,
        string toPort,
        string label,
        string name,
        string kind,
        SourceSpan? origin,
        IEnumerable<ViewPoint> route,
        ViewStyle? style)
    {
        ArgumentNullException.ThrowIfNull(from);
        ArgumentNullException.ThrowIfNull(to);
        ArgumentNullException.ThrowIfNull(fromPort);
        ArgumentNullException.ThrowIfNull(toPort);
        ArgumentNullException.ThrowIfNull(label);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(kind);
        From = from;
        To = to;
        FromPort = fromPort;
        ToPort = toPort;
        Label = label;
        Name = name;
        Kind = kind;
        Origin = origin;
        Route = new ValueList<ViewPoint>(route);
        Style = style;
    }

    /// <summary>The <see cref="ViewNode.Id"/> the edge leaves.</summary>
    public string From { get; }

    /// <summary>The <see cref="ViewNode.Id"/> the edge reaches.</summary>
    public string To { get; }

    /// <summary>The <see cref="ViewPort.Id"/> of the port of <see cref="From"/> the edge leaves from; empty for the node itself.</summary>
    public string FromPort { get; }

    /// <summary>The <see cref="ViewPort.Id"/> of the port of <see cref="To"/> the edge reaches; empty for the node itself.</summary>
    public string ToPort { get; }

    /// <summary>What the edge carries: a connector's name, a transition's trigger, guard and effect, a succession's guard; empty for none.</summary>
    public string Label { get; }

    /// <summary>The edge's own name; empty for one anonymous or named by a migration.</summary>
    public string Name { get; }

    /// <summary>What the edge is (<c>connection</c>, <c>transition</c>, <c>succession</c>, <c>flow</c>), as the service spells it.</summary>
    public string Kind { get; }

    /// <summary>Where the edge is declared; null when the service located it nowhere.</summary>
    public SourceSpan? Origin { get; }

    /// <summary>The waypoints the edge follows; empty when no route gives any.</summary>
    public ValueList<ViewPoint> Route { get; }

    /// <summary>How a style draws the edge; null when no style colours it.</summary>
    public ViewStyle? Style { get; }
}
