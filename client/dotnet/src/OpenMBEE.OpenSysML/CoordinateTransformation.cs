using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A coordinate transformation held as a value: the frames it relates, the shape the model states it
/// in and the pose that shape resolves to. The service reports it and never accepts it.
/// </summary>
/// <remarks>
/// <para>At most one of <see cref="Placement"/>, <see cref="Steps"/> and <see cref="Affine"/> is set; a
/// transformation of no library shape carries its frames alone.</para>
/// <para>Two transformations are equal as the service compares them: one object read twice, else equal
/// source, target and stated shape; a transformation of no shape is only the object it was read from.</para>
/// </remarks>
public sealed class CoordinateTransformation : IEquatable<CoordinateTransformation>
{
    internal CoordinateTransformation(
        string name,
        string? transformationId,
        string? typeId,
        long? instanceId,
        CoordinateFrame? source,
        CoordinateFrame? target,
        FramePlacement? placement,
        IEnumerable<FrameStep>? steps,
        AffineTransformation3D? affine,
        FramePose? pose)
    {
        ArgumentNullException.ThrowIfNull(name);
        Name = name;
        TransformationId = transformationId;
        TypeId = typeId;
        InstanceId = instanceId;
        Source = source;
        Target = target;
        Placement = placement;
        Steps = steps is null ? null : new ValueList<FrameStep>(steps);
        Affine = affine;
        Pose = pose;
    }

    /// <summary>The transformation as a reader names it: its declared name, else its type.</summary>
    public string Name { get; }

    /// <summary>The qualified name of the usage declaring the transformation, or null for none.</summary>
    public string? TransformationId { get; }

    /// <summary>The qualified name of the transformation's type (<c>MeasurementReferences::TranslationRotationSequence</c>), or null for none.</summary>
    public string? TypeId { get; }

    /// <summary>The id of the object the transformation was read from, or null for none.</summary>
    public long? InstanceId { get; }

    /// <summary>The frame transformed from, or null when the model states none.</summary>
    public CoordinateFrame? Source { get; }

    /// <summary>The frame transformed to; null when the model states none, and inside the frame whose transformation this is.</summary>
    public CoordinateFrame? Target { get; }

    /// <summary>The shape of a <c>CoordinateFramePlacement</c>, or null for another shape.</summary>
    public FramePlacement? Placement { get; }

    /// <summary>The elements of a <c>TranslationRotationSequence</c>, in the order applied, or null for another shape.</summary>
    public ValueList<FrameStep>? Steps { get; }

    /// <summary>The shape of an <c>AffineTransformationMatrix3d</c>, or null for another shape.</summary>
    public AffineTransformation3D? Affine { get; }

    /// <summary>Where the target sits in the source; null when no source is stated or the shape resolves to none.</summary>
    public FramePose? Pose { get; }

    /// <inheritdoc/>
    public bool Equals(CoordinateTransformation? other)
    {
        if (other is null)
        {
            return false;
        }

        var sameObject = InstanceId == other.InstanceId && string.Equals(TransformationId, other.TransformationId, StringComparison.Ordinal);
        if (sameObject && InstanceId is not null)
        {
            return true;
        }

        if (!Equals(Source, other.Source) || !Equals(Target, other.Target))
        {
            return false;
        }

        return Placement is null && Steps is null && Affine is null && other.Placement is null && other.Steps is null && other.Affine is null
            ? sameObject
            : Equals(Placement, other.Placement) && Steps == other.Steps && Equals(Affine, other.Affine);
    }

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as CoordinateTransformation);

    /// <summary>One hash for every transformation: one object read twice is equal whatever else differs, so no part may enter it.</summary>
    /// <returns>The hash.</returns>
    public override int GetHashCode() => typeof(CoordinateTransformation).GetHashCode();

    /// <summary>The transformation over the frames it relates, as <c>trs (datum → lbcf)</c>, with <c>?</c> for a frame not stated.</summary>
    /// <returns>The transformation's text.</returns>
    public override string ToString() => $"{Name} ({Source?.Name ?? "?"} → {Target?.Name ?? "?"})";
}
