using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.Loader;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The private services this copy of the client started, and the leases connections hold on them.
/// </summary>
/// <remarks>
/// <para>
/// The registry's scope is the assembly load context the client assembly is loaded in, not the
/// caller's: its state is static, so every plugin sharing one loaded copy shares its services, while a
/// copy loaded into another context has registries of its own. Within it there is one shared service
/// per <see cref="ServiceIdentity"/>; an isolated service belongs to one connection and is never
/// shared.
/// </para>
/// <para>
/// An entry is starting, then ready, then stopping, and then gone; or starting, then failed, then
/// gone. Concurrent acquirers of one identity wait for one start, each holding a reservation that
/// keeps the entry; one that gives up releases its own reservation and no other's, and the start is
/// cancelled only when no holder remains. A failed start stops its child and gives every waiter the
/// same failure. The last release stops a ready service. A service that exits on its own fails its
/// entry and ends every lease on it; it is never restarted, and a later acquirer starts another.
/// <see cref="StopAllAsync"/> ends every lease on a shared service and stops them all; an acquirer
/// that arrives while an entry stops waits for it to be gone, then starts afresh. No lock is ever
/// held across an await.
/// </para>
/// </remarks>
/// <param name="launch">Starts a service; <see cref="PrivateService.StartAsync"/> but in tests.</param>
internal sealed partial class ServiceRegistry(ServiceRegistry.Launcher launch)
{
    private readonly Lock gate = new();
    private readonly Dictionary<ServiceIdentity, Entry> shared = [];
    private readonly HashSet<Entry> live = [];

    /// <summary>Starts a service of an identity, logging to a logger, until a token is cancelled.</summary>
    /// <param name="identity">What to start.</param>
    /// <param name="logger">Where its life is logged.</param>
    /// <param name="cancellationToken">Cancels the start.</param>
    /// <returns>The running service.</returns>
    internal delegate Task<IServiceProcess> Launcher(ServiceIdentity identity, ILogger logger, CancellationToken cancellationToken);

    internal enum State
    {
        Starting,
        Ready,
        Stopping,
        Failed,
    }

    /// <summary>The registry of the load context this copy of the client is loaded in.</summary>
    internal static ServiceRegistry Shared { get; } = CreateShared();

    /// <summary>
    /// Takes a lease on the service of an identity: the shared one, started if there is none, or a
    /// new isolated one. Cancellation and failure release what was taken.
    /// </summary>
    /// <param name="identity">The service wanted.</param>
    /// <param name="isolated">Whether to start one for this lease alone.</param>
    /// <param name="logger">Where a service this call starts logs its life.</param>
    /// <param name="cancellationToken">Cancels this acquirer's wait, and the start when no other waits for it.</param>
    /// <returns>A lease on a running service, which the caller releases.</returns>
    /// <exception cref="ServiceStartException">The service could not be started, or was stopped as it started.</exception>
    /// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
    internal async Task<Lease> AcquireAsync(ServiceIdentity identity, bool isolated, ILogger logger, CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        Entry entry;
        while (true)
        {
            Entry? created = null;
            Task? gone = null;
            lock (gate)
            {
                if (isolated || !shared.TryGetValue(identity, out var existing))
                {
                    entry = created = new Entry(identity, isShared: !isolated, logger);
                    if (!isolated)
                    {
                        shared.Add(identity, created);
                    }

                    live.Add(created);
                }
                else if (existing.State == State.Starting || (existing.State == State.Ready && !existing.Process!.Exited.IsCompleted))
                {
                    entry = existing;
                }
                else
                {
                    // Stopping, or exited and about to be failed: wait until it is gone, then start afresh.
                    entry = existing;
                    gone = existing.Gone.Task;
                }

                if (gone is null)
                {
                    entry.Holders++;
                }
            }

            if (gone is not null)
            {
                await gone.WaitAsync(cancellationToken).ConfigureAwait(false);
                continue;
            }

            if (created is not null)
            {
                _ = Task.Run(() => StartAsync(created), CancellationToken.None);
            }

            break;
        }

        var lease = new Lease(this, entry);
        try
        {
            await entry.Started.Task.WaitAsync(cancellationToken).ConfigureAwait(false);
            return lease;
        }
        catch (Exception error)
        {
            await lease.ReleaseAsync(logger).ConfigureAwait(false);
            if (error is OperationCanceledException && !cancellationToken.IsCancellationRequested)
            {
                throw new ServiceStartException(
                    $"The private sysml-grpc at {identity.Path} was stopped as it started. {Volatile.Read(ref entry.EndReason)}".TrimEnd(), error);
            }

            throw;
        }
    }

