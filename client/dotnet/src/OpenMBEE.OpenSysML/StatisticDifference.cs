using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The relative difference of each statistic of the runs' distribution from the tool's, as a signed percentage
/// (<c>-20.0%</c>), empty where it is not compared: a pooled tool distribution compares the mean alone.
/// </summary>
public sealed record StatisticDifference
{
    /// <summary>Creates the differences.</summary>
    /// <param name="min">The difference of the minimum.</param>
    /// <param name="mean">The difference of the mean.</param>
    /// <param name="p50">The difference of the median.</param>
    /// <param name="p90">The difference of the 90th percentile.</param>
    /// <param name="max">The difference of the maximum.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public StatisticDifference(string min, string mean, string p50, string p90, string max)
    {
        ArgumentNullException.ThrowIfNull(min);
        ArgumentNullException.ThrowIfNull(mean);
        ArgumentNullException.ThrowIfNull(p50);
        ArgumentNullException.ThrowIfNull(p90);
        ArgumentNullException.ThrowIfNull(max);
        Min = min;
        Mean = mean;
        P50 = p50;
        P90 = p90;
        Max = max;
    }

    /// <summary>The difference of the minimum; empty where it is not compared.</summary>
    public string Min { get; }

    /// <summary>The difference of the mean; empty where it is not compared.</summary>
    public string Mean { get; }

    /// <summary>The difference of the median; empty where it is not compared.</summary>
    public string P50 { get; }

    /// <summary>The difference of the 90th percentile; empty where it is not compared.</summary>
    public string P90 { get; }

    /// <summary>The difference of the maximum; empty where it is not compared.</summary>
    public string Max { get; }
}
