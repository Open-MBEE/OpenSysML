using System;

namespace OpenMBEE.OpenSysML.Typed;

/// <summary>
/// The base class of generated typed classes: a view over one <see cref="OpenSysML.Instance"/>, read through
/// the generated interfaces it implements. It holds no state but the instance and its instantiation, so a
/// view is as immutable, and as safe to share between threads, as they are.
/// </summary>
public abstract class TypedObject : ITypedObject, IEquatable<TypedObject>
{
    /// <summary>Creates a view.</summary>
    /// <param name="instance">The instance to view.</param>
    /// <param name="instantiation">The instantiation the instance was built by, which nested objects are resolved through; null when there is none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="instance"/> is null.</exception>
    protected TypedObject(Instance instance, Instantiation? instantiation)
    {
        ArgumentNullException.ThrowIfNull(instance);
        Instance = instance;
        Instantiation = instantiation;
    }

    /// <inheritdoc/>
    public Instance Instance { get; }

    /// <inheritdoc/>
    public Instantiation? Instantiation { get; }

    /// <summary>Whether another view is of the same generated class over an equal instance.</summary>
    /// <param name="other">The other view.</param>
    /// <returns>Whether the two are equal.</returns>
    public bool Equals(TypedObject? other) =>
        other is not null && other.GetType() == GetType() && other.Instance.Equals(Instance);

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as TypedObject);

    /// <inheritdoc/>
    public override int GetHashCode() => HashCode.Combine(GetType(), Instance.Id);

    /// <summary>The view, as <c>Vehicle(Demo::Vehicle#1)</c>.</summary>
    /// <returns>The view's text.</returns>
    public override string ToString() => $"{GetType().Name}({Instance})";
}
