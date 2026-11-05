using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>How the check engine's explicit-state search of a behavior went.</summary>
public sealed record CheckSearch
{
    /// <summary>Creates search statistics, copying their lists.</summary>
    /// <param name="states">Distinct states visited.</param>
    /// <param name="moves">Moves made.</param>
    /// <param name="maxDepth">The longest schedule searched.</param>
    /// <param name="boundsHit">The bounds the search ran into.</param>
    /// <param name="horizon">The simulated time the search stopped the clock at; meaningful when <paramref name="horizonSet"/>.</param>
    /// <param name="horizonSet">Whether the search ran to a horizon.</param>
    /// <param name="massBounded">Whether the violations' probabilities are lower bounds.</param>
    /// <param name="notes">Why the searched runs' coverage is narrower than their schedules.</param>
    /// <param name="scope">Why the finals were observed short of quiescence.</param>
    /// <param name="status">How the search ended, as <c>sysml</c> reports it.</param>
    /// <exception cref="ArgumentNullException">A list or <paramref name="status"/> is null.</exception>
    /// <exception cref="ArgumentException">A list holds a null.</exception>
    public CheckSearch(
        int states,
        int moves,
        int maxDepth,
        IEnumerable<string> boundsHit,
        double horizon,
        bool horizonSet,
        bool massBounded,
        IEnumerable<string> notes,
        IEnumerable<string> scope,
        string status)
    {
        ArgumentNullException.ThrowIfNull(status);
        States = states;
        Moves = moves;
        MaxDepth = maxDepth;
        BoundsHit = new ValueList<string>(boundsHit);
        Horizon = horizon;
        HorizonSet = horizonSet;
        MassBounded = massBounded;
        Notes = new ValueList<string>(notes);
        Scope = new ValueList<string>(scope);
        Status = status;
    }

    /// <summary>Distinct states visited.</summary>
    public int States { get; }

    /// <summary>Moves made.</summary>
    public int Moves { get; }

    /// <summary>The longest schedule searched.</summary>
    public int MaxDepth { get; }

    /// <summary>The bounds the search ran into; empty when it was exhaustive.</summary>
    public ValueList<string> BoundsHit { get; }

    /// <summary>The simulated time the search stopped the clock at, when <see cref="HorizonSet"/> says it ran to a horizon.</summary>
    public double Horizon { get; }

    /// <summary>Whether the search ran to a horizon, so <see cref="Horizon"/> is meaningful.</summary>
    public bool HorizonSet { get; }

    /// <summary>Whether the violations' probabilities are lower bounds.</summary>
    public bool MassBounded { get; }

    /// <summary>Why the searched runs' coverage is narrower than their schedules.</summary>
    public ValueList<string> Notes { get; }

    /// <summary>Why the finals were observed short of quiescence.</summary>
    public ValueList<string> Scope { get; }

    /// <summary>How the search ended, as <c>sysml</c> reports it.</summary>
    public string Status { get; }
}
