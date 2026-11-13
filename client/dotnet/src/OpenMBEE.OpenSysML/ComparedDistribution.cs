namespace OpenMBEE.OpenSysML;

/// <summary>
/// A distribution of numbers by its statistics, as the tool stored them or as the runs produced them. A pooled one,
/// pooled from a tool's summaries, knows its <see cref="Count"/> and <see cref="Mean"/> alone, the others then 0.
/// </summary>
public sealed record ComparedDistribution
{
    internal ComparedDistribution(long count, double min, double mean, double p50, double p90, double max, double deviation)
    {
        Count = count;
        Min = min;
        Mean = mean;
        P50 = p50;
        P90 = p90;
        Max = max;
        Deviation = deviation;
    }

    /// <summary>How many numbers the distribution holds.</summary>
    public long Count { get; }

    /// <summary>The smallest number.</summary>
    public double Min { get; }

    /// <summary>The arithmetic mean.</summary>
    public double Mean { get; }

    /// <summary>The median.</summary>
    public double P50 { get; }

    /// <summary>The 90th percentile.</summary>
    public double P90 { get; }

    /// <summary>The largest number.</summary>
    public double Max { get; }

    /// <summary>The standard deviation.</summary>
    public double Deviation { get; }
}
