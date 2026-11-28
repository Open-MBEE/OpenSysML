using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.RecordRunAsync"/> runs an analysis case and where it records the runs, as
/// <c>sysml -record-run</c> does: once with neither <see cref="Ranges"/> nor <see cref="Runs"/>, once per row of a sweep
/// with <see cref="Ranges"/>, or as a Monte Carlo sample with <see cref="Runs"/>. Collections are copied when they are
/// set. Both ranges and runs, a negative number of runs, and runs without a seed are refused by the service with
/// <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class RecordOptions
{
    private readonly ValueList<Value> arguments;
    private readonly ValueDictionary<Value> namedArguments;
    private readonly ValueList<SweepRange> ranges;

    /// <summary>One run of the case, on its own subject and parameter defaults, recorded into the default package.</summary>
    public static RecordOptions Default { get; } = new();

    /// <summary>
    /// The case's subject, as <see cref="AnalysisOptions.SubjectSymbolId"/> names one, built anew for every run; null
    /// leaves the case's own <c>subject</c> binding to supply it.
    /// </summary>
    public string? SubjectSymbolId { get; init; }

    /// <summary>Values for the case's <c>in</c> parameters, in declaration order, as every run binds them.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public IReadOnlyList<Value> Arguments
    {
        get => arguments;
        init => arguments = new ValueList<Value>(value);
    }

    /// <summary>Values for the case's <c>in</c> parameters, by name, as every run binds them.</summary>
    /// <exception cref="ArgumentNullException">The dictionary is null.</exception>
    /// <exception cref="ArgumentException">A name or a value is null.</exception>
    public IReadOnlyDictionary<string, Value> NamedArguments
    {
        get => namedArguments;
        init => namedArguments = new ValueDictionary<Value>(value);
    }

    /// <summary>
    /// The engine the runs are put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to the
    /// service. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }

    /// <summary>
    /// The qualified name of the package the records go into (<c>sysml -record-into</c>); null for a <c>Records</c>
    /// package beside the package enclosing the case's own.
    /// </summary>
    public string? RecordInto { get; init; }

    /// <summary>
    /// The ranges of a sweep (<c>sysml -sweep</c>), whose rows are run as <see cref="Model.RunSweepAsync"/> runs them and
    /// each completed one recorded; empty records no sweep.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A range is null.</exception>
    public IReadOnlyList<SweepRange> Ranges
    {
        get => ranges;
        init => ranges = new ValueList<SweepRange>(value);
    }

    /// <summary>
    /// How many runs of a Monte Carlo sample to make (<c>sysml -runs</c>), as <see cref="Model.RunAnalysisRunsAsync"/>
    /// makes them, recording each completed run and the sample's conclusion; 0 records no sample. Needs a <see cref="Seed"/>.
    /// </summary>
    public long Runs { get; init; }

    /// <summary>The seed a sample's draws are taken from; read only with <see cref="Runs"/>, which requires it. Null sends none.</summary>
    public ulong? Seed { get; init; }

    /// <summary>How the case is run: once, over the rows of a sweep, or as a sample, as the service reads the options.</summary>
    internal RecordMode Mode => Ranges.Count > 0 ? RecordMode.Sweep : Runs > 0 ? RecordMode.Sample : RecordMode.Run;
}

/// <summary>How a recording runs its case, which decides the one answer of the runs it carries.</summary>
internal enum RecordMode
{
    /// <summary>Once, answered as a run of the case.</summary>
    Run,

    /// <summary>Once per row of a sweep, answered as its table.</summary>
    Sweep,

    /// <summary>As a Monte Carlo sample, answered as its runs.</summary>
    Sample,
}
