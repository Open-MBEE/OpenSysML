using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// How one RPC's scenarios are answered through the client's public API: which request fields the
/// API can send, and the call that reads them, makes the typed call and renders what it returned.
/// </summary>
internal sealed record RpcAdapter
{
    private RpcAdapter(string method, Type requestType, FrozenSet<string> fields, Func<Connection, IMessage, CancellationToken, Task<ScenarioAnswer>> call)
    {
        Method = method;
        RequestType = requestType;
        Fields = fields;
        Call = call;
    }

    /// <summary>The bare RPC name.</summary>
    internal string Method { get; }

    /// <summary>The request message type the call reads.</summary>
    internal Type RequestType { get; }

    /// <summary>The request fields, by schema name, the public API has a way to send.</summary>
    internal FrozenSet<string> Fields { get; }

    /// <summary>Makes the call: the connection, the decoded request, and the cancellation of the call.</summary>
    internal Func<Connection, IMessage, CancellationToken, Task<ScenarioAnswer>> Call { get; }

    /// <summary>An adapter for one RPC.</summary>
    /// <typeparam name="TRequest">The RPC's request message.</typeparam>
    /// <param name="method">The bare RPC name.</param>
    /// <param name="fields">Every request field the call reads, by schema name.</param>
    /// <param name="call">The call.</param>
    /// <returns>The adapter.</returns>
    internal static RpcAdapter Of<TRequest>(string method, IEnumerable<string> fields, Func<Connection, TRequest, CancellationToken, Task<ScenarioAnswer>> call)
        where TRequest : IMessage =>
        new(method, typeof(TRequest), fields.ToFrozenSet(StringComparer.Ordinal), (connection, request, cancellationToken) => call(connection, (TRequest)request, cancellationToken));
}

/// <summary>
/// A request, or a part of one, the public API has no way to send. An adapter throws it where it
/// reads the request, and the scenario is reported skipped for <see cref="Exception.Message"/>.
/// </summary>
internal sealed class UnexpressibleRequestException : Exception
{
    /// <summary>Creates the exception.</summary>
    /// <param name="reason">What the API cannot express, stably worded: the expected-skips list names it.</param>
    internal UnexpressibleRequestException(string reason)
        : base(reason)
    {
    }
}

/// <summary>Every RPC the public API covers, by bare name, each answered by its area's adapter.</summary>
internal static class ApiAdapters
{
    /// <summary>The adapters, by bare RPC name.</summary>
    internal static FrozenDictionary<string, RpcAdapter> All { get; } =
        ConnectionArea.Adapters
            .Concat(ModelArea.Adapters)
            .Concat(ExecutionArea.Adapters)
            .Concat(VerificationArea.Adapters)
            .Concat(SamplingArea.Adapters)
            .Concat(ConversionArea.Adapters)
            .Concat(QueryDocsArea.Adapters)
            .Concat(EditingArea.Adapters)
            .Concat(GraphDataArea.Adapters)
            .ToFrozenDictionary(adapter => adapter.Method, StringComparer.Ordinal);

    /// <summary>Why a scenario of an RPC the public API does not cover is skipped.</summary>
    /// <param name="method">The bare RPC name.</param>
    /// <returns>The reason, as the expected-skips list derives it.</returns>
    internal static string Uncovered(string method) => $"the public API does not cover {method}";

    /// <summary>The model a model RPC names, as the public API holds one by hash.</summary>
    /// <param name="connection">The connection.</param>
    /// <param name="modelHash">The request's model hash.</param>
    /// <returns>The model, borrowing the connection; nothing is sent.</returns>
    /// <exception cref="UnexpressibleRequestException">The hash is blank, which the API refuses to name.</exception>
    internal static Model ModelOf(Connection connection, string modelHash) =>
        string.IsNullOrWhiteSpace(modelHash)
            ? throw new UnexpressibleRequestException("the public API names a model by a hash that is not blank")
            : connection.ModelByHash(modelHash);
}
