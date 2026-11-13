using System;
using System.IO;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Whether a sysml-grpc service binary is there, and what its absence means.
/// A developer without one skips the tests that need it. Where one was promised
/// by setting OPENSYSML_REQUIRE_SERVICE, as CI does, its absence is the failure
/// those tests exist to catch, so each such skip fails instead.
/// </summary>
/// <param name="environment">Reads an environment variable; null when it is unset.</param>
internal sealed class ServiceGate(Func<string, string?> environment)
{
    /// <summary>Set where a service is provided, so a missing one fails instead of skipping.</summary>
    internal const string RequireServiceVariable = "OPENSYSML_REQUIRE_SERVICE";

    /// <summary>Names the sysml-grpc binary the integration tests run.</summary>
    internal const string BinaryVariable = "OPENSYSML_GRPC_BINARY";

    /// <summary>The gate over this process's environment.</summary>
    internal static ServiceGate FromProcess { get; } = new(Environment.GetEnvironmentVariable);

    /// <summary>Whether the environment promised a service, making its absence a failure.</summary>
    internal bool Required
    {
        get
        {
            var value = environment(RequireServiceVariable)?.Trim();
            return !(string.IsNullOrEmpty(value)
                || value == "0"
                || string.Equals(value, "false", StringComparison.OrdinalIgnoreCase)
                || string.Equals(value, "no", StringComparison.OrdinalIgnoreCase));
        }
    }

    /// <summary>Decides, without acting on it, whether a test needing the binary runs, skips or fails.</summary>
    internal Decision Decide()
    {
        var path = environment(BinaryVariable);
        string reason;
        if (string.IsNullOrWhiteSpace(path))
        {
            reason = $"${BinaryVariable} names no sysml-grpc binary";
        }
        else if (!File.Exists(path))
        {
            reason = $"${BinaryVariable} names {path}, which is not a file";
        }
        else
        {
            return new Decision(Path.GetFullPath(path), null, Fails: false);
        }

        return Required
            ? new Decision(null, $"${RequireServiceVariable} is set, so this test must run against a service, but {reason}.", Fails: true)
            : new Decision(null, reason, Fails: false);
    }

    /// <summary>The full path of the configured sysml-grpc binary; skips or fails the running test without one.</summary>
    internal string RequireBinary()
    {
        var decision = Decide();
        if (decision.Binary is not null)
        {
            return decision.Binary;
        }

        if (decision.Fails)
        {
            Assert.Fail(decision.Reason);
        }

        Assert.Skip(decision.Reason!);
        throw new InvalidOperationException("Assert.Skip returned.");
    }

    /// <summary>What the gate decided: run against <paramref name="Binary"/>, or skip or fail for <paramref name="Reason"/>.</summary>
    /// <param name="Binary">The full path of the binary, when there is one to run.</param>
    /// <param name="Reason">Why there is none, when there is not.</param>
    /// <param name="Fails">Whether its absence fails the test rather than skipping it.</param>
    internal sealed record Decision(string? Binary, string? Reason, bool Fails);
}
