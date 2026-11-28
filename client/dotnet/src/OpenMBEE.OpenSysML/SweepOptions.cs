using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.RunSweepAsync"/> runs a sweep: an analysis case's subject, the values every row binds, how
/// many rows to draw and the seed they are drawn from, and the engine asked. Collections are copied when they are
/// set. A negative number of samples is refused by the service in its answer, and an engine it does not know with
/// <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class SweepOptions
{
    private readonly ValueList<Value> arguments;
    private readonly ValueDictionary<Value> namedArguments;

    /// <summary>Stepping through every range, binding nothing else, by the engine the service chooses.</summary>
    public static SweepOptions Default { get; } = new();

    /// <summary>
    /// An analysis case's subject, as <see cref="AnalysisOptions.SubjectSymbolId"/> names one, built anew for every
    /// row; null leaves the case's own binding to supply it. A calc has none.
    /// </summary>
    public string? SubjectSymbolId { get; init; }

    /// <summary>Values for the target's input parameters, in declaration order, as every row binds them.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public IReadOnlyList<Value> Arguments
    {
        get => arguments;
        init => arguments = new ValueList<Value>(value);
    }

    /// <summary>Values for the target's input parameters, by name, as every row binds them.</summary>
    /// <exception cref="ArgumentNullException">The dictionary is null.</exception>
    /// <exception cref="ArgumentException">A name or a value is null.</exception>
    public IReadOnlyDictionary<string, Value> NamedArguments
    {
        get => namedArguments;
        init => namedArguments = new ValueDictionary<Value>(value);
    }

    /// <summary>
    /// How many rows to draw uniformly from the ranges instead of stepping through them; 0 steps through them. Drawing
    /// needs a <see cref="Seed"/> and refuses a range with a step.
    /// </summary>
    public long Samples { get; init; }

    /// <summary>The seed rows are drawn from: the same seed draws the same table on every platform. Read only with <see cref="Samples"/>.</summary>
    public ulong Seed { get; init; }

    /// <summary>
    /// The engine the sweep is put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to the
    /// service. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }
}
