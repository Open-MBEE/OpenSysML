using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>The row-major shape rules an array and a tensor share.</summary>
internal static class Shape
{
    /// <summary>How many elements dimensions hold, refusing a dimension that is not positive or a product beyond <see cref="long"/>.</summary>
    internal static long Size(ValueList<long> dimensions, string what)
    {
        var size = 1L;
        for (var i = 0; i < dimensions.Count; i++)
        {
            if (dimensions[i] <= 0)
            {
                throw new ArgumentException($"{what} dimension {i} is {dimensions[i]}, not positive.", nameof(dimensions));
            }

            try
            {
                size = checked(size * dimensions[i]);
            }
            catch (OverflowException overflow)
            {
                throw new ArgumentException($"{what} dimensions {dimensions} hold more than {long.MaxValue} elements.", nameof(dimensions), overflow);
            }
        }

        return size;
    }

    /// <summary>Refuses elements that do not fill dimensions exactly.</summary>
    internal static void RequireFilled(ValueList<long> dimensions, int count, string what, string elements)
    {
        var size = Size(dimensions, what);
        if (size != count)
        {
            throw new ArgumentException($"{what} of dimensions {dimensions} holds {count} {elements}, not {size}.", elements);
        }
    }

    /// <summary>The row-major offset of a multi-index, one coordinate per dimension.</summary>
    internal static int Offset(ValueList<long> dimensions, ReadOnlySpan<long> index, string what)
    {
        if (index.Length != dimensions.Count)
        {
            throw new ArgumentException($"The index has {index.Length} coordinate(s); the {what} has rank {dimensions.Count}.", nameof(index));
        }

        var flat = 0L;
        for (var i = 0; i < index.Length; i++)
        {
            if (index[i] < 0 || index[i] >= dimensions[i])
            {
                throw new ArgumentOutOfRangeException(nameof(index), index[i], $"Coordinate {i} is outside dimension {i} of extent {dimensions[i]}.");
            }

            flat = (flat * dimensions[i]) + index[i];
        }

        return (int)flat;
    }

    /// <summary>The one unit every quantity is written in, or null when they differ or there are none.</summary>
    internal static string? SharedUnit(IEnumerable<Quantity> quantities)
    {
        string? shared = null;
        foreach (var quantity in quantities)
        {
            if (shared is null)
            {
                shared = quantity.Unit;
            }
            else if (!string.Equals(shared, quantity.Unit, StringComparison.Ordinal))
            {
                return null;
            }
        }

        return shared;
    }
}
