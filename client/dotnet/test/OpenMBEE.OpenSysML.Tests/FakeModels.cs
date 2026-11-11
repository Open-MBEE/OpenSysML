using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Answers model methods on a <see cref="FakeServiceHandler"/>, each from a function of its decoded request.</summary>
internal static class FakeModels
{
    /// <summary>Every capability the model operations gate on, so a test withholds one by leaving it out.</summary>
    internal static readonly string[] ModelCapabilities =
    [
        Capabilities.StrictConformance,
        Capabilities.InlineLanguage,
        Capabilities.ParseSources,
        Capabilities.ParseSourcesAffected,
        Capabilities.EvaluateSubject,
        Capabilities.Query,
        Capabilities.QueryOrderBy,
        Capabilities.Schedule,
        Capabilities.ScheduleExplore,
        Capabilities.Performer,
        Capabilities.StateTrace,
        Capabilities.ActionTrace,
        Capabilities.Check,
        Capabilities.Engines,
        Capabilities.Verification,
        Capabilities.VerificationQuestions,
        Capabilities.VerificationArguments,
        Capabilities.Runs,
        Capabilities.SelfCheck,
        Capabilities.RecordRun,
        Capabilities.Convert,
        Capabilities.ConvertDocuments,
        Capabilities.ConvertCompact,
        Capabilities.ConvertCheckDiagnostics,
        Capabilities.ConvertLibraryStubs,
        Capabilities.Migrate,
        Capabilities.OslcQuery,
        Capabilities.DocumentQuery,
        Capabilities.RenderDocument,
        Capabilities.RenderDocumentHtml,
        Capabilities.RenderDocumentAsciidoc,
        Capabilities.RenderView,
        Capabilities.ApplyEdits,
        Capabilities.EditDocuments,
        Capabilities.ExportGraphs,
        Capabilities.ImportData,
        Capabilities.CompareResults,
        Capabilities.SymbolMembers,
        Capabilities.ElementProperties,
        Capabilities.InstancePaths,
        .. Internal.Mapping.EditWire.GateOrder,
        .. Internal.Mapping.ValueWire.SendGateOrder,
    ];

    /// <summary>Answers a method from its request; methods answered before keep their answers.</summary>
    internal static FakeServiceHandler On<TRequest>(this FakeServiceHandler handler, string method, MessageParser<TRequest> parser, Func<TRequest, IMessage> answer)
        where TRequest : IMessage<TRequest>, new()
    {
        var previous = handler.Respond;
        handler.Respond = (sent, token) => sent.Method == method
            ? Task.FromResult(handler.Answer(answer(sent.Message(parser))))
            : previous(sent, token);
        return handler;
    }

    /// <summary>Refuses a method with a status the service sends.</summary>
    internal static FakeServiceHandler Refusing(this FakeServiceHandler handler, string method, StatusCode status, string message)
    {
        var previous = handler.Respond;
        handler.Respond = (sent, token) => sent.Method == method ? Task.FromResult(handler.Refuse(status, message)) : previous(sent, token);
        return handler;
    }

    /// <summary>Answers ParseFile with a model of this hash, one root and these diagnostics.</summary>
    internal static FakeServiceHandler Parsing(this FakeServiceHandler handler, string hash, Wire.SymbolInfo root, params Wire.Diagnostic[] diagnostics) =>
        handler.On("ParseFile", Wire.ParseFileRequest.Parser, _ => new Wire.ParseFileResponse { ModelHash = hash, Root = root, Diagnostics = { diagnostics } });

    /// <summary>Loads inline notation through the fake, answered by <see cref="Parsing"/>.</summary>
    internal static Task<Model> LoadAsync(this Connection connection) =>
        connection.LoadContentAsync("package P;", cancellationToken: TestContext.Current.CancellationToken);