    /// <summary>
    /// Ends every lease on a shared service and stops them all, after trying every one. Isolated
    /// services are left to their connections.
    /// </summary>
    /// <param name="cancellationToken">Cancels the wait; the services are stopped regardless.</param>
    /// <returns>How many services were stopped.</returns>
    /// <exception cref="ServiceStopException">Some could not be stopped cleanly.</exception>
    internal async Task<int> StopAllAsync(CancellationToken cancellationToken)
    {
        var ready = new List<Entry>();
        var starting = new List<Entry>();
        var already = new List<Task<Exception?>>();
        lock (gate)
        {
            foreach (var entry in shared.Values)
            {
                switch (entry.State)
                {
                    case State.Ready:
                        entry.State = State.Stopping;
                        ready.Add(entry);
                        break;
                    case State.Starting:
                        entry.State = State.Stopping;
                        starting.Add(entry);
                        break;
                    default:
                        already.Add(entry.Gone.Task);
                        break;
                }
            }
        }

        var stops = ready.Select(entry => StopEntryAsync(entry, StoppedByCaller(entry), entry.Logger))
            .Concat(starting.Select(entry => CancelStartAsync(entry, StoppedByCaller(entry))))
            .ToList();
        var failures = (await Task.WhenAll(stops.Concat(already)).WaitAsync(cancellationToken).ConfigureAwait(false)).OfType<Exception>().ToList();
        if (failures.Count > 0)
        {
            throw new ServiceStopException(failures);
        }

        return stops.Count;
    }

    /// <summary>The entries the registry knows, shared and isolated, for tests and diagnostics.</summary>
    /// <returns>A snapshot of each.</returns>
    internal IReadOnlyList<EntryView> Entries()
    {
        lock (gate)
        {
            return [.. live.Select(entry => new EntryView(entry.Identity, entry.IsShared, entry.State.ToString(), entry.Holders, entry.Process?.ProcessId))];
        }
    }

    /// <summary>
    /// Closes the input of every service this registry started, without waiting, so each exits
    /// promptly when the load context holding this copy of the client unloads; cancels starts in progress.
    /// </summary>
    internal void CloseAllInputs()
    {
        List<Entry> entries;
        lock (gate)
        {
            entries = [.. live];
            foreach (var entry in entries)
            {
                if (entry.State == State.Starting)
                {
                    entry.State = State.Stopping;
                }
            }
        }

        foreach (var entry in entries)
        {
            End(entry, "The client is being unloaded, so the private sysml-grpc it started is stopped.");
            try
            {
                entry.StartCancel.Cancel();
            }
            catch (AggregateException error)
            {
                LogStopFailed(entry.Logger, entry.Process?.ProcessId ?? 0, entry.Identity.Path, error);
            }

            entry.Process?.CloseInput();
        }
    }

    private static ServiceRegistry CreateShared()
    {
        var registry = new ServiceRegistry(PrivateService.StartAsync);
        if (AssemblyLoadContext.GetLoadContext(typeof(ServiceRegistry).Assembly) is { IsCollectible: true } context)
        {
            context.Unloading += _ => registry.CloseAllInputs();
        }

        return registry;
    }

    private static string StoppedByCaller(Entry entry) =>
        $"The private sysml-grpc{Pid(entry)} was stopped by Connection.StopSharedServicesAsync: a connection to it can no longer be used, and one opened afterwards starts a new one.";

    private static string Pid(Entry entry) => entry.Process is { } process ? $" (process {process.ProcessId})" : string.Empty;

