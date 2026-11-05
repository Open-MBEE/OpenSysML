using System;

namespace OpenMBEE.OpenSysML;

/// <summary>How many completed runs produced one distinct value of an observable that is not numeric.</summary>
public sealed record RunCountedValue
{
    /// <summary>Creates a count.</summary>
    /// <param name="value">The distinct value, as the service spells it: an object by the usage it occurs as and its id.</param>
    /// <param name="count">The completed runs that produced it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
    public RunCountedValue(Value value, long count)
    {
        ArgumentNullException.ThrowIfNull(value);
        Value = value;
        Count = count;
    }

    /// <summary>The distinct value, as the service spells it: an object by the usage it occurs as and its id.</summary>
    public Value Value { get; }

    /// <summary>The completed runs that produced it.</summary>
    public long Count { get; }
}
