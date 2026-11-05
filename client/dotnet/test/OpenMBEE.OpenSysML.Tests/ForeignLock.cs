using System;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Channels;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// One run of a foreign client's lock holder (test/interop): a process that takes the shared cache lock with
/// that client's own calls. Disposal kills it if it still runs, so a failed assertion leaves no holder behind.
/// </summary>
internal sealed class ForeignLock : IAsyncDisposable
{
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(60);
    private static readonly TimeSpan BuildPatience = TimeSpan.FromMinutes(10);
    private static readonly SemaphoreSlim RustBuild = new(1, 1);
    private static string? rustHolder;

    private readonly Process process;
    private readonly Channel<string> lines = Channel.CreateUnbounded<string>();
    private readonly StringBuilder errors = new();
    private readonly Task pumps;

    private ForeignLock(ForeignClient client, Process process)
    {
        Client = client;
        this.process = process;
        pumps = Task.WhenAll(PumpOutputAsync(), PumpErrorsAsync());
    }

    /// <summary>The client whose lock this is.</summary>
    internal ForeignClient Client { get; }

    /// <summary>Starts a holder taking the lock, and waits until it holds it.</summary>
    /// <param name="client">The client.</param>
    /// <param name="lockFile">The lock file.</param>
    /// <param name="start">Starts a stand-in process instead of the foreign holder; null in interoperability tests.</param>
    /// <param name="within">How long acquisition may wait; the usual patience when null.</param>
    /// <returns>The holder, which the caller disposes.</returns>
    internal static async Task<ForeignLock> HoldAsync(ForeignClient client, string lockFile, Func<Process>? start = null, TimeSpan? within = null)
    {
        var holder = start is null ? await StartAsync(client, lockFile) : new ForeignLock(client, start());
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
    /// <param name="client">The client.</param>
    /// <param name="lockFile">The lock file.</param>
    /// <returns>The holder, which the caller disposes.</returns>
    internal static async Task<ForeignLock> StartAsync(ForeignClient client, string lockFile) =>
        new(client, Start(await CommandAsync(client), "hold", lockFile));

    /// <summary>Whether the client could take the lock right now; it releases it at once if so.</summary>
    /// <param name="client">The client.</param>
    /// <param name="lockFile">The lock file.</param>
    /// <returns>Whether it was free.</returns>
    internal static async Task<bool> IsFreeAsync(ForeignClient client, string lockFile)
    {
        await using var run = new ForeignLock(client, Start(await CommandAsync(client), "try", lockFile));
        var answer = await run.NextAsync(Patience);
        await run.ExitedAsync();
        return answer switch
        {
            "free" => true,
            "busy" => false,
            _ => throw new InvalidOperationException($"The {client} holder answered {answer ?? "nothing"} to try.\n{run.Errors}"),
        };
    }

    /// <summary>Waits until the holder holds the lock, failing the test if it does not within a while.</summary>
    /// <param name="within">How long to wait; the usual patience when null.</param>
    /// <returns>When it holds it.</returns>
    internal async Task HeldAsync(TimeSpan? within = null)
    {
        var line = await NextAsync(within ?? Patience);
        Assert.True(line == "held", $"The {Client} holder did not take the lock: it said {line ?? "nothing"}.\n{Errors}");
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
        Assert.True(line == "released", $"The {Client} holder did not release the lock: it said {line ?? "nothing"}.\n{Errors}");
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

    private static async Task<string[]> CommandAsync(ForeignClient client)
    {
        var tool = InteropGate.FromProcess.Require(client);
        return client switch
        {
            ForeignClient.Python => [tool, Repository.PathOf("client", "dotnet", "test", "interop", "lock_holder.py")],
            ForeignClient.Java => [tool, Repository.PathOf("client", "dotnet", "test", "interop", "LockHolder.java")],
            _ => [await RustHolderAsync(tool)],
        };
    }

    // Built once per test run from its locked manifest, outside the source tree.
    private static async Task<string> RustHolderAsync(string cargo)
    {
        await RustBuild.WaitAsync(CancellationToken.None);
        try
        {
            if (rustHolder is not null)
            {
                return rustHolder;
            }

            var target = Repository.PathOf("bin", "dotnet", "interop", "rust");
            var start = new ProcessStartInfo(cargo)
            {
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            foreach (var argument in new[] { "build", "--release", "--locked", "--quiet", "--manifest-path", Repository.PathOf("client", "dotnet", "test", "interop", "rust-lock-holder", "Cargo.toml"), "--target-dir", target })
            {
                start.ArgumentList.Add(argument);
            }

            using var build = Process.Start(start) ?? throw new InvalidOperationException($"{cargo} did not start.");
            var output = build.StandardOutput.ReadToEndAsync();
            var error = build.StandardError.ReadToEndAsync();
            using var timeout = new CancellationTokenSource(BuildPatience);
            try
            {
                await build.WaitForExitAsync(timeout.Token);
            }
            catch (OperationCanceledException)
            {
                build.Kill(entireProcessTree: true);
                Assert.Fail($"Building the Rust lock holder took more than {BuildPatience}.");
            }

            Assert.True(build.ExitCode == 0, $"Building the Rust lock holder failed with code {build.ExitCode}:\n{await output}{await error}");
            rustHolder = Path.Combine(target, "release", OperatingSystem.IsWindows() ? "opensysml-lock-holder.exe" : "opensysml-lock-holder");
            return rustHolder;
        }
        finally
        {
            RustBuild.Release();
        }
    }

    private static Process Start(string[] command, string mode, string lockFile)
    {
        var start = new ProcessStartInfo(command[0])
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var argument in command[1..])
        {
            start.ArgumentList.Add(argument);
        }

        start.ArgumentList.Add(mode);
        start.ArgumentList.Add(lockFile);
        return Process.Start(start) ?? throw new InvalidOperationException($"{command[0]} did not start.");
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
        Assert.True(process.ExitCode == 0, $"The {Client} holder exited with code {process.ExitCode}.\n{Errors}");
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
