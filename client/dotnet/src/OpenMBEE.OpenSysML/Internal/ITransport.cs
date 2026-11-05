using System;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// One protocol's way of making a unary call. A transport maps every failure to the client's
/// contract: <see cref="ServiceException"/> for a status the service sent,
/// <see cref="TransportException"/> otherwise, and <see cref="OperationCanceledException"/> carrying
/// the call's token once that token is cancelled. It never retries.
/// </summary>
internal interface ITransport : IAsyncDisposable
{
    /// <summary>The protocol this transport speaks.</summary>
    Protocol Protocol { get; }

    /// <summary>How this transport encodes messages.</summary>
    MessageEncoding Encoding { get; }

    /// <summary>Makes one unary call.</summary>
    /// <typeparam name="TRequest">The request message.</typeparam>
    /// <typeparam name="TResponse">The response message.</typeparam>
    /// <param name="method">The method, already resolved against the schema.</param>
    /// <param name="request">The request.</param>
    /// <param name="deadline">When the call must have completed, in UTC.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The response.</returns>
    Task<TResponse> UnaryAsync<TRequest, TResponse>(
        RpcMethod<TRequest, TResponse> method,
        TRequest request,
        DateTime deadline,
        CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>;
}
