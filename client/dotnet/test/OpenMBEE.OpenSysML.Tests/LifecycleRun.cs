using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Owns a race's connection attempts and resolved child processes, including on failure or timeout.</summary>
internal sealed class LifecycleRun : IAsyncDisposable
{
    private static readonly TimeSpan DefaultCleanupTimeout = TimeSpan.FromSeconds(10);
    private readonly Lock gate = new();
    private readonly CancellationTokenSource closing = new();
    private readonly List<Task<Connection>> openings = [];
    private readonly HashSet<int> processes = [];
    private readonly TimeSpan cleanupTimeout;
    private Task? disposal;

    /// <summary>Creates a run with a bounded orderly cleanup before its owned-process fallback.</summary>
    /// <param name="cleanupTimeout">The orderly cleanup deadline; null for ten seconds.</param>
    internal LifecycleRun(TimeSpan? cleanupTimeout = null)
    {
        this.cleanupTimeout = cleanupTimeout ?? DefaultCleanupTimeout;
        ArgumentOutOfRangeException.ThrowIfLessThanOrEqual(this.cleanupTimeout, TimeSpan.Zero);
        Registry = new ServiceRegistry(LaunchAsync);
    }

    /// <summary>The registry owned by this run alone.</summary>
    internal ServiceRegistry Registry { get; }

    /// <summary>Every child this run has resolved, even after it leaves the registry.</summary>
    internal IReadOnlyList<int> ProcessIds
    {
        get
        {
            lock (gate)
            {
                return [.. processes];
            }
        }
    }

    /// <summary>Opens a connection whose attempt and eventual ownership belong to this run.</summary>
    /// <param name="options">The connection's options.</param>
    /// <param name="cancellationToken">Cancels this open.</param>
    /// <returns>The connection.</returns>
    internal Task<Connection> OpenAsync(ConnectionOptions options, CancellationToken cancellationToken)
    {
        lock (gate)
        {
            ObjectDisposedException.ThrowIf(disposal is not null, this);
            var opening = OpenCoreAsync(options, cancellationToken);
            openings.Add(opening);
            return opening;
        }
    }

    public ValueTask DisposeAsync()
    {
        lock (gate)
        {
            disposal ??= DisposeCoreAsync([.. openings]);
            return new ValueTask(disposal);
        }
    }

    private async Task<Connection> OpenCoreAsync(ConnectionOptions options, CancellationToken cancellationToken)
    {
        using var linked = CancellationTokenSource.CreateLinkedTokenSource(closing.Token, cancellationToken);
        return await Connection.OpenAsync(options, _ => null, Registry, linked.Token);
    }

    private async Task<IServiceProcess> LaunchAsync(ServiceIdentity identity, ILogger logger, CancellationToken cancellationToken)
    {
        var service = await PrivateService.StartAsync(identity, logger, cancellationToken);
        bool ended;
        lock (gate)
        {
            processes.Add(service.ProcessId);
            ended = disposal is not null;
        }

        if (ended)
        {
            await KillAsync(service.ProcessId);
        }

        return service;
    }

    private async Task DisposeCoreAsync(IReadOnlyList<Task<Connection>> pending)
    {
        var orderly = Task.Run(async () =>
        {
            try
            {
                await closing.CancelAsync();
                await Task.WhenAll(pending.Select(CloseAsync));
            }
            finally
            {
                await Registry.StopAllAsync(CancellationToken.None);
            }
        });
        _ = orderly.ContinueWith(
            completed =>
            {
                _ = completed.Exception;
                closing.Dispose();
            },
            CancellationToken.None,
            TaskContinuationOptions.ExecuteSynchronously,
            TaskScheduler.Default);
        try
        {
            await orderly.WaitAsync(cleanupTimeout);
        }
        catch (TimeoutException error)
        {
            throw new TimeoutException($"Orderly lifecycle cleanup did not complete within {cleanupTimeout}; owned processes: [{string.Join(", ", ProcessIds)}].", error);
        }
        finally
        {
            await Task.WhenAll(ProcessIds.Select(KillAsync));
        }
    }

    private static async Task CloseAsync(Task<Connection> opening)
    {
        Connection connection;
        try
        {
            connection = await opening;
        }
        catch (Exception) when (opening.IsFaulted || opening.IsCanceled)
        {
            // The caller observes failed opens; only a successful acquisition owns a connection.
            return;
        }

        await connection.DisposeAsync();
    }

    private static async Task KillAsync(int processId)
    {
        FakeChild.Kill(processId);
        await FakeChild.AssertGoneAsync(processId);
    }
}
