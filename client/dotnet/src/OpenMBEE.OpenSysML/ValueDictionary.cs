using System;
using System.Collections;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An immutable dictionary from names to values, compared by its entries whatever order it was built in:
/// two dictionaries are equal, and hash alike, when they hold the same names, compared ordinally, with
/// equal values. It enumerates its entries in ordinal order of their names, so it reads and
/// renders the same however it was built. It copies what it is built from, and <c>default</c> is
/// the empty dictionary.
/// </summary>
/// <typeparam name="TValue">The value type.</typeparam>
public readonly struct ValueDictionary<TValue> : IReadOnlyDictionary<string, TValue>, IEquatable<ValueDictionary<TValue>>
{
    private readonly KeyValuePair<string, TValue>[]? entries;

    /// <summary>Creates a dictionary holding a copy of the given entries.</summary>
    /// <param name="entries">The entries; no name may repeat, and no name or value may be null.</param>
    /// <exception cref="ArgumentNullException"><paramref name="entries"/> is null.</exception>
    /// <exception cref="ArgumentException">A name repeats, or a name or a value is null.</exception>
    public ValueDictionary(IEnumerable<KeyValuePair<string, TValue>> entries)
    {
        ArgumentNullException.ThrowIfNull(entries);
        var copy = entries.ToArray();
        foreach (var entry in copy)
        {
            if (entry.Key is null)
            {
                throw new ArgumentException("A name is null; a dictionary holds no null name.", nameof(entries));
            }

            if (entry.Value is null)
            {
                throw new ArgumentException($"The value of {entry.Key} is null; a dictionary holds no null value.", nameof(entries));
            }
        }

        Array.Sort(copy, (a, b) => string.CompareOrdinal(a.Key, b.Key));
        for (var i = 1; i < copy.Length; i++)
        {
            if (string.Equals(copy[i - 1].Key, copy[i].Key, StringComparison.Ordinal))
            {
                throw new ArgumentException($"The name {copy[i].Key} repeats; a dictionary holds each name once.", nameof(entries));
            }
        }

        this.entries = copy.Length == 0 ? null : copy;
    }

    /// <summary>How many entries the dictionary holds.</summary>
    public int Count => entries?.Length ?? 0;

    /// <summary>Whether the dictionary holds no entry.</summary>
    public bool IsEmpty => entries is null;

    /// <summary>The names, in ordinal order.</summary>
    public IEnumerable<string> Keys => Entries.Select(entry => entry.Key);

    /// <summary>The values, in the ordinal order of their names.</summary>
    public IEnumerable<TValue> Values => Entries.Select(entry => entry.Value);

    private KeyValuePair<string, TValue>[] Entries => entries ?? [];

    /// <summary>The value of a name.</summary>
    /// <param name="key">The name.</param>
    /// <exception cref="KeyNotFoundException">The dictionary holds no such name.</exception>
    public TValue this[string key] =>
        TryGetValue(key, out var value) ? value : throw new KeyNotFoundException($"The dictionary holds no entry named {key}.");

    /// <inheritdoc/>
    public bool ContainsKey(string key) => IndexOf(key) >= 0;

    /// <inheritdoc/>
    public bool TryGetValue(string key, [MaybeNullWhen(false)] out TValue value)
    {
        var index = IndexOf(key);
        value = index >= 0 ? entries![index].Value : default;
        return index >= 0;
    }

    /// <inheritdoc/>
    public IEnumerator<KeyValuePair<string, TValue>> GetEnumerator() => ((IEnumerable<KeyValuePair<string, TValue>>)Entries).GetEnumerator();

    /// <inheritdoc/>
    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();

    /// <inheritdoc/>
    public bool Equals(ValueDictionary<TValue> other)
    {
        if (Count != other.Count)
        {
            return false;
        }

        var comparer = EqualityComparer<TValue>.Default;
        for (var i = 0; i < Count; i++)
        {
            var (mine, theirs) = (entries![i], other.entries![i]);
            if (!string.Equals(mine.Key, theirs.Key, StringComparison.Ordinal) || !comparer.Equals(mine.Value, theirs.Value))
            {
                return false;
            }
        }

        return true;
    }

    /// <inheritdoc/>
    public override bool Equals(object? obj) => obj is ValueDictionary<TValue> other && Equals(other);

    /// <inheritdoc/>
    public override int GetHashCode()
    {
        var hash = new HashCode();
        hash.Add(Count);
        foreach (var (key, value) in Entries)
        {
            hash.Add(key, StringComparer.Ordinal);
            hash.Add(value);
        }

        return hash.ToHashCode();
    }

    /// <summary>The entries in ordinal order of their names, as <c>{a: 1, b: 2}</c>, each value culture-invariant.</summary>
    /// <returns>The dictionary's text.</returns>
    public override string ToString() =>
        "{" + string.Join(", ", Entries.Select(entry => $"{entry.Key}: {ValueText.Item(entry.Value)}")) + "}";

    /// <summary>Whether two dictionaries hold the same names with equal values.</summary>
    /// <param name="left">A dictionary.</param>
    /// <param name="right">Another dictionary.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(ValueDictionary<TValue> left, ValueDictionary<TValue> right) => left.Equals(right);

    /// <summary>Whether two dictionaries differ.</summary>
    /// <param name="left">A dictionary.</param>
    /// <param name="right">Another dictionary.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(ValueDictionary<TValue> left, ValueDictionary<TValue> right) => !left.Equals(right);

    private int IndexOf(string key)
    {
        ArgumentNullException.ThrowIfNull(key);
        var (low, high) = (0, Count - 1);
        while (low <= high)
        {
            var middle = low + ((high - low) / 2);
            var order = string.CompareOrdinal(entries![middle].Key, key);
            if (order == 0)
            {
                return middle;
            }

            (low, high) = order < 0 ? (middle + 1, high) : (low, middle - 1);
        }

        return -1;
    }
}
