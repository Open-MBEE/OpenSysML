using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Querying a model, running and rendering its document queries and documents, and rendering its views, through
/// <see cref="Model"/>'s calls. A document query or document the model declares wrongly reaches the caller as a
/// <see cref="QueryException"/> keeping the status the service failed the call with, which is rendered as that
/// refusal.
/// </summary>
internal static class QueryDocsArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.QueryRequest>("Query", ["model_hash", "query", "oslc_query"], QueryAsync),
        RpcAdapter.Of<Wire.RunDocumentQueryRequest>("RunDocumentQuery", ["model_hash", "query_id", "bindings"], RunDocumentQueryAsync),
        RpcAdapter.Of<Wire.RenderDocumentRequest>("RenderDocument", ["model_hash", "document_id", "form"], RenderDocumentAsync),
        RpcAdapter.Of<Wire.RenderViewRequest>("RenderView", ["model_hash", "view", "ports", "show", "depth", "compartments"], RenderViewAsync),
    ];

    private static async Task<ScenarioAnswer> QueryAsync(Connection connection, Wire.QueryRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var result = (request.Query, request.OslcQuery) switch
        {
            ({ }, { Length: > 0 }) => throw new UnexpressibleRequestException("the public API names a query in one form, structured or OSLC text, so it cannot send both"),
            (null, { Length: 0 }) => throw new UnexpressibleRequestException("the public API always names a query, so it cannot send a request naming none"),
            (null, var text) => await model.QueryOslcAsync(text, cancellationToken).ConfigureAwait(false),
            ({ } query, _) => await model.QueryAsync(Query(query), cancellationToken).ConfigureAwait(false),
        };
        return new ScenarioAnswer.Answered(Rendering.QueryResult(result));
    }

    private static async Task<ScenarioAnswer> RunDocumentQueryAsync(Connection connection, Wire.RunDocumentQueryRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.QueryId.Length == 0)
        {
            throw new UnexpressibleRequestException("the public API names a document query by a qualified name that is not empty");
        }

        var bindings = request.Bindings.Select((binding, i) => Binding(binding, $"bindings[{i}]")).ToList();
        try
        {
            return new ScenarioAnswer.Answered(Rendering.DocumentQueryResult(await model.RunDocumentQueryAsync(request.QueryId, bindings, cancellationToken).ConfigureAwait(false)));
        }
        catch (QueryException failure)
        {
            return new ScenarioAnswer.Refused(failure.StatusCode, failure.ServiceMessage);
        }
    }

    private static async Task<ScenarioAnswer> RenderDocumentAsync(Connection connection, Wire.RenderDocumentRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.DocumentId.Length == 0)
        {
            throw new UnexpressibleRequestException("the public API names a document by a qualified name that is not empty");
        }

        var form = request.Form switch
        {
            "" => DocumentForm.Markdown,
            "html" => DocumentForm.Html,
            "asciidoc" => DocumentForm.AsciiDoc,
            "markdown" => throw new UnexpressibleRequestException("the public API sends the Markdown form as the empty form it defaults to"),
            var other => throw new UnexpressibleRequestException($"the public API names a form as a DocumentForm, which has none spelled \"{other}\""),
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.RenderedDocument(await model.RenderDocumentAsync(request.DocumentId, form, cancellationToken).ConfigureAwait(false)));
        }
        catch (QueryException failure)
        {
            return new ScenarioAnswer.Refused(failure.StatusCode, failure.ServiceMessage);
        }
    }

    private static async Task<ScenarioAnswer> RenderViewAsync(Connection connection, Wire.RenderViewRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.View.Length == 0)
        {
            throw new UnexpressibleRequestException("the public API names a view by a name that is not empty");
        }

        var options = new RenderViewOptions
        {
            Ports = request.Ports switch
            {
                "" => RenderViewPorts.Minimal,
                "full" => RenderViewPorts.Full,
                "minimal" => throw new UnexpressibleRequestException("the public API sends minimal ports as the empty selection they default to"),
                var other => throw new UnexpressibleRequestException($"the public API names ports as a RenderViewPorts, which has none spelled \"{other}\""),
            },
            Show = request.Show,
            Depth = request.Depth == 0 ? null : request.Depth,
            Compartments = request.Compartments switch
            {
                "" => RenderViewCompartments.Positioned,
                "most" => RenderViewCompartments.Most,
                "ports" => RenderViewCompartments.Ports,
                "positioned" => throw new UnexpressibleRequestException("the public API sends positioned compartments as the empty selection they default to"),
                var other => throw new UnexpressibleRequestException($"the public API names compartments as a RenderViewCompartments, which has none spelled \"{other}\""),
            },
        };
        return new ScenarioAnswer.Answered(Rendering.RenderedView(await model.RenderViewAsync(request.View, options, cancellationToken).ConfigureAwait(false)));
    }

    // A structured query, its constraint read form by form; one the public types refuse is unexpressible.
    private static Query Query(Wire.Query query) => new()
    {
        Scope = query.Scope,
        Select = query.Select,
        Where = query.Where is { } where ? Constraint(where, "query.where") : null,
        OrderBy = query.OrderBy,
    };

    private static QueryConstraint Constraint(Wire.Constraint constraint, string path)
    {
        switch (constraint.ConstraintCase)
        {
            case Wire.Constraint.ConstraintOneofCase.Primitive:
                var primitive = constraint.Primitive;
                if (!Enum.IsDefined((ComparisonOperator)(int)primitive.Operator))
                {
                    throw new UnexpressibleRequestException($"the public API names a comparison's operator, which {path}.primitive does not");
                }

                return Held(() => new QueryConstraint.Comparison(primitive.Property, (ComparisonOperator)(int)primitive.Operator, primitive.Value, primitive.Inverse), path + ".primitive");
            case Wire.Constraint.ConstraintOneofCase.Composite:
                var composite = constraint.Composite;
                if (!Enum.IsDefined((CompositeOperator)(int)composite.Operator))
                {
                    throw new UnexpressibleRequestException($"the public API names a composite's operator, which {path}.composite does not");
                }

                var nested = composite.Constraint.Select((each, i) => Constraint(each, $"{path}.composite.constraint[{i}]")).ToList();
                return Held(() => new QueryConstraint.Composite((CompositeOperator)(int)composite.Operator, nested), path + ".composite");
            default:
                throw new UnexpressibleRequestException($"the public API names a constraint's form, which {path} does not");
        }
    }

    private static DocumentBinding Binding(Wire.DocumentQueryBinding binding, string path)
    {
        var values = binding.Values.Select((value, i) => Value(value, $"{path}.values[{i}]")).ToList();
        return Held(() => new DocumentBinding(binding.Parameter, values), path);
    }

    // A bound value read arm by arm with public constructors, so the call writes it back onto the wire itself.
    private static DocumentValue Value(Wire.DocumentValue value, string path) => value.KindCase switch
    {
        Wire.DocumentValue.KindOneofCase.ElementId => new DocumentValue.ElementValue(value.ElementId, value.ElementType),
        Wire.DocumentValue.KindOneofCase.None => new DocumentValue.ElementValue(null, value.ElementType),
        Wire.DocumentValue.KindOneofCase.StringValue => new DocumentValue.TextValue(value.StringValue),
        Wire.DocumentValue.KindOneofCase.IntValue => Number(new Wire.Value { IntValue = value.IntValue }, path),
        Wire.DocumentValue.KindOneofCase.BigIntValue => Number(new Wire.Value { BigIntValue = value.BigIntValue }, path),
        Wire.DocumentValue.KindOneofCase.RationalValue => Number(new Wire.Value { RationalValue = value.RationalValue }, path),
        Wire.DocumentValue.KindOneofCase.RealValue => Number(new Wire.Value { RealValue = value.RealValue }, path),
        Wire.DocumentValue.KindOneofCase.BoolValue => new DocumentValue.BooleanValue(value.BoolValue),
        Wire.DocumentValue.KindOneofCase.Infinity => new DocumentValue.InfinityValue(),
        Wire.DocumentValue.KindOneofCase.Quantity =>
            new DocumentValue.QuantityValue(((Value.QuantityValue)RequestValues.Read(new Wire.Value { Quantity = value.Quantity }, path)).Quantity),
        Wire.DocumentValue.KindOneofCase.Object => new DocumentValue.ObjectValue(new DocumentObject(
            value.Object.InstanceId,
            value.Object.Path,
            value.Object.Element is { } element
                ? Value(element, path + ".object.element") as DocumentValue.ElementValue
                    ?? throw new UnexpressibleRequestException($"the public API holds an object's element as an element value, which {path}.object.element is not")
                : null)),
        var kind => throw new UnexpressibleRequestException($"the public API binds no value as {path} spells it ({kind}), which queries answer and are never bound to"),
    };

    private static DocumentValue.NumberValue Number(Wire.Value value, string path) =>
        new((Value.NumberValue)RequestValues.Read(value, path));

    private static T Held<T>(Func<T> make, string path)
    {
        try
        {
            return make();
        }
        catch (ArgumentException invalid)
        {
            var reason = invalid.ParamName is { } name ? invalid.Message.Replace($" (Parameter '{name}')", string.Empty, StringComparison.Ordinal) : invalid.Message;
            throw new UnexpressibleRequestException($"the public API cannot hold {path} as spelled: {reason}");
        }
    }
}
