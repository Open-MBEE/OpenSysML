using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One entry-parameter binding of a document query: the parameter and the values bound to it, in order. A
/// nonscalar parameter takes several values, and a parameter named by several bindings takes all of their values,
/// appended in the order the bindings are given.
/// </summary>
/// <remarks>
/// A binding holds only what a query can be bound to: an element by qualified name
/// (<see cref="DocumentValue.ElementRef"/>), an object the service holds by path or id
/// (<see cref="DocumentValue.ObjectRef(string)"/>, <see cref="DocumentValue.ObjectRef(long)"/>), a string, a number,
/// a boolean or a quantity. An element or object a query answered can be bound again as answered: the type and
/// element it carries are sent, and ignored. Infinity and the rows of <c>Verdicts</c>, <c>States</c> and
/// <c>Events</c> are only ever answered, so a binding refuses them.
/// </remarks>
public sealed record DocumentBinding
{
    /// <summary>Creates a binding.</summary>
    /// <param name="parameter">The entry parameter's name.</param>
    /// <param name="values">The values bound to it, in order; copied.</param>
    /// <exception cref="ArgumentNullException">The parameter or the values are null.</exception>
    /// <exception cref="ArgumentException">
    /// The parameter is empty; a value is null; or a value is one a query answers but cannot be bound to: infinity, a
    /// row, an anonymous element, or an object named by neither a path nor a positive id.
    /// </exception>
    public DocumentBinding(string parameter, params IEnumerable<DocumentValue> values)
    {
        ArgumentException.ThrowIfNullOrEmpty(parameter);
        var held = new ValueList<DocumentValue>(values);
        foreach (var (value, i) in held.Select((value, i) => (value, i)))
        {
            if (Unbindable(value) is { } reason)
            {
                throw new ArgumentException($"The value {parameter}[{i}] {reason}.", nameof(values));
            }
        }

        Parameter = parameter;
        Values = held;
    }

    /// <summary>The entry parameter's name.</summary>
    public string Parameter { get; }

    /// <summary>The values bound to it, in order.</summary>
    public ValueList<DocumentValue> Values { get; }

    private static string? Unbindable(DocumentValue value) => value switch
    {
        DocumentValue.ElementValue { Id: null or "" } => "is an element named by no qualified name, which a query cannot be bound to",
        DocumentValue.ObjectValue { Value: { Path: "", InstanceId: <= 0 } } => "is an object named by neither a path nor an id, which ids count up from 1",
        DocumentValue.ObjectValue { Value.InstanceId: < 0 } => "is an object of a negative id, which ids count up from 1",
        DocumentValue.InfinityValue => "is infinity, which queries answer and are never bound to",
        DocumentValue.VerdictValue => "is a verdict row, which queries answer and are never bound to",
        DocumentValue.StateValue => "is a state row, which queries answer and are never bound to",
        DocumentValue.EventValue => "is an event row, which queries answer and are never bound to",
        _ => null,
    };
}
