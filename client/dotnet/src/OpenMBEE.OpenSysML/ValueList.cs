using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An immutable list compared by its elements in order: two lists are equal, and hash alike, when
/// they hold equal elements in the same order. It copies what it is built from, so changing that
/// afterwards changes nothing here, and <c>default</c> is the empty list.
/// </summary>
/// <typeparam name="T">The element type.</typeparam>
public readonly struct ValueList<T> : IReadOnlyList<T>, IEquatable<ValueList<T>>
{
    private readonly T[]? items;

    /// <summary>Creates a list holding a copy of the given elements, in order.</summary>
    /// <param name="items">The elements; none may be null.</param>
    /// <exception cref="ArgumentNullException"><paramref name="items"/> is null.</exception>
    /// <exception cref="ArgumentException">An element is null.</exception>
    public ValueList(IEnumerable<T> items)
    {
        ArgumentNullException.ThrowIfNull(items);
        var copy = items.ToArray();
        for (var i = 0; i < copy.Length; i++)
        {
            if (copy[i] is null)
            {
                throw new ArgumentException($"Element {i} is null; a list holds no null element.", nameof(items));
            }
        }

        this.items = copy.Length == 0 ? null : copy;
    }

    /// <summary>How many elements the list holds.</summary>
    public int Count => items?.Length ?? 0;

    /// <summary>Whether the list holds no element.</summary>
    public bool IsEmpty => items is null;

    /// <summary>The element at an index.</summary>
    /// <param name="index">The index, from zero.</param>
    /// <exception cref="ArgumentOutOfRangeException">The index is outside the list.</exception>
    public T this[int index]
    {
        get
        {
            ArgumentOutOfRangeException.ThrowIfNegative(index);
            ArgumentOutOfRangeException.ThrowIfGreaterThanOrEqual(index, Count);
            return items![index];
        }
    }

    /// <summary>The elements, read-only, without copying them.</summary>
    /// <returns>A read-only span over the elements.</returns>
    public ReadOnlySpan<T> AsSpan() => items;

    /// <inheritdoc/>
    public IEnumerator<T> GetEnumerator() => ((IEnumerable<T>)(items ?? [])).GetEnumerator();

    /// <inheritdoc/>
    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();

    /// <inheritdoc/>
    public bool Equals(ValueList<T> other) => AsSpan().SequenceEqual(other.AsSpan(), EqualityComparer<T>.Default);

    /// <inheritdoc/>
    public override bool Equals(object? obj) => obj is ValueList<T> other && Equals(other);

    /// <inheritdoc/>
    public override int GetHashCode()
    {
        var hash = new HashCode();
        hash.Add(Count);
        foreach (var item in AsSpan())
        {
            hash.Add(item);
        }

        return hash.ToHashCode();
    }

    /// <summary>The elements in order, as <c>[a, b]</c>, each culture-invariant.</summary>
    /// <returns>The list's text.</returns>
    public override string ToString() => $"[{ValueText.Join(items ?? [])}]";

    /// <summary>Whether two lists hold equal elements in the same order.</summary>
    /// <param name="left">A list.</param>
    /// <param name="right">Another list.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(ValueList<T> left, ValueList<T> right) => left.Equals(right);

    /// <summary>Whether two lists differ.</summary>
    /// <param name="left">A list.</param>
    /// <param name="right">Another list.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(ValueList<T> left, ValueList<T> right) => !left.Equals(right);
}
