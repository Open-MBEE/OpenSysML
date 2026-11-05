namespace OpenMBEE.OpenSysML;

/// <summary>
/// The least and the greatest model-draw probability of an <see cref="Outcome"/> over the schedulers
/// resolving its scheduling choices: bounds over schedulers, not shares of the schedules enumerated.
/// </summary>
/// <param name="Min">The least probability.</param>
/// <param name="Max">The greatest probability.</param>
public sealed record ProbabilityRange(double Min, double Max);
