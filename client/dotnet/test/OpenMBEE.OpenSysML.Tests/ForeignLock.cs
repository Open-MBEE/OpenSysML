using System;
using System.Diagnostics;
using System.Text;
using System.Threading;
using System.Threading.Channels;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// One run of the foreign lock holder, test/interop/lock_holder.py: a process that takes the shared cache lock with
/// the Python client's own calls. The Java and Rust clients take the same lock, so it stands for them too. Disposal
/// kills it if it still runs, so a failed assertion leaves no holder behind.
/// </summary>
internal sealed class ForeignLock : IAsyncDisposable
{
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(60);

    private readonly Process process;
    private readonly Channel<string> lines = Channel.CreateUnbounded<string>();
    private readonly StringBuilder errors = new();
    private readonly Task pumps;

    private ForeignLock(Process process)
    {
        this.process = process;
        pumps = Task.WhenAll(PumpOutputAsync(), PumpErrorsAsync());
    }

    /// <summary>Starts a holder taking the lock, and waits until it holds it.</summary>
    /// <param name="lockFile">The lock file.</param>
    /// <param name="start">Starts a stand-in process instead of the foreign holder; null in interoperability tests.</param>
    /// <param name="within">How long acquisition may wait; the usual patience when null.</param>
    /// <returns>The holder, which the caller disposes.</returns>
    internal static async Task<ForeignLock> HoldAsync(string lockFile, Func<Process>? start = null, TimeSpan? within = null)
    {
        var holder = start is null ? Start(lockFile) : new ForeignLock(start());
        try
        {
            await holder.HeldAsync(within);
            return holder;
        }
        catch
        {
            await holder.DisposeAsync();
            throw;
        }
    }

    /// <summary>Starts a holder taking the lock, which may have to wait for it.</summary>
    /// <param name="lockFile">The lock file.</param>
    /// <returns>The holder, which the caller disposes.</returns>
    internal static ForeignLock Start(string lockFile) => new(Launch("hold", lockFile));

    /// <summary>Whether the Python client could take the lock right now; it releases it at once if so.</summary>
    /// <param name="lockFile">The lock file.</param>
    /// <returns>Whether it was free.</returns>
    internal static async Task<bool> IsFreeAsync(string lockFile)
    {
        await using var run = new ForeignLock(Launch("try", lockFile));
        var answer = await run.NextAsync(Patience);
        await run.ExitedAsync();
        return answer switch
        {
            "free" => true,
            "busy" => false,
            _ => throw new InvalidOperationException($"The Python holder answered {answer ?? "nothing"} to try.\n{run.Errors}"),
        };
    }

    /// <summary>Waits until the holder holds the lock, failing the test if it does not within a while.</summary>
    /// <param name="within">How long to wait; the usual patience when null.</param>
    /// <returns>When it holds it.</returns>
    internal async Task HeldAsync(TimeSpan? within = null)
    {
        var line = await NextAsync(within ?? Patience);
        Assert.True(line == "held", $"The Python holder did not take the lock: it said {line ?? "nothing"}.\n{Errors}");
    }

    /// <summary>Whether the holder says it holds the lock within a time.</summary>
    /// <param name="within">How long to wait.</param>
    /// <returns>Whether it said so.</returns>
    internal async Task<bool> HeldWithinAsync(TimeSpan within)
    {
        using var timeout = new CancellationTokenSource(within);
        try
        {
            await lines.Reader.WaitToReadAsync(timeout.Token);
        }
        catch (OperationCanceledException)
        {
            return false;
        }

        await HeldAsync();
        return true;
    }

    /// <summary>Has the holder release the lock and exit.</summary>
    /// <returns>When it has released it and exited.</returns>
    internal async Task ReleaseAsync()
    {
        await process.StandardInput.WriteLineAsync();
        await process.StandardInput.FlushAsync();
        var line = await NextAsync(Patience);
        Assert.True(line == "released", $"The Python holder did not release the lock: it said {line ?? "nothing"}.\n{Errors}");
        await ExitedAsync();
    }

    /// <summary>Kills the holder, as a crash would end it, and waits for it to be gone.</summary>
    internal void Kill()
    {
        process.Kill(entireProcessTree: true);
        process.WaitForExit();
    }

    public async ValueTask DisposeAsync()
    {
        if (!process.HasExited)
        {
            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync(CancellationToken.None);
        }

        try
        {
            await pumps.WaitAsync(TimeSpan.FromSeconds(10));
        }
        catch (TimeoutException)
        {
        }

        process.Dispose();
    }

    private string Errors
    {
        get
        {
            lock (errors)
            {
                return errors.ToString();
            }
        }
    }

    private static Process Launch(string mode, string lockFile)
    {
        var start = new ProcessStartInfo(InteropGate.FromProcess.Require())
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        start.ArgumentList.Add(Repository.PathOf("client", "dotnet", "test", "interop", "lock_holder.py"));
        start.ArgumentList.Add(mode);
        start.ArgumentList.Add(lockFile);
        return Process.Start(start) ?? throw new InvalidOperationException($"{start.FileName} did not start.");
    }

    private async Task<string?> NextAsync(TimeSpan within)
    {
        using var timeout = new CancellationTokenSource(within);
        try
        {
            return (await lines.Reader.ReadAsync(timeout.Token)).Trim();
        }
        catch (Exception error) when (error is OperationCanceledException or ChannelClosedException)
        {
            return null;
        }
    }

    private async Task ExitedAsync()
    {
        using var timeout = new CancellationTokenSource(Patience);
        await process.WaitForExitAsync(timeout.Token);
        Assert.True(process.ExitCode == 0, $"The Python holder exited with code {process.ExitCode}.\n{Errors}");
    }

    private async Task PumpOutputAsync()
    {
        while (await process.StandardOutput.ReadLineAsync(CancellationToken.None) is { } line)
        {
            lines.Writer.TryWrite(line);
        }

        lines.Writer.TryComplete();
    }

    private async Task PumpErrorsAsync()
    {
        while (await process.StandardError.ReadLineAsync(CancellationToken.None) is { } line)
        {
            lock (errors)
            {
                errors.AppendLine(line);
            }
        }
    }
}
