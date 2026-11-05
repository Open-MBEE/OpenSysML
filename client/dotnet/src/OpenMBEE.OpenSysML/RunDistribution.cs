using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One observable summarised over the completed runs: its extremes, mean, deviation, percentiles and histogram where
/// its values are numbers, or how many runs produced each distinct value where they are not.
/// </summary>
public sealed record RunDistribution
{
    /// <summary>Creates a distribution, copying its collections.</summary>
    /// <param name="name">The observable's name.</param>
    /// <param name="unit">The unit the numbers carry; empty when none, or the values are not numbers.</param>
    /// <param name="count">The completed runs that produced the observable.</param>
    /// <param name="numeric">Whether the values are numbers.</param>
    /// <param name="min">The least value; null when the values are not numbers.</param>
    /// <param name="max">The greatest value; null when the values are not numbers.</param>
    /// <param name="mean">The mean value; null when the values are not numbers.</param>
    /// <param name="deviation">The sample standard deviation of the numbers.</param>
    /// <param name="percentiles">The value at each percentile reported.</param>
    /// <param name="histogram">The histogram's bins, in ascending order.</param>
    /// <param name="values">How many runs produced each distinct value, where the values are not numbers.</param>
    /// <exception cref="ArgumentNullException"><paramref name="name"/>, <paramref name="unit"/> or a collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public RunDistribution(
        string name,
        string unit,
        long count,
        bool numeric,
        Value? min,
        Value? max,
        Value? mean,
        double deviation,
        IEnumerable<RunPercentile> percentiles,
        IEnumerable<RunHistogramBin> histogram,
        IEnumerable<RunCountedValue> values)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(unit);
        Name = name;
        Unit = unit;
        Count = count;
        Numeric = numeric;
        Min = min;
        Max = max;
        Mean = mean;
        Deviation = deviation;
        Percentiles = new ValueList<RunPercentile>(percentiles);
        Histogram = new ValueList<RunHistogramBin>(histogram);
        Values = new ValueList<RunCountedValue>(values);
    }

    /// <summary>The observable's name: a feature of the action, <c>clock</c>, or <c>observed</c> for a Monte Carlo case.</summary>
    public string Name { get; }

    /// <summary>The unit the numbers carry; empty when none, or the values are not numbers.</summary>
    public string Unit { get; }

    /// <summary>The completed runs that produced the observable.</summary>
    public long Count { get; }

    /// <summary>
    /// Whether the values are numbers, so <see cref="Min"/>, <see cref="Max"/>, <see cref="Mean"/>,
    /// <see cref="Deviation"/>, <see cref="Percentiles"/> and <see cref="Histogram"/> are reported; otherwise
    /// <see cref="Values"/> counts each distinct value.
    /// </summary>
    public bool Numeric { get; }

    /// <summary>The least value; null when the values are not numbers.</summary>
    public Value? Min { get; }

    /// <summary>The greatest value; null when the values are not numbers.</summary>
    public Value? Max { get; }

    /// <summary>The mean value; null when the values are not numbers.</summary>
    public Value? Mean { get; }

    /// <summary>The sample standard deviation of the numbers.</summary>
    public double Deviation { get; }

    /// <summary>The value at each percentile reported, in the order reported.</summary>
    public ValueList<RunPercentile> Percentiles { get; }

    /// <summary>The histogram's bins, in ascending order.</summary>
    public ValueList<RunHistogramBin> Histogram { get; }

    /// <summary>How many runs produced each distinct value, where the values are not numbers.</summary>
    public ValueList<RunCountedValue> Values { get; }
}
