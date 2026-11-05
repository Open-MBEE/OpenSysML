using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One row a document query answered: the element it selected and, per projected column in column order, the values
/// the row holds for it.
/// </summary>
public sealed record DocumentRow
{
    /// <summary>Creates a row.</summary>
    /// <param name="element">The selected element: an element value with its qualified name, an object the service holds, or a verdict, state or event row.</param>
    /// <param name="cells">The row's values for each column, in column order; copied.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A cell, or a value in one, is null.</exception>
    public DocumentRow(DocumentValue element, IEnumerable<IEnumerable<DocumentValue>> cells)
    {
        ArgumentNullException.ThrowIfNull(element);
        ArgumentNullException.ThrowIfNull(cells);
        Element = element;
        Cells = new ValueList<ValueList<DocumentValue>>(cells.Select(cell => cell is null
            ? throw new ArgumentException("A cell is null; an empty cell holds no values.", nameof(cells))
            : new ValueList<DocumentValue>(cell)));
    }

    /// <summary>
    /// The selected element: an <see cref="DocumentValue.ElementValue"/> with its qualified name, an
    /// <see cref="DocumentValue.ObjectValue"/> for a row over an object the service holds, or a
    /// <see cref="DocumentValue.VerdictValue"/>, <see cref="DocumentValue.StateValue"/> or
    /// <see cref="DocumentValue.EventValue"/> for a row <c>Verdicts</c>, <c>States</c> or <c>Events</c> answered.
    /// </summary>
    public DocumentValue Element { get; }

    /// <summary>The row's values for each projected column, in column order; a cell holds several values for a nonscalar property.</summary>
    public ValueList<ValueList<DocumentValue>> Cells { get; }
}
