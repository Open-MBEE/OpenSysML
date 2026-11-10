namespace OpenMBEE.OpenSysML;

/// <summary>
/// The least and the greatest model-draw probability of an <see cref="Outcome"/> over the schedulers
/// resolving its scheduling choices: bounds over schedulers, not shares of the schedules enumerated.
/// </summary>
public sealed record ProbabilityRange
{
    internal ProbabilityRange(double min, double max)
    {
        Min = min;
        Max = max;
    }

    /// <summary>The least probability.</summary>
    public double Min { get; }

    /// <summary>The greatest probability.</summary>
    public double Max { get; }
}
