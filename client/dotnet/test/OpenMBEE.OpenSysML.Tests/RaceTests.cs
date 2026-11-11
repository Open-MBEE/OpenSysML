using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The race runner must finish or fail within a deadline, even when a party ignores cancellation.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class RaceTests
{
    private static readonly TimeSpan Deadline = TimeSpan.FromSeconds(1);
    private static readonly TimeSpan TestLimit = TimeSpan.FromSeconds(10);

    // The test's own limit, which must not pass for the TimeoutException a race's deadline raises: a round that has
    // not returned within it fails as that, naming the limit, rather than as a race that timed out.
    private static async Task<T> Watched<T>(Task<T> round)
    {
        try
        {
            return await round.WaitAsync(TestLimit, Token);
        }
        catch (TimeoutException) when (!round.IsCompleted)
        {
            throw new InvalidOperationException($"The race did not return within the test's {TestLimit}, though its deadline is {Deadline}.");
        }
    }
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    public async Task EveryPartyCanEnterBeforeAnyPartyFinishes()
    {
        var entered = 0;
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var parties = Enumerable.Range(0, 4).Select(_ => (Func<CancellationToken, Task>)(async token =>
        {
            if (Interlocked.Increment(ref entered) == 4)
            {
                release.TrySetResult();
            }

            await release.Task.WaitAsync(token);
        })).ToList();

        Assert.Empty(await Race.RunAsync(parties, Token));
        Assert.Equal(4, entered);
    }

    [Fact]
    public async Task FailuresAndVoluntaryCancellationsAreReturnedInPartyOrder()
    {
        var failure = new InvalidOperationException("a party failed");
        var failures = await Race.RunAsync(
            [_ => Task.FromException(failure), _ => Task.FromCanceled(new CancellationToken(canceled: true)), _ => Task.CompletedTask],
            Token);

        Assert.Collection(failures, error => Assert.Same(failure, error), error => Assert.IsType<OperationCanceledException>(error));
    }

    [Fact]
    public async Task ADeadlineCancelsACooperativePartyAndReportsATimeout()
    {
        var finished = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var failure = await Assert.ThrowsAsync<TimeoutException>(() => Watched(Race.RunAsync(
            [async token =>
            {
                try
                {
                    await Task.Delay(Timeout.Infinite, token);
                }
                finally
                {
                    finished.TrySetResult();
                }
            }],
            Token,
            Deadline)));

        Assert.Contains("lifecycle race", failure.Message, StringComparison.Ordinal);
        await finished.Task.WaitAsync(TestLimit, Token);
    }

    [Fact]
    public async Task ADeadlineReturnsEvenWhenAPartyIgnoresCancellationAndFaultsLater()
    {
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        try
        {
            var failure = await Assert.ThrowsAsync<TimeoutException>(() =>
                Watched(Race.RunAsync([_ => release.Task], Token, Deadline)));

            Assert.Contains("unfinished parties: [0]", failure.Message, StringComparison.Ordinal);
            Assert.False(release.Task.IsCompleted);
        }
        finally
        {
            release.TrySetException(new InvalidOperationException("a late party failure"));
        }
    }

    [Fact]
    public async Task CancellingTheCallerCancelsThePartyAndPreservesTheCallersToken()
    {
        using var caller = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var entered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var finished = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var running = Race.RunAsync(
            [async token =>
            {
                entered.TrySetResult();
                try
                {
                    await Task.Delay(Timeout.Infinite, token);
                }
                finally
                {
                    finished.TrySetResult();
                }
            }],
            caller.Token);
        await entered.Task.WaitAsync(TestLimit, Token);

        await caller.CancelAsync();

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => running.WaitAsync(TestLimit, Token));
        Assert.Equal(caller.Token, failure.CancellationToken);
        await finished.Task.WaitAsync(TestLimit, Token);
    }

    [Fact]
    public async Task AnAlreadyCancelledCallerNeverStartsAParty()
    {
        using var caller = new CancellationTokenSource();
        await caller.CancelAsync();
        var entered = false;

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => Race.RunAsync(
            [_ =>
            {
                entered = true;
                return Task.CompletedTask;
            }],
            caller.Token));

        Assert.Equal(caller.Token, failure.CancellationToken);
        Assert.False(entered);
    }
}
