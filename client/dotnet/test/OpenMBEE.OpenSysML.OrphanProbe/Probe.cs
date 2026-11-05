using System;
using System.Collections.Concurrent;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.OrphanProbe;

/// <summary>The probe's modes; see Program.cs.</summary>
internal static class Probe
{
    private const string Usage = "usage: OpenMBEE.OpenSysML.OrphanProbe sibling | (serve|race) <binary> <wire> <siblings> | leak <binary> <wire>";
    private static readonly TimeSpan SiblingLifetime = TimeSpan.FromMinutes(2);

    // Held so the connection, and with it the lease on the service, lives as long as the probe.
    private static Connection? held;

    internal static async Task<int> RunAsync(string[] args)
    {
        if (args is ["sibling"])
        {
            Thread.Sleep(SiblingLifetime);
            return 0;
        }

        if (args.Length < 3 || Options(args[1], args[2]) is not { } options)
        {
            await Console.Error.WriteLineAsync(Usage).ConfigureAwait(false);
            return 2;
        }

        var count = args.Length > 3 ? int.Parse(args[3], CultureInfo.InvariantCulture) : 1;
        ExitWhenInputEnds();
        var siblings = new ConcurrentQueue<Process>();
        int service;
        try
        {
            (held, service) = args[0] switch
            {
                "serve" => await ServeAsync(options, count, siblings).ConfigureAwait(false),
                "race" => await RaceAsync(options, count, siblings).ConfigureAwait(false),
                "leak" => await LeakAsync(options, siblings).ConfigureAwait(false),
                _ => throw new ArgumentException(Usage),
            };
        }
#pragma warning disable CA1031 // Whatever failed is the probe's answer; the test reads it from stderr.
        catch (Exception error)
#pragma warning restore CA1031
        {
            await Console.Error.WriteLineAsync($"probe failed: {error}").ConfigureAwait(false);
            foreach (var sibling in siblings)
            {
                sibling.Kill();
            }

            return 1;
        }

        var ids = string.Join(',', siblings.Select(sibling => sibling.Id.ToString(CultureInfo.InvariantCulture)));
        var address = JsonSerializer.Serialize(held.Address.ToString());
        await Console.Out.WriteLineAsync(FormattableString.Invariant($"{{\"probe\":{Environment.ProcessId},\"service\":{service},\"address\":{address},\"siblings\":[{ids}]}}")).ConfigureAwait(false);
        await Console.Out.FlushAsync().ConfigureAwait(false);
        await Task.Delay(Timeout.Infinite).ConfigureAwait(false);
        return 0;
    }

    /// <summary>Opens through the public API and the registry this copy of the client shares, then starts the siblings.</summary>
    private static async Task<(Connection, int)> ServeAsync(ConnectionOptions options, int count, ConcurrentQueue<Process> siblings)
    {
        var connection = await Connection.OpenAsync(options).ConfigureAwait(false);
        var service = ServiceRegistry.Shared.Entries().Single().ProcessId!.Value;
        for (var i = 0; i < count; i++)
        {
            siblings.Enqueue(StartSibling());
        }

        return (connection, service);
    }

    /// <summary>
    /// Releases the service's start and every sibling's from one barrier, so the child and the siblings
    /// are created at the same moment, when a pipe end made inheritable for one could reach another.
    /// </summary>
    private static async Task<(Connection, int)> RaceAsync(ConnectionOptions options, int count, ConcurrentQueue<Process> siblings)
    {
        using var barrier = new Barrier(count + 1);
        var registry = new ServiceRegistry(async (identity, logger, token) =>
        {
            barrier.SignalAndWait(token);
            return await PrivateService.StartAsync(identity, logger, token).ConfigureAwait(false);
        });
        var threads = Enumerable.Range(0, count)
            .Select(_ => new Thread(() =>
            {
                barrier.SignalAndWait();
                siblings.Enqueue(StartSibling());
            })
            { IsBackground = true })
            .ToList();
        threads.ForEach(thread => thread.Start());
        var connection = await Connection.OpenAsync(options, _ => null, registry, CancellationToken.None).ConfigureAwait(false);
        threads.ForEach(thread => thread.Join());
        return (connection, registry.Entries().Single().ProcessId!.Value);
    }

    /// <summary>Hands one sibling a duplicate of the write end of the service's input, as a leak would.</summary>
    private static async Task<(Connection, int)> LeakAsync(ConnectionOptions options, ConcurrentQueue<Process> siblings)
    {
        var registry = new ServiceRegistry(async (identity, logger, token) =>
        {
            var service = (PrivateService)await PrivateService.StartAsync(identity, logger, token).ConfigureAwait(false);
            siblings.Enqueue(Leak.WhileStarting(service.Input, StartSibling));
            return service;
        });
        var connection = await Connection.OpenAsync(options, _ => null, registry, CancellationToken.None).ConfigureAwait(false);
        return (connection, registry.Entries().Single().ProcessId!.Value);
    }

    private static ConnectionOptions? Options(string binary, string wire) => wire switch
    {
        "grpc" => new ConnectionOptions { BinaryPath = binary },
        "connect-proto" => new ConnectionOptions { BinaryPath = binary, Protocol = Protocol.Connect },
        "connect-json" => new ConnectionOptions { BinaryPath = binary, Protocol = Protocol.Connect, Encoding = MessageEncoding.Json },
        _ => null,
    };

    /// <summary>Starts this probe again as an unrelated sibling, with its standard streams redirected as the service's are.</summary>
    private static Process StartSibling()
    {
        var self = Environment.ProcessPath ?? throw new InvalidOperationException("The probe cannot tell its own path.");
        var start = new ProcessStartInfo(self)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        if (Path.GetFileNameWithoutExtension(self) == "dotnet")
        {
            start.ArgumentList.Add(typeof(Probe).Assembly.Location);
        }

        start.ArgumentList.Add("sibling");
        return Process.Start(start) ?? throw new InvalidOperationException("The sibling did not start.");
    }

    private static void ExitWhenInputEnds()
    {
        new Thread(() =>
        {
            using var input = Console.OpenStandardInput();
            var buffer = new byte[256];
            while (input.Read(buffer) > 0)
            {
            }

            Environment.Exit(3);
        })
        { IsBackground = true }.Start();
    }
}
