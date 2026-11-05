using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Starts in-memory services for a <see cref="ServiceRegistry"/> under test, recording each start, so
/// its state machine is driven without processes. By default a start succeeds at once; a test may hold
/// starts at a gate, fail them, or decide each one itself.
/// </summary>
internal sealed class FakeLauncher
{
    private readonly ConcurrentQueue<Launch> launches = new();

    /// <summary>A registry starting services through this launcher.</summary>
    internal ServiceRegistry Registry => new(LaunchAsync);

    /// <summary>Every start, in order.</summary>
    internal IReadOnlyList<Launch> Launches => [.. launches];

    /// <summary>When set, each start waits for it before it completes.</summary>
    internal TaskCompletionSource? Gate { get; set; }

    /// <summary>Whether a start held at the gate ignores its cancellation, as a start already past the point of no return does.</summary>
    internal bool IgnoreCancellation { get; set; }

    /// <summary>When set, each start fails with it once through the gate.</summary>
    internal Exception? Failure { get; set; }

    /// <summary>How each started service stops; null for a clean stop.</summary>
    internal Func<FakeProcess, Exception?> StopFailure { get; set; } = _ => null;

    /// <summary>An identity naming a path, with the default limits.</summary>
    internal static ServiceIdentity Identity(string path = "/bin/sysml-grpc", string sha256 = "aa", string? version = null) =>
        new(path, sha256, ConnectionOptions.DefaultStartupTimeout, ConnectionOptions.DefaultShutdownTimeout, version);

    private async Task<IServiceProcess> LaunchAsync(ServiceIdentity identity, ILogger logger, CancellationToken cancellationToken)
    {
        var launch = new Launch(identity, cancellationToken);
        launches.Enqueue(launch);
        if (Gate is { } gate)
        {
            await gate.Task.WaitAsync(IgnoreCancellation ? CancellationToken.None : cancellationToken);
        }

        if (Failure is { } failure)
        {
            throw failure;
        }

        var process = new FakeProcess(this);
        launch.Process = process;
        return process;
    }

    /// <summary>One start.</summary>
    /// <param name="Identity">What was started.</param>
    /// <param name="Token">The token the start was given.</param>
    internal sealed record Launch(ServiceIdentity Identity, CancellationToken Token)
    {
        /// <summary>The service started, once it was.</summary>
        internal FakeProcess? Process { get; set; }
    }
}

/// <summary>An in-memory service of a <see cref="FakeLauncher"/>.</summary>
internal sealed class FakeProcess(FakeLauncher launcher) : IServiceProcess
{
    private static int nextProcessId = 1_000_000;
    private readonly TaskCompletionSource<string> exited = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private int stops;
    private int inputClosed;

    public Uri Address { get; } = new("http://127.0.0.1:1/");

    public int ProcessId { get; } = Interlocked.Increment(ref nextProcessId);

    public Task<string> Exited => exited.Task;

    /// <summary>How many times it was asked to stop.</summary>
    internal int Stops => Volatile.Read(ref stops);

    /// <summary>Whether its input was closed.</summary>
    internal bool InputClosed => Volatile.Read(ref inputClosed) != 0;

    public Task StopAsync()
    {
        Interlocked.Increment(ref stops);
        exited.TrySetResult("exited with code 0");
        return launcher.StopFailure(this) is { } failure ? Task.FromException(failure) : Task.CompletedTask;
    }

    public void CloseInput() => Interlocked.Exchange(ref inputClosed, 1);

    public string DescribeTail() => "\n  it last wrote:\n    a fake";

    /// <summary>Exits on its own.</summary>
    internal void Die(int code) => exited.TrySetResult($"exited with code {code}");
}
