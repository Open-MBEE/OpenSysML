using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Answers scenarios through the client's public typed API over one <see cref="Connection"/>. The
/// runner decodes each request into its wire message; this reads that message as the arguments of a
/// typed call, makes the call and renders what the call returned, by the RPC's <see cref="RpcAdapter"/>.
/// No request is sent raw. An RPC with no adapter, or a request setting a field its adapter cannot
/// send, is unexpressible, and the scenario is skipped for that reason.
/// </summary>
internal sealed class ApiScenarioClient : IScenarioClient
{
    private readonly Connection connection;
    private readonly IReadOnlyDictionary<string, RpcAdapter> adapters;

    /// <summary>A client answering through one connection.</summary>
    /// <param name="protocol">The protocol's name in the report: grpc, connect or connect-json.</param>
    /// <param name="service">What answers, for the report.</param>
    /// <param name="connection">The open connection, which the caller keeps and disposes.</param>
    /// <param name="adapters">The adapters by bare RPC name; null for <see cref="ApiAdapters.All"/>.</param>
    internal ApiScenarioClient(string protocol, string service, Connection connection, IReadOnlyDictionary<string, RpcAdapter>? adapters = null)
    {
        ArgumentNullException.ThrowIfNull(protocol);
        ArgumentNullException.ThrowIfNull(service);
        ArgumentNullException.ThrowIfNull(connection);
        Protocol = protocol;
        Service = service;
        this.connection = connection;
        this.adapters = adapters ?? ApiAdapters.All;
    }

    /// <inheritdoc/>
    public string Protocol { get; }

    /// <inheritdoc/>
    public string Service { get; }

    /// <inheritdoc/>
    public IReadOnlyCollection<string> Capabilities => connection.Capabilities.Names;

    /// <inheritdoc/>
    public async Task<ModelParse> ParseAsync(ScenarioModel model, IReadOnlyList<FixtureSource> documents, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(model);
        ArgumentNullException.ThrowIfNull(documents);
        var language = ModelArea.Language(model.Language);
        try
        {
            var parsed = model.IsMultiDocument
                ? await connection.ParseSourcesAsync(
                    documents.Select(document => SourceDocument.Inline(document.Content, document.Name, language)),
                    new SourcesOptions { StrictConformance = model.StrictConformance },
                    cancellationToken).ConfigureAwait(false)
                : await connection.LoadContentAsync(
                    documents.Single().Content,
                    new ParseOptions { Language = language, StrictConformance = model.StrictConformance },
                    cancellationToken).ConfigureAwait(false);
            return new ModelParse(parsed.Hash, string.Empty, Diagnostics(parsed.ParseDiagnostics));
        }
        catch (ModelException failure)
        {
            return new ModelParse(string.Empty, failure.Message, Diagnostics(failure.Diagnostics));
        }
    }

    /// <inheritdoc/>
    public async Task<ScenarioAnswer> CallAsync(MethodDescriptor method, IMessage request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(method);
        ArgumentNullException.ThrowIfNull(request);
        if (!adapters.TryGetValue(method.Name, out var adapter))
        {
            return new ScenarioAnswer.Unexpressible(ApiAdapters.Uncovered(method.Name));
        }

        if (request.GetType() != adapter.RequestType)
        {
            throw new InvalidOperationException($"the {method.Name} adapter reads a {adapter.RequestType}, not the {request.GetType()} the scenario decoded");
        }

        if (Unsendable(request, adapter.Fields) is { } field)
        {
            return new ScenarioAnswer.Unexpressible($"the public API has no way to send {field.FullName}");
        }

        try
        {
            return await adapter.Call(connection, request, cancellationToken).ConfigureAwait(false);
        }
        catch (UnexpressibleRequestException unexpressible)
        {
            return new ScenarioAnswer.Unexpressible(unexpressible.Message);
        }
        catch (InvalidRequestException refused)
        {
            // The API refuses to send it, so it is a request the API cannot express.
            return new ScenarioAnswer.Unexpressible(refused.Message);
        }
        catch (CapabilityException refused)
        {
            // The client refuses before sending what the service, lacking the capability, would refuse.
            return new ScenarioAnswer.Refused(StatusCode.Unimplemented, refused.Message);
        }
        catch (ServiceException refused)
        {
            return new ScenarioAnswer.Refused(refused.StatusCode, refused.ServiceMessage);
        }
        catch (TransportException failed)
        {
            return new ScenarioAnswer.Refused(failed.StatusCode, failed.Message);
        }
    }

    private static FieldDescriptor? Unsendable(IMessage request, FrozenSet<string> fields) =>
        request.Descriptor.Fields.InFieldNumberOrder()
            .FirstOrDefault(field => !fields.Contains(field.Name) && Normalizer.IsSet(field, request, field.Accessor.GetValue(request)));

    private static List<ModelParseDiagnostic> Diagnostics(IEnumerable<Diagnostic> diagnostics) =>
        [.. diagnostics.Select(diagnostic => new ModelParseDiagnostic(diagnostic.SeverityName, diagnostic.Message))];
}
