using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Every distinct outcome a behavior reached under the orders of its choice points the budget let
/// run, and how the exploration ended: what <see cref="Model.ExploreActionAsync"/>,
/// <see cref="Model.ExploreStateAsync"/> and <see cref="Model.ExploreAnalysisAsync"/> answer.
/// </summary>
public sealed record Exploration
{
    /// <summary>Creates an exploration, copying its collections.</summary>
    /// <param name="outcomes">The distinct outcomes, in canonical order.</param>
    /// <param name="complete">Whether every linearization the library admits was run.</param>
    /// <param name="runs">How many runs were made.</param>
    /// <param name="budgetsHit">The budgets hit, <c>runs</c> before <c>depth</c>.</param>
    /// <param name="runsBudget">The runs the exploration could make.</param>
    /// <param name="depthBudget">The choice points one run could resolve.</param>
    /// <param name="probabilitiesLowerBound">Whether weighted probabilities are lower bounds.</param>
    /// <param name="failedLinearizations">How many linearizations ended in runtime errors.</param>
    /// <param name="unenumerated">The constructs whose unordered alternatives the runs took in one order.</param>
    /// <param name="standing">The engine that answered, the strength of its evidence and its bounds; <see cref="Standing.None"/> when none was named.</param>
    /// <exception cref="ArgumentNullException">A collection or <paramref name="standing"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Exploration(
        IEnumerable<Outcome> outcomes,
        bool complete,
        int runs,
        IEnumerable<string> budgetsHit,
        int runsBudget,
        int depthBudget,
        bool probabilitiesLowerBound,
        int failedLinearizations,
        IEnumerable<string> unenumerated,
        Standing standing)
    {
        ArgumentNullException.ThrowIfNull(standing);
        Outcomes = new ValueList<Outcome>(outcomes);
        Complete = complete;
        Runs = runs;
        BudgetsHit = new ValueList<string>(budgetsHit);
        RunsBudget = runsBudget;
        DepthBudget = depthBudget;
        ProbabilitiesLowerBound = probabilitiesLowerBound;
        FailedLinearizations = failedLinearizations;
        Unenumerated = new ValueList<string>(unenumerated);
        Standing = standing;
    }

    /// <summary>Every distinct outcome reached, in canonical order.</summary>
    public ValueList<Outcome> Outcomes { get; }

    /// <summary>
    /// Whether every linearization the library admits was run, so <see cref="Outcomes"/> is the whole set:
    /// no budget was hit and nothing was left <see cref="Unenumerated"/>.
    /// </summary>
    public bool Complete { get; }

    /// <summary>How many runs were made.</summary>
    public int Runs { get; }

    /// <summary>The budgets hit, <c>runs</c> before <c>depth</c>; empty when none was.</summary>
    public ValueList<string> BudgetsHit { get; }

    /// <summary>The runs the exploration could make.</summary>
    public int RunsBudget { get; }

    /// <summary>The choice points one run could resolve before the rest take their first alternative.</summary>
    public int DepthBudget { get; }

    /// <summary>Whether weighted probabilities are lower bounds, because a budget kept some linearizations unexplored.</summary>
    public bool ProbabilitiesLowerBound { get; }

    /// <summary>How many linearizations ended in runtime-error outcomes.</summary>
    public int FailedLinearizations { get; }

    /// <summary>
    /// The constructs whose unordered alternatives the runs took in one order without a choice point, by
    /// identifier; <see cref="Complete"/> is false while any is listed.
    /// </summary>
    public ValueList<string> Unenumerated { get; }

    /// <summary>
    /// The engine that answered an analysis case's exploration, the strength of its evidence and the bounds it ran
    /// under; <see cref="Standing.None"/> for a behavior's, whose answer names no engine.
    /// </summary>
    public Standing Standing { get; }
}
