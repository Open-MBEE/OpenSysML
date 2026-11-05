using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An object the service holds for a model, as a document value or a trace record names it: by the
/// id it was created with and by the label a session reaches it under, with the usage it is held under.
/// </summary>
public sealed record DocumentObject
{
    /// <summary>Creates an object reference.</summary>
    /// <param name="instanceId">The object's id, as an instantiation answers it.</param>
    /// <param name="path">The label a session reaches it under (<c>Garage::car</c>, <c>#2</c>, <c>Garage::car.wheels[2]</c>); empty when none was given.</param>
    /// <param name="element">The usage the object is held under; null when the service reported none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    public DocumentObject(long instanceId, string path, DocumentValue.ElementValue? element)
    {
        ArgumentNullException.ThrowIfNull(path);
        InstanceId = instanceId;
        Path = path;
        Element = element;
    }

    /// <summary>The object's id, as an instantiation answers it.</summary>
    public long InstanceId { get; }

    /// <summary>
    /// The label a session reaches the object under: the qualified name it was instantiated as
    /// (<c>Garage::car</c>), its id (<c>#2</c>), or a path through the feature values of either
    /// (<c>Garage::car.wheels[2]</c>); empty when none was given.
    /// </summary>
    public string Path { get; }

    /// <summary>The usage the object is held under, its own definition or usage for an object an instantiation created; null when the service reported none.</summary>
    public DocumentValue.ElementValue? Element { get; }

    /// <summary>The object as its path, or <c>#2</c> by id when it has none.</summary>
    /// <returns>The object's text.</returns>
    public override string ToString() => Path.Length > 0 ? Path : $"#{Internal.ValueText.Whole(InstanceId)}";
}
