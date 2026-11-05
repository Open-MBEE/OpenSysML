using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a query selected: its elements, in the order the model declares them. A query that matches nothing answers
/// with no elements; one the service cannot evaluate is refused instead.
/// </summary>
public sealed record QueryResult
{
    /// <summary>Creates a result.</summary>
    /// <param name="elements">The elements selected, in declaration order; copied.</param>
    /// <exception cref="ArgumentNullException"><paramref name="elements"/> is null.</exception>
    /// <exception cref="ArgumentException">An element is null.</exception>
    public QueryResult(IEnumerable<QueryElement> elements) => Elements = new ValueList<QueryElement>(elements);

    /// <summary>The elements selected, in the order the model declares them.</summary>
    public ValueList<QueryElement> Elements { get; }
}
