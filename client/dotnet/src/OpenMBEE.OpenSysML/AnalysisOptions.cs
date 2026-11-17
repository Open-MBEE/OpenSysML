using System;
using System.Collections.Generic;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.RunAnalysisAsync"/> and <see cref="Model.ExploreAnalysisAsync"/> run an analysis case: its
/// subject, the values bound to its <c>in</c> parameters, the schedule its choice points are resolved under, the
/// engine asked, and whether to preview the external tool it calls instead. Collections are copied when they are
/// set. A schedule or an engine the service cannot read is refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class AnalysisOptions
{
    private readonly ValueList<Value> arguments;
    private readonly ValueDictionary<Value> namedArguments;

    /// <summary>The case's own subject and parameter defaults, the service's default schedule and engine.</summary>
    public static AnalysisOptions Default { get; } = new();

    /// <summary>
    /// The case's subject: the qualified name of a part definition or usage the service builds an object of, or a path
    /// from one to a nested object (<c>Mission::mission.vehicle</c>), every explored run its own; null leaves the case's
    /// own <c>subject</c> binding to supply it.
    /// </summary>
    public string? SubjectSymbolId { get; init; }

    /// <summary>Values for the case's <c>in</c> parameters, in declaration order; the subject is never among them.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public IReadOnlyList<Value> Arguments
    {
        get => arguments;
        init => arguments = new ValueList<Value>(value);
    }

    /// <summary>Values for the case's <c>in</c> parameters, by name.</summary>
    /// <exception cref="ArgumentNullException">The dictionary is null.</exception>
    /// <exception cref="ArgumentException">A name or a value is null.</exception>
    public IReadOnlyDictionary<string, Value> NamedArguments
    {
        get => namedArguments;
        init => namedArguments = new ValueDictionary<Value>(value);
    }

    /// <summary>
    /// The policy the run resolves its choice points under, as <see cref="RunOptions.Schedule"/> spells it: one order
    /// for <see cref="Model.RunAnalysisAsync"/>, an exploring one for <see cref="Model.ExploreAnalysisAsync"/>; null
    /// leaves it to the call. Naming one needs <see cref="Capabilities.Schedule"/>, and an exploring one
    /// <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Schedule { get; init; }

    /// <summary>
    /// The engine the case is put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to the
    /// service. <c>explore</c> asks what an exploring schedule does, so only <see cref="Model.ExploreAnalysisAsync"/>
    /// takes it. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }

    /// <summary>
    /// Whether to run the case only to the first call of an external tool and answer what the tool would be given,
    /// in <see cref="Analysis.ToolDryRun"/>, without starting it (<c>sysml -tool-dry-run</c>); nothing the run did is
    /// kept. A dry run previews one run, so an exploration takes none.
    /// </summary>
    public bool ToolDryRun { get; init; }

    /// <summary>
    /// Whether the case is explored rather than run once, as the service reads the options: <see cref="Schedule"/> is
    /// <c>explore</c> or <c>explore:…</c>, or <see cref="Engine"/> names the <c>explore</c> engine.
    /// </summary>
    public bool Explores => Schedule is { } schedule && RunOptions.IsExploring(schedule) || EngineExplores;

    /// <summary>Whether the engine selects the explore engine, as the service reads its name.</summary>
    internal bool EngineExplores => Engines.Explores(Engine);
}
