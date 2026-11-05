using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.ExecuteActionAsync"/>, <see cref="Model.ExecuteStateAsync"/> and their
/// exploring counterparts run a behavior: the scheduling policy, the object performing it, the
/// run's trace, the events posted to an action's performer, and the run's clock and draws, each as the <c>sysml</c> command's flag of the same
/// name sets it. Values outside what the service accepts (an unknown draw policy, a negative advance or
/// clock step, a malformed schedule) are refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class RunOptions
{
    /// <summary>The scheduling policy that explores every order (<c>sysml -schedule explore</c>), with its default budget.</summary>
    public const string Explore = "explore";

    /// <summary>The service's defaults: its default schedule, outside any object, without a trace or events, to completion.</summary>
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
    /// Whether a run returns its trace, failed or not: a state run's in <see cref="StateRun.Trace"/>, which needs
    /// <see cref="Capabilities.StateTrace"/>, and an action run's token steps, waits, accepts, sends, decisions,
    /// assignments and completion in <see cref="ActionRun.Trace"/>, which needs <see cref="Capabilities.ActionTrace"/>.
    /// An exploration describes no single run and has none.
    /// </summary>
    public bool Trace { get; init; }

    /// <summary>
    /// The signals or items an action run posts to the object <see cref="PerformerSymbolId"/> names, in order, as the
    /// REPL's <c>%send</c> does: each is posted once every token of the run waits, and the accept waiting for it takes it
    /// on the next step. Each names a signal or item definition, qualified or as the action's namespace sees it. Null or
    /// empty posts none. Only a single action run takes them; a state run takes its events as an argument, and an
    /// exploration none.
    /// </summary>
    public IReadOnlyList<string>? Events { get; init; }

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
