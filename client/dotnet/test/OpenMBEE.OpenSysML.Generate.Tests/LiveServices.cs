using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Tests;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// The sysml-grpc services a test class reads models through, at explicit addresses: one serving every wire,
/// and one withholding the capabilities a test names. Started when first needed and stopped after the class;
/// a test needing one skips, or fails where a service was promised, without a binary.
/// </summary>
public sealed class LiveServices : IAsyncDisposable
{
    private readonly ConcurrentDictionary<string, Lazy<Task<ServiceProcess>>> services = new(StringComparer.Ordinal);

    /// <summary>The address of the service withholding the given capabilities, comma-separated; null for none.</summary>
    /// <param name="withheld">The capabilities it withholds.</param>
    /// <returns>The address, as host:port.</returns>
    internal async Task<string> AddressAsync(string? withheld = null)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var service = services.GetOrAdd(
            withheld ?? string.Empty,
            _ => new Lazy<Task<ServiceProcess>>(() => ServiceProcess.StartAsync(binary, "connect", CancellationToken.None, withheld)));
        return (await service.Value).Address;
    }

    /// <summary>A connection over a wire to the service withholding the given capabilities.</summary>
    /// <param name="wire">The wire to speak.</param>
    /// <param name="cancellationToken">Cancels the open.</param>
    /// <param name="withheld">The capabilities the service withholds, comma-separated; null for none.</param>
    /// <param name="handler">The HTTP handler the connection sends through; null for its own.</param>
    /// <returns>The open connection.</returns>
    internal async Task<Connection> OpenAsync(ClientWire wire, CancellationToken cancellationToken, string? withheld = null, HttpMessageHandler? handler = null)
    {
        var options = wire.To(await AddressAsync(withheld));
        return await Connection.OpenAsync(handler is null ? options : new ConnectionOptions { Address = options.Address, Protocol = options.Protocol, Encoding = options.Encoding, HttpMessageHandler = handler }, cancellationToken);
    }

    /// <summary>Stops every service started.</summary>
    /// <returns>When they have stopped.</returns>
    public async ValueTask DisposeAsync()
    {
        foreach (var service in services.Values.Where(service => service.IsValueCreated))
        {
            if (service.Value.IsCompletedSuccessfully)
            {
                await (await service.Value).DisposeAsync();
            }
        }
    }

    /// <summary>A handler that records the method of every request it carries to the service.</summary>
    internal sealed class Recording() : DelegatingHandler(new SocketsHttpHandler())
    {
        private readonly ConcurrentQueue<string> methods = new();

        /// <summary>Every method called, in order.</summary>
        internal IReadOnlyList<string> Methods => [.. methods];

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            methods.Enqueue(request.RequestUri!.AbsolutePath);
            return base.SendAsync(request, cancellationToken);
        }
    }
}