    /// <summary>A diagnostic as the service sends one.</summary>
    internal static Wire.Diagnostic Diagnostic(string severity, string message, string code = "", Wire.Span? span = null) =>
        new() { Severity = severity, Message = message, Code = code, Span = span };

    /// <summary>A symbol as the service sends one, with no facts beyond these.</summary>
    internal static Wire.SymbolInfo Symbol(string id, string name, string kind, params string[] children) =>
        new() { Id = id, Name = name, Kind = kind, ChildIds = { children } };
}

/// <summary>
/// A model the fake service holds: symbols by id, each with its owner, answering GetSymbol, and Query
/// by name or id with the <c>owner</c> and <c>name</c> properties, as the service does.
/// </summary>
internal sealed class FakeModelTree
{
    private readonly Dictionary<string, (Wire.SymbolInfo Symbol, string Owner)> symbols = new(StringComparer.Ordinal);
    private readonly Dictionary<string, string> aliases = new(StringComparer.Ordinal);
    private readonly HashSet<string> unqueryable = new(StringComparer.Ordinal);

    /// <summary>The root symbol, which owns no other.</summary>
    internal Wire.SymbolInfo Root { get; private set; } = new();

    /// <summary>Adds the root.</summary>
    internal FakeModelTree WithRoot(string id, string kind = "RootNamespace")
    {
        Root = FakeModels.Symbol(id, string.Empty, kind);
        symbols[id] = (Root, string.Empty);
        return this;
    }

    /// <summary>Adds a symbol under an owner, which lists it as a child; one not queryable is one the query cannot see.</summary>
    internal FakeModelTree With(string owner, string id, string name, string kind = "partDef", bool queryable = true)
    {
        var symbol = FakeModels.Symbol(id, name, kind);
        symbols[id] = (symbol, owner);
        symbols[owner].Symbol.ChildIds.Add(id);
        if (!queryable)
        {
            unqueryable.Add(id);
        }

        return this;
    }

    /// <summary>Makes GetSymbol resolve a name to a symbol of another id, as an import or alias does.</summary>
    internal FakeModelTree WithAlias(string name, string target)
    {
        aliases[name] = target;
        return this;
    }

    /// <summary>Makes the fake answer GetSymbol and, when the service has the capability, Query, from this tree.</summary>
    internal FakeServiceHandler Serve(FakeServiceHandler handler, string hash)
    {
        handler.Parsing(hash, Root.Clone());
        handler.On("GetSymbol", Wire.GetSymbolRequest.Parser, request =>
        {
            Assert.Equal(hash, request.ModelHash);
            return symbols.TryGetValue(aliases.GetValueOrDefault(request.SymbolId, request.SymbolId), out var entry)
                ? new Wire.SymbolResponse { Symbol = entry.Symbol.Clone() }
                : new Wire.SymbolResponse { Error = $"symbol not found: {request.SymbolId}" };
        });
        handler.On("Query", Wire.QueryRequest.Parser, request => Query(hash, request));
        return handler;
    }

    private Wire.QueryResponse Query(string hash, Wire.QueryRequest request)
    {
        Assert.Equal(hash, request.ModelHash);
        var where = request.Query.Where?.Primitive;
        var response = new Wire.QueryResponse();
        foreach (var (id, (symbol, owner)) in symbols.Where(entry => entry.Value.Owner.Length > 0 && !unqueryable.Contains(entry.Key)))
        {
            var properties = new Dictionary<string, string>(StringComparer.Ordinal) { ["@id"] = id, ["name"] = symbol.Name, ["owner"] = owner };
            if (where is not null)
            {
                Assert.Equal(Wire.PrimitiveOperator.Equal, where.Operator);
                if (!where.Value.Contains(properties[where.Property]))
                {
                    continue;
                }
            }

            var element = new Wire.QueryResultElement { Id = id, Type = symbol.Kind };
            foreach (var selected in request.Query.Select)
            {
                element.Properties[selected] = properties[selected];
            }

            response.Elements.Add(element);
        }

        return response;
    }
}
