using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>An <c>AffineTransformationMatrix3d</c>: a 3×3 rotation matrix and a translation.</summary>
public sealed record AffineTransformation3D
{
    internal AffineTransformation3D(IEnumerable<double> rotation, IEnumerable<double> translation)
    {
        Rotation = new ValueList<double>(rotation);
        Translation = new ValueList<double>(translation);
    }

    /// <summary>The <c>rotationMatrix</c>: nine numbers, row-major.</summary>
    public ValueList<double> Rotation { get; }

    /// <summary>The <c>translationVector</c>: three numbers.</summary>
    public ValueList<double> Translation { get; }
}
