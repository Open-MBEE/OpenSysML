using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A <c>CoordinateFramePlacement</c>: the target frame's origin in the source frame and, when
/// stated, its basis directions there.
/// </summary>
public sealed record FramePlacement
{
    internal FramePlacement(Value origin, IEnumerable<Value> basisDirections)
    {
        ArgumentNullException.ThrowIfNull(origin);
        Origin = origin;
        BasisDirections = new ValueList<Value>(basisDirections);
    }

    /// <summary>The target's origin in the source frame: a vector quantity, or a quantity for a one-dimensional frame.</summary>
    public Value Origin { get; }

    /// <summary>The target's basis directions in the source frame, one per target axis; empty for the source's own orientation.</summary>
    public ValueList<Value> BasisDirections { get; }
}
