using System;

namespace OpenMBEE.OpenSysML;

/// <summary>The value at one percentile of the completed runs of a numeric observable.</summary>
public sealed record RunPercentile
{
    /// <summary>Creates a percentile.</summary>
    /// <param name="percentile">The percentile, as <c>50</c> for the median.</param>
    /// <param name="value">The observable's value at it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
    public RunPercentile(double percentile, Value value)
    {
        ArgumentNullException.ThrowIfNull(value);
        Percentile = percentile;
        Value = value;
    }

    /// <summary>The percentile, as <c>50</c> for the median.</summary>
    public double Percentile { get; }

    /// <summary>The observable's value at it.</summary>
    public Value Value { get; }
}
