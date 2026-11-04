using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.ExecuteActionAsync"/>, <see cref="Model.ExecuteStateAsync"/> and their
/// exploring counterparts run a behavior: the scheduling policy, the object performing it, the
/// trace, events, and the run's clock and draws, each as the <c>sysml</c> command's flag of the same
/// name sets it. Values outside what the service accepts (an unknown draw policy, a negative advance or
/// clock step, a malformed schedule) are refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class RunOptions
{
    private readonly ValueList<string> events = new([]);

    /// <summary>The scheduling policy that explores every order (<c>sysml -schedule explore</c>), with its default budget.</summary>
    public const string Explore = "explore";

    /// <summary>The service's defaults: its default schedule, outside any object, without a trace, to completion.</summary>
    public static RunOptions Default { get; } = new();

    /// <summary>
    /// The policy the run resolves its choice points under (<c>sysml -schedule</c>): <c>declared</c>,
    /// <c>reverse</c> or <c>seed:&lt;n&gt;</c> for one run, <c>explore</c> or
    /// <c>explore:runs=&lt;n&gt;,depth=&lt;d&gt;</c> for an exploration; null leaves it to the call, the
    /// service's default (<c>reverse</c>) for a run and <see cref="Explore"/> for an exploration. Naming
    /// one needs <see cref="Capabilities.Schedule"/>, and an exploring one <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Schedule { get; init; }

    /// <summary>
    /// The object the behavior is performed by: the qualified name of a part definition or usage, or a
    /// path from one to a nested object (<c>Mission::mission.vehicle</c>), made anew for each run; null
    /// runs outside any object. Needs <see cref="Capabilities.Performer"/>.
    /// </summary>
    public string? PerformerSymbolId { get; init; }

    /// <summary>
    /// Whether a run returns its trace, failed or not, in <see cref="ActionRun.Trace"/> or
    /// <see cref="StateRun.Trace"/>; an exploration describes no single run. Needs
    /// <see cref="Capabilities.ActionTrace"/> for an action or <see cref="Capabilities.StateTrace"/> for a state.
    /// </summary>
    public bool Trace { get; init; }

    /// <summary>
    /// Event names sent to the behavior, in order; empty for none. An action needs a performer and a
    /// non-exploring schedule. For a state run, supply these or the call's events argument, not both.
    /// The list is copied when set.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">An event is null.</exception>
    public IReadOnlyList<string> Events
    {
        get => events;
        init => events = new ValueList<string>(value);
    }

    /// <summary>
    /// Simulated time, in seconds, to run for (<c>sysml -advance</c>): the behavior is started and its clock
    /// advanced by this much, and an action not completed by then fails with the instant it waits for;
    /// null runs to completion, a state machine to quiescence.
    /// </summary>
    public double? Advance { get; init; }

    /// <summary>
    /// The seed the model's own draws, weighted decisions and random functions, are taken from
    /// (<c>sysml -seed</c>); the same seed draws the same run. Null leaves them to the schedule's own seed.
    /// </summary>
    public ulong? Seed { get; init; }

    /// <summary>
    /// How the run resolves the draws of RandomFunctions (<c>sysml -draws</c>): <c>random</c>, the service's
    /// default, <c>min</c>, <c>max</c> or <c>average</c>; null for the default.
    /// </summary>
    public string? Draws { get; init; }

    /// <summary>
    /// The step, in seconds, the run's clock ticks by (<c>sysml -clock-step</c>): a wait comes due at the
    /// first multiple of it not before the wait ends; 0, the default, is a continuous clock.
    /// </summary>
    public double ClockStep { get; init; }

    /// <summary>Whether <see cref="Schedule"/> explores every order rather than running one: <c>explore</c> or <c>explore:…</c>.</summary>
    public bool Explores => Schedule is { } schedule && IsExploring(schedule);

    /// <summary>Whether a schedule spells an exploration, as the service reads it.</summary>
    internal static bool IsExploring(string schedule) =>
        schedule == Explore || schedule.StartsWith(Explore + ":", StringComparison.Ordinal);
}
