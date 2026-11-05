namespace OpenMBEE.OpenSysML;

/// <summary>
/// A distribution of numbers by its statistics, as the tool stored them or as the runs produced them. A pooled one,
/// pooled from a tool's summaries, knows its <see cref="Count"/> and <see cref="Mean"/> alone, the others then 0.
/// </summary>
/// <param name="Count">How many numbers the distribution holds.</param>
/// <param name="Min">The smallest number.</param>
/// <param name="Mean">The arithmetic mean.</param>
/// <param name="P50">The median.</param>
/// <param name="P90">The 90th percentile.</param>
/// <param name="Max">The largest number.</param>
/// <param name="Deviation">The standard deviation.</param>
public sealed record ComparedDistribution(long Count, double Min, double Mean, double P50, double P90, double Max, double Deviation);
