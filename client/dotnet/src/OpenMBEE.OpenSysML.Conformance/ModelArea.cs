using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf.Collections;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Parsing and reading a model: ParseFile and ParseSources through <see cref="Connection"/>, and
/// GetSymbol, GetDiagnostics, Evaluate and Instantiate through the <see cref="Model"/> a hash names.
/// A failure the service reports in its answer reaches the caller as a <see cref="ModelException"/>,
/// which is rendered back into the response's error and diagnostics.
/// </summary>
internal static class ModelArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ParseFileRequest>("ParseFile", ["file_path", "content", "language", "strict_conformance"], ParseFileAsync),
        RpcAdapter.Of<Wire.ParseSourcesRequest>("ParseSources", ["documents", "strict_conformance", "base_model_hash"], ParseSourcesAsync),
        RpcAdapter.Of<Wire.GetSymbolRequest>("GetSymbol", ["model_hash", "symbol_id"], GetSymbolAsync),
        RpcAdapter.Of<Wire.DiagnosticsRequest>("GetDiagnostics", ["model_hash", "disable_lint", "enable_lint"], GetDiagnosticsAsync),
        RpcAdapter.Of<Wire.EvaluateRequest>("Evaluate", ["model_hash", "expression", "context_symbol_id", "subject_symbol_id"], EvaluateAsync),
        RpcAdapter.Of<Wire.InstantiateRequest>("Instantiate", ["model_hash", "symbol_id"], InstantiateAsync),
    ];

    /// <summary>The language a request names, as the public API names it; null for none.</summary>
    /// <param name="name">The language as the schema spells it, or empty.</param>
    /// <returns>The language, or null.</returns>
    /// <exception cref="UnexpressibleRequestException">The API has no language of that name.</exception>
    internal static SourceLanguage? Language(string name) => name switch
    {
        "" => null,
        "sysml" => SourceLanguage.SysML,
        "kerml" => SourceLanguage.KerML,
        _ => throw new UnexpressibleRequestException($"the public API names a language as a SourceLanguage, which has none spelled \"{name}\""),
    };

    private static async Task<ScenarioAnswer> ParseFileAsync(Connection connection, Wire.ParseFileRequest request, CancellationToken cancellationToken)
    {
        var options = new ParseOptions { StrictConformance = request.StrictConformance };
        Task<Model> parsing = request.SourceCase switch
        {
            Wire.ParseFileRequest.SourceOneofCase.FilePath when request.Language.Length > 0 =>
                throw new UnexpressibleRequestException("the public API reads a file's notation from its extension, so it cannot name a language for one"),
            Wire.ParseFileRequest.SourceOneofCase.FilePath when request.FilePath.Length == 0 =>
                throw new UnexpressibleRequestException("the public API names a file by a path that is not empty"),
            Wire.ParseFileRequest.SourceOneofCase.FilePath => connection.LoadAsync(request.FilePath, options, cancellationToken),
            Wire.ParseFileRequest.SourceOneofCase.Content =>
                connection.LoadContentAsync(request.Content, new ParseOptions { Language = Language(request.Language), StrictConformance = request.StrictConformance }, cancellationToken),
            _ => throw new UnexpressibleRequestException("the public API always names a source, so it cannot send a request naming none"),
        };

        var response = new Wire.ParseFileResponse();
        try
        {
            var model = await parsing.ConfigureAwait(false);
            response.ModelHash = model.Hash;
            if (model.Root is { } root)
            {
                response.Root = Rendering.Symbol(root);
            }

            response.Diagnostics.AddRange(Rendering.Diagnostics(model.ParseDiagnostics));
        }
        catch (ModelException failure)
        {
            response.Error = InBand(failure, response.Diagnostics);
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> ParseSourcesAsync(Connection connection, Wire.ParseSourcesRequest request, CancellationToken cancellationToken)
    {
        var documents = new List<SourceDocument>(request.Documents.Count);
        foreach (var document in request.Documents)
        {
            documents.Add(Document(document));
        }

        var options = new SourcesOptions
        {
            StrictConformance = request.StrictConformance,
            BaseModelHash = request.BaseModelHash.Length > 0 ? request.BaseModelHash : null,
        };
        var response = new Wire.ParseSourcesResponse();
        try
        {
            var model = await connection.ParseSourcesAsync(documents, options, cancellationToken).ConfigureAwait(false);
            response.ModelHash = model.Hash;
            foreach (var root in model.Roots)
            {
                response.Roots.Add(Rendering.Symbol(root));
            }

            response.Diagnostics.AddRange(Rendering.Diagnostics(model.ParseDiagnostics));
            response.Affected.AddRange(model.Affected);
        }
        catch (ModelException failure)
        {
            response.Error = InBand(failure, response.Diagnostics);
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static SourceDocument Document(Wire.SourceDocument document) => document.SourceCase switch
    {
        Wire.SourceDocument.SourceOneofCase.FilePath when document.Language.Length > 0 || document.Name.Length > 0 =>
            throw new UnexpressibleRequestException("the public API names a file document by its path alone, so it cannot give one a name or a language"),
        Wire.SourceDocument.SourceOneofCase.FilePath when document.FilePath.Length == 0 =>
            throw new UnexpressibleRequestException("the public API names a file document by a path that is not empty"),
        Wire.SourceDocument.SourceOneofCase.FilePath => SourceDocument.FromFile(document.FilePath),
        Wire.SourceDocument.SourceOneofCase.Content =>
            SourceDocument.Inline(document.Content, document.Name.Length > 0 ? document.Name : null, Language(document.Language)),
        _ => throw new UnexpressibleRequestException("the public API always names a document's source, so it cannot send a document naming none"),
    };

    private static async Task<ScenarioAnswer> GetSymbolAsync(Connection connection, Wire.GetSymbolRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var response = new Wire.SymbolResponse();
        try
        {
            response.Symbol = Rendering.Symbol(await model.SymbolAsync(request.SymbolId, cancellationToken).ConfigureAwait(false));
        }
        catch (ModelException failure)
        {
            response.Error = failure.Message;
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> GetDiagnosticsAsync(Connection connection, Wire.DiagnosticsRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = new DiagnosticsOptions { DisableLint = [.. request.DisableLint], EnableLint = [.. request.EnableLint] };
        var response = new Wire.DiagnosticsResponse();
        try
        {
            response.Diagnostics.AddRange(Rendering.Diagnostics(await model.GetDiagnosticsAsync(options, cancellationToken).ConfigureAwait(false)));
        }
        catch (ModelException failure)
        {
            response.Error = InBand(failure, response.Diagnostics);
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> EvaluateAsync(Connection connection, Wire.EvaluateRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = new EvalOptions
        {
            ContextSymbolId = request.ContextSymbolId.Length > 0 ? request.ContextSymbolId : null,
            SubjectSymbolId = request.SubjectSymbolId.Length > 0 ? request.SubjectSymbolId : null,
        };
        var response = new Wire.EvaluateResponse();
        try
        {
            var evaluation = await model.EvaluateAsync(request.Expression, options, cancellationToken).ConfigureAwait(false);
            response.Result = Rendering.Value(evaluation.Result);
            response.Diagnostics.AddRange(Rendering.Diagnostics(evaluation.Diagnostics));
        }
        catch (ModelException failure)
        {
            response.Error = InBand(failure, response.Diagnostics);
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> InstantiateAsync(Connection connection, Wire.InstantiateRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var response = new Wire.InstantiateResponse();
        try
        {
            var instantiation = await model.InstantiateAsync(request.SymbolId, cancellationToken).ConfigureAwait(false);
            response.Instance = Rendering.Instance(instantiation.Root);
            foreach (var reachable in instantiation.Reachable)
            {
                response.Instances.Add(Rendering.Instance(reachable));
            }

            response.Diagnostics.AddRange(Rendering.Diagnostics(instantiation.Diagnostics));
        }
        catch (ModelException failure)
        {
            response.Error = InBand(failure, response.Diagnostics);
        }

        return new ScenarioAnswer.Answered(response);
    }

    // A failure the answer reported: its text is the response's error, its diagnostics the response's.
    private static string InBand(ModelException failure, RepeatedField<Wire.Diagnostic> diagnostics)
    {
        foreach (var diagnostic in Rendering.Diagnostics(failure.Diagnostics))
        {
            diagnostics.Add(diagnostic);
        }

        return failure.Message;
    }
}
