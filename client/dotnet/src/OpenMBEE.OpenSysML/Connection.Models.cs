using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Connection
{
    internal const string ParseFileMethod = "sysml.SysMLService/ParseFile";
    internal const string ParseSourcesMethod = "sysml.SysMLService/ParseSources";

    /// <summary>
    /// Parses a file the service reads, and returns the model, which the service caches under its
    /// <see cref="Model.Hash"/>. The path is read by the service, in its own file system, and passed
    /// as given: a relative path resolves against the service's working directory.
    /// </summary>
    /// <param name="path">The file to parse.</param>
    /// <param name="options">How strictly to judge it; null for <see cref="ParseOptions.Default"/>. A file's
    /// notation comes from its extension, so <see cref="ParseOptions.Language"/> must be null.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The model, borrowing this connection. A source with errors is still a model: see <see cref="Model.Errors"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="InvalidRequestException">The options name a language, or one that is not a <see cref="SourceLanguage"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The options need a capability the service lacks; nothing was sent.</exception>
    /// <exception cref="ModelFileNotFoundException">The service could not read the file.</exception>
    /// <exception cref="ModelException">The service reported that the source could not be parsed at all.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Model> LoadAsync(string path, ParseOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        ThrowIfDisposed();
        options ??= ParseOptions.Default;
        if (options.Language is not null)
        {
            throw new InvalidRequestException(
                $"ParseOptions.Language names {options.Language}, but {path} is a file, whose notation its extension says; leave Language null to load it.");
        }

        return await ParseAsync(new Wire.ParseFileRequest { FilePath = path }, options, new ValueList<string>([path]), cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Parses notation given inline, and returns the model, which the service caches under its <see cref="Model.Hash"/>.</summary>
    /// <param name="content">The notation.</param>
    /// <param name="options">Its notation and how strictly to judge it; null for <see cref="ParseOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The model, borrowing this connection. A source with errors is still a model: see <see cref="Model.Errors"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="content"/> is null.</exception>
    /// <exception cref="InvalidRequestException">The options name a language that is not a <see cref="SourceLanguage"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The options need a capability the service lacks; nothing was sent.</exception>
    /// <exception cref="ModelException">The service reported that the source could not be parsed at all.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Model> LoadContentAsync(string content, ParseOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(content);
        ThrowIfDisposed();
        options ??= ParseOptions.Default;
        if (options.Language is { } language && !Enum.IsDefined(language))
        {
            throw new InvalidRequestException($"ParseOptions.Language is {language}, which is not a SourceLanguage.");
        }

        var request = new Wire.ParseFileRequest { Content = content };
        if (options.Language is { } named)
        {
            request.Language = LanguageName(named);
        }

        return await ParseAsync(request, options, default, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// Parses several documents as one model, so a name one declares resolves in another and an import
    /// between them is satisfied. Documents are parsed in the order given; the model's
    /// <see cref="Model.Roots"/> holds one root per document, in that order.
    /// </summary>
    /// <param name="documents">The documents, which the service requires to be at least one, each of its own name.</param>
    /// <param name="options">How strictly to judge them, and the model to relate the answer to; null for <see cref="SourcesOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The model, borrowing this connection.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="documents"/> is null.</exception>
    /// <exception cref="ArgumentException">A document is null.</exception>
    /// <exception cref="InvalidRequestException"><see cref="SourcesOptions.BaseModelHash"/> is empty; nothing was sent.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.ParseSources"/>, or one the options or a document's
    /// language need; nothing was sent.
    /// </exception>
    /// <exception cref="ModelFileNotFoundException">The service could not read a document's file.</exception>
    /// <exception cref="ModelException">The service reported that the documents could not be parsed at all.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a request of no document or two of one name.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Model> ParseSourcesAsync(IEnumerable<SourceDocument> documents, SourcesOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(documents);
        var sources = documents.ToList();
        if (sources.Contains(null!))
        {
            throw new ArgumentException("A document is null.", nameof(documents));
        }

        ThrowIfDisposed();
        options ??= SourcesOptions.Default;
        if (options.BaseModelHash is { Length: 0 })
        {
            throw new InvalidRequestException("SourcesOptions.BaseModelHash is empty; leave it null to relate the answer to no model.");
        }

        // The order the service checks them in, so the first refusal is the one it would make.
        var capabilities = Capabilities;
        capabilities.Require(OpenSysML.Capabilities.ParseSources);
        if (options.StrictConformance)
        {
            capabilities.Require(OpenSysML.Capabilities.StrictConformance);
        }

        if (options.BaseModelHash is not null)
        {
            capabilities.Require(OpenSysML.Capabilities.ParseSourcesAffected);
        }

        var request = new Wire.ParseSourcesRequest { StrictConformance = options.StrictConformance, BaseModelHash = options.BaseModelHash ?? string.Empty };
        foreach (var source in sources)
        {
            var document = new Wire.SourceDocument();
            if (source.FilePath is { } file)
            {
                document.FilePath = file;
            }
            else
            {
                document.Content = source.Content!;
                document.Name = source.Name ?? string.Empty;
                if (source.Language is { } language)
                {
                    capabilities.Require(OpenSysML.Capabilities.InlineLanguage);
                    document.Language = LanguageName(language);
                }
            }

            request.Documents.Add(document);
        }

        var response = await FileCallAsync<Wire.ParseSourcesRequest, Wire.ParseSourcesResponse>(ParseSourcesMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        RequireParsed(response.Error, response.ModelHash, ParseSourcesMethod, diagnostics);
        return new Model(
            this,
            response.ModelHash,
            ParseSourcesMethod,
            new ValueList<Symbol>(response.Roots.Select((root, i) => ModelWire.Symbol(root, ParseSourcesMethod, $"roots[{i}]"))),
            diagnostics,
            new ValueList<string>(sources.Select(source => source.FilePath ?? source.Name ?? string.Empty)),
            options.BaseModelHash,
            new ValueList<string>(response.Affected));
    }

    /// <summary>
    /// A handle on a model the service parsed already, named by its hash, as a host that kept a hash
    /// across connections holds one. Nothing is sent: a hash the service does not hold is reported by
    /// the first call made on the model, as <see cref="ModelNotFoundException"/>, and the handle
    /// carries neither roots nor the diagnostics of that parse.
    /// </summary>
    /// <param name="modelHash">A hash a parse returned.</param>
    /// <returns>The model, borrowing this connection.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="modelHash"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="modelHash"/> is empty or white space.</exception>
    /// <exception cref="ObjectDisposedException">The connection was disposed.</exception>
    public Model ModelByHash(string modelHash)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(modelHash);
        ThrowIfDisposed();
        return new Model(this, modelHash, string.Empty, default, default, default, null, default);
    }

    /// <summary>Refuses a typed call once disposal has begun, before any of its checks.</summary>
    /// <exception cref="ObjectDisposedException">The connection was disposed.</exception>
    internal void ThrowIfDisposed() => ObjectDisposedException.ThrowIf(Volatile.Read(ref disposing) != 0, this);

    /// <summary>Calls a method on the typed path, which owns the request's checks.</summary>
    internal Task<TResponse> UnaryAsync<TRequest, TResponse>(string method, TRequest request, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        if (!RpcMethods.TryResolve<TRequest, TResponse>(method, out var resolved, out var problem))
        {
            throw new InvalidOperationException(problem);
        }

        return InvokeAsync(resolved, request, cancellationToken);
    }

    /// <summary>
    /// Calls a method on a model the service caches. Such a method answers <see cref="StatusCode.NotFound"/>
    /// for one thing alone, a model it does not hold, so that status is a <see cref="ModelNotFoundException"/>.
    /// </summary>
    internal async Task<TResponse> ModelCallAsync<TRequest, TResponse>(string method, TRequest request, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        try
        {
            return await UnaryAsync<TRequest, TResponse>(method, request, cancellationToken).ConfigureAwait(false);
        }
        catch (ServiceException error) when (IsPlainNotFound(error))
        {
            throw new ModelNotFoundException(error.ServiceMessage, error.Method, error.InnerException);
        }
    }

    private static string LanguageName(SourceLanguage language) => language == SourceLanguage.KerML ? "kerml" : "sysml";

    private static bool IsPlainNotFound(ServiceException error) =>
        error.StatusCode == StatusCode.NotFound && error.GetType() == typeof(ServiceException);

    private static void RequireParsed(string error, string modelHash, string method, ValueList<Diagnostic> diagnostics)
    {
        if (error.Length > 0)
        {
            throw new ModelException(error, method, diagnostics);
        }

        if (modelHash.Length == 0)
        {
            throw new TransportException(StatusCode.Internal, $"{method} answered neither a model hash nor a failure.", method);
        }
    }

    private async Task<Model> ParseAsync(Wire.ParseFileRequest request, ParseOptions options, ValueList<string> documents, CancellationToken cancellationToken)
    {
        // The order the service checks them in, so the first refusal is the one it would make.
        if (options.StrictConformance)
        {
            Capabilities.Require(OpenSysML.Capabilities.StrictConformance);
            request.StrictConformance = true;
        }

        if (request.Language.Length > 0)
        {
            Capabilities.Require(OpenSysML.Capabilities.InlineLanguage);
        }

        var response = await FileCallAsync<Wire.ParseFileRequest, Wire.ParseFileResponse>(ParseFileMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        RequireParsed(response.Error, response.ModelHash, ParseFileMethod, diagnostics);
        var roots = response.Root is { } root ? new ValueList<Symbol>([ModelWire.Symbol(root, ParseFileMethod, "root")]) : default;
        return new Model(this, response.ModelHash, ParseFileMethod, roots, diagnostics, documents, null, default);
    }

    // A call reading files the service names answers NotFound for one thing alone, a file it could not read.
    private async Task<TResponse> FileCallAsync<TRequest, TResponse>(string method, TRequest request, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        try
        {
            return await UnaryAsync<TRequest, TResponse>(method, request, cancellationToken).ConfigureAwait(false);
        }
        catch (ServiceException error) when (IsPlainNotFound(error))
        {
            throw new ModelFileNotFoundException(error.ServiceMessage, error.Method, error.InnerException);
        }
    }
}
