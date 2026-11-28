using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Runs operations at the same moment, so the tests can repeat a race rather than hope for it.</summary>
internal static class Race
{
    private static readonly TimeSpan RoundTimeout = TimeSpan.FromSeconds(30);

    /// <summary>
    /// Runs every party at once, each on a thread of its own, all released together by one barrier, so
    /// none waits for a free thread-pool thread, and returns what each failed with.
    /// </summary>
    /// <param name="parties">The operations.</param>
    /// <param name="cancellationToken">Cancels the round and its operations.</param>
    /// <param name="timeout">The round's deadline; null for thirty seconds.</param>
    /// <returns>Every exception a party ended with, cancellations included.</returns>
    internal static async Task<IReadOnlyList<Exception>> RunAsync(
        IReadOnlyList<Func<CancellationToken, Task>> parties,
        CancellationToken cancellationToken,
        TimeSpan? timeout = null)
    {
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(parties.Count);
        var limit = timeout ?? RoundTimeout;
        ArgumentOutOfRangeException.ThrowIfLessThanOrEqual(limit, TimeSpan.Zero);
        cancellationToken.ThrowIfCancellationRequested();
        using var round = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        round.CancelAfter(limit);
        var token = round.Token;
        var barrier = new Barrier(parties.Count);
        var tasks = parties
            .Select(party => Task.Factory.StartNew(
                () =>
                {
                    barrier.SignalAndWait(token);
                    return party(token);
                },
                CancellationToken.None,
                TaskCreationOptions.LongRunning,
                TaskScheduler.Default).Unwrap())
            .ToList();
        var all = Task.WhenAll(tasks);
        // A timed-out party may still run: observe late faults and keep its barrier alive until it ends.
        _ = all.ContinueWith(
            completed =>
            {
                _ = completed.Exception;
                barrier.Dispose();
            },
            CancellationToken.None,
            TaskContinuationOptions.ExecuteSynchronously,
            TaskScheduler.Default);
        try
        {
            await all.WaitAsync(token);
        }
        catch (OperationCanceledException error) when (token.IsCancellationRequested)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var pending = string.Join(", ", tasks.Select((task, index) => (task, index)).Where(item => !item.task.IsCompleted).Select(item => item.index));
            throw new TimeoutException($"The lifecycle race did not complete within {limit}; unfinished parties: [{pending}].", error);
        }
        catch (Exception) when (all.IsCompleted)
        {
            // Each task's own exception is collected below.
        }

        return [.. tasks.Where(task => !task.IsCompletedSuccessfully).SelectMany(Failures)];
    }

    private static IEnumerable<Exception> Failures(Task task) =>
        task.IsCanceled ? [new OperationCanceledException("a party was cancelled")] : task.Exception!.InnerExceptions;
}
