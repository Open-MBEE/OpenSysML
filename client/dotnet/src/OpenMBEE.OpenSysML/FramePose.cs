using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Where a transformation's target frame sits in its source frame, in the source's axis units: the
/// point with target coordinates <c>v</c> lies at <c>Origin + Basis · v</c>.
/// </summary>
public sealed record FramePose
{
    internal FramePose(IEnumerable<double> origin, IEnumerable<double> basis)
    {
        Origin = new ValueList<double>(origin);
        Basis = new ValueList<double>(basis);
    }

    /// <summary>The target's origin, one number per source axis.</summary>
    public ValueList<double> Origin { get; }

    /// <summary>The n×n basis over the source's n axes, row-major: column j is the direction of target axis j in the source.</summary>
    public ValueList<double> Basis { get; }
}
