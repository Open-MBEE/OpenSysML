using System;
using System.Collections.Concurrent;
using System.ComponentModel;
using System.Diagnostics;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Whether Python, which the foreign lock holder runs on, is there, and what its absence means. A developer without
/// it skips the tests that need it. Where they were promised by setting OPENSYSML_REQUIRE_INTEROP, as CI does, an
/// absence is the failure those tests exist to catch, so each such skip fails instead.
/// </summary>
/// <param name="environment">Reads an environment variable; null when it is unset.</param>
/// <param name="faultOf">
/// Why a Python found does not run, or null when it does; by default it is run with <c>--version</c>, since a
/// placeholder such as Windows' app execution alias for a Python not installed is a file but not a tool.
/// </param>
internal sealed class InteropGate(Func<string, string?> environment, Func<string, string?>? faultOf = null)
{
    private static readonly ConcurrentDictionary<string, string?> Faults = new(StringComparer.Ordinal);

    /// <summary>Set where Python is provided, so its absence fails instead of skipping.</summary>
    internal const string RequireVariable = "OPENSYSML_REQUIRE_INTEROP";

    /// <summary>The gate over this process's environment.</summary>
    internal static InteropGate FromProcess { get; } = new(Environment.GetEnvironmentVariable);

    /// <summary>Whether the environment promised Python, making its absence a failure.</summary>
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

    /// <summary>Decides, without acting on it, whether a test needing Python runs, skips or fails.</summary>
    /// <returns>The Python to run, or why there is none.</returns>
    internal Decision Decide()
    {
        var named = environment("PYTHON");
        string[] names = OperatingSystem.IsWindows() ? ["python", "python3"] : ["python3", "python"];
        var tool = (string.IsNullOrWhiteSpace(named) ? names : [named, .. names]).Select(Find).FirstOrDefault(found => found is not null);
        var reason = $"no Python: neither $PYTHON nor $PATH names {string.Join(" or ", names)}";
        if (tool is not null)
        {
            if ((faultOf ?? FaultOf)(tool) is not { } fault)
            {
                return new Decision(tool, null, Fails: false);
            }

            reason = $"no working Python: {tool} {fault}";
        }

        return Required
            ? new Decision(null, $"${RequireVariable} is set, so this test must run against the Python client's lock, but there is {reason}.", Fails: true)
            : new Decision(null, reason, Fails: false);
    }

    /// <summary>The full path of Python; skips or fails the running test without it.</summary>
    /// <returns>The path.</returns>
    internal string Require()
    {
        var decision = Decide();
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

    // Each Python is run once per test run.
    private static string? FaultOf(string tool) => Faults.GetOrAdd(tool, _ =>
    {
        var start = new ProcessStartInfo(tool) { RedirectStandardOutput = true, RedirectStandardError = true, UseShellExecute = false, CreateNoWindow = true };
        start.ArgumentList.Add("--version");
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

    private string? Find(string name) => ToolSearch.Find(name, environment("PATH"));

    /// <summary>What the gate decided: run <paramref name="Tool"/>, or skip or fail for <paramref name="Reason"/>.</summary>
    /// <param name="Tool">The full path of Python, when there is one.</param>
    /// <param name="Reason">Why there is none, when there is not.</param>
    /// <param name="Fails">Whether its absence fails the test rather than skipping it.</param>
    internal sealed record Decision(string? Tool, string? Reason, bool Fails);
}
