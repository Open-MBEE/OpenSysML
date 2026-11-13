namespace OpenMBEE.OpenSysML;

/// <summary>A waypoint of an edge's route on a rendering's canvas, as a <c>DiagramLayout::Route</c> gives it.</summary>
public sealed record ViewPoint
{
    internal ViewPoint(double x, double y)
    {
        X = x;
        Y = y;
    }

    /// <summary>The point's horizontal position.</summary>
    public double X { get; }

    /// <summary>The point's vertical position.</summary>
    public double Y { get; }
}
