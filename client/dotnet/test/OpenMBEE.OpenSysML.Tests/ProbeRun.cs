using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Channels;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// One run of a probe (test/OpenMBEE.OpenSysML.OrphanProbe, LoadContextProbe or CacheProbe) as a process
/// of its own. Every process id it reports is recorded as it is read, and disposal kills the probe and
/// every one of them that still runs, so a failed assertion leaves nothing behind.
/// </summary>
internal sealed class ProbeRun : IAsyncDisposable
{
    /// <summary>The orphan probe's project and assembly name.</summary>
    internal const string Orphan = "OpenMBEE.OpenSysML.OrphanProbe";

    /// <summary>The load-context probe's project and assembly name.</summary>
    internal const string LoadContext = "OpenMBEE.OpenSysML.LoadContextProbe";

    /// <summary>The cache probe's project and assembly name.</summary>
    internal const string Cache = "OpenMBEE.OpenSysML.CacheProbe";

    private static readonly TimeSpan ReadTimeout = TimeSpan.FromSeconds(60);
    private static readonly string[] ProcessIdFields = ["probe", "service"];

    private readonly Process process;
    private readonly Channel<string> lines = Channel.CreateUnbounded<string>();
    private readonly StringBuilder errors = new();
    private readonly ConcurrentDictionary<int, bool> owned = new();
    private readonly Task pumps;

    private ProbeRun(Process process)
    {
        this.process = process;
        pumps = Task.WhenAll(PumpOutputAsync(), PumpErrorsAsync());
    }

    /// <summary>The probe's process id.</summary>
    internal int ProcessId => process.Id;

    /// <summary>Starts a probe with arguments, with no service named in its environment.</summary>
    /// <param name="probe"><see cref="Orphan"/>, <see cref="LoadContext"/> or <see cref="Cache"/>.</param>
    /// <param name="arguments">Its arguments.</param>
    /// <returns>The run, which the caller disposes.</returns>
    internal static ProbeRun Start(string probe, params string[] arguments) =>
        Start(probe, new Dictionary<string, string>(), arguments);

    /// <summary>Starts a probe with arguments, with no service named in its environment and these variables set in it.</summary>
    /// <param name="probe"><see cref="Orphan"/>, <see cref="LoadContext"/> or <see cref="Cache"/>.</param>
    /// <param name="environment">Variables to set in the probe's environment alone, by name.</param>
    /// <param name="arguments">Its arguments.</param>
    /// <returns>The run, which the caller disposes.</returns>
    internal static ProbeRun Start(string probe, IReadOnlyDictionary<string, string> environment, params string[] arguments)
    {
        FakeChild.EnsureRuntimeIsFound();
        var launcher = Repository.PathOf(
            "client", "dotnet", "test", probe, "bin",
            typeof(ProbeRun).Assembly.GetCustomAttribute<AssemblyConfigurationAttribute>()?.Configuration ?? "Release",
            "net10.0",
            OperatingSystem.IsWindows() ? probe + ".exe" : probe);
        if (!File.Exists(launcher))
        {
            throw new InvalidOperationException($"{launcher} is missing: build the solution, which builds test/{probe}.");
        }

        var start = new ProcessStartInfo(launcher)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        start.Environment.Remove("OPENSYSML_SERVICE");
        foreach (var (name, value) in environment)
        {
            start.Environment[name] = value;
        }

        foreach (var argument in arguments)
        {
            start.ArgumentList.Add(argument);
        }

        var run = new ProbeRun(Process.Start(start) ?? throw new InvalidOperationException($"{launcher} did not start."));
        run.owned[run.ProcessId] = true;
        return run;
    }

    /// <summary>The argument a probe takes for a wire.</summary>
    internal static string Argument(ClientWire wire) => wire switch
    {
        ClientWire.ConnectProto => "connect-proto",
        ClientWire.ConnectJson => "connect-json",
        _ => "grpc",
    };

    /// <summary>Reads the next line the probe writes, as JSON, recording the process ids it names.</summary>
    /// <returns>The line.</returns>
    internal async Task<JsonElement> ReadAsync()
    {
        using var timeout = new CancellationTokenSource(ReadTimeout);
        string line;
        try
        {
            line = await lines.Reader.ReadAsync(timeout.Token);
        }
        catch (Exception error) when (error is OperationCanceledException or ChannelClosedException)
        {
            var ending = process.HasExited ? $"it exited with code {process.ExitCode}" : $"it wrote nothing within {ReadTimeout}";
            Assert.Fail($"The probe gave no answer: {ending}.\n{Errors}");
            throw;
        }

        var element = JsonDocument.Parse(line).RootElement.Clone();
        foreach (var field in ProcessIdFields)
        {
            if (element.TryGetProperty(field, out var id))
            {
                owned[id.GetInt32()] = true;
            }
        }

        if (element.TryGetProperty("siblings", out var siblings))
        {
            foreach (var sibling in siblings.EnumerateArray())
            {
                owned[sibling.GetInt32()] = true;
            }
        }

        return element;
    }

    /// <summary>Reads every line until the probe exits by itself, which it must do within a while, with code 0.</summary>
    /// <returns>The lines, as JSON.</returns>
    internal async Task<IReadOnlyList<JsonElement>> ReadToExitAsync()
    {
        var read = new List<JsonElement>();
        using var timeout = new CancellationTokenSource(ReadTimeout);
        try
        {
            await process.WaitForExitAsync(timeout.Token);
        }
        catch (OperationCanceledException)
        {
            Assert.Fail($"The probe did not exit by itself within {ReadTimeout}: something it loaded keeps it running.\n{Errors}");
        }

        await DrainAsync();
        while (lines.Reader.TryPeek(out _))
        {
            read.Add(await ReadAsync());
        }

        Assert.True(process.ExitCode == 0, $"The probe exited with code {process.ExitCode}.\n{Errors}");
        return read;
    }

    /// <summary>Writes a line to the probe's input, which its waiting modes take as the go-ahead.</summary>
    /// <returns>When it is written.</returns>
    internal async Task SendLineAsync()
    {
        await process.StandardInput.WriteLineAsync();
        await process.StandardInput.FlushAsync();
    }

    /// <summary>Kills the probe alone, leaving whatever it started to fend for itself.</summary>
    internal void KillProbe()
    {
        process.Kill(entireProcessTree: false);
        process.WaitForExit();
    }

    /// <summary>What the probe wrote to its error output.</summary>
    internal string Errors
    {
        get
        {
            lock (errors)
            {
                return errors.ToString();
            }
        }
    }

    public async ValueTask DisposeAsync()
    {
        if (!process.HasExited)
        {
            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync(CancellationToken.None);
        }

        foreach (var id in owned.Keys)
        {
            FakeChild.Kill(id);
        }

        foreach (var id in owned.Keys)
        {
            await FakeChild.AssertGoneAsync(id);
        }

        await DrainAsync();
        process.Dispose();
    }

    // On Windows a process the probe started may hold an inherited copy of its output open past its exit;
    // what the probe wrote has been read by then, and the pumps end with that process.
    private async Task DrainAsync()
    {
        try
        {
            await pumps.WaitAsync(TimeSpan.FromSeconds(10));
        }
        catch (TimeoutException)
        {
        }
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
