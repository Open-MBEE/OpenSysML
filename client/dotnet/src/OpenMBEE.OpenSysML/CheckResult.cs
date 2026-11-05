using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a search of every schedule of a behavior found, as <c>sysml -check-property</c> reports it: the
/// verdict, each violation, divergence and final outcome with a schedule that replays it, and, when the
/// symbolic engine answered, its claim. A property that fails is a result whose <see cref="Verdict"/> is
/// <c>fails</c>, never an exception.
/// </summary>
public sealed record CheckResult
{
    /// <summary>Creates a check result, copying its collections.</summary>
    /// <param name="verdict"><c>holds</c>, <c>fails</c> or <c>undecided</c>.</param>
    /// <param name="lines">The report as the command prints it, one line each.</param>
    /// <param name="search">How the explicit-state search went; null when no search ran.</param>
    /// <param name="violations">Each violated property, deadlock or failure witnessed.</param>
    /// <param name="divergences">Each feature whose final value the schedule decides.</param>
    /// <param name="finals">Each distinct outcome of the complete schedules.</param>
    /// <param name="claim">What the symbolic engine asserted; empty when it did not answer.</param>
    /// <param name="freeInputs">The inputs the symbolic engine ranged over.</param>
    /// <param name="assumptions">The assumptions it rested on.</param>
    /// <param name="witness">Its witness; null when it gave none.</param>
    /// <param name="contrast">For a sensitivity, the contrasting schedule; null otherwise.</param>
    /// <param name="standing">The engine that answered, the strength of its evidence and its bounds.</param>
    /// <param name="reason">Why nothing could be decided, when the verdict is undecided.</param>
    /// <exception cref="ArgumentNullException">An argument other than an optional one is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public CheckResult(
        string verdict,
        IEnumerable<string> lines,
        CheckSearch? search,
        IEnumerable<CheckViolation> violations,
        IEnumerable<CheckDivergence> divergences,
        IEnumerable<CheckFinal> finals,
        string claim,
        IEnumerable<string> freeInputs,
        IEnumerable<string> assumptions,
        CheckWitness? witness,
        CheckWitness? contrast,
        Standing standing,
        string reason)
    {
        ArgumentNullException.ThrowIfNull(verdict);
        ArgumentNullException.ThrowIfNull(claim);
        ArgumentNullException.ThrowIfNull(standing);
        ArgumentNullException.ThrowIfNull(reason);
        Verdict = verdict;
        Lines = new ValueList<string>(lines);
        Search = search;
        Violations = new ValueList<CheckViolation>(violations);
        Divergences = new ValueList<CheckDivergence>(divergences);
        Finals = new ValueList<CheckFinal>(finals);
        Claim = claim;
        FreeInputs = new ValueList<string>(freeInputs);
        Assumptions = new ValueList<string>(assumptions);
        Witness = witness;
        Contrast = contrast;
        Standing = standing;
        Reason = reason;
    }

    /// <summary>
    /// <c>holds</c> when every schedule within the bounds was searched clean, <c>fails</c> on a violation or a
    /// divergence, <c>undecided</c> when a bound or a refusal left schedules unsearched.
    /// </summary>
    public string Verdict { get; }

    /// <summary>Whether every schedule within the bounds was searched clean.</summary>
    public bool Holds => Verdict == "holds";

    /// <summary>The report as the command prints it, one line each.</summary>
    public ValueList<string> Lines { get; }

    /// <summary>How the check engine's explicit-state search went; null when no search ran, as for a symbolic answer.</summary>
    public CheckSearch? Search { get; }

    /// <summary>Each violated property, deadlock or failure, with the schedule reaching it.</summary>
    public ValueList<CheckViolation> Violations { get; }

    /// <summary>Each feature whose final value the schedule decides, with a schedule reaching each value.</summary>
    public ValueList<CheckDivergence> Divergences { get; }

    /// <summary>Each distinct outcome of the complete schedules, with a schedule reaching it.</summary>
    public ValueList<CheckFinal> Finals { get; }

    /// <summary>What the symbolic engine asserted (<c>holds</c>, <c>violated</c>, <c>sensitive</c>, …); empty when it did not answer.</summary>
    public string Claim { get; }

    /// <summary>The inputs the symbolic engine ranged over.</summary>
    public ValueList<string> FreeInputs { get; }

    /// <summary>The assumptions the symbolic engine's claim rests on.</summary>
    public ValueList<string> Assumptions { get; }

    /// <summary>The symbolic engine's witness; null when it gave none.</summary>
    public CheckWitness? Witness { get; }

    /// <summary>For a sensitivity, the contrasting schedule; null otherwise.</summary>
    public CheckWitness? Contrast { get; }

    /// <summary>The engine that answered, the strength of its evidence and the bounds it ran under.</summary>
    public Standing Standing { get; }

    /// <summary>Why nothing could be decided, when the verdict is undecided; empty otherwise.</summary>
    public string Reason { get; }

    /// <summary>The report as the command prints it, or the verdict when it printed none.</summary>
    /// <returns>The report's text.</returns>
    public override string ToString() => Lines.IsEmpty ? Verdict : string.Join('\n', Lines);
}
