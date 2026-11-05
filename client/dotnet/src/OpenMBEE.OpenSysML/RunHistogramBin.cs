using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One bin of a numeric observable's histogram: how many completed runs fell in [<see cref="Low"/>, <see cref="High"/>].</summary>
public sealed record RunHistogramBin
{
    /// <summary>Creates a bin.</summary>
    /// <param name="low">The bin's lower edge.</param>
    /// <param name="high">The bin's upper edge; equal to <paramref name="low"/> for a single distinct value.</param>
    /// <param name="count">The completed runs whose value fell in the bin.</param>
    /// <exception cref="ArgumentNullException"><paramref name="low"/> or <paramref name="high"/> is null.</exception>
    public RunHistogramBin(Value low, Value high, long count)
    {
        ArgumentNullException.ThrowIfNull(low);
        ArgumentNullException.ThrowIfNull(high);
        Low = low;
        High = high;
        Count = count;
    }

    /// <summary>The bin's lower edge.</summary>
    public Value Low { get; }

    /// <summary>The bin's upper edge; equal to <see cref="Low"/> for a single distinct value.</summary>
    public Value High { get; }

    /// <summary>The completed runs whose value fell in the bin.</summary>
    public long Count { get; }
}
