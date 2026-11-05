using System;
using System.Collections.Concurrent;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>A client whose shared cache lock the .NET client's must interoperate with.</summary>
public enum ForeignClient
{
    /// <summary>The Python client: <c>fcntl.lockf</c>, or <c>msvcrt.locking</c> on Windows.</summary>
    Python,

    /// <summary>The Java client: <c>FileChannel.tryLock</c>, polled.</summary>
    Java,

    /// <summary>The Rust client: <c>rustix::fs::fcntl_lock</c>, or fs4's <c>LockFileEx</c> on Windows.</summary>
    Rust,
}

/// <summary>
/// Whether the toolchain a foreign client's lock holder needs is there, and what its absence means. A developer
/// without one skips the tests that need it. Where they were promised by setting OPENSYSML_REQUIRE_INTEROP, as CI
/// does, an absence is the failure those tests exist to catch, so each such skip fails instead.
/// </summary>
/// <param name="environment">Reads an environment variable; null when it is unset.</param>
/// <param name="faultOf">
/// Why a tool found for a client does not run, or null when it does; by default it is run with its version option,
/// since a placeholder such as macOS's <c>/usr/bin/java</c> without a JDK is a file but not a tool.
/// </param>
internal sealed class InteropGate(Func<string, string?> environment, Func<ForeignClient, string, string?>? faultOf = null)
{
    private static readonly ConcurrentDictionary<string, string?> Faults = new(StringComparer.Ordinal);

    /// <summary>Set where the toolchains are provided, so a missing one fails instead of skipping.</summary>
    internal const string RequireVariable = "OPENSYSML_REQUIRE_INTEROP";

    /// <summary>The gate over this process's environment.</summary>
    internal static InteropGate FromProcess { get; } = new(Environment.GetEnvironmentVariable);

    /// <summary>Every foreign client, for a theory each runs.</summary>
    public static TheoryData<ForeignClient> All => [ForeignClient.Python, ForeignClient.Java, ForeignClient.Rust];

    /// <summary>Whether the environment promised the toolchains, making an absence a failure.</summary>
    internal bool Required
    {
        get
        {
            var value = environment(RequireVariable)?.Trim();
            return !(string.IsNullOrEmpty(value)
                || value == "0"
                || string.Equals(value, "false", StringComparison.OrdinalIgnoreCase)
                || string.Equals(value, "no", StringComparison.OrdinalIgnoreCase));
        }
    }

    /// <summary>Decides, without acting on it, whether a test needing a client's toolchain runs, skips or fails.</summary>
    /// <param name="client">The client.</param>
    /// <returns>The tool to run (python, java or cargo), or why there is none.</returns>
    internal Decision Decide(ForeignClient client)
    {
        string? tool;
        string reason;
        switch (client)
        {
            case ForeignClient.Python:
                var named = environment("PYTHON");
                string[] names = OperatingSystem.IsWindows() ? ["python", "python3"] : ["python3", "python"];
                tool = (string.IsNullOrWhiteSpace(named) ? names : [named, .. names]).Select(Find).FirstOrDefault(found => found is not null);
                reason = $"no Python: neither $PYTHON nor $PATH names {string.Join(" or ", names)}";
                break;
            case ForeignClient.Java:
                tool = JavaFrom(environment("JAVA_HOME") is { Length: > 0 } javaHome ? Path.Combine(javaHome, "bin") : null) ?? JavaFrom(null);
                reason = "no JDK: neither $JAVA_HOME/bin nor $PATH has both java and javac, which runs LockHolder.java from source";
                break;
            default:
                var home = environment(OperatingSystem.IsWindows() ? "USERPROFILE" : "HOME");
                tool = Find("cargo") ?? (home is { Length: > 0 } ? ToolSearch.Existing(Path.Combine(home, ".cargo", "bin", ToolSearch.Executable("cargo"))) : null);
                reason = "no Rust: neither $PATH nor ~/.cargo/bin has cargo, which builds the Rust lock holder";
                break;
        }

        if (tool is not null)
        {
            if ((faultOf ?? FaultOf)(client, tool) is not { } fault)
            {
                return new Decision(tool, null, Fails: false);
            }

            reason = $"no working {client} toolchain: {tool} {fault}";
        }

        return Required
            ? new Decision(null, $"${RequireVariable} is set, so this test must run against the {client} client's lock, but there is {reason}.", Fails: true)
            : new Decision(null, reason, Fails: false);
    }

    /// <summary>The full path of a client's tool; skips or fails the running test without one.</summary>
    /// <param name="client">The client.</param>
    /// <returns>The path of python, java or cargo.</returns>
    internal string Require(ForeignClient client)
    {
        var decision = Decide(client);
        if (decision.Tool is not null)
        {
            return decision.Tool;
        }

        if (decision.Fails)
        {
            Assert.Fail(decision.Reason);
        }

        Assert.Skip(decision.Reason!);
        throw new InvalidOperationException("Assert.Skip returned.");
    }

    // Each tool is run once per test run.
    private static string? FaultOf(ForeignClient client, string tool) => Faults.GetOrAdd(tool, _ =>
    {
        var start = new ProcessStartInfo(tool) { RedirectStandardOutput = true, RedirectStandardError = true, UseShellExecute = false, CreateNoWindow = true };
        start.ArgumentList.Add(client == ForeignClient.Java ? "-version" : "--version");
        try
        {
            using var process = Process.Start(start)!;
            var output = process.StandardOutput.ReadToEndAsync();
            var error = process.StandardError.ReadToEndAsync();
            if (!process.WaitForExit(TimeSpan.FromSeconds(60)))
            {
                process.Kill(entireProcessTree: true);
                return "did not answer its version option within a minute";
            }

            var said = (error.Result + output.Result).Trim().Split('\n')[0].Trim();
            return process.ExitCode == 0 ? null : $"exits with code {process.ExitCode} when asked its version: {said}";
        }
        catch (Win32Exception failure)
        {
            return $"cannot be run: {failure.Message}";
        }
    });

    private string? JavaFrom(string? directory)
    {
        if (directory is null)
        {
            return Find("javac") is null ? null : Find("java");
        }

        return ToolSearch.Existing(Path.Combine(directory, ToolSearch.Executable("javac"))) is null
            ? null
            : ToolSearch.Existing(Path.Combine(directory, ToolSearch.Executable("java")));
    }

    private string? Find(string name) => ToolSearch.Find(name, environment("PATH"));

    /// <summary>What the gate decided: run <paramref name="Tool"/>, or skip or fail for <paramref name="Reason"/>.</summary>
    /// <param name="Tool">The full path of the tool, when there is one.</param>
    /// <param name="Reason">Why there is none, when there is not.</param>
    /// <param name="Fails">Whether its absence fails the test rather than skipping it.</param>
    internal sealed record Decision(string? Tool, string? Reason, bool Fails);
}
