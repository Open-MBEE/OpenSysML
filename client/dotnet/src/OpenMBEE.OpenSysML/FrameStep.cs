using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One step of a <c>TranslationRotationSequence</c>: a translation when <see cref="Translation"/> is
/// set, otherwise a rotation. Steps are equal by what they do, whatever object each was read from.
/// </summary>
public sealed record FrameStep
{
    internal FrameStep(Value? translation, FrameRotation? rotation, long? instanceId)
    {
        if ((translation is null) == (rotation is null))
        {
            throw new ArgumentException("A step is exactly one of a translation and a rotation.", nameof(translation));
        }

        Translation = translation;
        Rotation = rotation;
        InstanceId = instanceId;
    }

    /// <summary>The translation vector, or null for a rotation.</summary>
    public Value? Translation { get; }

    /// <summary>The rotation, or null for a translation.</summary>
    public FrameRotation? Rotation { get; }

    /// <summary>The id of the <c>Translation</c> or <c>Rotation</c> object the step was read from, or null for none.</summary>
    public long? InstanceId { get; }

    /// <inheritdoc/>
    public bool Equals(FrameStep? other) =>
        other is not null && Equals(Translation, other.Translation) && Equals(Rotation, other.Rotation);

    /// <inheritdoc/>
    public override int GetHashCode() => HashCode.Combine(Translation, Rotation);
}
