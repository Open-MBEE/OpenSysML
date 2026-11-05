using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Converts between <see cref="Query"/> and the service's <c>Query</c>, and reads a <c>QueryResponse</c>.</summary>
internal static class QueryWire
{
    /// <summary>Writes a query for a request.</summary>
    internal static Wire.Query ToWire(Query query) => new()
    {
        Scope = { query.Scope },
        Select = { query.Select },
        Where = query.Where is { } where ? ToWire(where) : null,
        OrderBy = { query.OrderBy },
    };

    /// <summary>Reads what a query selected; an element the schema says always names its id and type must.</summary>
    /// <exception cref="TransportException">An element names no id or no type.</exception>
    internal static QueryResult Result(Wire.QueryResponse response, string method) =>
        new(response.Elements.Select((element, i) => element.Id.Length == 0 || element.Type.Length == 0
            ? throw new TransportException(StatusCode.Internal, $"{method} answered elements[{i}] with no {(element.Id.Length == 0 ? "id" : "type")}, which every selected element reports.", method)
            : new QueryElement(element.Id, element.Type, element.Properties)));

    private static Wire.Constraint ToWire(QueryConstraint constraint) => constraint switch
    {
        QueryConstraint.Comparison comparison => new Wire.Constraint
        {
            Primitive = new Wire.PrimitiveConstraint
            {
                Inverse = comparison.Inverse,
                Property = comparison.Property,
                Operator = (Wire.PrimitiveOperator)(int)comparison.Operator,
                Value = { comparison.Values },
            },
        },
        QueryConstraint.Composite composite => new Wire.Constraint
        {
            Composite = new Wire.CompositeConstraint
            {
                Operator = (Wire.CompositeOperator)(int)composite.Operator,
                Constraint = { composite.Constraints.Select(ToWire) },
            },
        },
        _ => throw new System.InvalidOperationException($"No wire form is written for a {constraint.GetType().FullName}."),
    };
}
