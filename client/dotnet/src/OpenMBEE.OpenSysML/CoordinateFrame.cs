using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A coordinate frame or measurement scale held as a value: what a feature typed by a
/// <c>MeasurementReferences::CoordinateFrame</c> evaluates to. The service reports it and never accepts it.
/// </summary>
/// <remarks>
/// Two frames are equal as the service compares them: the object and usage they were read from when
/// either has an object, otherwise their dimensions, axes, scale and transformation, whatever names them.
/// </remarks>
public sealed class CoordinateFrame : IEquatable<CoordinateFrame>
{
    internal CoordinateFrame(
        string name,
        string? frameId,
        string? typeId,
        long? instanceId,
        IEnumerable<long> dimensions,
        IEnumerable<Value.MeasurementRefValue> axes,
        CoordinateTransformation? transformation,
        MeasurementScale? scale)
    {
        ArgumentNullException.ThrowIfNull(name);
        Name = name;
        FrameId = frameId;
        TypeId = typeId;
        InstanceId = instanceId;
        Dimensions = new ValueList<long>(dimensions);
        Axes = new ValueList<Value.MeasurementRefValue>(axes);
        Transformation = transformation;
        Scale = scale;
    }

    /// <summary>The frame as a reader names it: its declared name, else the expression that composed it (<c>datum / s</c>).</summary>
    public string Name { get; }

    /// <summary>The qualified name of the usage declaring the frame (<c>Frames::datum</c>), or null for one composed.</summary>
    public string? FrameId { get; }

    /// <summary>The qualified name of the frame's type, or null for none.</summary>
    public string? TypeId { get; }

    /// <summary>The id of the object the frame was read from, or null for none.</summary>
    public long? InstanceId { get; }

    /// <summary>The frame's <c>dimensions</c>: <c>[3]</c> for a three-dimensional frame, empty for a scale.</summary>
    public ValueList<long> Dimensions { get; }

    /// <summary>The frame's <c>mRefs</c>, one unit per axis.</summary>
    public ValueList<Value.MeasurementRefValue> Axes { get; }

    /// <summary>What places the frame in another, or null when the model states none. Its target is null: this frame is its target.</summary>
    public CoordinateTransformation? Transformation { get; }

    /// <summary>What a measurement scale states beyond its unit; null for a frame that is no scale.</summary>
    public MeasurementScale? Scale { get; }

    /// <inheritdoc/>
    public bool Equals(CoordinateFrame? other)
    {
        if (other is null)
        {
            return false;
        }

        if (InstanceId is not null || other.InstanceId is not null)
        {
            return InstanceId == other.InstanceId && string.Equals(FrameId, other.FrameId, StringComparison.Ordinal);
        }

        return Dimensions == other.Dimensions
            && Axes == other.Axes
            && Equals(Scale, other.Scale)
            && Equals(Transformation, other.Transformation);
    }

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as CoordinateFrame);

    /// <inheritdoc/>
    public override int GetHashCode() =>
        InstanceId is { } instance
            ? HashCode.Combine(instance, FrameId is null ? 0 : StringComparer.Ordinal.GetHashCode(FrameId))
            : HashCode.Combine(Dimensions, Axes, Scale);

    /// <summary>The frame over its axis units, as <c>datum [mm, mm, mm]</c>; a scale over its one unit.</summary>
    /// <returns>The frame's text.</returns>
    public override string ToString() =>
        $"{Name} [{(Scale is { } scale ? UnitText(scale.Unit) : string.Join(", ", Axes.Select(UnitText)))}]";

    // MeasurementRefValue writes its unit in brackets; an axis is the unit alone.
    private static string UnitText(Value.MeasurementRefValue reference) => reference.ToString()[1..^1];
}
