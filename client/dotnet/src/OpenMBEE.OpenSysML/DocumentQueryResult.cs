using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a document query answered, as <c>%run-query</c> answers it but typed: the columns it projects and its rows,
/// both in the deterministic order the query engine reports. A query that selects nothing keeps its columns and
/// answers no rows.
/// </summary>
public sealed record DocumentQueryResult
{
    /// <summary>Creates a result.</summary>
    /// <param name="columns">The projected properties' names, in projection order; copied.</param>
    /// <param name="rows">The rows, in the engine's order; copied.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A column or a row is null.</exception>
    public DocumentQueryResult(IEnumerable<string> columns, IEnumerable<DocumentRow> rows)
    {
        Columns = new ValueList<string>(columns);
        Rows = new ValueList<DocumentRow>(rows);
    }

    /// <summary>The projected properties' names, in projection order.</summary>
    public ValueList<string> Columns { get; }

    /// <summary>The rows, in the engine's order.</summary>
    public ValueList<DocumentRow> Rows { get; }
}
