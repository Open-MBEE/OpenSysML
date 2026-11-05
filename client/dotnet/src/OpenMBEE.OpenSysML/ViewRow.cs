using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>One row of a tabular view rendering: its cells, one per column, and where the element it reports is declared.</summary>
public sealed record ViewRow
{
    /// <summary>Creates a row.</summary>
    /// <param name="cells">The row's cells, one per column; copied.</param>
    /// <param name="origin">Where the element the row reports is declared; null when the service located it nowhere.</param>
    /// <exception cref="ArgumentNullException"><paramref name="cells"/> is null.</exception>
    /// <exception cref="ArgumentException">A cell is null.</exception>
    public ViewRow(IEnumerable<string> cells, SourceSpan? origin)
    {
        Cells = new ValueList<string>(cells);
        Origin = origin;
    }

    /// <summary>The row's cells, one per <see cref="RenderedView.Columns"/> entry.</summary>
    public ValueList<string> Cells { get; }

    /// <summary>Where the element the row reports is declared; null when the service located it nowhere.</summary>
    public SourceSpan? Origin { get; }
}