    private async Task StartAsync(Entry entry)
    {
        IServiceProcess process;
        try
        {
            process = await launch(entry.Identity, entry.Logger, entry.StartCancel.Token).ConfigureAwait(false);
        }
        catch (Exception error)
        {
            lock (gate)
            {
                if (entry.State == State.Starting)
                {
                    entry.State = State.Failed;
                }

                Forget(entry);
            }

            if (error is OperationCanceledException)
            {
                entry.Started.TrySetCanceled(entry.StartCancel.Token);
            }
            else
            {
                entry.Started.TrySetException(error);
            }

            entry.Gone.TrySetResult(null);
            return;
        }

        bool ready;
        lock (gate)
        {
            entry.Process = process;
            ready = entry.State == State.Starting;
            if (ready)
            {
                entry.State = State.Ready;
            }
        }

        if (!ready)
        {
            // No holder is left, or every service is being stopped: this one goes too.
            var failure = await StopProcessAsync(entry, entry.Logger).ConfigureAwait(false);
            entry.Started.TrySetCanceled(entry.StartCancel.Token);
            entry.Gone.TrySetResult(failure);
            return;
        }

        entry.Started.TrySetResult(process);
        await WatchAsync(entry, process).ConfigureAwait(false);
    }

    private async Task WatchAsync(Entry entry, IServiceProcess process)
    {
        var ending = await process.Exited.ConfigureAwait(false);
        lock (gate)
        {
            if (entry.State != State.Ready)
            {
                return;
            }

            entry.State = State.Failed;
            if (entry.IsShared)
            {
                shared.Remove(entry.Identity);
            }
        }

        LogDied(entry.Logger, process.ProcessId, entry.Identity.Path, ending, process.DescribeTail());
        End(entry, $"The private sysml-grpc (process {process.ProcessId}) {ending} on its own; it is not restarted, so this connection "
            + "can no longer be used. Open another, which starts a new one.");
        entry.Gone.TrySetResult(await StopProcessAsync(entry, entry.Logger).ConfigureAwait(false));
    }

    private async Task ReleaseAsync(Entry entry, ILogger logger)
    {
        bool stop;
        bool cancel;
        lock (gate)
        {
            entry.Holders--;
            stop = entry.Holders == 0 && entry.State == State.Ready;
            cancel = entry.Holders == 0 && entry.State == State.Starting;
            if (stop || cancel)
            {
                entry.State = State.Stopping;
            }
        }

        if (stop)
        {
            await StopEntryAsync(entry, $"The private sysml-grpc (process {entry.Process!.ProcessId}) was stopped when its last connection closed.", logger).ConfigureAwait(false);
        }
        else if (cancel)
        {
            await CancelStartAsync(entry, "The private sysml-grpc was stopped as it started, since nothing waits for it any more.").ConfigureAwait(false);
        }
    }

    private async Task<Exception?> StopEntryAsync(Entry entry, string reason, ILogger logger)
    {
        End(entry, reason);
        var failure = await StopProcessAsync(entry, logger).ConfigureAwait(false);
        entry.Gone.TrySetResult(failure);
        return failure;
    }

    private static async Task<Exception?> CancelStartAsync(Entry entry, string reason)
    {
        End(entry, reason);
        try
        {
            await entry.StartCancel.CancelAsync().ConfigureAwait(false);
        }
        catch (AggregateException error)
        {
            LogStopFailed(entry.Logger, 0, entry.Identity.Path, error);
        }

        return await entry.Gone.Task.ConfigureAwait(false);
    }

    private async Task<Exception?> StopProcessAsync(Entry entry, ILogger logger)
    {
        var process = entry.Process!;
        Exception? failure = null;
        try
        {
            await process.StopAsync().ConfigureAwait(false);
        }
#pragma warning disable CA1031 // A stop that fails is logged, and reported to whoever asked for it.
        catch (Exception error)
#pragma warning restore CA1031
        {
            LogStopFailed(logger, process.ProcessId, entry.Identity.Path, error);
            failure = error;
        }

        lock (gate)
        {
            Forget(entry);
        }

        return failure;
    }

