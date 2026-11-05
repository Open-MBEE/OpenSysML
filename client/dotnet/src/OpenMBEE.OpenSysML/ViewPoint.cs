namespace OpenMBEE.OpenSysML;

/// <summary>A waypoint of an edge's route on a rendering's canvas, as a <c>DiagramLayout::Route</c> gives it.</summary>
/// <param name="X">The point's horizontal position.</param>
/// <param name="Y">The point's vertical position.</param>
public sealed record ViewPoint(double X, double Y);
