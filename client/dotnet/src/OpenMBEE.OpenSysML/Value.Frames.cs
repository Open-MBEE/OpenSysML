using System;
using System.Diagnostics.CodeAnalysis;

namespace OpenMBEE.OpenSysML;

/// <content>The coordinate frame arms, which the service reports and never accepts.</content>
public abstract partial class Value
{
    /// <summary>A coordinate frame or measurement scale held as a value. Never an argument.</summary>
    public sealed class CoordinateFrameValue : Value
    {
        internal CoordinateFrameValue(CoordinateFrame frame)
        {
            ArgumentNullException.ThrowIfNull(frame);
            Frame = frame;
        }

        /// <summary>The frame.</summary>
        public CoordinateFrame Frame { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.CoordinateFrame;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is CoordinateFrameValue that && Frame.Equals(that.Frame);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Frame);

        /// <summary>The frame over its axis units, as <c>datum [mm, mm, mm]</c>.</summary>
        /// <returns>The frame's text.</returns>
        public override string ToString() => Frame.ToString();
    }

    /// <summary>A coordinate transformation held as a value. Never an argument.</summary>
    public sealed class CoordinateTransformationValue : Value
    {
        internal CoordinateTransformationValue(CoordinateTransformation transformation)
        {
            ArgumentNullException.ThrowIfNull(transformation);
            Transformation = transformation;
        }

        /// <summary>The transformation.</summary>
        public CoordinateTransformation Transformation { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.CoordinateTransformation;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is CoordinateTransformationValue that && Transformation.Equals(that.Transformation);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Transformation);

        /// <summary>The transformation over the frames it relates, as <c>trs (datum → lbcf)</c>.</summary>
        /// <returns>The transformation's text.</returns>
        public override string ToString() => Transformation.ToString();
    }
}
