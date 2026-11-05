using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A copy of the FakeService stand-in, in a directory of its own under the name the client looks
/// for, with the script it follows beside it. Each copy is a distinct binary path, so copies never
/// share a private service; one copy opened twice does.
/// </summary>
internal sealed class FakeChild : IDisposable
{
    private static readonly JsonSerializerOptions Json = new() { PropertyNamingPolicy = JsonNamingPolicy.CamelCase };

    private FakeChild(string directory)
    {
        Directory = directory;
        Path = System.IO.Path.Combine(directory, OperatingSystem.IsWindows() ? "sysml-grpc.exe" : "sysml-grpc");
    }

    /// <summary>The copy's directory.</summary>
    internal string Directory { get; }

    /// <summary>The launcher to start.</summary>
    internal string Path { get; }

    /// <summary>Where the stand-in writes its arguments, when its script says to.</summary>
    internal string ArgumentsFile => System.IO.Path.Combine(Directory, "arguments.txt");

    /// <summary>Where the stand-in writes once its input ends, when its script says to.</summary>
    internal string ExitMarker => System.IO.Path.Combine(Directory, "exited.txt");

    /// <summary>Where the stand-in writes its process id, when its script says to.</summary>
    internal string ProcessIdFile => System.IO.Path.Combine(Directory, "pid.txt");

    /// <summary>The process id the stand-in wrote, once it has.</summary>
    internal async Task<int> ProcessIdAsync()
    {
        string text = string.Empty;
        await ServiceRegistryTests.WaitUntilAsync(() => File.Exists(ProcessIdFile) && (text = File.ReadAllText(ProcessIdFile)).Length > 0);
        return int.Parse(text, System.Globalization.CultureInfo.InvariantCulture);
    }

    /// <summary>The stand-in's build output, which every copy is made from.</summary>
    internal static string BuildOutput { get; } = Repository.PathOf(
        "client", "dotnet", "test", "FakeService", "bin",
        typeof(FakeChild).Assembly.GetCustomAttribute<AssemblyConfigurationAttribute>()?.Configuration ?? "Release",
        "net10.0");

    /// <summary>Copies the stand-in, to follow a script.</summary>
    /// <param name="script">What it does; its file paths may use <see cref="ArgumentsFile"/> and <see cref="ExitMarker"/>.</param>
    /// <returns>The copy, which the caller disposes.</returns>
    internal static FakeChild Create(Func<FakeChild, FakeScript> script)
    {
        EnsureRuntimeIsFound();
        var directory = System.IO.Path.Combine(System.IO.Path.GetTempPath(), "opensysml-fake-" + Guid.NewGuid().ToString("N"));
        System.IO.Directory.CreateDirectory(directory);
        var child = new FakeChild(directory);
        var launcher = System.IO.Path.Combine(BuildOutput, OperatingSystem.IsWindows() ? "FakeService.exe" : "FakeService");
        if (!File.Exists(launcher))
        {
            throw new InvalidOperationException($"{launcher} is missing: build the solution, which builds test/FakeService.");
        }

        foreach (var file in System.IO.Directory.EnumerateFiles(BuildOutput))
        {
            File.Copy(file, System.IO.Path.Combine(directory, System.IO.Path.GetFileName(file)));
        }

        File.Move(System.IO.Path.Combine(directory, System.IO.Path.GetFileName(launcher)), child.Path);
        File.WriteAllText(System.IO.Path.Combine(directory, "fake-service.json"), JsonSerializer.Serialize(script(child), Json));
        return child;
    }

    /// <summary>A copy that reports an address, records its arguments, and exits in order when its input ends.</summary>
    internal static FakeChild Serving() =>
        Create(child => new FakeScript { Report = "127.0.0.1:50051", ArgumentsFile = child.ArgumentsFile, ExitMarker = child.ExitMarker });

    /// <summary>Waits for a process to be gone, failing the test if it is still running after a while.</summary>
    /// <param name="processId">The process.</param>
    /// <returns>When it is gone.</returns>
    internal static async Task AssertGoneAsync(int processId)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(15);
        while (IsRunning(processId))
        {
            Assert.True(DateTime.UtcNow < deadline, $"process {processId} is still running");
            await Task.Delay(20, CancellationToken.None);
        }
    }

    /// <summary>Whether a process is running.</summary>
    internal static bool IsRunning(int processId)
    {
        try
        {
            using var process = Process.GetProcessById(processId);
            return !process.HasExited;
        }
        catch (ArgumentException)
        {
            return false;
        }
        catch (InvalidOperationException)
        {
            return false;
        }
    }

    /// <summary>Kills a process the test started, if it still runs.</summary>
    internal static void Kill(int processId)
    {
        try
        {
            using var process = Process.GetProcessById(processId);
            process.Kill(entireProcessTree: true);
        }
        catch (ArgumentException)
        {
        }
        catch (InvalidOperationException)
        {
        }
    }

    public void Dispose()
    {
        try
        {
            System.IO.Directory.Delete(Directory, recursive: true);
        }
        catch (IOException)
        {
        }
        catch (UnauthorizedAccessException)
        {
        }
    }

    // The launcher finds the runtime through DOTNET_ROOT where it is not installed in the default place;
    // dotnet test sets it for its own host, and children inherit it. Set it if this process was not given it.
    internal static void EnsureRuntimeIsFound()
    {
        if (string.IsNullOrEmpty(Environment.GetEnvironmentVariable("DOTNET_ROOT")))
        {
            var runtime = new DirectoryInfo(RuntimeEnvironment.GetRuntimeDirectory());
            Environment.SetEnvironmentVariable("DOTNET_ROOT", runtime.Parent?.Parent?.Parent?.FullName);
        }
    }
}

/// <summary>What a <see cref="FakeChild"/> does, as test/FakeService reads it.</summary>
internal sealed record FakeScript
{
    public string? ArgumentsFile { get; init; }

    public string? ProcessIdFile { get; init; }

    public string? Stderr { get; init; }

    public int StderrBytesBeforeReport { get; init; }

    public int FloodLineLength { get; init; } = 100;

    public int DelayMilliseconds { get; init; }

    public string? Report { get; init; }

    public bool ReportWithoutNewline { get; init; }

    public bool CloseStdout { get; init; }

    public int? ExitCode { get; init; }

    public bool FloodAfterReport { get; init; }

    public bool IgnoreStdinEnd { get; init; }

    public string? ExitMarker { get; init; }
}
