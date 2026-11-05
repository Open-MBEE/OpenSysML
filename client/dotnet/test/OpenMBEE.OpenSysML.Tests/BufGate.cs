using System;
using System.Collections.Generic;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Whether buf is there to build the schema the committed protocol code is held against, and what its absence means.
/// A developer without one skips the test that needs it. Where one was promised by setting OPENSYSML_REQUIRE_BUF, as
/// CI does, its absence is the failure that test exists to catch, so the skip fails instead.
/// </summary>
/// <param name="environment">Reads an environment variable; null when it is unset.</param>
internal sealed class BufGate(Func<string, string?> environment)
{
    /// <summary>Set where buf is provided, so a missing one fails instead of skipping.</summary>
    internal const string RequireVariable = "OPENSYSML_REQUIRE_BUF";

    /// <summary>Names the buf command as the Makefile reads it: a program, then any arguments before buf's own.</summary>
    internal const string CommandVariable = "BUF";

    /// <summary>The gate over this process's environment.</summary>
    internal static BufGate FromProcess { get; } = new(Environment.GetEnvironmentVariable);

    /// <summary>Whether the environment promised buf, making its absence a failure.</summary>
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

    /// <summary>Decides, without acting on it, whether the test needing buf runs, skips or fails.</summary>
    internal Decision Decide()
    {
        var searchPath = environment("PATH");
        var named = environment(CommandVariable);
        string reason;
        if (!string.IsNullOrWhiteSpace(named))
        {
            // Make hands $(BUF) to the shell, which splits it into words.
            var words = named.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries);
            if (ToolSearch.Find(words[0], searchPath) is { } program)
            {
                return new Decision([program, .. words[1..]], null, Fails: false);
            }

            reason = $"${CommandVariable} is \"{named.Trim()}\", and {words[0]} is neither a file nor on $PATH";
        }
        else if (ToolSearch.Find("buf", searchPath) is { } buf)
        {
            return new Decision([buf], null, Fails: false);
        }
        else
        {
            reason = $"no buf: neither ${CommandVariable} nor $PATH names one";
        }

        return Required
            ? new Decision(null, $"${RequireVariable} is set, so this test must build the schema with buf, but {reason}.", Fails: true)
            : new Decision(null, reason, Fails: false);
    }

    /// <summary>The buf command, its program a full path; skips or fails the running test without one.</summary>
    internal IReadOnlyList<string> RequireCommand()
    {
        var decision = Decide();
        if (decision.Command is not null)
        {
            return decision.Command;
        }

        if (decision.Fails)
        {
            Assert.Fail(decision.Reason);
        }

        Assert.Skip(decision.Reason!);
        throw new InvalidOperationException("Assert.Skip returned.");
    }

    /// <summary>What the gate decided: run <paramref name="Command"/>, or skip or fail for <paramref name="Reason"/>.</summary>
    /// <param name="Command">The program's full path and the arguments before buf's own, when there is one to run.</param>
    /// <param name="Reason">Why there is none, when there is not.</param>
    /// <param name="Fails">Whether its absence fails the test rather than skipping it.</param>
    internal sealed record Decision(IReadOnlyList<string>? Command, string? Reason, bool Fails);
}