    private static void End(Entry entry, string reason)
    {
        if (Interlocked.CompareExchange(ref entry.EndReason, reason, null) is not null)
        {
            return;
        }

        try
        {
            entry.Ended.Cancel();
        }
        catch (AggregateException error)
        {
            LogStopFailed(entry.Logger, entry.Process?.ProcessId ?? 0, entry.Identity.Path, error);
        }
    }

    // Called under the gate.
    private void Forget(Entry entry)
    {
        if (entry.IsShared && shared.TryGetValue(entry.Identity, out var current) && ReferenceEquals(current, entry))
        {
            shared.Remove(entry.Identity);
        }

        live.Remove(entry);
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "The private service {Path}, process {ProcessId}, {Ending} on its own; its connections fail from now on{Tail}")]
    private static partial void LogDied(ILogger logger, int processId, string path, string ending, string tail);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Stopping the private service {Path}, process {ProcessId}, failed")]
    private static partial void LogStopFailed(ILogger logger, int processId, string path, Exception error);

    /// <summary>A snapshot of an entry.</summary>
    /// <param name="Identity">What it runs.</param>
    /// <param name="IsShared">Whether connections share it.</param>
    /// <param name="State">Starting, Ready, Stopping or Failed.</param>
    /// <param name="Holders">Leases and reservations held on it.</param>
    /// <param name="ProcessId">Its process, once started.</param>
    internal sealed record EntryView(ServiceIdentity Identity, bool IsShared, string State, int Holders, int? ProcessId);

    /// <summary>
    /// One connection's hold on a service: taken when the connection starts opening, and released once,
    /// when it closes or its open fails. The service runs while any hold remains.
    /// </summary>
    internal sealed class Lease
    {
        private readonly ServiceRegistry registry;
        private readonly Entry entry;
        private int released;

        internal Lease(ServiceRegistry registry, Entry entry)
        {
            this.registry = registry;
            this.entry = entry;
        }

        /// <summary>The service's base URL.</summary>
        internal Uri Address => entry.Process!.Address;

        /// <summary>The service's process id.</summary>
        internal int ProcessId => entry.Process!.ProcessId;

        /// <summary>Whether other connections share the service.</summary>
        internal bool IsShared => entry.IsShared;

        /// <summary>Cancelled once the service can no longer be used: it exited, or was stopped.</summary>
        internal CancellationToken Ended => entry.Ended.Token;

        /// <summary>Why the service can no longer be used; null while it can.</summary>
        internal string? EndReason => Volatile.Read(ref entry.EndReason);

        /// <summary>Gives the hold up, stopping the service when it was the last; once, never throwing.</summary>
        /// <param name="logger">Where a failure to stop is logged.</param>
        /// <returns>When the hold is given up, and the service stopped if this stopped it.</returns>
        internal Task ReleaseAsync(ILogger logger) =>
            Interlocked.Exchange(ref released, 1) == 0 ? registry.ReleaseAsync(entry, logger) : Task.CompletedTask;
    }

    /// <summary>A service of the registry, from its start until it is gone. Fields are guarded by the gate unless noted.</summary>
    internal sealed class Entry(ServiceIdentity identity, bool isShared, ILogger logger)
    {
        /// <summary>Why leases on it ended; written once, outside the gate.</summary>
        internal string? EndReason;

        internal ServiceIdentity Identity { get; } = identity;

        internal bool IsShared { get; } = isShared;

        /// <summary>Where the service logs: the logger of the connection that started it.</summary>
        internal ILogger Logger { get; } = logger;

        internal State State { get; set; } = State.Starting;

        internal int Holders { get; set; }

        /// <summary>The running service, once started; set before <see cref="Started"/> completes.</summary>
        internal IServiceProcess? Process { get; set; }

        /// <summary>Cancels the start.</summary>
        internal CancellationTokenSource StartCancel { get; } = new();

        /// <summary>Completes with the service once it is ready, or with why it never will be.</summary>
        internal TaskCompletionSource<IServiceProcess> Started { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);

        /// <summary>Cancelled when its leases end.</summary>
        internal CancellationTokenSource Ended { get; } = new();

        /// <summary>Completes once its process is gone and it is forgotten, with the failure to stop it, if any.</summary>
        internal TaskCompletionSource<Exception?> Gone { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);
    }
}
