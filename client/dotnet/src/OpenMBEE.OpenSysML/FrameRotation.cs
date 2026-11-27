using System;

namespace OpenMBEE.OpenSysML;

/// <summary>A turn by an angle about an axis, one step of a <c>TranslationRotationSequence</c>.</summary>
public sealed record FrameRotation
{
    internal FrameRotation(Value axisDirection, Quantity angle, bool isIntrinsic)
    {
        ArgumentNullException.ThrowIfNull(axisDirection);
        ArgumentNullException.ThrowIfNull(angle);
        AxisDirection = axisDirection;
        Angle = angle;
        IsIntrinsic = isIntrinsic;
    }

    /// <summary>The axis turned about.</summary>
    public Value AxisDirection { get; }

    /// <summary>The angle turned by.</summary>
    public Quantity Angle { get; }

    /// <summary>Whether the turn is about the axes as already rotated, rather than about the source's fixed axes.</summary>
    public bool IsIntrinsic { get; }
}
