using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One limit an analysis engine ran under, as its plan names it, and whether the run stopped at it.</summary>
public sealed record Bound
{
    /// <summary>Creates a bound.</summary>
    /// <param name="name">The bound's name as the budget spells it: <c>runs</c>, <c>depth</c>, <c>steps</c>, <c>elements</c>, <c>solver</c>, <c>configurations</c>.</param>
    /// <param name="limit">The value the run was held to: a count, or milliseconds for a time.</param>
    /// <param name="reached">Whether the run stopped at the limit rather than finishing within it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    public Bound(string name, long limit, bool reached)
    {
        ArgumentNullException.ThrowIfNull(name);
        Name = name;
        Limit = limit;
        Reached = reached;
    }

    /// <summary>The bound's name as the budget spells it: <c>runs</c>, <c>depth</c>, <c>steps</c>, <c>elements</c>, <c>solver</c>, <c>configurations</c>.</summary>
    public string Name { get; }

    /// <summary>The value the run was held to: a count, or milliseconds for a time.</summary>
    public long Limit { get; }

    /// <summary>Whether the run stopped at the limit rather than finishing within it, which is what lowers an answer's strength.</summary>
    public bool Reached { get; }
}
